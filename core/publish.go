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
			if err := c.writePacket(puback); err != nil {
				c.core.logger.Error("Failed to send PUBACK",
					zap.String("clientID", c.ID),
					zap.String("packetID", pub.PacketID.Hex()),
					zap.Error(err))

				return err
			}
		}
		return nil
	}

	// 验证 QoS (只支持 0 和 1)
	if !pub.QoS.IsValid() {
		c.core.logger.Warn("Invalid QoS level", zap.String("clientID", c.ID), zap.Uint8("qos", uint8(pub.QoS)))
		puback := packet.NewPubackPacket(pub.PacketID, packet.ReasonQoSNotSupported)
		if err := c.writePacket(puback); err != nil {
			c.core.logger.Error("Failed to send PUBACK",
				zap.String("clientID", c.ID),
				zap.String("packetID", pub.PacketID.Hex()),
				zap.Error(err))
			return err
		}

		return nil // 忽略无效 QoS 的消息
	}

	// 每客户端发布速率限制（令牌桶）。
	// 放在钩子/入队之前，避免超限客户端消耗后续处理资源。
	if c.publishLimiter != nil && !c.publishLimiter.Allow() {
		c.core.stats.MessagesRejected.Add(1)
		c.messagesRejected.Add(1)
		c.core.emit(Event{
			Type:     EventMessageRejected,
			ClientID: c.ID,
			Topic:    pub.Topic,
			Bytes:    len(pub.Payload),
			QoS:      pub.QoS,
			Reason:   packet.ReasonMessageRateTooHigh.String(),
		})
		c.core.logger.Warn("Publish rate exceeded",
			zap.String("clientID", c.ID),
			zap.String("topic", pub.Topic),
			zap.Float64("maxPublishRate", c.core.options.MaxPublishRate))

		if pub.QoS == packet.QoS1 {
			puback := packet.NewPubackPacket(pub.PacketID, packet.ReasonMessageRateTooHigh)
			if err := c.writePacket(puback); err != nil {
				c.core.logger.Error("Failed to send PUBACK after rate limit rejection",
					zap.String("clientID", c.ID),
					zap.String("packetID", pub.PacketID.Hex()),
					zap.Error(err))
				return err
			}
		}
		return nil
	}

	// 先调用 OnPublish 钩子（此时可以访问原始 packet）
	pubCtx := &PublishContext{
		ClientID: c.ID,
		Packet:   pub,
		Identity: c.session.getIdentity(),
	}

	if err := c.core.options.Hooks.callOnPublish(pubCtx); err != nil {
		c.core.logger.Debug("MessageHandler.OnPublish error",
			zap.String("clientID", c.ID),
			zap.String("topic", pub.Topic),
			zap.Error(err))

		// 明确拒绝：必须回 PUBACK，否则发布端会一直把它当作未确认消息重传。
		c.core.stats.MessagesRejected.Add(1)
		c.messagesRejected.Add(1)
		c.core.emit(Event{
			Type:     EventMessageRejected,
			ClientID: c.ID,
			Topic:    pub.Topic,
			Bytes:    len(pub.Payload),
			QoS:      pub.QoS,
			Reason:   packet.ReasonNotAuthorized.String(),
		})
		if pub.QoS == packet.QoS1 {
			puback := packet.NewPubackPacket(pub.PacketID, packet.ReasonNotAuthorized)
			if err := c.writePacket(puback); err != nil {
				c.core.logger.Error("Failed to send PUBACK after publish rejection",
					zap.String("clientID", c.ID),
					zap.String("packetID", pub.PacketID.Hex()),
					zap.Error(err))
				return err
			}
		}

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
			if err := c.writePacket(puback); err != nil {
				c.core.logger.Error("Failed to send PUBACK",
					zap.String("clientID", c.ID),
					zap.String("packetID", pub.PacketID.Hex()),
					zap.Error(err))
				return err
			}
		}
		return nil
	}

	// QoS 0: 直接发布
	// QoS 1: 发布并发送 PUBACK
	publishSuccess := c.core.publish(msg)

	if publishSuccess {
		// 发布成功
		if msg.QoS == packet.QoS1 {
			puback := packet.NewPubackPacket(msg.PacketID, packet.ReasonSuccess)
			if err := c.writePacket(puback); err != nil {
				c.core.logger.Error("Failed to send PUBACK",
					zap.String("clientID", c.ID),
					zap.String("packetID", msg.PacketID.Hex()),
					zap.Error(err))

				return err
			}
		}
	} else {
		// 发布失败
		if msg.QoS == packet.QoS1 {
			// 对于 QoS 1，仍然发送 PUBACK，表示已接收但无法处理
			puback := packet.NewPubackPacket(msg.PacketID, packet.ReasonQuotaExceeded)
			if err := c.writePacket(puback); err != nil {
				c.core.logger.Error("Failed to send PUBACK after publish failure",
					zap.String("clientID", c.ID),
					zap.String("packetID", msg.PacketID.Hex()),
					zap.Error(err))

				return err
			}
		} else {
			// 对于 QoS 0，丢弃消息
			c.core.logger.Debug("QoS 0 message dropped due to publish failure",
				zap.String("clientID", c.ID),
				zap.String("topic", pub.Topic))
		}
	}

	return nil
}

// handlePuback 处理 QoS 1 确认 - 客户端已收到消息
func (c *Client) handlePuback(p *packet.PubackPacket) error {
	c.session.pendingAckMu.Lock()
	pending, ok := c.session.pendingAck[p.PacketID]
	var delErr error
	if ok {
		// 在同一个临界区内完成"删除持久化条目 + 移除 pendingAck"：
		// 补货逻辑会在同一把锁内检查 pendingAck 并入队，二者互斥，
		// 避免 ACK 与补货交错时把同一条消息重复装载。
		delErr = c.queue.Delete(p.PacketID)
		delete(c.session.pendingAck, p.PacketID)
	}
	c.session.pendingAckMu.Unlock()

	if delErr != nil {
		c.core.logger.Debug("Failed to delete acknowledged message from queue",
			zap.String("clientID", c.ID),
			zap.String("packetID", p.PacketID.Hex()),
			zap.Error(delErr))
	}

	if ok {
		topic := ""
		if pending.msg != nil && pending.msg.Packet != nil {
			topic = pending.msg.Packet.Topic
		}

		if p.ReasonCode.IsError() {
			// 订阅端明确拒绝/无法处理：消息不会重试，必须留下可见记录。
			c.core.logger.Warn("Message rejected by subscriber",
				zap.String("clientID", c.ID),
				zap.String("packetID", p.PacketID.Hex()),
				zap.String("topic", topic),
				zap.String("reason", p.ReasonCode.String()))
			c.core.stats.MessagesDropped.Add(1)
		} else {
			c.core.logger.Debug("Message acknowledged",
				zap.String("clientID", c.ID),
				zap.String("packetID", p.PacketID.Hex()),
				zap.String("topic", topic),
				zap.Uint8("reasonCode", uint8(p.ReasonCode)))
		}

		// ACK 释放了处理窗口：立即补货，避免积压消息等待 RetransmitInterval 才被装载
		c.kickRefill()
	}
	return nil
}

func (c *Client) handleSubscribe(p *packet.SubscribePacket) error {
	suback := packet.NewSubackPacket(p.PacketID)

	// 验证并处理每个订阅
	validSubs := make([]packet.Subscription, 0, len(p.Subscriptions))
	isNewSubs := make([]bool, 0, len(p.Subscriptions))            // 记录每个订阅是否是新订阅
	replayRetained := make([]bool, 0, len(p.Subscriptions))       // 是否重放保留消息（同包内重复过滤器只在首次出现时重放）
	seenTopics := make(map[string]struct{}, len(p.Subscriptions)) // 同一 SUBSCRIBE 包内的去重

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

		// RetainHandling 只有 0/1/2 三个合法值，线格式是 2 位，对端可以传 3。
		// 不能把未知值当作 0 处理，否则会静默发送本不该发送的保留消息。
		if !sub.Options.ValidRetainHandling() {
			suback.ReasonCodes = append(suback.ReasonCodes, packet.ReasonProtocolError)
			c.core.logger.Warn("Invalid retain handling",
				zap.String("clientID", c.ID),
				zap.String("topic", sub.Topic),
				zap.Uint8("retainHandling", sub.Options.RetainHandling))
			continue
		}

		// 调用 OnSubscribe 钩子
		subCtx := &SubscribeContext{
			ClientID:     c.ID,
			Packet:       p,
			Subscription: &sub,
			Identity:     c.session.getIdentity(),
		}
		if err := c.core.options.Hooks.callOnSubscribe(subCtx); err != nil {
			suback.ReasonCodes = append(suback.ReasonCodes, packet.ReasonNotAuthorized)
			c.core.logger.Debug("Subscribe rejected by hook",
				zap.String("clientID", c.ID),
				zap.String("topic", sub.Topic),
				zap.Error(err))
			continue
		}

		// 同一包内重复的过滤器：仍按请求顺序回一个原因码，但只处理一次，
		// 避免重复计数与重复重放保留消息（以最后一次出现的选项为准）。
		_, duplicate := seenTopics[sub.Topic]
		seenTopics[sub.Topic] = struct{}{}

		isNew := false
		if !duplicate {
			// 检查是否是新订阅（在添加之前检查）
			c.session.subsMu.RLock()
			_, exists := c.session.subscriptions[sub.Topic]
			c.session.subsMu.RUnlock()
			isNew = !exists
		}

		validSubs = append(validSubs, sub)
		isNewSubs = append(isNewSubs, isNew)
		replayRetained = append(replayRetained, !duplicate)
		switch sub.Options.QoS {
		case packet.QoS0:
			suback.ReasonCodes = append(suback.ReasonCodes, packet.ReasonGrantedQoS0)
		case packet.QoS1:
			suback.ReasonCodes = append(suback.ReasonCodes, packet.ReasonGrantedQoS1)
		default:
			suback.ReasonCodes = append(suback.ReasonCodes, packet.ReasonQoSNotSupported)
		}
	}

	// 合并锁操作，一次性添加所有有效订阅
	if len(validSubs) > 0 {
		c.session.subsMu.Lock()
		for _, sub := range validSubs {
			c.session.subscriptions[sub.Topic] = sub.Options
		}
		c.session.subsMu.Unlock()

		// 在锁外处理 core 订阅和日志
		for _, sub := range validSubs {
			c.core.addSubscription(c.ID, sub)
			c.core.logger.Debug("Client subscribed",
				zap.String("clientID", c.ID),
				zap.String("topic", sub.Topic),
				zap.Uint8("qos", uint8(sub.Options.QoS)))
		}

		// 订阅属于会话状态：持久化，进程重启后仍能恢复
		c.core.saveSession(c, 0)

		for _, sub := range validSubs {
			c.core.emit(Event{
				Type:     EventSubscribed,
				ClientID: c.ID,
				Topic:    sub.Topic,
				QoS:      sub.Options.QoS,
				Extra: map[string]string{
					"no_local":            boolString(sub.Options.NoLocal),
					"retain_as_published": boolString(sub.Options.RetainAsPublished),
				},
			})
		}
	}

	// 先发送 SUBACK，确保客户端收到确认后再发送保留消息
	if err := c.writePacket(suback); err != nil {
		c.core.logger.Error("Failed to send SUBACK",
			zap.String("clientID", c.ID),
			zap.String("packetID", p.PacketID.Hex()),
			zap.Error(err))
		return err
	}

	// 在 SUBACK 之后发送保留消息
	// 这样客户端收到 SUBACK 后才会注册 handler，然后才能处理保留消息
	for i, sub := range validSubs {
		if !replayRetained[i] {
			continue
		}
		// 逐条发送保留消息，避免一次性加载所有消息到内存
		c.sendRetainedMessages(
			sub.Topic,
			isNewSubs[i],
			sub.Options.NoLocal,
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
		c.session.subsMu.Lock()
		for _, topic := range validTopics {
			delete(c.session.subscriptions, topic)
		}
		c.session.subsMu.Unlock()

		// 在锁外处理 core 操作和日志
		for _, topic := range validTopics {
			c.core.removeSubscription(c.ID, topic)
			c.core.logger.Debug("Client unsubscribed", zap.String("clientID", c.ID), zap.String("topic", topic))
		}

		// 订阅属于会话状态：持久化
		c.core.saveSession(c, 0)

		for _, topic := range validTopics {
			c.core.emit(Event{
				Type:     EventUnsubscribed,
				ClientID: c.ID,
				Topic:    topic,
			})
		}
	}

	if err := c.writePacket(unsuback); err != nil {
		c.core.logger.Error("Failed to send UNSUBACK",
			zap.String("clientID", c.ID),
			zap.String("packetID", p.PacketID.Hex()),
			zap.Error(err))
		return err
	}

	return nil
}

// sendRetainedMessages 逐条发送订阅时的保留消息
// 避免一次性加载所有消息到内存
func (c *Client) sendRetainedMessages(topic string, isNewSubscription bool,
	noLocal bool, retainHandling uint8, subQoS packet.QoS) {
	// 不发送保留消息
	if retainHandling == packet.RetainHandlingNever {
		return
	}

	// 仅新订阅时发送
	if retainHandling == packet.RetainHandlingNewOnly && !isNewSubscription {
		return
	}

	// 获取匹配的主题列表（只是索引，不包含消息体）
	topics := c.core.retainStore.matchTopics(topic)
	if len(topics) == 0 {
		return
	}

	c.core.logger.Debug("Sending retained messages",
		zap.String("clientID", c.ID),
		zap.String("topic", topic),
		zap.Int("count", len(topics)),
		zap.Bool("isNewSubscription", isNewSubscription),
		zap.Uint8("retainHandling", retainHandling))

	// 逐条从 messageStore 获取消息并发送
	for _, t := range topics {
		msg, err := c.core.store.getRetain(t)
		if err != nil {
			c.core.logger.Debug("Failed to get retain message from store",
				zap.String("topic", t),
				zap.Error(err))
			continue
		}
		if msg == nil {
			continue
		}
		if noLocal && msg.Packet != nil && msg.Packet.Properties != nil && msg.Packet.Properties.SourceClientID == c.ID {
			continue
		}

		// !!! 这里可以不复制消息，直接修改 Retain 和 QoS 后投递，因为消息是从持久化存储中读取的，不会影响其他地方 !!!

		// 因订阅而重放的保留消息必须带 Retain=true，接收方才能区分历史消息与实时消息。
		// RetainAsPublished 只影响实时转发（见 Core.deliver）。
		msg.Retain = true

		// 应用 QoS 降级：取订阅 QoS 和消息 QoS 的较小值
		msg.QoS = min(subQoS, msg.QoS)
		if err := c.deliver(msg); err != nil {
			c.core.logger.Debug("Failed to deliver retained message",
				zap.String("clientID", c.ID),
				zap.String("topic", msg.Packet.Topic),
				zap.Error(err))
		}
	}
}
