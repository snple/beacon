package client

import (
	"time"

	"github.com/danclive/nson-go"
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

	pkg := packet.NewPublishPacket(topic, payload)
	pkg.QoS = opts.QoS
	pkg.Retain = opts.Retain

	// 过期时间和元数据
	pkg.Properties.ExpiryTime = expiryTime
	pkg.Properties.ContentType = opts.ContentType
	pkg.Properties.TraceID = opts.TraceID

	// 请求-响应模式属性
	pkg.Properties.TargetClientID = opts.TargetClientID
	pkg.Properties.ResponseTopic = opts.ResponseTopic
	pkg.Properties.CorrelationData = opts.CorrelationData

	// 用户属性
	pkg.Properties.UserProperties = opts.UserProperties

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

	return c.Publish(topic, payload, opts.WithTargetClientID(targetClientID))
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

	// 2. 放入持久化队列（QoS0 和 QoS1 都持久化以提高到达率）
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
			// QoS0: 已持久化到队列，等待连接后发送（提高到达率）
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

	// 4. 尝试放入发送队列
	if err := conn.sendQueue.tryEnqueue(&msg); err != nil {
		// 失败：
		// !!! 由于上面重新分配了 PacketID，这里不可能出现消息已在队列中的情况 !!!

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

		return err
	}

	// 5. 消息已入队，触发发送
	conn.triggerSend()

	// 6. QoS1: 等待 ACK（仅对首次发布的消息）
	if msg.Packet.QoS == packet.QoS1 {
		return c.waitForPublishAck(conn, msg.Packet.PacketID, timeout)
	}

	return nil
}

// waitForPublishAck 等待 QoS1 消息的 PUBACK
//
// 注意：
// 1. 仅对首次发布的消息调用（重传消息不等待）
// 2. 如果超时或连接断开，消息仍在持久化中，会被重传机制处理
// 3. 使用 defer 确保 pendingAck 始终被清理
func (c *Client) waitForPublishAck(conn *Conn, packetID nson.Id, timeout time.Duration) error {
	// 创建等待通道
	waitCh := make(chan error, 1)

	conn.pendingAckMu.Lock()
	conn.pendingAck[packetID] = waitCh
	conn.pendingAckMu.Unlock()

	// 确保退出时清理 pendingAck
	defer func() {
		conn.pendingAckMu.Lock()
		delete(conn.pendingAck, packetID)
		conn.pendingAckMu.Unlock()
	}()

	timeoutCh := time.After(timeout)
	select {
	case err := <-waitCh:
		// 收到 PUBACK
		return err
	case <-timeoutCh:
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
// 使用 processing 标志避免重复启动
func (c *Conn) triggerSend() {
	if c.processing.CompareAndSwap(false, true) {
		go c.processSendQueue()
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
			continue
		}

		// 发送消息
		if err := c.sendMessage(msg); err != nil {
			c.logger.Warn("Failed to send message from queue",
				zap.Error(err),
				zap.String("topic", msg.Packet.Topic),
				zap.String("packetID", msg.Packet.PacketID.Hex()))
			// 发送失败，消息已出队
			// QoS1 消息仍在持久化队列中，重传机制会处理
			// QoS0 消息应从持久化队列删除（已尽力发送）
			if msg.Packet.QoS == packet.QoS0 && c.client.queue != nil {
				c.client.queue.Delete(msg.Packet.PacketID)
			}
		} else {
			// 发送成功
			// QoS0: 从持久化队列删除（TCP 写成功即可）
			if msg.Packet.QoS == packet.QoS0 && c.client.queue != nil {
				c.client.queue.Delete(msg.Packet.PacketID)
			}
			// QoS1: 等待 PUBACK，在 handlePuback 中删除
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
				RetainHandling:    0, // 默认总是发送保留消息
			},
		})
	}

	// 先记录订阅
	c.subscribedTopicsMu.Lock()
	for _, topic := range topics {
		c.subscribedTopics[topic] = true
	}
	c.subscribedTopicsMu.Unlock()

	if err := conn.writePacket(sub); err != nil {
		// 发送失败，回滚
		c.subscribedTopicsMu.Lock()
		for _, topic := range topics {
			delete(c.subscribedTopics, topic)
		}
		c.subscribedTopicsMu.Unlock()
		return err
	}

	// 等待确认
	if err := conn.waitAck(sub.PacketID, 30*time.Second); err != nil {
		// 确认失败，回滚
		c.subscribedTopicsMu.Lock()
		for _, topic := range topics {
			delete(c.subscribedTopics, topic)
		}
		c.subscribedTopicsMu.Unlock()
		return err
	}

	return nil
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

	if err := conn.writePacket(unsub); err != nil {
		return err
	}

	if err := conn.waitAck(unsub.PacketID, 30*time.Second); err != nil {
		return err
	}

	// 确认成功后移除订阅记录
	c.subscribedTopicsMu.Lock()
	for _, topic := range topics {
		delete(c.subscribedTopics, topic)
	}
	c.subscribedTopicsMu.Unlock()

	return nil
}
