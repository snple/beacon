package core

import (
	"github.com/snple/beacon/packet"
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
// 与客户端发布的差异：管理面发布是受信来源，**不调用 OnPublish 钩子**
// （发布侧 ACL 不生效），也不受每客户端发布速率限制；订阅侧的 OnDeliver 仍生效。
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
			Reason:   "publish",
			Retained: res.Retained,
		})
	}

	return res, nil
}
