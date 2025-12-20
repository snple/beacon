package edge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/device"
	"github.com/snple/beacon/dt"
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

// buildNode: 为 Node 下的 Wire 和 Pin 添加 NodeID 前缀
func buildNode(node *dt.Node) *dt.Node {
	if node == nil {
		return nil
	}

	// 深拷贝 Tags
	tags := make([]string, len(node.Tags))
	copy(tags, node.Tags)

	// 深拷贝 Wires
	wires := make([]dt.Wire, len(node.Wires))
	for i, wire := range node.Wires {
		wires[i] = dt.Wire{
			ID:   node.ID + "." + wire.ID,
			Name: wire.Name,
			Tags: make([]string, len(wire.Tags)),
			Type: wire.Type,
			Pins: make([]dt.Pin, len(wire.Pins)),
		}
		copy(wires[i].Tags, wire.Tags)
		for j, pin := range wire.Pins {
			wires[i].Pins[j] = dt.Pin{
				ID:   node.ID + "." + pin.ID,
				Name: pin.Name,
				Tags: make([]string, len(pin.Tags)),
				Addr: pin.Addr,
				Type: pin.Type,
				Rw:   pin.Rw,
			}
			copy(wires[i].Pins[j].Tags, pin.Tags)
		}
	}

	return &dt.Node{
		ID:      node.ID,
		Name:    node.Name,
		Tags:    tags,
		Device:  node.Device,
		Updated: node.Updated,
		Wires:   wires,
	}
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
func (es *EdgeService) SetPinValue(ctx context.Context, value dt.PinValue, realtime bool) error {
	if value.ID == "" {
		return fmt.Errorf("please supply valid pinID")
	}
	if value.Value == nil {
		return fmt.Errorf("please supply valid value")
	}

	if value.Updated.IsZero() {
		value.Updated = time.Now()
	}

	pin, err := es.storage.GetPinByID(value.ID)
	if err != nil {
		return err
	}

	if pin.Rw == device.RO {
		return fmt.Errorf("pin is not writable")
	}

	if uint32(value.Value.DataType()) != pin.Type {
		return fmt.Errorf("invalid value for Pin.Type")
	}

	if err := es.storage.SetPinValue(ctx, value); err != nil {
		return err
	}

	// 通知 PinValue 变更
	if err := es.NotifyPinValue(value.ID, value.Value, realtime); err != nil {
		return err
	}

	es.dopts.logger.Sugar().Infof("SetPinValue: pinID=%s value=%v", value.ID, value)
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
func (es *EdgeService) ListPinValues(ctx context.Context, after time.Time, limit int) ([]dt.PinValue, error) {
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
func (es *EdgeService) SetPinWrite(ctx context.Context, value dt.PinValue) error {
	if value.ID == "" {
		return fmt.Errorf("please supply valid pinID")
	}
	if value.Value == nil {
		return fmt.Errorf("please supply valid value")
	}

	if value.Updated.IsZero() {
		value.Updated = time.Now()
	}

	pin, err := es.storage.GetPinByID(value.ID)
	if err != nil {
		return err
	}
	if pin.Rw == device.RO {
		return fmt.Errorf("pin is not writable")
	}

	if uint32(value.Value.DataType()) != pin.Type {
		return fmt.Errorf("invalid value for Pin.Type")
	}

	if err := es.storage.SetPinWrite(ctx, value); err != nil {
		return err
	}

	es.dopts.logger.Sugar().Infof("SetPinWrite: pinID=%s value=%v", value.ID, value)

	// 🔥 执行硬件操作
	if es.deviceMgr != nil {
		// 解析 wireID 和 pinName（格式：wireID.pinName）
		parts := strings.SplitN(value.ID, ".", 2)
		if len(parts) == 2 {
			wireID, pinName := parts[0], parts[1]
			if err := es.deviceMgr.Execute(ctx, wireID, pinName, value.Value); err != nil {
				// 硬件执行失败记录错误，但不影响数据保存
				es.dopts.logger.Sugar().Errorf("Execute actuator for pin %s: %v", value.ID, err)
			}
		}
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
func (es *EdgeService) ListPinWrites(ctx context.Context, after time.Time, limit int) ([]dt.PinValue, error) {
	_ = ctx
	return es.storage.ListPinWrites(after, limit)
}
