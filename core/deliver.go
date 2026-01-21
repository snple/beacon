package core

import (
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// publish 发布消息 (内部使用)
func (c *Core) publish(msg Message) bool {
	// 进入队列前重新分配 PacketID，以确保顺序和唯一性
	msg.PacketID = nson.NewId()

	// 处理保留消息
	if msg.Retain && c.options.RetainEnabled {
		if len(msg.Packet.Payload) == 0 {
			// 删除保留消息
			c.retainStore.remove(msg.Packet.Topic)
			// 从持久化存储中删除
			c.store.deleteRetain(msg.Packet.Topic)
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
			if c.store != nil {
				// !!! 这里传指针
				if err := c.store.setRetain(msg.Packet.Topic, &msg); err != nil {
					c.logger.Error("Failed to persist retain message",
						zap.String("topic", msg.Packet.Topic),
						zap.Error(err))
				}
			}
		}
	}

	// 放入消息队列（现在使用 Queue.Enqueue）
	if err := c.queue.Enqueue(&msg); err != nil {
		// 遇到错误，记录日志并丢弃消息
		c.logger.Error("message dropped, dispatch queue enqueue failed",
			zap.String("topic", msg.Packet.Topic),
			zap.String("packetID", msg.PacketID.Hex()),
			zap.Error(err))
		c.stats.MessagesDropped.Add(1)
		return false
	}

	// 成功入队，触发分发协程
	select {
	case c.queueTrigger <- struct{}{}:
	default:
		// 已有触发信号，无需重复发送
	}

	// 统计消息接收数
	c.stats.MessagesReceived.Add(1)
	return true
}

// PublishOptions 发布选项
type PublishOptions struct {
	QoS         packet.QoS
	Retain      bool
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
		case <-c.queueTrigger:
			for {
				// 从队列中取出消息并分发
				msg, err := c.queue.Dequeue()
				if err != nil {
					// 队列为空或其他错误，继续下一轮
					break
				}

				// 分发消息
				c.deliver(msg)
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

	subscribers := c.subTree.matchTopic(msg.Packet.Topic)

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
		// !!! 这里要复制消息，因为不同客户端的 QoS 可能不同，不能影响原消息 !!!
		copiedMsg := msg.Copy() // 复制消息，不过 PublishPacket 仍是指针，没有深拷贝
		copiedMsg.QoS = cq.qos  // 设置实际发送的 QoS

		// 直接调用 Deliver，持久化逻辑在其中统一处理
		// 注意：不要在这里检查 IsClosed()，因为：
		// 1. QoS1消息即使客户端离线也需要持久化
		// 2. client.deliver() 内部会处理离线情况
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
		// 直接调用deliver，不要在这里检查IsClosed
		// QoS1消息即使客户端离线也需要持久化
		if err := client.deliver(msg); err != nil {
			c.logger.Debug("Target delivery failed",
				zap.String("targetClientID", targetID),
				zap.String("topic", msg.Packet.Topic),
				zap.Error(err))
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
	// PacketID 属于“投递层字段”，必须对每个目标客户端独立分配
	// 无论 QoS=0 还是 QoS=1，都需要 PacketID
	msg.PacketID = nson.NewId()

	// 检查消息是否过期（过期消息直接丢弃，无论 QoS）
	if msg.IsExpired() {
		c.core.stats.MessagesDropped.Add(1)
		return nil
	}

	// QoS 0 和 QoS 1 都需要放入发送队列
	if err := c.queue.Enqueue(msg); err != nil {
		// 遇到错误，记录日志并丢弃消息
		c.core.logger.Debug("Message dropped, client dispatch queue enqueue failed",
			zap.String("clientID", c.ID),
			zap.String("topic", msg.Packet.Topic),
			zap.Error(err))
		c.core.stats.MessagesDropped.Add(1)
		return err
	}

	// 尝试放入发送队列
	conn := c.getConn()
	if conn == nil || conn.closed.Load() {
		// 客户端离线，但是已经放入了队列，等待重传机制处理
		c.core.logger.Debug("Client offline, message queued for retransmit",
			zap.String("clientID", c.ID),
			zap.String("topic", msg.Packet.Topic),
			zap.String("packetID", msg.PacketID.Hex()))

		return nil
	}

	if err := conn.sendQueue.tryEnqueue(msg); err != nil {
		// 失败：
		// !!! 由于上面重新分配了 PacketID，这里不可能出现消息已在队列中的情况 !!!

		// 队列已满
		if msg.QoS == packet.QoS0 {
			// QoS0: 虽然列队已满，但消息并没有直接丢弃，因为已经持久化到队列中，等待重传机制处理
			c.core.logger.Debug("QoS0 message dropped (queue full)",
				zap.String("clientID", c.ID),
				zap.String("topic", msg.Packet.Topic),
				zap.String("packetID", msg.PacketID.Hex()))

			return nil
		}

		// QoS1: 已持久化，等待重传机制处理
		c.core.logger.Debug("QoS1 message queued for retransmit (queue full)",
			zap.String("clientID", c.ID),
			zap.String("topic", msg.Packet.Topic),
			zap.String("packetID", msg.PacketID.Hex()))

		return err
	}

	// 成功入队，触发发送协程
	conn.triggerSend()

	return nil
}

// triggerSend 触发发送协程（非阻塞）
// 使用 processing 标志避免重复启动
func (c *conn) triggerSend() {
	if c.processing.CompareAndSwap(false, true) {
		go c.processSendQueue()
	}
}

// processSendQueue 处理发送队列中的消息
func (c *conn) processSendQueue() {
	defer c.processing.Store(false)

	for !c.closed.Load() {
		// 尝试从队列取消息
		msg, ok := c.sendQueue.tryDequeue()
		if !ok {
			// 队列已空
			break
		}

		// 检查消息是否过期（过期消息直接丢弃，无论 QoS）
		if msg.IsExpired() {
			c.client.core.stats.MessagesDropped.Add(1)
			continue
		}

		// 发送消息
		if err := c.sendMessage(msg); err != nil {
			c.client.core.logger.Debug("Failed to send message from queue",
				zap.String("clientID", c.client.ID),
				zap.String("topic", msg.Packet.Topic),
				zap.Error(err))

			// 如果是 QoS1 且持久化失败，则已经在持久化层有备份
			// 如果是 QoS0，这里也已经持久化到队列，等待重传机制处理
		}

		// 发送成功，统计消息发送数
		c.client.core.stats.MessagesSent.Add(1)
	}
}

// pendingMessage 等待确认的消息
type pendingMessage struct {
	msg        *Message // 最终的 PUBLISH 包（只读）
	lastSentAt int64    // 最后发送时间（Unix 时间戳）
}

// sendMessage 发送消息到客户端
func (c *conn) sendMessage(msg *Message) error {
	if msg.Packet == nil {
		return ErrInvalidMessage
	}

	// 基于只读 PublishPacket 构造一次性出站 packet

	// 构建最终发送的 PUBLISH 包
	// !!! 这里要复制消息，因为要修改 QoS 和 PacketID 等字段，不能影响原始消息 !!!
	pub := msg.Packet.Copy()
	pub.Dup = msg.Dup
	pub.QoS = msg.QoS
	pub.PacketID = msg.PacketID // 无论 QoS=0 还是 QoS=1 都使用 PacketID

	// 调用 OnDeliver 钩子，检查是否允许投递
	deliverCtx := &DeliverContext{
		ClientID: c.client.ID,
		Packet:   &pub,
	}
	if !c.client.core.options.Hooks.callOnDeliver(deliverCtx) {
		return ErrDeliveryRejected // 钩子拒绝投递
	}

	// 发送消息
	if err := c.writePacket(&pub); err != nil {
		return err
	}

	// 发送成功后的处理
	if pub.QoS == packet.QoS1 {
		// QoS=1: 加入 pendingAck 等待确认
		// 注意：如果是重传的消息，pendingAck 中可能已经存在该记录
		// 此时需要更新 lastSentAt 为实际发送时间
		c.client.session.pendingAckMu.Lock()
		c.client.session.pendingAck[pub.PacketID] = pendingMessage{
			msg:        msg,
			lastSentAt: time.Now().Unix(),
		}
		c.client.session.pendingAckMu.Unlock()
	} else {
		// QoS=0: TCP 写成功就从发送队列删除
		if err := c.client.queue.Delete(pub.PacketID); err != nil {
			c.client.core.logger.Debug("Failed to delete QoS0 message from queue after send",
				zap.String("clientID", c.client.ID),
				zap.String("packetID", pub.PacketID.Hex()),
				zap.Error(err))
		}
	}

	return nil
}

// PublishToClient 向指定客户端发送消息
// 如果客户端在线但未订阅该主题，仍然会将消息投递给该客户端
func (c *Core) PublishToClient(clientID string, topic string, payload []byte, options PublishOptions) error {
	pub := packet.NewPublishPacket(topic, payload)
	pub.QoS = options.QoS
	pub.Retain = options.Retain

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
		if client.Closed() {
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
		if !client.Closed() {
			clients = append(clients, client)
		}
	}
	c.clientsMu.RUnlock()

	pub := packet.NewPublishPacket(topic, payload)
	pub.QoS = options.QoS
	pub.Retain = options.Retain

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
