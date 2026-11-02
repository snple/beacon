package core

import (
	"sync/atomic"
	"testing"

	"github.com/snple/beacon/packet"
)

func newTestMailbox(cap int) (*mailbox, *atomic.Int64) {
	var released atomic.Int64
	mb := newMailbox(cap, func(*Message) { released.Add(1) })
	return mb, &released
}

func testMsg(t *testing.T, channel string, prio packet.Priority) *Message {
	t.Helper()
	return testMsgSize(t, channel, prio, 1)
}

func testMsgSize(t *testing.T, channel string, prio packet.Priority, payloadLen int) *Message {
	t.Helper()
	rec := &packet.Record{
		Channel: channel,
		Payload: make([]byte, payloadLen),
		ID:      packet.NewPacketID(),
	}
	rec.SetPriority(prio)
	m := newMessage(rec)
	m.refs.Store(1) // 测试中由调用方直接持有
	return m
}

func TestMailboxPriorityOrder(t *testing.T) {
	mb, _ := newTestMailbox(100)

	low := testMsg(t, "a", packet.PriorityLow)
	med := testMsg(t, "a", packet.PriorityMedium)
	high := testMsg(t, "a", packet.PriorityHigh)

	mb.enqueue(low)
	mb.enqueue(med)
	mb.enqueue(high)

	batch, more := mb.takePending(10, 0)
	if len(batch) != 3 || more {
		t.Fatalf("batch=%d more=%v", len(batch), more)
	}
	if batch[0].id != high.ID || batch[1].id != med.ID || batch[2].id != low.ID {
		t.Fatalf("priority order violated")
	}
}

func TestMailboxLowDeletedOnSend(t *testing.T) {
	mb, released := newTestMailbox(100)
	m := testMsg(t, "a", packet.PriorityLow)
	mb.enqueue(m)

	batch, _ := mb.takePending(10, 0)
	mb.applySent(batch)

	if released.Load() != 1 {
		t.Fatalf("low priority should be released after send, released=%d", released.Load())
	}
	pending, delivered, _ := mb.depth()
	if pending != [3]uint32{} || delivered != 0 {
		t.Fatalf("queue should be empty: pending=%v delivered=%d", pending, delivered)
	}
}

func TestMailboxReliableStaysUntilCommit(t *testing.T) {
	mb, released := newTestMailbox(100)
	m := testMsg(t, "a", packet.PriorityHigh)
	mb.enqueue(m)

	batch, _ := mb.takePending(10, 0)
	mb.applySent(batch)

	if released.Load() != 0 {
		t.Fatal("reliable message must not be released before commit")
	}
	_, delivered, oldest := mb.depth()
	if delivered != 1 || oldest != m.ID {
		t.Fatalf("delivered=%d oldest=%s", delivered, oldest.Hex())
	}

	committed, popped := mb.commit([]packet.PacketID{m.ID})
	if committed != 1 || popped != 1 {
		t.Fatalf("commit=%d popped=%d", committed, popped)
	}
	if released.Load() != 1 {
		t.Fatal("message should be released after commit")
	}
}

func TestMailboxIndependentCommit(t *testing.T) {
	mb, released := newTestMailbox(100)
	a := testMsg(t, "a", packet.PriorityMedium)
	b := testMsg(t, "a", packet.PriorityMedium)
	c := testMsg(t, "a", packet.PriorityMedium)
	mb.enqueue(a)
	mb.enqueue(b)
	mb.enqueue(c)

	batch, _ := mb.takePending(10, 0)
	mb.applySent(batch)

	// 乱序确认中间一条：不应被压缩掉
	committed, popped := mb.commit([]packet.PacketID{b.ID})
	if committed != 1 || popped != 0 {
		t.Fatalf("commit=%d popped=%d (head still uncommitted)", committed, popped)
	}
	if released.Load() != 0 {
		t.Fatal("nothing should be released yet")
	}

	// 补齐头部后应一次压缩三条（b 已确认）
	committed, popped = mb.commit([]packet.PacketID{a.ID, c.ID})
	if committed != 2 || popped != 3 {
		t.Fatalf("commit=%d popped=%d", committed, popped)
	}
	if released.Load() != 3 {
		t.Fatalf("released=%d", released.Load())
	}
}

func TestMailboxDropLowestPriority(t *testing.T) {
	mb, released := newTestMailbox(2)

	mb.enqueue(testMsg(t, "a", packet.PriorityLow))
	mb.enqueue(testMsg(t, "a", packet.PriorityMedium))

	enq, evicted := mb.enqueue(testMsg(t, "a", packet.PriorityHigh))
	if !enq || !evicted {
		t.Fatalf("expected enqueue with eviction, got enq=%v evicted=%v", enq, evicted)
	}
	if released.Load() != 1 {
		t.Fatalf("low priority message should be evicted, released=%d", released.Load())
	}

	pending, _, _ := mb.depth()
	if pending[0] != 0 || pending[1] != 1 || pending[2] != 1 {
		t.Fatalf("unexpected pending depths: %v", pending)
	}
}

func TestMailboxDropNewWhenLowerPriority(t *testing.T) {
	mb, released := newTestMailbox(1)
	mb.enqueue(testMsg(t, "a", packet.PriorityHigh))

	dropped := testMsg(t, "a", packet.PriorityLow)
	enq, evicted := mb.enqueue(dropped)
	if enq || !evicted {
		t.Fatalf("new low-priority message should be dropped, enq=%v", enq)
	}
	// 被丢弃的新消息由调用方释放（这里手工释放并验证 mailbox 未重复释放）
	if released.Load() != 0 {
		t.Fatal("mailbox must not release the dropped new message")
	}

	pending, _, _ := mb.depth()
	if pending[2] != 1 || pending[0] != 0 {
		t.Fatalf("high priority message should stay: %v", pending)
	}
}

func TestMailboxShareCap(t *testing.T) {
	mb, _ := newTestMailbox(1000)

	for i := 0; i < 100; i++ {
		mb.enqueue(testMsg(t, "a", packet.PriorityHigh))
	}
	for i := 0; i < 100; i++ {
		mb.enqueue(testMsg(t, "a", packet.PriorityLow))
	}

	batch, more := mb.takePending(10, 0)
	if !more {
		t.Fatal("expected more")
	}

	highCount := 0
	for _, tk := range batch {
		if tk.prio == packet.PriorityHigh {
			highCount++
		}
	}
	// High 存在下级非空队列时最多占 80%
	if highCount > 8 {
		t.Fatalf("high priority took %d of 10, expected <= 8", highCount)
	}
	if highCount < 7 {
		t.Fatalf("high priority should still dominate, got %d", highCount)
	}
}

func TestMailboxDeliveredCursor(t *testing.T) {
	mb, _ := newTestMailbox(100)
	a := testMsg(t, "a", packet.PriorityHigh)
	b := testMsg(t, "a", packet.PriorityMedium)
	mb.enqueue(a)
	mb.enqueue(b)

	batch, _ := mb.takePending(10, 0)
	mb.applySent(batch)

	// served=0：从头取，不应算 repeated
	got1, next1, more1, repeated1 := mb.drainDelivered(0, 1, 0)
	if len(got1) != 1 || next1 != 1 || !more1 || repeated1 {
		t.Fatalf("first drain: n=%d next=%d more=%v repeated=%v", len(got1), next1, more1, repeated1)
	}
	mb.releaseBatch(got1)

	// served=1：继续取第二条
	got2, next2, _, repeated2 := mb.drainDelivered(1, 1, 0)
	if len(got2) != 1 || next2 != 2 || repeated2 {
		t.Fatalf("second drain: n=%d next=%d repeated=%v", len(got2), next2, repeated2)
	}
	mb.releaseBatch(got2)

	// served 已到末尾：从头重发，并标记 repeated
	got3, _, _, repeated3 := mb.drainDelivered(2, 10, 0)
	if len(got3) != 2 || !repeated3 {
		t.Fatalf("third drain: n=%d repeated=%v", len(got3), repeated3)
	}
	mb.releaseBatch(got3)
}

func TestMailboxRestore(t *testing.T) {
	mb, _ := newTestMailbox(100)
	a := testMsg(t, "a", packet.PriorityHigh)
	b := testMsg(t, "a", packet.PriorityLow)
	mb.enqueue(a)
	mb.enqueue(b)

	batch, _ := mb.takePending(10, 0)
	if len(batch) != 2 {
		t.Fatalf("expected 2")
	}
	// 模拟写失败
	mb.restore(batch)

	pending, delivered, _ := mb.depth()
	if pending[2] != 1 || pending[0] != 1 || delivered != 0 {
		t.Fatalf("restore failed: %v delivered=%d", pending, delivered)
	}

	again, _ := mb.takePending(10, 0)
	if len(again) != 2 || again[0].id != a.ID || again[1].id != b.ID {
		t.Fatalf("restore order violated")
	}
}

func TestMailboxByteBudget(t *testing.T) {
	mb, _ := newTestMailbox(100)

	// 单条消息近似开销 160 字节 + payload
	big := testMsgSize(t, "a", packet.PriorityHigh, 2000)
	small := testMsg(t, "a", packet.PriorityHigh)
	mb.enqueue(big)
	mb.enqueue(small)

	// 预算只够一条 big：应只返回 big，且不再返回后面的 small
	batch, more := mb.takePending(10, 1000)
	if len(batch) != 1 {
		t.Fatalf("expected oversized single message, got %d", len(batch))
	}
	if !more {
		t.Fatal("expected more=true when queue still has items")
	}

	// 预算足够两条
	mb2, _ := newTestMailbox(100)
	mb2.enqueue(testMsg(t, "a", packet.PriorityHigh))
	mb2.enqueue(testMsg(t, "a", packet.PriorityHigh))
	b2, more2 := mb2.takePending(10, 10000)
	if len(b2) != 2 || more2 {
		t.Fatalf("n=%d more=%v", len(b2), more2)
	}
}
