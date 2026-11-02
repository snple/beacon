package core

import (
	"errors"
	"time"

	"github.com/snple/beacon/internal/mq"
	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// errDeliveryDropped 表示这条投递应当被丢弃（过期、超出客户端包大小、被钩子拒绝），
// 它不是连接级错误，发送协程可以继续处理后续消息。
var errDeliveryDropped = errors.New("delivery dropped")

// defaultSendWindow 是客户端未声明接收窗口时使用的发送窗口。
const defaultSendWindow = 100

// publish 接收一条消息：注册到 messageStore，并进入全局待分发队列。
//
// 消息内容在这里只保存一份（messageStore 以 broker 生成的唯一 ID 为键），
// 之后的全局队列、客户端队列、待确认列表都只引用这个 ID，不再复制消息。
//
// 返回注册后的消息；nil 表示被拒绝（配额超限或无法编码），调用方应回 QuotaExceeded。
func (c *Core) publish(pub *packet.PublishPacket) *Message {
	if limit := c.options.MaxPendingMessages; limit > 0 && c.pending.len() >= limit {
		c.emit(Event{
			Type:   EventMessageRejected,
			Topic:  pub.Topic,
			Bytes:  len(pub.Payload),
			QoS:    pub.QoS,
			Reason: packet.ReasonQuotaExceeded.String(),
			Detail: "pending_messages",
		})
		c.logger.Warn("Message rejected, pending queue quota exceeded",
			zap.String("topic", pub.Topic),
			zap.Int("pending", c.pending.len()),
			zap.Int("limit", limit))
		c.stats.MessagesRejected.Add(1)
		return nil
	}

	if limit := c.options.MaxPendingBytes; limit > 0 {
		if _, used := c.msgs.stats(); used >= limit {
			c.emit(Event{
				Type:   EventMessageRejected,
				Topic:  pub.Topic,
				Bytes:  len(pub.Payload),
				QoS:    pub.QoS,
				Reason: packet.ReasonQuotaExceeded.String(),
				Detail: "pending_bytes",
			})
			c.logger.Warn("Message rejected, memory quota exceeded",
				zap.String("topic", pub.Topic),
				zap.Int64("pendingBytes", used),
				zap.Int64("limit", limit))
			c.stats.MessagesRejected.Add(1)
			return nil
		}
	}

	msg, err := newMessage(pub)
	if err != nil {
		// 编码失败（例如主题超出协议上限）：在入队前拒绝，避免分发阶段逐客户端失败。
		c.logger.Warn("Message rejected, encode failed",
			zap.String("topic", pub.Topic),
			zap.Error(err))
		c.stats.MessagesRejected.Add(1)
		return nil
	}

	// 引用计数 1：这份引用交给全局分发队列
	c.msgs.put(msg)

	// 保留消息需要独立于投递链路存活，内部会额外持有一个引用
	c.handleRetainedMessage(msg)

	c.pending.enqueue(msg.ID)
	c.triggerDispatch()
	c.stats.MessagesReceived.Add(1)

	c.emit(Event{
		Type:     EventMessagePublished,
		ClientID: msg.sourceClientID(),
		Topic:    pub.Topic,
		Bytes:    len(pub.Payload),
		QoS:      msg.QoS,
		Retained: msg.Retain,
	})

	return msg
}

// triggerDispatch 非阻塞地唤醒分发协程。已有触发信号时无需重复发送。
func (c *Core) triggerDispatch() {
	select {
	case c.queueTrigger <- struct{}{}:
	default:
	}
}

// handleRetainedMessage 维护保留消息：空载荷表示删除，否则覆盖。
func (c *Core) handleRetainedMessage(msg *Message) {
	if msg == nil || msg.Packet == nil || !msg.Retain || !c.options.RetainEnabled {
		return
	}

	// 带 TargetClientID 的定向消息不能进入通用 retained 存储，
	// 否则后续普通订阅者会收到本应只发给单个客户端的历史消息。
	if msg.targetClientID() != "" {
		return
	}

	topic := msg.Packet.Topic
	if len(msg.Packet.Payload) == 0 {
		c.retainStore.remove(topic)
		return
	}

	var expiryTime int64
	if msg.Packet.Properties != nil {
		expiryTime = msg.Packet.Properties.ExpiryTime
	}

	// 保留消息必须比本次分发存活更久：retainStore 持有一个独立引用
	c.msgs.retain(msg)

	if err := c.retainStore.set(topic, msg, expiryTime); err != nil {
		c.msgs.release(msg)
		c.logger.Error("Failed to set retain message",
			zap.String("topic", topic),
			zap.Error(err))
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

	// PublisherID 是 broker 侧发布时使用的身份（影响 NoLocal 判断与审计）。
	// 为空表示系统发布。
	PublisherID string
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

// dispatchLoop 从全局队列取出消息 ID，解析出共享消息并分发。
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

				id, ok := c.pending.dequeue()
				if !ok {
					break
				}

				// take：全局队列持有的引用转移到这里
				msg := c.msgs.take(id)
				if msg == nil {
					continue
				}

				c.deliver(msg)

				// 归还全局队列的引用：若没有其它持有者，消息内容此时即可被 GC 回收
				c.msgs.release(msg)
			}
		}
	}
}

// deliver 把一条共享消息投递给所有匹配的订阅者。
//
// 这里是 1→N 分发的关键：每个订阅者只生成一个 delivery（持有共享消息的引用），
// 不做 payload/属性拷贝。
func (c *Core) deliver(msg *Message) {
	if targetID := msg.targetClientID(); targetID != "" {
		c.deliverToTarget(msg, targetID)
		return
	}

	subscribers := c.subTree.matchTopic(msg.Topic())

	type target struct {
		client *Client
		opts   packet.SubscribeOptions
	}

	targets := make([]target, 0, len(subscribers))

	c.clientsMu.RLock()
	for clientID, subOpts := range subscribers {
		client, ok := c.clients[clientID]
		if !ok {
			continue
		}
		// NoLocal：消息来源是订阅者自己时不投递
		if subOpts.NoLocal && msg.sourceClientID() == clientID {
			c.logger.Debug("Skipping delivery due to NoLocal option",
				zap.String("clientID", clientID),
				zap.String("topic", msg.Topic()))
			continue
		}
		targets = append(targets, target{client: client, opts: subOpts})
	}
	c.clientsMu.RUnlock()

	for _, t := range targets {
		// 有效 QoS 取订阅 QoS 与发布 QoS 的较小值；
		// RetainAsPublished=false（默认）时实时转发不携带 Retain 标志，
		// 因订阅而重放的保留消息由 sendRetainedMessages 统一置为 true。
		qos := min(t.opts.QoS, msg.QoS)
		retain := t.opts.RetainAsPublished && msg.Retain

		if err := t.client.enqueueDelivery(msg, qos, retain); err != nil {
			c.logger.Debug("message delivery failed",
				zap.String("clientID", t.client.ID),
				zap.String("topic", msg.Topic()),
				zap.Error(err))
		}
	}
}

// deliverToTarget 投递消息给指定的目标客户端
func (c *Core) deliverToTarget(msg *Message, targetID string) {
	c.clientsMu.RLock()
	client, exists := c.clients[targetID]
	c.clientsMu.RUnlock()

	if !exists {
		c.logger.Debug("Target client not found",
			zap.String("targetClientID", targetID),
			zap.String("topic", msg.Topic()))
		c.stats.MessagesDropped.Add(1)
		return
	}

	if err := client.enqueueDelivery(msg, msg.QoS, msg.Retain); err != nil {
		c.logger.Debug("Target delivery failed",
			zap.String("targetClientID", targetID),
			zap.String("topic", msg.Topic()),
			zap.Error(err))
	}
}

// enqueueDelivery 为客户端生成一次投递并入队。
//
// 入队只保存 delivery（共享消息的引用 + 逐客户端参数），不复制消息内容。
// 客户端离线时同样入队：投递保留在内存队列中，重连后补发。
func (c *Client) enqueueDelivery(m *Message, qos packet.QoS, retain bool) error {
	if m.IsExpired() {
		c.core.stats.MessagesDropped.Add(1)
		return nil
	}

	// 客户端队列配额：防止单个慢消费者无限占用内存
	if !c.enforceQueueQuota() {
		return ErrClientQueueFull
	}

	d := newDelivery(m, qos, retain)

	c.core.msgs.retain(m) // 队列持有一个引用
	c.queue.pushBack(d)

	if conn := c.getConn(); conn != nil && !conn.closed.Load() {
		conn.triggerSend()
	}

	return nil
}

// enforceQueueQuota 在入队前应用客户端待发送队列配额。
// 返回 false 表示应丢弃这条新消息（drop-newest 策略）。
func (c *Client) enforceQueueQuota() bool {
	limit := c.core.options.MaxClientQueueSize
	if limit <= 0 {
		return true
	}

	if c.queue.len() < limit {
		return true
	}

	if c.core.options.ClientQueueOverflowPolicy != QueueOverflowDropOldest {
		c.core.stats.MessagesDropped.Add(1)
		c.messagesDropped.Add(1)
		c.core.logger.Debug("Message dropped, client queue quota exceeded",
			zap.String("clientID", c.ID),
			zap.Int("limit", limit),
			zap.String("policy", c.core.options.ClientQueueOverflowPolicy.String()))
		return false
	}

	dropped := 0
	for c.queue.len() >= limit {
		if !c.queue.dropOldest() {
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

// processSendQueue 发送协程：先重发超时未确认的消息，再按 FIFO 发送新消息。
//
// 队列本身就在内存中，不需要"从持久化队列补货"这一层；窗口（客户端接收窗口）用
// 待确认条数衡量，ACK 与入队都会唤醒本协程。
func (c *conn) processSendQueue() {
	// 正常情况下由 triggerSend 置位；直接调用（测试）时也保证内部 triggerSend 只打标记，
	// 不会启动第二个发送协程。
	c.processing.Store(true)

	for {
		c.pendingSend.Store(false)
		aborted := false

		for !c.closed.Load() {
			// 1) 重发：条目仍由待确认队列持有引用
			if d, ok := c.nextResend(); ok {
				err := c.sendDelivery(d)
				if err == nil {
					c.client.core.stats.MessagesSent.Add(1)
					c.client.messagesSent.Add(1)
					continue
				}
				if errors.Is(err, errDeliveryDropped) {
					// 过期/超长/被拒：从待确认队列移除并释放引用
					c.client.session.pendingAck.drop(d.packetID)
					c.client.core.stats.MessagesDropped.Add(1)
					continue
				}
				aborted = true
				break
			}

			// 2) 窗口允许时发送新消息
			if !c.windowAvailable() {
				break
			}

			d, ok := c.client.queue.popFront() // 引用所有权转移给本协程
			if !ok {
				break
			}

			err := c.sendDelivery(d)
			if err == nil {
				c.finishSent(d)
				continue
			}
			if errors.Is(err, errDeliveryDropped) {
				c.client.core.msgs.release(d.msg)
				c.client.core.stats.MessagesDropped.Add(1)
				continue
			}

			// 写失败通常意味着连接已不可用：放回队头并标记重发，
			// 交由重连后的发送协程继续（不在这里原地忙等）。
			d.dup.Store(true)
			c.client.queue.pushFront(d)
			aborted = true
			break
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

// windowAvailable 报告是否还有可用发送窗口。
//
// 窗口等于客户端声明的接收窗口，用"已发送未确认"的条数衡量；QoS0 的投递同样
// 受这一顺序窗口约束，避免绕过流控。
func (c *conn) windowAvailable() bool {
	window := int(c.sendWindow)
	if window <= 0 {
		window = defaultSendWindow
	}
	return c.client.session.pendingAck.len() < window
}

// sendDelivery 把一次投递写到连接上。
// 返回 errDeliveryDropped 表示该投递被丢弃（不是连接错误）。
func (c *conn) sendDelivery(d *delivery) error {
	m := d.msg

	if m.IsExpired() {
		c.client.core.logger.Debug("Message expired, dropping",
			zap.String("clientID", c.client.ID),
			zap.String("topic", m.Topic()),
			zap.String("packetID", d.packetID.Hex()))
		return errDeliveryDropped
	}

	frame := m.frame(d)
	if c.maxPacketSize > 0 && frame.Size() > c.maxPacketSize {
		c.client.core.logger.Warn("Message dropped: packet size exceeds client maxPacketSize",
			zap.String("clientID", c.client.ID),
			zap.String("topic", m.Topic()),
			zap.String("packetID", d.packetID.Hex()),
			zap.Uint32("size", frame.Size()),
			zap.Uint32("maxPacketSize", c.maxPacketSize))
		return errDeliveryDropped
	}

	// 注册了 OnDeliver 钩子时，钩子可能修改或拒绝消息，会破坏共享的预编码帧，
	// 因此走"深拷贝 + 完整编码"的兼容路径；未注册钩子（常见情况）走零拷贝快路径。
	if c.client.core.options.Hooks.hasOnDeliver() {
		if err := c.sendDeliveryWithHook(d); err != nil {
			return err
		}
	} else if err := c.writePublishFrame(&frame); err != nil {
		return err
	}

	d.lastSentAt.Store(time.Now().Unix())
	return nil
}

// sendDeliveryWithHook 是注册了 OnDeliver 钩子时的兼容路径：
// 钩子拿到的是独立副本，可以安全修改，不影响共享消息与其它订阅者。
func (c *conn) sendDeliveryWithHook(d *delivery) error {
	pub := d.msg.Packet.DeepCopy()
	pub.Dup = d.dup.Load()
	pub.QoS = d.qos
	pub.Retain = d.retain
	pub.PacketID = d.packetID

	deliverCtx := &DeliverContext{
		ClientID: c.client.ID,
		Packet:   &pub,
		Identity: c.client.session.getIdentity(),
	}
	if !c.client.core.options.Hooks.callOnDeliver(deliverCtx) {
		return errDeliveryDropped
	}

	return c.writePacket(&pub)
}

// finishSent 处理发送成功后的所有权转移与统计。
func (c *conn) finishSent(d *delivery) {
	if d.qos == packet.QoS1 {
		// QoS1 需要等待 PUBACK：引用转移给待确认队列
		c.client.session.pendingAck.pushBack(d)
	} else {
		// QoS0 已写出，释放引用
		c.client.core.msgs.release(d.msg)
	}

	c.client.messagesSent.Add(1)
	c.client.core.stats.MessagesSent.Add(1)
}

// addResend 排队一次重发（同一 PacketID 只排一次）。
func (c *conn) addResend(d *delivery) bool {
	c.resendMu.Lock()
	defer c.resendMu.Unlock()

	if c.resendSet == nil {
		c.resendSet = make(map[packet.PacketID]struct{})
	}
	if _, exists := c.resendSet[d.packetID]; exists {
		return false
	}

	c.resendSet[d.packetID] = struct{}{}
	c.resendIDs = append(c.resendIDs, d)
	return true
}

// nextResend 取出下一个待重发的投递；期间已被 ACK 的条目会被跳过。
func (c *conn) nextResend() (*delivery, bool) {
	c.resendMu.Lock()
	defer c.resendMu.Unlock()

	for len(c.resendIDs) > 0 {
		d := c.resendIDs[0]
		c.resendIDs[0] = nil
		c.resendIDs = c.resendIDs[1:]
		delete(c.resendSet, d.packetID)

		// ACK 可能已经到达：只有仍在待确认队列中的投递才需要重发
		if c.client.session.pendingAck.get(d.packetID) != d {
			continue
		}

		d.dup.Store(true)
		d.retries.Add(1)
		return d, true
	}

	return nil, false
}

// PublishToClient 向指定客户端发送消息
//
// 如果客户端在线但未订阅该主题，仍然会将消息投递给该客户端；
// 如果客户端离线但会话仍保留，消息进入其待投递队列，重连后补发
// （与客户端侧通过 TargetClientID 发起的定向投递语义一致）。
func (c *Core) PublishToClient(clientID string, topic string, payload []byte, options PublishOptions) error {
	// 与客户端发布保持一致：非法主题无法被任何订阅匹配，必须拒绝。
	if !packet.ValidateTopicName(topic) {
		return ErrInvalidTopic
	}

	c.clientsMu.RLock()
	client, exists := c.clients[clientID]
	c.clientsMu.RUnlock()

	if !exists {
		return NewClientNotFoundError(clientID)
	}

	pub := c.newServerPublishPacket(topic, payload, clientID, options)
	msg, err := newMessage(pub)
	if err != nil {
		return err
	}

	c.msgs.put(msg)
	defer c.msgs.release(msg)

	return client.enqueueDelivery(msg, msg.QoS, msg.Retain)
}

// Broadcast 向所有在线客户端广播消息。
//
// 注意：这是即时的、仅在线客户的扇出，不进入订阅树，也不为离线会话入队。
// 需要按订阅关系投递（含离线会话入队）时请使用主题发布（Publish）。
func (c *Core) Broadcast(topic string, payload []byte, options PublishOptions) int {
	if !packet.ValidateTopicName(topic) {
		c.logger.Warn("Broadcast rejected, invalid topic name", zap.String("topic", topic))
		return 0
	}

	c.clientsMu.RLock()
	clients := make([]*Client, 0, len(c.clients))
	for _, client := range c.clients {
		if !client.Closed() {
			clients = append(clients, client)
		}
	}
	c.clientsMu.RUnlock()

	pub := c.newServerPublishPacket(topic, payload, "", options)
	msg, err := newMessage(pub)
	if err != nil {
		c.logger.Warn("Broadcast rejected, encode failed",
			zap.String("topic", topic),
			zap.Error(err))
		return 0
	}

	c.msgs.put(msg)
	defer c.msgs.release(msg)

	c.handleRetainedMessage(msg)

	successCount := 0
	for _, client := range clients {
		if err := client.enqueueDelivery(msg, msg.QoS, msg.Retain); err == nil {
			successCount++
		}
	}

	return successCount
}
