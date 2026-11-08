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
	rec := &packet.Record{
		Channel: channel,
		Payload: []byte{0},
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

	for i, want := range []*Message{high, med, low} {
		got, ok := mb.takeDelivery()
		if !ok || got.id != want.ID {
			t.Fatalf("take #%d: got %v ok=%v want %s", i, got.id, ok, want.ID.Hex())
		}
		mb.applySent(got)
		if got.prio.RequiresCommit() {
			mb.commit(got.id)
		}
	}
}

func TestMailboxLowDeletedOnSend(t *testing.T) {
	mb, released := newTestMailbox(100)
	m := testMsg(t, "a", packet.PriorityLow)
	mb.enqueue(m)

	got, ok := mb.takeDelivery()
	if !ok {
		t.Fatal("expected pending message")
	}
	mb.applySent(got)

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

	got, ok := mb.takeDelivery()
	if !ok {
		t.Fatal("expected pending message")
	}
	mb.applySent(got)

	if released.Load() != 0 {
		t.Fatal("reliable message must not be released before commit")
	}
	_, delivered, oldest := mb.depth()
	if delivered != 1 || oldest != m.ID {
		t.Fatalf("delivered=%d oldest=%s", delivered, oldest.Hex())
	}

	if popped := mb.commit(m.ID); popped != 1 {
		t.Fatalf("popped=%d", popped)
	}
	if released.Load() != 1 {
		t.Fatal("message should be released after commit")
	}
}

func TestMailboxCommitUnknownIsNoop(t *testing.T) {
	mb, released := newTestMailbox(100)
	m := testMsg(t, "a", packet.PriorityMedium)
	mb.enqueue(m)

	got, _ := mb.takeDelivery()
	mb.applySent(got)

	if popped := mb.commit(packet.NewPacketID()); popped != 0 {
		t.Fatalf("unknown commit popped=%d", popped)
	}
	if released.Load() != 0 {
		t.Fatal("nothing should be released")
	}
	if _, delivered, _ := mb.depth(); delivered != 1 {
		t.Fatalf("delivered=%d", delivered)
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

func TestMailboxHigherPriorityUsesFreeSlot(t *testing.T) {
	mb, _ := newTestMailbox(100)
	med := testMsg(t, "a", packet.PriorityMedium)
	mb.enqueue(med)

	got, ok := mb.takeDelivery()
	if !ok || got.id != med.ID {
		t.Fatalf("expected medium message, got %v ok=%v", got.id, ok)
	}
	mb.applySent(got)

	// 同级和更低优先级不能占用第二个投递位。
	mb.enqueue(testMsg(t, "a", packet.PriorityLow))
	mb.enqueue(testMsg(t, "a", packet.PriorityMedium))
	if got, ok := mb.takeDelivery(); ok {
		t.Fatalf("same/lower priority should wait for commit, got %s", got.id.Hex())
	}

	// 高优先级可以使用空闲投递位。
	high := testMsg(t, "a", packet.PriorityHigh)
	mb.enqueue(high)
	got, ok = mb.takeDelivery()
	if !ok || got.id != high.ID {
		t.Fatalf("high priority should bypass unacked medium, got %v ok=%v", got.id, ok)
	}
	mb.applySent(got)

	if _, delivered, _ := mb.depth(); delivered != 2 {
		t.Fatalf("delivered=%d, want 2", delivered)
	}
	if _, ok := mb.takeDelivery(); ok {
		t.Fatal("both delivery slots should be occupied")
	}

	// 确认 Medium 后 High 仍占一个位；同级 Medium 仍不能进入。
	if popped := mb.commit(med.ID); popped != 1 {
		t.Fatalf("popped=%d", popped)
	}
	if _, ok := mb.takeDelivery(); ok {
		t.Fatal("pending medium must still wait while high is unacked")
	}
}

func TestMailboxDeliveredCursor(t *testing.T) {
	mb, _ := newTestMailbox(100)
	a := testMsg(t, "a", packet.PriorityMedium)
	mb.enqueue(a)

	first, _ := mb.takeDelivery()
	mb.applySent(first)

	b := testMsg(t, "a", packet.PriorityHigh)
	mb.enqueue(b)
	second, _ := mb.takeDelivery()
	mb.applySent(second)

	// served=0：从头取，不应算 repeated
	got1, next1, repeated1, ok1 := mb.drainDelivered(0)
	if !ok1 || got1.id != a.ID || next1 != 1 || repeated1 {
		t.Fatalf("first drain: id=%s next=%d repeated=%v ok=%v", got1.id.Hex(), next1, repeated1, ok1)
	}
	mb.release(got1)

	// served=1：继续取第二条
	got2, next2, repeated2, ok2 := mb.drainDelivered(1)
	if !ok2 || got2.id != b.ID || next2 != 2 || repeated2 {
		t.Fatalf("second drain: id=%s next=%d repeated=%v ok=%v", got2.id.Hex(), next2, repeated2, ok2)
	}
	mb.release(got2)

	// served 已到末尾：从头重发，并标记 repeated
	got3, next3, repeated3, ok3 := mb.drainDelivered(2)
	if !ok3 || got3.id != a.ID || next3 != 1 || !repeated3 {
		t.Fatalf("third drain: id=%s next=%d repeated=%v ok=%v", got3.id.Hex(), next3, repeated3, ok3)
	}
	mb.release(got3)
}

func TestMailboxRestore(t *testing.T) {
	mb, _ := newTestMailbox(100)
	a := testMsg(t, "a", packet.PriorityHigh)
	b := testMsg(t, "a", packet.PriorityLow)
	mb.enqueue(a)
	mb.enqueue(b)

	first, ok := mb.takeDelivery()
	if !ok || first.id != a.ID {
		t.Fatalf("first take: id=%s ok=%v", first.id.Hex(), ok)
	}
	// 模拟写失败
	mb.restore(first)

	pending, delivered, _ := mb.depth()
	if pending[2] != 1 || pending[0] != 1 || delivered != 0 {
		t.Fatalf("restore failed: %v delivered=%d", pending, delivered)
	}

	again, ok := mb.takeDelivery()
	if !ok || again.id != a.ID {
		t.Fatalf("restore order violated: id=%s ok=%v", again.id.Hex(), ok)
	}
}
