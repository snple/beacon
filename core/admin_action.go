package core

import (
	"strconv"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// PublishResult 描述一次发布的**入队结果**（不是投递结果）。
type PublishResult struct {
	// MatchedSubscribers 是入队瞬间匹配到的订阅者数量（已排除 NoLocal 自身）。
	// 该值是快照，与后续订阅变更存在竞态，仅用于展示。
	MatchedSubscribers int
	// Retained 表示消息是否已写入保留消息存储。
	Retained bool
	// Enqueued 表示消息是否已进入全局分发队列。
	Enqueued bool
}

// Publish 以 broker 自身身份向主题发布消息。
//
// 语义与客户端发布一致：校验主题、进入全局分发队列、按订阅树投递、
// 处理保留消息、为离线会话入队，并受全局配额约束。
//
// 分发是异步的：返回 nil 只表示"已入队"，不代表已投递；QoS1 在这里
// 也没有 PUBACK 语义。opts.PublisherID 为空表示系统发布。
func (c *Core) Publish(topic string, payload []byte, opts PublishOptions) (PublishResult, error) {
	var res PublishResult

	if !packet.ValidateTopicName(topic) {
		return res, ErrInvalidTopic
	}
	if !opts.QoS.IsValid() {
		return res, ErrInvalidQoS
	}
	if !c.Ready() {
		return res, ErrCoreNotRunning
	}

	pub := c.newServerPublishPacket(topic, payload, "", opts)
	pub.QoS = opts.QoS
	if pub.Properties != nil {
		pub.Properties.SourceClientID = opts.PublisherID
	}

	for clientID, subOpts := range c.subTree.matchTopic(topic) {
		if subOpts.NoLocal && opts.PublisherID != "" && clientID == opts.PublisherID {
			continue
		}
		res.MatchedSubscribers++
	}

	msg := c.newPublishMessage(pub)
	if !c.publish(msg) {
		return PublishResult{MatchedSubscribers: res.MatchedSubscribers}, ErrQuotaExceeded
	}

	res.Enqueued = true
	// 空负载 + Retain 表示"删除保留消息"，此时不应报告为已保留。
	res.Retained = pub.Retain && c.options.RetainEnabled && len(payload) > 0

	if opts.PublisherID != "" {
		c.emit(Event{
			Type:     EventAdminAction,
			ClientID: opts.PublisherID,
			Topic:    topic,
			Bytes:    len(payload),
			QoS:      opts.QoS,
			Actor:    opts.PublisherID,
			Extra:    map[string]string{"action": "publish", "retained": boolString(res.Retained)},
		})
	}

	return res, nil
}

func boolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// DeleteSession 删除会话：会话记录、订阅与待投递队列一并清理。
// 在线客户端会先被踢下线。返回 ClientNotFoundError 表示该 clientID 不存在。
func (c *Core) DeleteSession(clientID string) error {
	c.clientsMu.Lock()
	client, exists := c.clients[clientID]
	if exists {
		delete(c.clients, clientID)
	}
	c.clientsMu.Unlock()

	if !exists {
		return NewClientNotFoundError(clientID)
	}

	client.adminDeleted.Store(true)

	online := false
	if conn := client.getConn(); conn != nil {
		online = !conn.closed.Load()
		// detach 只切换状态；真正关闭与等待在锁外完成。
		conn.detach(packet.ReasonSessionTakenOver)
		conn.closeLocal(packet.ReasonSessionTakenOver)
		conn.wait()
	}
	c.uncountClient(client)

	c.offlineSessionsMu.Lock()
	delete(c.offlineSessions, clientID)
	c.offlineSessionsMu.Unlock()

	client.touch()
	c.cleanupClient(clientID, client)

	c.emit(Event{
		Type:     EventAdminAction,
		ClientID: clientID,
		Reason:   "delete_session",
		Extra:    map[string]string{"action": "delete_session", "was_online": boolString(online)},
	})
	c.logger.Info("Session deleted by admin",
		zap.String("clientID", clientID),
		zap.Bool("wasOnline", online))

	return nil
}

// DeleteRetained 删除单条保留消息（幂等：不存在时返回 nil）。
func (c *Core) DeleteRetained(topic string) error {
	if !packet.ValidateTopicName(topic) {
		return ErrInvalidTopic
	}
	if c.store == nil {
		return ErrCoreNotRunning
	}

	existed := c.retainStore.exists(topic)
	c.retainStore.remove(topic)

	if err := c.store.deleteRetain(topic); err != nil {
		return err
	}

	if existed {
		c.emit(Event{
			Type:   EventAdminAction,
			Topic:  topic,
			Reason: "delete_retained",
			Extra:  map[string]string{"action": "delete_retained"},
		})
		c.logger.Info("Retained message deleted by admin", zap.String("topic", topic))
	}

	return nil
}

// PurgeClientQueue 清空某客户端的待发送积压，返回清除的条数。
func (c *Core) PurgeClientQueue(clientID string) (int64, error) {
	c.clientsMu.RLock()
	client, exists := c.clients[clientID]
	c.clientsMu.RUnlock()

	if !exists {
		return 0, NewClientNotFoundError(clientID)
	}
	if client.queue == nil {
		return 0, nil
	}

	n := client.queue.Count()
	if err := client.queue.Clear(); err != nil {
		return 0, err
	}

	c.emit(Event{
		Type:     EventAdminAction,
		ClientID: clientID,
		Reason:   "purge_queue",
		Extra:    map[string]string{"action": "purge_queue", "purged": strconv.FormatInt(n, 10)},
	})
	c.logger.Info("Client queue purged by admin",
		zap.String("clientID", clientID),
		zap.Int64("purged", n))

	return n, nil
}
