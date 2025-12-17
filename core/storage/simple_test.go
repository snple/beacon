package storage

import (
	"testing"
	"time"
)

func TestSimpleEncode(t *testing.T) {
	node := &Node{
		ID:      "test",
		Name:    "test",
		Status:  1,
		Updated: time.Now(),
		Wires: []Wire{
			{
				ID:   "w1",
				Name: "Wire1",
				Type: "t1",
				Pins: []Pin{
					{ID: "p1", Name: "P1", Addr: "a1", Type: 1, Rw: 3},
				},
			},
			{
				ID:   "w2",
				Name: "Wire2",
				Type: "", // 空字符串
				Pins: []Pin{
					{ID: "p2", Name: "P2", Addr: "a2", Type: 1, Rw: 3},
				},
			},
		},
	}

	data, err := encodeNode(node)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	t.Logf("Encoded %d bytes", len(data))

	decoded, err := decodeNode(data)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	t.Logf("Decoded: %+v", decoded)

	if decoded.ID != node.ID {
		t.Errorf("ID mismatch")
	}
	if len(decoded.Wires) != len(node.Wires) {
		t.Errorf("Wires count mismatch: got %d, want %d", len(decoded.Wires), len(node.Wires))
	}
}
