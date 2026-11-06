package client

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/snple/beacon/packet"
	"github.com/snple/beacon/stream"
	streampacket "github.com/snple/beacon/stream/packet"
	"go.uber.org/zap"
)

// stream 相关错误。
var (
	// ErrStreamClientNotCapable 客户端没有启用 stream 能力。
	ErrStreamClientNotCapable = errors.New("client stream capability is not enabled")
	// ErrStreamNotSupported 服务端未开启 stream 能力。
	ErrStreamNotSupported = errors.New("stream not supported by server")
	// ErrStreamRejected 目标客户端或其他环节拒绝了建流。
	ErrStreamRejected = errors.New("stream rejected")
	// ErrStreamTimeout 建流协商超时。
	ErrStreamTimeout = errors.New("stream negotiation timeout")
	// ErrStreamAborted 流在建立过程中被服务端中止。
	ErrStreamAborted = errors.New("stream aborted")
	// ErrStreamNotFound 待建流不存在或已过期。
	ErrStreamNotFound = errors.New("stream not found")
	// ErrStreamTargetUnavailable 目标不在线或未声明 stream 能力。
	ErrStreamTargetUnavailable = errors.New("stream target unavailable")
	// ErrStreamAlreadyHandled 请求已被 Accept/Reject 处理过，不能再用另一种方式处理。
	ErrStreamAlreadyHandled = errors.New("stream request already handled")
)

const defaultStreamChallengeSize = 32

// streamWait 跟踪某个待建流的数据面连接，服务端 ABORT 时可立即中断等待。
type streamWait struct {
	abort chan struct{}

	mu      sync.Mutex
	conn    net.Conn
	aborted bool
}

func newStreamWait() *streamWait {
	return &streamWait{abort: make(chan struct{})}
}

func (w *streamWait) setConn(conn net.Conn) {
	w.mu.Lock()
	w.conn = conn
	aborted := w.aborted
	w.mu.Unlock()
	if aborted {
		_ = conn.Close()
	}
}

func (w *streamWait) abortNow() {
	w.mu.Lock()
	if w.aborted {
		w.mu.Unlock()
		return
	}
	w.aborted = true
	conn := w.conn
	w.mu.Unlock()

	close(w.abort)
	if conn != nil {
		_ = conn.Close()
	}
}

func (w *streamWait) isAborted() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.aborted
}

// StreamSupported 报告服务端是否实现了 stream 控制包。
//
// 旧服务端不会声明支持；此时 OpenStream 直接返回 ErrStreamNotSupported，
// 避免发送对端不认识的包导致连接被断开。
func (c *Client) StreamSupported() bool {
	if !c.opts.StreamCapable {
		return false
	}
	cn := c.currentConn()
	return cn != nil && !cn.closed.Load() && cn.connack != nil && cn.connack.StreamSupported()
}

// StreamEnabled 报告 CONNACK 时服务端是否已启用 stream。
//
// 服务端可以运行期开关，因此该值可能过期；OpenStream 以服务端返回结果为准。
func (c *Client) StreamEnabled() bool {
	if !c.opts.StreamCapable {
		return false
	}
	cn := c.currentConn()
	return cn != nil && !cn.closed.Load() && cn.connack != nil && cn.connack.StreamEnabled()
}

// OnStreamRequest 设置入站建流请求的回调。
//
// 回调在独立 goroutine 中执行，可以安全地阻塞在 Accept/Reject 上。
// 传 nil 表示改用 AcceptStream 从队列中取请求。
func (c *Client) OnStreamRequest(fn func(*StreamRequest)) {
	c.streamMu.Lock()
	c.streamHandler = fn
	c.streamMu.Unlock()
}

// AcceptStream 等待下一个建流请求并自动接受，返回已建立的流。
//
// 如果设置了 OnStreamRequest 回调，请求会交给回调处理，本方法不会收到请求。
func (c *Client) AcceptStream(ctx context.Context) (*stream.Stream, error) {
	select {
	case req := <-c.streamQueue:
		return req.Accept(ctx, nil, nil)
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, ErrClientClosed
	}
}

// OpenStream 向 target 发起一条流，阻塞直到对端接受并完成数据面握手。
func (c *Client) OpenStream(
	ctx context.Context,
	target string,
	topic string,
	metadata []byte,
	properties map[string]string,
) (*stream.Stream, error) {
	cn, err := c.streamConn()
	if err != nil {
		return nil, err
	}
	if target == "" {
		return nil, errors.New("stream target is required")
	}

	reqID := c.nextRequestID()
	req := &packet.StreamOpenPacket{
		RequestID:  reqID,
		Target:     target,
		Topic:      topic,
		Metadata:   metadata,
		Properties: properties,
	}

	resp, err := c.roundTripOn(ctx, cn, req, reqID, c.opts.StreamOpenTimeout)
	if err != nil {
		return nil, err
	}
	ok, isOk := resp.(*packet.StreamOpenOkPacket)
	if !isOk {
		return nil, fmt.Errorf("expected STREAM_OPEN_OK, got %s", resp.Type())
	}
	if ok.ReasonCode != packet.ReasonSuccess {
		return nil, streamReasonError(ok.ReasonCode, ok.Message)
	}
	if ok.StreamID == 0 || len(ok.Ticket) == 0 || len(ok.Challenge) == 0 {
		return nil, ErrStreamAborted
	}

	wait := c.registerStreamWait(ok.StreamID)
	defer c.unregisterStreamWait(ok.StreamID)

	conn, err := c.dialStream(ctx, wait, ok.StreamAddress, ok.TLS)
	if err != nil {
		return nil, err
	}
	wait.setConn(conn)
	if wait.isAborted() {
		_ = conn.Close()
		return nil, ErrStreamAborted
	}

	bindTimeout := streamBindTimeout(ok.BindTimeoutMs)
	fail := func(err error) (*stream.Stream, error) {
		_ = conn.Close()
		if wait.isAborted() {
			return nil, ErrStreamAborted
		}
		return nil, err
	}

	if err := streampacket.WritePacket(conn, &streampacket.BindPacket{
		Version:   streampacket.Version,
		StreamID:  ok.StreamID,
		Role:      streampacket.RoleInitiator,
		Ticket:    ok.Ticket,
		Challenge: ok.Challenge,
	}); err != nil {
		return fail(NewConnectionError("stream bind", err))
	}

	pkt, err := readStreamPacket(conn, bindTimeout)
	if err != nil {
		return fail(err)
	}
	bindOk, isBindOk := pkt.(*streampacket.BindOkPacket)
	if !isBindOk {
		return fail(fmt.Errorf("expected BIND_OK, got %s", pkt.Type()))
	}
	if bindOk.ReasonCode != streampacket.Success {
		_ = conn.Close()
		return nil, streamBindError(bindOk.ReasonCode, bindOk.Message)
	}

	_ = conn.SetDeadline(time.Time{})
	return stream.New(ok.StreamID, topic, c.ClientID(), target, ok.Metadata, ok.Properties, conn), nil
}

// StreamRequest 是入站建流请求。
type StreamRequest struct {
	client *Client
	conn   *conn

	StreamID   uint64
	Source     string
	Topic      string
	Metadata   []byte
	Properties map[string]string

	// 数据面地址与票据由服务端随 STREAM_REQUEST 下发，接受后可直接连接。
	StreamAddress string
	TLS           bool
	BindTimeoutMs uint32
	ticketBytes   []byte

	once     sync.Once
	accepted bool
	rejected bool
	result   *stream.Stream
	err      error
}

// Accept 接受建流请求，返回完成数据面握手后的流。
//
// 同一个请求只处理一次；重复调用返回首次结果。
func (r *StreamRequest) Accept(
	ctx context.Context,
	metadata []byte,
	properties map[string]string,
) (*stream.Stream, error) {
	first := false
	r.once.Do(func() {
		first = true
		r.accepted = true
		r.result, r.err = r.accept(ctx, metadata, properties)
	})
	if !first {
		if r.accepted {
			// 重复调用 Accept：返回首次结果。
			return r.result, r.err
		}
		return nil, ErrStreamAlreadyHandled
	}
	if r.result == nil && r.err == nil {
		// 状态异常，不能返回一个“成功但无流”的结果。
		return nil, ErrStreamAborted
	}
	return r.result, r.err
}

// Reject 拒绝建流请求。
func (r *StreamRequest) Reject(reason packet.ReasonCode, message string) error {
	first := false
	r.once.Do(func() {
		first = true
		r.rejected = true
		r.err = r.reject(reason, message)
	})
	if !first {
		if r.rejected {
			// 重复调用 Reject：幂等返回首次结果。
			return r.err
		}
		return ErrStreamAlreadyHandled
	}
	return r.err
}

func (r *StreamRequest) accept(
	ctx context.Context,
	metadata []byte,
	properties map[string]string,
) (*stream.Stream, error) {
	c := r.client
	cn := r.conn
	if cn == nil || cn.closed.Load() {
		return nil, ErrNotConnected
	}

	// 提前注册：ACCEPT 与 BIND 期间收到的 ABORT 都能立即生效。
	wait := c.registerStreamWait(r.StreamID)
	defer c.unregisterStreamWait(r.StreamID)

	challenge := make([]byte, defaultStreamChallengeSize)
	if _, err := rand.Read(challenge); err != nil {
		return nil, fmt.Errorf("failed to generate stream challenge: %w", err)
	}

	if err := cn.write(&packet.StreamAcceptPacket{
		StreamID:   r.StreamID,
		Accepted:   true,
		ReasonCode: packet.ReasonSuccess,
		Challenge:  challenge,
		Metadata:   metadata,
		Properties: properties,
	}); err != nil {
		return nil, NewConnectionError("stream accept", err)
	}

	// 接受结果直接由数据面 BIND 表达：服务端在 BIND 与 ACCEPT 跨连接乱序时
	// 会短暂等待 ACCEPT 到达，因此这里无需额外的控制面确认。
	conn, err := c.dialStream(ctx, wait, r.StreamAddress, r.TLS)
	if err != nil {
		return nil, err
	}
	wait.setConn(conn)
	if wait.isAborted() {
		_ = conn.Close()
		return nil, ErrStreamAborted
	}

	bindTimeout := streamBindTimeout(r.BindTimeoutMs)
	fail := func(err error) (*stream.Stream, error) {
		_ = conn.Close()
		if wait.isAborted() {
			return nil, ErrStreamAborted
		}
		return nil, err
	}

	if err := streampacket.WritePacket(conn, &streampacket.BindPacket{
		Version:   streampacket.Version,
		StreamID:  r.StreamID,
		Role:      streampacket.RoleAcceptor,
		Ticket:    r.ticketBytes,
		Challenge: challenge,
	}); err != nil {
		return fail(NewConnectionError("stream bind", err))
	}

	pkt, err := readStreamPacket(conn, bindTimeout)
	if err != nil {
		return fail(err)
	}
	bindOk, isBindOk := pkt.(*streampacket.BindOkPacket)
	if !isBindOk {
		return fail(fmt.Errorf("expected BIND_OK, got %s", pkt.Type()))
	}
	if bindOk.ReasonCode != streampacket.Success {
		_ = conn.Close()
		return nil, streamBindError(bindOk.ReasonCode, bindOk.Message)
	}

	_ = conn.SetDeadline(time.Time{})
	return stream.New(r.StreamID, r.Topic, c.ClientID(), r.Source, r.Metadata, r.Properties, conn), nil
}

func (r *StreamRequest) reject(reason packet.ReasonCode, message string) error {
	cn := r.conn
	if cn == nil || cn.closed.Load() {
		return ErrNotConnected
	}
	if reason == packet.ReasonSuccess {
		reason = packet.ReasonStreamRejected
	}

	if err := cn.write(&packet.StreamAcceptPacket{
		StreamID:   r.StreamID,
		Accepted:   false,
		ReasonCode: reason,
		Message:    message,
	}); err != nil {
		return NewConnectionError("stream reject", err)
	}
	return nil
}

// dispatchStreamRequest 由 readLoop 调用；不能阻塞读循环。
func (c *Client) dispatchStreamRequest(cn *conn, p *packet.StreamRequestPacket) {
	req := &StreamRequest{
		client:        c,
		conn:          cn,
		StreamID:      p.StreamID,
		Source:        p.Source,
		Topic:         p.Topic,
		Metadata:      cloneStreamBytes(p.Metadata),
		Properties:    cloneStreamMap(p.Properties),
		StreamAddress: p.StreamAddress,
		TLS:           p.TLS,
		BindTimeoutMs: p.BindTimeoutMs,
		ticketBytes:   cloneStreamBytes(p.Ticket),
	}

	c.streamMu.Lock()
	handler := c.streamHandler
	c.streamMu.Unlock()

	if handler != nil {
		go handler(req)
		return
	}

	select {
	case c.streamQueue <- req:
	default:
		c.logger.Warn("stream request queue full, rejecting",
			zap.Uint64("streamID", p.StreamID), zap.String("source", p.Source))
		// 不能在这个 goroutine 里等待响应：它正是读循环。
		go func() { _ = req.Reject(packet.ReasonServerBusy, "stream request queue is full") }()
	}
}

func (c *Client) handleStreamAbort(p *packet.StreamAbortPacket) {
	c.streamMu.Lock()
	wait := c.streamWaits[p.StreamID]
	delete(c.streamWaits, p.StreamID)
	c.streamMu.Unlock()

	if wait != nil {
		wait.abortNow()
	}
	c.logger.Debug("stream aborted", zap.Uint64("streamID", p.StreamID), zap.String("reason", p.ReasonCode.String()))
}

func (c *Client) registerStreamWait(id uint64) *streamWait {
	wait := newStreamWait()
	c.streamMu.Lock()
	if c.streamWaits == nil {
		c.streamWaits = make(map[uint64]*streamWait)
	}
	c.streamWaits[id] = wait
	c.streamMu.Unlock()
	return wait
}

func (c *Client) unregisterStreamWait(id uint64) {
	c.streamMu.Lock()
	delete(c.streamWaits, id)
	c.streamMu.Unlock()
}

func (c *Client) streamConn() (*conn, error) {
	if !c.opts.StreamCapable {
		return nil, ErrStreamClientNotCapable
	}
	cn := c.currentConn()
	if cn == nil || cn.closed.Load() {
		return nil, ErrNotConnected
	}
	if cn.connack == nil || !cn.connack.StreamSupported() {
		return nil, ErrStreamNotSupported
	}
	return cn, nil
}

func (c *Client) roundTripOn(
	ctx context.Context,
	cn *conn,
	pkt packet.Packet,
	requestID uint64,
	timeout time.Duration,
) (packet.Packet, error) {
	if cn == nil || cn.closed.Load() {
		return nil, ErrNotConnected
	}

	ch := make(chan packet.Packet, 1)
	cn.register(requestID, ch)
	defer cn.unregister(requestID)

	if err := cn.write(pkt); err != nil {
		return nil, NewConnectionError("write", err)
	}

	var timerC <-chan time.Time
	if timeout > 0 {
		t := time.NewTimer(timeout)
		defer t.Stop()
		timerC = t.C
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			if err := cn.Err(); err != nil {
				return nil, err
			}
			return nil, ErrNotConnected
		}
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timerC:
		return nil, ErrStreamTimeout
	case <-c.ctx.Done():
		return nil, ErrClientClosed
	}
}

func (c *Client) dialStream(ctx context.Context, wait *streamWait, address string, tlsEnabled bool) (net.Conn, error) {
	if c.opts.StreamAddress != "" {
		address = c.opts.StreamAddress
	}
	if address == "" {
		return nil, ErrStreamNotSupported
	}
	if wait != nil && wait.isAborted() {
		return nil, ErrStreamAborted
	}

	dialCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	done := make(chan struct{})
	defer close(done)
	go func() {
		var abort <-chan struct{}
		if wait != nil {
			abort = wait.abort
		}
		select {
		case <-abort:
			cancel()
		case <-c.ctx.Done():
			cancel()
		case <-done:
		}
	}()

	dialer := &net.Dialer{Timeout: c.opts.StreamDialTimeout}
	var (
		conn net.Conn
		err  error
	)
	if tlsEnabled || c.opts.StreamTLSConfig != nil {
		cfg := c.opts.StreamTLSConfig
		if cfg == nil {
			host, _, splitErr := net.SplitHostPort(address)
			if splitErr != nil {
				return nil, splitErr
			}
			cfg = &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
		}
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: cfg}).DialContext(dialCtx, "tcp", address)
	} else {
		conn, err = dialer.DialContext(dialCtx, "tcp", address)
	}
	if err != nil {
		if wait != nil && wait.isAborted() {
			return nil, ErrStreamAborted
		}
		if c.closed.Load() {
			return nil, ErrClientClosed
		}
		return nil, NewConnectionError("stream dial", err)
	}
	return conn, nil
}

func readStreamPacket(conn net.Conn, timeout time.Duration) (streampacket.Packet, error) {
	if timeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
	}
	pkt, err := streampacket.ReadPacket(conn)
	_ = conn.SetReadDeadline(time.Time{})
	if err != nil {
		return nil, NewConnectionError("stream handshake", err)
	}
	return pkt, nil
}

func streamBindTimeout(ms uint32) time.Duration {
	if ms > 0 {
		return time.Duration(ms) * time.Millisecond
	}
	return 30 * time.Second
}

func streamReasonError(code packet.ReasonCode, message string) error {
	if message == "" {
		message = code.String()
	}
	switch code {
	case packet.ReasonStreamDisabled:
		return fmt.Errorf("%w: %s", ErrStreamNotSupported, message)
	case packet.ReasonStreamNotFound:
		return fmt.Errorf("%w: %s", ErrStreamNotFound, message)
	case packet.ReasonStreamTargetUnavailable:
		return fmt.Errorf("%w: %s", ErrStreamTargetUnavailable, message)
	case packet.ReasonStreamTimeout:
		return fmt.Errorf("%w: %s", ErrStreamTimeout, message)
	default:
		return fmt.Errorf("%w: %s", ErrStreamRejected, message)
	}
}

func streamBindError(code streampacket.ReasonCode, message string) error {
	if message == "" {
		message = code.String()
	}
	switch code {
	case streampacket.StreamNotFound:
		return fmt.Errorf("%w: %s", ErrStreamNotFound, message)
	case streampacket.StreamLimitExceeded:
		return fmt.Errorf("%w: %s", ErrStreamRejected, message)
	default:
		return fmt.Errorf("%w: %s", ErrStreamAborted, message)
	}
}

func cloneStreamBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

func cloneStreamMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
