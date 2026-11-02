package packet

import (
	"bytes"
	"testing"
)

func TestNewPacketIDUnique(t *testing.T) {
	const count = 10000

	seen := make(map[PacketID]struct{}, count)
	for i := 0; i < count; i++ {
		id := NewPacketID()
		if id.IsZero() {
			t.Fatal("NewPacketID returned zero value")
		}
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate PacketID at %d: %s", i, id.Hex())
		}
		seen[id] = struct{}{}

		if got := len(id.Hex()); got != 24 {
			t.Fatalf("Hex length = %d, want 24", got)
		}
		if id.String() != id.Hex() {
			t.Fatalf("String() = %q, want %q", id.String(), id.Hex())
		}
	}
}

func TestPacketIDTimestampNonDecreasing(t *testing.T) {
	prev := NewPacketID()
	for i := 0; i < 1000; i++ {
		id := NewPacketID()
		// 前 6 字节是毫秒时间戳，必须单调不减，队列 key 的 FIFO 排序依赖这一点。
		if bytes.Compare(id[:6], prev[:6]) < 0 {
			t.Fatalf("timestamp went backwards: %x < %x", id[:6], prev[:6])
		}
		prev = id
	}
}

func TestPacketIDIsZero(t *testing.T) {
	if !(PacketID{}).IsZero() {
		t.Fatal("zero value should report IsZero")
	}
	if NewPacketID().IsZero() {
		t.Fatal("generated PacketID should not be zero")
	}
}
