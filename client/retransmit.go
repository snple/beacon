package client

import (
	"time"

	"go.uber.org/zap"
)

// retransmitLoop 重传协程，定期检查并重传未确认的 QoS1 消息
func (c *Client) retransmitLoop() {
	defer c.connWG.Done()

	// 连接成功后，首先尝试重传持久化的消息
	c.triggerRetransmit()

	ticker := time.NewTicker(c.options.RetransmitInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.connCtx.Done():
			return
		case <-ticker.C:
			if !c.connected.Load() {
				return
			}
			c.triggerRetransmit()
		}
	}
}

// triggerRetransmit 触发重传（非阻塞）
// 使用 retransmitting 标志避免重复运行
func (c *Client) triggerRetransmit() {
	if c.retransmitting.CompareAndSwap(false, true) {
		go c.processRetransmit()
	}
}

// processRetransmit 处理消息重传
// 策略：从持久化存储加载未确认的消息并重新发送
func (c *Client) processRetransmit() {
	defer c.retransmitting.Store(false)

	if c.store == nil || !c.connected.Load() {
		return
	}

	// 构建排除列表（当前已在等待 ACK 的消息）
	c.pendingAckMu.Lock()
	excludePacketIDs := make(map[uint16]bool, len(c.pendingAck))
	for packetID := range c.pendingAck {
		excludePacketIDs[packetID] = true
	}
	c.pendingAckMu.Unlock()

	// 计算可加载的消息数量（使用发送队列可用容量的 60%）
	available := c.sendQueue.Available()
	if available == 0 {
		return
	}

	loadCount := int(float64(available) * 0.6)
	if loadCount == 0 {
		loadCount = 1
	}
	if loadCount > retransmitBatchSize {
		loadCount = retransmitBatchSize
	}

	// 从持久化存储加载消息
	messages, err := c.store.GetBatch(loadCount, excludePacketIDs)
	if err != nil {
		c.logger.Warn("Failed to load pending messages for retransmit",
			zap.Error(err))
		return
	}

	if len(messages) == 0 {
		return
	}

	sent := 0
	for _, stored := range messages {
		if !c.connected.Load() {
			break
		}

		// 检查消息是否过期
		if stored.ExpiryTime > 0 && time.Now().Unix() > stored.ExpiryTime {
			// 删除过期消息
			c.store.Delete(stored.PacketID)
			c.logger.Debug("Expired message deleted from store",
				zap.Uint16("packetID", stored.PacketID),
				zap.String("topic", stored.Topic))
			continue
		}

		// 转换为 Message
		msg := &Message{
			Topic:           stored.Topic,
			Payload:         stored.Payload,
			QoS:             stored.QoS,
			Retain:          stored.Retain,
			PacketID:        stored.PacketID,
			Priority:        stored.Priority,
			TraceID:         stored.TraceID,
			ContentType:     stored.ContentType,
			UserProperties:  stored.UserProperties,
			ExpiryTime:      stored.ExpiryTime,
			TargetClientID:  stored.TargetClientID,
			ResponseTopic:   stored.ResponseTopic,
			CorrelationData: stored.CorrelationData,
		}

		// 构建队列消息
		queueMsg := &QueuedMessage{
			Message:     msg,
			QoS:         stored.QoS,
			EnqueueTime: stored.EnqueueTime,
		}

		// 尝试放入发送队列
		if !c.sendQueue.TryEnqueue(queueMsg) {
			// 队列已满，停止加载
			c.logger.Debug("Send queue full during retransmit, stopping",
				zap.Int("sent", sent))
			break
		}

		// 更新最后发送时间
		c.store.UpdateLastSentTime(stored.PacketID)

		sent++
	}

	// 触发发送
	if sent > 0 {
		c.triggerSend()
		c.logger.Debug("Retransmit completed",
			zap.Int("count", sent),
			zap.Int("queueAvailable", c.sendQueue.Available()))
	}
}
