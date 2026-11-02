package core

import (
	"sync"

	"github.com/snple/beacon/packet"
)

// messageStore 保存 broker 已接收、尚未完成投递的消息，key 为 broker 生成的唯一 ID。
//
// 引用计数规则：
//   - put 注册时引用计数为 1，该引用由发布路径交给全局分发队列；
//   - 每个额外持有者（客户端队列条目、待确认条目、保留消息）retain 一次；
//   - 持有者用完 release，归零即从 map 移除。
//
// 归零后消息内容（payload 等）随最后一个引用消失被 GC 回收，因此 broker 只需
// 保证「引用计数正确」，不需要（也无法）依赖 GC 自行判断 map 中的条目是否还有用。
type messageStore struct {
	mu    sync.Mutex
	msgs  map[packet.PacketID]*Message
	bytes int64
}

func newMessageStore() *messageStore {
	return &messageStore{msgs: make(map[packet.PacketID]*Message)}
}

// put 注册一条新消息，引用计数置为 1（调用方接管这个引用）。
func (s *messageStore) put(m *Message) {
	if m == nil {
		return
	}

	s.mu.Lock()
	m.refs = 1
	s.msgs[m.ID] = m
	s.bytes += m.Size()
	s.mu.Unlock()
}

// retain 为一个已知的消息增加引用（调用方持有该指针时避免查表）。
func (s *messageStore) retain(m *Message) {
	if m == nil {
		return
	}

	s.mu.Lock()
	m.refs++
	s.mu.Unlock()
}

// take 把队列持有的引用转移给调用方（引用计数不变）。
// 调用方随后必须调用 release 归还这个引用。
func (s *messageStore) take(id packet.PacketID) *Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.msgs[id]
}

// release 释放一个引用；归零时从 map 移除。
func (s *messageStore) release(m *Message) {
	if m == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	m.refs--
	if m.refs > 0 {
		return
	}

	// 只在确实登记过时才扣减内存水位，避免重复 release 造成负数
	if _, ok := s.msgs[m.ID]; ok {
		delete(s.msgs, m.ID)
		s.bytes -= m.Size()
	}
	if m.refs < 0 {
		m.refs = 0
	}
}

// stats 返回当前消息条数与近似内存占用。
func (s *messageStore) stats() (count int, bytes int64) {
	s.mu.Lock()
	count = len(s.msgs)
	bytes = max(s.bytes, 0)
	s.mu.Unlock()
	return count, bytes
}
