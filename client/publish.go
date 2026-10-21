package client

import (
	"errors"
	"sort"
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
	if c.closed.Load() {
		return ErrClientClosed
	}
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
	if c.closed.Load() {
		return ErrClientClosed
	}

	// 1. 分配 PacketID（无论 QoS 0 还是 QoS 1）
	msg.Packet.PacketID = c.allocatePacketID()

	c.logger.Debug("Publishing message",
		zap.String("topic", msg.Packet.Topic),
		zap.Uint8("qos", uint8(msg.Packet.QoS)),
		zap.String("packetID", msg.Packet.PacketID.Hex()),
		zap.String("traceID", msg.Packet.Properties.TraceID))

	// 2. 出站队列配额：超限时明确拒绝，避免离线/断连期间无限增长。
	if c.options != nil {
		if limit := c.options.MaxQueueSize; limit > 0 && c.queue.Count() >= int64(limit) {
			c.logger.Warn("Outbound queue full, message rejected",
				zap.String("topic", msg.Packet.Topic),
				zap.String("packetID", msg.Packet.PacketID.Hex()),
				zap.Int("limit", limit))
			return ErrQueueFull
		}
	}

	conn := c.getConn()
	online := conn != nil && !conn.IsClosed()

	// 3. QoS1：先注册 pendingAck，再入队（避免 PUBACK 先于注册到达而丢失）
	var waitCh chan error
	if online && msg.Packet.QoS == packet.QoS1 {
		waitCh = conn.registerPendingAck(msg.Packet.PacketID)
	}

	// 4. 在线时把"持久化入队 + 发送队列入队"与补货互斥：
	//    否则补货可能在这两步之间把消息发走，发布路径又入队一次 → 重复发送。
	if online {
		conn.enqueueMu.Lock()
	}

	// 5. 放入持久化队列（QoS0/QoS1 都持久化：QoS0 尽力保证到达，QoS1 保证处理）
	if err := c.queue.Enqueue(&msg); err != nil {
		if online {
			conn.enqueueMu.Unlock()
			if msg.Packet.QoS == packet.QoS1 {
				conn.clearPendingAck(msg.Packet.PacketID)
			}
		}
		if c.closed.Load() {
			// Close() 与本次发布并发：存储已关闭，返回明确的关闭错误。
			return ErrClientClosed
		}
		c.logger.Error("Failed to enqueue message to persistent queue",
			zap.String("topic", msg.Packet.Topic),
			zap.String("packetID", msg.Packet.PacketID.Hex()),
			zap.Error(err))
		return err
	}

	// 没有连接：消息留在持久化队列，等待连接后由补货机制发送
	if !online {
		if msg.Packet.QoS == packet.QoS0 {
			c.logger.Debug("Not connected, QoS0 message queued",
				zap.String("topic", msg.Packet.Topic),
				zap.String("packetID", msg.Packet.PacketID.Hex()))
			return nil
		}
		c.logger.Debug("Not connected, QoS1 message queued for retransmit",
			zap.String("topic", msg.Packet.Topic),
			zap.String("packetID", msg.Packet.PacketID.Hex()))
		return nil
	}

	// 6. 尝试放入发送队列
	err := conn.sendQueue.tryEnqueue(&msg)
	conn.enqueueMu.Unlock()

	if err != nil {
		if errors.Is(err, ErrMessageAlreadyInQueue) {
			// 已经由补货装入发送队列，等待 ACK 即可
			conn.triggerSend()
			if msg.Packet.QoS == packet.QoS1 {
				return c.waitForPublishAck(conn, msg.Packet.PacketID, waitCh, timeout)
			}
			return nil
		}

		// 发送队列已满：清理刚注册的 pendingAck（消息留在持久化层，由补货时再注册）
		if msg.Packet.QoS == packet.QoS1 {
			conn.clearPendingAck(msg.Packet.PacketID)
		}

		c.logger.Debug("Send queue full, message queued for retransmit",
			zap.String("topic", msg.Packet.Topic),
			zap.String("packetID", msg.Packet.PacketID.Hex()))
		return nil
	}

	// 7. 消息已入队，触发发送
	conn.triggerSend()

	// 8. QoS1: 等待 ACK（仅对首次发布的消息）
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

// triggerSend 触发发送协程（非阻塞）。
// 使用 processing 标志避免重复启动；若已有发送协程在运行，则打上 pendingSend 标记，
// 让它在退出前再处理一轮，避免丢失唤醒。
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
		return
	}

	c.pendingSend.Store(true)
}

// processSendQueue 处理发送队列中的消息。
// 内存发送队列排空后会立即尝试从持久化队列补货（由 PUBACK/发送完成驱动），
// 不再依赖 RetransmitInterval 定时器。
func (c *Conn) processSendQueue() {
	// 正常情况下由 triggerSend 置位；直接调用（测试）时也保证内部 triggerSend 只打标记，
	// 不会启动第二个发送协程。
	c.processing.Store(true)

	for {
		c.pendingSend.Store(false)
		aborted := false

		for !c.closed.Load() {
			// 尝试从队列取消息
			msg, ok := c.sendQueue.tryDequeue()
			if !ok {
				// 内存发送队列已空：立即从持久化队列补货
				if c.client.refillNow() {
					continue
				}
				break
			}

			// 检查消息是否过期
			if msg.IsExpired() {
				c.logger.Debug("Message expired, dropping",
					zap.String("topic", msg.Packet.Topic),
					zap.String("packetID", msg.Packet.PacketID.Hex()))

				// 从持久化队列删除
				c.discardQueuedMessage(msg)
				continue
			}

			// 发送消息
			if err := c.sendMessage(msg); err != nil {
				if errors.Is(err, ErrMessageTooLarge) {
					// 消息过大，直接丢弃不重试
					c.discardQueuedMessage(msg)
					continue
				}

				c.logger.Warn("Failed to send message from queue",
					zap.Error(err),
					zap.String("topic", msg.Packet.Topic),
					zap.String("packetID", msg.Packet.PacketID.Hex()))

				if err := c.requeueAfterSendFailure(msg); err != nil {
					c.logger.Warn("Failed to persist Dup flag after send failure",
						zap.String("topic", msg.Packet.Topic),
						zap.String("packetID", msg.Packet.PacketID.Hex()),
						zap.Error(err))
				}

				// 写入失败通常意味着连接不可用：退出本轮，由重传 tick 或新的触发再试，
				// 避免在同一条失败消息上原地忙等。
				aborted = true
				break
			}

			// 发送成功
			if msg.Packet.QoS == packet.QoS0 {
				// QoS0: 从持久化队列删除（TCP 写成功即可）
				c.discardQueuedMessage(msg)
			} else {
				// QoS1: 等待 PUBACK，由 handlePuback 删除持久化条目
				c.sendQueue.finish(msg.Packet.PacketID)
			}
		}

		// 释放处理权；若期间有新工作则重新接手
		c.processing.Store(false)
		if aborted || c.closed.Load() || !c.pendingSend.Swap(false) {
			return
		}
		if !c.processing.CompareAndSwap(false, true) {
			return
		}
	}
}

// discardQueuedMessage 删除持久化条目并结束发送队列中的生命周期。
// 与补货共用 enqueueMu，避免补货在"删除持久化条目"与"finish"之间重复装载同一条消息。
func (c *Conn) discardQueuedMessage(msg *Message) {
	c.enqueueMu.Lock()
	defer c.enqueueMu.Unlock()

	if c.client.queue != nil {
		if err := c.client.queue.Delete(msg.Packet.PacketID); err != nil {
			c.logger.Debug("Failed to delete message from queue",
				zap.String("packetID", msg.Packet.PacketID.Hex()),
				zap.Error(err))
		}
	}
	c.sendQueue.finish(msg.Packet.PacketID)
}

// requeueAfterSendFailure 将发送失败的消息标记 Dup 后重新持久化，并结束本次发送。
func (c *Conn) requeueAfterSendFailure(msg *Message) error {
	c.enqueueMu.Lock()
	defer c.enqueueMu.Unlock()

	dupMsg := *msg
	dupMsg.Dup = true

	var err error
	if c.client.queue != nil {
		err = c.client.queue.Enqueue(&dupMsg)
	}
	c.sendQueue.finish(msg.Packet.PacketID)
	return err
}

// Subscribe 订阅主题（轮询模式，通过 PollMessage 获取消息）
func (c *Client) Subscribe(topics ...string) error {
	return c.SubscribeWithOptions(topics, nil)
}

// SubscribeWithOptions 订阅主题（带选项）
func (c *Client) SubscribeWithOptions(topics []string, opts *SubscribeOptions) error {
	if c.closed.Load() {
		return ErrClientClosed
	}
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

	// 先记录订阅及其选项：重连恢复时必须原样重放，否则 QoS/NoLocal 会被降级。
	// 记录的是"期望订阅集合"，因此发送失败/超时后不撤销——重连时会重新发送。
	c.subscribedTopicsMu.Lock()
	for _, topic := range topics {
		c.subscribedTopics[topic] = *opts
	}
	c.subscribedTopicsMu.Unlock()

	// 先注册等待 ACK，再发送（避免 SUBACK 先于注册到达的竞态）
	ackCh := conn.registerPendingAck(sub.PacketID)

	if err := conn.writePacket(sub); err != nil {
		// 发送失败：SUBSCRIBE 可能已在途，保留订阅记录，重连后会重新发送。
		conn.clearPendingAck(sub.PacketID)
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
			// 部分失败：只忘记被拒绝的订阅，服务端已授予的必须保留，
			// 否则重连时不会重新订阅它们。
			c.pruneRejectedSubscriptions(topics, err)
			return err
		}
		return nil
	case <-timer.C:
		// 没有拿到 SUBACK：保留记录，重连后会重新发送（重复订阅是幂等的）。
		conn.clearPendingAck(sub.PacketID)
		return ErrPublishTimeout
	case <-conn.ctx.Done():
		conn.clearPendingAck(sub.PacketID)
		return ErrNotConnected
	}
}

// restoreSubscriptions 在服务端没有恢复会话（SessionPresent=false）时重放本地订阅。
//
// 按订阅选项分组，避免每个主题单独一次往返；选项必须原样恢复，否则
// QoS1/NoLocal/RetainAsPublished/RetainHandling 会被静默降级为默认值。
func (c *Client) restoreSubscriptions() {
	c.subscribedTopicsMu.RLock()
	groups := make(map[SubscribeOptions][]string)
	for topic, opts := range c.subscribedTopics {
		groups[opts] = append(groups[opts], topic)
	}
	c.subscribedTopicsMu.RUnlock()

	if len(groups) == 0 {
		return
	}

	// 固定顺序，便于日志排查与测试断言（map 遍历本身是随机的）。
	type group struct {
		opts   SubscribeOptions
		topics []string
	}
	ordered := make([]group, 0, len(groups))
	for opts, topics := range groups {
		sort.Strings(topics)
		ordered = append(ordered, group{opts: opts, topics: topics})
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].topics[0] < ordered[j].topics[0] })

	for _, g := range ordered {
		opts := g.opts
		if err := c.SubscribeWithOptions(g.topics, &opts); err != nil {
			c.logger.Warn("Failed to restore subscriptions",
				zap.Int("topics", len(g.topics)),
				zap.Uint8("qos", uint8(opts.QoS)),
				zap.Bool("noLocal", opts.NoLocal),
				zap.Error(err))
		}
	}
}

// pruneRejectedSubscriptions 从本地订阅记录中移除被服务端明确拒绝的主题。
// err 不是 SubscriptionError（例如连接层错误）时不做任何删除：此时无法判断结果。
func (c *Client) pruneRejectedSubscriptions(topics []string, err error) {
	var subErr *SubscriptionError
	if !errors.As(err, &subErr) || len(subErr.Codes) == 0 {
		return
	}

	c.subscribedTopicsMu.Lock()
	for i, topic := range topics {
		if !subErr.Granted(i) {
			delete(c.subscribedTopics, topic)
		}
	}
	c.subscribedTopicsMu.Unlock()
}

// Unsubscribe 取消订阅
func (c *Client) Unsubscribe(topics ...string) error {
	if c.closed.Load() {
		return ErrClientClosed
	}
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
