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
		// 复制后再置 Dup：原消息可能仍被发送协程/发送队列引用，直接改写会形成数据竞争。
		msg := item.msg.Copy()
		msg.Dup = true // 标记为重发

		// 收集到入队之间可能已被 ACK：在同一把锁内复查 pendingAck 并入队，避免重复投递。
		c.session.pendingAckMu.Lock()
		if _, still := c.session.pendingAck[msg.PacketID]; !still {
			c.session.pendingAckMu.Unlock()
			continue
		}
		err := conn.sendQueue.tryEnqueue(&msg)
		c.session.pendingAckMu.Unlock()

		if err != nil {
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

	// 本 pass 内已发送的消息（Peek 非破坏性，且异步发送与 pendingAck 注册存在竞态，
	// 避免同一条消息在一次重传中被重复加载/发送——否则会导致重复投递）
	sentInPass := make(map[nson.Id]struct{})

	for range loadCount {
		// "读取队头 + 检查 pendingAck + 入队" 必须在同一个临界区内完成：
		// handlePuback 也在同一把锁内完成"删除持久化条目 + 移除 pendingAck"，
		// 否则 ACK 与补货交错时会重复装载同一条消息。
		c.session.pendingAckMu.Lock()

		msg, err := c.queue.Peek()
		if err != nil {
			c.session.pendingAckMu.Unlock()
			if errors.Is(err, ErrCorruptedMessage) {
				// 头部消息损坏：删除后继续处理下一条，避免阻塞重传
				c.core.logger.Error("dropped corrupted queued message",
					zap.String("clientID", c.ID),
					zap.Error(err))
				if delErr := c.queue.DeleteHead(); delErr != nil {
					c.core.logger.Warn("failed to delete corrupted queued message",
						zap.String("clientID", c.ID),
						zap.Error(delErr))
					break
				}
				c.core.stats.MessagesDropped.Add(1)
				continue
			}
			break // 队列已空
		}

		// 本 pass 已发送过该消息（FIFO 队头，后续消息必然在其之后），停止处理
		if _, done := sentInPass[msg.PacketID]; done {
			c.session.pendingAckMu.Unlock()
			break
		}

		if msg.IsExpired() {
			c.session.pendingAckMu.Unlock()
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
		if _, exists := c.session.pendingAck[msg.PacketID]; exists {
			c.session.pendingAckMu.Unlock()
			break
		}

		// 尝试放入发送队列（持有锁，避免与 ACK 处理交错）
		err = conn.sendQueue.tryEnqueue(msg)
		c.session.pendingAckMu.Unlock()
		if err != nil {
			if errors.Is(err, ErrMessageAlreadyInQueue) {
				// 消息已在队列中，跳过
				continue
			}

			// 队列已满，停止加载
			break
		}

		// 成功入队，触发发送协程
		conn.triggerSend()

		sentInPass[msg.PacketID] = struct{}{}
		sent++
	}

	return sent
}

// refillNow 同步把持久化队列中的消息补进发送队列，返回是否装载了消息。
// 与 retransmitLoop 共用 retransmitting 标志，避免并发装载。
func (c *Client) refillNow() bool {
	if !c.session.retransmitting.CompareAndSwap(false, true) {
		return false
	}
	defer c.session.retransmitting.Store(false)

	return c.retransmitQueuedMessages() > 0
}

// hasRefillableHead 廉价判断持久化队列头部是否可以补进发送队列。
// 只读取队头 PacketID，不读取消息体，用于 ACK 后决定是否唤醒发送协程。
func (c *Client) hasRefillableHead() bool {
	conn := c.getConn()
	if conn == nil || conn.closed.Load() {
		return false
	}
	if conn.sendQueue.available() == 0 {
		return false
	}

	id, ok, err := c.queue.PeekID()
	if err != nil || !ok {
		return false
	}

	c.session.pendingAckMu.Lock()
	_, pending := c.session.pendingAck[id]
	c.session.pendingAckMu.Unlock()

	return !pending
}

// kickRefill ACK 释放窗口后立即唤醒发送协程补货（重传 tick 仅作为兜底）。
func (c *Client) kickRefill() {
	if !c.hasRefillableHead() {
		return
	}
	if conn := c.getConn(); conn != nil {
		conn.triggerSend()
	}
}
