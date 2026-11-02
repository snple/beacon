package core

import (
	"container/list"
	"sync"
	"sync/atomic"

	"github.com/snple/beacon/packet"
)

// delivery 是一次「向某个客户端投递某条消息」的引用。
//
// 它只保存共享消息的指针（消息内容由 messageStore 按 ID 唯一保存）与逐客户端的
// 投递参数，因此向 N 个订阅者投递同一条消息只产生 N 个小结构体，不复制载荷。
// delivery 本身持有一个 messageStore 引用：在队列之间移动就是引用所有权的转移。
type delivery struct {
	msg       *Message        // 共享消息（持有一个 messageStore 引用）
	packetID  packet.PacketID // 出站 PUBLISH 的 PacketID：逐投递唯一，用于 ACK 匹配
	qos       packet.QoS      // 对该客户端的有效 QoS（min(订阅 QoS, 发布 QoS)）
	retain    bool            // 出站是否置 Retain 标志
	timestamp int64           // 对应消息的接收时间（Unix 秒），仅用于观测

	// 以下字段在入队后由发送协程与重传协程并发访问，使用原子操作。
	dup        atomic.Bool
	lastSentAt atomic.Int64
	retries    atomic.Int32
}

func newDelivery(m *Message, qos packet.QoS, retain bool) *delivery {
	return &delivery{
		msg:       m,
		packetID:  packet.NewPacketID(),
		qos:       qos,
		retain:    retain,
		timestamp: m.Timestamp,
	}
}

// deliveryQueue 是以出站 PacketID 为索引的 FIFO 队列，出队与按 ID 删除都是 O(1)。
// 队列元素只是 *delivery 指针，队列本身不持有任何消息内容。
type deliveryQueue struct {
	mu     sync.Mutex
	list   *list.List
	index  map[packet.PacketID]*list.Element
	store  *messageStore
	closed bool
}

func newDeliveryQueue(store *messageStore) *deliveryQueue {
	return &deliveryQueue{
		list:  list.New(),
		index: make(map[packet.PacketID]*list.Element),
		store: store,
	}
}

// pushBack 追加到队尾（引用的所有权转移给队列）。
// 队列已关闭时直接释放引用，避免会话被清理后消息被永久持有。
func (q *deliveryQueue) pushBack(d *delivery) {
	q.push(d, false)
}

// pushFront 插入到队头（引用的所有权转移给队列）。
func (q *deliveryQueue) pushFront(d *delivery) {
	q.push(d, true)
}

func (q *deliveryQueue) push(d *delivery, front bool) {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		q.store.release(d.msg)
		return
	}

	if front {
		q.index[d.packetID] = q.list.PushFront(d)
	} else {
		q.index[d.packetID] = q.list.PushBack(d)
	}
	q.mu.Unlock()
}

// popFront 取出队头，引用的所有权转移给调用方。
func (q *deliveryQueue) popFront() (*delivery, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	elem := q.list.Front()
	if elem == nil {
		return nil, false
	}

	d := q.list.Remove(elem).(*delivery)
	delete(q.index, d.packetID)
	return d, true
}

// peek 返回队头（不改变引用计数）。
func (q *deliveryQueue) peek() *delivery {
	q.mu.Lock()
	defer q.mu.Unlock()

	if elem := q.list.Front(); elem != nil {
		return elem.Value.(*delivery)
	}
	return nil
}

// get 按 PacketID 查找（不改变引用计数）。
func (q *deliveryQueue) get(packetID packet.PacketID) *delivery {
	q.mu.Lock()
	defer q.mu.Unlock()

	if elem, ok := q.index[packetID]; ok {
		return elem.Value.(*delivery)
	}
	return nil
}

// remove 按 PacketID 移除，引用的所有权转移给调用方。
func (q *deliveryQueue) remove(packetID packet.PacketID) (*delivery, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	elem, ok := q.index[packetID]
	if !ok {
		return nil, false
	}

	d := q.list.Remove(elem).(*delivery)
	delete(q.index, packetID)
	return d, true
}

// drop 按 PacketID 移除并释放其持有的消息引用。
func (q *deliveryQueue) drop(packetID packet.PacketID) bool {
	d, ok := q.remove(packetID)
	if ok {
		q.store.release(d.msg)
	}
	return ok
}

// dropOldest 移除队头并释放引用（客户端队列溢出策略 drop-oldest）。
func (q *deliveryQueue) dropOldest() bool {
	d, ok := q.popFront()
	if ok {
		q.store.release(d.msg)
	}
	return ok
}

// len 返回队列长度。
func (q *deliveryQueue) len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.list.Len()
}

// close 关闭队列：清空现有元素并标记为已废弃。
// 关闭后入队的投递会立即释放其消息引用（发送协程可能在清理后才把失败投递放回队头）。
func (q *deliveryQueue) close() {
	q.mu.Lock()
	msgs := make([]*Message, 0, q.list.Len())
	for elem := q.list.Front(); elem != nil; elem = elem.Next() {
		msgs = append(msgs, elem.Value.(*delivery).msg)
	}
	q.list.Init()
	clear(q.index)
	q.closed = true
	q.mu.Unlock()

	for _, m := range msgs {
		q.store.release(m)
	}
}

// snapshot 返回当前顺序的快照（持有队列锁期间复制指针，不影响引用计数）。
func (q *deliveryQueue) snapshot() []*delivery {
	q.mu.Lock()
	defer q.mu.Unlock()

	out := make([]*delivery, 0, q.list.Len())
	for elem := q.list.Front(); elem != nil; elem = elem.Next() {
		out = append(out, elem.Value.(*delivery))
	}
	return out
}

// expired 返回超过重传间隔仍未发送（或发送后未确认）的条目快照。
func (q *deliveryQueue) expired(now, interval int64) []*delivery {
	q.mu.Lock()
	defer q.mu.Unlock()

	var out []*delivery
	for elem := q.list.Front(); elem != nil; elem = elem.Next() {
		d := elem.Value.(*delivery)
		last := d.lastSentAt.Load()
		if last > 0 && now-last >= interval {
			out = append(out, d)
		}
	}
	return out
}

// dropExpired 删除所有已过期的条目并释放引用，返回删除条数。
func (q *deliveryQueue) dropExpired() int {
	q.mu.Lock()
	var dropped []*Message
	for elem := q.list.Front(); elem != nil; {
		next := elem.Next()
		d := elem.Value.(*delivery)
		if d.msg.IsExpired() {
			q.list.Remove(elem)
			delete(q.index, d.packetID)
			dropped = append(dropped, d.msg)
		}
		elem = next
	}
	q.mu.Unlock()

	for _, m := range dropped {
		q.store.release(m)
	}
	return len(dropped)
}
