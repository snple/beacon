package client

import (
	"sync/atomic"
)

// sendQueue 客户端发送队列（用于流量控制）
// 基于 core 的 ReceiveWindow 实现流控，限制客户端可以同时发送的未确认消息数量
type sendQueue struct {
	capacity int           // 队列容量（等于 core 的 ReceiveWindow）
	queue    chan *Message // 消息队列
	used     atomic.Int32  // 当前使用量
}

// newSendQueue 创建发送队列
func newSendQueue(capacity int) *sendQueue {
	if capacity <= 0 {
		capacity = defaultReceiveWindow
	}

	return &sendQueue{
		capacity: capacity,
		queue:    make(chan *Message, capacity),
	}
}

// TryEnqueue 尝试将消息放入队列
// 返回 true 表示成功，false 表示队列已满
func (q *sendQueue) tryEnqueue(msg *Message) bool {
	select {
	case q.queue <- msg:
		q.used.Add(1)
		return true
	default:
		// 队列已满
		return false
	}
}

// TryDequeue 尝试从队列中取出消息（非阻塞）
func (q *sendQueue) tryDequeue() (*Message, bool) {
	select {
	case qm := <-q.queue:
		q.used.Add(-1)
		return qm, true
	default:
		return nil, false
	}
}

// Available 返回队列可用空间
func (q *sendQueue) available() int {
	used := int(q.used.Load())
	available := q.capacity - used
	if available < 0 {
		available = 0
	}
	return available
}

// Used 返回队列当前使用量
func (q *sendQueue) Used() int {
	return int(q.used.Load())
}

// Close 关闭队列
func (q *sendQueue) close() {
	close(q.queue)
}

// UsageRatio 返回队列使用率 (0.0 - 1.0)
func (q *sendQueue) usageRatio() float64 {
	if q.capacity == 0 {
		return 0
	}
	return float64(q.Used()) / float64(q.capacity)
}

// UpdateCapacity 更新队列容量（当收到 core 的新 ReceiveWindow 值时调用）
// 注意：这个操作不是线程安全的，应该在初始化或重连时调用
func (q *sendQueue) updateCapacity(newCapacity int) {
	if newCapacity <= 0 {
		newCapacity = defaultReceiveWindow
	}
	q.capacity = newCapacity
}
