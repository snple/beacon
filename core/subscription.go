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
		results[sub.Topic] = packet.ReasonSuccess
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
func (t *subTree) matchTopic(topic string) map[string]packet.SubscribeOptions {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]packet.SubscribeOptions)
	t.matchNode(t.root, topic, 0, result)
	return result
}

func (t *subTree) matchNode(node *topicNode, topic string, startIdx int, result map[string]packet.SubscribeOptions) {
	// 找下一个分隔符
	endIdx := strings.IndexByte(topic[startIdx:], '/')
	var part string
	var isLast bool

	if endIdx == -1 {
		part = topic[startIdx:]
		isLast = true
	} else {
		endIdx += startIdx
		part = topic[startIdx:endIdx]
		isLast = false
	}

	if isLast {
		// 到达主题末尾，收集订阅者
		// 精确匹配
		if child := node.children[part]; child != nil {
			for clientID, opts := range child.subscribers {
				if existingOpts, exists := result[clientID]; !exists || opts.QoS > existingOpts.QoS {
					result[clientID] = opts
				}
			}
			// 检查精确匹配后的 ** 通配符
			if multiNode := child.children[packet.TopicWildcardMulti]; multiNode != nil {
				for clientID, opts := range multiNode.subscribers {
					if existingOpts, exists := result[clientID]; !exists || opts.QoS > existingOpts.QoS {
						result[clientID] = opts
					}
				}
			}
		}
		// * 单层通配符匹配
		if plusNode := node.children[packet.TopicWildcardSingle]; plusNode != nil {
			for clientID, opts := range plusNode.subscribers {
				if existingOpts, exists := result[clientID]; !exists || opts.QoS > existingOpts.QoS {
					result[clientID] = opts
				}
			}
			// 检查 * 后的 ** 通配符
			if multiNode := plusNode.children[packet.TopicWildcardMulti]; multiNode != nil {
				for clientID, opts := range multiNode.subscribers {
					if existingOpts, exists := result[clientID]; !exists || opts.QoS > existingOpts.QoS {
						result[clientID] = opts
					}
				}
			}
		}
		// ** 多层通配符匹配
		if hashNode := node.children[packet.TopicWildcardMulti]; hashNode != nil {
			for clientID, opts := range hashNode.subscribers {
				if existingOpts, exists := result[clientID]; !exists || opts.QoS > existingOpts.QoS {
					result[clientID] = opts
				}
			}
		}
		return
	}

	// 精确匹配
	if child := node.children[part]; child != nil {
		t.matchNode(child, topic, endIdx+1, result)
	}

	// * 单层通配符匹配
	if plusNode := node.children[packet.TopicWildcardSingle]; plusNode != nil {
		t.matchNode(plusNode, topic, endIdx+1, result)
	}

	// ** 多层通配符匹配 (匹配剩余所有层级)
	if hashNode := node.children[packet.TopicWildcardMulti]; hashNode != nil {
		for clientID, opts := range hashNode.subscribers {
			if existingOpts, exists := result[clientID]; !exists || opts.QoS > existingOpts.QoS {
				result[clientID] = opts
			}
		}
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
	node := t.root

	for _, part := range parts {
		if node.children[part] == nil {
			return false
		}
		node = node.children[part]
	}

	_, exists := node.subscribers[clientID]
	if exists {
		delete(node.subscribers, clientID)
	}
	return exists
}

// removeClientFromNode 递归移除客户端的所有订阅
// 返回移除的订阅数量
func (t *subTree) removeClientFromNode(node *topicNode, clientID string) int {
	count := 0
	if _, exists := node.subscribers[clientID]; exists {
		delete(node.subscribers, clientID)
		count++
	}
	for _, child := range node.children {
		count += t.removeClientFromNode(child, clientID)
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
	return strings.Split(topic, "/")
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
