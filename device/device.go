package device

import "github.com/danclive/nson-go"

// ============================================================================
// 核心数据结构（设计为与 storage.Node/Wire/Pin 兼容）
// ============================================================================

// Device 设备模板定义
type Device struct {
	ID    string   // 设备类型标识（如 "smart_bulb_color"）
	Name  string   // 设备显示名称（如 "彩色智能灯泡"）
	Tags  []string // 标签列表（用于分类和查询，如 "lighting", "smart_home" 等）
	Desc  string   // 设备描述
	Wires []Wire   // Wire 模板列表
}

// Wire 端点模板定义（对应 storage.Wire）
type Wire struct {
	Name string   // Wire 名称
	Tags []string // 标签列表（用于分类和查询）
	Desc string   // Wire 描述
	Type string   // Wire 类型（可选，用于标识功能类型）
	Pins []Pin    // Pin 模板列表
}

// Pin 属性模板定义（对应 storage.Pin）
type Pin struct {
	Name      string     // Pin 名称
	Tags      []string   // 标签列表（用于分类和查询）
	Desc      string     // Pin 描述
	Type      uint32     // 数据类型（nson.DataType）
	Rw        int32      // 读写权限：0=只读，1=只写，2=读写，3=内部可写
	Default   nson.Value // 默认值（可选）
	Min       nson.Value // 最小值（可选，用于数值类型）
	Max       nson.Value // 最大值（可选，用于数值类型）
	Step      nson.Value // 步进值（可选，用于数值类型，如 0.5, 1, 10）
	Precision int        // 精度/小数位数（可选，用于浮点类型）
	Unit      string     // 单位（可选，如 "°C", "%", "W", "lux"）
	Enum      []EnumItem // 枚举值列表（可选，用于限定取值范围）
}

// EnumItem 枚举项
type EnumItem struct {
	Value nson.Value // 值
	Label string     // 显示名称
}

// Enum 创建枚举项
func Enum(value nson.Value, label string) EnumItem {
	return EnumItem{Value: value, Label: label}
}

// ============================================================================
// Pin Builder
// ============================================================================

// PinBuilderT Pin 构建器
type PinBuilderT struct {
	pin Pin
}

// PinBuilder 创建 Pin 构建器
func PinBuilder(name string, typ uint32, rw int32) *PinBuilderT {
	return &PinBuilderT{
		pin: Pin{
			Name: name,
			Type: typ,
			Rw:   rw,
			Tags: []string{},
		},
	}
}

// Desc 设置描述
func (p *PinBuilderT) Desc(desc string) *PinBuilderT {
	p.pin.Desc = desc
	return p
}

// Tags 设置标签
func (p *PinBuilderT) Tags(tags ...string) *PinBuilderT {
	p.pin.Tags = tags
	return p
}

// Default 设置默认值
func (p *PinBuilderT) Default(def nson.Value) *PinBuilderT {
	p.pin.Default = def
	return p
}

// Min 设置最小值
func (p *PinBuilderT) Min(min nson.Value) *PinBuilderT {
	p.pin.Min = min
	return p
}

// Max 设置最大值
func (p *PinBuilderT) Max(max nson.Value) *PinBuilderT {
	p.pin.Max = max
	return p
}

// Range 同时设置最小值和最大值
func (p *PinBuilderT) Range(min, max nson.Value) *PinBuilderT {
	p.pin.Min = min
	p.pin.Max = max
	return p
}

// Step 设置步进值
func (p *PinBuilderT) Step(step nson.Value) *PinBuilderT {
	p.pin.Step = step
	return p
}

// Precision 设置精度（小数位数）
func (p *PinBuilderT) Precision(precision int) *PinBuilderT {
	p.pin.Precision = precision
	return p
}

// Unit 设置单位
func (p *PinBuilderT) Unit(unit string) *PinBuilderT {
	p.pin.Unit = unit
	return p
}

// Enum 设置枚举值（带标签）
func (p *PinBuilderT) Enum(items ...EnumItem) *PinBuilderT {
	p.pin.Enum = items
	return p
}

// EnumValues 设置枚举值（简化版，无标签）
func (p *PinBuilderT) EnumValues(values ...nson.Value) *PinBuilderT {
	p.pin.Enum = make([]EnumItem, len(values))
	for i, v := range values {
		p.pin.Enum[i] = EnumItem{Value: v}
	}
	return p
}

// Build 构建 Pin
func (p *PinBuilderT) Build() Pin {
	return p.pin
}

// ============================================================================
// Wire Builder
// ============================================================================

// WireBuilderT Wire 构建器
type WireBuilderT struct {
	wire Wire
}

// WireBuilder 创建 Wire 构建器
func WireBuilder(name string) *WireBuilderT {
	return &WireBuilderT{
		wire: Wire{
			Name: name,
			Pins: []Pin{},
			Tags: []string{},
		},
	}
}

// Desc 设置描述
func (w *WireBuilderT) Desc(desc string) *WireBuilderT {
	w.wire.Desc = desc
	return w
}

// Tags 设置标签
func (w *WireBuilderT) Tags(tags ...string) *WireBuilderT {
	w.wire.Tags = tags
	return w
}

// Type 设置类型
func (w *WireBuilderT) Type(typ string) *WireBuilderT {
	w.wire.Type = typ
	return w
}

// Pin 添加 Pin
func (w *WireBuilderT) Pin(p Pin) *WireBuilderT {
	w.wire.Pins = append(w.wire.Pins, p)
	return w
}

// Build 构建 Wire（可选，用于显式构建）
func (w *WireBuilderT) Build() Wire {
	return w.wire
}

// ============================================================================
// Device Builder
// ============================================================================

// DeviceBuilderT 设备构建器
type DeviceBuilderT struct {
	device Device
}

// DeviceBuilder 创建设备构建器
func DeviceBuilder(id, name string) *DeviceBuilderT {
	return &DeviceBuilderT{
		device: Device{
			ID:    id,
			Name:  name,
			Wires: []Wire{},
			Tags:  []string{},
		},
	}
}

// Desc 设置描述
func (d *DeviceBuilderT) Desc(desc string) *DeviceBuilderT {
	d.device.Desc = desc
	return d
}

// Tags 设置标签
func (d *DeviceBuilderT) Tags(tags ...string) *DeviceBuilderT {
	d.device.Tags = tags
	return d
}

// Wire 添加 Wire
func (d *DeviceBuilderT) Wire(w *WireBuilderT) *DeviceBuilderT {
	d.device.Wires = append(d.device.Wires, w.wire)
	return d
}

// Done 完成构建
func (d *DeviceBuilderT) Done() Device {
	return d.device
}

// ============================================================================
// 兼容旧 API（可选，逐步迁移后删除）
// ============================================================================

// New 创建设备构建器（兼容旧 API）
func New(id, name string) *DeviceBuilderT {
	return DeviceBuilder(id, name)
}

// ============================================================================
// 常量定义
// ============================================================================

// 读写权限
const (
	RO = 0 // 只读（Read Only）
	WO = 1 // 只写（Write Only）
	RW = 2 // 读写（Read Write）
	IW = 3 // 内部可写（Internal Write）
)

// 开关状态
const (
	ON  = 1
	OFF = 0
)

// 默认名称
const (
	DEFAULT_NODE  = "node"
	DEFAULT_WIRE  = "wire"
	DEFAULT_TAG   = "tag"
	DEFAULT_CONST = "const"
)
