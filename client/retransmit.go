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
		// 复制后再置 Dup，避免与发送协程读取同一 Message 形成数据竞争。
		dupMsg := *msg
		dupMsg.Dup = true

		// 收集到入队之间可能已被 ACK：在同一把锁内复查 pendingAck 并入队，避免重复投递。
		conn.pendingAckMu.Lock()
		if _, still := conn.pendingAck[dupMsg.Packet.PacketID]; !still {
			conn.pendingAckMu.Unlock()
			continue
		}
		err := conn.sendQueue.tryEnqueue(&dupMsg)
		conn.pendingAckMu.Unlock()

		if err != nil {
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
		loaded, stop := c.loadOneQueuedMessage(conn, sentInPass)
		if loaded {
			// 成功入队，触发发送协程
			conn.triggerSend()
			sent++
		}
		if stop {
			break
		}
	}

	return sent
}

// loadOneQueuedMessage 尝试把持久化队列的队头消息装入发送队列。
// 返回 (loaded, stop)：loaded 表示已装载，stop 表示应结束本次补货。
//
// 加锁顺序为 enqueueMu → pendingAckMu：
//   - enqueueMu 与 publishMessage 的"持久化入队 + 发送队列入队"互斥，避免重复入队；
//   - pendingAckMu 与 handlePuback 的"删除队列条目 + 移除 pendingAck"互斥，避免重复装载。
func (c *Client) loadOneQueuedMessage(conn *Conn, sentInPass map[nson.Id]struct{}) (loaded, stop bool) {
	conn.enqueueMu.Lock()
	defer conn.enqueueMu.Unlock()

	conn.pendingAckMu.Lock()
	defer conn.pendingAckMu.Unlock()

	msg, err := c.queue.Peek()
	if err != nil {
		if errors.Is(err, ErrCorruptedMessage) {
			// 头部消息损坏：删除后继续处理下一条，避免阻塞重传
			c.logger.Error("dropped corrupted queued message", zap.Error(err))
			if delErr := c.queue.DeleteHead(); delErr != nil {
				c.logger.Warn("failed to delete corrupted queued message", zap.Error(delErr))
				return false, true
			}
			return false, false
		}
		return false, true // 队列为空或其他错误
	}

	// 本 pass 已发送过该消息（FIFO 队头），停止处理
	if _, done := sentInPass[msg.Packet.PacketID]; done {
		return false, true
	}

	// 检查消息是否过期
	if msg.IsExpired() {
		if err := c.queue.Delete(msg.Packet.PacketID); err != nil {
			c.logger.Warn("Failed to delete expired queued message",
				zap.String("packetID", msg.Packet.PacketID.Hex()),
				zap.Error(err))
		}
		return false, false
	}

	// 队列是 FIFO 的：头部消息仍在等待 PUBACK 时不能跳过它去发送后面的消息
	if _, exists := conn.pendingAck[msg.Packet.PacketID]; exists {
		return false, true
	}

	if err := conn.sendQueue.tryEnqueue(msg); err != nil {
		if errors.Is(err, ErrMessageAlreadyInQueue) {
			return false, false
		}
		return false, true // 队列已满，停止加载
	}

	sentInPass[msg.Packet.PacketID] = struct{}{}
	return true, false
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

// refillNow 同步把持久化队列中的消息补进发送队列，返回是否装载了消息。
// 与 TriggerRetransmit 共用 retransmitting 标志，避免并发装载。
func (c *Client) refillNow() bool {
	if c.queue == nil {
		return false
	}
	if !c.retransmitting.CompareAndSwap(false, true) {
		return false
	}
	defer c.retransmitting.Store(false)

	return c.retransmitQueuedMessages() > 0
}

// kickRefill PUBACK 释放窗口后立即唤醒发送协程补货（重传 tick 仅作为兜底）。
func (c *Client) kickRefill() {
	conn := c.getConn()
	if conn == nil || conn.IsClosed() {
		return
	}
	if conn.sendQueue.available() == 0 {
		return
	}

	id, ok, err := c.queue.PeekID()
	if err != nil || !ok {
		return
	}

	conn.pendingAckMu.Lock()
	_, pending := conn.pendingAck[id]
	conn.pendingAckMu.Unlock()
	if pending {
		return
	}

	conn.triggerSend()
}
