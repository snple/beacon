package core

import (
	"sync/atomic"
	"time"

	"github.com/snple/beacon/packet"
)

// SendQueue 客户端发送队列（用于流量控制）
// 使用 channel + 计数器实现，提供容量监控能力
type SendQueue struct {
	capacity int                 // 队列容量（等于客户端的 ReceiveWindow）
	queue    chan *QueuedMessage // 消息队列
	used     atomic.Int32        // 当前使用量
}

// QueuedMessage 队列中的消息
type QueuedMessage struct {
	Message     *Message
	QoS         packet.QoS
	Dup         bool  // 是否为重传消息
	EnqueueTime int64 // 入队时间戳（Unix秒）
}

// NewSendQueue 创建发送队列
func NewSendQueue(capacity int) *SendQueue {
	if capacity <= 0 {
		capacity = 100 // 默认值
	}

	return &SendQueue{
		capacity: capacity,
		queue:    make(chan *QueuedMessage, capacity),
	}
}

// TryEnqueue 尝试将消息放入队列
// 返回 true 表示成功，false 表示队列已满
func (q *SendQueue) TryEnqueue(msg *Message, qos packet.QoS, dup bool) bool {
	qm := &QueuedMessage{
		Message:     msg,
		QoS:         qos,
		Dup:         dup,
		EnqueueTime: time.Now().Unix(),
	}

	select {
	case q.queue <- qm:
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
	available := max(q.capacity-used, 0)
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
