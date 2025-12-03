// Package device 提供标准设备定义注册表
//
// 设计理念：Builder 链式 API，声明式设备定义
//
// 使用示例：
//
//	var SmartBulb = New("smart_bulb", "智能灯泡").
//	    Category("lighting").
//	    Wire("light").
//	        Pin("onoff", Bool, RW).
//	        Pin("level", U32, RW).
//	    Done()
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
	Name    string     // Pin 名称
	Tags    []string   // 标签列表（用于分类和查询）
	Desc    string     // Pin 描述
	Type    uint32     // 数据类型（nson.DataType）
	Rw      int32      // 读写权限：0=只读，1=读写
	Default nson.Value // 默认值（可选）

}

// ============================================================================
// Builder API
// ============================================================================

// DeviceBuilder 设备构建器
type DeviceBuilder struct {
	device      Device
	currentWire *Wire
}

// New 创建设备构建器
func New(id, name string) *DeviceBuilder {
	return &DeviceBuilder{
		device: Device{
			ID:    id,
			Name:  name,
			Wires: make([]Wire, 0),
		},
	}
}

// Desc 设置描述
func (b *DeviceBuilder) Desc(desc string) *DeviceBuilder {
	b.device.Desc = desc
	return b
}

// Tags 设置标签
func (b *DeviceBuilder) Tags(tags ...string) *DeviceBuilder {
	b.device.Tags = tags
	return b
}

// Wire 创建新的 Wire
func (b *DeviceBuilder) Wire(name string) *DeviceBuilder {
	b.saveCurrentWire()
	b.currentWire = &Wire{
		Name: name,
		Pins: make([]Pin, 0),
	}
	return b
}

// WireType 设置当前 Wire 的类型
func (b *DeviceBuilder) WireType(typ string) *DeviceBuilder {
	if b.currentWire == nil {
		panic("must call Wire() before WireType()")
	}
	b.currentWire.Type = typ
	return b
}

// WireDesc 设置当前 Wire 的描述
func (b *DeviceBuilder) WireDesc(desc string) *DeviceBuilder {
	if b.currentWire == nil {
		panic("must call Wire() before WireDesc()")
	}
	b.currentWire.Desc = desc
	return b
}

// WireTags 设置当前 Wire 的标签列表
func (b *DeviceBuilder) WireTags(tags ...string) *DeviceBuilder {
	if b.currentWire == nil {
		panic("must call Wire() before WireTags()")
	}
	b.currentWire.Tags = tags
	return b
}

// Pin 添加 Pin 到当前 Wire
func (b *DeviceBuilder) Pin(name string, typ uint32, rw int32) *DeviceBuilder {
	if b.currentWire == nil {
		panic("must call Wire() before Pin()")
	}
	b.currentWire.Pins = append(b.currentWire.Pins, Pin{
		Name: name,
		Type: typ,
		Rw:   rw,
		Tags: []string{},
	})
	return b
}

// PinWithDesc 添加带描述的 Pin
func (b *DeviceBuilder) PinWithDesc(name, desc string, typ uint32, rw int32) *DeviceBuilder {
	if b.currentWire == nil {
		panic("must call Wire() before Pin()")
	}
	b.currentWire.Pins = append(b.currentWire.Pins, Pin{
		Name: name,
		Desc: desc,
		Type: typ,
		Rw:   rw,
		Tags: []string{},
	})
	return b
}

// PinFull 添加完整配置的 Pin
func (b *DeviceBuilder) PinFull(name, desc string, typ uint32, rw int32, def nson.Value, tags []string) *DeviceBuilder {
	if b.currentWire == nil {
		panic("must call Wire() before Pin()")
	}
	if tags == nil {
		tags = []string{}
	}
	b.currentWire.Pins = append(b.currentWire.Pins, Pin{
		Name:    name,
		Desc:    desc,
		Type:    typ,
		Rw:      rw,
		Default: def,
		Tags:    tags,
	})
	return b
}

// Done 完成构建
func (b *DeviceBuilder) Done() Device {
	b.saveCurrentWire()
	return b.device
}

func (b *DeviceBuilder) saveCurrentWire() {
	if b.currentWire != nil {
		b.device.Wires = append(b.device.Wires, *b.currentWire)
		b.currentWire = nil
	}
}

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
