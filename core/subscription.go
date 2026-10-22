package core

import (
	"strings"
	"sync"

	"github.com/snple/beacon/packet"
)

// ============================================================================
// SubInfo - 订阅信息
// ============================================================================

// SubInfo 订阅信息
// type SubInfo struct {
// 	ClientID string
// 	QoS      packet.QoS
// }

// ============================================================================
// SubTree - 订阅树（用于主题匹配）
// ============================================================================

// subTree 订阅树，管理主题订阅和通配符匹配
// 支持批量订阅/取消订阅、主题匹配等功能
type subTree struct {
	root *topicNode
	mu   sync.RWMutex
}

// topicNode 主题树节点
type topicNode struct {
	children    map[string]*topicNode
	subscribers map[string]packet.SubscribeOptions // clientID -> SubscribeOptions
}

func newTopicNode() *topicNode {
	return &topicNode{
		children:    make(map[string]*topicNode),
		subscribers: make(map[string]packet.SubscribeOptions),
	}
}

// newSubTree 创建新的订阅树
func newSubTree() *subTree {
	return &subTree{
		root: newTopicNode(),
	}
}

// ============================================================================
// 订阅/取消订阅操作
// ============================================================================

// subscribe 添加单个订阅
// 返回 true 表示是新订阅，false 表示更新已有订阅
func (t *subTree) subscribe(clientID, topic string, opts packet.SubscribeOptions) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.addSubscription(clientID, topic, opts)
}

// subscribeMultiple 批量订阅主题
// 返回每个主题的订阅结果
func (t *subTree) subscribeMultiple(clientID string, subscriptions []packet.Subscription) map[string]packet.ReasonCode {
	t.mu.Lock()
	defer t.mu.Unlock()

	results := make(map[string]packet.ReasonCode, len(subscriptions))

	for _, sub := range subscriptions {
		// 验证主题
		if !validateTopicFilter(sub.Topic) {
			results[sub.Topic] = packet.ReasonTopicFilterInvalid
			continue
		}

		t.addSubscription(clientID, sub.Topic, sub.Options)
		switch sub.Options.QoS {
		case packet.QoS0:
			results[sub.Topic] = packet.ReasonGrantedQoS0
		case packet.QoS1:
			results[sub.Topic] = packet.ReasonGrantedQoS1
		default:
			results[sub.Topic] = packet.ReasonQoSNotSupported
		}
	}

	return results
}

// unsubscribe 移除单个订阅
// 返回 true 表示订阅存在并被移除，false 表示订阅不存在
func (t *subTree) unsubscribe(clientID, topic string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.removeSubscription(clientID, topic)
}

// unsubscribeMultiple 批量取消订阅
// 返回每个主题的取消订阅结果
func (t *subTree) unsubscribeMultiple(clientID string, topics []string) map[string]packet.ReasonCode {
	t.mu.Lock()
	defer t.mu.Unlock()

	results := make(map[string]packet.ReasonCode, len(topics))

	for _, topic := range topics {
		if t.removeSubscription(clientID, topic) {
			results[topic] = packet.ReasonSuccess
		} else {
			results[topic] = packet.ReasonNoSubscriptionExisted
		}
	}

	return results
}

// unsubscribeClient 移除客户端的所有订阅
// 返回被取消的订阅数量
func (t *subTree) unsubscribeClient(clientID string) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.removeClientFromNode(t.root, clientID)
}

// ============================================================================
// 主题匹配
// ============================================================================

// matchTopic 匹配主题，返回订阅者 map（自动去重，保留最高 QoS 的选项）
// 返回 map[clientID]SubscribeOptions
//
// 通配符语义：
//   - *  匹配单个层级
//   - ** 匹配零个或多个剩余层级，且只能作为最后一层
//     （ValidateTopicFilter 拒绝 a/**/b 这类中缀写法）
func (t *subTree) matchTopic(topic string) map[string]packet.SubscribeOptions {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]packet.SubscribeOptions)
	t.matchNode(t.root, topic, 0, result)
	return result
}

// collectSubscribers 将 subscribers 合并到 result（自动去重，保留最高 QoS）
func collectSubscribers(subscribers map[string]packet.SubscribeOptions, result map[string]packet.SubscribeOptions) {
	for clientID, opts := range subscribers {
		if existingOpts, exists := result[clientID]; !exists || opts.QoS > existingOpts.QoS {
			result[clientID] = opts
		}
	}
}

func (t *subTree) matchNode(node *topicNode, topic string, startIdx int, result map[string]packet.SubscribeOptions) {
	if node == nil {
		return
	}

	// 主题已耗尽：当前节点的订阅者匹配；** 子节点（** 匹配零层）的订阅者同样匹配
	if startIdx >= len(topic) {
		collectSubscribers(node.subscribers, result)
		if hashNode := node.children[packet.TopicWildcardMulti]; hashNode != nil {
			collectSubscribers(hashNode.subscribers, result)
		}
		return
	}

	// 找下一个分隔符
	endIdx := strings.IndexByte(topic[startIdx:], '/')
	var part string
	var nextIdx int

	if endIdx == -1 {
		part = topic[startIdx:]
		nextIdx = len(topic)
	} else {
		endIdx += startIdx
		part = topic[startIdx:endIdx]
		nextIdx = endIdx + 1
	}

	// 精确匹配
	if child := node.children[part]; child != nil {
		t.matchNode(child, topic, nextIdx, result)
	}

	// * 单层通配符匹配
	if plusNode := node.children[packet.TopicWildcardSingle]; plusNode != nil {
		t.matchNode(plusNode, topic, nextIdx, result)
	}

	// ** 多层通配符：只能作为最后一层（由 ValidateTopicFilter 保证），
	// 因此它匹配当前层及其后的所有层级（含零层），无需再枚举后续层级。
	if hashNode := node.children[packet.TopicWildcardMulti]; hashNode != nil {
		collectSubscribers(hashNode.subscribers, result)
	}
}

// ============================================================================
// 查询方法
// ============================================================================

// subscriptionCount 返回订阅总数
func (t *subTree) subscriptionCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.countNode(t.root)
}

// getClientTopics 获取客户端订阅的所有主题
func (t *subTree) getClientTopics(clientID string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var topics []string
	t.collectClientTopics(t.root, clientID, "", &topics)
	return topics
}

// ============================================================================
// 内部辅助方法（需持有锁）
// ============================================================================

// addSubscription 添加订阅
// 返回 true 表示是新订阅
func (t *subTree) addSubscription(clientID, topic string, opts packet.SubscribeOptions) bool {
	parts := splitTopic(topic)
	node := t.root

	for _, part := range parts {
		if node.children[part] == nil {
			node.children[part] = newTopicNode()
		}
		node = node.children[part]
	}

	_, exists := node.subscribers[clientID]
	node.subscribers[clientID] = opts
	return !exists
}

// removeSubscription 移除订阅
// 返回 true 表示成功移除
func (t *subTree) removeSubscription(clientID, topic string) bool {
	parts := splitTopic(topic)
	path := make([]*topicNode, 0, len(parts)+1)

	node := t.root
	path = append(path, node)

	for _, part := range parts {
		child := node.children[part]
		if child == nil {
			return false
		}
		node = child
		path = append(path, node)
	}

	_, exists := node.subscribers[clientID]
	if !exists {
		return false
	}
	delete(node.subscribers, clientID)

	t.pruneEmptyPath(path, parts)
	return true
}

// pruneEmptyPath 自底向上删除不再包含订阅者或子节点的空节点，避免主题 churn 造成内存泄漏。
// path[0] 为根节点，parts[i] 是 path[i+1] 对应的层级名。
func (t *subTree) pruneEmptyPath(path []*topicNode, parts []string) {
	for i := len(path) - 1; i > 0; i-- {
		node := path[i]
		if len(node.subscribers) > 0 || len(node.children) > 0 {
			return
		}
		delete(path[i-1].children, parts[i-1])
	}
}

// removeClientFromNode 递归移除客户端的所有订阅
// 返回移除的订阅数量
func (t *subTree) removeClientFromNode(node *topicNode, clientID string) int {
	count := 0
	if _, exists := node.subscribers[clientID]; exists {
		delete(node.subscribers, clientID)
		count++
	}
	for part, child := range node.children {
		count += t.removeClientFromNode(child, clientID)
		// 自底向上剪枝：删除不再使用的空节点
		if len(child.subscribers) == 0 && len(child.children) == 0 {
			delete(node.children, part)
		}
	}
	return count
}

// countNode 递归计算节点的订阅总数（需持有锁）
func (t *subTree) countNode(node *topicNode) int {
	count := len(node.subscribers)
	for _, child := range node.children {
		count += t.countNode(child)
	}
	return count
}

// collectClientTopics 收集客户端订阅的所有主题（需持有锁）
func (t *subTree) collectClientTopics(node *topicNode, clientID, prefix string, topics *[]string) {
	if _, exists := node.subscribers[clientID]; exists {
		if prefix == "" {
			// 根节点的订阅（不应该存在，但防御性处理）
			*topics = append(*topics, "/")
		} else {
			*topics = append(*topics, prefix)
		}
	}

	for part, child := range node.children {
		var childPrefix string
		if prefix == "" {
			childPrefix = part
		} else {
			childPrefix = prefix + "/" + part
		}
		t.collectClientTopics(child, clientID, childPrefix, topics)
	}
}

// ============================================================================
// 工具函数
// ============================================================================

func splitTopic(topic string) []string {
	if topic == "" {
		return nil
	}
	parts := strings.Split(topic, "/")

	// 折叠连续的 **（防御性处理；合法过滤器中 ** 只能出现在末层，
	// 因此连续 ** 主要出现在直接调用内部工具函数时）。
	dst := parts[:0]
	for _, p := range parts {
		if len(dst) > 0 && dst[len(dst)-1] == packet.TopicWildcardMulti && p == packet.TopicWildcardMulti {
			continue
		}
		dst = append(dst, p)
	}
	return dst
}

// validateTopicFilter 验证主题过滤器
func validateTopicFilter(topic string) bool {
	if len(topic) == 0 || len(topic) > packet.MaxTopicLength {
		return false
	}

	// 不允许空字符
	if strings.ContainsRune(topic, 0) {
		return false
	}

	return true
}
