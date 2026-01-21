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

// conn 表示单次 TCP 连接
// 负责管理底层网络连接、读写协程、心跳超时、包确认等连接级别的状态
type conn struct {
	// 所属的客户端会话
	client *Client

	// TCP 连接
	conn net.Conn

	// 连接参数
	keepAlive   uint16
	traceID     string
	connectedAt time.Time

	// 断开连接包（正常断开时非 nil）
	disconnectPacket *packet.DisconnectPacket

	// 流量控制（连接级别）
	maxPacketSize uint32     // 客户端最大包大小（发送时检查）
	sendWindow    uint16     // 客户端接收窗口大小(core 发送窗口)
	sendQueue     *sendQueue // 发送队列（基于客户端的 ReceiveWindow）

	// 消息发送控制
	processing atomic.Bool // true: 正在处理发送队列

	// 生命周期
	ctx       context.Context
	cancel    context.CancelFunc
	closed    atomic.Bool
	closeOnce sync.Once
}

// newConn 创建新的连接对象
func newConn(client *Client, netConn net.Conn, connect *packet.ConnectPacket) *conn {
	ctx, cancel := context.WithCancel(context.Background())
	conn := &conn{
		client:      client,
		conn:        netConn,
		keepAlive:   connect.KeepAlive,
		connectedAt: time.Now(),
		ctx:         ctx,
		cancel:      cancel,
	}

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

	// 创建发送队列（基于客户端的接收窗口）
	if conn.sendWindow > 0 {
		conn.sendQueue = newSendQueue(int(conn.sendWindow))
	} else {
		// 如果客户端未指定接收窗口，使用默认值
		conn.sendQueue = newSendQueue(100)
		conn.sendWindow = 100
	}

	return conn
}

// Serve 开始处理客户端消息
func (c *conn) serve() {
	reason := packet.ReasonNormalDisconnect
	defer c.close(reason)

	// 发送队列里可能已经有消息，触发发送协程
	c.triggerSend()

	// 设置读取超时
	keepAliveTimeout := c.calculateKeepAliveTimeout()
	for !c.closed.Load() {
		// 更新读取超时
		c.conn.SetReadDeadline(time.Now().Add(keepAliveTimeout))

		pkt, err := packet.ReadPacket(c.conn, c.client.core.MaxPacketSize())
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

			reason = packet.ReasonProtocolError
			return
		}

		if err := c.handlePacket(pkt); err != nil {
			c.client.core.logger.Debug("Handle packet error",
				zap.String("clientID", c.client.ID),
				zap.Error(err))

			if !errors.Is(err, io.EOF) {
				// 协议错误
				reason = packet.ReasonProtocolError
			}
			return
		}
	}
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
	case *packet.RegisterPacket:
		return c.client.handleRegister(p)
	case *packet.UnregisterPacket:
		return c.client.handleUnregister(p)
	case *packet.RequestPacket:
		return c.client.handleRequest(p)
	case *packet.ResponsePacket:
		return c.client.handleResponse(p)
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

// writePacket 发送数据包
func (c *conn) writePacket(pkt packet.Packet) error {
	if c.closed.Load() {
		return errors.New("connection closed")
	}

	if err := packet.WritePacket(c.conn, pkt, c.maxPacketSize); err != nil {
		return err
	}

	return nil
}

// close 关闭连接
func (c *conn) close(reason packet.ReasonCode) {
	c.closeOnce.Do(func() {
		// 发送 DISCONNECT (如果不是正常断开)
		// 注意：必须在设置 closed 标志之前发送，以避免并发写入竞态
		// 直接写入连接，不使用 writePacket (它会检查 closed 标志)
		if reason != packet.ReasonNormalDisconnect {
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
			c.conn.Close()
		}
	})
}
