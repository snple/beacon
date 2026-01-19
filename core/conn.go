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

// Conn 表示单次 TCP 连接
// 负责管理底层网络连接、读写协程、心跳超时、包确认等连接级别的状态
type Conn struct {
	// 所属的客户端会话
	client *Client

	// TCP 连接
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
	connMu sync.RWMutex

	// 连接参数
	KeepAlive   uint16
	TraceID     string
	ConnectedAt time.Time

	// 断开连接包（正常断开时非 nil）
	DisconnectPacket *packet.DisconnectPacket

	// 流量控制（连接级别）
	sendWindow uint16     // 客户端接收窗口大小(core 发送窗口)
	sendQueue  *sendQueue // 发送队列（基于客户端的 ReceiveWindow）

	// 消息发送控制
	processing atomic.Bool // true: 正在处理发送队列
	writeMu    sync.Mutex

	// 生命周期
	closed    atomic.Bool
	cleanup   atomic.Bool // 关闭时是否清理（会话被接管时为 false）
	closeCh   chan struct{}
	closeOnce sync.Once
}

// newConn 创建新的连接对象
func newConn(client *Client, netConn net.Conn, connect *packet.ConnectPacket) *Conn {
	conn := &Conn{
		client:      client,
		conn:        netConn,
		reader:      bufio.NewReader(netConn),
		writer:      bufio.NewWriter(netConn),
		KeepAlive:   connect.KeepAlive,
		ConnectedAt: time.Now(),
		closeCh:     make(chan struct{}),
	}

	// 应用 Core 默认的 KeepAlive（如果客户端未指定或指定值过大）
	if client.core.options.KeepAlive > 0 && (conn.KeepAlive == 0 || client.core.options.KeepAlive < conn.KeepAlive) {
		conn.KeepAlive = client.core.options.KeepAlive
	}

	// 从 CONNECT 属性中读取客户端的初始接收窗口并设置为 core 的发送窗口
	if connect.Properties != nil {
		conn.sendWindow = connect.Properties.ReceiveWindow
		conn.TraceID = connect.Properties.TraceID
	}

	// 创建发送队列（基于客户端的接收窗口）
	if conn.sendWindow > 0 {
		conn.sendQueue = newSendQueue(int(conn.sendWindow))
	} else {
		// 如果客户端未指定接收窗口，使用默认值
		conn.sendQueue = newSendQueue(100)
		conn.sendWindow = 100
	}

	conn.cleanup.Store(true)

	return conn
}

// Serve 开始处理客户端消息
func (conn *Conn) Serve() {
	defer conn.close(packet.ReasonNormalDisconnect, false)

	// 设置读取超时
	keepAliveTimeout := conn.calculateKeepAliveTimeout()

	for !conn.closed.Load() {
		// 更新读取超时
		conn.conn.SetReadDeadline(time.Now().Add(keepAliveTimeout))

		pkt, err := packet.ReadPacket(conn.reader)
		if err != nil {
			if errors.Is(err, io.EOF) || conn.closed.Load() {
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				conn.client.core.logger.Debug("Client keepalive timeout",
					zap.String("clientID", conn.client.ID))
				return
			}
			conn.client.core.logger.Debug("Read packet error",
				zap.String("clientID", conn.client.ID),
				zap.Error(err))
			return
		}

		if err := conn.handlePacket(pkt); err != nil {
			conn.client.core.logger.Debug("Handle packet error",
				zap.String("clientID", conn.client.ID),
				zap.Error(err))
			return
		}
	}
}

// calculateKeepAliveTimeout 计算 KeepAlive 超时时间
func (conn *Conn) calculateKeepAliveTimeout() time.Duration {
	if conn.KeepAlive == 0 {
		return defaultKeepAliveTimeout
	}
	return time.Duration(float64(conn.KeepAlive) * float64(time.Second) * keepAliveMultiplier)
}

// handlePacket 处理收到的包
func (conn *Conn) handlePacket(pkt packet.Packet) error {
	switch p := pkt.(type) {
	case *packet.PublishPacket:
		return conn.client.handlePublish(p)
	case *packet.PubackPacket:
		return conn.client.handlePuback(p)
	case *packet.SubscribePacket:
		return conn.client.handleSubscribe(p)
	case *packet.UnsubscribePacket:
		return conn.client.handleUnsubscribe(p)
	case *packet.PingPacket:
		return conn.handlePing(p)
	case *packet.DisconnectPacket:
		return conn.handleDisconnect(p)
	case *packet.RegisterPacket:
		return conn.client.handleRegister(p)
	case *packet.UnregisterPacket:
		return conn.client.handleUnregister(p)
	case *packet.RequestPacket:
		return conn.client.handleRequest(p)
	case *packet.ResponsePacket:
		return conn.client.handleResponse(p)
	case *packet.AuthPacket:
		return conn.client.handleAuth(p)
	case *packet.TracePacket:
		return conn.client.handleTrace(p)
	default:
		conn.client.core.logger.Debug("Unknown packet type",
			zap.Uint8("type", uint8(pkt.Type())))
		return nil
	}
}

// handlePing 处理 PING 包
func (conn *Conn) handlePing(p *packet.PingPacket) error {
	// 回复 PONG，回显客户端时间戳
	pong := &packet.PongPacket{
		Seq:  p.Seq,
		Echo: p.Timestamp,
	}
	return conn.writePacket(pong)
}

// handleDisconnect 处理 DISCONNECT 包
func (conn *Conn) handleDisconnect(p *packet.DisconnectPacket) error {
	conn.client.core.logger.Debug("Client disconnect request",
		zap.String("clientID", conn.client.ID),
		zap.Uint8("reasonCode", uint8(p.ReasonCode)))
	// 保存断开包供 hooks 使用
	conn.DisconnectPacket = p
	return io.EOF // 触发断开
}

// writePacket 发送数据包
func (conn *Conn) writePacket(pkt packet.Packet) error {
	if conn.closed.Load() {
		return errors.New("connection closed")
	}

	// 检查 writer 是否为 nil（测试环境中可能没有真实连接）
	if conn.writer == nil {
		return errors.New("writer not initialized")
	}

	conn.writeMu.Lock()
	defer conn.writeMu.Unlock()

	if err := packet.WritePacket(conn.writer, pkt); err != nil {
		return err
	}
	return conn.writer.Flush()
}

// triggerSend 触发发送协程（非阻塞）
// 使用 processing 标志避免重复启动
func (conn *Conn) triggerSend() {
	if conn.processing.CompareAndSwap(false, true) {
		go conn.processSendQueue()
	}
}

// processSendQueue 处理发送队列中的消息
func (conn *Conn) processSendQueue() {
	defer conn.processing.Store(false)

	for !conn.closed.Load() {
		// 尝试从队列取消息
		msg, ok := conn.sendQueue.tryDequeue()
		if !ok {
			// 队列已空
			break
		}

		// 发送消息
		if err := conn.client.sendMessage(msg); err != nil {
			conn.client.core.logger.Debug("Failed to send message from queue",
				zap.String("clientID", conn.client.ID),
				zap.String("topic", msg.Packet.Topic),
				zap.Error(err))

			// 如果是 QoS1 且持久化失败，则已经在持久化层有备份
			// 如果是 QoS0，则丢弃
			if msg.QoS == packet.QoS0 {
				conn.client.core.stats.MessagesDropped.Add(1)
			}
		}
	}
}

// close 关闭连接
// cleanup: true 表示执行清理逻辑，false 表示跳过清理（会话被接管时）
func (conn *Conn) close(reason packet.ReasonCode, cleanup bool) {
	conn.closeOnce.Do(func() {
		conn.closed.Store(true)
		conn.cleanup.Store(cleanup)
		close(conn.closeCh)

		// 发送 DISCONNECT (如果不是正常断开)
		if reason != packet.ReasonNormalDisconnect {
			disconnect := packet.NewDisconnectPacket(reason)
			conn.writePacket(disconnect)
		}

		// 关闭连接（检查是否为 nil）
		if conn.conn != nil {
			conn.conn.Close()
		}
	})
}

// IsClosed 检查连接是否已关闭
func (conn *Conn) IsClosed() bool {
	return conn.closed.Load()
}
