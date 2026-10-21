package core

import (
	"strings"
	"sync"
	"time"

	"github.com/snple/beacon/packet"
)

// retainEntry 保留消息索引条目（仅存储索引信息，不存储消息本身）
type retainEntry struct {
	topic      string // 消息主题
	expiryTime int64  // 过期时间 (Unix 时间戳，0 表示永不过期)
}

// retainNode 保留消息树节点
type retainNode struct {
	children map[string]*retainNode
	entry    *retainEntry // 该节点的保留消息索引
}

func newRetainNode() *retainNode {
	return &retainNode{
		children: make(map[string]*retainNode),
	}
}

// retainStore 保留消息索引存储 - 使用树结构优化匹配
// 设计原则：只存储 topic 索引和过期时间，消息内容存储在 messageStore
type retainStore struct {
	root  *retainNode
	count int
	mu    sync.RWMutex
}

// NewretainStore 创建新的保留消息存储
func newRetainStore() *retainStore {
	return &retainStore{
		root: newRetainNode(),
	}
}

// set 设置保留消息索引
// 参数 expiryTime: 过期时间 (Unix 时间戳，0 表示永不过期)
func (s *retainStore) set(topic string, expiryTime int64) error {
	if topic == "" {
		return ErrInvalidRetainMessage
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	node := s.root
	startIdx := 0
	for startIdx < len(topic) {
		endIdx := strings.IndexByte(topic[startIdx:], '/')
		var part string
		if endIdx == -1 {
			part = topic[startIdx:]
			startIdx = len(topic)
		} else {
			endIdx += startIdx
			part = topic[startIdx:endIdx]
			startIdx = endIdx + 1
		}

		if node.children[part] == nil {
			node.children[part] = newRetainNode()
		}
		node = node.children[part]
	}

	if node.entry == nil {
		s.count++
	}

	node.entry = &retainEntry{
		topic:      topic,
		expiryTime: expiryTime,
	}

	return nil
}

// exists 检查主题是否存在保留消息索引
func (s *retainStore) exists(topic string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node := s.root
	startIdx := 0
	for startIdx < len(topic) {
		endIdx := strings.IndexByte(topic[startIdx:], '/')
		var part string
		if endIdx == -1 {
			part = topic[startIdx:]
			startIdx = len(topic)
		} else {
			endIdx += startIdx
			part = topic[startIdx:endIdx]
			startIdx = endIdx + 1
		}

		node = node.children[part]
		if node == nil {
			return false
		}
	}
	return node.entry != nil
}

// getEntry 获取保留消息索引条目
func (s *retainStore) getEntry(topic string) *retainEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node := s.root
	startIdx := 0
	for startIdx < len(topic) {
		endIdx := strings.IndexByte(topic[startIdx:], '/')
		var part string
		if endIdx == -1 {
			part = topic[startIdx:]
			startIdx = len(topic)
		} else {
			endIdx += startIdx
			part = topic[startIdx:endIdx]
			startIdx = endIdx + 1
		}

		node = node.children[part]
		if node == nil {
			return nil
		}
	}
	return node.entry
}

// matchTopics 匹配主题模式，返回所有匹配的主题列表
// 同时过滤掉已过期的条目
//
// 通配符语义：* 匹配单层；** 匹配零个或多个层级（可出现在任意位置）
func (s *retainStore) matchTopics(pattern string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().Unix()
	var result []string
	result = s.matchNodeTopics(s.root, splitTopic(pattern), 0, result, now)
	return result
}

func (s *retainStore) remove(topic string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	node := s.root
	path := []*retainNode{node}
	var parts []string

	startIdx := 0
	for startIdx < len(topic) {
		endIdx := strings.IndexByte(topic[startIdx:], '/')
		var part string
		if endIdx == -1 {
			part = topic[startIdx:]
			startIdx = len(topic)
		} else {
			endIdx += startIdx
			part = topic[startIdx:endIdx]
			startIdx = endIdx + 1
		}

		node = node.children[part]
		if node == nil {
			return
		}
		path = append(path, node)
		parts = append(parts, part)
	}

	if node.entry == nil {
		return
	}
	node.entry = nil
	s.count--

	// 自底向上剪枝：保留消息删除后不应留下空节点
	for i := len(path) - 1; i > 0; i-- {
		n := path[i]
		if n.entry != nil || len(n.children) > 0 {
			return
		}
		delete(path[i-1].children, parts[i-1])
	}
}

// matchNodeTopics 递归匹配节点，返回主题列表
// parts 为模式按 '/' 切分的层级数组，idx 为当前处理到的层级下标
func (s *retainStore) matchNodeTopics(node *retainNode, parts []string, idx int, result []string, now int64) []string {
	if node == nil {
		return result
	}

	// 模式已耗尽：当前节点的主题匹配；** 子节点（** 匹配零层）的主题同样匹配
	if idx >= len(parts) {
		if node.entry != nil && !s.isExpired(node.entry, now) {
			result = append(result, node.entry.topic)
		}
		if hashNode := node.children[packet.TopicWildcardMulti]; hashNode != nil {
			result = s.collectAllTopics(hashNode, result, now)
		}
		return result
	}

	part := parts[idx]
	switch part {
	case packet.TopicWildcardMulti: // **
		// ** 只能作为最后一层；出现中缀的过滤器非法，不匹配任何主题
		if idx != len(parts)-1 {
			return result
		}
		// 匹配当前节点及所有后代（等价于零个或多个层级）
		result = s.collectAllTopics(node, result, now)
	case packet.TopicWildcardSingle: // *
		// * 匹配当前层级的所有子节点
		for _, child := range node.children {
			result = s.matchNodeTopics(child, parts, idx+1, result, now)
		}
	default:
		// 精确匹配
		if child := node.children[part]; child != nil {
			result = s.matchNodeTopics(child, parts, idx+1, result, now)
		}
	}
	return result
}

// collectAllTopics 收集节点及所有子节点的主题
func (s *retainStore) collectAllTopics(node *retainNode, result []string, now int64) []string {
	if node.entry != nil && !s.isExpired(node.entry, now) {
		result = append(result, node.entry.topic)
	}
	for _, child := range node.children {
		result = s.collectAllTopics(child, result, now)
	}
	return result
}

// isExpired 检查条目是否过期
func (s *retainStore) isExpired(entry *retainEntry, now int64) bool {
	if entry == nil || entry.expiryTime == 0 {
		return false
	}
	return now > entry.expiryTime
}

// cleanupExpired 清理过期的保留消息索引，返回清理的数量和已清理的主题列表
func (s *retainStore) cleanupExpired() (int, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	var expiredTopics []string
	s.cleanupExpiredNode(s.root, now, &expiredTopics)
	s.count -= len(expiredTopics)
	return len(expiredTopics), expiredTopics
}

// cleanupExpiredNode 递归清理过期节点
func (s *retainStore) cleanupExpiredNode(node *retainNode, now int64, expiredTopics *[]string) {
	if node.entry != nil && s.isExpired(node.entry, now) {
		*expiredTopics = append(*expiredTopics, node.entry.topic)
		node.entry = nil
	}
	for part, child := range node.children {
		s.cleanupExpiredNode(child, now, expiredTopics)
		// 自底向上剪枝：过期条目清理后删除空节点
		if child.entry == nil && len(child.children) == 0 {
			delete(node.children, part)
		}
	}
}

// Clear 清除所有保留消息
func (s *retainStore) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.root = newRetainNode()
	s.count = 0
}

// matchTopic 检查主题是否匹配模式（供测试/工具场景使用）
// 支持 * (单层通配符) 和 ** (多层通配符，匹配零个或多个层级，可出现在任意位置)
func matchTopic(pattern, topic string) bool {
	return matchTopicParts(splitTopic(pattern), splitTopic(topic), 0, 0)
}

// matchTopicParts 按层级递归匹配主题
// parts 为模式层级，topic 为主题层级
func matchTopicParts(parts, topic []string, pi, ti int) bool {
	if pi == len(parts) {
		return ti == len(topic)
	}

	part := parts[pi]
	if part == packet.TopicWildcardMulti {
		// ** 只能作为最后一层：匹配剩余全部层级（含零层）；非末层视为非法模式
		return pi == len(parts)-1
	}

	if ti == len(topic) {
		return false
	}

	if part == packet.TopicWildcardSingle || part == topic[ti] {
		return matchTopicParts(parts, topic, pi+1, ti+1)
	}

	return false
}
