package edge

import (
	"context"
	"fmt"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/device"
	"github.com/snple/beacon/dt"
	"github.com/snple/beacon/edge/storage"
)

func buildNodeFromTemplate(nodeID, name string, dev device.Device) (*dt.Node, error) {
	newNode := &dt.Node{
		ID:      nodeID,
		Name:    name,
		Tags:    dev.Tags,
		Device:  dev.ID,
		Updated: time.Now(),
		Wires:   make([]dt.Wire, 0, len(dev.Wires)),
	}

	for _, tw := range dev.Wires {
		wire := dt.Wire{
			ID:   stableWireID(nodeID, tw.Name),
			Name: tw.Name,
			Tags: tw.Tags,
			Type: tw.Type,
			Pins: make([]dt.Pin, 0, len(tw.Pins)),
		}

		for _, tp := range tw.Pins {
			wire.Pins = append(wire.Pins, dt.Pin{
				ID:   stablePinID(nodeID, tw.Name, tp.Name),
				Name: tp.Name,
				Tags: tp.Tags,
				Addr: "",
				Type: tp.Type,
				Rw:   tp.Rw,
			})
		}

		newNode.Wires = append(newNode.Wires, wire)
	}

	return newNode, nil
}

func stableWireID(nodeID, wireName string) string {
	return nodeID + "." + wireName
}

func stablePinID(nodeID, wireName, pinName string) string {
	return nodeID + "." + wireName + "." + pinName
}

// Node returns the current node configuration.
func (es *EdgeService) Node() (*dt.Node, error) {
	return cloneNode(es.storage.GetNode())
}

// WireByID fetches a wire config by ID.
func (es *EdgeService) WireByID(ctx context.Context, id string) (*dt.Wire, error) {
	_ = ctx
	w, err := es.storage.GetWireByID(id)
	if err != nil {
		return nil, err
	}
	return cloneWire(w), nil
}

// WireByName fetches a wire config by name.
func (es *EdgeService) WireByName(ctx context.Context, name string) (*dt.Wire, error) {
	_ = ctx
	w, err := es.storage.GetWireByName(name)
	if err != nil {
		return nil, err
	}
	return cloneWire(w), nil
}

// Wires lists all wires.
func (es *EdgeService) Wires(ctx context.Context) ([]*dt.Wire, error) {
	_ = ctx
	ws := es.storage.ListWires()
	out := make([]*dt.Wire, 0, len(ws))
	for _, w := range ws {
		out = append(out, cloneWire(w))
	}
	return out, nil
}

// PinByID fetches a pin config by ID.
func (es *EdgeService) PinByID(ctx context.Context, id string) (*dt.Pin, error) {
	_ = ctx
	p, err := es.storage.GetPinByID(id)
	if err != nil {
		return nil, err
	}
	return clonePin(p), nil
}

// PinByName fetches a pin config by name ("wire.pin").
func (es *EdgeService) PinByName(ctx context.Context, name string) (*dt.Pin, error) {
	_ = ctx
	p, err := es.storage.GetPinByName(name)
	if err != nil {
		return nil, err
	}
	return clonePin(p), nil
}

// Pins lists pins. If wireID is non-empty, it filters by wire.
func (es *EdgeService) Pins(ctx context.Context, wireID string) ([]*dt.Pin, error) {
	_ = ctx
	var pins []*dt.Pin
	if wireID != "" {
		ps, err := es.storage.ListPinsByWire(wireID)
		if err != nil {
			return nil, err
		}
		pins = ps
	} else {
		pins = es.storage.ListPins()
	}

	out := make([]*dt.Pin, 0, len(pins))
	for _, p := range pins {
		out = append(out, clonePin(p))
	}
	return out, nil
}

// GetPinValue returns latest PinValue.
func (es *EdgeService) GetPinValue(ctx context.Context, pinID string) (nson.Value, time.Time, error) {
	_ = ctx
	return es.storage.GetPinValue(pinID)
}

// SetPinValue sets PinValue, validates pin exists and datatype matches Pin.Type.
// If updated is zero, time.Now() is used.
func (es *EdgeService) SetPinValue(ctx context.Context, pinID string, value nson.Value, realtime bool) error {
	if pinID == "" {
		return fmt.Errorf("please supply valid pinID")
	}
	if value == nil {
		return fmt.Errorf("please supply valid value")
	}

	pin, err := es.storage.GetPinByID(pinID)
	if err != nil {
		return err
	}
	if uint32(value.DataType()) != pin.Type {
		return fmt.Errorf("invalid value for Pin.Type")
	}

	if err := es.storage.SetPinValue(ctx, pinID, value, time.Now()); err != nil {
		return err
	}

	// 通知 PinValue 变更
	if err := es.NotifyPinValue(pinID, value, realtime); err != nil {
		return err
	}

	return nil
}

// DeletePinValue deletes PinValue.
func (es *EdgeService) DeletePinValue(ctx context.Context, pinID string) error {
	if pinID == "" {
		return fmt.Errorf("please supply valid pinID")
	}
	return es.storage.DeletePinValue(ctx, pinID)
}

// ListPinValues lists PinValue entries after time.
func (es *EdgeService) ListPinValues(ctx context.Context, after time.Time, limit int) ([]storage.PinValueEntry, error) {
	_ = ctx
	return es.storage.ListPinValues(after, limit)
}

// GetPinWrite returns latest PinWrite.
func (es *EdgeService) GetPinWrite(ctx context.Context, pinID string) (nson.Value, time.Time, error) {
	_ = ctx
	return es.storage.GetPinWrite(pinID)
}

// SetPinWrite sets PinWrite, validates pin exists, is writable, and datatype matches Pin.Type.
// If updated is zero, time.Now() is used.
func (es *EdgeService) SetPinWrite(ctx context.Context, pinID string, value nson.Value, updated time.Time) error {
	if pinID == "" {
		return fmt.Errorf("please supply valid pinID")
	}
	if value == nil {
		return fmt.Errorf("please supply valid value")
	}

	pin, err := es.storage.GetPinByID(pinID)
	if err != nil {
		return err
	}
	if pin.Rw == device.RO {
		return fmt.Errorf("pin is not writable")
	}
	if uint32(value.DataType()) != pin.Type {
		return fmt.Errorf("invalid value for Pin.Type")
	}

	if updated.IsZero() {
		updated = time.Now()
	}

	if err := es.storage.SetPinWrite(ctx, pinID, value, updated); err != nil {
		return err
	}

	return nil
}

// DeletePinWrite deletes PinWrite.
func (es *EdgeService) DeletePinWrite(ctx context.Context, pinID string) error {
	if pinID == "" {
		return fmt.Errorf("please supply valid pinID")
	}
	return es.storage.DeletePinWrite(ctx, pinID)
}

// ListPinWrites lists PinWrite entries after time.
func (es *EdgeService) ListPinWrites(ctx context.Context, after time.Time, limit int) ([]storage.PinValueEntry, error) {
	_ = ctx
	return es.storage.ListPinWrites(after, limit)
}

func cloneNode(node *dt.Node, err error) (*dt.Node, error) {
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, fmt.Errorf("node not initialized")
	}

	out := &dt.Node{
		ID:     node.ID,
		Name:   node.Name,
		Tags:   append([]string(nil), node.Tags...),
		Device: node.Device,
		Wires:  make([]dt.Wire, 0, len(node.Wires)),
	}
	for i := range node.Wires {
		out.Wires = append(out.Wires, *cloneWire(&node.Wires[i]))
	}
	return out, nil
}

func cloneWire(w *dt.Wire) *dt.Wire {
	if w == nil {
		return nil
	}
	out := &dt.Wire{
		ID:   w.ID,
		Name: w.Name,
		Tags: append([]string(nil), w.Tags...),
		Type: w.Type,
		Pins: make([]dt.Pin, 0, len(w.Pins)),
	}
	for i := range w.Pins {
		out.Pins = append(out.Pins, *clonePin(&w.Pins[i]))
	}
	return out
}

func clonePin(p *dt.Pin) *dt.Pin {
	if p == nil {
		return nil
	}
	return &dt.Pin{
		ID:   p.ID,
		Name: p.Name,
		Tags: append([]string(nil), p.Tags...),
		Addr: p.Addr,
		Type: p.Type,
		Rw:   p.Rw,
	}
}
