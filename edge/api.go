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

func buildNodeFromTemplate(nodeID, name string, dev device.Device) (dt.Node, error) {
	newNode := dt.Node{
		ID:      nodeID,
		Name:    name,
		Tags:    dev.Tags,
		Device:  dev.ID,
		Updated: time.Now(),
		Wires:   make([]dt.Wire, 0, len(dev.Wires)),
	}

	for _, tw := range dev.Wires {
		wire := dt.Wire{
			ID:   tw.Name,
			Name: tw.Name,
			Tags: tw.Tags,
			Type: tw.Type,
			Pins: make([]dt.Pin, 0, len(tw.Pins)),
		}

		for _, tp := range tw.Pins {
			wire.Pins = append(wire.Pins, dt.Pin{
				ID:   tw.Name + "." + tp.Name,
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

// Node returns the current node configuration.
func (es *EdgeService) Node() dt.Node {
	return es.storage.GetNode()
}

// WireByID fetches a wire config by ID.
func (es *EdgeService) WireByID(id string) (*dt.Wire, error) {
	return es.storage.GetWireByID(id)
}

// Wires lists all wires.
func (es *EdgeService) Wires() []dt.Wire {
	return es.storage.ListWires()
}

// PinByID fetches a pin config by ID.
func (es *EdgeService) PinByID(id string) (*dt.Pin, error) {
	return es.storage.GetPinByID(id)
}

// Pins lists pins. If wireID is non-empty, it filters by wire.
func (es *EdgeService) Pins(wireID string) ([]dt.Pin, error) {
	var pins []dt.Pin
	if wireID != "" {
		ps, err := es.storage.ListPinsByWire(wireID)
		if err != nil {
			return nil, err
		}
		pins = ps
	} else {
		pins = es.storage.ListPins()
	}

	return pins, nil
}

// GetPinValue returns latest PinValue.
func (es *EdgeService) GetPinValue(pinID string) (nson.Value, time.Time, error) {
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

	es.dopts.logger.Sugar().Infof("SetPinValue: pinID=%s value=%v", pinID, value)

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

	es.dopts.logger.Sugar().Infof("SetPinWrite: pinID=%s value=%v", pinID, value)

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
