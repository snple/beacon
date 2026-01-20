package core

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/danclive/nson-go"
)

// sendQueue 客户端发送队列（用于流量控制）
// 使用 channel + 计数器实现，提供容量监控能力
// 通过 PacketID 去重，防止同一消息被重复加入队列
type sendQueue struct {
	capacity int           // 队列容量（等于客户端的 ReceiveWindow）
	queue    chan *Message // 消息队列
	used     atomic.Int32  // 当前使用量

	inQueue map[nson.Id]bool // 已在队列中的消息 PacketID（QoS 1）
	mu      sync.Mutex       // 保护 inQueue map
}

// newSendQueue 创建发送队列
func newSendQueue(capacity int) *sendQueue {
	if capacity <= 0 {
		capacity = 100 // 默认值
	}

	return &sendQueue{
		capacity: capacity,
		queue:    make(chan *Message, capacity),
		inQueue:  make(map[nson.Id]bool),
	}
}

var (
	// ErrSendQueueFull 发送队列已满
	ErrSendQueueFull = fmt.Errorf("send queue is full")
	// ErrMessageAlreadyInQueue 消息已在队列中
	ErrMessageAlreadyInQueue = fmt.Errorf("message already in send queue")
)

// tryEnqueue 尝试将消息放入队列
// 返回 nil 表示成功，非 nil 表示队列已满或消息已在队列中
// 对于 QoS 1 消息，通过 PacketID 去重，防止同一消息被重复加入队列
func (q *sendQueue) tryEnqueue(msg *Message) error {
	// QoS 1 消息需要检查是否已在队列中
	if !msg.PacketID.IsZero() {
		q.mu.Lock()
		if q.inQueue[msg.PacketID] {
			// 消息已在队列中，拒绝重复添加
			q.mu.Unlock()
			return ErrMessageAlreadyInQueue
		}
		q.mu.Unlock()
	}

	select {
	case q.queue <- msg:
		q.used.Add(1)
		// QoS 1 消息标记为已在队列中
		if !msg.PacketID.IsZero() {
			q.mu.Lock()
			q.inQueue[msg.PacketID] = true
			q.mu.Unlock()
		}
		return nil
	default:
		// 队列已满
		return ErrSendQueueFull
	}
}

// TryDequeue 尝试从队列中取出消息（非阻塞）
func (q *sendQueue) tryDequeue() (*Message, bool) {
	select {
	case qm, ok := <-q.queue:
		if !ok {
			return nil, false
		}

		q.used.Add(-1)
		// QoS 1 消息从 inQueue 中移除
		if !qm.PacketID.IsZero() {
			q.mu.Lock()
			delete(q.inQueue, qm.PacketID)
			q.mu.Unlock()
		}
		return qm, true
	default:
		return nil, false
	}
}

// Available 返回队列可用空间
func (q *sendQueue) available() int {
	used := int(q.used.Load())
	available := max(q.capacity-used, 0)
	return available
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
	return float64(q.used.Load()) / float64(q.capacity)
}
