package core

import (
	"errors"
	"time"

	"github.com/danclive/nson-go"
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
		if !client.Closed() {
			clients = append(clients, client)
		}
	}
	c.clientsMu.RUnlock()

	for _, client := range clients {
		// 调用 processRetransmit 触发重传和加载逻辑
		if client.session.retransmitting.CompareAndSwap(false, true) {
			go client.processRetransmit()
		}
	}
}

// processRetransmit 处理消息重传（在 goroutine 中执行）
// 重传策略：1) 重传超时未确认的消息；2) 从持久化存储加载待发送的消息
// 使用 retransmitting 标志确保同一时间只有一个重传任务在运行
func (c *Client) processRetransmit() {
	defer c.session.retransmitting.Store(false)

	totalSent := 0

	// 1. 重传已发送但未确认的消息（pendingAck 中的超时消息）
	totalSent += c.retransmitUnackedMessages()

	// 2. 重传发送队列中的消息
	totalSent += c.retransmitQueuedMessages()

	if totalSent > 0 {
		c.core.logger.Debug("Retransmit completed",
			zap.String("clientID", c.ID),
			zap.Int("totalSent", totalSent))
	}
}

// retransmitUnackedMessages 重传已发送但未确认的消息
// 策略：检查 pendingAck 中超过重传间隔的消息，重新放入发送队列
func (c *Client) retransmitUnackedMessages() int {
	sent := 0
	now := time.Now().Unix()
	retransmitInterval := int64(c.core.options.RetransmitInterval.Seconds())

	// 收集需要重发的消息和过期消息
	var toResend []*pendingMessage
	var expiredPacketIDs []nson.Id

	c.session.pendingAckMu.Lock()
	for packetID, pending := range c.session.pendingAck {
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
		// 从持久化存储删除
		if err := c.queue.Delete(packetID); err != nil {
			c.core.logger.Warn("Failed to delete expired pending message",
				zap.String("clientID", c.ID),
				zap.String("packetID", packetID.Hex()),
				zap.Error(err))
		}

		delete(c.session.pendingAck, packetID)
		c.core.stats.MessagesDropped.Add(1)
	}
	c.session.pendingAckMu.Unlock()

	// 获取连接
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil || conn.closed.Load() {
		return sent
	}

	// 尝试将需要重传的消息放入队列
	for _, item := range toResend {
		// !!! 这里可以直接修改原消息，既然走到这一步，消息已经在重发状态 !!!
		msg := item.msg
		msg.Dup = true // 标记为重发

		if err := conn.sendQueue.tryEnqueue(msg); err != nil {
			if errors.Is(err, ErrMessageAlreadyInQueue) {
				// 消息已在队列中，跳过
				continue
			}

			// 队列已满，停止重传
			break
		}

		// 触发发送协程
		conn.triggerSend()

		sent++
	}

	return sent
}

// retransmitQueuedMessages 重传发送队列中的消息
func (c *Client) retransmitQueuedMessages() int {
	sent := 0

	// 计算可加载的消息数量
	// 使用可用容量的 50% 避免队列立即饱和
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil || conn.closed.Load() {
		return 0
	}

	available := conn.sendQueue.available()
	if available == 0 {
		return 0
	}

	loadCount := int(float64(available) * 0.5)
	if loadCount == 0 {
		loadCount = 1
	}

	// 限制单次加载数量
	if loadCount > overflowBatchSize {
		loadCount = overflowBatchSize
	}

	for range loadCount {
		msg, err := c.queue.Peek()
		if err != nil {
			break // 队列已空
		}

		if msg.IsExpired() {
			// 删除过期消息
			if err := c.queue.Delete(msg.PacketID); err != nil {
				c.core.logger.Warn("Failed to delete expired queued message",
					zap.String("clientID", c.ID),
					zap.String("packetID", msg.PacketID.Hex()),
					zap.Error(err))
			}

			c.core.stats.MessagesDropped.Add(1)
			continue
		}

		// 排除在 pendingAck 中的消息
		// 队列是 FIFO 的，如果头部消息在 pendingAck 中（等待确认），
		// 说明它已经被发送了，我们应该停止处理，而不是继续处理后面的消息
		c.session.pendingAckMu.Lock()
		_, exists := c.session.pendingAck[msg.PacketID]
		c.session.pendingAckMu.Unlock()
		if exists {
			break
		}

		// 转换为 Message
		msg.Dup = true // 标记为重发

		// 尝试放入发送队列
		if err := conn.sendQueue.tryEnqueue(msg); err != nil {
			if errors.Is(err, ErrMessageAlreadyInQueue) {
				// 消息已在队列中，跳过
				continue
			}

			// 队列已满，停止加载
			break
		}

		// 成功入队，触发发送协程
		conn.triggerSend()

		sent++
	}

	return sent
}
