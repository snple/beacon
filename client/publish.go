package client

import (
	"errors"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/internal/mq"
	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// Publish 发布消息（使用 Builder 模式选项，推荐）
//
// 用法：
//
//	err := c.Publish("topic", payload,
//	    client.NewPublishOptions().
//	        WithQoS(packet.QoS1).
//	        WithRetain(true).
//	        WithTraceID("trace-123"),
//	)
func (c *Client) Publish(topic string, payload []byte, opts *PublishOptions) error {
	if opts == nil {
		opts = NewPublishOptions()
	}

	// 确定超时时间：优先使用选项指定的，否则使用配置默认值
	timeout := opts.Timeout
	if timeout == 0 && c.options != nil {
		timeout = c.options.PublishTimeout
	}
	if timeout == 0 {
		timeout = defaultPublishTimeout
	}

	// 计算绝对过期时间戳
	var expiryTime int64
	if opts.Expiry > 0 {
		expiryTime = time.Now().Unix() + int64(opts.Expiry)
	}

	pkg := packet.NewPublishPacket(topic, mq.CloneBytes(payload))
	pkg.QoS = opts.QoS
	pkg.Retain = opts.Retain

	// 过期时间和元数据
	pkg.Properties.ExpiryTime = expiryTime
	pkg.Properties.ContentType = opts.ContentType
	pkg.Properties.TraceID = opts.TraceID

	// 请求-响应模式属性
	pkg.Properties.TargetClientID = opts.TargetClientID
	pkg.Properties.ResponseTopic = opts.ResponseTopic
	pkg.Properties.CorrelationData = mq.CloneBytes(opts.CorrelationData)

	// 用户属性
	pkg.Properties.UserProperties = mq.CloneStringMap(opts.UserProperties)

	msg := Message{
		Packet:    pkg,
		Timestamp: time.Now().Unix(),
	}

	return c.publishMessage(msg, timeout)
}

func (c *Client) PublishToClient(targetClientID, topic string, payload []byte, opts *PublishOptions) error {
	if opts == nil {
		opts = NewPublishOptions()
	}

	optsCopy := *opts
	optsCopy.TargetClientID = targetClientID

	return c.Publish(topic, payload, &optsCopy)
}

// publishMessage 内部发布消息方法
//
// 新流程（参考 core）：
// 1. 分配 PacketID（QoS 0 和 QoS 1 都分配，用于队列索引）
// 2. 放入持久化队列 (queue)
// 3. 如果客户端在线，尝试放入发送队列 (sendQueue)
//   - 成功：触发发送
//   - 失败（队列满）：等待重传机制处理
//
// 4. QoS=1: 等待 ACK（仅首次发布）
func (c *Client) publishMessage(msg Message, timeout time.Duration) error {
	// 1. 分配 PacketID（无论 QoS 0 还是 QoS 1）
	msg.Packet.PacketID = c.allocatePacketID()

	c.logger.Debug("Publishing message",
		zap.String("topic", msg.Packet.Topic),
		zap.Uint8("qos", uint8(msg.Packet.QoS)),
		zap.String("packetID", msg.Packet.PacketID.Hex()),
		zap.String("traceID", msg.Packet.Properties.TraceID))

	// 2. 放入持久化队列（QoS0/QoS1 都持久化：QoS0 尽力保证到达，QoS1 保证处理）
	// 出站队列配额：超限时明确拒绝，避免离线/断连期间无限增长。
	if c.options != nil {
		if limit := c.options.MaxQueueSize; limit > 0 && c.queue.Count() >= int64(limit) {
			c.logger.Warn("Outbound queue full, message rejected",
				zap.String("topic", msg.Packet.Topic),
				zap.String("packetID", msg.Packet.PacketID.Hex()),
				zap.Int("limit", limit))
			return ErrQueueFull
		}
	}
	if err := c.queue.Enqueue(&msg); err != nil {
		c.logger.Error("Failed to enqueue message to persistent queue",
			zap.String("topic", msg.Packet.Topic),
			zap.String("packetID", msg.Packet.PacketID.Hex()),
			zap.Error(err))
		return err
	}

	// 3. 获取当前连接
	conn := c.getConn()

	// 如果没有连接
	if conn == nil || conn.IsClosed() {
		if msg.Packet.QoS == packet.QoS0 {
			// QoS0: 已持久化到队列，等待连接后发送（尽力保证到达，无需 ACK）
			c.logger.Debug("Not connected, QoS0 message queued",
				zap.String("topic", msg.Packet.Topic),
				zap.String("packetID", msg.Packet.PacketID.Hex()))
			return nil
		}
		// QoS1: 已持久化，等待重连后由重传机制处理
		c.logger.Debug("Not connected, QoS1 message queued for retransmit",
			zap.String("topic", msg.Packet.Topic),
			zap.String("packetID", msg.Packet.PacketID.Hex()))
		return nil
	}

	// 4. QoS1：先注册 pendingAck，再放入发送队列。
	//    避免 processSendQueue 立即发送消息、服务端 PUBACK 先于注册到达而丢失，
	//    导致偶发 30s 假超时（与 Subscribe/Unsubscribe 的“先注册再发送”保持一致）。
	var waitCh chan error
	if msg.Packet.QoS == packet.QoS1 {
		waitCh = conn.registerPendingAck(msg.Packet.PacketID)
	}

	// 5. 尝试放入发送队列
	if err := conn.sendQueue.tryEnqueue(&msg); err != nil {
		// 失败：
		// !!! 由于上面重新分配了 PacketID，这里不可能出现消息已在队列中的情况 !!!

		// 队列已满：清理刚注册的 pendingAck（消息留在持久化层，由重传机制发送时再注册）
		if msg.Packet.QoS == packet.QoS1 {
			conn.clearPendingAck(msg.Packet.PacketID)
		}

		// 队列已满
		if msg.Packet.QoS == packet.QoS0 {
			// QoS0: 虽然列队已满，但消息并没有直接丢弃，因为已经持久化到队列中，等待重传机制处理
			c.logger.Debug("Send queue full, QoS0 message queued for retransmit",
				zap.String("topic", msg.Packet.Topic),
				zap.String("packetID", msg.Packet.PacketID.Hex()))
			return nil
		}
		// QoS1: 已持久化，等待重传机制处理
		c.logger.Debug("Send queue full, QoS1 message queued for retransmit",
			zap.String("topic", msg.Packet.Topic),
			zap.String("packetID", msg.Packet.PacketID.Hex()))

		return nil
	}

	// 6. 消息已入队，触发发送
	conn.triggerSend()

	// 7. QoS1: 等待 ACK（仅对首次发布的消息）
	if msg.Packet.QoS == packet.QoS1 {
		return c.waitForPublishAck(conn, msg.Packet.PacketID, waitCh, timeout)
	}

	return nil
}

// waitForPublishAck 等待 QoS1 消息的 PUBACK
//
// 注意：
// 1. 仅对首次发布的消息调用（重传消息不等待）
// 2. 如果超时或连接断开，消息仍在持久化中，会被重传机制处理
// 3. 使用 defer 确保 pendingAck 始终被清理
func (c *Client) waitForPublishAck(conn *Conn, packetID nson.Id, waitCh chan error, timeout time.Duration) error {
	// 注意：这里不在退出时清理 pendingAck——超时后条目保留，
	// 由 retransmitUnackedMessages 按 RetransmitInterval 继续重传直到收到 PUBACK。
	// 收到 ACK（handleAck）或连接关闭（conn.close）时条目会被相应清理。

	// 使用 time.NewTimer + Stop，避免 ACK 提前到达时 timer 残留到超时点（泄漏）
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-waitCh:
		// 收到 PUBACK
		return err
	case <-timer.C:
		// 超时：消息仍在持久化中，会被重传
		c.logger.Warn("Publish ACK timeout, message will be retransmitted",
			zap.String("packetID", packetID.Hex()))
		return ErrPublishTimeout
	case <-conn.ctx.Done():
		// 连接断开：消息在持久化中，会被重传
		c.logger.Debug("Connection lost while waiting for ACK, message will be retransmitted",
			zap.String("packetID", packetID.Hex()))
		return ErrNotConnected
	case <-c.ctx.Done():
		// 客户端关闭
		return ErrClientClosed
	}
}

// triggerSend 触发发送协程（非阻塞）
// 使用 processing 标志避免重复启动。
// processSendQueue 被纳入 conn.wg 跟踪，确保关闭/重连前其已退出（H3）。
func (c *Conn) triggerSend() {
	// 已关闭的连接不再启动新的发送协程，避免与 close 的 wg.Wait 竞态
	if c.closed.Load() {
		return
	}

	if c.processing.CompareAndSwap(false, true) {
		c.wg.Go(func() {
			c.processSendQueue()
		})
	}
}

// processSendQueue 处理发送队列中的消息
func (c *Conn) processSendQueue() {
	defer c.processing.Store(false)

	for !c.closed.Load() {
		// 尝试从队列取消息
		msg, ok := c.sendQueue.tryDequeue()
		if !ok {
			// 队列为空，退出
			return
		}

		// 检查消息是否过期
		if msg.IsExpired() {
			c.logger.Debug("Message expired, dropping",
				zap.String("topic", msg.Packet.Topic),
				zap.String("packetID", msg.Packet.PacketID.Hex()))

			// 从持久化队列删除
			if c.client.queue != nil {
				c.client.queue.Delete(msg.Packet.PacketID)
			}
			c.sendQueue.finish(msg.Packet.PacketID)
			continue
		}

		// 发送消息
		if err := c.sendMessage(msg); err != nil {
			if errors.Is(err, ErrMessageTooLarge) {
				// 消息过大，直接丢弃不重试
				if c.client.queue != nil {
					c.client.queue.Delete(msg.Packet.PacketID)
				}
				c.sendQueue.finish(msg.Packet.PacketID)
				continue
			}

			c.logger.Warn("Failed to send message from queue",
				zap.Error(err),
				zap.String("topic", msg.Packet.Topic),
				zap.String("packetID", msg.Packet.PacketID.Hex()))

			dupMsg := *msg
			dupMsg.Dup = true
			if c.client.queue != nil {
				if err := c.client.queue.Enqueue(&dupMsg); err != nil {
					c.logger.Warn("Failed to persist Dup flag after send failure",
						zap.String("topic", msg.Packet.Topic),
						zap.String("packetID", msg.Packet.PacketID.Hex()),
						zap.Error(err))
				}
			}

			c.sendQueue.finish(msg.Packet.PacketID)
		} else {
			// 发送成功
			// QoS0: 从持久化队列删除（TCP 写成功即可）
			if msg.Packet.QoS == packet.QoS0 && c.client.queue != nil {
				c.client.queue.Delete(msg.Packet.PacketID)
			}
			// QoS1: 等待 PUBACK，在 handlePuback 中删除
			c.sendQueue.finish(msg.Packet.PacketID)
		}
	}
}

// Subscribe 订阅主题（轮询模式，通过 PollMessage 获取消息）
func (c *Client) Subscribe(topics ...string) error {
	return c.SubscribeWithOptions(topics, nil)
}

// SubscribeWithOptions 订阅主题（带选项）
func (c *Client) SubscribeWithOptions(topics []string, opts *SubscribeOptions) error {
	if !c.connected.Load() {
		return ErrNotConnected
	}

	if len(topics) == 0 {
		return ErrTopicsEmpty
	}

	if opts == nil {
		opts = NewSubscribeOptions()
	}

	conn := c.getConn()
	if conn == nil {
		return ErrNotConnected
	}

	sub := packet.NewSubscribePacket(c.allocatePacketID())
	for _, topic := range topics {
		sub.Subscriptions = append(sub.Subscriptions, packet.Subscription{
			Topic: topic,
			Options: packet.SubscribeOptions{
				QoS:               opts.QoS,
				NoLocal:           opts.NoLocal,
				RetainAsPublished: opts.RetainAsPublished,
				RetainHandling:    opts.RetainHandling,
			},
		})
	}

	// 先记录订阅
	c.subscribedTopicsMu.Lock()
	for _, topic := range topics {
		c.subscribedTopics[topic] = true
	}
	c.subscribedTopicsMu.Unlock()

	// 先注册等待 ACK，再发送（避免 SUBACK 先于注册到达的竞态）
	ackCh := conn.registerPendingAck(sub.PacketID)

	if err := conn.writePacket(sub); err != nil {
		// 发送失败，清理
		conn.clearPendingAck(sub.PacketID)
		c.subscribedTopicsMu.Lock()
		for _, topic := range topics {
			delete(c.subscribedTopics, topic)
		}
		c.subscribedTopicsMu.Unlock()
		return err
	}

	// 等待确认
	timeout := 30 * time.Second
	if c.options != nil && c.options.PublishTimeout > 0 {
		timeout = c.options.PublishTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-ackCh:
		if err != nil {
			c.subscribedTopicsMu.Lock()
			for _, topic := range topics {
				delete(c.subscribedTopics, topic)
			}
			c.subscribedTopicsMu.Unlock()
			return err
		}
		return nil
	case <-timer.C:
		conn.clearPendingAck(sub.PacketID)
		c.subscribedTopicsMu.Lock()
		for _, topic := range topics {
			delete(c.subscribedTopics, topic)
		}
		c.subscribedTopicsMu.Unlock()
		return ErrPublishTimeout
	case <-conn.ctx.Done():
		conn.clearPendingAck(sub.PacketID)
		c.subscribedTopicsMu.Lock()
		for _, topic := range topics {
			delete(c.subscribedTopics, topic)
		}
		c.subscribedTopicsMu.Unlock()
		return ErrNotConnected
	}
}

// Unsubscribe 取消订阅
func (c *Client) Unsubscribe(topics ...string) error {
	if !c.connected.Load() {
		return ErrNotConnected
	}

	conn := c.getConn()
	if conn == nil {
		return ErrNotConnected
	}

	unsub := packet.NewUnsubscribePacket(c.allocatePacketID())
	unsub.Topics = topics

	// 先注册等待 ACK，再发送（避免 UNSUBACK 先于注册到达的竞态）
	ackCh := conn.registerPendingAck(unsub.PacketID)

	if err := conn.writePacket(unsub); err != nil {
		conn.clearPendingAck(unsub.PacketID)
		return err
	}

	timeout := 30 * time.Second
	if c.options != nil && c.options.PublishTimeout > 0 {
		timeout = c.options.PublishTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-ackCh:
		if err != nil {
			return err
		}
	case <-timer.C:
		conn.clearPendingAck(unsub.PacketID)
		return ErrPublishTimeout
	case <-conn.ctx.Done():
		conn.clearPendingAck(unsub.PacketID)
		return ErrNotConnected
	}

	// 确认成功后移除订阅记录
	c.subscribedTopicsMu.Lock()
	for _, topic := range topics {
		delete(c.subscribedTopics, topic)
	}
	c.subscribedTopicsMu.Unlock()

	return nil
}
