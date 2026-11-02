package core

import (
	"sync"

	"github.com/snple/beacon/packet"
)

// pendingQueue 是全局待分发队列。
//
// 队列里只保存消息 ID（每个 ID 对应 messageStore 中的一条共享消息），
// 因此队列本身与消息条数无关地保持轻量，也不复制任何消息内容。
type pendingQueue struct {
	mu   sync.Mutex
	ids  []packet.PacketID
	head int
}

const pendingQueueCompactThreshold = 1024

func newPendingQueue() *pendingQueue {
	return &pendingQueue{}
}

// enqueue 追加到队尾。
func (q *pendingQueue) enqueue(id packet.PacketID) {
	q.mu.Lock()
	q.ids = append(q.ids, id)
	q.mu.Unlock()
}

// dequeue 取出队头。
func (q *pendingQueue) dequeue() (packet.PacketID, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.head >= len(q.ids) {
		q.resetLocked()
		return packet.PacketID{}, false
	}

	id := q.ids[q.head]
	q.head++
	if q.head >= len(q.ids) {
		q.resetLocked()
	} else if q.head >= pendingQueueCompactThreshold && q.head >= len(q.ids)/2 {
		q.compactLocked()
	}

	return id, true
}

// len 返回待分发条数。
func (q *pendingQueue) len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.ids) - q.head
}

// resetLocked 在队列清空后复位；容量过大时释放底层数组，避免空闲时长期占用内存。
func (q *pendingQueue) resetLocked() {
	if cap(q.ids) > 4096 {
		q.ids = nil
	} else {
		q.ids = q.ids[:0]
	}
	q.head = 0
}

// compactLocked 移除已消费前缀，避免队列持续非空时底层 slice 不断增长。
func (q *pendingQueue) compactLocked() {
	remaining := copy(q.ids, q.ids[q.head:])
	q.ids = q.ids[:remaining]
	q.head = 0
}
