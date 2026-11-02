package mq

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/packet"
)

// queueTestMsg 是队列泛型测试用的消息类型，实现 mq 的二进制编解码契约。
type queueTestMsg struct {
	ID   packet.PacketID
	Body string
}

func (m queueTestMsg) MarshalBinary() ([]byte, error) {
	if len(m.Body) > 0xFFFF {
		return nil, errors.New("body too long")
	}

	data := make([]byte, 1+12+2+len(m.Body))
	data[0] = 1 // 格式版本
	copy(data[1:13], m.ID[:])
	binary.BigEndian.PutUint16(data[13:15], uint16(len(m.Body)))
	copy(data[15:], m.Body)
	return data, nil
}

func (m *queueTestMsg) UnmarshalBinary(data []byte) error {
	const headerSize = 1 + 12 + 2

	if len(data) < headerSize || data[0] != 1 {
		return errors.New("invalid queue test message")
	}

	bodyLen := int(binary.BigEndian.Uint16(data[13:15]))
	if len(data) != headerSize+bodyLen {
		return errors.New("invalid queue test message length")
	}

	copy(m.ID[:], data[1:13])
	m.Body = string(data[headerSize:])
	return nil
}

func newTestQueue(t *testing.T) (*Queue[queueTestMsg], *badger.DB) {
	t.Helper()

	db, err := badger.Open(badger.DefaultOptions("").WithInMemory(true).WithLogger(nil))
	if err != nil {
		t.Fatalf("open badger: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	q := NewQueue(db, "test:", func(m queueTestMsg) packet.PacketID { return m.ID })
	return q, db
}

func TestQueue_FIFOAndDelete(t *testing.T) {
	q, _ := newTestQueue(t)

	ids := []packet.PacketID{packet.NewPacketID(), packet.NewPacketID(), packet.NewPacketID()}
	// 确保 ID 严格递增，避免同一毫秒内计数回绕影响断言。
	for i := range ids {
		if i > 0 && string(ids[i][:]) <= string(ids[i-1][:]) {
			t.Fatalf("test IDs are not increasing: %x <= %x", ids[i], ids[i-1])
		}
		if err := q.Enqueue(queueTestMsg{ID: ids[i], Body: string(rune('a' + i))}); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}

	if size, err := q.Size(); err != nil || size != 3 {
		t.Fatalf("Size = %d, %v; want 3, nil", size, err)
	}

	first, err := q.Peek()
	if err != nil || first.ID != ids[0] {
		t.Fatalf("Peek = %+v, %v; want ID %x", first, err, ids[0])
	}

	id, ok, err := q.PeekID()
	if err != nil || !ok || id != ids[0] {
		t.Fatalf("PeekID = %x, %v, %v; want %x", id, ok, err, ids[0])
	}

	for i := 0; i < 3; i++ {
		msg, err := q.Dequeue()
		if err != nil {
			t.Fatalf("dequeue %d: %v", i, err)
		}
		if msg.ID != ids[i] {
			t.Fatalf("dequeue %d: got %x, want %x", i, msg.ID, ids[i])
		}
	}
	if _, err := q.Dequeue(); !errors.Is(err, ErrQueueEmpty) {
		t.Fatalf("expected ErrQueueEmpty, got %v", err)
	}
}

func TestQueue_PutOverwriteAndDeleteIfExists(t *testing.T) {
	q, _ := newTestQueue(t)
	id := packet.NewPacketID()

	created, err := q.Put(queueTestMsg{ID: id, Body: "v1"})
	if err != nil || !created {
		t.Fatalf("Put v1 = %v, %v; want created=true", created, err)
	}

	created, err = q.Put(queueTestMsg{ID: id, Body: "v2"})
	if err != nil || created {
		t.Fatalf("Put v2 = %v, %v; want created=false", created, err)
	}

	got, err := q.Get(id)
	if err != nil || got.Body != "v2" {
		t.Fatalf("Get = %+v, %v; want body v2", got, err)
	}
	if size, _ := q.Size(); size != 1 {
		t.Fatalf("Size = %d, want 1", size)
	}

	existed, err := q.DeleteIfExists(id)
	if err != nil || !existed {
		t.Fatalf("DeleteIfExists = %v, %v; want true", existed, err)
	}
	existed, err = q.DeleteIfExists(id)
	if err != nil || existed {
		t.Fatalf("second DeleteIfExists = %v, %v; want false", existed, err)
	}
}

func TestQueue_CorruptedHeadIsRemoved(t *testing.T) {
	q, db := newTestQueue(t)
	id := packet.NewPacketID()

	if err := db.Update(func(txn *badger.Txn) error {
		return txn.Set(q.makeKey(id), []byte("not-a-valid-message"))
	}); err != nil {
		t.Fatalf("seed corrupted entry: %v", err)
	}

	if _, err := q.Dequeue(); !errors.Is(err, ErrCorruptedMessage) {
		t.Fatalf("expected ErrCorruptedMessage, got %v", err)
	}
	if size, _ := q.Size(); size != 0 {
		t.Fatalf("corrupted entry was not removed, size = %d", size)
	}

	// 后续消息仍可正常处理
	next := packet.NewPacketID()
	if err := q.Enqueue(queueTestMsg{ID: next, Body: "ok"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	msg, err := q.Dequeue()
	if err != nil || msg.ID != next {
		t.Fatalf("dequeue after corruption = %+v, %v", msg, err)
	}
}

func TestQueue_Clear(t *testing.T) {
	q, _ := newTestQueue(t)
	for i := 0; i < 10; i++ {
		if err := q.Enqueue(queueTestMsg{ID: packet.NewPacketID(), Body: "x"}); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}

	if err := q.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if size, err := q.Size(); err != nil || size != 0 {
		t.Fatalf("Size after Clear = %d, %v; want 0", size, err)
	}
	if empty, err := q.IsEmpty(); err != nil || !empty {
		t.Fatalf("IsEmpty = %v, %v; want true", empty, err)
	}
}

// TestQueue_MalformedKeyLengthDoesNotPanic 验证同前缀下长度异常的 key
// 不会让 PeekID/GetFirstId/GetLastId 在读路径 panic（切片转数组要求长度足够）。
func TestQueue_MalformedKeyLengthDoesNotPanic(t *testing.T) {
	q, db := newTestQueue(t)

	if err := db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("test:short"), []byte("x"))
	}); err != nil {
		t.Fatalf("seed short key: %v", err)
	}

	id, found, err := q.PeekID()
	if err != nil {
		t.Fatalf("PeekID: %v", err)
	}
	if !found || !id.IsZero() {
		t.Fatalf("PeekID = %s, found=%v; want zero id", id.Hex(), found)
	}

	if _, err := q.GetFirstId(); err != nil {
		t.Fatalf("GetFirstId: %v", err)
	}
	if _, err := q.GetLastId(); err != nil {
		t.Fatalf("GetLastId: %v", err)
	}
}
