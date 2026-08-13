package client

import (
	"errors"
	"testing"
	"time"

	"github.com/danclive/nson-go"
	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/packet"
)

// setupClientQueueDB 创建用于客户端队列测试的临时 Badger 数据库
func setupClientQueueDB(t *testing.T) (*badger.DB, func()) {
	t.Helper()

	dir := t.TempDir()
	opts := badger.DefaultOptions(dir).WithLoggingLevel(badger.ERROR)
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatalf("failed to open badger db: %v", err)
	}

	return db, func() { db.Close() }
}

// clientQueueTestMessage 创建客户端测试消息
func clientQueueTestMessage(topic string, payload []byte, qos packet.QoS) *Message {
	pub := packet.NewPublishPacket(topic, payload)
	pub.QoS = qos
	pub.PacketID = nson.NewId()

	return &Message{
		Packet:    pub,
		Timestamp: time.Now().Unix(),
	}
}

// TestClientQueueDequeue_SkipsCorruptedHead 验证客户端 Dequeue 遇到损坏的头部消息时
// 会清除它并继续（与 core 侧一致的 H2 回归）。
func TestClientQueueDequeue_SkipsCorruptedHead(t *testing.T) {
	db, cleanup := setupClientQueueDB(t)
	defer cleanup()

	q := NewQueue(db, "test:")

	msg1 := clientQueueTestMessage("topic/1", []byte("first"), packet.QoS1)
	msg2 := clientQueueTestMessage("topic/2", []byte("second"), packet.QoS1)

	if err := q.Enqueue(msg1); err != nil {
		t.Fatalf("failed to enqueue msg1: %v", err)
	}
	if err := q.Enqueue(msg2); err != nil {
		t.Fatalf("failed to enqueue msg2: %v", err)
	}

	// 覆盖头部消息（msg1）的值为损坏数据
	// key 格式 = 队列前缀 + PacketID（与 mq.Queue.makeKey 一致）
	corruptKey := append([]byte("test:"), msg1.Packet.PacketID[:]...)
	if err := db.Update(func(txn *badger.Txn) error {
		return txn.Set(corruptKey, []byte{0xde, 0xad, 0xbe, 0xef})
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
	if got.Packet.PacketID != msg2.Packet.PacketID {
		t.Fatalf("expected packetID %v, got %v", msg2.Packet.PacketID, got.Packet.PacketID)
	}

	// 队列应已清空
	if _, err := q.Dequeue(); !errors.Is(err, ErrQueueEmpty) {
		t.Fatalf("expected ErrQueueEmpty, got %v", err)
	}
}
