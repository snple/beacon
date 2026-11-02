package core

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snple/beacon/internal/wsconn"
	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// conn 表示单次 TCP 连接
// 负责管理底层网络连接、读写协程、心跳超时、包确认等连接级别的状态
type conn struct {
	// 所属的客户端会话
	client *Client

	// TCP 连接
	conn net.Conn
	// 保护写入操作
	writeMu sync.Mutex

	// 连接参数
	keepAlive    uint16
	traceID      string
	connectedAt  time.Time
	writeTimeout time.Duration // 单次写超时（0 表示不设置）

	// 接入方式与协议版本（管理面展示）
	transport string // "tcp" / "tls" / "ws"
	protocol  string // 例如 "MX/2"

	// lastActivity 最后一次收到数据包的时间（unix 纳秒）
	lastActivity atomic.Int64

	// closeErr 记录导致连接关闭的底层错误（管理面展示用）
	errMu    sync.Mutex
	closeErr error

	// 断开连接包（正常断开时非 nil）
	disconnectPacket *packet.DisconnectPacket

	// 流量控制（连接级别）
	maxPacketSize uint32 // 客户端最大包大小（发送时检查）
	sendWindow    uint16 // 客户端声明的接收窗口（= 待确认投递上限）

	// 消息发送控制
	processing  atomic.Bool // true: 正在处理发送队列
	pendingSend atomic.Bool // true: 处理期间又有新工作，发送协程退出前需再处理一轮

	// 待重发的投递（由重传定时器/重连触发），发送协程统一写出。
	resendMu  sync.Mutex
	resendIDs []*delivery
	resendSet map[packet.PacketID]struct{}

	// 发送协程跟踪：processSendQueue 纳入 wg，确保关闭存储前其已退出
	wg sync.WaitGroup

	// 生命周期
	ctx         context.Context
	cancel      context.CancelFunc
	closed      atomic.Bool
	closeReason atomic.Uint32 // packet.ReasonCode，记录连接关闭原因供 hooks/遗嘱判断
	closeOnce   sync.Once
}

// newConn 创建新的连接对象
func newConn(client *Client, netConn net.Conn, connect *packet.ConnectPacket) *conn {
	ctx, cancel := context.WithCancel(context.Background())
	conn := &conn{
		client:       client,
		conn:         netConn,
		keepAlive:    connect.KeepAlive,
		connectedAt:  time.Now(),
		writeTimeout: client.core.options.WriteTimeout,
		transport:    transportOf(netConn),
		protocol:     fmt.Sprintf("%s/%d", connect.ProtocolName, connect.ProtocolVersion),
		ctx:          ctx,
		cancel:       cancel,
	}
	conn.lastActivity.Store(conn.connectedAt.UnixNano())

	// 应用 Core 默认的 KeepAlive（如果客户端未指定或指定值过大）
	if client.core.options.KeepAlive > 0 && (conn.keepAlive == 0 || client.core.options.KeepAlive < conn.keepAlive) {
		conn.keepAlive = client.core.options.KeepAlive
	}

	// 从 CONNECT 属性中读取客户端的初始接收窗口并设置为 core 的发送窗口
	if connect.Properties != nil {
		conn.maxPacketSize = connect.Properties.MaxPacketSize
		conn.sendWindow = connect.Properties.ReceiveWindow
		conn.traceID = connect.Properties.TraceID
	}

	if conn.sendWindow == 0 {
		conn.sendWindow = defaultSendWindow
	}

	conn.resendSet = make(map[packet.PacketID]struct{})

	return conn
}

// Serve 开始处理客户端消息
func (c *conn) serve() {
	reason := packet.ReasonNormalDisconnect
	// 必须在 defer 中读取 reason，否则参数会在 defer 语句执行时求值，
	// 后面设置的心跳超时/协议错误原因码全部失效。
	defer func() { c.close(reason) }()

	// 会话恢复：上一次连接上"已发送但未确认"的投递需要在新连接上重发，
	// 否则要等一个完整的重传间隔才会被继续投递。
	c.resendPendingAcks()

	// 队列里可能已有消息（离线期间入队的），触发发送协程
	c.triggerSend()

	// 设置读取超时
	keepAliveTimeout := c.calculateKeepAliveTimeout()
	for !c.closed.Load() {
		// 更新读取超时
		_ = c.conn.SetReadDeadline(time.Now().Add(keepAliveTimeout))

		pkt, size, err := packet.ReadPacketWithSize(c.conn, c.client.core.MaxPacketSize())
		if err != nil {
			if errors.Is(err, io.EOF) || c.closed.Load() {
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				c.client.core.logger.Debug("Client keepalive timeout",
					zap.String("clientID", c.client.ID))

				reason = packet.ReasonKeepAliveTimeout
				return
			}
			c.client.core.logger.Debug("Read packet error",
				zap.String("clientID", c.client.ID),
				zap.Error(err))

			c.setCloseErr(err)
			reason = packet.ReasonProtocolError
			return
		}

		c.lastActivity.Store(time.Now().UnixNano())
		c.client.bytesReceived.Add(int64(size))
		c.client.core.stats.BytesReceived.Add(int64(size))

		if err := c.handlePacket(pkt); err != nil {
			c.client.core.logger.Debug("Handle packet error",
				zap.String("clientID", c.client.ID),
				zap.Error(err))

			reason = closeReasonFromError(err)
			return
		}
	}
}

// closeReasonError 让包处理函数指定连接关闭原因码（默认按 ProtocolError 处理）。
type closeReasonError struct {
	reason packet.ReasonCode
	err    error
}

func (e *closeReasonError) Error() string { return e.err.Error() }

func (e *closeReasonError) Unwrap() error { return e.err }

// closeReasonFromError 把 handlePacket 返回的错误映射为连接关闭原因。
func closeReasonFromError(err error) packet.ReasonCode {
	var cre *closeReasonError
	if errors.As(err, &cre) {
		return cre.reason
	}
	if errors.Is(err, io.EOF) {
		// 客户端发来 DISCONNECT：正常断开，不再回 DISCONNECT。
		return packet.ReasonNormalDisconnect
	}
	return packet.ReasonProtocolError
}

// calculateKeepAliveTimeout 计算 KeepAlive 超时时间
func (c *conn) calculateKeepAliveTimeout() time.Duration {
	if c.keepAlive == 0 {
		return defaultKeepAliveTimeout
	}
	return time.Duration(float64(c.keepAlive) * float64(time.Second) * keepAliveMultiplier)
}

// handlePacket 处理收到的包
func (c *conn) handlePacket(pkt packet.Packet) error {
	switch p := pkt.(type) {
	case *packet.PublishPacket:
		return c.client.handlePublish(p)
	case *packet.PubackPacket:
		return c.client.handlePuback(p)
	case *packet.SubscribePacket:
		return c.client.handleSubscribe(p)
	case *packet.UnsubscribePacket:
		return c.client.handleUnsubscribe(p)
	case *packet.PingPacket:
		return c.handlePing(p)
	case *packet.DisconnectPacket:
		return c.handleDisconnect(p)
	case *packet.AuthPacket:
		return c.client.handleAuth(p)
	case *packet.TracePacket:
		return c.client.handleTrace(p)
	default:
		c.client.core.logger.Debug("Unknown packet type",
			zap.Uint8("type", uint8(pkt.Type())))
		return nil
	}
}

// handlePing 处理 PING 包
func (c *conn) handlePing(p *packet.PingPacket) error {
	// 回复 PONG，回显客户端时间戳
	pong := &packet.PongPacket{
		Seq:  p.Seq,
		Echo: p.Timestamp,
	}
	if err := c.writePacket(pong); err != nil {
		c.client.core.logger.Error("Failed to send PONG",
			zap.String("clientID", c.client.ID),
			zap.Error(err))
		return err
	}
	return nil
}

// handleDisconnect 处理 DISCONNECT 包
func (c *conn) handleDisconnect(p *packet.DisconnectPacket) error {
	c.client.core.logger.Debug("Client disconnect request",
		zap.String("clientID", c.client.ID),
		zap.Uint8("reasonCode", uint8(p.ReasonCode)))
	// 保存断开包供 hooks 使用
	c.disconnectPacket = p
	return io.EOF // 触发断开
}

// writePacket 发送数据包（控制包与兼容路径）。
func (c *conn) writePacket(pkt packet.Packet) error {
	// 编码先于任何写入完成：编码失败不应影响连接。
	frame, err := packet.EncodePacket(pkt, c.maxPacketSize)
	if err != nil {
		return err
	}

	_, err = c.writeSegments([][]byte{frame})
	return err
}

// writePublishFrame 分段写出 PUBLISH 帧。
//
// 固定头、flags、PacketID 单独构造，topic/属性/载荷直接引用共享内存：
// 向 N 个订阅者投递同一条消息不会复制任何消息内容。
func (c *conn) writePublishFrame(frame *packet.PublishFrame) error {
	_, err := c.writeSegments(frame.Segments())
	return err
}

// writeSegments 串行化写入多段数据，并统计字节数。
//
// 分段写（TCP writev / WebSocket 单条消息）避免拼接缓冲区；TLS 等不支持
// 分段写的 Writer 会退化为顺序 Write，数据仍然不复制。
func (c *conn) writeSegments(segs [][]byte) (int, error) {
	if c.closed.Load() || c.conn == nil {
		return 0, ErrClientClosed
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if c.writeTimeout > 0 {
		if err := c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout)); err != nil {
			return 0, err
		}
		defer func() { _ = c.conn.SetWriteDeadline(time.Time{}) }()
	}

	n, err := packet.WriteSegments(c.conn, segs)
	if n > 0 {
		c.client.bytesSent.Add(n)
		c.client.core.stats.BytesSent.Add(n)
	}

	// 写错误（超时、对端不读、连接重置）意味着流中可能已有半包，连接不可继续使用。
	if err != nil {
		_ = c.conn.Close()
		return int(n), &packet.WriteError{Err: err}
	}

	return int(n), nil
}

// resendPendingAcks 把待确认的投递全部标记为重发（新连接建立时调用）。
func (c *conn) resendPendingAcks() {
	for _, d := range c.client.session.pendingAck.snapshot() {
		c.addResend(d)
	}
}

// closeReasonCode 返回连接关闭原因。
func (c *conn) closeReasonCode() packet.ReasonCode {
	return packet.ReasonCode(c.closeReason.Load())
}

// wait 等待连接的所有发送协程退出
// 用于停机/会话接管时同步，确保不再有协程访问该连接的队列。
// 注意：不要在持有 clientsMu 时调用（发送协程可能通过用户钩子回调 core API）。
func (c *conn) wait() {
	c.wg.Wait()
}

// close 关闭连接（非正常断开时向对端发送 DISCONNECT，受写超时约束）。
func (c *conn) close(reason packet.ReasonCode) {
	c.closeWith(reason, true)
}

// closeLocal 关闭连接但不发送 DISCONNECT。
// 用于停机、会话接管等不需要（或不应等待）通知对端的场景。
func (c *conn) closeLocal(reason packet.ReasonCode) {
	c.closeWith(reason, false)
}

// detach 仅切换本地状态（无网络 IO、无阻塞），供必须持有全局锁的路径使用。
// 真正的 socket 关闭由调用方在锁外通过 closeLocal 完成。
func (c *conn) detach(reason packet.ReasonCode) {
	c.closeReason.Store(uint32(reason))
	c.cancel()
	c.closed.Store(true)
}

func (c *conn) closeWith(reason packet.ReasonCode, sendDisconnect bool) {
	c.closeOnce.Do(func() {
		c.closeReason.Store(uint32(reason))

		// 发送 DISCONNECT (如果不是正常断开)。
		// 必须在设置 closed 标志之前发送，否则 writePacket 会直接返回；
		// 该写入受 writeTimeout 约束，不会无限阻塞关闭流程。
		if sendDisconnect && reason != packet.ReasonNormalDisconnect {
			disconnect := packet.NewDisconnectPacket(reason)
			if err := c.writePacket(disconnect); err != nil {
				c.client.core.logger.Debug("Failed to send DISCONNECT",
					zap.String("clientID", c.client.ID),
					zap.Error(err))
			}
		}

		c.cancel()
		// 设置 closed 标志，阻止新的写入
		c.closed.Store(true)

		// 关闭连接（检查是否为 nil）
		if c.conn != nil {
			_ = c.conn.Close()
		}
	})
}

// setCloseErr 记录导致连接关闭的底层错误。
func (c *conn) setCloseErr(err error) {
	if err == nil {
		return
	}
	c.errMu.Lock()
	if c.closeErr == nil {
		c.closeErr = err
	}
	c.errMu.Unlock()
}

// err 返回导致连接关闭的底层错误（可能为 nil）。
func (c *conn) err() error {
	c.errMu.Lock()
	defer c.errMu.Unlock()
	return c.closeErr
}

// lastActiveAt 返回最后一次收到数据包的时间。
func (c *conn) lastActiveAt() time.Time {
	ns := c.lastActivity.Load()
	if ns <= 0 {
		return c.connectedAt
	}
	return time.Unix(0, ns)
}

// transportOf 根据底层连接类型推断接入方式。
func transportOf(netConn net.Conn) string {
	switch netConn.(type) {
	case *wsconn.Conn:
		return "ws"
	case *tls.Conn:
		return "tls"
	default:
		return "tcp"
	}
}
