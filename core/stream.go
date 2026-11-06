package core

import (
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snple/beacon/packet"
	streampacket "github.com/snple/beacon/stream/packet"
	"go.uber.org/zap"
)

const (
	streamTicketSize        = 32
	streamMinChallengeSize  = 16
	streamMaxChallengeSize  = 64
	streamRelayBufferSize   = 32 * 1024
	streamPendingSweepEvery = time.Second
	// streamAcceptGrace 是 BIND 先于 ACCEPT 到达时的最大等待时间。
	streamAcceptGrace = time.Second
)

// streamListenerEntry 是已注册的 stream 数据面监听器。
type streamListenerEntry struct {
	listener net.Listener
	address  string
	tls      bool
}

// streamPending 是控制面已协商、等待数据面绑定的流。
type streamPending struct {
	id         uint64
	topic      string
	metadata   []byte
	properties map[string]string

	sourceID    string
	targetID    string
	source      *conn
	target      *conn
	sourceReqID uint64

	ticket    []byte
	challenge []byte
	accepted  bool
	// sourceNotified 表示已向发起方写过成功或失败的 STREAM_OPEN_OK；
	// 之后的异常只能用 STREAM_ABORT 通知。
	sourceNotified bool
	acceptMeta     []byte
	acceptProps    map[string]string
	deadline       time.Time
	// 数据面参数在 OPEN 时固定，避免协商过程中监听器变化导致两端拿到不同地址。
	streamAddr  string
	streamTLS   bool
	bindTimeout time.Duration
	// 控制面两端的主机 IP，用于校验数据面连接来源（StreamAllowAnySourceIP）。
	sourceIP   string
	targetIP   string
	sourceData *streamDataConn
	targetData *streamDataConn
	pairing    bool
	done       bool

	// acceptCh 在目标接受时关闭，用于唤醒可能先到达的 BIND 连接。
	acceptCh chan struct{}
}

// wakeAcceptorLocked 唤醒等待接受结果的 BIND 协程（调用方必须持有 streamMu）。
func (p *streamPending) wakeAcceptorLocked() {
	if p.acceptCh != nil {
		close(p.acceptCh)
		p.acceptCh = nil
	}
}

// streamDataConn 在握手阶段串行化写入，避免多个 goroutine 同时写 BIND_OK。
type streamDataConn struct {
	net.Conn
	writeMu sync.Mutex
}

func (c *streamDataConn) writePacket(pkt streampacket.Packet) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return streampacket.WritePacket(c.Conn, pkt)
}

// CloseWrite 需要显式转发：嵌入的 net.Conn 接口不包含 CloseWrite 方法。
func (c *streamDataConn) CloseWrite() error {
	if cw, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return c.Conn.Close()
}

// streamSession 是已建立并正在转发的流。
type streamSession struct {
	id       uint64
	topic    string
	sourceID string
	targetID string
	source   *conn
	target   *conn

	sourceData *streamDataConn
	targetData *streamDataConn

	createdAt time.Time
	bytesAToB atomic.Int64
	bytesBToA atomic.Int64
	closed    atomic.Bool
}

// StreamEnabled 报告 stream 能力当前是否开启。
func (c *Core) StreamEnabled() bool { return c.streamEnabled.Load() }

// SetStreamEnabled 在运行期开启或关闭 stream 能力。
//
// 关闭只影响之后的新建流：已建立与已接受待绑定的流不会被强制中断。
func (c *Core) SetStreamEnabled(enabled bool) {
	old := c.streamEnabled.Swap(enabled)
	if old != enabled {
		c.logger.Info("stream capability changed", zap.Bool("enabled", enabled))
	}
}

// StreamInfo 是活跃流的公开视图。
type StreamInfo struct {
	StreamID      uint64
	Topic         string
	Source        string
	Target        string
	EstablishedAt time.Time
	BytesAToB     int64
	BytesBToA     int64
}

// Streams 返回当前活跃流（按 StreamID 升序）。
func (c *Core) Streams() []StreamInfo {
	c.streamMu.Lock()
	sessions := make([]*streamSession, 0, len(c.streams))
	for _, s := range c.streams {
		sessions = append(sessions, s)
	}
	c.streamMu.Unlock()

	out := make([]StreamInfo, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, StreamInfo{
			StreamID:      s.id,
			Topic:         s.topic,
			Source:        s.sourceID,
			Target:        s.targetID,
			EstablishedAt: s.createdAt,
			BytesAToB:     s.bytesAToB.Load(),
			BytesBToA:     s.bytesBToA.Load(),
		})
	}
	// 按 ID 排序，保证管理面输出稳定。
	sort.Slice(out, func(i, j int) bool { return out[i].StreamID < out[j].StreamID })
	return out
}

// ============================================================================
// 监听器
// ============================================================================

// ServeStreamTCP 在独立端口上启动 stream 数据面监听器。
func (c *Core) ServeStreamTCP(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()
	return c.serveStream(listener, false)
}

// ServeStreamTLS 在独立端口上启动 TLS 的 stream 数据面监听器。
func (c *Core) ServeStreamTLS(address string, cfg *tls.Config) error {
	if cfg == nil {
		return errors.New("tls config is required")
	}
	listener, err := tls.Listen("tcp", address, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()
	return c.serveStream(listener, true)
}

// ServeStream 在给定的普通监听器上提供 stream 数据面。
func (c *Core) ServeStream(listener net.Listener) error {
	return c.serveStream(listener, false)
}

// ServeStreamTLSListener 在给定的 TLS 监听器上提供 stream 数据面。
func (c *Core) ServeStreamTLSListener(listener net.Listener) error {
	return c.serveStream(listener, true)
}

func (c *Core) serveStream(listener net.Listener, tlsEnabled bool) error {
	if !c.running.Load() {
		return ErrCoreNotRunning
	}

	id := c.registerStreamListener(listener, tlsEnabled)
	defer c.unregisterStreamListener(id)

	scheme := "tcp"
	if tlsEnabled {
		scheme = "tls"
	}
	c.logger.Info("started stream listener",
		zap.String("address", listener.Addr().String()), zap.String("protocol", scheme))

	for c.running.Load() {
		nc, err := listener.Accept()
		if err != nil {
			if !c.running.Load() || errors.Is(err, net.ErrClosed) {
				return nil
			}
			c.logger.Error("stream accept error", zap.Error(err))
			select {
			case <-c.ctx.Done():
				return nil
			case <-time.After(100 * time.Millisecond):
			}
			continue
		}

		if !c.spawnTask(func() { c.handleStreamConn(nc) }) {
			_ = nc.Close()
			return nil
		}
	}
	return nil
}

func (c *Core) registerStreamListener(listener net.Listener, tlsEnabled bool) uint64 {
	c.streamListenersMu.Lock()
	defer c.streamListenersMu.Unlock()

	c.streamListenerSeq++
	id := c.streamListenerSeq
	if c.streamListeners == nil {
		c.streamListeners = make(map[uint64]streamListenerEntry)
	}
	c.streamListeners[id] = streamListenerEntry{
		listener: listener,
		address:  listener.Addr().String(),
		tls:      tlsEnabled,
	}
	return id
}

func (c *Core) unregisterStreamListener(id uint64) {
	c.streamListenersMu.Lock()
	delete(c.streamListeners, id)
	c.streamListenersMu.Unlock()
}

// streamAdvertise 返回告知客户端的地址与是否 TLS。
func (c *Core) streamAdvertise() (addr string, tlsEnabled bool, ok bool) {
	c.streamListenersMu.Lock()
	defer c.streamListenersMu.Unlock()

	if len(c.streamListeners) == 0 {
		return "", false, false
	}

	var bestID uint64
	for id := range c.streamListeners {
		if bestID == 0 || id < bestID {
			bestID = id
		}
	}
	entry := c.streamListeners[bestID]
	addr = entry.address
	if c.options.StreamAdvertiseAddr != "" {
		addr = c.options.StreamAdvertiseAddr
	}
	return addr, entry.tls, true
}

func (c *Core) closeStreamListeners() {
	c.streamListenersMu.Lock()
	listeners := make([]net.Listener, 0, len(c.streamListeners))
	for _, e := range c.streamListeners {
		listeners = append(listeners, e.listener)
	}
	c.streamListeners = make(map[uint64]streamListenerEntry)
	c.streamListenersMu.Unlock()

	for _, listener := range listeners {
		_ = listener.Close()
	}
}

func (c *Core) addStreamConn(nc net.Conn) bool {
	c.streamConnsMu.Lock()
	defer c.streamConnsMu.Unlock()
	if c.stopped.Load() {
		_ = nc.Close()
		return false
	}
	if c.streamConns == nil {
		c.streamConns = make(map[net.Conn]struct{})
	}
	c.streamConns[nc] = struct{}{}
	return true
}

func (c *Core) removeStreamConn(nc net.Conn) {
	c.streamConnsMu.Lock()
	delete(c.streamConns, nc)
	c.streamConnsMu.Unlock()
}

func (c *Core) closeStreamConns() {
	c.streamConnsMu.Lock()
	conns := make([]net.Conn, 0, len(c.streamConns))
	for nc := range c.streamConns {
		conns = append(conns, nc)
	}
	c.streamConns = make(map[net.Conn]struct{})
	c.streamConnsMu.Unlock()

	for _, nc := range conns {
		_ = nc.Close()
	}
}

func (c *Core) acquireStreamHandshakeSlot() (func(), bool) {
	if c.streamHandshakeSlots == nil {
		return func() {}, true
	}
	select {
	case c.streamHandshakeSlots <- struct{}{}:
		var once sync.Once
		return func() { once.Do(func() { <-c.streamHandshakeSlots }) }, true
	default:
		return nil, false
	}
}

// ============================================================================
// 数据面
// ============================================================================

func (c *Core) handleStreamConn(nc net.Conn) {
	if !c.addStreamConn(nc) {
		return
	}

	transferred := false
	defer func() {
		if !transferred {
			c.removeStreamConn(nc)
			_ = nc.Close()
		}
	}()

	release, ok := c.acquireStreamHandshakeSlot()
	if !ok {
		c.writeStreamHandshake(nc, &streampacket.BindOkPacket{
			ReasonCode: streampacket.ServerUnavailable,
			Message:    "too many concurrent stream handshakes",
		})
		return
	}
	defer release()

	_ = nc.SetReadDeadline(time.Now().Add(c.options.StreamHandshakeTimeout))
	pkt, err := streampacket.ReadPacket(nc)
	_ = nc.SetReadDeadline(time.Time{})
	if err != nil {
		c.logger.Debug("failed to read stream bind", zap.String("remote", safeAddr(nc)), zap.Error(err))
		return
	}

	bind, ok := pkt.(*streampacket.BindPacket)
	if !ok {
		c.writeStreamHandshake(nc, &streampacket.BindOkPacket{
			ReasonCode: streampacket.MalformedPacket,
			Message:    "first packet must be BIND",
		})
		return
	}
	if bind.Version != streampacket.Version {
		c.writeStreamHandshake(nc, &streampacket.BindOkPacket{
			StreamID:   bind.StreamID,
			Role:       bind.Role,
			ReasonCode: streampacket.ProtocolError,
			Message:    "unsupported stream protocol version",
		})
		return
	}

	dataConn := &streamDataConn{Conn: nc}
	pending, pair, reason, msg := c.bindStreamData(bind, dataConn)
	if reason != streampacket.Success {
		// 校验失败立即告知本端，不需要等待对端。
		c.writeStreamHandshake(nc, &streampacket.BindOkPacket{
			StreamID:   bind.StreamID,
			Role:       bind.Role,
			ReasonCode: reason,
			Message:    msg,
		})
		return
	}

	// 校验通过：连接所有权转移给 pending，先挂起等待对端 BIND。
	transferred = true
	if !pair {
		return
	}
	if !c.spawnTask(func() { c.pairStream(pending) }) {
		c.abortPending(pending, packet.ReasonServerUnavailable, "core is stopping")
	}
}

// writeStreamHandshake 写出数据面握手错误包，带写超时，避免对端不读时挂住协程。
func (c *Core) writeStreamHandshake(nc net.Conn, pkt streampacket.Packet) {
	if d := c.options.StreamHandshakeTimeout; d > 0 {
		_ = nc.SetWriteDeadline(time.Now().Add(d))
	}
	_ = (&streamDataConn{Conn: nc}).writePacket(pkt)
	_ = nc.SetWriteDeadline(time.Time{})
}

// bindStreamData 校验 BIND 并登记数据面连接。
// pair=true 表示两端都已绑定，调用方应启动配对。
// bindStreamData 校验 BIND 并登记数据面连接。
// pair=true 表示两端都已绑定，调用方应启动配对。
// bindStreamData 校验 BIND 并登记数据面连接。
// pair=true 表示两端都已绑定，调用方应启动配对。
func (c *Core) bindStreamData(bind *streampacket.BindPacket, nc *streamDataConn) (pending *streamPending, pair bool, reason streampacket.ReasonCode, msg string) {
	// 目标客户端的 ACCEPT 与数据面 BIND 走两条连接，BIND 可能先到；
	// 给 ACCEPT 一个很短的追赶窗口。等待前先校验票据，避免无关连接占用窗口。
	grace := time.NewTimer(streamAcceptGrace)
	defer grace.Stop()

	for {
		c.streamMu.Lock()
		p := c.streamPending[bind.StreamID]
		if p == nil || p.done {
			c.streamMu.Unlock()
			return nil, false, streampacket.StreamNotFound, "stream not found"
		}
		if time.Now().After(p.deadline) {
			c.streamMu.Unlock()
			return nil, false, streampacket.StreamNotFound, "bind deadline exceeded"
		}
		if !p.sourceHealthy() || !p.targetHealthy() {
			c.streamMu.Unlock()
			return nil, false, streampacket.NotAuthorized, "stream session is no longer active"
		}
		if !c.streamBindSourceAllowed(p, bind.Role, nc.RemoteAddr()) {
			c.streamMu.Unlock()
			return nil, false, streampacket.NotAuthorized, "stream source address mismatch"
		}
		if subtle.ConstantTimeCompare(bind.Ticket, p.ticket) != 1 {
			c.streamMu.Unlock()
			return nil, false, streampacket.NotAuthorized, "invalid ticket"
		}

		if !p.accepted {
			// 接受前只有接受方可以 BIND；且已经通过票据校验。
			if bind.Role != streampacket.RoleAcceptor {
				c.streamMu.Unlock()
				return nil, false, streampacket.NotAuthorized, "initiator bound before accept"
			}
			ch := p.acceptCh
			c.streamMu.Unlock()
			select {
			case <-ch:
				continue
			case <-grace.C:
				return nil, false, streampacket.BindFailed, "stream not accepted"
			case <-c.ctx.Done():
				return nil, false, streampacket.ServerUnavailable, "core is stopping"
			}
		}

		if subtle.ConstantTimeCompare(bind.Challenge, p.challenge) != 1 {
			c.streamMu.Unlock()
			return nil, false, streampacket.NotAuthorized, "invalid challenge"
		}

		switch bind.Role {
		case streampacket.RoleInitiator:
			if p.sourceData != nil {
				c.streamMu.Unlock()
				return nil, false, streampacket.BindFailed, "initiator already bound"
			}
			p.sourceData = nc
		case streampacket.RoleAcceptor:
			if p.targetData != nil {
				c.streamMu.Unlock()
				return nil, false, streampacket.BindFailed, "acceptor already bound"
			}
			p.targetData = nc
		default:
			c.streamMu.Unlock()
			return nil, false, streampacket.MalformedPacket, "invalid role"
		}

		ready := p.sourceData != nil && p.targetData != nil && !p.pairing
		if ready {
			p.pairing = true
		}
		c.streamMu.Unlock()
		return p, ready, streampacket.Success, ""
	}
}

func (c *Core) pairStream(p *streamPending) {
	success := false
	defer func() {
		if !success {
			c.abortPending(p, packet.ReasonStreamBindFailed, "stream pairing failed")
		}
	}()

	if p.targetData == nil || p.sourceData == nil {
		return
	}

	// 两端都已完成 BIND 校验：各回一个 BIND_OK 表示流已建立，随后进入裸字节转发。
	_ = p.sourceData.SetWriteDeadline(time.Now().Add(p.bindTimeout))
	if err := p.sourceData.writePacket(&streampacket.BindOkPacket{
		StreamID:   p.id,
		Role:       streampacket.RoleInitiator,
		ReasonCode: streampacket.Success,
	}); err != nil {
		return
	}
	_ = p.sourceData.SetWriteDeadline(time.Time{})

	_ = p.targetData.SetWriteDeadline(time.Now().Add(p.bindTimeout))
	if err := p.targetData.writePacket(&streampacket.BindOkPacket{
		StreamID:   p.id,
		Role:       streampacket.RoleAcceptor,
		ReasonCode: streampacket.Success,
	}); err != nil {
		return
	}
	_ = p.targetData.SetWriteDeadline(time.Time{})

	c.streamMu.Lock()
	if p.done {
		c.streamMu.Unlock()
		return
	}
	p.done = true
	// 计数从 pending 转移到 active，保持不变。
	delete(c.streamPending, p.id)
	session := &streamSession{
		id:         p.id,
		topic:      p.topic,
		sourceID:   p.sourceID,
		targetID:   p.targetID,
		source:     p.source,
		target:     p.target,
		sourceData: p.sourceData,
		targetData: p.targetData,
		createdAt:  time.Now(),
	}
	c.streams[p.id] = session
	c.streamMu.Unlock()

	success = true
	c.stats.StreamsActive.Add(1)
	c.stats.StreamsTotal.Add(1)
	c.logger.Info("stream established",
		zap.Uint64("streamID", p.id),
		zap.String("source", p.sourceID),
		zap.String("target", p.targetID),
		zap.String("topic", p.topic))

	if !c.spawnTask(func() { c.relayStream(session) }) {
		c.finishStream(session)
	}
}

func (c *Core) relayStream(s *streamSession) {
	defer c.finishStream(s)

	var wg sync.WaitGroup
	wg.Add(2)

	closeBoth := func() {
		_ = s.sourceData.Close()
		_ = s.targetData.Close()
	}

	go func() {
		defer wg.Done()
		if hard := c.relayData(s.sourceData, s.targetData, &s.bytesAToB, &c.stats.StreamBytesAToB); hard {
			closeBoth()
		}
	}()
	go func() {
		defer wg.Done()
		if hard := c.relayData(s.targetData, s.sourceData, &s.bytesBToA, &c.stats.StreamBytesBToA); hard {
			closeBoth()
		}
	}()

	wg.Wait()
}

// relayData 单向转发；返回 true 表示发生了需要立刻关闭两端的错误。
func (c *Core) relayData(src, dst net.Conn, streamCounter, totalCounter *atomic.Int64) (hard bool) {
	buf := make([]byte, streamRelayBufferSize)
	idle := c.options.StreamIdleTimeout
	writeTimeout := c.options.StreamWriteTimeout

	for {
		if idle > 0 {
			_ = src.SetReadDeadline(time.Now().Add(idle))
		}
		n, err := src.Read(buf)
		if n > 0 {
			if writeTimeout > 0 {
				_ = dst.SetWriteDeadline(time.Now().Add(writeTimeout))
			}
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return true
			}
			streamCounter.Add(int64(n))
			totalCounter.Add(int64(n))
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			if cw, ok := dst.(interface{ CloseWrite() error }); ok {
				_ = cw.CloseWrite()
			}
			return false
		}
		return true
	}
}

func (c *Core) finishStream(s *streamSession) {
	if !s.closed.CompareAndSwap(false, true) {
		return
	}

	c.streamMu.Lock()
	if cur, ok := c.streams[s.id]; ok && cur == s {
		delete(c.streams, s.id)
		c.decClientStreamLocked(s.sourceID)
		c.decClientStreamLocked(s.targetID)
	}
	c.streamMu.Unlock()

	c.stats.StreamsActive.Add(-1)
	_ = s.sourceData.Close()
	_ = s.targetData.Close()
	c.removeStreamConn(s.sourceData.Conn)
	c.removeStreamConn(s.targetData.Conn)

	c.logger.Info("stream closed",
		zap.Uint64("streamID", s.id),
		zap.Int64("aToB", s.bytesAToB.Load()),
		zap.Int64("bToA", s.bytesBToA.Load()))
}

// abortPending 取消一个待建流并通知相关客户端。
func (c *Core) abortPending(p *streamPending, reason packet.ReasonCode, msg string) {
	if p == nil {
		return
	}
	c.streamMu.Lock()
	if p.done {
		c.streamMu.Unlock()
		return
	}
	p.done = true
	p.wakeAcceptorLocked()
	c.dropPendingLocked(p)
	c.streamMu.Unlock()

	c.discardPending(p, reason, msg)
}

// discardPending 关闭数据面连接并通知双方（p.done 已置位）。
func (c *Core) discardPending(p *streamPending, reason packet.ReasonCode, msg string) {
	if p.sourceData != nil {
		_ = p.sourceData.Close()
		c.removeStreamConn(p.sourceData.Conn)
	}
	if p.targetData != nil {
		_ = p.targetData.Close()
		c.removeStreamConn(p.targetData.Conn)
	}

	// 通知走异步：这些写可能发生在另一个客户端的读循环里，
	// 对端不读时不能把当前连接的读循环阻塞到写超时。
	if !p.sourceNotified {
		// 发起方仍在等待 STREAM_OPEN_OK：必须用该响应返回失败。
		src, reqID := p.source, p.sourceReqID
		c.spawnTask(func() { _ = c.sendStreamOpenError(src, reqID, reason, msg) })
	} else {
		src, id := p.source, p.id
		c.spawnTask(func() { c.sendStreamAbort(src, id, reason, msg) })
	}
	if p.accepted {
		dst, id := p.target, p.id
		c.spawnTask(func() { c.sendStreamAbort(dst, id, reason, msg) })
	}
}

func (c *Core) sendStreamOpenError(cn *conn, requestID uint64, reason packet.ReasonCode, msg string) error {
	if cn == nil || cn.closed.Load() {
		return nil
	}
	return cn.writePacket(&packet.StreamOpenOkPacket{
		RequestID:  requestID,
		ReasonCode: reason,
		Message:    msg,
	})
}

func (c *Core) sendStreamAbort(cn *conn, streamID uint64, reason packet.ReasonCode, msg string) {
	if cn == nil || cn.closed.Load() {
		return
	}
	_ = cn.writePacket(&packet.StreamAbortPacket{
		StreamID:   streamID,
		ReasonCode: reason,
		Message:    msg,
	})
}

// closeStreamsForConn 在一条主连接结束时清理与之关联的全部流。
func (c *Core) closeStreamsForConn(cn *conn) {
	if cn == nil {
		return
	}

	c.streamMu.Lock()
	var pending []*streamPending
	for _, p := range c.streamPending {
		if p.done {
			continue
		}
		if p.source == cn || p.target == cn {
			p.done = true
			p.wakeAcceptorLocked()
			c.dropPendingLocked(p)
			pending = append(pending, p)
		}
	}
	var sessions []*streamSession
	for id, s := range c.streams {
		if s.source == cn || s.target == cn {
			delete(c.streams, id)
			sessions = append(sessions, s)
		}
	}
	c.streamMu.Unlock()

	for _, p := range pending {
		c.discardPending(p, packet.ReasonNormalDisconnect, "peer session closed")
	}
	for _, s := range sessions {
		c.finishStream(s)
	}
}

func (c *Core) streamCleanupLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(streamPendingSweepEvery)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case now := <-ticker.C:
			c.expirePendingStreams(now)
		}
	}
}

func (c *Core) expirePendingStreams(now time.Time) {
	c.streamMu.Lock()
	var expired []*streamPending
	for _, p := range c.streamPending {
		if p.done || now.Before(p.deadline) {
			continue
		}
		p.done = true
		p.wakeAcceptorLocked()
		c.dropPendingLocked(p)
		expired = append(expired, p)
	}
	c.streamMu.Unlock()

	for _, p := range expired {
		c.discardPending(p, packet.ReasonStreamTimeout, "stream negotiation timeout")
	}
}

// ============================================================================
// 控制面
// ============================================================================

func (cn *conn) handleStreamOpen(p *packet.StreamOpenPacket) error {
	if err := cn.checkOwner(); err != nil {
		return err
	}
	c := cn.core

	if !cn.streamCapable.Load() {
		return c.sendStreamOpenError(cn, p.RequestID, packet.ReasonStreamDisabled,
			"client did not declare stream support")
	}
	if !c.streamEnabled.Load() {
		return c.sendStreamOpenError(cn, p.RequestID, packet.ReasonStreamDisabled,
			"stream capability is disabled")
	}
	addr, tlsEnabled, ok := c.streamAdvertise()
	if !ok {
		return c.sendStreamOpenError(cn, p.RequestID, packet.ReasonStreamDisabled,
			"stream listener is not configured")
	}
	if p.RequestID == 0 {
		return newProtocolError(packet.ReasonProtocolError, "stream open without request id")
	}
	if p.Target == "" || p.Target == cn.client.ID {
		return c.sendStreamOpenError(cn, p.RequestID, packet.ReasonStreamTargetUnavailable,
			"invalid stream target")
	}

	if err := c.options.Hooks.callOnStreamOpen(&StreamOpenContext{
		ClientID:   cn.client.ID,
		Identity:   cn.client.getIdentity(),
		Target:     p.Target,
		Topic:      p.Topic,
		Metadata:   p.Metadata,
		Properties: p.Properties,
	}); err != nil {
		c.stats.StreamsRejected.Add(1)
		return c.sendStreamOpenError(cn, p.RequestID, packet.ReasonStreamUnauthorized, err.Error())
	}

	c.clientsMu.RLock()
	target := c.clients[p.Target]
	c.clientsMu.RUnlock()
	if target == nil {
		c.stats.StreamsRejected.Add(1)
		return c.sendStreamOpenError(cn, p.RequestID, packet.ReasonStreamTargetUnavailable,
			"target client not found")
	}
	tcn := target.currentConn()
	if tcn == nil || tcn.closed.Load() || !target.owns(tcn) {
		c.stats.StreamsRejected.Add(1)
		return c.sendStreamOpenError(cn, p.RequestID, packet.ReasonStreamTargetUnavailable,
			"target client is offline")
	}
	if !tcn.streamCapable.Load() {
		c.stats.StreamsRejected.Add(1)
		return c.sendStreamOpenError(cn, p.RequestID, packet.ReasonStreamTargetUnavailable,
			"target client does not support stream")
	}

	ticket := make([]byte, streamTicketSize)
	if _, err := rand.Read(ticket); err != nil {
		c.stats.StreamsRejected.Add(1)
		return c.sendStreamOpenError(cn, p.RequestID, packet.ReasonUnspecifiedError,
			"failed to generate stream ticket")
	}

	streamID := c.streamIDGen.Add(1)
	pending := &streamPending{
		id:          streamID,
		topic:       p.Topic,
		metadata:    cloneBytes(p.Metadata),
		properties:  cloneStringMap(p.Properties),
		sourceID:    cn.client.ID,
		targetID:    p.Target,
		source:      cn,
		target:      tcn,
		sourceReqID: p.RequestID,
		ticket:      ticket,
		deadline:    time.Now().Add(c.options.StreamAcceptTimeout),
		streamAddr:  addr,
		streamTLS:   tlsEnabled,
		bindTimeout: c.options.StreamBindTimeout,
		sourceIP:    controlHost(cn.remoteAddr),
		targetIP:    controlHost(tcn.remoteAddr),
		acceptCh:    make(chan struct{}),
	}

	if !c.registerPending(pending) {
		c.stats.StreamsRejected.Add(1)
		return c.sendStreamOpenError(cn, p.RequestID, packet.ReasonStreamLimitExceeded,
			"stream limit exceeded")
	}

	req := &packet.StreamRequestPacket{
		StreamID:      streamID,
		Source:        cn.client.ID,
		Topic:         p.Topic,
		Metadata:      pending.metadata,
		Properties:    pending.properties,
		Ticket:        pending.ticket,
		StreamAddress: pending.streamAddr,
		TLS:           pending.streamTLS,
		BindTimeoutMs: uint32(pending.bindTimeout / time.Millisecond),
	}
	// 转发请求可能在目标不读时阻塞，放到独立任务里，避免拖住发起方的读循环。
	if !c.spawnTask(func() {
		c.streamMu.Lock()
		alive := !pending.done && c.streamPending[pending.id] == pending
		c.streamMu.Unlock()
		if !alive || !pending.targetHealthy() {
			c.abortPending(pending, packet.ReasonStreamTargetUnavailable, "target session changed")
			return
		}
		if err := tcn.writePacket(req); err != nil {
			c.abortPending(pending, packet.ReasonStreamBindFailed, "failed to forward stream request")
		}
	}) {
		c.abortPending(pending, packet.ReasonServerUnavailable, "core is stopping")
		return nil
	}

	c.logger.Debug("stream requested",
		zap.Uint64("streamID", streamID),
		zap.String("source", cn.client.ID),
		zap.String("target", p.Target),
		zap.String("topic", p.Topic),
		zap.String("address", addr),
		zap.Bool("tls", tlsEnabled))
	return nil
}

func (cn *conn) handleStreamAccept(p *packet.StreamAcceptPacket) error {
	if err := cn.checkOwner(); err != nil {
		return err
	}
	c := cn.core

	c.streamMu.Lock()
	pending := c.streamPending[p.StreamID]
	if pending == nil || pending.done || pending.target != cn {
		// 请求已被移除/过期：接受方稍后的 BIND 会得到 StreamNotFound。
		c.streamMu.Unlock()
		return nil
	}
	if pending.accepted {
		c.streamMu.Unlock()
		return nil
	}

	if !p.Accepted {
		pending.done = true
		pending.wakeAcceptorLocked()
		c.dropPendingLocked(pending)
		c.streamMu.Unlock()

		c.stats.StreamsRejected.Add(1)
		c.discardPending(pending, packet.ReasonStreamRejected, "target rejected stream")
		return nil
	}

	if len(p.Challenge) < streamMinChallengeSize || len(p.Challenge) > streamMaxChallengeSize {
		pending.done = true
		pending.wakeAcceptorLocked()
		c.dropPendingLocked(pending)
		c.streamMu.Unlock()

		c.stats.StreamsRejected.Add(1)
		c.discardPending(pending, packet.ReasonStreamBindFailed, "invalid stream challenge")
		return nil
	}

	pending.challenge = cloneBytes(p.Challenge)
	pending.acceptMeta = cloneBytes(p.Metadata)
	pending.acceptProps = cloneStringMap(p.Properties)
	pending.accepted = true
	pending.wakeAcceptorLocked()
	pending.deadline = time.Now().Add(pending.bindTimeout)
	c.streamMu.Unlock()

	bindTimeoutMs := uint32(pending.bindTimeout / time.Millisecond)

	// 等待期间流可能已被取消（对端断开/超时），此时不能再告诉发起方成功。
	c.streamMu.Lock()
	stillPending := !pending.done && c.streamPending[pending.id] == pending
	c.streamMu.Unlock()
	if !stillPending {
		return nil
	}

	// OPEN_OK 发给发起方：同样是跨客户端写，放到独立任务里避免阻塞接受方的读循环。
	notify := &packet.StreamOpenOkPacket{
		RequestID:     pending.sourceReqID,
		ReasonCode:    packet.ReasonSuccess,
		StreamID:      p.StreamID,
		StreamAddress: pending.streamAddr,
		TLS:           pending.streamTLS,
		Ticket:        pending.ticket,
		Challenge:     pending.challenge,
		Metadata:      pending.acceptMeta,
		Properties:    pending.acceptProps,
		BindTimeoutMs: bindTimeoutMs,
	}
	source := pending.source
	if !c.spawnTask(func() {
		if err := source.writePacket(notify); err != nil {
			c.abortPending(pending, packet.ReasonStreamBindFailed, "failed to notify stream initiator")
			return
		}
		// 成功通知已写出；此后的异常一律用 STREAM_ABORT 通知发起方。
		c.streamMu.Lock()
		if !pending.done && c.streamPending[pending.id] == pending {
			pending.sourceNotified = true
		}
		c.streamMu.Unlock()
	}) {
		c.abortPending(pending, packet.ReasonServerUnavailable, "core is stopping")
	}

	return nil
}

// ============================================================================
// 辅助
// ============================================================================

func (p *streamPending) sourceHealthy() bool {
	return p.source != nil && p.source.client != nil &&
		!p.source.closed.Load() && p.source.client.owns(p.source)
}

func (p *streamPending) targetHealthy() bool {
	return p.target != nil && p.target.client != nil &&
		!p.target.closed.Load() && p.target.client.owns(p.target)
}

func (c *Core) registerPending(p *streamPending) bool {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()

	if max := c.options.StreamMaxPending; max > 0 && len(c.streamPending) >= max {
		return false
	}
	if max := c.options.StreamMaxActive; max > 0 && len(c.streams) >= max {
		return false
	}
	if max := c.options.StreamMaxPerClient; max > 0 {
		if c.streamPerClient[p.sourceID] >= max || c.streamPerClient[p.targetID] >= max {
			return false
		}
	}

	if c.streamPending == nil {
		c.streamPending = make(map[uint64]*streamPending)
	}
	if c.streamPerClient == nil {
		c.streamPerClient = make(map[string]int)
	}
	c.streamPending[p.id] = p
	c.streamPerClient[p.sourceID]++
	c.streamPerClient[p.targetID]++
	return true
}

// dropPendingLocked 从 pending 表移除并释放双方的流计数（调用方必须持有 streamMu）。
func (c *Core) dropPendingLocked(p *streamPending) {
	if cur, ok := c.streamPending[p.id]; !ok || cur != p {
		return
	}
	delete(c.streamPending, p.id)
	c.decClientStreamLocked(p.sourceID)
	c.decClientStreamLocked(p.targetID)
}

func (c *Core) decClientStreamLocked(clientID string) {
	n := c.streamPerClient[clientID]
	if n <= 1 {
		delete(c.streamPerClient, clientID)
		return
	}
	c.streamPerClient[clientID] = n - 1
}

// controlHost 从 host:port 中取出主机部分。
func controlHost(addr string) string {
	if addr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// streamBindSourceAllowed 校验数据面连接的源 IP 是否与控制面角色一致。
func (c *Core) streamBindSourceAllowed(p *streamPending, role streampacket.Role, addr net.Addr) bool {
	if c.options.StreamAllowAnySourceIP || addr == nil {
		return true
	}
	want := p.sourceIP
	if role == streampacket.RoleAcceptor {
		want = p.targetIP
	}
	if want == "" {
		return true
	}
	got := controlHost(addr.String())
	wantIP, gotIP := net.ParseIP(want), net.ParseIP(got)
	if wantIP != nil && gotIP != nil {
		return wantIP.Equal(gotIP)
	}
	return want == got
}

func cloneBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

func cloneStringMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
