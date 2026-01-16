package core

import (
	"strings"
	"sync"

	"github.com/snple/beacon/packet"
)

// retainNode 保留消息树节点
type retainNode struct {
	children map[string]*retainNode
	message  *Message // 该节点的保留消息
}

func newRetainNode() *retainNode {
	return &retainNode{
		children: make(map[string]*retainNode),
	}
}

// retainStore 保留消息存储 - 使用树结构优化匹配
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

// Set 设置保留消息 - 使用树结构存储
func (s *retainStore) set(topic string, msg *Message) error {
	if msg == nil || msg.Packet == nil || !msg.Packet.Retain {
		// 如果消息为 nil 或 Retain 标志为 false，则不存储
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

	if node.message == nil {
		s.count++
	}

	node.message = msg

	return nil
}

// Get 获取保留消息
func (s *retainStore) get(topic string) *Message {
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
	return node.message
}

// MatchForSubscription 获取与主题模式匹配的保留消息，支持 RetainHandling 选项
// retainHandling: 0 = 总是发送, 1 = 仅新订阅发送, 2 = 不发送
func (s *retainStore) matchForSubscription(topic string, isNewSubscription bool, retainAsPublished bool,
	retainHandling uint8) []*Message {
	// retainHandling == 2: 不发送保留消息
	if retainHandling == 2 {
		return nil
	}

	// retainHandling == 1: 仅新订阅时发送
	if retainHandling == 1 && !isNewSubscription {
		return nil
	}

	// retainHandling == 0: 总是发送 (默认)
	msgs := s.match(topic)

	// 根据 retainAsPublished 设置 Retain 标志
	if !retainAsPublished {
		// retainAsPublished == false: 发送时清除 Retain 标志
		for i := range msgs {
			msgs[i] = &Message{
				Packet:    msgs[i].Packet,
				Dup:       msgs[i].Dup,
				QoS:       msgs[i].QoS,
				Retain:    false,
				Timestamp: msgs[i].Timestamp,
			}
		}
	}

	return msgs
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
	if node.message != nil {
		node.message = nil
		s.count--
	}
}

// Match 匹配主题模式，返回所有匹配的保留消息 - 树遍历优化
func (s *retainStore) match(pattern string) []*Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Message
	result = s.matchNode(s.root, pattern, 0, result)
	return result
}

func (s *retainStore) matchNode(node *retainNode, pattern string, startIdx int, result []*Message) []*Message {
	if startIdx >= len(pattern) {
		// 到达模式末尾，收集当前节点的消息
		if node.message != nil {
			result = append(result, node.message)
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
		result = s.collectAll(node, result)
	case packet.TopicWildcardSingle: // *
		// * 匹配当前层级的所有子节点
		for _, child := range node.children {
			result = s.matchNode(child, pattern, nextIdx, result)
		}
	default:
		// 精确匹配
		if child := node.children[part]; child != nil {
			result = s.matchNode(child, pattern, nextIdx, result)
		}
	}
	return result
}

// collectAll 收集节点及所有子节点的消息
func (s *retainStore) collectAll(node *retainNode, result []*Message) []*Message {
	if node.message != nil {
		result = append(result, node.message)
	}
	for _, child := range node.children {
		result = s.collectAll(child, result)
	}
	return result
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
