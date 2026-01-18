package client

import (
	"time"

	"go.uber.org/zap"
)

// retransmitLoop 重传协程，定期检查并重传未确认的 QoS1 消息
// 注意：这个协程跨连接运行，不绑定到单个 Connection
func (c *Client) retransmitLoop() {
	defer c.retransmitting.Store(false)

	// 首次尝试重传
	c.processRetransmit()

	ticker := time.NewTicker(c.options.RetransmitInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.rootCtx.Done():
			return
		case <-ticker.C:
			c.processRetransmit()
		}
	}
}

// processRetransmit 处理消息重传
// 策略：从持久化存储加载未确认的消息并重新发送
func (c *Client) processRetransmit() {
	if c.store == nil {
		return
	}

	// 获取当前连接（如果有）
	conn := c.getConn()

	// 构建排除列表（当前已在等待 ACK 的消息）
	excludePacketIDs := make(map[uint16]bool)
	if conn != nil {
		conn.pendingAckMu.Lock()
		for packetID := range conn.pendingAck {
			excludePacketIDs[packetID] = true
		}
		conn.pendingAckMu.Unlock()
	}

	// 无连接时不重传（等待重连）
	if conn == nil {
		return
	}

	// 计算可加载的消息数量（使用发送队列可用容量的 60%）
	available := conn.sendQueue.available()
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
		if stored.Message.IsExpired() {
			// 删除过期消息
			c.store.Delete(stored.PacketID)

			c.logger.Debug("Expired message deleted from store",
				zap.Uint16("packetID", stored.PacketID),
				zap.String("topic", stored.Message.Packet.Topic))

			continue
		}

		// 转换为 Message
		msg := stored.Message
		msg.Dup = true // 标记为重发

		// 尝试放入发送队列
		if !conn.sendQueue.tryEnqueue(msg) {
			// 队列已满，停止加载
			c.logger.Debug("Send queue full during retransmit, stopping",
				zap.Int("sent", sent))
			break
		}

		sent++
	}

	// 触发发送
	if sent > 0 {
		c.triggerSend()
		c.logger.Debug("Retransmit completed",
			zap.Int("count", sent),
			zap.Int("queueAvailable", conn.sendQueue.available()))
	}
}
