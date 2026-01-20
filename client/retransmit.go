package client

import (
	"errors"
	"time"

	"go.uber.org/zap"
)

// retransmitLoop 重传协程，定期检查并从持久化队列加载消息
// 注意：这个协程跨连接运行，不绑定到单个 Connection
func (c *Client) retransmitLoop() {
	defer c.retransmitting.Store(false)

	// 首次尝试重传
	c.processRetransmit()

	ticker := time.NewTicker(c.options.RetransmitInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.processRetransmit()
		}
	}
}

// processRetransmit 处理消息重传（在 goroutine 中执行）
// 重传策略：从持久化队列 (queue) 加载消息到发送队列 (sendQueue)
// 使用 retransmitting 标志确保同一时间只有一个重传任务在运行
func (c *Client) processRetransmit() {
	if c.queue == nil {
		return
	}

	// 从持久化队列加载消息到发送队列
	sent := c.retransmitQueuedMessages()

	if sent > 0 {
		c.logger.Debug("Retransmit completed",
			zap.Int("sent", sent))
	}
}

// retransmitQueuedMessages 从持久化队列加载消息到发送队列
// 参考 core 的实现
func (c *Client) retransmitQueuedMessages() int {
	sent := 0

	// 获取连接
	conn := c.getConn()
	if conn == nil || conn.IsClosed() {
		return 0
	}

	// 计算可加载的消息数量
	// 使用可用容量的 50% 避免队列立即饱和
	available := conn.sendQueue.available()
	if available == 0 {
		return 0
	}

	loadCount := int(float64(available) * 0.5)
	if loadCount == 0 {
		loadCount = 1
	}

	// 限制单次加载数量
	if loadCount > retransmitBatchSize {
		loadCount = retransmitBatchSize
	}

	for range loadCount {
		// Peek 获取队列头部消息（不删除）
		msg, err := c.queue.Peek()
		if err != nil {
			// 队列为空或其他错误
			break
		}

		// 检查消息是否过期
		if msg.IsExpired() {
			// 删除过期消息并继续
			if err := c.queue.Delete(msg.Packet.PacketID); err != nil {
				c.logger.Warn("Failed to delete expired queued message",
					zap.String("packetID", msg.Packet.PacketID.Hex()),
					zap.Error(err))
			}

			continue
		}

		// 标记为重发
		msg.Dup = true

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
		c.triggerSend()

		sent++
	}

	// 如果有消息入队，触发发送
	if sent > 0 {
		c.triggerSend()
	}

	return sent
}

// TriggerRetransmit 手动触发重传（可由外部调用）
func (c *Client) TriggerRetransmit() {
	if c.retransmitting.CompareAndSwap(false, true) {
		go c.processRetransmit()
	}
}
