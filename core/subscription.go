package core

import (
	"strings"
	"sync"

	"github.com/snple/beacon/packet"
)

// 对象池：减少 SubscriptionInfo slice 的内存分配
var subInfoPool = sync.Pool{
	New: func() interface{} {
		s := make([]SubscriptionInfo, 0, 16)
		return &s
	},
}

// SubscriptionInfo 订阅信息
type SubscriptionInfo struct {
	ClientID string
	QoS      packet.QoS
}

// SubscriptionTree 订阅树 (用于主题匹配)
type SubscriptionTree struct {
	root *topicNode
	mu   sync.RWMutex
}

type topicNode struct {
	children    map[string]*topicNode
	subscribers map[string]packet.QoS // clientID -> QoS
}

func newTopicNode() *topicNode {
	return &topicNode{
		children:    make(map[string]*topicNode),
		subscribers: make(map[string]packet.QoS),
	}
}

// NewSubscriptionTree 创建新的订阅树
func NewSubscriptionTree() *SubscriptionTree {
	return &SubscriptionTree{
		root: newTopicNode(),
	}
}

// Add 添加订阅，返回 true 如果是新订阅
func (t *SubscriptionTree) Add(clientID, topic string, qos packet.QoS) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	parts := splitTopic(topic)
	node := t.root

	for _, part := range parts {
		if node.children[part] == nil {
			node.children[part] = newTopicNode()
		}
		node = node.children[part]
	}

	_, exists := node.subscribers[clientID]
	node.subscribers[clientID] = qos
	return !exists
}

// Remove 移除订阅，返回 true 如果订阅存在并被移除
func (t *SubscriptionTree) Remove(clientID, topic string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

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

// RemoveClient 移除客户端的所有订阅
func (t *SubscriptionTree) RemoveClient(clientID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.removeClientFromNode(t.root, clientID)
}

func (t *SubscriptionTree) removeClientFromNode(node *topicNode, clientID string) {
	delete(node.subscribers, clientID)
	for _, child := range node.children {
		t.removeClientFromNode(child, clientID)
	}
}

// Match 匹配主题，返回所有订阅者
func (t *SubscriptionTree) Match(topic string) []SubscriptionInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// 使用对象池获取 slice，减少内存分配
	resultPtr := subInfoPool.Get().(*[]SubscriptionInfo)
	result := (*resultPtr)[:0] // 重置长度但保留容量

	t.matchNode(t.root, topic, 0, &result)

	// 复制结果并归还池
	finalResult := make([]SubscriptionInfo, len(result))
	copy(finalResult, result)
	subInfoPool.Put(resultPtr)

	return finalResult
}

func (t *SubscriptionTree) matchNode(node *topicNode, topic string, startIdx int, result *[]SubscriptionInfo) {
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
			for clientID, qos := range child.subscribers {
				*result = append(*result, SubscriptionInfo{
					ClientID: clientID,
					QoS:      qos,
				})
			}
			// 检查精确匹配后的 ** 通配符
			if multiNode := child.children[packet.TopicWildcardMulti]; multiNode != nil {
				for clientID, qos := range multiNode.subscribers {
					*result = append(*result, SubscriptionInfo{
						ClientID: clientID,
						QoS:      qos,
					})
				}
			}
		}
		// * 单层通配符匹配
		if plusNode := node.children[packet.TopicWildcardSingle]; plusNode != nil {
			for clientID, qos := range plusNode.subscribers {
				*result = append(*result, SubscriptionInfo{
					ClientID: clientID,
					QoS:      qos,
				})
			}
			// 检查 * 后的 ** 通配符
			if multiNode := plusNode.children[packet.TopicWildcardMulti]; multiNode != nil {
				for clientID, qos := range multiNode.subscribers {
					*result = append(*result, SubscriptionInfo{
						ClientID: clientID,
						QoS:      qos,
					})
				}
			}
		}
		// ** 多层通配符匹配
		if hashNode := node.children[packet.TopicWildcardMulti]; hashNode != nil {
			for clientID, qos := range hashNode.subscribers {
				*result = append(*result, SubscriptionInfo{
					ClientID: clientID,
					QoS:      qos,
				})
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
		for clientID, qos := range hashNode.subscribers {
			*result = append(*result, SubscriptionInfo{
				ClientID: clientID,
				QoS:      qos,
			})
		}
	}
}

// Count 返回订阅总数
func (t *SubscriptionTree) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.countNode(t.root)
}

func (t *SubscriptionTree) countNode(node *topicNode) int {
	count := len(node.subscribers)
	for _, child := range node.children {
		count += t.countNode(child)
	}
	return count
}

func splitTopic(topic string) []string {
	if topic == "" {
		return nil
	}
	return strings.Split(topic, "/")
}
