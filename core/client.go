package core

import (
	"bufio"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// 客户端相关常量
const (
	// 超时相关
	keepAliveMultiplier     = 1.5 // KeepAlive * 1.5 作为读取超时
	defaultKeepAliveTimeout = 60 * time.Second

	// PacketID 相关
	minPacketID = 1
	maxPacketID = 65535

	// 重传相关
	overflowBatchSize = 100 // 从持久化加载的每批最大条数
)

// Client 表示一个已连接的客户端
type Client struct {
	ID      string
	conn    net.Conn
	core    *Core
	reader  *bufio.Reader
	writer  *bufio.Writer
	writeMu sync.Mutex

	// 连接信息
	KeepAlive        uint16
	CleanSession     bool
	SessionExpiry    uint32 // 会话过期时间（秒），0 表示断开即清理
	AuthMethod       string // 认证方法
	AuthData         []byte // 认证数据
	TraceID          string
	ConnectedAt      time.Time                // 连接建立时间
	DisconnectPacket *packet.DisconnectPacket // 断开连接包（正常断开时非 nil）

	// 遗嘱消息
	WillPacket *packet.PublishPacket

	// 会话状态
	subscriptions map[string]packet.SubscribeOptions
	subsMu        sync.RWMutex

	// 包 ID 管理
	nextPacketID atomic.Uint32

	// QoS 状态 (只需要 QoS 0/1)
	pendingAck map[uint16]*pendingMessage // QoS 1: 等待客户端 ACK 的消息
	qosMu      sync.Mutex

	// 消息发送
	processing atomic.Bool // true: 正在处理客户端消息
	// 流量控制
	retransmitting atomic.Bool // true: 正在重传消息
	sendWindow     uint16      // 客户端接收窗口大小(core 发送窗口)
	sendQueue      *SendQueue  // 发送队列（基于客户端的 ReceiveWindow）

	// 生命周期
	closed    atomic.Bool
	closeCh   chan struct{}
	closeOnce sync.Once
}

// NewClient 创建新的客户端
func NewClient(conn net.Conn, connect *packet.ConnectPacket, core *Core) *Client {
	c := &Client{
		ID:            connect.ClientID,
		conn:          conn,
		core:          core,
		reader:        bufio.NewReader(conn),
		writer:        bufio.NewWriter(conn),
		KeepAlive:     connect.KeepAlive,
		CleanSession:  connect.Flags.CleanSession,
		ConnectedAt:   time.Now(),
		subscriptions: make(map[string]packet.SubscribeOptions),
		pendingAck:    make(map[uint16]*pendingMessage),
		closeCh:       make(chan struct{}),
	}

	// 从 CONNECT 属性中读取客户端的初始接收窗口并设置为 core 的发送窗口
	if connect.Properties != nil && connect.Properties.ReceiveWindow != nil {
		c.sendWindow = *connect.Properties.ReceiveWindow
	}

	// 创建发送队列（基于客户端的接收窗口）
	if c.sendWindow > 0 {
		c.sendQueue = NewSendQueue(int(c.sendWindow))
	} else {
		// 如果客户端未指定接收窗口，使用默认值
		c.sendQueue = NewSendQueue(100)
		c.sendWindow = 100
	}

	// 提取遗嘱消息
	c.extractWillMessage(connect)

	// 从属性中提取认证信息和特性
	c.extractConnectProperties(connect, core)

	c.nextPacketID.Store(minPacketID)

	return c
}

// extractConnectProperties 从连接属性中提取信息
func (c *Client) extractConnectProperties(connect *packet.ConnectPacket, core *Core) {
	if connect.Properties != nil {
		c.AuthMethod = connect.Properties.AuthMethod
		c.AuthData = connect.Properties.AuthData
		c.TraceID = connect.Properties.TraceID
	}

	// 处理会话过期时间
	// 规则：
	// 1. CleanSession=true: SessionExpiry 无意义，设为 0（断开即清理）
	// 2. CleanSession=false:
	//    - 客户端未设置或设置为 0: 使用 core 的 MaxSessionExpiry
	//    - 客户端设置了值: 取 min(客户端值, MaxSessionExpiry)
	if c.CleanSession {
		c.SessionExpiry = 0
		return
	}

	// CleanSession=false，需要保留会话
	var sessionExpiry uint32

	if connect.Properties != nil && connect.Properties.SessionExpiry != nil {
		sessionExpiry = *connect.Properties.SessionExpiry
	}

	// 如果客户端未设置或设置为 0，使用 core 默认值
	if sessionExpiry == 0 {
		sessionExpiry = core.options.MaxSessionExpiry
	} else if core.options.MaxSessionExpiry > 0 && sessionExpiry > core.options.MaxSessionExpiry {
		// 客户端设置的值超过最大值，使用最大值
		sessionExpiry = core.options.MaxSessionExpiry
	}

	c.SessionExpiry = sessionExpiry
}

// Serve 开始处理客户端消息
func (c *Client) Serve() {
	defer c.Close(packet.ReasonNormalDisconnect)

	// 处理持久化消息
	if c.CleanSession {
		// CleanSession=true: 清空该客户端在 BadgerDB 中的旧消息
		c.clearPersistedMessages()
	}

	// 设置读取超时
	keepAliveTimeout := c.calculateKeepAliveTimeout()

	for !c.closed.Load() {
		// 更新读取超时
		c.conn.SetReadDeadline(time.Now().Add(keepAliveTimeout))

		pkt, err := packet.ReadPacket(c.reader)
		if err != nil {
			if errors.Is(err, io.EOF) || c.closed.Load() {
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				c.core.logger.Debug("Client keepalive timeout", zap.String("clientID", c.ID))
				return
			}
			c.core.logger.Debug("Read packet error", zap.String("clientID", c.ID), zap.Error(err))
			return
		}

		if err := c.handlePacket(pkt); err != nil {
			c.core.logger.Debug("Handle packet error", zap.String("clientID", c.ID), zap.Error(err))
			return
		}
	}
}

// calculateKeepAliveTimeout 计算 KeepAlive 超时时间
func (c *Client) calculateKeepAliveTimeout() time.Duration {
	if c.KeepAlive == 0 {
		return defaultKeepAliveTimeout
	}
	return time.Duration(float64(c.KeepAlive) * float64(time.Second) * keepAliveMultiplier)
}

func (c *Client) handlePacket(pkt packet.Packet) error {
	switch p := pkt.(type) {
	case *packet.PublishPacket:
		return c.handlePublish(p)
	case *packet.PubackPacket:
		return c.handlePuback(p)
	case *packet.SubscribePacket:
		return c.handleSubscribe(p)
	case *packet.UnsubscribePacket:
		return c.handleUnsubscribe(p)
	case *packet.PingPacket:
		return c.handlePing(p)
	case *packet.DisconnectPacket:
		return c.handleDisconnect(p)
	case *packet.RegisterPacket:
		return c.handleRegister(p)
	case *packet.UnregisterPacket:
		return c.handleUnregister(p)
	case *packet.RequestPacket:
		return c.handleRequest(p)
	case *packet.ResponsePacket:
		return c.handleResponse(p)
	case *packet.AuthPacket:
		return c.handleAuth(p)
	case *packet.TracePacket:
		return c.handleTrace(p)
	default:
		c.core.logger.Debug("Unknown packet type", zap.Uint8("type", uint8(pkt.Type())))
		return nil
	}
}

func (c *Client) handlePing(p *packet.PingPacket) error {
	// 回复 PONG，回显客户端时间戳
	pong := &packet.PongPacket{
		Seq:  p.Seq,
		Echo: p.Timestamp,
	}
	return c.WritePacket(pong)
}

func (c *Client) handleDisconnect(p *packet.DisconnectPacket) error {
	c.core.logger.Debug("Client disconnect request",
		zap.String("clientID", c.ID),
		zap.Uint8("reasonCode", uint8(p.ReasonCode)))
	// 保存断开包供 hooks 使用
	c.DisconnectPacket = p
	return io.EOF // 触发断开
}

// allocatePacketID 分配新的 PacketID
func (c *Client) allocatePacketID() uint16 {
	id := c.nextPacketID.Add(1)
	if id == 0 || id > maxPacketID {
		c.nextPacketID.Store(minPacketID)
		return minPacketID
	}
	return uint16(id)
}

// WritePacket 发送数据包
func (c *Client) WritePacket(pkt packet.Packet) error {
	if c.closed.Load() {
		return errors.New("client closed")
	}

	// 检查 writer 是否为 nil（测试环境中可能没有真实连接）
	if c.writer == nil {
		return errors.New("writer not initialized")
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if err := packet.WritePacket(c.writer, pkt); err != nil {
		return err
	}
	return c.writer.Flush()
}

// Close 关闭客户端连接
func (c *Client) Close(reason packet.ReasonCode) {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		close(c.closeCh)

		// 关闭发送队列
		if c.sendQueue != nil {
			c.sendQueue.Close()
		}

		// 发送 DISCONNECT (如果不是正常断开)
		if reason != packet.ReasonNormalDisconnect {
			disconnect := packet.NewDisconnectPacket(reason)
			c.WritePacket(disconnect)
		}

		// 关闭连接（检查是否为 nil）
		if c.conn != nil {
			c.conn.Close()
		}
	})
}

// IsClosed 检查客户端是否已关闭
func (c *Client) IsClosed() bool {
	return c.closed.Load()
}
