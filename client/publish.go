package client

import (
	"time"

	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// Publish 发布消息（使用 Builder 模式选项，推荐）
//
// 用法：
//
//	err := c.Publish("topic", payload,
//	    client.NewPublishOptions().
//	        WithQoS(packet.QoS1).
//	        WithRetain(true).
//	        WithTraceID("trace-123"),
//	)
func (c *Client) Publish(topic string, payload []byte, opts *PublishOptions) error {
	if !c.connected.Load() {
		return ErrNotConnected
	}

	if opts == nil {
		opts = NewPublishOptions()
	}

	// 确定超时时间：优先使用选项指定的，否则使用配置默认值
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = c.options.PublishTimeout
	}
	if timeout == 0 {
		timeout = defaultPublishTimeout
	}

	// 计算绝对过期时间戳
	var expiryTime int64
	if opts.Expiry > 0 {
		expiryTime = time.Now().Unix() + int64(opts.Expiry)
	}

	// 构建消息
	msg := &Message{
		Topic:           topic,
		Payload:         payload,
		QoS:             opts.QoS,
		Retain:          opts.Retain,
		Priority:        opts.Priority,
		TraceID:         opts.TraceID,
		ContentType:     opts.ContentType,
		ExpiryTime:      expiryTime,
		UserProperties:  opts.UserProperties,
		TargetClientID:  opts.TargetClientID,
		ResponseTopic:   opts.ResponseTopic,
		CorrelationData: opts.CorrelationData,
	}

	return c.publishMessage(msg, opts.QoS, timeout)
}

func (c *Client) PublishToClient(targetClientID, topic string, payload []byte, opts *PublishOptions) error {
	if opts == nil {
		opts = NewPublishOptions()
	}

	return c.Publish(topic, payload, opts.WithTargetClientID(targetClientID))
}

func (c *Client) PublishToCore(topic string, payload []byte, opts *PublishOptions) error {
	if opts == nil {
		opts = NewPublishOptions()
	}

	return c.Publish(topic, payload, opts.WithTargetClientID(packet.TargetToCore))
}

// publishMessage 内部发布消息方法
func (c *Client) publishMessage(msg *Message, qos packet.QoS, timeout time.Duration) error {
	// QoS 1: 分配 PacketID
	if qos == packet.QoS1 {
		msg.PacketID = c.allocatePacketID()

		// QoS 1: 先持久化到存储（确保不丢失）
		if c.store != nil {
			storedMsg := &StoredMessage{
				PacketID:        msg.PacketID,
				Topic:           msg.Topic,
				Payload:         msg.Payload,
				QoS:             msg.QoS,
				Retain:          msg.Retain,
				Priority:        msg.Priority,
				TraceID:         msg.TraceID,
				ContentType:     msg.ContentType,
				UserProperties:  msg.UserProperties,
				ExpiryTime:      msg.ExpiryTime,
				EnqueueTime:     time.Now().Unix(),
				TargetClientID:  msg.TargetClientID,
				ResponseTopic:   msg.ResponseTopic,
				CorrelationData: msg.CorrelationData,
			}
			if err := c.store.Save(storedMsg); err != nil {
				c.logger.Error("Failed to persist QoS1 message",
					zap.String("topic", msg.Topic),
					zap.Uint16("packetID", msg.PacketID),
					zap.Error(err))
				// 即使持久化失败，仍然尝试发送
			}
		}
	}

	// 构建队列消息
	queueMsg := &QueuedMessage{
		Message:     msg,
		QoS:         qos,
		EnqueueTime: time.Now().Unix(),
	}

	// 尝试放入发送队列
	if c.sendQueue != nil {
		if !c.sendQueue.TryEnqueue(queueMsg) {
			// 队列已满
			if qos == packet.QoS0 {
				// QoS0: 直接丢弃
				c.logger.Warn("Send queue full, QoS0 message dropped",
					zap.String("topic", msg.Topic),
					zap.Int("queueUsed", c.sendQueue.Used()),
					zap.Int("queueCapacity", c.sendQueue.Capacity()))
				return ErrSendQueueFull
			}
			// QoS1: 等待重传机制处理
			c.logger.Debug("Send queue full, QoS1 message queued for retransmit",
				zap.String("topic", msg.Topic),
				zap.Uint16("packetID", msg.PacketID))
			return nil // 不返回错误
		}

		// 触发发送协程
		c.triggerSend()

		// QoS 1: 等待确认
		if qos == packet.QoS1 {
			ch := make(chan error, 1)
			c.pendingAckMu.Lock()
			c.pendingAck[msg.PacketID] = ch
			c.pendingAckMu.Unlock()

			timeoutCh := time.After(timeout)
			select {
			case err := <-ch:
				return err
			case <-timeoutCh:
				c.pendingAckMu.Lock()
				delete(c.pendingAck, msg.PacketID)
				c.pendingAckMu.Unlock()
				return ErrPublishTimeout
			case <-c.ctx.Done():
				return c.ctx.Err()
			}
		}

		return nil
	}

	// 没有发送队列（不应该发生），直接发送
	return c.sendMessageDirect(msg)
}

// triggerSend 触发发送协程（非阻塞）
// 使用 processing 标志避免重复启动
func (c *Client) triggerSend() {
	if c.processing.CompareAndSwap(false, true) {
		go c.processSendQueue()
	}
}

// processSendQueue 处理发送队列中的消息
func (c *Client) processSendQueue() {
	defer c.processing.Store(false)

	for c.connected.Load() {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// 尝试从队列取消息
		qm, ok := c.sendQueue.TryDequeue()
		if !ok {
			// 队列为空，退出
			return
		}

		// 检查消息是否过期
		if qm.Message.ExpiryTime > 0 && time.Now().Unix() > qm.Message.ExpiryTime {
			c.logger.Debug("Message expired, dropping",
				zap.String("topic", qm.Message.Topic),
				zap.Uint16("packetID", qm.Message.PacketID))
			continue
		}

		// 发送消息
		if err := c.sendMessage(qm); err != nil {
			c.logger.Warn("Failed to send message from queue",
				zap.Error(err),
				zap.String("topic", qm.Message.Topic),
				zap.Uint16("packetID", qm.Message.PacketID))
		}
	}
}

// sendMessageDirect 直接发送消息到 core（不经过队列）
func (c *Client) sendMessageDirect(msg *Message) error {
	pub := packet.NewPublishPacket(msg.Topic, msg.Payload)
	pub.QoS = msg.QoS
	pub.Retain = msg.Retain
	pub.PacketID = msg.PacketID

	// 属性
	pub.Properties.Priority = &msg.Priority
	pub.Properties.TraceID = msg.TraceID
	pub.Properties.ContentType = msg.ContentType
	pub.Properties.ExpiryTime = msg.ExpiryTime
	pub.Properties.UserProperties = msg.UserProperties
	pub.Properties.TargetClientID = msg.TargetClientID
	pub.Properties.ResponseTopic = msg.ResponseTopic
	pub.Properties.CorrelationData = msg.CorrelationData

	return c.writePacket(pub)
}

// Subscribe 订阅主题（轮询模式，通过 PollMessage 获取消息）
//
// 用法：
//
//	err := c.Subscribe("topic1", "topic2")
//	// 然后通过 PollMessage 获取消息
func (c *Client) Subscribe(topics ...string) error {
	return c.SubscribeWithOptions(topics, nil)
}

// SubscribeWithOptions 订阅主题（带选项）
func (c *Client) SubscribeWithOptions(topics []string, opts *SubscribeOptions) error {
	if !c.connected.Load() {
		return ErrNotConnected
	}

	if len(topics) == 0 {
		return ErrTopicsEmpty
	}

	if opts == nil {
		opts = NewSubscribeOptions()
	}

	sub := packet.NewSubscribePacket(c.allocatePacketID())
	for _, topic := range topics {
		sub.AddSubscription(topic, opts.QoS)
	}

	// **重要**: 在发送 SUBSCRIBE 之前就记录主题
	// 这样当服务端发送保留消息时，消息队列已经就绪
	// 如果 SUBACK 失败，我们再回滚
	c.subscribedTopicsMu.Lock()
	for _, topic := range topics {
		c.subscribedTopics[topic] = true
	}
	c.subscribedTopicsMu.Unlock()

	// 等待确认
	ch := make(chan error, 1)
	c.pendingAckMu.Lock()
	c.pendingAck[sub.PacketID] = ch
	c.pendingAckMu.Unlock()

	if err := c.writePacket(sub); err != nil {
		// 发送失败，回滚订阅
		c.subscribedTopicsMu.Lock()
		for _, topic := range topics {
			delete(c.subscribedTopics, topic)
		}
		c.subscribedTopicsMu.Unlock()
		c.pendingAckMu.Lock()
		delete(c.pendingAck, sub.PacketID)
		c.pendingAckMu.Unlock()
		return err
	}

	select {
	case err := <-ch:
		if err != nil {
			// SUBACK 失败，回滚订阅
			c.subscribedTopicsMu.Lock()
			for _, topic := range topics {
				delete(c.subscribedTopics, topic)
			}
			c.subscribedTopicsMu.Unlock()
			return err
		}
		return nil
	case <-time.After(30 * time.Second):
		// 超时，回滚订阅
		c.subscribedTopicsMu.Lock()
		for _, topic := range topics {
			delete(c.subscribedTopics, topic)
		}
		c.subscribedTopicsMu.Unlock()
		c.pendingAckMu.Lock()
		delete(c.pendingAck, sub.PacketID)
		c.pendingAckMu.Unlock()
		return ErrSubscribeTimeout
	case <-c.ctx.Done():
		// 取消，回滚订阅
		c.subscribedTopicsMu.Lock()
		for _, topic := range topics {
			delete(c.subscribedTopics, topic)
		}
		c.subscribedTopicsMu.Unlock()
		return c.ctx.Err()
	}
}

// Unsubscribe 取消订阅
func (c *Client) Unsubscribe(topics ...string) error {
	if !c.connected.Load() {
		return ErrNotConnected
	}

	unsub := packet.NewUnsubscribePacket(c.allocatePacketID())
	unsub.Topics = topics

	// 等待确认
	ch := make(chan error, 1)
	c.pendingAckMu.Lock()
	c.pendingAck[unsub.PacketID] = ch
	c.pendingAckMu.Unlock()

	if err := c.writePacket(unsub); err != nil {
		c.pendingAckMu.Lock()
		delete(c.pendingAck, unsub.PacketID)
		c.pendingAckMu.Unlock()
		return err
	}

	select {
	case err := <-ch:
		if err != nil {
			return err
		}
		// 确认成功后移除订阅记录
		c.subscribedTopicsMu.Lock()
		for _, topic := range topics {
			delete(c.subscribedTopics, topic)
		}
		c.subscribedTopicsMu.Unlock()
		return nil
	case <-time.After(30 * time.Second):
		c.pendingAckMu.Lock()
		delete(c.pendingAck, unsub.PacketID)
		c.pendingAckMu.Unlock()
		return ErrUnsubscribeTimeout
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

func (c *Client) handlePublish(p *packet.PublishPacket) {
	// 调用 OnPublish 钩子（先检查钩子，避免不必要的数据拷贝）
	if !c.options.Hooks.callOnPublish(&PublishContext{
		ClientID: c.clientID,
		Packet:   p,
	}) {
		// 钩子返回 false，丢弃消息
		c.logger.Debug("Message rejected by hook",
			zap.String("topic", p.Topic))

		// QoS 1: 仍然发送 ACK（避免重传）
		if p.QoS == packet.QoS1 {
			puback := packet.NewPubackPacket(p.PacketID, packet.ReasonSuccess)
			c.writePacket(puback)
		}
		return
	}

	// 钩子通过后，创建消息
	msg := &Message{
		Topic:    p.Topic,
		Payload:  p.Payload,
		QoS:      p.QoS,
		Retain:   p.Retain,
		PacketID: p.PacketID,
	}

	if p.Properties != nil {
		if p.Properties.Priority != nil {
			msg.Priority = *p.Properties.Priority
		}
		msg.TraceID = p.Properties.TraceID
		msg.ContentType = p.Properties.ContentType
		msg.ExpiryTime = p.Properties.ExpiryTime
		msg.UserProperties = p.Properties.UserProperties
		msg.TargetClientID = p.Properties.TargetClientID
		msg.SourceClientID = p.Properties.SourceClientID
		msg.ResponseTopic = p.Properties.ResponseTopic
		msg.CorrelationData = p.Properties.CorrelationData
	}

	// 轮询模式：检查是否有消息队列，如果有则放入队列
	c.messageQueueMu.Lock()
	if c.messageQueue == nil {
		c.messageQueueMu.Unlock()

		// 没有消息队列，记录警告并丢弃消息
		c.logger.Warn("Message received but no queue available",
			zap.String("topic", p.Topic))

		// QoS 1: 仍然发送 ACK（避免重传）
		if p.QoS == packet.QoS1 {
			puback := packet.NewPubackPacket(p.PacketID, packet.ReasonSuccess)
			c.writePacket(puback)
		}
		return
	}
	c.messageQueueMu.Unlock()

	// 非阻塞尝试放入队列
	select {
	case c.messageQueue <- msg:
		// 成功放入队列
		c.logger.Debug("Enqueued message for polling",
			zap.String("topic", p.Topic))

		// QoS 1: 成功入队后发送 ACK
		if p.QoS == packet.QoS1 {
			puback := packet.NewPubackPacket(p.PacketID, packet.ReasonSuccess)
			c.writePacket(puback)
		}
	default:
		// 队列已满
		c.logger.Warn("Message queue full, message dropped",
			zap.String("topic", p.Topic))

		// QoS 1: 不发送 ACK，让 core 重传
		// QoS 0: 直接丢弃
	}
}

// handlePuback 处理 PUBACK
func (c *Client) handlePuback(p *packet.PubackPacket) {
	// 将 ReasonCode 转换为 error 传递给调用者
	var err error
	if p.ReasonCode != packet.ReasonSuccess {
		err = NewPublishWarningError(p.ReasonCode.String())
	}

	// 从持久化存储中删除已确认的消息
	if c.store != nil {
		if delErr := c.store.Delete(p.PacketID); delErr != nil {
			c.logger.Warn("Failed to delete persisted message after ACK",
				zap.Uint16("packetID", p.PacketID),
				zap.Error(delErr))
		}
	}

	c.handleAck(p.PacketID, err)
}

// handleSuback 处理 SUBACK
func (c *Client) handleSuback(p *packet.SubackPacket) {
	var err error
	// 检查是否有任何订阅失败
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
func (c *Client) handleUnsuback(p *packet.UnsubackPacket) {
	var err error
	// 检查是否有任何取消订阅失败
	for i, code := range p.ReasonCodes {
		if code != packet.ReasonSuccess {
			err = NewUnsubscriptionError(i, code.String())
			break
		}
	}
	c.handleAck(p.PacketID, err)
}

func (c *Client) handleAck(packetID uint16, err error) {
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
