package client

import (
	"sync/atomic"

	"github.com/snple/beacon/packet"
)

// SendQueue 客户端发送队列（用于流量控制）
// 基于 core 的 ReceiveWindow 实现流控，限制客户端可以同时发送的未确认消息数量
type SendQueue struct {
	capacity int                 // 队列容量（等于 core 的 ReceiveWindow）
	queue    chan *QueuedMessage // 消息队列
	used     atomic.Int32        // 当前使用量
}

// QueuedMessage 队列中的消息
type QueuedMessage struct {
	Message     *Message   // 完整的消息数据
	QoS         packet.QoS // QoS 级别（用于快速判断）
	EnqueueTime int64      // 入队时间戳（Unix秒）
}

// NewSendQueue 创建发送队列
func NewSendQueue(capacity int) *SendQueue {
	if capacity <= 0 {
		capacity = defaultReceiveWindow
	}

	return &SendQueue{
		capacity: capacity,
		queue:    make(chan *QueuedMessage, capacity),
	}
}

// TryEnqueue 尝试将消息放入队列
// 返回 true 表示成功，false 表示队列已满
func (q *SendQueue) TryEnqueue(msg *QueuedMessage) bool {
	select {
	case q.queue <- msg:
		q.used.Add(1)
		return true
	default:
		// 队列已满
		return false
	}
}

// Dequeue 从队列中取出消息（阻塞）
func (q *SendQueue) Dequeue() (*QueuedMessage, bool) {
	qm, ok := <-q.queue
	if ok {
		q.used.Add(-1)
	}
	return qm, ok
}

// TryDequeue 尝试从队列中取出消息（非阻塞）
func (q *SendQueue) TryDequeue() (*QueuedMessage, bool) {
	select {
	case qm := <-q.queue:
		q.used.Add(-1)
		return qm, true
	default:
		return nil, false
	}
}

// Available 返回队列可用空间
func (q *SendQueue) Available() int {
	used := int(q.used.Load())
	available := q.capacity - used
	if available < 0 {
		available = 0
	}
	return available
}

// Used 返回队列当前使用量
func (q *SendQueue) Used() int {
	return int(q.used.Load())
}

// Capacity 返回队列容量
func (q *SendQueue) Capacity() int {
	return q.capacity
}

// IsFull 检查队列是否已满
func (q *SendQueue) IsFull() bool {
	return q.Available() == 0
}

// IsEmpty 检查队列是否为空
func (q *SendQueue) IsEmpty() bool {
	return q.Used() == 0
}

// Close 关闭队列
func (q *SendQueue) Close() {
	close(q.queue)
}

// UsageRatio 返回队列使用率 (0.0 - 1.0)
func (q *SendQueue) UsageRatio() float64 {
	if q.capacity == 0 {
		return 0
	}
	return float64(q.Used()) / float64(q.capacity)
}

// UpdateCapacity 更新队列容量（当收到 core 的新 ReceiveWindow 值时调用）
// 注意：这个操作不是线程安全的，应该在初始化或重连时调用
func (q *SendQueue) UpdateCapacity(newCapacity int) {
	if newCapacity <= 0 {
		newCapacity = defaultReceiveWindow
	}
	q.capacity = newCapacity
}

// Drain 清空队列中的所有消息
func (q *SendQueue) Drain() []*QueuedMessage {
	var messages []*QueuedMessage
	for {
		select {
		case msg := <-q.queue:
			q.used.Add(-1)
			messages = append(messages, msg)
		default:
			return messages
		}
	}
}

// Len 返回队列中消息数量（与 Used 相同，提供更语义化的接口）
func (q *SendQueue) Len() int {
	return q.Used()
}
