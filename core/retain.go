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
func (s *retainStore) matchTopics(pattern string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().Unix()
	var result []string
	result = s.matchNodeTopics(s.root, pattern, 0, result, now)
	return result
}

func (s *retainStore) remove(topic string) {
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

		node = node.children[part]
		if node == nil {
			return
		}
	}
	if node.entry != nil {
		node.entry = nil
		s.count--
	}
}

// matchNodeTopics 递归匹配节点，返回主题列表
func (s *retainStore) matchNodeTopics(node *retainNode, pattern string, startIdx int, result []string, now int64) []string {
	if startIdx >= len(pattern) {
		// 到达模式末尾，收集当前节点的主题
		if node.entry != nil && !s.isExpired(node.entry, now) {
			result = append(result, node.entry.topic)
		}
		return result
	}

	// 找下一个分隔符
	endIdx := strings.IndexByte(pattern[startIdx:], '/')
	var part string
	var nextIdx int
	if endIdx == -1 {
		part = pattern[startIdx:]
		nextIdx = len(pattern)
	} else {
		endIdx += startIdx
		part = pattern[startIdx:endIdx]
		nextIdx = endIdx + 1
	}

	switch part {
	case packet.TopicWildcardMulti: // **
		// ** 匹配当前节点及所有子节点
		result = s.collectAllTopics(node, result, now)
	case packet.TopicWildcardSingle: // *
		// * 匹配当前层级的所有子节点
		for _, child := range node.children {
			result = s.matchNodeTopics(child, pattern, nextIdx, result, now)
		}
	default:
		// 精确匹配
		if child := node.children[part]; child != nil {
			result = s.matchNodeTopics(child, pattern, nextIdx, result, now)
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
	for _, child := range node.children {
		s.cleanupExpiredNode(child, now, expiredTopics)
	}
}

// Clear 清除所有保留消息
func (s *retainStore) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.root = newRetainNode()
	s.count = 0
}

// matchTopic 检查主题是否匹配模式 (保留用于其他场景)
// 支持 * (单层通配符) 和 ** (多层通配符)
func matchTopic(pattern, topic string) bool {
	pi, ti := 0, 0
	plen, tlen := len(pattern), len(topic)

	for pi < plen && ti < tlen {
		// 找 pattern 当前部分
		pEnd := strings.IndexByte(pattern[pi:], '/')
		var pPart string
		if pEnd == -1 {
			pPart = pattern[pi:]
			pEnd = plen
		} else {
			pEnd += pi
			pPart = pattern[pi:pEnd]
		}

		// 找 topic 当前部分
		tEnd := strings.IndexByte(topic[ti:], '/')
		var tPart string
		if tEnd == -1 {
			tPart = topic[ti:]
			tEnd = tlen
		} else {
			tEnd += ti
			tPart = topic[ti:tEnd]
		}

		switch pPart {
		case packet.TopicWildcardMulti: // **
			return true
		case packet.TopicWildcardSingle: // *
			// 匹配当前层级
		default:
			if pPart != tPart {
				return false
			}
		}

		pi = pEnd + 1
		ti = tEnd + 1
	}

	// 检查是否都处理完
	if pi >= plen && ti >= tlen {
		return true
	}

	// 模式以 ** 结尾可以匹配空
	if pi < plen && pattern[pi:] == packet.TopicWildcardMulti {
		return true
	}

	return false
}
