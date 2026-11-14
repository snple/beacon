package core

import (
	"sync/atomic"
	"testing"

	"github.com/snple/beacon/packet"
)

func newTestMailbox(cap int) (*mailbox, *atomic.Int64) {
	var released atomic.Int64
	mb := newMailbox(cap, 0, func(*Message) { released.Add(1) })
	return mb, &released
}

func testMsg(t *testing.T, channel string, prio packet.Priority) *Message {
	t.Helper()
	return testMsgReliable(t, channel, prio, prio != packet.PriorityLow)
}

func testMsgReliable(t *testing.T, channel string, prio packet.Priority, reliable bool) *Message {
	t.Helper()
	rec := &packet.Record{
		Channel: channel,
		Payload: []byte{0},
		ID:      packet.NewPacketID(),
	}
	rec.SetPriority(prio)
	rec.SetReliable(reliable)
	m := newMessage(rec)
	m.refs.Store(1) // 测试中由调用方直接持有
	return m
}

func commitOne(mb *mailbox, id packet.PacketID) (int, []nackedEntry, int) {
	return mb.commit([]packet.CommitEntry{{ID: id, Action: packet.CommitActionCommit}})
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
		got, ok := mb.takeDelivery(10)
		if !ok || got.id != want.ID {
			t.Fatalf("take #%d: got %v ok=%v want %s", i, got.id, ok, want.ID.Hex())
		}
		mb.applySent(got)
		if got.reliable {
			if n, _, _ := commitOne(mb, got.id); n != 1 {
				t.Fatalf("commit #%d = %d", i, n)
			}
		}
	}
}

func TestMailboxUnreliableDeletedOnSend(t *testing.T) {
	mb, released := newTestMailbox(100)
	m := testMsgReliable(t, "a", packet.PriorityHigh, false)
	mb.enqueue(m)

	got, ok := mb.takeDelivery(1)
	if !ok {
		t.Fatal("expected pending message")
	}
	mb.applySent(got)

	if released.Load() != 1 {
		t.Fatalf("unreliable message should be released after send, released=%d", released.Load())
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

	got, ok := mb.takeDelivery(1)
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

	if n, _, _ := commitOne(mb, m.ID); n != 1 {
		t.Fatalf("committed=%d", n)
	}
	if released.Load() != 1 {
		t.Fatal("message should be released after commit")
	}
}

func TestMailboxReliabilityIndependentOfPriority(t *testing.T) {
	mb, released := newTestMailbox(100)

	lowReliable := testMsgReliable(t, "a", packet.PriorityLow, true)
	highUnreliable := testMsgReliable(t, "a", packet.PriorityHigh, false)
	mb.enqueue(lowReliable)
	mb.enqueue(highUnreliable)

	first, ok := mb.takeDelivery(1)
	if !ok || first.id != highUnreliable.ID || first.reliable {
		t.Fatalf("high unreliable should go first: %+v", first)
	}
	mb.applySent(first)

	second, ok := mb.takeDelivery(1)
	if !ok || second.id != lowReliable.ID || !second.reliable {
		t.Fatalf("low reliable should follow: %+v", second)
	}
	mb.applySent(second)

	if released.Load() != 1 {
		t.Fatalf("only unreliable should be released, got %d", released.Load())
	}
	if _, delivered, _ := mb.depth(); delivered != 1 {
		t.Fatalf("delivered=%d, want 1", delivered)
	}
}

func TestMailboxCommitUnknownIsNoop(t *testing.T) {
	mb, released := newTestMailbox(100)
	m := testMsg(t, "a", packet.PriorityMedium)
	mb.enqueue(m)

	got, _ := mb.takeDelivery(1)
	mb.applySent(got)

	if n, _, unknown := commitOne(mb, packet.NewPacketID()); n != 0 || unknown != 1 {
		t.Fatalf("unknown commit: committed=%d unknown=%d", n, unknown)
	}
	if released.Load() != 0 {
		t.Fatal("nothing should be released")
	}
	if _, delivered, _ := mb.depth(); delivered != 1 {
		t.Fatalf("delivered=%d", delivered)
	}
}

func TestMailboxNackRemovesDelivered(t *testing.T) {
	mb, _ := newTestMailbox(100)
	m := testMsg(t, "a", packet.PriorityMedium)
	mb.enqueue(m)
	got, _ := mb.takeDelivery(1)
	mb.applySent(got)

	committed, nacked, unknown := mb.commit([]packet.CommitEntry{{
		ID:     m.ID,
		Action: packet.CommitActionNack,
		Reason: packet.ReasonUnspecifiedError,
	}})
	if committed != 0 || len(nacked) != 1 || unknown != 0 || nacked[0].id != m.ID {
		t.Fatalf("nack mismatch: committed=%d nacked=%v unknown=%d", committed, nacked, unknown)
	}
	if _, delivered, _ := mb.depth(); delivered != 0 {
		t.Fatalf("delivered=%d, want 0", delivered)
	}
	mb.onRelease(nacked[0].msg)
}

func TestMailboxDropLowestValue(t *testing.T) {
	mb, released := newTestMailbox(2)

	mb.enqueue(testMsg(t, "a", packet.PriorityLow))
	mb.enqueue(testMsg(t, "a", packet.PriorityMedium))

	enq, evicted := mb.enqueue(testMsg(t, "a", packet.PriorityHigh))
	if !enq || !evicted {
		t.Fatalf("expected enqueue with eviction, got enq=%v evicted=%v", enq, evicted)
	}
	if released.Load() != 1 {
		t.Fatalf("unreliable low-priority message should be evicted, released=%d", released.Load())
	}

	pending, _, _ := mb.depth()
	if pending[0] != 0 || pending[1] != 1 || pending[2] != 1 {
		t.Fatalf("unexpected pending depths: %v", pending)
	}
}

func TestMailboxReliabilityBeatsPriorityOnEviction(t *testing.T) {
	mb, released := newTestMailbox(1)
	// 队列里是不可靠的 High；新来的是可靠的 Low：应淘汰不可靠消息。
	mb.enqueue(testMsgReliable(t, "a", packet.PriorityHigh, false))

	reliableLow := testMsgReliable(t, "a", packet.PriorityLow, true)
	enq, evicted := mb.enqueue(reliableLow)
	if !enq || !evicted {
		t.Fatalf("reliable low should evict unreliable high: enq=%v evicted=%v", enq, evicted)
	}
	if released.Load() != 1 {
		t.Fatalf("released=%d, want 1", released.Load())
	}
	pending, _, _ := mb.depth()
	if pending[0] != 1 {
		t.Fatalf("pending=%v, want reliable low queued", pending)
	}
}

func TestMailboxDropNewWhenLowerValue(t *testing.T) {
	mb, released := newTestMailbox(1)
	mb.enqueue(testMsg(t, "a", packet.PriorityHigh))

	dropped := testMsg(t, "a", packet.PriorityLow)
	enq, evicted := mb.enqueue(dropped)
	if enq || !evicted {
		t.Fatalf("new low-value message should be dropped, enq=%v", enq)
	}
	if released.Load() != 0 {
		t.Fatal("mailbox must not release the dropped new message")
	}

	pending, _, _ := mb.depth()
	if pending[2] != 1 || pending[0] != 0 {
		t.Fatalf("high priority message should stay: %v", pending)
	}
}

func TestMailboxWindowControlsNewDelivery(t *testing.T) {
	mb, _ := newTestMailbox(100)
	for i := 0; i < 3; i++ {
		mb.enqueue(testMsg(t, "a", packet.PriorityMedium))
	}

	first, ok := mb.takeDelivery(2)
	if !ok {
		t.Fatal("first delivery")
	}
	mb.applySent(first)
	second, ok := mb.takeDelivery(1)
	if !ok {
		t.Fatal("second delivery")
	}
	mb.applySent(second)

	if _, ok := mb.takeDelivery(0); ok {
		t.Fatal("window full: no new delivery allowed")
	}
	if _, delivered, _ := mb.depth(); delivered != 2 {
		t.Fatalf("delivered=%d, want 2", delivered)
	}

	if n, _, _ := commitOne(mb, first.id); n != 1 {
		t.Fatalf("commit=%d", n)
	}
	third, ok := mb.takeDelivery(1)
	if !ok {
		t.Fatal("window free slot should allow next delivery")
	}
	mb.applySent(third)
}

func TestMailboxRedeliver(t *testing.T) {
	mb, _ := newTestMailbox(100)
	var ids []packet.PacketID
	for i := 0; i < 2; i++ {
		m := testMsg(t, "a", packet.PriorityMedium)
		ids = append(ids, m.ID)
		mb.enqueue(m)
		got, _ := mb.takeDelivery(1)
		mb.applySent(got)
	}

	got := mb.redeliver(1)
	if len(got) != 1 || got[0].id != ids[0] {
		t.Fatalf("redeliver first: %+v", got)
	}
	mb.release(got[0])

	all := mb.redeliver(10)
	if len(all) != 2 || all[0].id != ids[0] || all[1].id != ids[1] {
		t.Fatalf("redeliver all: %+v", all)
	}
	for _, t0 := range all {
		mb.release(t0)
	}
}

func TestMailboxRestore(t *testing.T) {
	mb, _ := newTestMailbox(100)
	a := testMsg(t, "a", packet.PriorityHigh)
	b := testMsg(t, "a", packet.PriorityLow)
	mb.enqueue(a)
	mb.enqueue(b)

	first, ok := mb.takeDelivery(1)
	if !ok || first.id != a.ID {
		t.Fatalf("first take: id=%s ok=%v", first.id.Hex(), ok)
	}
	mb.restore(first)

	pending, delivered, _ := mb.depth()
	if pending[2] != 1 || pending[0] != 1 || delivered != 0 {
		t.Fatalf("restore failed: %v delivered=%d", pending, delivered)
	}

	again, ok := mb.takeDelivery(1)
	if !ok || again.id != a.ID {
		t.Fatalf("restore order violated: id=%s ok=%v", again.id.Hex(), ok)
	}
}

func TestMailboxByteCapDropsNewMessage(t *testing.T) {
	var released atomic.Int64
	mb := newMailbox(100, 300, func(*Message) { released.Add(1) })

	a := testMsg(t, "a", packet.PriorityMedium)
	if enqueued, evicted := mb.enqueue(a); !enqueued || evicted {
		t.Fatalf("first message should fit: enqueued=%v evicted=%v", enqueued, evicted)
	}

	b := testMsg(t, "a", packet.PriorityMedium)
	if enqueued, _ := mb.enqueue(b); enqueued {
		t.Fatal("second message should exceed the byte cap")
	}

	got, ok := mb.takeDelivery(1)
	if !ok || got.id != a.ID {
		t.Fatalf("original message should remain queued: id=%s ok=%v", got.id.Hex(), ok)
	}
	if released.Load() != 0 {
		t.Fatalf("no message should have been released, got %d", released.Load())
	}
}

func TestMailboxClosedRejectsEnqueue(t *testing.T) {
	mb, released := newTestMailbox(10)

	a := testMsg(t, "a", packet.PriorityMedium)
	mb.enqueue(a)
	if n := mb.releaseAll(); n != 1 {
		t.Fatalf("releaseAll = %d, want 1", n)
	}
	if released.Load() != 1 {
		t.Fatalf("released = %d, want 1", released.Load())
	}

	b := testMsg(t, "a", packet.PriorityMedium)
	if enqueued, evicted := mb.enqueue(b); enqueued || evicted {
		t.Fatalf("closed mailbox must reject enqueue: enqueued=%v evicted=%v", enqueued, evicted)
	}
	if !b.release() {
		t.Fatal("test message should be fully released")
	}
}
