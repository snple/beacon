package core

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/danclive/nson-go"
	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/packet"
)

// setupTestDB 创建临时测试数据库
func setupTestDB(t *testing.T) (*badger.DB, func()) {
	t.Helper()

	// 创建临时目录
	dir, err := os.MkdirTemp("", "queue_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// 打开 Badger 数据库
	opts := badger.DefaultOptions(dir).WithLoggingLevel(badger.ERROR)
	db, err := badger.Open(opts)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to open badger db: %v", err)
	}

	// 返回清理函数
	cleanup := func() {
		db.Close()
		os.RemoveAll(dir)
	}

	return db, cleanup
}

// createTestMessage 创建测试消息
func createTestMessage(topic string, payload []byte, qos packet.QoS) *Message {
	return &Message{
		Packet: &packet.PublishPacket{
			Topic:   topic,
			Payload: payload,
		},
		QoS:       qos,
		PacketID:  nson.NewId(),
		Timestamp: time.Now().Unix(),
	}
}

// equalId 比较两个 nson.Id 是否相等
func equalId(a, b nson.Id) bool {
	return bytes.Equal(a[:], b[:])
}

// TestQueueOrdering 测试队列的消息有序性
func TestQueueOrdering(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	queue := NewQueue(db, "test_queue_")

	// 生成一系列消息，使用固定的时间戳顺序
	messageCount := 100
	messages := make([]*Message, messageCount)

	t.Logf("Creating %d messages with sequential IDs...", messageCount)

	for i := 0; i < messageCount; i++ {
		msg := createTestMessage("test/topic", []byte{byte(i)}, packet.QoS1)

		// 确保 PacketID 是按顺序生成的（稍微延迟以确保 ID 递增）
		if i > 0 {
			time.Sleep(1 * time.Microsecond)
		}
		msg.PacketID = nson.NewId()
		messages[i] = msg

		// 入队
		if err := queue.Enqueue(msg); err != nil {
			t.Fatalf("failed to enqueue message %d: %v", i, err)
		}
	}

	t.Logf("Enqueued %d messages", messageCount)

	// 验证队列大小
	size, err := queue.Size()
	if err != nil {
		t.Fatalf("failed to get queue size: %v", err)
	}
	if size != uint64(messageCount) {
		t.Fatalf("expected queue size %d, got %d", messageCount, size)
	}

	// 检查第一个和最后一个 PacketID
	firstID, err := queue.GetFirstId()
	if err != nil {
		t.Fatalf("failed to get first ID: %v", err)
	}
	lastID, err := queue.GetLastId()
	if err != nil {
		t.Fatalf("failed to get last ID: %v", err)
	}

	t.Logf("First PacketID: %s", firstID.Hex())
	t.Logf("Last PacketID: %s", lastID.Hex())

	// 验证第一个和最后一个 ID 是否符合预期
	if !equalId(firstID, messages[0].PacketID) {
		t.Errorf("first ID mismatch: expected %s, got %s", messages[0].PacketID.Hex(), firstID.Hex())
	}
	if !equalId(lastID, messages[messageCount-1].PacketID) {
		t.Errorf("last ID mismatch: expected %s, got %s", messages[messageCount-1].PacketID.Hex(), lastID.Hex())
	}

	// 依次出队并验证顺序
	t.Log("Dequeuing messages and verifying order...")
	for i := 0; i < messageCount; i++ {
		msg, err := queue.Dequeue()
		if err != nil {
			t.Fatalf("failed to dequeue message %d: %v", i, err)
		}

		// 验证 PacketID 是否按顺序
		expectedID := messages[i].PacketID
		if !equalId(msg.PacketID, expectedID) {
			t.Errorf("message %d: PacketID mismatch, expected %s, got %s",
				i, expectedID.Hex(), msg.PacketID.Hex())
		}

		// 验证 payload
		if len(msg.Packet.Payload) != 1 || msg.Packet.Payload[0] != byte(i) {
			t.Errorf("message %d: payload mismatch, expected [%d], got %v",
				i, i, msg.Packet.Payload)
		}
	}

	// 验证队列已空
	isEmpty, err := queue.IsEmpty()
	if err != nil {
		t.Fatalf("failed to check if queue is empty: %v", err)
	}
	if !isEmpty {
		t.Error("queue should be empty after dequeuing all messages")
	}

	t.Log("All messages dequeued in correct order!")
}

// TestQueueOrderingWithRandomInsert 测试随机插入时的有序性
func TestQueueOrderingWithRandomInsert(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	queue := NewQueue(db, "test_random_")

	// 创建一些消息并打乱顺序插入
	messageCount := 50
	messages := make([]*Message, messageCount)

	// 先创建所有消息
	for i := 0; i < messageCount; i++ {
		time.Sleep(1 * time.Microsecond)
		msg := createTestMessage("test/topic", []byte{byte(i)}, packet.QoS1)
		msg.PacketID = nson.NewId()
		messages[i] = msg
	}

	t.Log("Inserting messages in reverse order...")
	// 倒序插入
	for i := messageCount - 1; i >= 0; i-- {
		if err := queue.Enqueue(messages[i]); err != nil {
			t.Fatalf("failed to enqueue message %d: %v", i, err)
		}
	}

	// 验证出队顺序仍然是按 PacketID 顺序（最小的先出）
	t.Log("Verifying dequeue order matches PacketID order...")
	for i := 0; i < messageCount; i++ {
		msg, err := queue.Dequeue()
		if err != nil {
			t.Fatalf("failed to dequeue message %d: %v", i, err)
		}

		// 验证应该得到最早生成的消息
		expectedID := messages[i].PacketID
		if !equalId(msg.PacketID, expectedID) {
			t.Errorf("message %d: PacketID mismatch, expected %s, got %s",
				i, expectedID.Hex(), msg.PacketID.Hex())
		}
	}

	t.Log("Messages dequeued in PacketID order despite reverse insertion!")
}

// TestQueuePeek 测试 Peek 操作不影响队列顺序
func TestQueuePeek(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	queue := NewQueue(db, "test_peek_")

	// 添加几条消息
	messageCount := 10
	messages := make([]*Message, messageCount)

	for i := 0; i < messageCount; i++ {
		time.Sleep(1 * time.Microsecond)
		msg := createTestMessage("test/topic", []byte{byte(i)}, packet.QoS1)
		msg.PacketID = nson.NewId()
		messages[i] = msg

		if err := queue.Enqueue(msg); err != nil {
			t.Fatalf("failed to enqueue message %d: %v", i, err)
		}
	}

	// 多次 Peek，应该总是返回第一条消息
	for i := 0; i < 5; i++ {
		msg, err := queue.Peek()
		if err != nil {
			t.Fatalf("failed to peek message: %v", err)
		}

		if !equalId(msg.PacketID, messages[0].PacketID) {
			t.Errorf("peek %d: expected first message %s, got %s",
				i, messages[0].PacketID.Hex(), msg.PacketID.Hex())
		}
	}

	// 验证队列大小没有变化
	size, err := queue.Size()
	if err != nil {
		t.Fatalf("failed to get queue size: %v", err)
	}
	if size != uint64(messageCount) {
		t.Errorf("expected queue size %d, got %d", messageCount, size)
	}

	t.Log("Peek operations preserved queue state correctly!")
}

// TestQueueDeleteMiddle 测试删除中间元素不影响其他元素顺序
func TestQueueDeleteMiddle(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	queue := NewQueue(db, "test_delete_")

	// 添加 10 条消息
	messageCount := 10
	messages := make([]*Message, messageCount)

	for i := 0; i < messageCount; i++ {
		time.Sleep(1 * time.Microsecond)
		msg := createTestMessage("test/topic", []byte{byte(i)}, packet.QoS1)
		msg.PacketID = nson.NewId()
		messages[i] = msg

		if err := queue.Enqueue(msg); err != nil {
			t.Fatalf("failed to enqueue message %d: %v", i, err)
		}
	}

	// 删除第 5 条消息（索引 4）
	deleteIndex := 4
	if err := queue.Delete(messages[deleteIndex].PacketID); err != nil {
		t.Fatalf("failed to delete message: %v", err)
	}

	t.Logf("Deleted message at index %d", deleteIndex)

	// 验证队列大小
	size, err := queue.Size()
	if err != nil {
		t.Fatalf("failed to get queue size: %v", err)
	}
	if size != uint64(messageCount-1) {
		t.Errorf("expected queue size %d, got %d", messageCount-1, size)
	}

	// 验证出队顺序，应该跳过被删除的那条
	expectedIndex := 0
	for i := 0; i < messageCount-1; i++ {
		// 跳过被删除的索引
		if expectedIndex == deleteIndex {
			expectedIndex++
		}

		msg, err := queue.Dequeue()
		if err != nil {
			t.Fatalf("failed to dequeue message %d: %v", i, err)
		}

		if !equalId(msg.PacketID, messages[expectedIndex].PacketID) {
			t.Errorf("message %d: expected PacketID %s, got %s",
				i, messages[expectedIndex].PacketID.Hex(), msg.PacketID.Hex())
		}

		expectedIndex++
	}

	t.Log("Queue order preserved after middle element deletion!")
}

// BenchmarkQueueEnqueue 基准测试入队性能
func BenchmarkQueueEnqueue(b *testing.B) {
	db, cleanup := setupTestDB(&testing.T{})
	defer cleanup()

	queue := NewQueue(db, "bench_enqueue_")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := createTestMessage("bench/topic", []byte("benchmark payload"), packet.QoS1)
		msg.PacketID = nson.NewId()
		if err := queue.Enqueue(msg); err != nil {
			b.Fatalf("failed to enqueue: %v", err)
		}
	}
}

// BenchmarkQueueDequeue 基准测试出队性能
func BenchmarkQueueDequeue(b *testing.B) {
	db, cleanup := setupTestDB(&testing.T{})
	defer cleanup()

	queue := NewQueue(db, "bench_dequeue_")

	// 预先填充队列
	for i := 0; i < b.N; i++ {
		msg := createTestMessage("bench/topic", []byte("benchmark payload"), packet.QoS1)
		msg.PacketID = nson.NewId()
		if err := queue.Enqueue(msg); err != nil {
			b.Fatalf("failed to enqueue: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := queue.Dequeue(); err != nil {
			b.Fatalf("failed to dequeue: %v", err)
		}
	}
}
