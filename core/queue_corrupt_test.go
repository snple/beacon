package core

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/packet"
)

// TestQueueDequeue_SkipsCorruptedHead 验证 Dequeue 遇到损坏的头部消息时会清除它并继续，
// 而不是永久阻塞队列（H2 回归：坏消息曾导致 dispatchLoop 永久卡死）。
func TestQueueDequeue_SkipsCorruptedHead(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	q := NewQueue(db, "test:")

	msg1 := createTestMessage("topic/1", []byte("first"), packet.QoS1)
	msg2 := createTestMessage("topic/2", []byte("second"), packet.QoS1)

	if err := q.Enqueue(msg1); err != nil {
		t.Fatalf("failed to enqueue msg1: %v", err)
	}
	if err := q.Enqueue(msg2); err != nil {
		t.Fatalf("failed to enqueue msg2: %v", err)
	}

	// 覆盖头部消息（msg1）的值为损坏数据，模拟持久化数据损坏
	corruptKey := q.makeKey(msg1.PacketID)
	if err := db.Update(func(txn *badger.Txn) error {
		return txn.Set(corruptKey, []byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01})
	}); err != nil {
		t.Fatalf("failed to corrupt head message: %v", err)
	}

	// 第一次 Dequeue：应返回 ErrCorruptedMessage 且坏消息被清除
	_, err := q.Dequeue()
	if !errors.Is(err, ErrCorruptedMessage) {
		t.Fatalf("expected ErrCorruptedMessage, got %v", err)
	}

	// 第二次 Dequeue：应取到 msg2（队列未被坏消息阻塞）
	got, err := q.Dequeue()
	if err != nil {
		t.Fatalf("expected msg2 after corrupted head removed, got error: %v", err)
	}
	if got.PacketID != msg2.PacketID {
		t.Fatalf("expected packetID %v, got %v", msg2.PacketID, got.PacketID)
	}

	// 队列应已清空
	if _, err := q.Dequeue(); !errors.Is(err, ErrQueueEmpty) {
		t.Fatalf("expected ErrQueueEmpty, got %v", err)
	}
}

// TestQueuePeek_ReportsCorruptedHead 验证 Peek 对损坏头部消息返回 ErrCorruptedMessage，
// 且 DeleteHead 可清除坏消息。
func TestQueuePeek_ReportsCorruptedHead(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	q := NewQueue(db, "test:")

	msg := createTestMessage("topic/1", []byte("hello"), packet.QoS1)
	if err := q.Enqueue(msg); err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	corruptKey := q.makeKey(msg.PacketID)
	if err := db.Update(func(txn *badger.Txn) error {
		return txn.Set(corruptKey, []byte{0xde, 0xad})
	}); err != nil {
		t.Fatalf("failed to corrupt message: %v", err)
	}

	_, err := q.Peek()
	if !errors.Is(err, ErrCorruptedMessage) {
		t.Fatalf("expected ErrCorruptedMessage, got %v", err)
	}

	// DeleteHead 应清除坏消息
	if err := q.DeleteHead(); err != nil {
		t.Fatalf("failed to delete head: %v", err)
	}

	if _, err := q.Peek(); !errors.Is(err, ErrQueueEmpty) {
		t.Fatalf("expected ErrQueueEmpty after DeleteHead, got %v", err)
	}
}
