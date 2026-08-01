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

// pendingMessage 待确认消息
// ch：同步等待者的 ack 通知（publish/subscribe/unsubscribe）；
// msg/lastSentAt：QoS1 发布消息的重传跟踪（subscribe/unsubscribe 时为 nil/0）。
type pendingMessage struct {
	ch         chan error
	msg        *Message
	lastSentAt int64 // Unix 秒
}

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
	pendingAck   map[nson.Id]pendingMessage
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
		pendingAck:     make(map[nson.Id]pendingMessage),
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

	// 手动确认模式：QoS1 且 AutoAck=false。
	// 消息入队后不自动发送 PUBACK，由应用处理完后显式调用 Message.Ack() 确认。
	//
	// 注意：这里不做“未确认重传去重”——若应用处理中崩溃/断电未确认，
	// 服务端会重传该消息，必须重新投递给应用，才能保证 QoS1“消息被处理”的语义。
	// 因此 QoS1 手动模式下为 at-least-once：应用如需不重复处理，请按
	// msg.Packet.PacketID 自行去重（处理完成后调用 Ack() 停止服务端重传）。
	manualAck := p.QoS == packet.QoS1 && (c.client.options == nil || !c.client.options.AutoAck)

	if manualAck {
		// 挂载确认回调（在入队前设置，避免与 PollMessage 消费产生竞态）
		msg.ack = func() error {
			puback := packet.NewPubackPacket(p.PacketID, packet.ReasonSuccess)
			return c.writePacket(puback)
		}
	}

	// 非阻塞放入消息队列
	select {
	case c.client.messageQueue <- msg:
		c.logger.Debug("Enqueued message for polling", zap.String("topic", p.Topic))
	default:
		// 队列满：QoS 1 返回配额超限，告知服务端丢弃并清理 pendingAck；QoS 0 直接丢弃
		if p.QoS == packet.QoS1 {
			c.logger.Warn("Message queue full, ACKing QoS1 message with quota exceeded",
				zap.String("topic", p.Topic))

			puback := packet.NewPubackPacket(p.PacketID, packet.ReasonQuotaExceeded)
			if err := c.writePacket(puback); err != nil {
				c.logger.Warn("Failed to send quota-exceeded PUBACK", zap.Error(err))
				return err
			}

			return nil
		}
		c.logger.Warn("Message queue full, QoS0 message dropped", zap.String("topic", p.Topic))
		return nil
	}

	// 自动确认模式（默认）：消息成功入队后再发送 PUBACK
	if p.QoS == packet.QoS1 && !manualAck {
		puback := packet.NewPubackPacket(p.PacketID, packet.ReasonSuccess)
		if err := c.writePacket(puback); err != nil {
			c.logger.Warn("Failed to send PUBACK", zap.Error(err))
			return err
		}
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
		if !code.IsSubackSuccess() {
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

	// Seq 不匹配时记录日志，尝试用 Echo 时间戳作为后备
	if p.Seq != 0 {
		c.logger.Debug("PONG seq mismatch, using echo timestamp fallback",
			zap.Uint32("expected", c.lastPingSeq.Load()),
			zap.Uint32("got", p.Seq))
	}

	if p.Echo > 0 && uint64(now) > p.Echo {
		c.lastRTTNanos.Store(int64(uint64(now) - p.Echo))
	}
}

// handleAck 处理确认响应
func (c *Conn) handleAck(packetID nson.Id, err error) {
	c.pendingAckMu.Lock()
	pending, ok := c.pendingAck[packetID]
	if ok {
		delete(c.pendingAck, packetID)
	}
	c.pendingAckMu.Unlock()

	if ok && pending.ch != nil {
		pending.ch <- err
		close(pending.ch)
	}
}

func (c *Conn) registerPendingAck(packetID nson.Id) chan error {
	ch := make(chan error, 1)
	c.pendingAckMu.Lock()
	// 懒初始化：白盒测试/直接构造 Conn 时 map 可能为 nil（赋值会 panic，
	// 而 delete 对 nil map 是安全的）
	if c.pendingAck == nil {
		c.pendingAck = make(map[nson.Id]pendingMessage)
	}
	c.pendingAck[packetID] = pendingMessage{ch: ch}
	c.pendingAckMu.Unlock()
	return ch
}

func (c *Conn) clearPendingAck(packetID nson.Id) {
	c.pendingAckMu.Lock()
	delete(c.pendingAck, packetID)
	c.pendingAckMu.Unlock()
}

// sendMessage 发送消息
// 只负责发送；PacketTooLarge 时返回 ErrMessageTooLarge（由调用方决定丢弃还是重试）
func (c *Conn) sendMessage(msg *Message) error {
	pub := msg.Packet.Copy()
	pub.Dup = msg.Dup

	if err := c.writePacket(&pub); err != nil {
		if errors.Is(err, &packet.PacketTooLargeError{}) {
			// 数据包超过客户端允许的最大大小，丢弃消息
			c.client.logger.Warn("Message dropped: packet size exceeds client maxPacketSize",
				zap.String("topic", pub.Topic),
				zap.String("packetID", pub.PacketID.Hex()),
				zap.Error(err))

			return ErrMessageTooLarge
		}

		// 其他写入错误
		return err
	}

	// 发送成功：QoS1 记录消息与发送时间，供重传机制（retransmitUnackedMessages）跟踪。
	// 与 core 侧对称：以实际发送时间作为重传计时起点。
	if msg.Packet.QoS == packet.QoS1 {
		packetID := msg.Packet.PacketID
		c.pendingAckMu.Lock()
		// 懒初始化：白盒测试/直接构造 Conn 时 map 可能为 nil（赋值会 panic）
		if c.pendingAck == nil {
			c.pendingAck = make(map[nson.Id]pendingMessage)
		}
		if pending, ok := c.pendingAck[packetID]; ok {
			pending.msg = msg
			pending.lastSentAt = time.Now().Unix()
			c.pendingAck[packetID] = pending
		} else {
			c.pendingAck[packetID] = pendingMessage{
				msg:        msg,
				lastSentAt: time.Now().Unix(),
			}
		}
		c.pendingAckMu.Unlock()
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
		for _, pending := range c.pendingAck {
			if pending.ch != nil {
				select {
				case pending.ch <- errors.New("conn closed"):
				default:
				}
			}
		}
		c.pendingAck = make(map[nson.Id]pendingMessage)
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
