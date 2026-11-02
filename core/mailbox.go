package core

import (
	"sync"

	"github.com/snple/beacon/packet"
)

// entry 是队列中的一个投递条目：只保存共享消息的指针与消息 ID。
type entry struct {
	msg *Message
	id  packet.PacketID
}

// taken 是从 pending 取出、正在写出或已写出但尚未落到 delivered 的条目。
type taken struct {
	entry
	prio packet.Priority
}

// deliveredEntry 是已写出、等待 COMMIT 的条目。
type deliveredEntry struct {
	entry
	committed bool
}

// prioQueue 是单个优先级的 FIFO，使用切片 + head 索引。
//
// 引用语义：队列中的每个条目持有一个 Message 引用，出队时所有权转移给调用方。
type prioQueue struct {
	items []entry
	head  int
}

const queueCompactThreshold = 1024

func (q *prioQueue) len() int { return len(q.items) - q.head }

func (q *prioQueue) pushBack(e entry) { q.items = append(q.items, e) }

func (q *prioQueue) pushFront(e entry) {
	if q.head > 0 {
		q.head--
		q.items[q.head] = e
		return
	}
	q.items = append(q.items, entry{})
	copy(q.items[1:], q.items[:len(q.items)-1])
	q.items[0] = e
}

func (q *prioQueue) popFront() (entry, bool) {
	if q.head >= len(q.items) {
		q.reset()
		return entry{}, false
	}
	e := q.items[q.head]
	q.items[q.head] = entry{}
	q.head++
	q.maybeCompact()
	return e, true
}

func (q *prioQueue) reset() {
	if cap(q.items) > 4096 {
		q.items = nil
	} else {
		q.items = q.items[:0]
	}
	q.head = 0
}

func (q *prioQueue) maybeCompact() {
	if q.head == len(q.items) {
		q.reset()
		return
	}
	if q.head >= queueCompactThreshold && q.head >= len(q.items)/2 {
		n := copy(q.items, q.items[q.head:])
		clear(q.items[n:])
		q.items = q.items[:n]
		q.head = 0
	}
}

// Mailbox 是一个客户端的投递队列：
//
//	pending[3]  未投递（Low / Medium / High），受 cap 约束
//	delivered   已投递未 COMMIT，只含 Medium / High，长度受 maxRecords 约束
//
// 两个队列的引用都是「所有权」语义：Message 的引用随条目在队列间移动而转移。
type mailbox struct {
	mu        sync.Mutex
	q         [3]prioQueue
	delivered []deliveredEntry
	dhead     int
	cap       int

	// onDropped 在丢弃消息时回调（释放引用并计数）。
	onRelease func(*Message)
}

func newMailbox(capacity int, onRelease func(*Message)) *mailbox {
	return &mailbox{cap: capacity, onRelease: onRelease}
}

// enqueue 把消息放入对应优先级的队尾。
//
// 返回 (enqueued, evicted)：
//   - enqueued=false 表示新消息被丢弃（队列已满且新消息优先级低于最低非空优先级）；
//     此时调用方仍持有消息的初始引用，需自行释放。
//   - evicted=true 表示本次入队挤掉了一条旧消息（已在此处释放）。
//
// 入队成功后队列持有一个额外引用。
func (m *mailbox) enqueue(msg *Message) (enqueued, evicted bool) {
	var release *Message

	m.mu.Lock()
	full := m.cap > 0 && m.totalLocked() >= m.cap
	if !full {
		msg.retain()
		m.q[msg.Priority].pushBack(entry{msg: msg, id: msg.ID})
		m.mu.Unlock()
		return true, false
	}

	victim := -1
	for p := 0; p < 3; p++ {
		if m.q[p].len() > 0 {
			victim = p
			break
		}
	}

	if victim < 0 || int(msg.Priority) < victim {
		m.mu.Unlock()
		// 不在这里释放：初始引用归调用方所有
		return false, true
	}

	if old, ok := m.q[victim].popFront(); ok {
		release = old.msg
	}
	msg.retain()
	m.q[msg.Priority].pushBack(entry{msg: msg, id: msg.ID})
	m.mu.Unlock()

	if release != nil {
		m.onRelease(release)
	}
	return true, true
}

func (m *mailbox) totalLocked() int {
	return m.q[0].len() + m.q[1].len() + m.q[2].len()
}

// takePending 按 High → Medium → Low 的顺序取出最多 maxRecords 条、maxBytes 字节。
//
// 取出的条目所有权转移给调用方：成功写出后必须调用 applySent，失败时调用 restore。
// more 表示被 maxRecords/maxBytes 截断且队列仍有剩余。
func (m *mailbox) takePending(maxRecords int, maxBytes int) (batch []taken, more bool) {
	if maxRecords <= 0 {
		maxRecords = 1
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	budget := maxRecords
	bytesLeft := maxBytes

	totalPending := m.totalLocked()
	if totalPending == 0 {
		return nil, false
	}

	// 份额：存在更低优先级非空队列时，本级最多占剩余预算的 80%
	for p := 2; p >= 0; p-- {
		if budget <= 0 {
			break
		}
		lower := false
		for lp := p - 1; lp >= 0; lp-- {
			if m.q[lp].len() > 0 {
				lower = true
				break
			}
		}

		limit := budget
		if lower {
			limit = budget * 8 / 10
			if limit < 1 {
				limit = 1
			}
		}

		for i := 0; i < limit && budget > 0; i++ {
			e, ok := m.q[p].popFront()
			if !ok {
				break
			}

			if maxBytes > 0 && bytesLeft > 0 {
				size := int(e.msg.Size())
				if size > bytesLeft {
					if len(batch) > 0 {
						// 放不下且已经有内容：放回去，结束
						m.q[p].pushFront(e)
						return batch, true
					}
					// 单条即超出预算：至少返回一条，然后停止
					batch = append(batch, taken{entry: e, prio: packet.Priority(p)})
					return batch, m.totalLocked() > 0
				}
				bytesLeft -= size
			}

			batch = append(batch, taken{entry: e, prio: packet.Priority(p)})
			budget--
		}
	}

	remaining := m.totalLocked()
	return batch, remaining > 0
}

// applySent 在写出成功后应用一批条目：
//   - Low：释放引用（不进入 delivered）
//   - Medium/High：按投递顺序追加到 delivered 尾部
func (m *mailbox) applySent(batch []taken) {
	if len(batch) == 0 {
		return
	}

	m.mu.Lock()
	var release []*Message
	for _, t := range batch {
		if t.prio.RequiresCommit() {
			m.delivered = append(m.delivered, deliveredEntry{entry: t.entry})
		} else {
			release = append(release, t.msg)
		}
	}
	m.mu.Unlock()

	for _, msg := range release {
		m.onRelease(msg)
	}
}

// restore 把未成功写出的条目按原顺序放回 pending 头部。
func (m *mailbox) restore(batch []taken) {
	if len(batch) == 0 {
		return
	}

	m.mu.Lock()
	for i := len(batch) - 1; i >= 0; i-- {
		t := batch[i]
		m.q[t.prio].pushFront(t.entry)
	}
	m.mu.Unlock()
}

// drainDelivered 返回 delivered 中从 served 开始的一批条目（不删除，引用由调用方保留）。
//
// nextServed 是下一次应该继续的位置；当 served 已经到达末尾时从头开始（重复投递），
// 此时 repeated 为 true，调用方应计入「无进展 pull」。
func (m *mailbox) drainDelivered(served, maxRecords, maxBytes int) (batch []taken, nextServed int, more, repeated bool) {
	if maxRecords <= 0 {
		maxRecords = 1
	}

	m.mu.Lock()
	n := len(m.delivered) - m.dhead
	if n <= 0 {
		m.mu.Unlock()
		return nil, 0, false, false
	}

	start := served
	if start >= n || start < 0 {
		start = 0
		repeated = true
	}

	bytesLeft := maxBytes
	cut := false
	for i := start; i < n && len(batch) < maxRecords; i++ {
		d := m.delivered[m.dhead+i]
		size := int(d.msg.Size())
		if maxBytes > 0 && bytesLeft > 0 {
			if size > bytesLeft {
				if len(batch) > 0 {
					cut = true
					break
				}
				// 单条即超出预算：至少返回一条，然后停止
				d.msg.retain()
				batch = append(batch, taken{entry: d.entry, prio: d.msg.Priority})
				cut = true
				break
			}
			bytesLeft -= size
		}
		d.msg.retain()
		batch = append(batch, taken{entry: d.entry, prio: d.msg.Priority})
	}
	nextServed = start + len(batch)
	more = cut || nextServed < n
	m.mu.Unlock()

	return batch, nextServed, more, repeated
}

// releaseBatch 释放一批只为发送而临时保留引用的条目（delivered 用的 retain）。
func (m *mailbox) releaseBatch(batch []taken) {
	for _, t := range batch {
		m.onRelease(t.msg)
	}
}

// commit 标记指定 ID 已确认，并从头部压缩连续已确认的前缀。
// 返回被压缩掉的条目数（调用方据此调整 served）。
func (m *mailbox) commit(ids []packet.PacketID) (committed, popped int) {
	if len(ids) == 0 {
		return 0, 0
	}

	set := make(map[packet.PacketID]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}

	m.mu.Lock()
	var release []*Message

	for i := m.dhead; i < len(m.delivered); i++ {
		e := &m.delivered[i]
		if e.committed {
			continue
		}
		if _, ok := set[e.id]; ok {
			e.committed = true
			committed++
		}
	}

	for m.dhead < len(m.delivered) && m.delivered[m.dhead].committed {
		release = append(release, m.delivered[m.dhead].msg)
		m.delivered[m.dhead] = deliveredEntry{}
		m.dhead++
		popped++
	}

	if m.dhead == len(m.delivered) {
		m.delivered = m.delivered[:0]
		m.dhead = 0
	} else if m.dhead >= queueCompactThreshold && m.dhead >= len(m.delivered)/2 {
		n := copy(m.delivered, m.delivered[m.dhead:])
		clear(m.delivered[n:])
		m.delivered = m.delivered[:n]
		m.dhead = 0
	}
	m.mu.Unlock()

	for _, msg := range release {
		m.onRelease(msg)
	}
	return committed, popped
}

// depth 返回当前积压：三个优先级的未投递条数、已投递未确认条数、最早的未确认 ID。
func (m *mailbox) depth() (pending [3]uint32, delivered uint32, oldest packet.PacketID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := 0; i < 3; i++ {
		pending[i] = uint32(m.q[i].len())
	}
	delivered = uint32(len(m.delivered) - m.dhead)
	if m.dhead < len(m.delivered) {
		oldest = m.delivered[m.dhead].id
	}
	return pending, delivered, oldest
}

// releaseAll 释放队列中全部条目（会话销毁时调用）。
func (m *mailbox) releaseAll() {
	m.mu.Lock()
	var release []*Message
	for p := 0; p < 3; p++ {
		for {
			e, ok := m.q[p].popFront()
			if !ok {
				break
			}
			release = append(release, e.msg)
		}
	}
	for i := m.dhead; i < len(m.delivered); i++ {
		release = append(release, m.delivered[i].msg)
	}
	m.delivered = nil
	m.dhead = 0
	m.mu.Unlock()

	for _, msg := range release {
		m.onRelease(msg)
	}
}
