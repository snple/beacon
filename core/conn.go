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

	wakeCh       chan struct{}
	pullInflight atomic.Bool
	registered   atomic.Bool

	disconnect atomic.Pointer[packet.DisconnectPacket]

	// endReason 记录连接结束的原因码，用于 OnDisconnect 钩子与统计。
	endReason atomic.Uint32

	pullMu     sync.Mutex
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
				cn.terminate(pe.reason)
			case errors.Is(err, errSessionStale):
				cn.setEndReason(packet.ReasonSessionTakenOver)
				cn.terminate(packet.ReasonSessionTakenOver)
			case errors.Is(err, errNoProgress):
				cn.setEndReason(packet.ReasonCommitRequired)
				cn.terminate(packet.ReasonCommitRequired)
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
		cn.terminate(packet.ReasonPacketTooLarge)
		return
	}

	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		cn.setEndReason(packet.ReasonKeepAliveTimeout)
		// 尽力通知对端。
		_ = cn.writePacket(packet.NewDisconnectPacket(packet.ReasonKeepAliveTimeout))
		return
	}

	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return
	}

	// 只有解码/分帧错误才是协议错误。RST、TLS 异常关闭等 I/O 错误属于
	// 异常断开，不能当作 MalformedPacket 处理。
	if errors.Is(err, packet.ErrMalformedPacket) || errors.Is(err, packet.ErrInvalidPacketType) {
		cn.setEndReason(packet.ReasonMalformedPacket)
		cn.terminate(packet.ReasonMalformedPacket)
		return
	}

	cn.setEndReason(packet.ReasonUnspecifiedError)
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
func (cn *conn) terminate(reason packet.ReasonCode) {
	cn.setEndReason(reason)
	select {
	case <-cn.ctx.Done():
	default:
		_ = cn.writePacket(packet.NewDisconnectPacket(reason))
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
	resp        *packet.PullOkPacket
	pending     []taken // 从 pending 取出、写成功后应用
	redelivered []taken // 仅为重发而 retain、写成功后释放
	empty       bool
	epoch       uint64 // 构建计划时的会话 resetEpoch
	hadNew      bool
}

// buildPlan 构造一次拉取计划（不改变除 noProgress 外的会话状态）。
//
// 单次最多返回 MaxCount 条记录；可靠消息受会话 ReceiveMaximum 约束。
// 先投递新消息（窗口有空位时），没有新消息再重发 delivered 中未确认的消息。
// 放不进客户端接收上限的记录直接为该客户端丢弃，避免永久堵住队列。
func (cn *conn) buildPlan(p *packet.PullPacket) *pullPlan {
	cl := cn.client
	opts := cn.core.options

	now := nowMillis()
	plan := &pullPlan{resp: packet.NewPullOkPacket(p.RequestID), epoch: cl.resetEpoch.Load()}

	maxCount := int(p.MaxCount)
	if maxCount <= 0 {
		maxCount = 1
	}
	if limit := opts.MaxPullBatch; limit > 0 && maxCount > limit {
		maxCount = limit
	}
	if maxCount > packet.MaxPullBatch {
		maxCount = packet.MaxPullBatch
	}
	maxBytes := int64(p.MaxBytes)

	window := int(cl.receiveWindow())
	if window < 1 {
		window = 1
	}
	deliveredLen := cl.mb.deliveredLen()
	if deliveredLen == 0 {
		// 没有未确认消息时重置无进展计数（例如所有 delivered 都已过期）。
		cn.pullMu.Lock()
		cn.noProgress = 0
		cn.pullMu.Unlock()
	}
	free := window - deliveredLen
	if free < 0 {
		free = 0
	}

	var total int64

	// 1) 窗口有空位时投递新消息。
	for free > 0 && len(plan.pending) < maxCount {
		t, ok := cl.mb.takeDelivery(free)
		if !ok {
			break
		}
		if t.msg.Expired(now) {
			cn.core.releaseMessage(t.msg)
			cn.core.stats.MessagesExpired.Add(1)
			cl.expired.Add(1)
			continue
		}

		// 投递钩子：拒绝的条目直接丢弃（不进入 delivered）。
		if cn.core.options.Hooks.hasOnDeliver() && !cn.core.options.Hooks.callOnDeliver(&DeliverContext{
			ClientID:    cl.ID,
			Channel:     t.msg.Channel,
			Source:      t.msg.Source,
			Priority:    t.prio,
			ExpiresAt:   t.msg.ExpiresAt,
			PublishedAt: t.msg.PublishedAt,
			Payload:     t.msg.Payload,
		}) {
			cn.core.releaseMessage(t.msg)
			cn.core.stats.MessagesDropped.Add(1)
			cl.dropped.Add(1)
			continue
		}

		rec := t.msg.record(false)
		if !fitsRecordInPacket(cn.maxPacketSize, &rec) {
			cn.dropPendingForSize(t)
			continue
		}
		recSize := int64(packet.RecordSize(&rec))
		if maxBytes > 0 && total+recSize > maxBytes {
			cl.mb.restore(t)
			break
		}
		if !fitsBatch(cn.maxPacketSize, total+recSize) {
			if len(plan.pending) == 0 {
				cn.dropPendingForSize(t)
				continue
			}
			cl.mb.restore(t)
			break
		}

		plan.pending = append(plan.pending, t)
		plan.resp.AddRecord(rec)
		total += recSize
		if t.reliable {
			free--
		}
	}

	// 2) 没有新消息时重发未确认的可靠消息。
	if len(plan.pending) == 0 {
		if _, deliveredDepth, _ := cl.mb.depth(); deliveredDepth > 0 {
			cn.pullMu.Lock()
			cn.noProgress++
			hard := opts.NoProgressHard > 0 && cn.noProgress > opts.NoProgressHard
			soft := opts.NoProgressSoft > 0 && cn.noProgress > opts.NoProgressSoft
			cn.pullMu.Unlock()

			if hard {
				return &pullPlan{resp: nil, empty: true}
			}
			if soft {
				pendingDepth, _, _ := cl.mb.depth()
				resp := packet.NewPullOkPacket(p.RequestID)
				resp.Reason = packet.ReasonCommitRequired
				resp.Pending = pendingDepth
				cn.core.stats.CommitRequired.Add(1)
				return &pullPlan{resp: resp}
			}

			for _, t := range cl.mb.redeliver(maxCount) {
				if t.msg.Expired(now) {
					cn.dropDeliveredExpired(t)
					continue
				}
				rec := t.msg.record(true)
				if !fitsRecordInPacket(cn.maxPacketSize, &rec) {
					cn.dropDeliveredForSize(t)
					continue
				}
				recSize := int64(packet.RecordSize(&rec))
				if maxBytes > 0 && total+recSize > maxBytes {
					cl.mb.release(t)
					break
				}
				if !fitsBatch(cn.maxPacketSize, total+recSize) {
					if len(plan.redelivered) == 0 {
						cn.dropDeliveredForSize(t)
						continue
					}
					cl.mb.release(t)
					break
				}
				plan.redelivered = append(plan.redelivered, t)
				plan.resp.AddRecord(rec)
				total += recSize
			}
		}
	}

	pendingDepth, _, _ := cl.mb.depth()
	plan.resp.Pending = pendingDepth
	plan.hadNew = len(plan.pending) > 0
	plan.empty = len(plan.pending) == 0 && len(plan.redelivered) == 0
	return plan
}

// dropDeliveredExpired 丢弃 delivered 中已过期的记录，并释放临时引用。
func (cn *conn) dropDeliveredExpired(t taken) {
	cl := cn.client
	if cl.mb.discardDelivered(t.id) {
		cn.core.stats.MessagesExpired.Add(1)
		cl.expired.Add(1)
		if cl.mb.deliveredLen() == 0 {
			cl.deliveredSince.Store(0)
		}
	}
	cl.mb.release(t)
}

// responseBudget 返回本连接可发送的最大响应帧字节数（0 表示不限制）。
func (cn *conn) responseBudget() int {
	if cn.maxPacketSize == 0 {
		return 0
	}
	return int(cn.maxPacketSize)
}

// declareRespOverhead 是 DECLARE_RESP 的固定头 + 固定字段（不含 Results 与 Properties）：
// fixed header + RequestID + Flags + Accepted + Count + Pending + Limits。
const declareRespOverhead = 5 + 4 + 1 + 2 + 2 + 12 + 26

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

// pullResponseOverhead 是 PULL_OK 的固定头 + 固定字段（RequestID/Flags/Reason/Count/Pending）。
const pullResponseOverhead = 5 + 4 + 1 + 1 + 2 + 12

func fitsBatch(limit uint32, recordBytes int64) bool {
	if limit == 0 {
		return true
	}
	return int64(pullResponseOverhead)+recordBytes <= int64(limit)
}

// fitsRecordInPacket 报告一条记录编码为 PULL_OK 后是否不超过 limit。
// limit 为 0 表示不限制。
func fitsRecordInPacket(limit uint32, rec *packet.Record) bool {
	return fitsBatch(limit, int64(packet.RecordSize(rec)))
}

// dropPendingForSize 丢弃一条无法通过本连接投递的 pending 消息。
func (cn *conn) dropPendingForSize(t taken) {
	cl := cn.client
	cn.core.releaseMessage(t.msg)
	cn.core.stats.MessagesDropped.Add(1)
	cl.dropped.Add(1)
	cn.core.logger.Debug("dropped record exceeding client packet limit",
		zap.String("clientID", cl.ID),
		zap.String("channel", t.msg.Channel),
		zap.Uint32("maxPacketSize", cn.maxPacketSize))
}

// dropDeliveredForSize 丢弃 delivered 中无法通过当前连接投递的消息。
func (cn *conn) dropDeliveredForSize(t taken) {
	cl := cn.client
	cl.mb.discardDelivered(t.id)
	cl.mb.release(t)

	cn.pullMu.Lock()
	cn.noProgress = 0
	cn.pullMu.Unlock()

	if cl.mb.deliveredLen() == 0 {
		cl.deliveredSince.Store(0)
	}
	cn.core.stats.MessagesDropped.Add(1)
	cl.dropped.Add(1)
	cn.core.logger.Debug("dropped uncommitted record exceeding client packet limit",
		zap.String("clientID", cl.ID),
		zap.String("channel", t.msg.Channel),
		zap.Uint32("maxPacketSize", cn.maxPacketSize))
}

// applyPlan 在写成功后推进状态。
func (cn *conn) applyPlan(plan *pullPlan) {
	cl := cn.client

	if plan.hadNew {
		cn.pullMu.Lock()
		cn.noProgress = 0
		cn.pullMu.Unlock()
	}

	for _, t := range plan.pending {
		cl.mb.applySent(t)
	}
	for _, t := range plan.redelivered {
		cl.mb.release(t)
	}

	n := len(plan.pending) + len(plan.redelivered)
	if n > 0 {
		cn.core.stats.MessagesSent.Add(int64(n))
		cl.sent.Add(int64(n))
	}
	if len(plan.redelivered) > 0 {
		cn.core.stats.DuplicatesSent.Add(int64(len(plan.redelivered)))
	}
	if cl.deliveredSince.Load() == 0 && cl.mb.deliveredLen() > 0 {
		cl.deliveredSince.Store(time.Now().UnixMilli())
	}
}

// stalePlan 校验计划是否仍属于当前会话状态。
//
// 写出是异步的：期间连接可能被同 ClientID 的新连接接管（fencing），
// 或会话被 CleanSession 重置。任一情况都不允许旧连接再推进会话状态。
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
	for _, t := range plan.pending {
		cl.mb.restore(t)
	}
	for _, t := range plan.redelivered {
		cl.mb.release(t)
	}
}

// discardPlan 在连接已失去会话所有权时丢弃计划。
func (cn *conn) discardPlan(plan *pullPlan) {
	cl := cn.client
	for _, t := range plan.pending {
		cn.core.releaseMessage(t.msg)
	}
	for _, t := range plan.redelivered {
		cl.mb.release(t)
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

// writePullRateLimited 拒绝并发的第二个 PULL。
//
// 正常客户端（含 SDK）不会触发这条路径：它意味着上一条 PULL 还没收到响应。
// 客户端应在约 1s 后重试，避免立刻重试造成忙循环。
func (cn *conn) writePullRateLimited(p *packet.PullPacket) error {
	pending, _, _ := cn.client.mb.depth()

	resp := packet.NewPullOkPacket(p.RequestID)
	resp.Reason = packet.ReasonRateLimited
	resp.Pending = pending

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
	pending, _, _ := cl.mb.depth()

	resp := packet.NewPullOkPacket(p.RequestID)
	resp.Pending = pending
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
	committed, nacked, unknown := cl.mb.commit(p.Entries)

	cn.pullMu.Lock()
	cn.noProgress = 0
	cn.pullMu.Unlock()

	for _, n := range nacked {
		if cn.core.options.Hooks.hasNack() {
			cn.core.options.Hooks.callOnNack(&NackContext{
				ClientID: cl.ID,
				ID:       n.id,
				Channel:  n.msg.Channel,
				Source:   n.msg.Source,
				Priority: n.msg.Priority,
				Reliable: n.msg.Reliable,
				Reason:   n.reason,
			})
		}
		cn.core.stats.MessagesNacked.Add(1)
		cl.nacked.Add(1)
		cn.core.releaseMessage(n.msg)
	}

	if cl.mb.deliveredLen() == 0 {
		cl.deliveredSince.Store(0)
	}

	pending, _, _ := cl.mb.depth()
	resp := packet.NewCommitOkPacket(p.RequestID)
	resp.Committed = uint16(committed)
	resp.Nacked = uint16(len(nacked))
	resp.Unknown = uint16(unknown)
	resp.Pending = pending

	if err := cn.writePacket(resp); err != nil {
		return err
	}

	// 确认后可能腾出了空间，唤醒自己的等待者
	cn.notify()
	return nil
}
