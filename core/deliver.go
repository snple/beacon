package core

import (
	"errors"
	"time"

	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// Publish 发布消息 (内部使用)
func (c *Core) publish(msg Message) {
	// 处理保留消息
	if msg.Retain && c.options.RetainEnabled {
		if len(msg.Packet.Payload) == 0 {
			// 删除保留消息
			c.retainStore.remove(msg.Packet.Topic)
			// 从持久化存储中删除
			if c.messageStore != nil {
				c.messageStore.deleteRetainMessage(msg.Packet.Topic)
			}
		} else {
			// 计算过期时间
			var expiryTime int64
			if msg.Packet.Properties != nil && msg.Packet.Properties.ExpiryTime > 0 {
				expiryTime = msg.Packet.Properties.ExpiryTime
			}

			// 先存入 retainStore 索引
			if err := c.retainStore.set(msg.Packet.Topic, expiryTime); err != nil {
				c.logger.Error("Failed to set retain message index",
					zap.String("topic", msg.Packet.Topic),
					zap.Error(err))
			}

			// 持久化保留消息到 messageStore
			if c.messageStore != nil {
				// !!! 这里传指针
				if err := c.messageStore.saveRetainMessage(msg.Packet.Topic, &msg); err != nil {
					c.logger.Error("Failed to persist retain message",
						zap.String("topic", msg.Packet.Topic),
						zap.Error(err))
				}
			}
		}
	}

	// 加入优先级队列
	priority := packet.PriorityNormal
	if msg.Packet.Properties != nil && msg.Packet.Properties.Priority != nil {
		priority = *msg.Packet.Properties.Priority
	}

	// 放入对应优先级的队列
	// !!! 这里把指针放了进去
	c.queues[priority].push(&msg)

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
					if msg := c.queues[i].pop(); msg != nil {
						// !!! push 放进去的是指针
						c.deliver(msg)
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

func (c *Core) deliver(msg *Message) {
	// 如果指定了 TargetClientID，直接投递给目标客户端
	if msg.Packet.Properties != nil && msg.Packet.Properties.TargetClientID != "" {
		c.deliverToTarget(msg)
		return
	}

	subscribers := c.subscriptions.match(msg.Packet.Topic)

	// 一次性获取所有需要的客户端，减少锁竞争
	c.clientsMu.RLock()
	type clientWithQoS struct {
		client *Client
		qos    packet.QoS
	}
	clients := make([]clientWithQoS, 0, len(subscribers))
	for clientID, subQoS := range subscribers {
		if client, ok := c.clients[clientID]; ok {
			// 确定发送 QoS (取订阅 QoS 和消息 QoS 的较小值)
			qos := min(subQoS, msg.QoS)
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

		// !!! 这里要复制消息，因为不同客户端的 QoS 可能不同，不能影响原消息 !!!
		copiedMsg := msg.Copy() // 复制消息，不过 PublishPacket 仍是指针，没有深拷贝
		copiedMsg.QoS = cq.qos  // 设置实际发送的 QoS

		// 直接调用 Deliver，持久化逻辑在其中统一处理
		if err := cq.client.deliver(&copiedMsg); err != nil {
			if copiedMsg.QoS == packet.QoS0 {
				c.logger.Debug("QoS0 message delivery failed",
					zap.String("clientID", cq.client.ID),
					zap.String("topic", copiedMsg.Packet.Topic),
					zap.Error(err))
				c.stats.MessagesDropped.Add(1)
			} else {
				c.logger.Debug("QoS1 message delivery failed",
					zap.String("clientID", cq.client.ID),
					zap.String("topic", copiedMsg.Packet.Topic),
					zap.Error(err))
			}
		}
	}
}

// deliverToTarget 投递消息给指定的目标客户端
func (c *Core) deliverToTarget(msg *Message) {
	if msg.Packet.Properties == nil || msg.Packet.Properties.TargetClientID == "" {
		c.logger.Debug("No target client ID specified for targeted delivery",
			zap.String("topic", msg.Packet.Topic))

		return
	}

	targetID := msg.Packet.Properties.TargetClientID

	// 先尝试普通客户端
	c.clientsMu.RLock()
	client, exists := c.clients[targetID]
	c.clientsMu.RUnlock()

	if exists {
		if client.IsClosed() {
			c.stats.MessagesDropped.Add(1)

			c.logger.Debug("Target client is closed",
				zap.String("targetClientID", targetID),
				zap.String("topic", msg.Packet.Topic))
			return
		}

		if err := client.deliver(msg); err != nil {
			c.logger.Debug("Target delivery failed",
				zap.String("targetClientID", targetID),
				zap.String("topic", msg.Packet.Topic),
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
		zap.String("topic", msg.Packet.Topic))
	c.stats.MessagesDropped.Add(1)
}

// deliver 向客户端发送消息
// 流控策略：
// 1. 检查消息是否过期
// 2. QoS1 消息先持久化
// 3. 尝试放入发送队列：
//   - 成功：启动发送协程处理
//   - 失败（队列满）：
//   - QoS0: 直接丢弃
//   - QoS1: 已持久化，等待重传机制处理
func (c *Client) deliver(msg *Message) error {
	// PacketID 属于“投递层字段”，必须对每个目标客户端独立分配，且不能写回原始 PublishPacket 。
	if msg.QoS == packet.QoS1 {
		msg.PacketID = c.allocatePacketID()
	}

	// 检查消息是否过期（过期消息直接丢弃，无论 QoS）
	if msg.IsExpired() {
		c.core.stats.MessagesDropped.Add(1)
		return nil
	}

	// QoS 1: 先持久化到存储（确保不丢失）
	if msg.QoS == packet.QoS1 && c.core.messageStore != nil {
		if err := c.core.messageStore.save(c.ID, msg); err != nil {
			c.core.logger.Error("Failed to persist QoS1 message",
				zap.String("clientID", c.ID),
				zap.String("topic", msg.Packet.Topic),
				zap.Error(err))
			return err
		}
		c.core.logger.Debug("QoS1 message persisted",
			zap.String("clientID", c.ID),
			zap.String("topic", msg.Packet.Topic),
			zap.Uint16("packetID", msg.PacketID))
	}

	// 如果客户端关闭，直接返回
	if c.closed.Load() {
		if msg.QoS == packet.QoS0 {
			c.core.logger.Debug("QoS0 message dropped, client closed",
				zap.String("clientID", c.ID),
				zap.String("topic", msg.Packet.Topic))
			c.core.stats.MessagesDropped.Add(1)
			return errors.New("client closed")
		}
		// QoS1: 后续会持久化，重连后重传
		return nil
	}

	// 尝试放入发送队列
	if c.sendQueue.tryEnqueue(msg) {
		// 成功入队，触发发送协程
		c.triggerSend()
		return nil
	}

	// 队列已满
	if msg.QoS == packet.QoS0 {
		// QoS0: 直接丢弃
		c.core.logger.Debug("QoS0 message dropped, send queue full",
			zap.String("clientID", c.ID),
			zap.String("topic", msg.Packet.Topic))

		c.core.stats.MessagesDropped.Add(1)
		return errors.New("send queue full")
	}

	// QoS1: 已持久化，等待重传机制处理
	c.core.logger.Debug("QoS1 message queued for retransmit (queue full)",
		zap.String("clientID", c.ID),
		zap.String("topic", msg.Packet.Topic),
		zap.Uint16("packetID", msg.PacketID))
	return nil
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
		msg, ok := c.sendQueue.tryDequeue()
		if !ok {
			// 队列已空
			break
		}

		// 发送消息
		// !!! 传的是 msg 指针，因为 tryEnqueue 内部使用了指针
		if err := c.sendMessage(msg); err != nil {
			c.core.logger.Debug("Failed to send message from queue",
				zap.String("clientID", c.ID),
				zap.String("topic", msg.Packet.Topic),
				zap.Error(err))

			// 如果是 QoS1 且持久化失败，则已经在持久化层有备份
			// 如果是 QoS0，则丢弃
			if msg.QoS == packet.QoS0 {
				c.core.stats.MessagesDropped.Add(1)
			}
		}
	}
}

// pendingMessage 等待确认的消息
type pendingMessage struct {
	msg        *Message // 最终的 PUBLISH 包（只读）
	lastSentAt int64    // 最后发送时间（Unix 时间戳）
}

// sendMessage 发送消息到客户端
func (c *Client) sendMessage(msg *Message) error {
	// 基于只读 PublishPacket 构造一次性出站 packet
	if msg.Packet == nil {
		return ErrInvalidMessage
	}

	// 构建最终发送的 PUBLISH 包
	// !!! 这里要复制消息，因为要修改 QoS 和 PacketID 等字段，不能影响原始消息 !!!
	pub := msg.Packet.Copy()
	pub.Dup = msg.Dup
	pub.QoS = msg.QoS
	pub.Dup = msg.Dup
	if msg.QoS > 0 {
		pub.PacketID = msg.PacketID
	} else {
		pub.PacketID = 0
	}

	// 调用 OnDeliver 钩子，检查是否允许投递
	deliverCtx := &DeliverContext{
		ClientID: c.ID,
		Packet:   &pub,
	}
	if !c.core.options.Hooks.callOnDeliver(deliverCtx) {
		return ErrDeliveryRejected // 钩子拒绝投递
	}

	// 先发送消息
	if err := c.WritePacket(&pub); err != nil {
		return err
	}

	// 发送成功后处理 QoS 1
	if pub.QoS == packet.QoS1 {
		// 加入 pendingAck 等待确认
		c.pendingAckMu.Lock()
		c.pendingAck[pub.PacketID] = pendingMessage{
			msg:        msg,
			lastSentAt: time.Now().Unix(),
		}
		c.pendingAckMu.Unlock()
	}
	// QoS 0: 发送后即清理，无需等待确认

	// 发送成功，统计消息发送数
	c.core.stats.MessagesSent.Add(1)
	return nil
}

// PublishToClient 向指定客户端发送消息
// 如果客户端在线但未订阅该主题，仍然会将消息投递给该客户端
func (c *Core) PublishToClient(clientID string, topic string, payload []byte, options PublishOptions) error {
	pub := packet.NewPublishPacket(topic, payload)
	pub.QoS = options.QoS
	pub.Retain = options.Retain
	if options.Priority != nil {
		pub.Properties.Priority = options.Priority
	}
	pub.Properties.TraceID = options.TraceID
	pub.Properties.ContentType = options.ContentType
	pub.Properties.TargetClientID = options.TargetClientID
	pub.Properties.ResponseTopic = options.ResponseTopic
	pub.Properties.CorrelationData = options.CorrelationData

	msg := Message{Packet: pub, Timestamp: time.Now().Unix()}

	if options.Expiry > 0 {
		expiryTime := time.Now().Add(time.Duration(options.Expiry) * time.Second)
		msg.Packet.Properties.ExpiryTime = expiryTime.Unix()
	}

	c.clientsMu.RLock()
	client, exists := c.clients[clientID]
	c.clientsMu.RUnlock()

	if exists {
		if client.IsClosed() {
			return NewClientClosedError(clientID)
		}
		return client.deliver(&msg)
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

	pub := packet.NewPublishPacket(topic, payload)
	pub.QoS = options.QoS
	pub.Retain = options.Retain
	if options.Priority != nil {
		pub.Properties.Priority = options.Priority
	}
	pub.Properties.TraceID = options.TraceID
	pub.Properties.ContentType = options.ContentType
	pub.Properties.ResponseTopic = options.ResponseTopic
	pub.Properties.CorrelationData = options.CorrelationData

	msg := Message{Packet: pub, Timestamp: time.Now().Unix()}

	if options.Expiry > 0 {
		expiryTime := time.Now().Add(time.Duration(options.Expiry) * time.Second)
		msg.Packet.Properties.ExpiryTime = expiryTime.Unix()
	}

	successCount := 0
	for _, client := range clients {
		if err := client.deliver(&msg); err == nil {
			successCount++
		}
	}

	return successCount
}
