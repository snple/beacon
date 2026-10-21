package mq

import (
	"errors"
	"testing"

	"github.com/danclive/nson-go"
	"github.com/dgraph-io/badger/v4"
)

type queueTestMsg struct {
	ID   nson.Id `nson:"id"`
	Body string  `nson:"body"`
}

func newTestQueue(t *testing.T) (*Queue[queueTestMsg], *badger.DB) {
	t.Helper()

	db, err := badger.Open(badger.DefaultOptions("").WithInMemory(true).WithLogger(nil))
	if err != nil {
		t.Fatalf("open badger: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	q := NewQueue(db, "test:", func(m queueTestMsg) nson.Id { return m.ID })
	return q, db
}

func TestQueue_FIFOAndDelete(t *testing.T) {
	q, _ := newTestQueue(t)

	ids := []nson.Id{nson.NewId(), nson.NewId(), nson.NewId()}
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
	id := nson.NewId()

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
	id := nson.NewId()

	if err := db.Update(func(txn *badger.Txn) error {
		return txn.Set(q.makeKey(id), []byte("not-an-nson-map"))
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
	next := nson.NewId()
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
		if err := q.Enqueue(queueTestMsg{ID: nson.NewId(), Body: "x"}); err != nil {
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
