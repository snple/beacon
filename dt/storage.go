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
	Desc string   `nson:"desc,omitempty"` // Wire 描述
	Type string   `nson:"type"`
	Pins []Pin    `nson:"pins"`
}

// Pin 点位配置
type Pin struct {
	ID        string     `nson:"id"`
	Name      string     `nson:"name"`
	Tags      []string   `nson:"tags,omitempty"`
	Desc      string     `nson:"desc,omitempty"` // Pin 描述
	Addr      string     `nson:"addr"`
	Type      uint8      `nson:"type"` // nson.DataType
	Rw        int32      `nson:"rw"`
	Default   nson.Value `nson:"default,omitempty"`   // 默认值
	Min       nson.Value `nson:"min,omitempty"`       // 最小值
	Max       nson.Value `nson:"max,omitempty"`       // 最大值
	Step      nson.Value `nson:"step,omitempty"`      // 步进值
	Precision int        `nson:"precision,omitempty"` // 精度/小数位数
	Unit      string     `nson:"unit,omitempty"`      // 单位
	Enum      []EnumItem `nson:"enum,omitempty"`      // 枚举值列表
}

// EnumItem 枚举项
type EnumItem struct {
	Value nson.Value `nson:"value"` // 值
	Label string     `nson:"label"` // 显示名称
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
		wires[i] = DeepCopyWire(&wire)
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
		pins[i] = DeepCopyPin(&pin)
	}

	return Wire{
		ID:   wire.ID,
		Name: wire.Name,
		Tags: tags,
		Desc: wire.Desc,
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

	// 深拷贝 Enum
	var enumItems []EnumItem
	if pin.Enum != nil {
		enumItems = make([]EnumItem, len(pin.Enum))
		for i, item := range pin.Enum {
			enumItems[i] = EnumItem{
				Value: item.Value,
				Label: item.Label,
			}
		}
	}

	return Pin{
		ID:        pin.ID,
		Name:      pin.Name,
		Tags:      tags,
		Desc:      pin.Desc,
		Addr:      pin.Addr,
		Type:      pin.Type,
		Rw:        pin.Rw,
		Default:   pin.Default,
		Min:       pin.Min,
		Max:       pin.Max,
		Step:      pin.Step,
		Precision: pin.Precision,
		Unit:      pin.Unit,
		Enum:      enumItems,
	}
}
