package mq

import (
	"errors"
	"testing"

	"github.com/danclive/nson-go"
)

type testMsg struct {
	id nson.Id
}

func testMsgID(m testMsg) nson.Id {
	return m.id
}

// TestSendQueue_EnqueueDequeueRoundtrip 验证入队/出队基本往返与计数
func TestSendQueue_EnqueueDequeueRoundtrip(t *testing.T) {
	q := NewSendQueue(4, testMsgID)

	m1 := testMsg{id: nson.NewId()}
	m2 := testMsg{id: nson.NewId()}

	if err := q.TryEnqueue(m1); err != nil {
		t.Fatalf("enqueue m1 failed: %v", err)
	}
	if err := q.TryEnqueue(m2); err != nil {
		t.Fatalf("enqueue m2 failed: %v", err)
	}

	if got := q.Used(); got != 2 {
		t.Fatalf("expected used=2, got %d", got)
	}
	if got := q.Available(); got != 2 {
		t.Fatalf("expected available=2, got %d", got)
	}

	got1, ok := q.TryDequeue()
	if !ok || got1 != m1 {
		t.Fatalf("expected m1 dequeued, ok=%v", ok)
	}
	got2, ok := q.TryDequeue()
	if !ok || got2 != m2 {
		t.Fatalf("expected m2 dequeued, ok=%v", ok)
	}

	// 空队列出队失败
	if _, ok := q.TryDequeue(); ok {
		t.Fatal("expected dequeue from empty queue to fail")
	}

	// finish 清理去重映射后，同一 PacketID 可再次入队
	q.Finish(m1.id)
	if err := q.TryEnqueue(m1); err != nil {
		t.Fatalf("re-enqueue after finish failed: %v", err)
	}
}

// TestSendQueue_DeduplicateByPacketID 验证 PacketID 去重
func TestSendQueue_DeduplicateByPacketID(t *testing.T) {
	q := NewSendQueue(4, testMsgID)

	id := nson.NewId()
	m := testMsg{id: id}

	if err := q.TryEnqueue(m); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	if err := q.TryEnqueue(m); !errors.Is(err, ErrMessageAlreadyInQueue) {
		t.Fatalf("expected ErrMessageAlreadyInQueue, got %v", err)
	}
}

// TestSendQueue_Full 验证队列满时返回 ErrSendQueueFull
func TestSendQueue_Full(t *testing.T) {
	q := NewSendQueue(2, testMsgID)

	if err := q.TryEnqueue(testMsg{id: nson.NewId()}); err != nil {
		t.Fatalf("enqueue 1 failed: %v", err)
	}
	if err := q.TryEnqueue(testMsg{id: nson.NewId()}); err != nil {
		t.Fatalf("enqueue 2 failed: %v", err)
	}

	if err := q.TryEnqueue(testMsg{id: nson.NewId()}); !errors.Is(err, ErrSendQueueFull) {
		t.Fatalf("expected ErrSendQueueFull, got %v", err)
	}
}

// TestSendQueue_ZeroCapacityUsesDefault 验证 capacity<=0 时使用默认容量 100
func TestSendQueue_ZeroCapacityUsesDefault(t *testing.T) {
	q := NewSendQueue(0, testMsgID)

	for i := 0; i < 100; i++ {
		if err := q.TryEnqueue(testMsg{id: nson.NewId()}); err != nil {
			t.Fatalf("enqueue %d failed: %v", i, err)
		}
	}
	if err := q.TryEnqueue(testMsg{id: nson.NewId()}); !errors.Is(err, ErrSendQueueFull) {
		t.Fatalf("expected queue full at capacity 100, got %v", err)
	}
}
