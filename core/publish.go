package core

import (
	"time"

	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

func (c *Client) handlePublish(pub *packet.PublishPacket) error {
	// 验证主题名（发布时不允许通配符）
	if !packet.ValidateTopicName(pub.Topic) {
		c.core.logger.Warn("Invalid topic name", zap.String("clientID", c.ID), zap.String("topic", pub.Topic))
		if pub.QoS == packet.QoS1 {
			puback := packet.NewPubackPacket(pub.PacketID, packet.ReasonTopicNameInvalid)
			c.WritePacket(puback)
		}
		return nil
	}

	// 验证 QoS (只支持 0 和 1)
	if !pub.QoS.IsValid() {
		c.core.logger.Warn("Invalid QoS level", zap.String("clientID", c.ID), zap.Uint8("qos", uint8(pub.QoS)))
		puback := packet.NewPubackPacket(pub.PacketID, packet.ReasonQoSNotSupported)
		c.WritePacket(puback)
		return nil // 忽略无效 QoS 的消息
	}

	// 先调用 OnPublish 钩子（此时可以访问原始 packet）
	pubCtx := &PublishContext{
		ClientID: c.ID,
		Packet:   pub,
	}

	if err := c.core.options.Hooks.callOnPublish(pubCtx); err != nil {
		c.core.logger.Debug("MessageHandler.OnPublish error",
			zap.String("clientID", c.ID),
			zap.String("topic", pub.Topic),
			zap.Error(err))
		return nil
	}

	// 由于后面要填充 SourceClientID，需要检查 Properties 是否为 nil
	if pub.Properties == nil {
		pub.Properties = packet.NewPublishProperties()
	}

	// 填充 SourceClientID（发送者标识，由 core 填充确保可信）
	pub.Properties.SourceClientID = c.ID

	// 如果客户端没有指定过期时间或者指定了错误的值，使用 core 默认值
	if pub.Properties.ExpiryTime <= 0 && c.core.options.DefaultMessageExpiry > 0 {
		expiry := time.Now().Add(c.core.options.DefaultMessageExpiry)
		pub.Properties.ExpiryTime = expiry.Unix()
	}

	msg := Message{
		Packet:    pub,
		Dup:       false,
		QoS:       pub.QoS,
		Retain:    pub.Retain,
		PacketID:  pub.PacketID,
		Timestamp: time.Now().Unix(),
	}

	// 如果消息已经过期，直接丢弃
	if msg.IsExpired() {
		c.core.logger.Debug("Message expired upon arrival",
			zap.String("clientID", c.ID),
			zap.String("topic", pub.Topic))
		if pub.QoS == packet.QoS1 {
			puback := packet.NewPubackPacket(pub.PacketID, packet.ReasonMessageExpired)
			c.WritePacket(puback)
		}
		return nil
	}

	// QoS 0: 直接发布
	// QoS 1: 发布并发送 PUBACK
	c.core.publish(msg)

	if msg.QoS == packet.QoS1 {
		puback := packet.NewPubackPacket(msg.PacketID, packet.ReasonSuccess)
		if err := c.WritePacket(puback); err != nil {
			c.core.logger.Warn("Failed to send PUBACK",
				zap.String("clientID", c.ID),
				zap.Uint16("packetID", msg.PacketID),
				zap.Error(err))

			return err
		}
	}

	return nil
}

// handlePuback 处理 QoS 1 确认 - 客户端已收到消息
func (c *Client) handlePuback(p *packet.PubackPacket) error {
	c.pendingAckMu.Lock()
	pending, ok := c.pendingAck[p.PacketID]
	if ok {
		delete(c.pendingAck, p.PacketID)
	}
	c.pendingAckMu.Unlock()

	if ok {
		c.core.logger.Debug("Message acknowledged",
			zap.String("clientID", c.ID),
			zap.Uint16("packetID", p.PacketID),
			zap.String("topic", pending.msg.Packet.Topic),
			zap.Uint8("reasonCode", uint8(p.ReasonCode)))

		// 收到 PUBACK 即表示消息已送达，删除持久化消息
		if c.core.messageStore != nil {
			if err := c.core.messageStore.delete(c.ID, pending.msg.PacketID); err != nil {
				c.core.logger.Warn("Failed to delete acknowledged message from storage",
					zap.String("clientID", c.ID),
					zap.Error(err))
			}
		}
	}
	return nil
}

func (c *Client) handleSubscribe(p *packet.SubscribePacket) error {
	suback := packet.NewSubackPacket(p.PacketID)

	// 验证并处理每个订阅
	validSubs := make([]packet.Subscription, 0, len(p.Subscriptions))
	isNewSubs := make([]bool, 0, len(p.Subscriptions)) // 记录每个订阅是否是新订阅

	for _, sub := range p.Subscriptions {
		// 验证主题过滤器
		if !packet.ValidateTopicFilter(sub.Topic) {
			suback.ReasonCodes = append(suback.ReasonCodes, packet.ReasonTopicFilterInvalid)
			c.core.logger.Warn("Invalid topic filter",
				zap.String("clientID", c.ID),
				zap.String("topic", sub.Topic))
			continue
		}

		// 验证 QoS（只支持 0 和 1）
		if !sub.Options.QoS.IsValid() {
			suback.ReasonCodes = append(suback.ReasonCodes, packet.ReasonQoSNotSupported)
			c.core.logger.Warn("Unsupported QoS",
				zap.String("clientID", c.ID),
				zap.Uint8("qos", uint8(sub.Options.QoS)))
			continue
		}

		// 调用 OnSubscribe 钩子
		subCtx := &SubscribeContext{
			ClientID:     c.ID,
			Packet:       p,
			Subscription: &sub,
		}
		if err := c.core.options.Hooks.callOnSubscribe(subCtx); err != nil {
			suback.ReasonCodes = append(suback.ReasonCodes, packet.ReasonNotAuthorized)
			c.core.logger.Debug("Subscribe rejected by hook",
				zap.String("clientID", c.ID),
				zap.String("topic", sub.Topic),
				zap.Error(err))
			continue
		}

		// 检查是否是新订阅（在添加之前检查）
		c.subsMu.RLock()
		_, exists := c.subscriptions[sub.Topic]
		c.subsMu.RUnlock()
		isNew := !exists

		validSubs = append(validSubs, sub)
		isNewSubs = append(isNewSubs, isNew)
		suback.ReasonCodes = append(suback.ReasonCodes, packet.ReasonCode(sub.Options.QoS))
	}

	// 合并锁操作，一次性添加所有有效订阅
	if len(validSubs) > 0 {
		c.subsMu.Lock()
		for _, sub := range validSubs {
			c.subscriptions[sub.Topic] = sub.Options
		}
		c.subsMu.Unlock()

		// 在锁外处理 core 订阅和日志
		for _, sub := range validSubs {
			c.core.Subscribe(c.ID, sub)
			c.core.logger.Debug("Client subscribed",
				zap.String("clientID", c.ID),
				zap.String("topic", sub.Topic),
				zap.Uint8("qos", uint8(sub.Options.QoS)))
		}
	}

	// 先发送 SUBACK，确保客户端收到确认后再发送保留消息
	if err := c.WritePacket(suback); err != nil {
		return err
	}

	// 在 SUBACK 之后发送保留消息
	// 这样客户端收到 SUBACK 后才会注册 handler，然后才能处理保留消息
	for i, sub := range validSubs {
		// 逐条发送保留消息，避免一次性加载所有消息到内存
		c.sendRetainedMessages(
			sub.Topic,
			isNewSubs[i],
			sub.Options.RetainAsPublished,
			sub.Options.RetainHandling,
			sub.Options.QoS,
		)
	}

	return nil
}

func (c *Client) handleUnsubscribe(p *packet.UnsubscribePacket) error {
	unsuback := packet.NewUnsubackPacket(p.PacketID)

	// 验证并处理每个取消订阅
	validTopics := make([]string, 0, len(p.Topics))
	for _, topic := range p.Topics {
		// 验证主题过滤器
		if !packet.ValidateTopicFilter(topic) {
			unsuback.ReasonCodes = append(unsuback.ReasonCodes, packet.ReasonTopicFilterInvalid)
			c.core.logger.Warn("Invalid topic filter for unsubscribe",
				zap.String("clientID", c.ID),
				zap.String("topic", topic))
			continue
		}

		// 调用 OnUnsubscribe 钩子
		unsubCtx := &UnsubscribeContext{
			ClientID: c.ID,
			Packet:   p,
			Topic:    topic,
		}
		c.core.options.Hooks.callOnUnsubscribe(unsubCtx)

		validTopics = append(validTopics, topic)
		unsuback.ReasonCodes = append(unsuback.ReasonCodes, packet.ReasonSuccess)
	}

	// 合并锁操作
	if len(validTopics) > 0 {
		c.subsMu.Lock()
		for _, topic := range validTopics {
			delete(c.subscriptions, topic)
		}
		c.subsMu.Unlock()

		// 在锁外处理 core 操作和日志
		for _, topic := range validTopics {
			c.core.Unsubscribe(c.ID, topic)
			c.core.logger.Debug("Client unsubscribed", zap.String("clientID", c.ID), zap.String("topic", topic))
		}
	}

	return c.WritePacket(unsuback)
}

// sendRetainedMessages 逐条发送订阅时的保留消息
// 避免一次性加载所有消息到内存
func (c *Client) sendRetainedMessages(topic string, isNewSubscription bool,
	retainAsPublished bool, retainHandling uint8, subQoS packet.QoS) {
	// retainHandling == 2: 不发送保留消息
	if retainHandling == 2 {
		return
	}

	// retainHandling == 1: 仅新订阅时发送
	if retainHandling == 1 && !isNewSubscription {
		return
	}

	// 获取匹配的主题列表（只是索引，不包含消息体）
	topics := c.core.retainStore.matchTopics(topic)
	if len(topics) == 0 {
		return
	}

	if c.core.messageStore == nil {
		return
	}

	c.core.logger.Debug("Sending retained messages",
		zap.String("clientID", c.ID),
		zap.String("topic", topic),
		zap.Int("count", len(topics)),
		zap.Bool("isNewSubscription", isNewSubscription),
		zap.Uint8("retainHandling", retainHandling),
		zap.Bool("retainAsPublished", retainAsPublished))

	// 逐条从 messageStore 获取消息并发送
	for _, t := range topics {
		msg, err := c.core.messageStore.getRetainMessage(t)
		if err != nil {
			c.core.logger.Debug("Failed to get retain message from store",
				zap.String("topic", t),
				zap.Error(err))
			continue
		}
		if msg == nil {
			continue
		}

		// !!! 这里可以不复制消息，直接修改 Retain 和 QoS 后投递，因为消息是从持久化存储中读取的，不会影响其他地方 !!!

		// 根据 retainAsPublished 设置 Retain 标志
		if !retainAsPublished {
			msg.Retain = false
		}

		// 应用 QoS 降级：取订阅 QoS 和消息 QoS 的较小值
		msg.QoS = min(subQoS, msg.QoS)
		c.deliver(msg)
	}
}
