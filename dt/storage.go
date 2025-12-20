package dt

import (
	"time"

	"github.com/danclive/nson-go"
)

// ============================================================================
// Storage 数据模型 - 用于 Core 和 Edge 的存储层
// ============================================================================

// Node 节点配置
type Node struct {
	ID      string    `nson:"id"`
	Name    string    `nson:"name"`
	Tags    []string  `nson:"tags,omitempty"`    // 节点标签列表
	Device  string    `nson:"device,omitempty"`  // 设备模板 ID（指向 device.Device.ID）
	Updated time.Time `nson:"updated,omitempty"` // Core 端使用
	Wires   []Wire    `nson:"wires"`
}

// Wire 通道配置
type Wire struct {
	ID   string   `nson:"id"`
	Name string   `nson:"name"`
	Tags []string `nson:"tags,omitempty"`
	Type string   `nson:"type"`
	Pins []Pin    `nson:"pins"`
}

// Pin 点位配置
type Pin struct {
	ID   string   `nson:"id"`
	Name string   `nson:"name"`
	Tags []string `nson:"tags,omitempty"`
	Addr string   `nson:"addr"`
	Type uint32   `nson:"type"` // nson.DataType
	Rw   int32    `nson:"rw"`
}

// PinValue Pin 值
type PinValue struct {
	ID      string     `nson:"id"`
	Value   nson.Value `nson:"value"`
	Updated time.Time  `nson:"updated"`
}

// PinNameValue Pin 名称值
type PinNameValue struct {
	Name    string     `nson:"name"`
	Value   nson.Value `nson:"value"`
	Updated time.Time  `nson:"updated"`
}

// DeepCopyNode 深拷贝 Node 及其嵌套结构
func DeepCopyNode(node *Node) Node {
	if node == nil {
		return Node{}
	}

	// 深拷贝 Tags
	tags := make([]string, len(node.Tags))
	copy(tags, node.Tags)

	// 深拷贝 Wires
	wires := make([]Wire, len(node.Wires))
	for i, wire := range node.Wires {
		wires[i] = Wire{
			ID:   wire.ID,
			Name: wire.Name,
			Tags: make([]string, len(wire.Tags)), // 深拷贝 Tags
			Type: wire.Type,
			Pins: make([]Pin, len(wire.Pins)), // 深拷贝 Pins
		}
		copy(wires[i].Tags, wire.Tags)
		for j, pin := range wire.Pins {
			wires[i].Pins[j] = Pin{
				ID:   pin.ID,
				Name: pin.Name,
				Tags: make([]string, len(pin.Tags)), // 深拷贝 Tags
				Addr: pin.Addr,
				Type: pin.Type,
				Rw:   pin.Rw,
			}
			copy(wires[i].Pins[j].Tags, pin.Tags)
		}
	}

	return Node{
		ID:      node.ID,
		Name:    node.Name,
		Tags:    tags,
		Device:  node.Device,
		Updated: node.Updated,
		Wires:   wires,
	}
}

// DeepCopyWire 深拷贝 Wire 及其嵌套结构
func DeepCopyWire(wire *Wire) Wire {
	if wire == nil {
		return Wire{}
	}

	// 深拷贝 Tags
	tags := make([]string, len(wire.Tags))
	copy(tags, wire.Tags)

	// 深拷贝 Pins
	pins := make([]Pin, len(wire.Pins))
	for i, pin := range wire.Pins {
		pins[i] = Pin{
			ID:   pin.ID,
			Name: pin.Name,
			Tags: make([]string, len(pin.Tags)), // 深拷贝 Tags
			Addr: pin.Addr,
			Type: pin.Type,
			Rw:   pin.Rw,
		}
		copy(pins[i].Tags, pin.Tags)
	}

	return Wire{
		ID:   wire.ID,
		Name: wire.Name,
		Tags: tags,
		Type: wire.Type,
		Pins: pins,
	}
}

// DeepCopyPin 深拷贝 Pin 结构
func DeepCopyPin(pin *Pin) Pin {
	if pin == nil {
		return Pin{}
	}

	// 深拷贝 Tags
	tags := make([]string, len(pin.Tags))
	copy(tags, pin.Tags)

	return Pin{
		ID:   pin.ID,
		Name: pin.Name,
		Tags: tags,
		Addr: pin.Addr,
		Type: pin.Type,
		Rw:   pin.Rw,
	}
}
