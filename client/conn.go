package client

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// Conn 表示单次 TCP 连接
// 负责管理底层网络连接、读写协程、心跳、包确认等连接级别的状态
type Conn struct {
	// 所属的客户端（会话管理器）
	client *Client

	// TCP 连接
	conn net.Conn
	// 保护写入操作
	writeMu sync.Mutex

	// 连接会话状态（每次连接可能不同）
	clientID       string // 实际使用的客户端 ID（可能由服务器分配）
	keepAlive      uint16 // 实际使用的心跳间隔
	sessionTimeout uint32 // 会话过期时间
	sessionPresent bool   // 服务端是否恢复了会话
	maxPacketSize  uint32 // 允许发送的最大包大小
	sendWindow     uint16 // 发送窗口大小

	// 发送队列（连接级别，允许离线缓存）
	sendQueue  *sendQueue
	processing atomic.Bool

	// 等待确认的包（连接级别，断开后失效）
	pendingAck   map[nson.Id]chan error
	pendingAckMu sync.Mutex

	// 心跳 RTT
	pingSeq      atomic.Uint32
	lastRTTNanos atomic.Int64
	lastPingSeq  atomic.Uint32
	lastPingSent atomic.Int64

	// 生命周期
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closed    atomic.Bool
	closeOnce sync.Once

	logger *zap.Logger
}

// newConn 创建新的连接对象，从旧连接迁移状态
func newConn(c *Client, netConn net.Conn, connack *packet.ConnackPacket) *Conn {
	// 提取连接参数
	clientID := c.options.ClientID
	if connack.Properties.ClientID != "" {
		clientID = connack.Properties.ClientID
	}

	keepAlive := c.options.KeepAlive
	if connack.Properties.KeepAlive != 0 {
		keepAlive = connack.Properties.KeepAlive
	}

	sessionTimeout := c.options.SessionTimeout
	if connack.Properties.SessionTimeout != 0 {
		sessionTimeout = connack.Properties.SessionTimeout
	}

	maxMessageSize := c.options.MaxPacketSize
	if connack.Properties.MaxPacketSize != 0 {
		maxMessageSize = connack.Properties.MaxPacketSize
	}

	sendWindow := uint16(defaultReceiveWindow)
	if connack.Properties.ReceiveWindow != 0 {
		sendWindow = connack.Properties.ReceiveWindow
	}

	sessionPresent := connack.SessionPresent

	ctx, cancel := context.WithCancel(c.ctx)

	conn := &Conn{
		client:         c,
		conn:           netConn,
		clientID:       clientID,
		keepAlive:      keepAlive,
		sessionTimeout: sessionTimeout,
		sessionPresent: sessionPresent,
		maxPacketSize:  maxMessageSize,
		sendWindow:     sendWindow,
		sendQueue:      newSendQueue(int(sendWindow)),
		pendingAck:     make(map[nson.Id]chan error),
		ctx:            ctx,
		cancel:         cancel,
		logger:         c.logger,
	}

	// 清空持久化队列（clean session）
	if conn.sessionTimeout == 0 {
		if err := c.queue.Clear(); err != nil {
			c.logger.Warn("Failed to clear queue for clean session", zap.Error(err))
		}
	}

	return conn
}

// start 启动连接的所有协程
func (conn *Conn) start() {
	// 启动接收协程
	conn.wg.Add(1)
	go conn.receiveLoop()

	// 启动心跳协程
	conn.wg.Add(1)
	go conn.keepAliveLoop()

	// 触发消息发送（从 sendQueue 发送消息）
	conn.triggerSend()
}

// receiveLoop 接收消息循环
func (c *Conn) receiveLoop() {
	defer c.wg.Done()

	for !c.closed.Load() {
		// 设置读取超时
		timeout := time.Duration(c.keepAlive) * time.Second * 2
		c.conn.SetReadDeadline(time.Now().Add(timeout))

		pkt, err := packet.ReadPacket(c.conn, c.client.MaxPacketSize())
		if err != nil {
			if errors.Is(err, io.EOF) || c.closed.Load() {
				return
			}

			c.client.logger.Debug("Read packet error",
				zap.String("clientID", c.clientID),
				zap.Error(err))
			c.close(err)

			return
		}

		if err := c.handlePacket(pkt); err != nil {
			c.client.logger.Debug("Handle packet error",
				zap.String("clientID", c.clientID),
				zap.Error(err))
			if !errors.Is(err, io.EOF) {
				// 协议错误
				c.close(NewServerDisconnectError("Protocol error"))
			}

			c.close(err)
			return
		}
	}
}

// keepAliveLoop 心跳循环
func (c *Conn) keepAliveLoop() {
	defer c.wg.Done()
	interval := time.Duration(c.keepAlive) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if c.closed.Load() {
				return
			}

			now := time.Now().UnixNano()
			seq := c.pingSeq.Add(1)
			c.lastPingSeq.Store(seq)
			c.lastPingSent.Store(now)

			ping := packet.NewPingPacket(seq, uint64(now))
			if err := c.writePacket(ping); err != nil {
				c.client.logger.Warn("Failed to send ping", zap.Error(err))
				c.close(err)
				return
			}
		}
	}
}

// handlePacket 处理收到的包
func (c *Conn) handlePacket(pkt packet.Packet) error {
	switch p := pkt.(type) {
	case *packet.PublishPacket:
		return c.handlePublish(p)
	case *packet.PubackPacket:
		c.handlePuback(p)
	case *packet.SubackPacket:
		c.handleSuback(p)
	case *packet.UnsubackPacket:
		c.handleUnsuback(p)
	case *packet.PongPacket:
		c.handlePong(p)
	case *packet.DisconnectPacket:
		c.close(NewServerDisconnectError(p.ReasonCode.String()))
	case *packet.RequestPacket:
		return c.client.handleRequest(p)
	case *packet.ResponsePacket:
		return c.client.handleResponse(p)
	case *packet.RegackPacket:
		c.client.handleRegack(p)
	case *packet.UnregackPacket:
		c.client.handleUnregack(p)
	case *packet.AuthPacket:
		c.client.handleAuth(p)
	case *packet.TracePacket:
		c.client.handleTrace(p)
	default:
		c.client.logger.Debug("Unknown packet type",
			zap.Uint8("type", uint8(pkt.Type())))
		return nil
	}

	return nil
}

// handlePublish 处理 PUBLISH 包
func (c *Conn) handlePublish(p *packet.PublishPacket) error {
	// 调用钩子
	if !c.client.options.Hooks.callOnPublish(&PublishContext{
		ClientID: c.clientID,
		Packet:   p,
	}) {
		c.logger.Debug("Message rejected by hook", zap.String("topic", p.Topic))

		// 这里钩子拒绝消息，但仍需回应 PUBACK（如果 QoS 1）
		if p.QoS == packet.QoS1 {
			puback := packet.NewPubackPacket(p.PacketID, packet.ReasonSuccess)
			if err := c.writePacket(puback); err != nil {
				c.logger.Warn("Failed to send PUBACK", zap.Error(err))
				return err
			}
		}
		return nil
	}

	msg := &Message{Packet: p}

	// 非阻塞放入消息队列
	select {
	case c.client.messageQueue <- msg:
		c.logger.Debug("Enqueued message for polling", zap.String("topic", p.Topic))
		if p.QoS == packet.QoS1 {
			puback := packet.NewPubackPacket(p.PacketID, packet.ReasonSuccess)
			if err := c.writePacket(puback); err != nil {
				c.logger.Warn("Failed to send PUBACK", zap.Error(err))
				return err
			}
		}
	default:
		c.logger.Warn("Message queue full, message dropped", zap.String("topic", p.Topic))
	}

	return nil
}

// handlePuback 处理 PUBACK
func (c *Conn) handlePuback(p *packet.PubackPacket) {
	var err error
	if p.ReasonCode != packet.ReasonSuccess {
		err = NewPublishWarningError(p.ReasonCode.String())
	}

	// 从持久化队列删除（无论成功失败）
	if delErr := c.client.queue.Delete(p.PacketID); delErr != nil {
		c.logger.Warn("Failed to delete message from queue after ACK",
			zap.String("packetID", p.PacketID.Hex()),
			zap.Error(delErr))
	}

	c.handleAck(p.PacketID, err)
}

// handleSuback 处理 SUBACK
func (c *Conn) handleSuback(p *packet.SubackPacket) {
	var err error
	for i, code := range p.ReasonCodes {
		if code != packet.ReasonSuccess && code != packet.ReasonCode(packet.QoS0) &&
			code != packet.ReasonCode(packet.QoS1) {
			err = NewSubscriptionError(i, code.String())
			break
		}
	}
	c.handleAck(p.PacketID, err)
}

// handleUnsuback 处理 UNSUBACK
func (c *Conn) handleUnsuback(p *packet.UnsubackPacket) {
	var err error
	for i, code := range p.ReasonCodes {
		if code != packet.ReasonSuccess {
			err = NewUnsubscriptionError(i, code.String())
			break
		}
	}
	c.handleAck(p.PacketID, err)
}

// handlePong 处理 PONG
func (c *Conn) handlePong(p *packet.PongPacket) {
	now := time.Now().UnixNano()
	if p.Seq != 0 && p.Seq == c.lastPingSeq.Load() {
		sent := c.lastPingSent.Load()
		if sent > 0 && now > sent {
			c.lastRTTNanos.Store(now - sent)
			return
		}
	}
	if p.Echo > 0 && uint64(now) > p.Echo {
		c.lastRTTNanos.Store(int64(uint64(now) - p.Echo))
	}
}

// handleAck 处理确认响应
func (c *Conn) handleAck(packetID nson.Id, err error) {
	c.pendingAckMu.Lock()
	ch, ok := c.pendingAck[packetID]
	if ok {
		delete(c.pendingAck, packetID)
	}
	c.pendingAckMu.Unlock()

	if ok {
		ch <- err
		close(ch)
	}
}

// sendMessage 发送消息
func (c *Conn) sendMessage(msg *Message) error {
	pub := msg.Packet // 复制数据包以修改 PacketID

	if err := c.writePacket(pub); err != nil {
		if errors.Is(err, &packet.PacketTooLargeError{}) {
			// 数据包超过客户端允许的最大大小，丢弃消息
			c.client.logger.Warn("Message dropped: packet size exceeds client maxPacketSize",
				zap.String("topic", pub.Topic),
				zap.String("packetID", pub.PacketID.Hex()),
				zap.Error(err))

			// 从队列中删除该消息
			if err := c.client.queue.Delete(pub.PacketID); err != nil {
				c.client.logger.Debug("Failed to delete oversized message from queue",
					zap.String("packetID", pub.PacketID.Hex()),
					zap.Error(err))
			}

			return nil
		}

		// 其他写入错误
		return err
	}

	// 发送成功
	if pub.QoS == packet.QoS0 {
		// QoS0: 从持久化队列删除（TCP 写成功即可）
		if c.client.queue != nil {
			if err := c.client.queue.Delete(pub.PacketID); err != nil {
				c.logger.Warn("Failed to delete message from queue after send",
					zap.String("packetID", pub.PacketID.Hex()),
					zap.Error(err))
			}
		}
	}

	return nil
}

func (c *Conn) writePacket(pkt packet.Packet) error {
	if c.closed.Load() {
		return errors.New("connection closed")
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	return packet.WritePacket(c.conn, pkt, c.maxPacketSize)
}

// waitAck 等待包确认
func (c *Conn) waitAck(packetID nson.Id, timeout time.Duration) error {
	ch := make(chan error, 1)
	c.pendingAckMu.Lock()
	c.pendingAck[packetID] = ch
	c.pendingAckMu.Unlock()
	select {
	case err := <-ch:
		return err
	case <-time.After(timeout):
		c.pendingAckMu.Lock()
		delete(c.pendingAck, packetID)
		c.pendingAckMu.Unlock()
		return ErrPublishTimeout
	case <-c.ctx.Done():
		return ErrNotConnected
	}
}

// close 关闭连接
func (c *Conn) close(err error) {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		if c.cancel != nil {
			c.cancel()
		}

		// 关闭网络连接
		if c.conn != nil {
			c.conn.Close()
		}

		// 清理等待中的确认
		c.pendingAckMu.Lock()
		for _, ch := range c.pendingAck {
			select {
			case ch <- errors.New("conn closed"):
			default:
			}
		}
		c.pendingAck = make(map[nson.Id]chan error)
		c.pendingAckMu.Unlock()

		// 通知客户端连接已断开
		c.client.onConnLost(err)
	})
}

// wait 等待连接关闭
func (c *Conn) wait() {
	c.wg.Wait()
}

// IsClosed 检查是否已关闭
func (c *Conn) IsClosed() bool {
	return c.closed.Load()
}

// LastRTT 返回最近一次 RTT
func (c *Conn) LastRTT() time.Duration {
	ns := c.lastRTTNanos.Load()
	if ns <= 0 {
		return 0
	}
	return time.Duration(ns)
}
