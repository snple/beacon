package core

import (
	"sync/atomic"
)

// sendQueue 客户端发送队列（用于流量控制）
// 使用 channel + 计数器实现，提供容量监控能力
type sendQueue struct {
	capacity int           // 队列容量（等于客户端的 ReceiveWindow）
	queue    chan *Message // 消息队列
	used     atomic.Int32  // 当前使用量
}

// newSendQueue 创建发送队列
func newSendQueue(capacity int) *sendQueue {
	if capacity <= 0 {
		capacity = 100 // 默认值
	}

	return &sendQueue{
		capacity: capacity,
		queue:    make(chan *Message, capacity),
	}
}

// tryEnqueue 尝试将消息放入队列
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

// Dequeue 从队列中取出消息（阻塞）
func (q *sendQueue) dequeue() (*Message, bool) {
	qm, ok := <-q.queue
	if ok {
		q.used.Add(-1)
	}
	return qm, ok
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
func (q *sendQueue) getAvailable() int {
	used := int(q.used.Load())
	available := max(q.capacity-used, 0)
	return available
}

// Used 返回队列当前使用量
func (q *sendQueue) getUsed() int {
	return int(q.used.Load())
}

// Capacity 返回队列容量
func (q *sendQueue) getCapacity() int {
	return q.capacity
}

// IsFull 检查队列是否已满
func (q *sendQueue) isFull() bool {
	return q.getAvailable() == 0
}

// IsEmpty 检查队列是否为空
func (q *sendQueue) isEmpty() bool {
	return q.getUsed() == 0
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
	return float64(q.getUsed()) / float64(q.getCapacity())
}
