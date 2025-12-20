package storage

import (
	"testing"
	"time"

	"github.com/snple/beacon/dt"
)

func TestSimpleEncode(t *testing.T) {
	node := &dt.Node{
		ID:      "test",
		Name:    "test",
		Updated: time.Now(),
		Wires: []dt.Wire{
			{
				ID:   "w1",
				Name: "Wire1",
				Type: "t1",
				Pins: []dt.Pin{
					{ID: "p1", Name: "P1", Addr: "a1", Type: 1, Rw: 3},
				},
			},
			{
				ID:   "w2",
				Name: "Wire2",
				Type: "", // 空字符串
				Pins: []dt.Pin{
					{ID: "p2", Name: "P2", Addr: "a2", Type: 1, Rw: 3},
				},
			},
		},
	}

	data, err := dt.EncodeNode(node)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	t.Logf("Encoded %d bytes", len(data))

	decoded, err := dt.DecodeNode(data)
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
