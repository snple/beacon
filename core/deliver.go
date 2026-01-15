package core

import (
	"errors"
	"time"

	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// Publish 发布消息 (内部使用)
func (c *Core) Publish(msg *Message) {
	// 处理保留消息
	if msg.Retain && c.options.RetainEnabled {
		if len(msg.Payload) == 0 {
			c.retainStore.Remove(msg.Topic)
			// 从持久化存储中删除
			if c.messageStore != nil {
				c.messageStore.DeleteRetainMessage(msg.Topic)
			}
		} else {
			c.retainStore.Set(msg.Topic, msg)
			// 持久化保留消息
			if c.messageStore != nil {
				if err := c.messageStore.SaveRetainMessage(msg.Topic, msg); err != nil {
					c.logger.Error("Failed to persist retain message",
						zap.String("topic", msg.Topic),
						zap.Error(err))
				}
			}
		}
	}

	// 加入优先级队列
	priority := packet.PriorityNormal
	if msg.Priority != nil {
		priority = *msg.Priority
	}
	c.queues[priority].Push(msg)

	// 通知 dispatchLoop 有新消息 (非阻塞)
	select {
	case c.msgNotify <- struct{}{}:
	default:
		// channel 已有通知，无需重复发送
	}

	c.stats.MessagesReceived.Add(1)
}

// PublishOptions 发布选项
type PublishOptions struct {
	QoS         packet.QoS
	Retain      bool
	Priority    *packet.Priority
	TraceID     string
	ContentType string
	Expiry      uint32

	// 请求-响应模式属性
	TargetClientID  string // 目标客户端ID，用于点对点消息
	ResponseTopic   string // 响应主题
	CorrelationData []byte // 关联数据
}

func (c *Core) dispatchLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.msgNotify:
			// 批量处理消息，提高吞吐量
			for {
				processed := false
				// 按优先级从高到低处理消息
				for i := len(c.queues) - 1; i >= 0; i-- {
					if msg := c.queues[i].Pop(); msg != nil {
						c.deliverMessage(msg)
						processed = true
						break // 保证高优先级优先
					}
				}
				if !processed {
					break // 所有队列都空了
				}
			}
		}
	}
}

func (c *Core) deliverMessage(msg *Message) {
	// 如果指定了 TargetClientID，直接投递给目标客户端
	if msg.TargetClientID != "" {
		c.deliverToTarget(msg)
		return
	}

	subscribers := c.subscriptions.Match(msg.Topic)

	// 一次性获取所有需要的客户端，减少锁竞争
	c.clientsMu.RLock()
	type clientWithQoS struct {
		client *Client
		qos    packet.QoS
	}
	clients := make([]clientWithQoS, 0, len(subscribers))
	for _, sub := range subscribers {
		if client, ok := c.clients[sub.ClientID]; ok {
			// 确定发送 QoS (取订阅 QoS 和消息 QoS 的较小值)
			qos := min(sub.QoS, msg.QoS)
			clients = append(clients, clientWithQoS{client: client, qos: qos})
		}
	}
	c.clientsMu.RUnlock()

	// 在锁外发送消息给普通客户端
	for _, cq := range clients {
		if cq.client.IsClosed() {
			c.stats.MessagesDropped.Add(1)
			continue
		}

		// 直接调用 DeliverWithQoS，持久化逻辑在其中统一处理
		if err := cq.client.DeliverWithQoS(msg, cq.qos); err != nil {
			if cq.qos == packet.QoS0 {
				c.logger.Debug("QoS0 message delivery failed",
					zap.String("clientID", cq.client.ID),
					zap.String("topic", msg.Topic),
					zap.Error(err))
				c.stats.MessagesDropped.Add(1)
			} else {
				c.logger.Debug("QoS1 message delivery failed",
					zap.String("clientID", cq.client.ID),
					zap.String("topic", msg.Topic),
					zap.Error(err))
			}
		}
	}
}

// deliverToTarget 投递消息给指定的目标客户端
func (c *Core) deliverToTarget(msg *Message) {
	targetID := msg.TargetClientID

	// 先尝试普通客户端
	c.clientsMu.RLock()
	client, exists := c.clients[targetID]
	c.clientsMu.RUnlock()

	if exists {
		if client.IsClosed() {
			c.stats.MessagesDropped.Add(1)
			return
		}

		// OnDeliver 钩子在 DeliverWithQoS 内部调用（那里有完整的 PublishPacket）
		if err := client.DeliverWithQoS(msg, msg.QoS); err != nil {
			c.logger.Debug("Target delivery failed",
				zap.String("targetClientID", targetID),
				zap.String("topic", msg.Topic),
				zap.Error(err))
			if msg.QoS == packet.QoS0 {
				c.stats.MessagesDropped.Add(1)
			}
		}
		return
	}

	// 目标客户端不存在
	c.logger.Debug("Target client not found",
		zap.String("targetClientID", targetID),
		zap.String("topic", msg.Topic))
	c.stats.MessagesDropped.Add(1)
}

// PublishToClient 向指定客户端发送消息
// 如果客户端在线但未订阅该主题，仍然会将消息投递给该客户端
func (c *Core) PublishToClient(clientID string, topic string, payload []byte, options PublishOptions) error {
	msg := &Message{
		Topic:           topic,
		Payload:         payload,
		QoS:             options.QoS,
		Retain:          options.Retain,
		Timestamp:       time.Now().Unix(),
		Priority:        options.Priority,
		TraceID:         options.TraceID,
		ContentType:     options.ContentType,
		TargetClientID:  options.TargetClientID,
		ResponseTopic:   options.ResponseTopic,
		CorrelationData: options.CorrelationData,
	}

	if options.Expiry > 0 {
		expiryTime := time.Now().Add(time.Duration(options.Expiry) * time.Second)
		msg.ExpiryTime = expiryTime.Unix()
	}

	c.clientsMu.RLock()
	client, exists := c.clients[clientID]
	c.clientsMu.RUnlock()

	if exists {
		if client.IsClosed() {
			return NewClientClosedError(clientID)
		}
		return client.DeliverWithQoS(msg, options.QoS)
	}

	return NewClientNotFoundError(clientID)
}

// Broadcast 向所有在线客户端广播消息
func (c *Core) Broadcast(topic string, payload []byte, options PublishOptions) int {
	c.clientsMu.RLock()
	clients := make([]*Client, 0, len(c.clients))
	for _, client := range c.clients {
		if !client.IsClosed() {
			clients = append(clients, client)
		}
	}
	c.clientsMu.RUnlock()

	msg := &Message{
		Topic:           topic,
		Payload:         payload,
		QoS:             options.QoS,
		Retain:          options.Retain,
		Timestamp:       time.Now().Unix(),
		Priority:        options.Priority,
		TraceID:         options.TraceID,
		ContentType:     options.ContentType,
		ResponseTopic:   options.ResponseTopic,
		CorrelationData: options.CorrelationData,
	}

	if options.Expiry > 0 {
		expiryTime := time.Now().Add(time.Duration(options.Expiry) * time.Second)
		msg.ExpiryTime = expiryTime.Unix()
	}

	successCount := 0
	for _, client := range clients {
		if err := client.DeliverWithQoS(msg, options.QoS); err == nil {
			successCount++
		}
	}

	return successCount
}

// DeliverWithQoS 向客户端发送消息，指定 QoS
// 流控策略：
// 1. 检查消息是否过期
// 2. QoS1 消息先持久化
// 3. 尝试放入发送队列：
//   - 成功：启动发送协程处理
//   - 失败（队列满）：
//   - QoS0: 直接丢弃
//   - QoS1: 已持久化，等待重传机制处理
func (c *Client) DeliverWithQoS(msg *Message, qos packet.QoS) error {
	// 检查消息是否过期（过期消息直接丢弃，无论 QoS）
	if msg.ExpiryTime > 0 && time.Now().Unix() > msg.ExpiryTime {
		c.core.stats.MessagesDropped.Add(1)
		return nil
	}

	// QoS 1: 先持久化到存储（确保不丢失）
	if qos == packet.QoS1 && c.core.messageStore != nil {
		if err := c.core.messageStore.Save(c.ID, msg); err != nil {
			c.core.logger.Error("Failed to persist QoS1 message",
				zap.String("clientID", c.ID),
				zap.String("topic", msg.Topic),
				zap.Error(err))
			return err
		}
		c.core.logger.Debug("QoS1 message persisted",
			zap.String("clientID", c.ID),
			zap.String("topic", msg.Topic))
	}

	// 如果客户端关闭，直接返回
	if c.closed.Load() {
		if qos == packet.QoS0 {
			c.core.logger.Debug("QoS0 message dropped, client closed",
				zap.String("clientID", c.ID),
				zap.String("topic", msg.Topic))
			c.core.stats.MessagesDropped.Add(1)
			return errors.New("client closed")
		}
		// QoS1: 后续会持久化，重连后重传
		return nil
	}

	// 尝试放入发送队列
	if c.sendQueue.TryEnqueue(msg, qos, false) {
		// 成功入队，触发发送协程
		c.triggerSend()
		return nil
	}

	// 队列已满
	if qos == packet.QoS0 {
		// QoS0: 直接丢弃
		c.core.logger.Debug("QoS0 message dropped, send queue full",
			zap.String("clientID", c.ID),
			zap.String("topic", msg.Topic),
			zap.Int("queueUsed", c.sendQueue.Used()),
			zap.Int("queueCapacity", c.sendQueue.Capacity()))
		c.core.stats.MessagesDropped.Add(1)
		return errors.New("send queue full")
	}

	// QoS1: 已持久化，等待重传机制处理
	c.core.logger.Debug("QoS1 message queued for retransmit (queue full)",
		zap.String("clientID", c.ID),
		zap.String("topic", msg.Topic))
	return nil
}

// Deliver 向客户端发送消息（使用消息原始 QoS）
func (c *Client) Deliver(msg *Message) error {
	return c.DeliverWithQoS(msg, msg.QoS)
}

// triggerSend 触发发送协程（非阻塞）
// 使用 delivering 标志避免重复启动
func (c *Client) triggerSend() {
	if c.processing.CompareAndSwap(false, true) {
		go c.processSendQueue()
	}
}

// processSendQueue 处理发送队列中的消息
func (c *Client) processSendQueue() {
	defer c.processing.Store(false)

	for !c.closed.Load() {
		// 尝试从队列取消息
		qm, ok := c.sendQueue.TryDequeue()
		if !ok {
			// 队列已空
			break
		}

		// 发送消息
		if err := c.sendMessage(qm.Message, qm.QoS, qm.Dup); err != nil {
			c.core.logger.Debug("Failed to send message from queue",
				zap.String("clientID", c.ID),
				zap.String("topic", qm.Message.Topic),
				zap.Error(err))

			// 如果是 QoS1 且持久化失败，则已经在持久化层有备份
			// 如果是 QoS0，则丢弃
			if qm.QoS == packet.QoS0 {
				c.core.stats.MessagesDropped.Add(1)
			}
		}
	}
}

// pendingMessage 等待确认的消息
type pendingMessage struct {
	msg        *Message
	qos        packet.QoS
	packetID   uint16
	lastSentAt int64 // 最后发送时间（Unix 时间戳）
}

// sendMessage 发送消息到客户端
func (c *Client) sendMessage(msg *Message, qos packet.QoS, dup bool) error {
	// 创建 PUBLISH 包
	pub := packet.NewPublishPacket(msg.Topic, msg.Payload)
	pub.QoS = qos
	pub.Retain = msg.Retain
	pub.Dup = dup

	if qos > 0 {
		pub.PacketID = c.allocatePacketID()
	}

	// 设置属性
	pub.Properties.Priority = msg.Priority
	pub.Properties.TraceID = msg.TraceID
	pub.Properties.ContentType = msg.ContentType
	pub.Properties.ExpiryTime = msg.ExpiryTime
	pub.Properties.UserProperties = msg.UserProperties
	pub.Properties.TargetClientID = msg.TargetClientID
	pub.Properties.SourceClientID = msg.SourceClientID
	pub.Properties.ResponseTopic = msg.ResponseTopic
	pub.Properties.CorrelationData = msg.CorrelationData

	// 调用 OnDeliver 钩子，检查是否允许投递
	deliverCtx := &DeliverContext{
		ClientID: c.ID,
		Packet:   pub,
		QoS:      qos,
	}
	if !c.core.options.Hooks.callOnDeliver(deliverCtx) {
		return errors.New("delivery rejected by OnDeliver hook") // 钩子拒绝投递
	}

	// 先发送消息
	if err := c.WritePacket(pub); err != nil {
		return err
	}

	// 发送成功后处理 QoS 1
	if qos == packet.QoS1 {
		// 加入 pendingAck 等待确认
		c.qosMu.Lock()
		c.pendingAck[pub.PacketID] = &pendingMessage{
			msg:        msg,
			qos:        qos,
			packetID:   pub.PacketID,
			lastSentAt: time.Now().Unix(),
		}
		c.qosMu.Unlock()
	}
	// QoS 0: 发送后即清理，无需等待确认

	// 发送成功，统计消息发送数
	c.core.stats.MessagesSent.Add(1)
	return nil
}
