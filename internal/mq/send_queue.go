package mq

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/danclive/nson-go"
)

// SendQueue 通用发送队列（用于流量控制）
// 使用 channel + 计数器实现，提供容量监控能力
// 通过 PacketID 去重，防止同一消息被重复加入队列。
//
// T 为消息类型（core.Message / client.Message），
// getID 用于提取消息的 PacketID 做去重：
//   - core：Message.PacketID
//   - client：Message.Packet.PacketID
type SendQueue[T any] struct {
	capacity int          // 队列容量（等于接收窗口）
	queue    chan T       // 消息队列
	used     atomic.Int32 // 当前使用量

	getID   func(T) nson.Id // 提取消息 PacketID（用于去重）
	inQueue map[nson.Id]bool
	mu      sync.Mutex // 保护 inQueue map
}

var (
	// ErrSendQueueFull 发送队列已满
	ErrSendQueueFull = fmt.Errorf("send queue is full")

	// ErrMessageAlreadyInQueue 消息已在队列中
	ErrMessageAlreadyInQueue = fmt.Errorf("message already in send queue")
)

// NewSendQueue 创建发送队列
// capacity <= 0 时使用默认容量 100
func NewSendQueue[T any](capacity int, getID func(T) nson.Id) *SendQueue[T] {
	if capacity <= 0 {
		capacity = 100 // 默认值
	}

	return &SendQueue[T]{
		capacity: capacity,
		queue:    make(chan T, capacity),
		inQueue:  make(map[nson.Id]bool),
		getID:    getID,
	}
}

// TryEnqueue 尝试将消息放入队列
// 返回 nil 表示成功，非 nil 表示队列已满或消息已在队列中
// 非零 PacketID 的消息通过 PacketID 去重，防止同一消息被重复加入队列
func (q *SendQueue[T]) TryEnqueue(msg T) error {
	id := q.getID(msg)

	q.mu.Lock()
	defer q.mu.Unlock()

	if !id.IsZero() {
		if q.inQueue[id] {
			return ErrMessageAlreadyInQueue
		}
	}

	select {
	case q.queue <- msg:
		q.used.Add(1)
		if !id.IsZero() {
			q.inQueue[id] = true
		}
		return nil
	default:
		return ErrSendQueueFull
	}
}

// TryDequeue 尝试从队列中取出消息（非阻塞）
// 调用方必须在消息处理完毕后调用 Finish() 清理 inQueue 去重映射，
// 否则该 PacketID 将永远无法再次入队
func (q *SendQueue[T]) TryDequeue() (T, bool) {
	select {
	case qm, ok := <-q.queue:
		if !ok {
			var zero T
			return zero, false
		}
		q.used.Add(-1)
		return qm, true
	default:
		var zero T
		return zero, false
	}
}

// Finish 标记消息已离开发送队列生命周期（已发送、发送失败或被丢弃）
func (q *SendQueue[T]) Finish(packetID nson.Id) {
	if packetID.IsZero() {
		return
	}

	q.mu.Lock()
	delete(q.inQueue, packetID)
	q.mu.Unlock()
}

// Available 返回队列可用空间
func (q *SendQueue[T]) Available() int {
	used := int(q.used.Load())
	return max(q.capacity-used, 0)
}

// Used 返回当前使用量
func (q *SendQueue[T]) Used() int32 {
	return q.used.Load()
}
