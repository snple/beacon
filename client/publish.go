package client

import (
	"time"

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
// 流程：
// 1. QoS=1: 分配 PacketID 并持久化（离线也可以）
// 2. 获取连接，如果没有连接：
//   - QoS=0: 返回错误（无法缓存）
//   - QoS=1: 返回成功（已持久化，等待重传）
//
// 3. 尝试入队，如果队列满：
//   - QoS=0: 返回错误
//   - QoS=1: 返回成功（已持久化，等待重传）
//
// 4. 入队成功：触发发送
// 5. QoS=1: 等待 ACK（仅首次发布，重传不等待）
func (c *Client) publishMessage(msg Message, timeout time.Duration) error {
	// QoS 1: 分配 PacketID 并持久化
	if msg.Packet.QoS == packet.QoS1 {
		c.logger.Debug("Publishing QoS1 message",
			zap.String("topic", msg.Packet.Topic),
			zap.String("traceID", msg.Packet.Properties.TraceID))

		// 分配 PacketID（使用 Client 级别的生成器，支持离线）
		msg.Packet.PacketID = c.allocatePacketID()
		if msg.Packet.PacketID == 0 {
			return ErrNotConnected // PacketID 分配失败
		}

		// 持久化到存储（确保不丢失）
		if c.store != nil {
			if err := c.store.save(&msg); err != nil {
				c.logger.Error("Failed to persist QoS1 message",
					zap.String("topic", msg.Packet.Topic),
					zap.Uint16("packetID", msg.Packet.PacketID),
					zap.Error(err))
				return err
			}
		}
	}

	// 获取当前连接
	conn := c.getConn()

	// 如果没有连接
	if conn == nil {
		if msg.Packet.QoS == packet.QoS0 {
			// QoS0: 无法缓存，直接失败
			c.logger.Warn("Not connected, QoS0 message dropped",
				zap.String("topic", msg.Packet.Topic))
			return ErrNotConnected
		}
		// QoS1: 已持久化，等待重连后由重传机制处理
		c.logger.Debug("Not connected, QoS1 message queued for retransmit",
			zap.String("topic", msg.Packet.Topic),
			zap.Uint16("packetID", msg.Packet.PacketID))
		return nil
	}

	// 尝试放入发送队列
	if !conn.sendQueue.tryEnqueue(&msg) {
		// 队列已满
		if msg.Packet.QoS == packet.QoS0 {
			// QoS0: 丢弃
			c.logger.Warn("Send queue full, QoS0 message dropped",
				zap.String("topic", msg.Packet.Topic))
			return ErrSendQueueFull
		}
		// QoS1: 已持久化，等待重传机制处理
		c.logger.Debug("Send queue full, QoS1 message queued for retransmit",
			zap.String("topic", msg.Packet.Topic),
			zap.Uint16("packetID", msg.Packet.PacketID))
		return nil
	}

	// 消息已入队，触发发送
	c.triggerSend()

	// QoS1: 等待 ACK（仅对首次发布的消息）
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
func (c *Client) waitForPublishAck(conn *Conn, packetID uint16, timeout time.Duration) error {
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
			zap.Uint16("packetID", packetID))
		return ErrPublishTimeout
	case <-conn.ctx.Done():
		// 连接断开：消息在持久化中，会被重传
		c.logger.Debug("Connection lost while waiting for ACK, message will be retransmitted",
			zap.Uint16("packetID", packetID))
		return ErrNotConnected
	case <-c.rootCtx.Done():
		// 客户端关闭
		return ErrNotConnected
	}
}

// triggerSend 触发发送协程（非阻塞）
// 使用 processing 标志避免重复启动
func (c *Client) triggerSend() {
	conn := c.getConn()
	if conn == nil {
		return
	}
	if conn.processing.CompareAndSwap(false, true) {
		go c.processSendQueue()
	}
}

// processSendQueue 处理发送队列中的消息
func (c *Client) processSendQueue() {
	// 获取当前连接
	conn := c.getConn()
	if conn == nil {
		return
	}
	defer conn.processing.Store(false)

	for {
		// 检查连接是否仍然有效
		if conn.IsClosed() {
			// 连接已关闭，退出（消息留在队列中，等待重连后发送）
			return
		}

		// 尝试从队列取消息
		msg, ok := conn.sendQueue.tryDequeue()
		if !ok {
			// 队列为空，退出
			return
		}

		// 检查消息是否过期
		if msg.IsExpired() {
			c.logger.Debug("Message expired, dropping",
				zap.String("topic", msg.Packet.Topic),
				zap.Uint16("packetID", msg.Packet.PacketID))
			continue
		}

		// 发送消息
		if err := c.sendMessage(msg); err != nil {
			c.logger.Warn("Failed to send message from queue",
				zap.Error(err),
				zap.String("topic", msg.Packet.Topic),
				zap.Uint16("packetID", msg.Packet.PacketID))
			// 发送失败，消息已出队，QoS1 消息会在持久化存储中，重传机制会处理
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
		sub.AddSubscription(topic, opts.QoS)
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
