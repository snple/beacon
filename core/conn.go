package core

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// errNoProgress 表示客户端在仍有未确认消息时反复拉取，超过硬上限。
var errNoProgress = errors.New("client keeps pulling without commit")

// errSessionStale 表示连接已不是会话的当前连接（被接管）。
var errSessionStale = ErrSessionTakenOver

// protocolError 表示对端违反了协议（非法包类型、非法时序）。
//
// serve 会据此向对端发送 DISCONNECT(reason) 再关闭，而不是静默断开。
type protocolError struct {
	reason packet.ReasonCode
	msg    string
}

func (e *protocolError) Error() string { return e.msg }

func newProtocolError(reason packet.ReasonCode, msg string) error {
	return &protocolError{reason: reason, msg: msg}
}

// conn 表示单次网络连接。
type conn struct {
	core   *Core
	client *Client
	nc     net.Conn

	writeMu      sync.Mutex
	writeTimeout time.Duration

	maxPacketSize     uint32 // 出站限制（对端声明）
	readMaxPacketSize uint32 // 入站限制
	keepAlive         uint16

	remoteAddr string

	ctx    context.Context
	cancel context.CancelFunc

	taskMu      sync.Mutex
	tasksClosed bool
	wg          sync.WaitGroup

	closed    atomic.Bool
	closeOnce sync.Once

	wakeCh        chan struct{}
	pullInflight  atomic.Bool
	serverBy      atomic.Bool
	registered    atomic.Bool
	streamCapable atomic.Bool

	disconnect atomic.Pointer[packet.DisconnectPacket]

	// endReason 记录连接结束的原因码，用于 OnDisconnect 钩子与统计。
	endReason atomic.Uint32

	pullMu     sync.Mutex
	served     int
	noProgress int
}

func newConn(core *Core, nc net.Conn) *conn {
	ctx, cancel := context.WithCancel(context.Background())
	cn := &conn{
		core:              core,
		nc:                nc,
		writeTimeout:      core.options.WriteTimeout,
		readMaxPacketSize: core.options.MaxPacketSize,
		keepAlive:         core.options.KeepAlive,
		ctx:               ctx,
		cancel:            cancel,
		wakeCh:            make(chan struct{}, 1),
	}
	if nc != nil {
		cn.remoteAddr = nc.RemoteAddr().String()
	}
	return cn
}

// spawn 启动受 wg 跟踪的协程；连接已关闭时返回 false。
func (cn *conn) spawn(fn func()) bool {
	cn.taskMu.Lock()
	defer cn.taskMu.Unlock()
	if cn.tasksClosed {
		return false
	}
	cn.wg.Add(1)
	go func() {
		defer cn.wg.Done()
		fn()
	}()
	return true
}

// notify 唤醒等待中的 PULL（非阻塞）。
func (cn *conn) notify() {
	select {
	case cn.wakeCh <- struct{}{}:
	default:
	}
}

// writePacket 写出单个数据包。
func (cn *conn) writePacket(pkt packet.Packet) error {
	if cn.closed.Load() {
		return net.ErrClosed
	}

	cn.writeMu.Lock()
	defer cn.writeMu.Unlock()

	if cn.writeTimeout > 0 {
		_ = cn.nc.SetWriteDeadline(time.Now().Add(cn.writeTimeout))
	}
	maxSize := cn.maxPacketSize
	if maxSize == 0 {
		maxSize = cn.core.options.MaxPacketSize
	}
	n, err := packet.WritePacketWithSize(cn.nc, pkt, maxSize)
	if err != nil {
		return err
	}
	cn.core.stats.BytesSent.Add(int64(n))
	if cn.writeTimeout > 0 {
		_ = cn.nc.SetWriteDeadline(time.Time{})
	}
	return nil
}

// serve 运行读循环直到连接结束。
func (cn *conn) serve() error {
	for {
		if cn.closed.Load() {
			return nil
		}
		if timeout := cn.readTimeout(); timeout > 0 {
			_ = cn.nc.SetReadDeadline(time.Now().Add(timeout))
		}

		pkt, size, err := packet.ReadPacketWithSize(cn.nc, cn.readMaxPacketSize)
		if err != nil {
			cn.classifyReadError(err)
			return err
		}

		cn.core.stats.BytesReceived.Add(int64(size))
		if cn.client != nil {
			cn.client.lastSeen.Store(time.Now().UnixMilli())
		}

		if err := cn.handlePacket(pkt); err != nil {
			var pe *protocolError
			switch {
			case errors.As(err, &pe):
				// 协议错误：显式通知对端后关闭，而不是让对端只看到 EOF。
				cn.setEndReason(pe.reason)
				cn.terminate(pe.reason, true)
			case errors.Is(err, errSessionStale):
				cn.setEndReason(packet.ReasonSessionTakenOver)
				cn.terminate(packet.ReasonSessionTakenOver, true)
			case errors.Is(err, errNoProgress):
				cn.setEndReason(packet.ReasonCommitRequired)
				cn.terminate(packet.ReasonCommitRequired, true)
			default:
				// 写失败等非协议错误：记录原因，避免被当成心跳超时。
				if cn.endReasonCode() == 0 {
					cn.setEndReason(packet.ReasonUnspecifiedError)
				}
			}
			return err
		}
	}
}

// classifyReadError 区分读取失败的原因：分帧/解码错误需要显式通知对端，
// 心跳超时与对端正常关闭则只记录原因。
func (cn *conn) classifyReadError(err error) {
	if cn.closed.Load() {
		return
	}

	var tooLarge *packet.PacketTooLargeError
	if errors.As(err, &tooLarge) {
		// 帧级超限：整条连接不可继续（包边界不可信），属于连接级错误。
		cn.setEndReason(packet.ReasonPacketTooLarge)
		cn.terminate(packet.ReasonPacketTooLarge, true)
		return
	}

	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		cn.setEndReason(packet.ReasonKeepAliveTimeout)
		return
	}

	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return
	}

	cn.setEndReason(packet.ReasonMalformedPacket)
	cn.terminate(packet.ReasonMalformedPacket, true)
}

func (cn *conn) readTimeout() time.Duration {
	ka := cn.keepAlive
	if ka == 0 {
		return 0
	}
	// 1.5 倍窗口：允许一个心跳周期再加半个周期的抖动
	return time.Duration(float64(ka)*1.5) * time.Second
}

// handlePacket 分发客户端请求。
func (cn *conn) handlePacket(pkt packet.Packet) error {
	switch p := pkt.(type) {
	case *packet.PushPacket:
		return cn.handlePush(p)
	case *packet.PullPacket:
		return cn.handlePull(p)
	case *packet.CommitPacket:
		return cn.handleCommit(p)
	case *packet.DeclarePacket:
		return cn.handleDeclare(p)
	case *packet.PingPacket:
		return cn.handlePing(p)
	case *packet.DisconnectPacket:
		return cn.handleDisconnect(p)
	case *packet.StreamOpenPacket:
		return cn.handleStreamOpen(p)
	case *packet.StreamAcceptPacket:
		return cn.handleStreamAccept(p)
	default:
		// 客户端发送了服务端方向的包（CONNACK/PUSH_OK/...）或重复的 CONNECT：
		// 这是协议错误，必须显式 DISCONNECT 而不是静默忽略。
		cn.core.logger.Debug("unexpected packet",
			zap.String("type", pkt.Type().String()),
			zap.String("remote", cn.remoteAddr))
		return newProtocolError(packet.ReasonProtocolError,
			"unexpected packet type: "+pkt.Type().String())
	}
}

// checkOwner 校验连接仍是会话的当前连接（fencing）。
func (cn *conn) checkOwner() error {
	cl := cn.client
	if cl == nil {
		return ErrNotAuthorized
	}
	if !cl.owns(cn) {
		return errSessionStale
	}
	return nil
}

func (cn *conn) handlePing(p *packet.PingPacket) error {
	if err := cn.checkOwner(); err != nil {
		return err
	}
	return cn.writePacket(packet.NewPongPacket(p.Seq, p.Timestamp))
}

func (cn *conn) handleDisconnect(p *packet.DisconnectPacket) error {
	cn.disconnect.Store(p)
	return io.EOF
}

// lastDisconnect 返回客户端发来的 DISCONNECT（若有）。
func (cn *conn) lastDisconnect() *packet.DisconnectPacket {
	return cn.disconnect.Load()
}

// setEndReason 记录连接结束原因。
func (cn *conn) setEndReason(reason packet.ReasonCode) {
	cn.endReason.Store(uint32(reason))
}

// endReasonCode 返回记录的连接结束原因（0 表示未记录）。
func (cn *conn) endReasonCode() packet.ReasonCode {
	return packet.ReasonCode(cn.endReason.Load())
}

// terminate 由服务端主动关闭连接：先发 DISCONNECT 再关闭。
func (cn *conn) terminate(reason packet.ReasonCode, suppressWill bool) {
	cn.serverBy.Store(true)
	cn.setEndReason(reason)
	select {
	case <-cn.ctx.Done():
	default:
		dp := packet.NewDisconnectPacket(reason)
		if suppressWill {
			dp.Flags |= packet.DisconnectFlagSuppressWill
		}
		_ = cn.writePacket(dp)
	}
	cn.close()
}

// close 关闭底层连接。后台协程由 wait 等待，避免在 waitLoop 自身
// （例如投递钩子调用管理面断开）中等待自己导致死锁。
func (cn *conn) close() {
	cn.closeOnce.Do(func() {
		cn.closed.Store(true)

		cn.taskMu.Lock()
		cn.tasksClosed = true
		cn.taskMu.Unlock()

		cn.cancel()
		if cn.nc != nil {
			_ = cn.nc.Close()
		}
	})
}

// wait 等待本连接的后台协程退出。
func (cn *conn) wait() { cn.wg.Wait() }

// ============================================================================
// PULL
// ============================================================================

// pullPlan 描述一次 PULL 的结果：写成功后才推进状态。
type pullPlan struct {
	resp       *packet.PullOkPacket
	pending    []taken // 从 pending 取出、写成功后应用
	delivered  []taken // 仅为发送而 retain、写成功后释放
	nextServed int
	empty      bool
	epoch      uint64 // 构建计划时的会话 resetEpoch
}

func recordsOf(batch []taken, dup bool) []packet.Record {
	out := make([]packet.Record, len(batch))
	for i, t := range batch {
		out[i] = t.msg.record(dup)
	}
	return out
}

// buildPlan 构造一次拉取计划（不改变任何状态）。
func (cn *conn) buildPlan(p *packet.PullPacket) *pullPlan {
	cl := cn.client
	opts := cn.core.options

	maxRecords := int(p.MaxRecords)
	if maxRecords <= 0 || maxRecords > int(opts.MaxRecords) {
		maxRecords = int(opts.MaxRecords)
	}
	maxBytes := int(p.MaxBytes)
	if maxBytes <= 0 || maxBytes > int(opts.MaxBytes) {
		maxBytes = int(opts.MaxBytes)
	}
	// 客户端在 CONNECT 中声明的接收上限同样必须满足，否则写包阶段才会失败并断连。
	maxBytes = cn.payloadBudget(maxBytes)

	pendingDepth, deliveredDepth, oldest := cl.mb.depth()

	plan := &pullPlan{resp: packet.NewPullOkPacket(p.RequestID), epoch: cl.resetEpoch.Load()}

	// 1) delivered 优先：未确认的消息先重发
	if deliveredDepth > 0 {
		cn.pullMu.Lock()
		batch, next, more, repeated := cl.mb.drainDelivered(cn.served, maxRecords, maxBytes)
		if len(batch) > 0 {
			if repeated {
				cn.noProgress++
			}

			if hard := opts.NoProgressHard; hard > 0 && cn.noProgress > hard {
				cl.mb.releaseBatch(batch)
				cn.pullMu.Unlock()
				return &pullPlan{resp: nil, empty: true}
			}
			if soft := opts.NoProgressSoft; soft > 0 && cn.noProgress > soft {
				cl.mb.releaseBatch(batch)
				resp := packet.NewPullOkPacket(p.RequestID)
				resp.Reason = packet.ReasonCommitRequired
				resp.RetryAfterMs = 1000
				resp.Pending = pendingDepth
				resp.Delivered = deliveredDepth
				resp.OldestUncommitted = oldest
				cn.core.stats.CommitRequired.Add(1)
				cn.pullMu.Unlock()
				return &pullPlan{resp: resp}
			}
			cn.pullMu.Unlock()

			if !cn.fitsClientPacket(batch) {
				cl.mb.releaseBatch(batch)
				return cn.packetTooLargePlan(p.RequestID, pendingDepth, deliveredDepth, oldest)
			}

			plan.resp.More = more
			plan.resp.Pending = pendingDepth
			plan.resp.Delivered = deliveredDepth
			plan.resp.OldestUncommitted = oldest
			plan.resp.Records = recordsOf(batch, true)
			plan.delivered = batch
			plan.nextServed = next
			return plan
		}
		cn.pullMu.Unlock()
	}

	// 2) 从 pending 取
	batch, more := cl.mb.takePending(maxRecords, maxBytes)

	// 投递钩子：拒绝的条目直接丢弃（不进入 delivered）
	if len(batch) > 0 && cn.core.options.Hooks.hasOnDeliver() {
		kept := batch[:0]
		for _, t := range batch {
			if cn.core.options.Hooks.callOnDeliver(&DeliverContext{
				ClientID: cl.ID,
				Channel:  t.msg.Channel,
				Source:   t.msg.Source,
				Priority: t.prio,
				Payload:  t.msg.Payload,
			}) {
				kept = append(kept, t)
				continue
			}
			cn.core.releaseMessage(t.msg)
			cn.core.stats.MessagesDropped.Add(1)
			cl.dropped.Add(1)
		}
		batch = kept
	}

	// 单条记录可能大于客户端声明的接收上限：恢复队列并明确返回错误，而不是写完才断连。
	if len(batch) > 0 && !cn.fitsClientPacket(batch) {
		cl.mb.restore(batch)
		return cn.packetTooLargePlan(p.RequestID, pendingDepth, deliveredDepth, oldest)
	}

	if len(batch) == 0 {
		plan.empty = true
		plan.resp.Pending = pendingDepth
		plan.resp.Delivered = deliveredDepth
		plan.resp.OldestUncommitted = oldest
		return plan
	}

	reliable := 0
	for _, t := range batch {
		if t.prio.RequiresCommit() {
			reliable++
		}
	}

	plan.resp.More = more
	plan.resp.Pending = [3]uint32{
		pendingDepth[0], pendingDepth[1], pendingDepth[2],
	}
	// 队列深度是扣除本批之后的近似值
	plan.resp.Pending[0] = saturatingSub(plan.resp.Pending[0], uint32(countPrio(batch, packet.PriorityLow)))
	plan.resp.Pending[1] = saturatingSub(plan.resp.Pending[1], uint32(countPrio(batch, packet.PriorityMedium)))
	plan.resp.Pending[2] = saturatingSub(plan.resp.Pending[2], uint32(countPrio(batch, packet.PriorityHigh)))
	plan.resp.Delivered = deliveredDepth + uint32(reliable)
	if !oldest.IsZero() {
		plan.resp.OldestUncommitted = oldest
	} else {
		for _, t := range batch {
			if t.prio.RequiresCommit() {
				plan.resp.OldestUncommitted = t.id
				break
			}
		}
	}
	plan.resp.Records = recordsOf(batch, false)
	plan.pending = batch
	plan.nextServed = 0
	return plan
}

// responseBudget 返回本连接可发送的最大响应帧字节数（0 表示不限制）。
func (cn *conn) responseBudget() int {
	if cn.maxPacketSize == 0 {
		return 0
	}
	return int(cn.maxPacketSize)
}

// pushResultSize 是 PUSH_OK 中单条结果的编码大小。
const pushResultSize = 1 + packet.PacketIDSize + 4 + 4

// pushOkOverhead 是 PUSH_OK 的固定头 + RequestID + Count。
const pushOkOverhead = 5 + 8 + 2

// trimPushResults 把结果裁剪到不超过对端声明的大小，至少保留一条。
//
// 失败项极多时（例如整批 rejected）逐条回显会超过小缓冲客户端的 MaxPacketSize，
// 此时写包失败将被迫断连；裁剪后仍能传达"本批失败"以及首条原因。
func trimPushResults(results []packet.PushResult, budget int) []packet.PushResult {
	if budget <= 0 || len(results) <= 1 {
		return results
	}
	max := (budget - pushOkOverhead) / pushResultSize
	if max < 1 {
		max = 1
	}
	if len(results) <= max {
		return results
	}
	return results[:max]
}

// declareRespOverhead 是 DECLARE_RESP 的固定头 + 固定字段（不含 Results）。
const declareRespOverhead = 5 + 8 + 2 + 2 + 12 + 4 + 4 + 2 + 4 + 4 + 2

// trimDeclareResults 把失败项裁剪到不超过对端声明的大小，至少保留一条。
func trimDeclareResults(results []packet.ChannelResult, budget int) []packet.ChannelResult {
	if budget <= 0 || len(results) <= 1 {
		return results
	}
	size := declareRespOverhead
	keep := 0
	for _, r := range results {
		size += 2 + len(r.Channel) + 1
		if size > budget && keep > 0 {
			break
		}
		keep++
	}
	if keep < 1 {
		keep = 1
	}
	return results[:keep]
}

// pullResponseOverhead 是 PULL_OK 的固定头 + 最小包体大小。
const pullResponseOverhead = 5 + 44

// payloadBudget 把批次字节预算限制在客户端声明的最大包大小之内。
func (cn *conn) payloadBudget(maxBytes int) int {
	limit := int(cn.maxPacketSize)
	if limit <= 0 {
		return maxBytes
	}
	limit -= pullResponseOverhead
	if limit < 1 {
		limit = 1
	}
	if maxBytes <= 0 || maxBytes > limit {
		return limit
	}
	return maxBytes
}

// pullRecordsSize 返回该批记录编码进 PULL_OK 后的近似总字节数。
func pullRecordsSize(batch []taken) int {
	total := pullResponseOverhead
	for _, t := range batch {
		rec := t.msg.record(false)
		total += packet.RecordSize(&rec)
	}
	return total
}

func (cn *conn) fitsClientPacket(batch []taken) bool {
	if cn.maxPacketSize == 0 {
		return true
	}
	return pullRecordsSize(batch) <= int(cn.maxPacketSize)
}

func (cn *conn) packetTooLargePlan(requestID uint64, pending [3]uint32, delivered uint32, oldest packet.PacketID) *pullPlan {
	resp := packet.NewPullOkPacket(requestID)
	resp.Reason = packet.ReasonPacketTooLarge
	resp.Pending = pending
	resp.Delivered = delivered
	resp.OldestUncommitted = oldest
	cn.core.stats.MessagesRejected.Add(1)
	return &pullPlan{resp: resp}
}

func countPrio(batch []taken, p packet.Priority) int {
	n := 0
	for _, t := range batch {
		if t.prio == p {
			n++
		}
	}
	return n
}

func saturatingSub(a, b uint32) uint32 {
	if b > a {
		return 0
	}
	return a - b
}

// applyPlan 在写成功后推进状态。
func (cn *conn) applyPlan(plan *pullPlan) {
	cl := cn.client

	cn.pullMu.Lock()
	if len(plan.delivered) > 0 {
		cn.served = plan.nextServed
		cn.core.stats.DuplicatesSent.Add(int64(len(plan.delivered)))
	}
	if len(plan.pending) > 0 {
		cn.served = 0
		cn.noProgress = 0
	}
	cn.pullMu.Unlock()

	if len(plan.delivered) > 0 {
		cl.mb.releaseBatch(plan.delivered)
	}
	if len(plan.pending) > 0 {
		cl.mb.applySent(plan.pending)
		if reliable := len(plan.pending) - countPrio(plan.pending, packet.PriorityLow); reliable > 0 {
			if cl.deliveredSince.Load() == 0 {
				cl.deliveredSince.Store(time.Now().UnixMilli())
			}
		}
	}

	n := len(plan.delivered) + len(plan.pending)
	if n > 0 {
		cn.core.stats.MessagesSent.Add(int64(n))
		cl.sent.Add(int64(n))
	}
}

// stalePlan 校验计划是否仍属于当前会话状态。
//
// 写出是异步的：期间连接可能被同 ClientID 的新连接接管（fencing），
// 或会话被 CleanSession 重置。任一情况都不允许旧连接再推进 served/delivered。
func (cn *conn) stalePlan(plan *pullPlan) error {
	if !cn.client.owns(cn) {
		return errSessionStale
	}
	if cn.client.resetEpoch.Load() != plan.epoch {
		return errSessionStale
	}
	return nil
}

// abortPlan 在写失败或被接管后回滚。
//
// 若会话已被 CleanSession 重置，则不回滚：把已取出的消息写回新会话会复活
// 本应被清理的数据，此时只释放引用。
func (cn *conn) abortPlan(plan *pullPlan) {
	if cn.stalePlan(plan) != nil {
		cn.discardPlan(plan)
		return
	}
	cl := cn.client
	if len(plan.delivered) > 0 {
		cl.mb.releaseBatch(plan.delivered)
	}
	if len(plan.pending) > 0 {
		cl.mb.restore(plan.pending)
	}
}

// discardPlan 在连接已失去会话所有权时丢弃计划。
//
// 此时会话可能已被接管或 CleanSession 重置，不能再把消息放回队列
// 或推进 delivered（旧连接不得产生任何状态变更），只释放本批引用。
func (cn *conn) discardPlan(plan *pullPlan) {
	cl := cn.client
	if len(plan.delivered) > 0 {
		cl.mb.releaseBatch(plan.delivered)
	}
	for _, t := range plan.pending {
		cn.core.releaseMessage(t.msg)
	}
}

// handlePull 处理 PULL。
//
// 同一时刻只允许一个在途 PULL：并发拉取会让同一条消息被重复投递，
// 也会让客户端的未确认集合互相覆盖。第二个请求会立刻收到 RateLimited。
func (cn *conn) handlePull(p *packet.PullPacket) error {
	if err := cn.checkOwner(); err != nil {
		return err
	}

	if !cn.pullInflight.CompareAndSwap(false, true) {
		return cn.writePullRateLimited(p)
	}

	plan := cn.buildPlan(p)
	if plan.resp == nil {
		cn.pullInflight.Store(false)
		return errNoProgress
	}

	// buildPlan 可能执行用户钩子；期间连接可能已被接管。
	if !cn.client.owns(cn) {
		cn.abortPlan(plan)
		cn.pullInflight.Store(false)
		return errSessionStale
	}

	if plan.empty {
		if waitMs := cn.waitMillis(p.WaitMs); waitMs > 0 {
			// 长轮询：把响应交给后台协程，读循环可以继续处理 PING/COMMIT。
			if cn.spawn(func() {
				defer cn.pullInflight.Store(false)
				cn.waitLoop(p, time.Duration(waitMs)*time.Millisecond)
			}) {
				return nil
			}
		}
	}

	if err := cn.writePacket(plan.resp); err != nil {
		cn.abortPlan(plan)
		cn.pullInflight.Store(false)
		return err
	}
	// 写出期间可能被接管或被 CleanSession 重置：旧连接不得再推进会话状态。
	if err := cn.stalePlan(plan); err != nil {
		cn.discardPlan(plan)
		cn.pullInflight.Store(false)
		return err
	}
	cn.applyPlan(plan)
	cn.pullInflight.Store(false)
	return nil
}

// pullRetryAfterMs 是拒绝并发 PULL 时回给客户端的建议重试间隔。
//
// 正常客户端（含 SDK）不会触发这条路径：它意味着上一条 PULL 还没收到响应。
// 这里给一个足够大的值，让裸协议客户端不会立刻重试造成忙循环。
const pullRetryAfterMs uint32 = 1000

// writePullRateLimited 拒绝并发的第二个 PULL。
func (cn *conn) writePullRateLimited(p *packet.PullPacket) error {
	pending, delivered, oldest := cn.client.mb.depth()

	resp := packet.NewPullOkPacket(p.RequestID)
	resp.Reason = packet.ReasonRateLimited
	resp.RetryAfterMs = pullRetryAfterMs
	resp.Pending = pending
	resp.Delivered = delivered
	resp.OldestUncommitted = oldest

	cn.core.stats.MessagesRejected.Add(1)
	return cn.writePacket(resp)
}

func (cn *conn) waitMillis(requested uint32) uint32 {
	wait := requested
	if limit := cn.core.options.MaxWaitMs; limit > 0 && wait > limit {
		wait = limit
	}
	return wait
}

func (cn *conn) waitLoop(p *packet.PullPacket, wait time.Duration) {
	timer := time.NewTimer(wait)
	defer timer.Stop()

	for {
		select {
		case <-cn.ctx.Done():
			return
		case <-timer.C:
			cn.writeEmptyPull(p)
			return
		case <-cn.wakeCh:
			if cn.closed.Load() || !cn.client.owns(cn) {
				return
			}
			plan := cn.buildPlan(p)
			if plan.resp == nil {
				return
			}
			if plan.empty {
				continue
			}
			// 构建期间可能发生接管：不要再用旧连接推进会话状态。
			if !cn.client.owns(cn) {
				cn.abortPlan(plan)
				return
			}
			if err := cn.writePacket(plan.resp); err != nil {
				cn.abortPlan(plan)
				return
			}
			if err := cn.stalePlan(plan); err != nil {
				cn.discardPlan(plan)
				return
			}
			cn.applyPlan(plan)
			return
		}
	}
}

func (cn *conn) writeEmptyPull(p *packet.PullPacket) {
	if !cn.client.owns(cn) {
		return
	}
	cl := cn.client
	pending, delivered, oldest := cl.mb.depth()

	resp := packet.NewPullOkPacket(p.RequestID)
	resp.Pending = pending
	resp.Delivered = delivered
	resp.OldestUncommitted = oldest
	_ = cn.writePacket(resp)
}

// ============================================================================
// COMMIT
// ============================================================================

func (cn *conn) handleCommit(p *packet.CommitPacket) error {
	if err := cn.checkOwner(); err != nil {
		return err
	}

	cl := cn.client
	committed, popped := cl.mb.commit(p.IDs)

	cn.pullMu.Lock()
	cn.served -= popped
	if cn.served < 0 {
		cn.served = 0
	}
	cn.noProgress = 0
	cn.pullMu.Unlock()

	pending, delivered, _ := cl.mb.depth()
	if delivered == 0 {
		cl.deliveredSince.Store(0)
	}

	resp := packet.NewCommitOkPacket(p.RequestID)
	resp.Committed = uint16(committed)
	resp.Pending = pending
	resp.Delivered = delivered

	if err := cn.writePacket(resp); err != nil {
		return err
	}

	// 确认后可能腾出了空间，唤醒自己的等待者
	cn.notify()
	return nil
}
