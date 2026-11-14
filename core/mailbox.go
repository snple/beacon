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

// taken 是从 pending/delivered 取出、正在写出或已写出但尚未落盘的条目。
type taken struct {
	entry
	prio     packet.Priority
	reliable bool
}

// prioQueue 是单个 (priority, reliability) 组合的 FIFO，使用切片 + head 索引。
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

func (q *prioQueue) peekFront() (entry, bool) {
	if q.head >= len(q.items) {
		return entry{}, false
	}
	return q.items[q.head], true
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

func relIndex(reliable bool) int {
	if reliable {
		return 1
	}
	return 0
}

// Mailbox 是一个客户端的投递队列：
//
//	pending[priority][reliability]  未投递消息（受条数/字节上限约束）
//	delivered                       已投递未确认的可靠消息（FIFO，受 ReceiveMaximum 约束）
//
// 不可靠消息不占 delivered 槽位（写出即释放）；可靠消息与优先级相互独立，
// 在窗口内按投递顺序等待 COMMIT 或 NACK。
type mailbox struct {
	mu sync.Mutex
	q  [3][2]prioQueue

	delivered []entry

	cap      int   // pending 条数上限（0 = 不限）
	capBytes int64 // pending 近似字节上限（0 = 不限）
	bytes    int64 // 当前 pending 的近似字节数

	// closed 表示会话已销毁：此后 enqueue/restore/applySent 都必须拒绝，
	// 避免与 destroySession 并发时把消息 retain 进一个无人消费的队列。
	closed bool

	// onRelease 在丢弃消息时回调（释放引用并计数）。
	onRelease func(*Message)
}

func newMailbox(capacity int, capacityBytes int64, onRelease func(*Message)) *mailbox {
	return &mailbox{cap: capacity, capBytes: capacityBytes, onRelease: onRelease}
}

// enqueue 把消息放入对应 (priority, reliability) 的队尾。
//
// 返回 (enqueued, evicted)：
//   - enqueued=false 表示新消息被丢弃（条数/字节上限已满，且新消息价值低于
//     最低非空队列，或单条消息就超过字节上限）；此时调用方仍持有消息的
//     初始引用，需自行释放。
//   - enqueued=true 且 evicted=true 表示本次入队挤掉了一条旧消息（已在此处释放）。
//
// 价值排序为 (reliable, priority)：不可靠低于可靠，同可靠性下低优先级更低。
func (m *mailbox) enqueue(msg *Message) (enqueued, evicted bool) {
	var release *Message
	size := msg.Size()
	p := int(msg.Priority)
	r := relIndex(msg.Reliable)

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return false, false
	}

	full := m.cap > 0 && m.totalLocked() >= m.cap
	byteFull := m.capBytes > 0 && m.bytes+size > m.capBytes
	if !full && !byteFull {
		msg.retain()
		m.q[p][r].pushBack(entry{msg: msg, id: msg.ID})
		m.bytes += size
		m.mu.Unlock()
		return true, false
	}

	// 仅字节上限超限时不为该客户端入队，也不驱逐其他消息。
	if byteFull && !full {
		m.mu.Unlock()
		return false, true
	}

	vp, vr := m.victimLocked()
	if vp < 0 {
		m.mu.Unlock()
		return false, true
	}
	if r < vr || (r == vr && p < vp) {
		m.mu.Unlock()
		return false, true
	}

	// 条数已满：挤出价值最低的旧消息；若挤出后仍超字节上限，
	// 则不驱逐，直接丢弃新消息。
	if byteFull {
		if old, ok := m.q[vp][vr].peekFront(); ok && m.bytes-old.msg.Size()+size > m.capBytes {
			m.mu.Unlock()
			return false, true
		}
	}

	if old, ok := m.q[vp][vr].popFront(); ok {
		release = old.msg
		m.bytes -= old.msg.Size()
	}
	msg.retain()
	m.q[p][r].pushBack(entry{msg: msg, id: msg.ID})
	m.bytes += size
	m.mu.Unlock()

	if release != nil {
		m.onRelease(release)
	}
	return true, true
}

// victimLocked 返回价值最低的非空队列：不可靠优先于可靠，同可靠性下低优先级优先。
func (m *mailbox) victimLocked() (int, int) {
	for r := 0; r < 2; r++ {
		for p := 0; p < 3; p++ {
			if m.q[p][r].len() > 0 {
				return p, r
			}
		}
	}
	return -1, -1
}

func (m *mailbox) totalLocked() int {
	n := 0
	for p := 0; p < 3; p++ {
		for r := 0; r < 2; r++ {
			n += m.q[p][r].len()
		}
	}
	return n
}

// highestPendingLocked 返回最高优先级的非空队列；同优先级优先可靠消息。
func (m *mailbox) highestPendingLocked() (int, int, bool) {
	for p := 2; p >= 0; p-- {
		for r := 1; r >= 0; r-- {
			if m.q[p][r].len() > 0 {
				return p, r, true
			}
		}
	}
	return 0, 0, false
}

// takeDelivery 取出一条可投递的新消息。
//
// freeSlots 是当前可用的可靠消息在途槽位；为 0 时不投递新消息（只允许重发
// delivered 中未确认的消息）。返回的 taken 所有权转移给调用方：成功写出后
// 必须调用 applySent，失败时调用 restore。
func (m *mailbox) takeDelivery(freeSlots int) (taken, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed || freeSlots <= 0 {
		return taken{}, false
	}
	p, r, ok := m.highestPendingLocked()
	if !ok {
		return taken{}, false
	}
	e, ok := m.q[p][r].popFront()
	if !ok {
		return taken{}, false
	}
	m.bytes -= e.msg.Size()
	return taken{entry: e, prio: packet.Priority(p), reliable: r == 1}, true
}

// applySent 在写出成功后应用条目：
//   - 不可靠：释放引用（不进入 delivered）
//   - 可靠：追加到 delivered，等待 COMMIT / NACK
func (m *mailbox) applySent(t taken) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		m.onRelease(t.msg)
		return
	}
	if t.reliable {
		m.delivered = append(m.delivered, t.entry)
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	m.onRelease(t.msg)
}

// restore 把未成功写出的条目放回 pending 头部。
func (m *mailbox) restore(t taken) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		m.onRelease(t.msg)
		return
	}
	m.q[t.prio][relIndex(t.reliable)].pushFront(t.entry)
	m.bytes += t.msg.Size()
	m.mu.Unlock()
}

// redeliver 返回最多 maxCount 条未确认的可靠消息（retain 后的临时引用）。
func (m *mailbox) redeliver(maxCount int) []taken {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed || maxCount <= 0 {
		return nil
	}
	n := len(m.delivered)
	if n > maxCount {
		n = maxCount
	}
	if n <= 0 {
		return nil
	}
	out := make([]taken, 0, n)
	for i := 0; i < n; i++ {
		e := m.delivered[i]
		e.msg.retain()
		out = append(out, taken{entry: e, prio: e.msg.Priority, reliable: true})
	}
	return out
}

// nackedEntry 是 NACK 移除的 delivered 条目（调用方负责调用钩子后释放）。
type nackedEntry struct {
	entry
	reason packet.ReasonCode
}

// commit 批量处理 COMMIT/NACK，返回确认数、NACK 条目与未知 ID 数。
//
// 不在 delivered 中的 ID 记为 unknown（幂等空操作）。committed 消息在此处释放；
// NACK 条目的引用交给调用方（用于回调钩子），之后由调用方释放。
func (m *mailbox) commit(entries []packet.CommitEntry) (committed int, nacked []nackedEntry, unknown int) {
	var release []*Message

	m.mu.Lock()
	for _, ce := range entries {
		idx := m.findDeliveredLocked(ce.ID)
		if idx < 0 {
			unknown++
			continue
		}
		e := m.delivered[idx]
		m.delivered = append(m.delivered[:idx], m.delivered[idx+1:]...)

		if ce.Action == packet.CommitActionNack {
			nacked = append(nacked, nackedEntry{entry: e, reason: ce.Reason})
			continue
		}
		committed++
		release = append(release, e.msg)
	}
	m.mu.Unlock()

	for _, msg := range release {
		m.onRelease(msg)
	}
	return committed, nacked, unknown
}

// discardDelivered 从 delivered 中移除指定 ID（用于接收上限变化后丢弃超限记录）。
func (m *mailbox) discardDelivered(id packet.PacketID) bool {
	var release *Message

	m.mu.Lock()
	idx := m.findDeliveredLocked(id)
	if idx >= 0 {
		release = m.delivered[idx].msg
		m.delivered = append(m.delivered[:idx], m.delivered[idx+1:]...)
	}
	m.mu.Unlock()

	if release != nil {
		m.onRelease(release)
		return true
	}
	return false
}

func (m *mailbox) findDeliveredLocked(id packet.PacketID) int {
	for i := range m.delivered {
		if m.delivered[i].id == id {
			return i
		}
	}
	return -1
}

// deliveredLen 返回已投递未确认的可靠消息数。
func (m *mailbox) deliveredLen() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.delivered)
}

// release 释放只为发送而临时保留引用的条目。
func (m *mailbox) release(t taken) {
	m.onRelease(t.msg)
}

// depth 返回当前积压：三个优先级的未投递条数、已投递未确认条数、最早的未确认 ID。
func (m *mailbox) depth() (pending [3]uint32, delivered uint32, oldest packet.PacketID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range 3 {
		pending[i] = uint32(m.q[i][0].len() + m.q[i][1].len())
	}
	delivered = uint32(len(m.delivered))
	if len(m.delivered) > 0 {
		oldest = m.delivered[0].id
	}
	return pending, delivered, oldest
}

// dropUnreliable 丢弃 pending 中全部不可靠消息，返回丢弃条数。
//
// 会话转入离线时调用：离线队列只保留可靠消息。
func (m *mailbox) dropUnreliable() int {
	var release []*Message
	count := 0

	m.mu.Lock()
	for p := 0; p < 3; p++ {
		for {
			e, ok := m.q[p][0].popFront()
			if !ok {
				break
			}
			m.bytes -= e.msg.Size()
			release = append(release, e.msg)
			count++
		}
	}
	m.mu.Unlock()

	for _, msg := range release {
		m.onRelease(msg)
	}
	return count
}

// dropExpired 丢弃所有已过期（ExpiresAt <= now）的 pending/delivered 消息，
// 返回丢弃条数。调用方负责统计。
func (m *mailbox) dropExpired(now int64) int {
	var release []*Message
	count := 0

	m.mu.Lock()
	for p := 0; p < 3; p++ {
		for r := 0; r < 2; r++ {
			q := &m.q[p][r]
			n := q.len()
			if n == 0 {
				continue
			}
			kept := make([]entry, 0, n)
			for i := q.head; i < len(q.items); i++ {
				e := q.items[i]
				q.items[i] = entry{}
				if e.msg.Expired(now) {
					m.bytes -= e.msg.Size()
					release = append(release, e.msg)
					count++
					continue
				}
				kept = append(kept, e)
			}
			q.items = kept
			q.head = 0
		}
	}
	if len(m.delivered) > 0 {
		kept := m.delivered[:0]
		for _, e := range m.delivered {
			if e.msg.Expired(now) {
				release = append(release, e.msg)
				count++
				continue
			}
			kept = append(kept, e)
		}
		clear(m.delivered[len(kept):])
		m.delivered = kept
	}
	m.mu.Unlock()

	for _, msg := range release {
		m.onRelease(msg)
	}
	return count
}

// drainLocked 清空全部队列并返回被移除的消息；调用方负责释放。
func (m *mailbox) drainLocked() []*Message {
	var release []*Message
	for p := 0; p < 3; p++ {
		for r := 0; r < 2; r++ {
			for {
				e, ok := m.q[p][r].popFront()
				if !ok {
					break
				}
				release = append(release, e.msg)
			}
		}
	}
	for _, e := range m.delivered {
		release = append(release, e.msg)
	}
	m.delivered = nil
	m.bytes = 0
	return release
}

// releaseAll 释放全部条目并关闭 mailbox（会话销毁时调用）。
func (m *mailbox) releaseAll() int {
	m.mu.Lock()
	m.closed = true
	release := m.drainLocked()
	m.mu.Unlock()

	for _, msg := range release {
		m.onRelease(msg)
	}
	return len(release)
}

// clearAll 释放全部条目但保持 mailbox 可用（CleanSession 重置时调用）。
func (m *mailbox) clearAll() int {
	m.mu.Lock()
	release := m.drainLocked()
	m.mu.Unlock()

	for _, msg := range release {
		m.onRelease(msg)
	}
	return len(release)
}
