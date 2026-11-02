package core

import (
	"strings"
	"sync"
	"time"

	"github.com/snple/beacon/packet"
)

// retainEntry 保留消息条目
type retainEntry struct {
	topic      string   // 消息主题
	msg        *Message // 保留的消息（持有一个 messageStore 引用）
	expiryTime int64    // 过期时间 (Unix 时间戳，0 表示永不过期)
}

// retainNode 保留消息树节点
type retainNode struct {
	children map[string]*retainNode
	entry    *retainEntry // 该节点的保留消息
}

func newRetainNode() *retainNode {
	return &retainNode{
		children: make(map[string]*retainNode),
	}
}

// retainStore 保留消息存储 - 使用树结构优化匹配
//
// 条目直接引用 messageStore 中的共享消息（并持有一个引用），
// 因此保留消息同样不会在内存中保存多份。
type retainStore struct {
	root  *retainNode
	store *messageStore
	mu    sync.RWMutex
}

// newRetainStore 创建新的保留消息存储
func newRetainStore(store *messageStore) *retainStore {
	return &retainStore{
		root:  newRetainNode(),
		store: store,
	}
}

// set 设置保留消息。
// 调用方必须已经为 msg 持有一个 messageStore 引用（所有权转移给 retainStore）。
// expiryTime: 过期时间 (Unix 时间戳，0 表示永不过期)。
func (s *retainStore) set(topic string, msg *Message, expiryTime int64) error {
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

	// 覆盖旧值时释放它持有的引用
	old := node.entry
	node.entry = &retainEntry{
		topic:      topic,
		msg:        msg,
		expiryTime: expiryTime,
	}

	if old != nil {
		s.store.release(old.msg)
	}

	return nil
}

// getEntry 获取保留消息条目
func (s *retainStore) getEntry(topic string) *retainEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lookupLocked(topic)
}

// matchTopics 匹配主题模式，返回所有匹配的主题列表
// 同时过滤掉已过期的条目
//
// 通配符语义：* 匹配单层；** 匹配零个或多个剩余层级，且只能作为最后一层
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

	removed := node.entry
	node.entry = nil
	s.store.release(removed.msg)

	// 自底向上剪枝：保留消息删除后不应留下空节点
	for i := len(path) - 1; i > 0; i-- {
		n := path[i]
		if n.entry != nil || len(n.children) > 0 {
			return
		}
		delete(path[i-1].children, parts[i-1])
	}
}

// matchMessages 匹配主题模式，返回匹配的保留消息。
//
// 返回的每条消息都额外持有一个引用，调用方用完后必须 release。
func (s *retainStore) matchMessages(pattern string) []*Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().Unix()
	var topics []string
	topics = s.matchNodeTopics(s.root, splitTopic(pattern), 0, topics, now)

	msgs := make([]*Message, 0, len(topics))
	for _, topic := range topics {
		if entry := s.lookupLocked(topic); entry != nil && entry.msg != nil {
			s.store.retain(entry.msg)
			msgs = append(msgs, entry.msg)
		}
	}
	return msgs
}

// lookupLocked 按主题查找条目（调用方必须持有 s.mu）。
func (s *retainStore) lookupLocked(topic string) *retainEntry {
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

// matchNodeTopics 递归匹配节点，返回主题列表
// parts 为模式按 '/' 切分的层级数组，idx 为当前处理到的层级下标。
// 语义以 packet.MatchTopic 为唯一参照（差分测试见 topic_match_consistency_test.go）。
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
	return len(expiredTopics), expiredTopics
}

// cleanupExpiredNode 递归清理过期节点
func (s *retainStore) cleanupExpiredNode(node *retainNode, now int64, expiredTopics *[]string) {
	if node.entry != nil && s.isExpired(node.entry, now) {
		*expiredTopics = append(*expiredTopics, node.entry.topic)
		s.store.release(node.entry.msg)
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

	s.releaseAllLocked(s.root)
	s.root = newRetainNode()
}

// releaseAllLocked 递归释放子树中的所有保留消息引用。
func (s *retainStore) releaseAllLocked(node *retainNode) {
	if node == nil {
		return
	}
	if node.entry != nil {
		s.store.release(node.entry.msg)
		node.entry = nil
	}
	for _, child := range node.children {
		s.releaseAllLocked(child)
	}
}
