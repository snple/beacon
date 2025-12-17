package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeEncoding(t *testing.T) {
	node := &Node{
		ID:      "node-001",
		Name:    "TestNode",
		Status:  1,
		Updated: time.Now(),
		Wires: []Wire{
			{
				ID:   "wire-001",
				Name: "Wire1",
				Type: "",
				Pins: []Pin{
					{ID: "pin-001", Name: "Pin1", Addr: "0x01", Type: 1, Rw: 3},
					{ID: "pin-002", Name: "Pin2", Addr: "0x02", Type: 1, Rw: 3},
				},
			},
		},
	}

	// 编码
	data, err := encodeNode(node)
	require.NoError(t, err, "encode should succeed")
	t.Logf("Encoded data length: %d bytes", len(data))
	t.Logf("Encoded data (hex): %x", data[:min(len(data), 100)])

	// 解码
	decoded, err := decodeNode(data)
	if err != nil {
		t.Logf("Decode error: %v", err)
	}
	require.NoError(t, err, "decode should succeed")

	// 验证
	assert.Equal(t, node.ID, decoded.ID)
	assert.Equal(t, node.Name, decoded.Name)
	assert.Equal(t, node.Status, decoded.Status)
	assert.Equal(t, len(node.Wires), len(decoded.Wires))

	if len(decoded.Wires) > 0 {
		assert.Equal(t, node.Wires[0].ID, decoded.Wires[0].ID)
		assert.Equal(t, len(node.Wires[0].Pins), len(decoded.Wires[0].Pins))
	}
}
