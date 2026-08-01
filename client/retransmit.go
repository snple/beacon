package client

import (
	"errors"
	"time"

	"github.com/danclive/nson-go"
	"go.uber.org/zap"
)

// retransmitLoop 重传协程，定期检查并从持久化队列加载消息
// 注意：这个协程跨连接运行，不绑定到单个 Connection
// 不再持有 retransmitting 标志（该标志仅用于 TriggerRetransmit 的一次性守卫，M6）
func (c *Client) retransmitLoop() {
	defer c.wg.Done()

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
// 重传策略：1) 重传超时未确认的 QoS1 发布消息；2) 从持久化存储加载待发送的消息
func (c *Client) processRetransmit() {
	if c.queue == nil {
		return
	}

	totalSent := 0

	// 1. 重传已发送但未确认的消息（pendingAck 中的超时消息）
	totalSent += c.retransmitUnackedMessages()

	// 2. 从持久化队列加载待发送消息（离线积压/队列满补发）
	totalSent += c.retransmitQueuedMessages()

	if totalSent > 0 {
		c.logger.Debug("Retransmit completed",
			zap.Int("sent", totalSent))
	}
}

// retransmitUnackedMessages 重传已发送但未确认的 QoS1 发布消息
// 策略：检查当前连接的 pendingAck 中超过重传间隔的消息，重新放入发送队列。
// 与 core 侧对称：sendMessage 在发送成功时记录 lastSentAt，超时未确认则置 Dup 重发。
func (c *Client) retransmitUnackedMessages() int {
	conn := c.getConn()
	if conn == nil || conn.IsClosed() {
		return 0
	}

	now := time.Now().Unix()
	retransmitInterval := int64(c.options.RetransmitInterval.Seconds())
	if retransmitInterval <= 0 {
		retransmitInterval = 1
	}

	var toResend []*Message
	var expiredIDs []nson.Id

	conn.pendingAckMu.Lock()
	for packetID, pending := range conn.pendingAck {
		// 仅处理 QoS1 发布消息（subscribe/unsubscribe 的 pending 没有 msg）
		if pending.msg == nil {
			continue
		}

		// 检查是否过期：过期则从持久化队列删除并清理 pendingAck
		if pending.msg.IsExpired() {
			expiredIDs = append(expiredIDs, packetID)
			continue
		}

		// 检查是否需要重传
		if pending.lastSentAt > 0 && now-pending.lastSentAt < retransmitInterval {
			continue
		}

		toResend = append(toResend, pending.msg)
	}

	for _, packetID := range expiredIDs {
		delete(conn.pendingAck, packetID)
	}
	conn.pendingAckMu.Unlock()

	// 删除过期消息（锁外执行持久化操作）
	for _, packetID := range expiredIDs {
		if err := c.queue.Delete(packetID); err != nil {
			c.logger.Warn("Failed to delete expired pending message",
				zap.String("packetID", packetID.Hex()),
				zap.Error(err))
		}
	}

	if len(toResend) == 0 {
		return 0
	}

	sent := 0
	for _, msg := range toResend {
		// 重传消息明确标记为 Dup
		msg.Dup = true

		if err := conn.sendQueue.tryEnqueue(msg); err != nil {
			if errors.Is(err, ErrMessageAlreadyInQueue) {
				// 消息已在队列中，跳过
				continue
			}

			// 队列已满，停止重传
			break
		}

		// 触发发送协程（sendMessage 会刷新 lastSentAt）
		conn.triggerSend()

		sent++
	}

	return sent
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

	// 本 pass 内已发送的消息（Peek 非破坏性，且异步发送与 pendingAck 注册存在竞态，
	// 避免同一条消息在一次重传中被重复加载/发送）
	sentInPass := make(map[nson.Id]struct{})

	for range loadCount {
		// Peek 获取队列头部消息（不删除）
		msg, err := c.queue.Peek()
		if err != nil {
			if errors.Is(err, ErrCorruptedMessage) {
				// 头部消息损坏：删除后继续处理下一条，避免阻塞重传
				c.logger.Error("dropped corrupted queued message", zap.Error(err))
				if delErr := c.queue.DeleteHead(); delErr != nil {
					c.logger.Warn("failed to delete corrupted queued message", zap.Error(delErr))
					break
				}
				continue
			}
			// 队列为空或其他错误
			break
		}

		// 本 pass 已发送过该消息（FIFO 队头），停止处理
		if _, done := sentInPass[msg.Packet.PacketID]; done {
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

		// 队列是 FIFO 的。如果头部消息已经发出且还在等待 PUBACK，
		// 当前连接不能跳过它去重发后面的消息，否则会打乱顺序并重复发送头部消息。
		conn.pendingAckMu.Lock()
		_, exists := conn.pendingAck[msg.Packet.PacketID]
		conn.pendingAckMu.Unlock()
		if exists {
			break
		}

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

		sentInPass[msg.Packet.PacketID] = struct{}{}
		sent++
	}

	return sent
}

// TriggerRetransmit 手动触发重传（可由外部调用）
func (c *Client) TriggerRetransmit() {
	if c.retransmitting.CompareAndSwap(false, true) {
		go func() {
			defer c.retransmitting.Store(false)
			c.processRetransmit()
		}()
	}
}
