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
//	pending[3]       未投递（Low / Medium / High），受 cap 约束
//	deliveredMedium  已投递未 COMMIT 的 Medium 消息
//	deliveredHigh    已投递未 COMMIT 的 High 消息
//
// Low 不占投递位。两组引用都是「所有权」语义：Message 的引用随消息
// 在队列间移动而转移。
type mailbox struct {
	mu sync.Mutex
	q  [3]prioQueue

	deliveredMedium *Message
	deliveredHigh   *Message

	cap int

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
	for p := range 3 {
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

// takeDelivery 尝试取出一条可投递的新消息。
//
// delivered 为空时按 High → Medium → Low 取一条；delivered 非空时，
// 只有优先级严格更高的消息可以占用空闲投递位，避免同级消息互相阻塞，
// 同时保证 High 始终能绕过未确认的 Medium。Low 不占用投递位。
//
// 取出的条目所有权转移给调用方：成功写出后必须调用 applySent，失败时调用 restore。
func (m *mailbox) takeDelivery() (taken, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.highestPendingLocked()
	if !ok || !m.canTakeLocked(p) {
		return taken{}, false
	}
	return m.popPendingLocked()
}

// canTakeLocked 报告优先级 p 的新消息是否可以投递：
// delivered 为空时都可以；否则只有严格更高优先级才能占用空闲投递位。
func (m *mailbox) canTakeLocked(p packet.Priority) bool {
	switch {
	case m.deliveredMedium == nil && m.deliveredHigh == nil:
		return true
	case m.deliveredMedium != nil && m.deliveredHigh == nil:
		return p > packet.PriorityMedium
	default:
		// deliveredHigh 已占用（无论 Medium 是否存在）时没有更高优先级可用。
		return false
	}
}

func (m *mailbox) highestPendingLocked() (packet.Priority, bool) {
	for p := 2; p >= 0; p-- {
		if m.q[p].len() > 0 {
			return packet.Priority(p), true
		}
	}
	return 0, false
}

func (m *mailbox) popPendingLocked() (taken, bool) {
	for p := 2; p >= 0; p-- {
		if e, ok := m.q[p].popFront(); ok {
			return taken{entry: e, prio: packet.Priority(p)}, true
		}
	}
	return taken{}, false
}

// applySent 在写出成功后应用条目：
//   - Low：释放引用（不进入 delivered）
//   - Medium/High：占用一个投递位
func (m *mailbox) applySent(t taken) {
	m.mu.Lock()
	switch t.prio {
	case packet.PriorityMedium:
		if m.deliveredMedium == nil {
			m.deliveredMedium = t.msg
			m.mu.Unlock()
			return
		}
	case packet.PriorityHigh:
		if m.deliveredHigh == nil {
			m.deliveredHigh = t.msg
			m.mu.Unlock()
			return
		}
	}

	if t.prio.RequiresCommit() {
		// 不应发生（takeDelivery 已检查投递位）。保守放回 pending，
		// 宁可重复投递也不能丢失可靠消息。
		m.q[t.prio].pushFront(t.entry)
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	m.onRelease(t.msg)
}

// restore 把未成功写出的条目放回 pending 头部。
func (m *mailbox) restore(t taken) {
	m.mu.Lock()
	m.q[t.prio].pushFront(t.entry)
	m.mu.Unlock()
}

// drainDelivered 返回 delivered 中从 served 开始的条目（不删除，引用由调用方保留）。
//
// nextServed 是下一次应该继续的位置；当 served 已经到达末尾时从头开始（重复投递），
// 此时 repeated 为 true，调用方应计入「无进展 pull」。
func (m *mailbox) drainDelivered(served int) (t taken, nextServed int, repeated, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	n := m.deliveredLenLocked()
	if n <= 0 {
		return taken{}, 0, false, false
	}

	start := served
	if start >= n || start < 0 {
		start = 0
		repeated = true
	}

	msg := m.deliveredMedium
	if start == 1 || msg == nil {
		msg = m.deliveredHigh
	}
	msg.retain()
	return taken{entry: entry{msg: msg, id: msg.ID}, prio: msg.Priority}, start + 1, repeated, true
}

func (m *mailbox) deliveredLenLocked() int {
	n := 0
	if m.deliveredMedium != nil {
		n++
	}
	if m.deliveredHigh != nil {
		n++
	}
	return n
}

// release 释放只为发送而临时保留引用的条目（delivered 用的 retain）。
func (m *mailbox) release(t taken) {
	m.onRelease(t.msg)
}

// commit 标记指定 ID 已确认，并移出对应投递位。
// 返回释放的条目数（0 或 1），调用方据此调整 served。
func (m *mailbox) commit(id packet.PacketID) (popped int) {
	m.mu.Lock()
	var release *Message

	switch {
	case m.deliveredMedium != nil && m.deliveredMedium.ID == id:
		release = m.deliveredMedium
		m.deliveredMedium = nil
	case m.deliveredHigh != nil && m.deliveredHigh.ID == id:
		release = m.deliveredHigh
		m.deliveredHigh = nil
	}
	m.mu.Unlock()

	if release != nil {
		m.onRelease(release)
		return 1
	}
	return 0
}

// depth 返回当前积压：三个优先级的未投递条数、已投递未确认条数、最早的未确认 ID。
func (m *mailbox) depth() (pending [3]uint32, delivered uint32, oldest packet.PacketID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range 3 {
		pending[i] = uint32(m.q[i].len())
	}
	if m.deliveredMedium != nil {
		delivered++
		oldest = m.deliveredMedium.ID
	}
	if m.deliveredHigh != nil {
		delivered++
		if oldest.IsZero() {
			oldest = m.deliveredHigh.ID
		}
	}
	return pending, delivered, oldest
}

// dropLow 丢弃 pending 中全部 Low 优先级消息，返回丢弃条数。
//
// 会话转入离线时调用：离线队列只保留 Medium/High。
func (m *mailbox) dropLow() int {
	m.mu.Lock()
	var release []*Message
	count := 0
	for {
		e, ok := m.q[packet.PriorityLow].popFront()
		if !ok {
			break
		}
		release = append(release, e.msg)
		count++
	}
	m.mu.Unlock()

	for _, msg := range release {
		m.onRelease(msg)
	}
	return count
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
	if m.deliveredMedium != nil {
		release = append(release, m.deliveredMedium)
		m.deliveredMedium = nil
	}
	if m.deliveredHigh != nil {
		release = append(release, m.deliveredHigh)
		m.deliveredHigh = nil
	}
	m.mu.Unlock()

	for _, msg := range release {
		m.onRelease(msg)
	}
}
