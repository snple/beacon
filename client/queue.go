package client

import (
	"container/list"
	"fmt"
	"sync"

	"github.com/snple/beacon/internal/mq"
	"github.com/snple/beacon/packet"
)

// Queue 是客户端待发送消息的内存 FIFO 队列，按 PacketID 建索引以便 O(1) 删除。
//
// 消息全部保存在内存中：客户端进程退出后队列即消失（不再落盘），
// 因此 QoS1 的「重连后补发」只在进程存活期间有效。
type Queue struct {
	mu    sync.Mutex
	list  *list.List                        // 元素为 *Message
	index map[packet.PacketID]*list.Element // Packet.PacketID -> 元素
}

// NewQueue 创建一个新的基于 PacketID 的内存队列
func NewQueue() *Queue {
	return &Queue{
		list:  list.New(),
		index: make(map[packet.PacketID]*list.Element),
	}
}

// Count 返回队列中的消息条数。
func (q *Queue) Count() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return int64(q.list.Len())
}

// IsEmpty 报告队列是否为空。
func (q *Queue) IsEmpty() (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.list.Len() == 0, nil
}

// Size 返回队列中的消息条数。
func (q *Queue) Size() (uint64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return uint64(q.list.Len()), nil
}

// Enqueue 将消息添加到队尾；相同 PacketID 已在队列中时就地覆盖其内容
// （重试路径会用标记了 Dup 的副本覆盖原消息，位置保持不变）。
func (q *Queue) Enqueue(msg *Message) error {
	if msg == nil || msg.Packet == nil {
		return fmt.Errorf("invalid message")
	}

	id := msg.Packet.PacketID

	q.mu.Lock()
	defer q.mu.Unlock()

	if elem, exists := q.index[id]; exists {
		elem.Value = msg
		return nil
	}
	q.index[id] = q.list.PushBack(msg)
	return nil
}

// Dequeue 从队列头部取出消息并删除
func (q *Queue) Dequeue() (*Message, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	elem := q.list.Front()
	if elem == nil {
		return nil, mq.ErrQueueEmpty
	}

	msg := q.list.Remove(elem).(*Message)
	delete(q.index, msg.Packet.PacketID)
	return msg, nil
}

// Peek 查看队列头部消息但不删除
func (q *Queue) Peek() (*Message, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if elem := q.list.Front(); elem != nil {
		return elem.Value.(*Message), nil
	}
	return nil, mq.ErrQueueEmpty
}

// PeekID 返回队头消息的 PacketID（不读取消息体）
func (q *Queue) PeekID() (packet.PacketID, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if elem := q.list.Front(); elem != nil {
		return elem.Value.(*Message).Packet.PacketID, true, nil
	}
	return packet.PacketID{}, false, nil
}

// Get 根据 PacketID 获取消息（不删除）
func (q *Queue) Get(packetID packet.PacketID) (*Message, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if elem, ok := q.index[packetID]; ok {
		return elem.Value.(*Message), true
	}
	return nil, false
}

// Delete 根据 PacketID 删除消息（条目不存在时返回 nil）
func (q *Queue) Delete(packetID packet.PacketID) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if elem, ok := q.index[packetID]; ok {
		q.list.Remove(elem)
		delete(q.index, packetID)
	}
	return nil
}

// DeleteHead 删除队列头部消息
func (q *Queue) DeleteHead() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	elem := q.list.Front()
	if elem == nil {
		return mq.ErrQueueEmpty
	}

	msg := q.list.Remove(elem).(*Message)
	delete(q.index, msg.Packet.PacketID)
	return nil
}

// Clear 清空队列
func (q *Queue) Clear() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.list.Init()
	clear(q.index)
	return nil
}
