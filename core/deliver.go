package core

import (
	"errors"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/internal/mq"
	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// publish 发布消息 (内部使用)
func (c *Core) publish(msg Message) bool {
	// 全局配额：待分发队列与存储水位。超限时拒绝新消息，
	// 让发布端（QoS1）收到 QuotaExceeded，而不是把磁盘写满。
	if limit := c.options.MaxPendingMessages; limit > 0 && c.queue.Count() >= int64(limit) {
		c.logger.Warn("Message rejected, pending queue quota exceeded",
			zap.String("topic", msg.Packet.Topic),
			zap.Int("pending", int(c.queue.Count())),
			zap.Int("limit", limit))
		c.stats.MessagesRejected.Add(1)
		return false
	}
	if limit := c.options.MaxStoreBytes; limit > 0 && c.storeBytes() >= limit {
		c.logger.Warn("Message rejected, store quota exceeded",
			zap.String("topic", msg.Packet.Topic),
			zap.Int64("storeBytes", c.storeBytes()),
			zap.Int64("limit", limit))
		c.stats.MessagesRejected.Add(1)
		return false
	}

	// 进入队列前重新分配 PacketID，以确保顺序和唯一性
	msg.PacketID = nson.NewId()
	c.handleRetainedMessage(&msg)

	// 放入消息队列（现在使用 Queue.Enqueue）
	if err := c.queue.Enqueue(&msg); err != nil {
		// 遇到错误，记录日志并丢弃消息
		c.logger.Error("message dropped, dispatch queue enqueue failed",
			zap.String("topic", msg.Packet.Topic),
			zap.String("packetID", msg.PacketID.Hex()),
			zap.Error(err))
		c.stats.MessagesDropped.Add(1)
		return false
	}

	// 成功入队，触发分发协程
	c.triggerDispatch()

	// 统计消息接收数
	c.stats.MessagesReceived.Add(1)
	return true
}

// triggerDispatch 非阻塞地唤醒分发协程。已有触发信号时无需重复发送。
func (c *Core) triggerDispatch() {
	select {
	case c.queueTrigger <- struct{}{}:
	default:
	}
}

func (c *Core) handleRetainedMessage(msg *Message) {
	if msg == nil || msg.Packet == nil {
		return
	}

	// 处理保留消息。带 TargetClientID 的定向消息不能进入通用 retained 存储，
	// 否则后续普通订阅者会收到本应只发给单个客户端的历史消息。
	if msg.Retain && c.options.RetainEnabled && (msg.Packet.Properties == nil || msg.Packet.Properties.TargetClientID == "") {
		if len(msg.Packet.Payload) == 0 {
			c.retainStore.remove(msg.Packet.Topic)
			if err := c.store.deleteRetain(msg.Packet.Topic); err != nil {
				c.logger.Warn("Failed to delete retained message",
					zap.String("topic", msg.Packet.Topic),
					zap.Error(err))
			}
			return
		}

		var expiryTime int64
		if msg.Packet.Properties != nil && msg.Packet.Properties.ExpiryTime > 0 {
			expiryTime = msg.Packet.Properties.ExpiryTime
		}

		if err := c.retainStore.set(msg.Packet.Topic, expiryTime); err != nil {
			c.logger.Error("Failed to set retain message index",
				zap.String("topic", msg.Packet.Topic),
				zap.Error(err))
		}

		if c.store != nil {
			if err := c.store.setRetain(msg.Packet.Topic, msg); err != nil {
				c.logger.Error("Failed to persist retain message",
					zap.String("topic", msg.Packet.Topic),
					zap.Error(err))
			}
		}
	}
}

func (c *Core) newPublishMessage(pub *packet.PublishPacket) Message {
	return Message{
		Packet:    pub,
		QoS:       pub.QoS,
		Retain:    pub.Retain,
		Timestamp: time.Now().Unix(),
	}
}

func (c *Core) newServerPublishPacket(topic string, payload []byte, targetClientID string, options PublishOptions) *packet.PublishPacket {
	pub := packet.NewPublishPacket(topic, mq.CloneBytes(payload))
	pub.QoS = options.QoS
	pub.Retain = options.Retain

	c.applyPublishProperties(pub, targetClientID, options)
	c.applyPublishExpiry(pub, options)

	return pub
}

// PublishOptions 发布选项
type PublishOptions struct {
	QoS            packet.QoS
	Retain         bool
	TraceID        string
	ContentType    string
	Expiry         uint32
	UserProperties map[string]string

	// 请求-响应模式属性
	ResponseTopic   string // 响应主题
	CorrelationData []byte // 关联数据
}

func (c *Core) applyPublishProperties(pub *packet.PublishPacket, targetClientID string, options PublishOptions) {
	if pub == nil || pub.Properties == nil {
		return
	}

	pub.Properties.TraceID = options.TraceID
	pub.Properties.ContentType = options.ContentType
	pub.Properties.TargetClientID = targetClientID
	pub.Properties.ResponseTopic = options.ResponseTopic
	pub.Properties.CorrelationData = mq.CloneBytes(options.CorrelationData)
	pub.Properties.UserProperties = mq.CloneStringMap(options.UserProperties)
}

func (c *Core) applyPublishExpiry(pub *packet.PublishPacket, options PublishOptions) {
	if pub == nil || pub.Properties == nil {
		return
	}

	var expiry time.Time
	switch {
	case options.Expiry > 0:
		expiry = time.Now().Add(time.Duration(options.Expiry) * time.Second)
	case c.options.DefaultMessageExpiry > 0:
		expiry = time.Now().Add(c.options.DefaultMessageExpiry)
	default:
		return
	}

	pub.Properties.ExpiryTime = expiry.Unix()
}

func (c *Core) dispatchLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.queueTrigger:
			for {
				// 持续高压下内层循环可能长时间不回到 select，从而饿死 ctx.Done。
				// 每轮处理前检查 ctx，保证 Core.Stop() 能及时退出。
				select {
				case <-c.ctx.Done():
					return
				default:
				}

				// 从队列中取出消息并分发
				msg, err := c.queue.Dequeue()
				if err != nil {
					if errors.Is(err, ErrCorruptedMessage) {
						// 损坏消息已在 Dequeue 中删除，跳过并继续处理后续消息，
						// 避免单条坏消息永久阻塞整个分发通道
						c.logger.Error("dropped corrupted message from dispatch queue",
							zap.Error(err))
						continue
					}
					// 队列为空或其他错误，等待下一轮触发
					break
				}

				// 分发消息
				c.deliver(msg)
			}
		}
	}
}

func (c *Core) deliver(msg *Message) {
	// 如果指定了 TargetClientID，直接投递给目标客户端
	if msg.Packet.Properties != nil && msg.Packet.Properties.TargetClientID != "" {
		c.deliverToTarget(msg)
		return
	}

	subscribers := c.subTree.matchTopic(msg.Packet.Topic)

	// 一次性获取所有需要的客户端，减少锁竞争
	c.clientsMu.RLock()
	type clientWithOptions struct {
		client *Client
		opts   packet.SubscribeOptions
	}
	clients := make([]clientWithOptions, 0, len(subscribers))
	for clientID, subOpts := range subscribers {
		if client, ok := c.clients[clientID]; ok {
			clients = append(clients, clientWithOptions{client: client, opts: subOpts})
		}
	}
	c.clientsMu.RUnlock()

	// 在锁外发送消息给普通客户端
	for _, co := range clients {
		// 检查 NoLocal 选项
		// 如果订阅者启用了 NoLocal，且消息来源是订阅者自己，则跳过投递
		if co.opts.NoLocal {
			if msg.Packet.Properties != nil && msg.Packet.Properties.SourceClientID == co.client.ID {
				c.logger.Debug("Skipping delivery due to NoLocal option",
					zap.String("clientID", co.client.ID),
					zap.String("topic", msg.Packet.Topic),
					zap.String("sourceClientID", msg.Packet.Properties.SourceClientID))
				continue
			}
		}

		// !!! 这里要复制消息，因为不同客户端的 QoS 可能不同，不能影响原消息 !!!
		copiedMsg := msg.Copy() // 复制消息
		// 确定发送 QoS (取订阅 QoS 和消息 QoS 的较小值)
		copiedMsg.QoS = min(co.opts.QoS, msg.QoS)
		// RetainAsPublished=false（默认）时实时转发不携带 Retain 标志；
		// 因订阅而重放的保留消息由 sendRetainedMessages 统一置为 true。
		if !co.opts.RetainAsPublished {
			copiedMsg.Retain = false
		}

		// 直接调用 Deliver，持久化逻辑在其中统一处理
		// 注意：不要在这里检查 IsClosed()，因为：
		// 1. 消息即使客户端离线也需要持久化
		// 2. client.deliver() 内部会处理离线情况
		if err := co.client.deliver(&copiedMsg); err != nil {
			c.logger.Debug("message delivery failed",
				zap.String("clientID", co.client.ID),
				zap.String("topic", copiedMsg.Packet.Topic),
				zap.Error(err))
		}
	}
}

// deliverToTarget 投递消息给指定的目标客户端
func (c *Core) deliverToTarget(msg *Message) {
	if msg.Packet.Properties == nil || msg.Packet.Properties.TargetClientID == "" {
		c.logger.Debug("No target client ID specified for targeted delivery",
			zap.String("topic", msg.Packet.Topic))

		return
	}

	targetID := msg.Packet.Properties.TargetClientID

	// 先尝试普通客户端
	c.clientsMu.RLock()
	client, exists := c.clients[targetID]
	c.clientsMu.RUnlock()

	if exists {
		// 直接调用 Deliver，持久化逻辑在其中统一处理
		// 注意：不要在这里检查 IsClosed()，因为：
		// 1. 消息即使客户端离线也需要持久化
		// 2. client.deliver() 内部会处理离线情况
		if err := client.deliver(msg); err != nil {
			c.logger.Debug("Target delivery failed",
				zap.String("targetClientID", targetID),
				zap.String("topic", msg.Packet.Topic),
				zap.Error(err))
		}

		return
	}

	// 目标客户端不存在
	c.logger.Debug("Target client not found",
		zap.String("targetClientID", targetID),
		zap.String("topic", msg.Packet.Topic))

	c.stats.MessagesDropped.Add(1)
}

// deliver 向客户端发送消息
// 流控策略（Beacon QoS 语义：QoS0 尽力保证到达性但无需 ACK；
// QoS1 接收方处理完消息后才 ACK，保证消息被处理而非仅协议层收到）：
// 1. 检查消息是否过期（过期直接丢弃，无论 QoS）
// 2. QoS0 与 QoS1 都先持久化到客户端队列：
//   - QoS0：离线或队列满时由重传机制补发，尽力保证到达（无需 ACK）
//   - QoS1：接收方处理失败未 ACK 时重发，保证消息被处理
//
// 3. 尝试放入发送队列：
//   - 成功：启动发送协程处理
//   - 失败（队列满）：消息仍在持久化队列中，等待重传机制加载发送
func (c *Client) deliver(msg *Message) error {
	// PacketID 属于“投递层字段”，必须对每个目标客户端独立分配
	// 无论 QoS=0 还是 QoS=1，都需要 PacketID
	msg.PacketID = nson.NewId()

	// 检查消息是否过期（过期消息直接丢弃，无论 QoS）
	if msg.IsExpired() {
		c.core.stats.MessagesDropped.Add(1)
		return nil
	}

	// 客户端队列配额：防止单个慢消费者无限占用磁盘
	if !c.enforceQueueQuota() {
		return ErrClientQueueFull
	}

	conn := c.getConn()
	online := conn != nil && !conn.closed.Load()

	// "持久化入队 + 发送队列入队" 与补货互斥，避免补货在两步之间发走消息导致重复发送。
	if online {
		conn.enqueueMu.Lock()
	}

	// QoS 0 和 QoS 1 都需要放入发送队列
	if err := c.queue.Enqueue(msg); err != nil {
		if online {
			conn.enqueueMu.Unlock()
		}
		// 遇到错误，记录日志并丢弃消息
		c.core.logger.Debug("Message dropped, client dispatch queue enqueue failed",
			zap.String("clientID", c.ID),
			zap.String("topic", msg.Packet.Topic),
			zap.Error(err))
		c.core.stats.MessagesDropped.Add(1)
		return err
	}

	// 客户端离线，但是已经放入了队列，等待重传机制处理
	if !online {
		c.core.logger.Debug("Client offline, message queued for retransmit",
			zap.String("clientID", c.ID),
			zap.String("topic", msg.Packet.Topic),
			zap.String("packetID", msg.PacketID.Hex()))

		return nil
	}

	err := conn.sendQueue.tryEnqueue(msg)
	conn.enqueueMu.Unlock()

	if err != nil {
		// 失败：
		// !!! 由于上面重新分配了 PacketID，这里不可能出现消息已在队列中的情况 !!!

		// 队列已满
		if msg.QoS == packet.QoS0 {
			// QoS0: 虽然列队已满，但消息并没有直接丢弃，因为已经持久化到队列中，等待重传机制处理
			c.core.logger.Debug("QoS0 message dropped (queue full)",
				zap.String("clientID", c.ID),
				zap.String("topic", msg.Packet.Topic),
				zap.String("packetID", msg.PacketID.Hex()))

			return nil
		}

		// QoS1: 已持久化，等待重传机制处理
		c.core.logger.Debug("QoS1 message queued for retransmit (queue full)",
			zap.String("clientID", c.ID),
			zap.String("topic", msg.Packet.Topic),
			zap.String("packetID", msg.PacketID.Hex()))

		return err
	}

	// 成功入队，触发发送协程
	conn.triggerSend()

	return nil
}

// enforceQueueQuota 在入队前应用客户端待发送队列配额。
// 返回 false 表示应丢弃这条新消息（drop-newest 策略）。
func (c *Client) enforceQueueQuota() bool {
	limit := c.core.options.MaxClientQueueSize
	if limit <= 0 {
		return true
	}

	if c.queue.Count() < int64(limit) {
		return true
	}

	if c.core.options.ClientQueueOverflowPolicy != QueueOverflowDropOldest {
		c.core.stats.MessagesDropped.Add(1)
		c.core.logger.Debug("Message dropped, client queue quota exceeded",
			zap.String("clientID", c.ID),
			zap.Int("limit", limit),
			zap.String("policy", c.core.options.ClientQueueOverflowPolicy.String()))
		return false
	}

	dropped := 0
	for c.queue.Count() >= int64(limit) {
		if _, err := c.queue.Dequeue(); err != nil {
			break
		}
		dropped++
	}
	if dropped > 0 {
		c.core.stats.MessagesDropped.Add(int64(dropped))
		c.core.logger.Debug("Client queue quota exceeded, dropped oldest messages",
			zap.String("clientID", c.ID),
			zap.Int("dropped", dropped),
			zap.Int("limit", limit))
	}
	return true
}

// triggerSend 触发发送协程（非阻塞）。
// 使用 processing 标志避免重复启动；若已有发送协程在运行，则打上 pendingSend 标记，
// 让它在退出前再处理一轮，避免"刚入队却无人处理"的丢失唤醒。
func (c *conn) triggerSend() {
	// 已关闭的连接不再启动新的发送协程，避免与 close 的 wg.Wait 竞态
	if c.closed.Load() {
		return
	}

	if c.processing.CompareAndSwap(false, true) {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.processSendQueue()
		}()
		return
	}

	c.pendingSend.Store(true)
}

// processSendQueue 处理发送队列中的消息。
// 内存发送队列排空后会立即尝试从持久化队列补货（由 ACK/发送完成驱动），
// 不再依赖 RetransmitInterval 定时器，避免突发后的秒级～30 秒级投递停顿。
func (c *conn) processSendQueue() {
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

			// 检查消息是否过期（过期消息直接丢弃，无论 QoS）
			if msg.IsExpired() {
				c.client.core.logger.Debug("Message expired, dropping",
					zap.String("clientID", c.client.ID),
					zap.String("topic", msg.Packet.Topic),
					zap.String("packetID", msg.PacketID.Hex()))

				// 从持久化队列删除
				if delErr := c.discardQueuedMessage(msg); delErr != nil {
					c.client.core.logger.Debug("Failed to delete expired message from queue",
						zap.String("clientID", c.client.ID),
						zap.String("packetID", msg.PacketID.Hex()),
						zap.Error(delErr))
				}

				// 统计消息丢弃数
				c.client.core.stats.MessagesDropped.Add(1)
				continue
			}

			// 发送消息
			if err := c.sendMessage(msg); err != nil {
				c.client.core.logger.Debug("Failed to send message from queue",
					zap.String("clientID", c.client.ID),
					zap.String("topic", msg.Packet.Topic),
					zap.Error(err))

				if errors.Is(err, ErrMessageTooLarge) {
					// 消息过大，直接丢弃不重试
					if delErr := c.discardQueuedMessage(msg); delErr != nil {
						c.client.core.logger.Debug("Failed to delete oversized message from queue",
							zap.String("clientID", c.client.ID),
							zap.String("packetID", msg.PacketID.Hex()),
							zap.Error(delErr))
					}
					c.client.core.stats.MessagesDropped.Add(1)
					continue
				}

				if errors.Is(err, ErrDeliveryRejected) {
					if delErr := c.discardQueuedMessage(msg); delErr != nil {
						c.client.core.logger.Debug("Failed to delete delivery-rejected message from queue",
							zap.String("clientID", c.client.ID),
							zap.String("packetID", msg.PacketID.Hex()),
							zap.Error(delErr))
					}
					c.client.core.stats.MessagesDropped.Add(1)
					continue
				}

				// 后续重试应明确标记为重复投递，但在 sendMessage 返回前仍保持 sendQueue 去重，
				// 以免同一条消息在写入进行中被重复入队。
				if err := c.requeueAfterSendFailure(msg); err != nil {
					c.client.core.logger.Warn("Failed to persist Dup flag after send failure",
						zap.String("clientID", c.client.ID),
						zap.String("topic", msg.Packet.Topic),
						zap.String("packetID", msg.PacketID.Hex()),
						zap.Error(err))
				}

				// 写入失败通常意味着连接已不可用：退出本轮，由重传 tick 或新的触发再试，
				// 避免在同一条失败消息上原地忙等。
				aborted = true
				break
			}

			// 发送成功：QoS0 从持久化队列删除，QoS1 等待 PUBACK
			if msg.QoS == packet.QoS0 {
				if delErr := c.discardQueuedMessage(msg); delErr != nil {
					c.client.core.logger.Debug("Failed to delete QoS0 message from queue after send",
						zap.String("clientID", c.client.ID),
						zap.String("packetID", msg.PacketID.Hex()),
						zap.Error(delErr))
				}
			} else {
				// QoS1 的删除由 handlePuback 在 pendingAck 锁内完成
				c.sendQueue.finish(msg.PacketID)
			}

			// 发送成功，统计消息发送数
			c.client.core.stats.MessagesSent.Add(1)
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
func (c *conn) discardQueuedMessage(msg *Message) error {
	c.enqueueMu.Lock()
	defer c.enqueueMu.Unlock()

	err := c.client.queue.Delete(msg.PacketID)
	c.sendQueue.finish(msg.PacketID)
	return err
}

// requeueAfterSendFailure 将发送失败的消息标记 Dup 后重新持久化，并结束本次发送。
// 与补货共用 enqueueMu，避免重复入队。
func (c *conn) requeueAfterSendFailure(msg *Message) error {
	c.enqueueMu.Lock()
	defer c.enqueueMu.Unlock()

	dupMsg := msg.Copy()
	dupMsg.Dup = true
	err := c.client.queue.Enqueue(&dupMsg)
	c.sendQueue.finish(msg.PacketID)
	return err
}

// pendingMessage 等待确认的消息
type pendingMessage struct {
	msg        *Message // 最终的 PUBLISH 包（只读）
	lastSentAt int64    // 最后发送时间（Unix 时间戳）
}

// sendMessage 发送消息到客户端
// 只负责发送和后处理（pendingAck 注册）；不负责持久化队列删除（由 processSendQueue 统一处理）
func (c *conn) sendMessage(msg *Message) error {
	if msg.Packet == nil {
		return ErrInvalidMessage
	}

	// 基于只读 PublishPacket 构造一次性出站 packet。
	// 这里需要深拷贝，隔离 OnDeliver 钩子和编码过程中的局部修改，避免污染共享消息内容层。
	pub := msg.Packet.DeepCopy()
	pub.Dup = msg.Dup
	pub.QoS = msg.QoS
	pub.Retain = msg.Retain
	pub.PacketID = msg.PacketID // 无论 QoS=0 还是 QoS=1 都使用 PacketID

	// 调用 OnDeliver 钩子，检查是否允许投递
	deliverCtx := &DeliverContext{
		ClientID: c.client.ID,
		Packet:   &pub,
	}
	if !c.client.core.options.Hooks.callOnDeliver(deliverCtx) {
		return ErrDeliveryRejected // 钩子拒绝投递
	}

	if err := c.writePacket(&pub); err != nil {
		if errors.Is(err, &packet.PacketTooLargeError{}) {
			// 数据包超过客户端允许的最大大小，返回哨兵错误由 processSendQueue 统一处理
			c.client.core.logger.Warn("Message dropped: packet size exceeds client maxPacketSize",
				zap.String("clientID", c.client.ID),
				zap.String("topic", pub.Topic),
				zap.String("packetID", pub.PacketID.Hex()),
				zap.Error(err))

			return ErrMessageTooLarge
		}

		// 其他写入错误
		return err
	}

	// 发送成功后的处理
	if pub.QoS == packet.QoS1 {
		// QoS=1: 加入 pendingAck 等待确认
		// 注意：如果是重传的消息，pendingAck 中可能已经存在该记录
		// 此时需要更新 lastSentAt 为实际发送时间
		c.client.session.pendingAckMu.Lock()
		c.client.session.pendingAck[pub.PacketID] = pendingMessage{
			msg:        msg,
			lastSentAt: time.Now().Unix(),
		}
		c.client.session.pendingAckMu.Unlock()
	}
	// QoS=0: 持久化队列删除由 processSendQueue 统一处理

	return nil
}

// PublishToClient 向指定客户端发送消息
//
// 如果客户端在线但未订阅该主题，仍然会将消息投递给该客户端；
// 如果客户端离线但会话仍保留，消息进入其待投递队列，重连后补发
// （与客户端侧通过 TargetClientID 发起的定向投递语义一致）。
func (c *Core) PublishToClient(clientID string, topic string, payload []byte, options PublishOptions) error {
	pub := c.newServerPublishPacket(topic, payload, clientID, options)
	msg := c.newPublishMessage(pub)

	c.clientsMu.RLock()
	client, exists := c.clients[clientID]
	c.clientsMu.RUnlock()

	if exists {
		return client.deliver(&msg)
	}

	return NewClientNotFoundError(clientID)
}

// Broadcast 向所有在线客户端广播消息。
//
// 注意：这是即时的、仅在线客户的扇出，不进入订阅树，也不为离线会话入队。
// 需要持久化/按订阅关系投递时请使用主题发布（Publish）。
func (c *Core) Broadcast(topic string, payload []byte, options PublishOptions) int {
	c.clientsMu.RLock()
	clients := make([]*Client, 0, len(c.clients))
	for _, client := range c.clients {
		if !client.Closed() {
			clients = append(clients, client)
		}
	}
	c.clientsMu.RUnlock()

	pub := c.newServerPublishPacket(topic, payload, "", options)
	msg := c.newPublishMessage(pub)
	c.handleRetainedMessage(&msg)

	successCount := 0
	for _, client := range clients {
		copiedMsg := msg.Copy()
		if err := client.deliver(&copiedMsg); err == nil {
			successCount++
		}
	}

	return successCount
}
