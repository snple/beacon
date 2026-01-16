package core

import (
	"time"

	"go.uber.org/zap"
)

// retransmitLoop 定期触发消息重传
// 检查所有客户端的发送队列，加载待发送的消息
func (c *Core) retransmitLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.options.RetransmitInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.triggerRetransmit()
		}
	}
}

// triggerRetransmit 触发所有客户端的消息重传
func (c *Core) triggerRetransmit() {
	c.clientsMu.RLock()
	clients := make([]*Client, 0, len(c.clients))
	for _, client := range c.clients {
		if !client.IsClosed() {
			clients = append(clients, client)
		}
	}
	c.clientsMu.RUnlock()

	for _, client := range clients {
		// 调用 processRetransmit 触发重传和加载逻辑
		if client.retransmitting.CompareAndSwap(false, true) {
			go client.processRetransmit()
		}
	}
}

// processRetransmit 处理消息重传（在 goroutine 中执行）
// 重传策略：1) 重传超时未确认的消息；2) 从持久化存储加载待发送的消息
// 使用 retransmitting 标志确保同一时间只有一个重传任务在运行
func (c *Client) processRetransmit() {
	defer c.retransmitting.Store(false)

	totalSent := 0

	// 1. 重传已发送但未确认的消息（pendingAck 中的超时消息）
	totalSent += c.retransmitUnackedMessages()

	// 2. 从持久化存储加载未发送的消息
	totalSent += c.loadPersistedMessages()

	if totalSent > 0 {
		c.core.logger.Debug("Retransmit completed",
			zap.String("clientID", c.ID),
			zap.Int("totalSent", totalSent))
	}
}

// loadPersistedMessages 从持久化存储加载待发送的消息
// 策略：根据队列可用容量的一定比例（60%）加载消息，避免队列立即饱和
func (c *Client) loadPersistedMessages() int {
	if c.core.messageStore == nil {
		return 0
	}

	// 计算可加载的消息数量
	// 使用可用容量的 60% 避免队列立即饱和
	available := c.sendQueue.available()
	if available == 0 {
		return 0
	}

	loadCount := int(float64(available) * 0.6)
	if loadCount == 0 {
		loadCount = 1
	}

	// 限制单次加载数量
	if loadCount > overflowBatchSize {
		loadCount = overflowBatchSize
	}

	// 构建排除列表（已在 pendingAck 中的消息）
	c.pendingAckMu.Lock()
	excludePacketIDs := make(map[uint16]bool, len(c.pendingAck))
	for packetID := range c.pendingAck {
		excludePacketIDs[packetID] = true
	}
	c.pendingAckMu.Unlock()

	// 从持久化存储加载消息
	messages, err := c.core.messageStore.getPendingMessagesBatch(c.ID, loadCount, excludePacketIDs)
	if err != nil {
		c.core.logger.Warn("Failed to load pending messages",
			zap.String("clientID", c.ID),
			zap.Error(err))
		return 0
	}

	sent := 0
	for _, stored := range messages {
		// 检查消息是否过期
		if stored.Message.IsExpired() {
			// 删除过期消息
			c.core.messageStore.delete(c.ID, stored.PacketID)

			c.core.logger.Debug("Expired message deleted from store",
				zap.String("clientID", c.ID),
				zap.Uint16("packetID", stored.PacketID),
				zap.String("topic", stored.Message.Packet.Topic))

			continue
		}

		// 转换为 Message
		msg := stored.Message
		msg.Dup = true // 标记为重发

		// 尝试放入发送队列
		if c.sendQueue.tryEnqueue(msg) {
			// 成功入队，触发发送协程
			c.triggerSend()

			sent++
		} else {
			// 队列已满，停止加载
			break
		}
	}

	// 如果成功加载了消息，触发发送
	if sent > 0 {
		c.triggerSend()
		c.core.logger.Debug("Loaded pending messages",
			zap.String("clientID", c.ID),
			zap.Int("count", sent),
			zap.Int("queueAvailable", c.sendQueue.available()))
	}

	return sent
}

// retransmitUnackedMessages 重传已发送但未确认的消息
// 策略：检查 pendingAck 中超过重传间隔的消息，重新放入发送队列
func (c *Client) retransmitUnackedMessages() int {
	sent := 0
	now := time.Now().Unix()
	retransmitInterval := int64(c.core.options.RetransmitInterval.Seconds())

	// 收集需要重发的消息和过期消息
	var toResend []*pendingMessage
	var expiredPacketIDs []uint16

	c.pendingAckMu.Lock()
	for packetID, pending := range c.pendingAck {
		// 检查是否过期
		if pending.msg.IsExpired() {
			expiredPacketIDs = append(expiredPacketIDs, packetID)
			continue
		}

		// 检查是否需要重传
		if now-pending.lastSentAt < retransmitInterval {
			continue
		}

		toResend = append(toResend, &pending)
	}

	// 删除过期消息
	for _, packetID := range expiredPacketIDs {
		pending := c.pendingAck[packetID]
		if c.core.messageStore != nil {
			c.core.messageStore.delete(c.ID, pending.msg.PacketID)
		}
		delete(c.pendingAck, packetID)
		c.core.stats.MessagesDropped.Add(1)
	}
	c.pendingAckMu.Unlock()

	// 尝试将需要重传的消息放入队列
	for _, item := range toResend {
		msg := item.msg
		msg.Dup = true // 标记为重发

		if c.sendQueue.tryEnqueue(item.msg) {
			// 成功入队，触发发送协程
			c.triggerSend()

			sent++
		} else {
			// 队列已满，停止重传
			break
		}
	}

	// 如果有消息加入队列，触发发送
	if sent > 0 {
		c.triggerSend()
	}

	return sent
}
