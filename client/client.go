package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// Client 是 Beacon v4 客户端：PUSH 发布、PULL 拉取、COMMIT 确认。
type Client struct {
	opts   *ClientOptions
	logger *zap.Logger

	mu      sync.RWMutex
	conn    *conn
	connack *packet.ConnackPacket
	// assignedID 是服务端在 CONNACK 中分配的 ClientID（可能覆盖配置值）。
	assignedID string
	// limits 是最近一次 DECLARE_RESP 公布的配额。
	limits *packet.Limits

	reqSeq atomic.Uint64

	ctx    context.Context
	cancel context.CancelFunc
	closed atomic.Bool

	// 当前会话的未确认消息（可靠优先级）
	uncommittedMu sync.Mutex
	uncommitted   []*Message

	// pullMu 串行化 Pull：并发拉取会让服务端重复投递同一批消息，
	// 也会让本地未确认集合互相覆盖。
	pullMu sync.Mutex

	// connectMu 串行化建连，避免重复 Connect 泄漏旧连接或启动多个 supervise。
	connectMu sync.Mutex

	// PollMessage 的本地缓冲
	bufMu sync.Mutex
	buf   []*Message

	// 会话变化回调
	onSessionReset func(prev, next uint64)

	supWG sync.WaitGroup
}

// NewWithOptions 创建客户端。
func NewWithOptions(opts *ClientOptions) (*Client, error) {
	if opts == nil {
		opts = NewClientOptions()
	}
	opts.applyDefaults()
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		opts:   opts,
		logger: opts.Logger,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// New 使用默认配置创建客户端。
func New() (*Client, error) { return NewWithOptions(NewClientOptions()) }

// OnSessionReset 设置会话重置回调（SessionID 变化时触发）。
func (c *Client) OnSessionReset(fn func(prev, next uint64)) {
	c.mu.Lock()
	c.onSessionReset = fn
	c.mu.Unlock()
}

// sessionResetCallback 返回当前回调（并发安全）。
func (c *Client) sessionResetCallback() func(prev, next uint64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.onSessionReset
}

// ClientID 返回服务端分配的客户端 ID。
func (c *Client) ClientID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.assignedID != "" {
		return c.assignedID
	}
	if c.connack != nil && c.connack.AssignedClientID != "" {
		return c.connack.AssignedClientID
	}
	return c.opts.ClientID
}

// SessionID 返回当前会话租约标识。
func (c *Client) SessionID() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.connack != nil {
		return c.connack.SessionID
	}
	return 0
}

// SessionPresent 报告本次连接是否复用了既有会话。
func (c *Client) SessionPresent() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connack != nil && c.connack.SessionPresent
}

// Connect 建立连接。
func (c *Client) Connect() error { return c.ConnectContext(context.Background()) }

// ConnectWithDialer 使用指定拨号器建立连接。
func (c *Client) ConnectWithDialer(d Dialer) error {
	c.mu.Lock()
	c.opts.Dialer = d
	c.mu.Unlock()
	return c.ConnectContext(context.Background())
}

// ConnectContext 建立连接，并在 AutoReconnect 开启时启动后台重连。
func (c *Client) ConnectContext(ctx context.Context) error {
	if c.closed.Load() {
		return ErrClientClosed
	}

	if err := c.connectOnce(ctx); err != nil {
		return err
	}

	if c.opts.AutoReconnect {
		c.supWG.Add(1)
		go c.supervise()
	}
	return nil
}

func (c *Client) dial() (net.Conn, error) {
	c.mu.RLock()
	custom := c.opts.Dialer
	c.mu.RUnlock()
	if custom != nil {
		return custom.Dial()
	}
	if hasWSScheme(c.opts.Address) {
		d := &WSDialer{
			Address:        c.opts.Address,
			DialTimeout:    c.opts.ConnectTimeout,
			MaxMessageSize: int64(c.opts.MaxPacketSize),
		}
		return d.Dial()
	}
	d := &TCPDialer{Address: c.opts.Address, DialTimeout: c.opts.ConnectTimeout}
	return d.Dial()
}

// connectOnce 完成一次握手。
func (c *Client) connectOnce(ctx context.Context) error {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	if c.closed.Load() {
		return ErrClientClosed
	}
	if cn := c.currentConn(); cn != nil && !cn.closed.Load() {
		return ErrAlreadyConnected
	}

	nc, err := c.dial()
	if err != nil {
		return err
	}

	cn := newConn(c, nc)

	connect := packet.NewConnectPacket(c.opts.ClientID)
	connect.KeepAlive = c.opts.KeepAlive
	connect.CleanSession = c.opts.CleanSession
	connect.MaxPacketSize = c.opts.MaxPacketSize
	connect.AuthMethod = c.opts.AuthMethod
	connect.AuthData = c.opts.AuthData

	if d := c.opts.ConnectTimeout; d > 0 {
		_ = nc.SetDeadline(time.Now().Add(d))
	}

	if err := cn.write(connect); err != nil {
		_ = nc.Close()
		return NewConnectionError("connect", err)
	}

	pkt, _, err := packet.ReadPacketWithSize(nc, c.opts.MaxPacketSize)
	if err != nil {
		_ = nc.Close()
		return NewConnectionError("read connack", err)
	}
	_ = nc.SetDeadline(time.Time{})

	connack, ok := pkt.(*packet.ConnackPacket)
	if !ok {
		_ = nc.Close()
		return fmt.Errorf("expected CONNACK, got %s", pkt.Type())
	}
	if err := reasonToError(connack.ReasonCode); err != nil {
		_ = nc.Close()
		return err
	}

	cn.connack = connack
	if connack.ServerMaxPacketSize > 0 && connack.ServerMaxPacketSize < cn.maxPacketSize {
		cn.maxPacketSize = connack.ServerMaxPacketSize
	}
	if c.closed.Load() {
		cn.close(ErrClientClosed)
		return ErrClientClosed
	}

	prevSession := c.SessionID()

	c.mu.Lock()
	c.conn = cn
	c.connack = connack
	if connack.AssignedClientID != "" {
		// 不修改 opts（构造后视为只读），避免与并发读取竞争。
		c.assignedID = connack.AssignedClientID
	}
	closedAfterAssign := c.closed.Load()
	c.mu.Unlock()

	// Close 可能发生在握手期间；不要留下一个无人关闭的新连接。
	if closedAfterAssign {
		cn.close(ErrClientClosed)
		return ErrClientClosed
	}

	if prevSession != 0 && prevSession != connack.SessionID {
		// 会话被重建：本地未确认状态作废
		c.dropLocalUncommitted()
		if fn := c.sessionResetCallback(); fn != nil {
			fn(prevSession, connack.SessionID)
		}
	}

	go cn.readLoop()
	// 服务端可在 CONNACK 中要求更短/更长的心跳间隔，必须采用它的值。
	keepAlive := c.opts.KeepAlive
	if connack.KeepAlive > 0 {
		keepAlive = connack.KeepAlive
	}
	pingInterval := time.Duration(keepAlive) * time.Second / 2
	if pingInterval < time.Second {
		pingInterval = time.Second
	}
	go cn.pingLoop(pingInterval)

	c.logger.Debug("connected",
		zap.String("clientID", connack.AssignedClientID),
		zap.Bool("sessionPresent", connack.SessionPresent),
		zap.Uint32("pending", connack.Pending[0]+connack.Pending[1]+connack.Pending[2]),
		zap.Uint32("delivered", connack.Delivered))

	// 订阅声明统一由 DECLARE 承担（CONNECT 不再携带）：每次建连后重新声明，
	// 这样 CleanSession 清空或被按新身份过滤掉的声明都会在重连后恢复。
	if len(c.opts.Channels) > 0 {
		if _, err := c.Declare(c.ctx, c.opts.Channels...); err != nil {
			cn.close(err)
			return err
		}
	}
	return nil
}

// supervise 在连接断开后自动重连。
func (c *Client) supervise() {
	defer c.supWG.Done()

	for {
		c.mu.RLock()
		cn := c.conn
		c.mu.RUnlock()
		if cn == nil {
			return
		}

		select {
		case <-c.ctx.Done():
			return
		case <-cn.Done():
		}

		err := cn.Err()
		if errors.Is(err, ErrSessionTakenOver) || errors.Is(err, ErrKicked) || errors.Is(err, ErrClientClosed) {
			c.logger.Debug("not reconnecting", zap.Error(err))
			return
		}
		if c.closed.Load() {
			return
		}

		backoff := c.opts.ReconnectMin
		for {
			select {
			case <-c.ctx.Done():
				return
			case <-time.After(backoff):
			}

			if err := c.connectOnce(c.ctx); err == nil {
				c.logger.Debug("reconnected")
				break
			} else if errors.Is(err, ErrSessionTakenOver) || errors.Is(err, ErrKicked) ||
				errors.Is(err, ErrAlreadyConnected) {
				c.logger.Debug("reconnect refused", zap.Error(err))
				return
			}

			backoff *= 2
			if backoff > c.opts.ReconnectMax {
				backoff = c.opts.ReconnectMax
			}
		}
	}
}

// Close 关闭客户端。
func (c *Client) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	c.cancel()

	c.mu.RLock()
	cn := c.conn
	c.mu.RUnlock()
	if cn != nil {
		_ = cn.write(packet.NewDisconnectPacket(packet.ReasonNormalDisconnect))
		cn.close(ErrClientClosed)
	}

	c.supWG.Wait()
	return nil
}

// Connected 报告当前是否已连接。
func (c *Client) Connected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn != nil && !c.conn.closed.Load()
}

func (c *Client) currentConn() *conn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}

// roundTrip 发送请求并等待响应。
func (c *Client) roundTrip(ctx context.Context, pkt packet.Packet, requestID uint64, timeout time.Duration) (packet.Packet, error) {
	cn := c.currentConn()
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
		return nil, ErrTimeout
	case <-c.ctx.Done():
		return nil, ErrClientClosed
	}
}

func (c *Client) nextRequestID() uint64 { return c.reqSeq.Add(1) }

// ============================================================================
// 本地未确认追踪
// ============================================================================

// trackUncommitted 把本批需要确认的消息并入未确认集合（按 ID 去重）。
//
// 服务端会重发未确认消息，因此同一 ID 可能多次出现；这里按 ID 去重，
// 保证集合规模只与「真正未确认的条数」有关。
func (c *Client) trackUncommitted(msgs []*Message) {
	c.uncommittedMu.Lock()
	defer c.uncommittedMu.Unlock()

	if len(c.uncommitted) == 0 {
		for _, m := range msgs {
			if m.NeedsCommit() && !m.acked.Load() {
				c.uncommitted = append(c.uncommitted, m)
			}
		}
		return
	}

	known := make(map[packet.PacketID]struct{}, len(c.uncommitted))
	for _, m := range c.uncommitted {
		known[m.ID] = struct{}{}
	}
	for _, m := range msgs {
		if !m.NeedsCommit() || m.acked.Load() {
			continue
		}
		if _, ok := known[m.ID]; ok {
			continue
		}
		known[m.ID] = struct{}{}
		c.uncommitted = append(c.uncommitted, m)
	}
}

func (c *Client) dropLocalUncommitted() {
	c.uncommittedMu.Lock()
	c.uncommitted = nil
	c.uncommittedMu.Unlock()

	c.bufMu.Lock()
	c.buf = nil
	c.bufMu.Unlock()
}

func (c *Client) hasUncommitted() bool {
	c.uncommittedMu.Lock()
	defer c.uncommittedMu.Unlock()
	return len(c.uncommitted) > 0
}
