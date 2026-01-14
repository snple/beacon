package edge

import (
	"context"
	"testing"

	"github.com/dgraph-io/badger/v4"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/device"
)

func TestApplyDeviceTemplate_GeneratesStableIDs(t *testing.T) {
	ctx := context.Background()

	dev := device.Device{
		ID:   "test_device_apply",
		Name: "Test Device",
		Wires: []device.Wire{
			{
				Name: "w1",
				Type: "test",
				Pins: []device.Pin{
					{Name: "p1", Type: nson.DataTypeBOOL, Rw: device.RW},
					{Name: "p2", Type: nson.DataTypeU8, Rw: device.RO},
				},
			},
			{
				Name: "w2",
				Pins: []device.Pin{
					{Name: "p", Type: nson.DataTypeSTRING, Rw: device.WO},
				},
			},
		},
	}

	nodeID := "SN123"
	dm := device.NewDeviceManager(dev)
	es, err := Edge(
		WithNodeID(nodeID, "secret"),
		WithDeviceManager(dm),
	)
	if err != nil {
		t.Fatalf("Edge(): %v", err)
	}
	defer es.Stop()

	node := es.GetNode()
	if node.ID != nodeID {
		t.Fatalf("node.ID=%q want %q", node.ID, nodeID)
	}
	if node.Device != dev.ID {
		t.Fatalf("node.Device=%q want %q", node.Device, dev.ID)
	}
	if len(node.Wires) != 2 {
		t.Fatalf("len(node.Wires)=%d", len(node.Wires))
	}

	if node.Wires[0].ID != "w1" {
		t.Fatalf("wire[0].ID=%q", node.Wires[0].ID)
	}
	if node.Wires[1].ID != "w2" {
		t.Fatalf("wire[1].ID=%q", node.Wires[1].ID)
	}

	if node.Wires[0].Pins[0].ID != "w1.p1" {
		t.Fatalf("pin id=%q", node.Wires[0].Pins[0].ID)
	}
	if node.Wires[0].Pins[1].ID != "w1.p2" {
		t.Fatalf("pin id=%q", node.Wires[0].Pins[1].ID)
	}
	if node.Wires[1].Pins[0].ID != "w2.p" {
		t.Fatalf("pin id=%q", node.Wires[1].Pins[0].ID)
	}

	_ = ctx
}

func TestApplyDeviceTemplate_NodeIDMismatch(t *testing.T) {
	ctx := context.Background()
	dev1 := device.Device{ID: "test_device_1", Name: "Test", Wires: []device.Wire{{Name: "w", Pins: []device.Pin{{Name: "p", Type: nson.DataTypeBOOL, Rw: device.RW}}}}}
	dev2 := device.Device{ID: "test_device_2", Name: "Test", Wires: []device.Wire{{Name: "w", Pins: []device.Pin{{Name: "p", Type: nson.DataTypeBOOL, Rw: device.RW}}}}}
	dir := t.TempDir()
	bo := badger.DefaultOptions(dir).WithValueDir(dir)

	// First create with dev1.
	dm1 := device.NewDeviceManager(dev1)
	es1, err := Edge(
		WithBadger(bo),
		WithNodeID("SN1", "secret"),
		WithDeviceManager(dm1),
	)
	if err != nil {
		t.Fatalf("Edge() first: %v", err)
	}
	es1.Stop()

	// Second create with dev2 on same storage should succeed (node config is regenerated from template each time).
	// Note: Device configuration is NOT persisted - it's provided via code/config file.
	dm2 := device.NewDeviceManager(dev2)
	es2, err := Edge(
		WithBadger(bo),
		WithNodeID("SN1", "secret"),
		WithDeviceManager(dm2),
	)
	if err != nil {
		t.Fatalf("Edge() second: %v", err)
	}
	defer es2.Stop()

	// Verify node uses dev2 template.
	node := es2.GetNode()
	if node.Device != dev2.ID {
		t.Fatalf("node.Device=%q want %q", node.Device, dev2.ID)
	}
	_ = ctx
}
