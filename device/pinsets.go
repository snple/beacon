package device

import (
	"github.com/danclive/nson-go"
	"github.com/snple/beacon/dt"
)

// ============================================================================
// 预定义的 Pin 集合（可复用）
// ============================================================================

// PinSet Pin 集合构建器
type PinSet struct {
	pins []Pin
}

// NewPinSet 创建 Pin 集合
func NewPinSet() *PinSet {
	return &PinSet{pins: make([]Pin, 0)}
}

// Add 添加 Pin
func (ps *PinSet) Add(name, desc string, typ uint32, rw int32, def nson.Value, tags []string) *PinSet {
	if tags == nil {
		tags = []string{}
	}
	ps.pins = append(ps.pins, Pin{
		Name:    name,
		Desc:    desc,
		Type:    typ,
		Rw:      rw,
		Default: def,
		Tags:    tags,
	})
	return ps
}

// Pins 获取所有 Pin
func (ps *PinSet) Pins() []Pin {
	return ps.pins
}

// ============================================================================
// 标准 Pin 集合（对应原来的 Cluster 概念）
// ============================================================================

var (
	// OnOffPins 开关控制
	OnOffPins = NewPinSet().
			Add("onoff", "开关状态", dt.TypeBool, RW, nson.Bool(false), []string{"control"}).
			Pins()

	// LevelControlPins 级别控制
	LevelControlPins = NewPinSet().
				Add("level", "当前级别(0-254)", dt.TypeU32, RW, nson.U32(0), []string{"control"}).
				Add("min_level", "最小级别", dt.TypeU32, RO, nson.U32(0), []string{"range"}).
				Add("max_level", "最大级别", dt.TypeU32, RO, nson.U32(254), []string{"range"}).
				Pins()

	// ColorControlPins 颜色控制
	ColorControlPins = NewPinSet().
				Add("hue", "色相(0-254)", dt.TypeU32, RW, nson.U32(0), []string{"color"}).
				Add("saturation", "饱和度(0-254)", dt.TypeU32, RW, nson.U32(0), []string{"color"}).
				Pins()

	// TemperaturePins 温度测量
	TemperaturePins = NewPinSet().
			Add("temperature", "当前温度(0.01°C)", dt.TypeI32, RO, nson.I32(2500), []string{"sensor"}).
			Add("temp_min", "最小可测温度", dt.TypeI32, RO, nson.I32(-4000), []string{"range"}).
			Add("temp_max", "最大可测温度", dt.TypeI32, RO, nson.I32(12500), []string{"range"}).
			Pins()

	// HumidityPins 湿度测量
	HumidityPins = NewPinSet().
			Add("humidity", "当前湿度(0.01%)", dt.TypeU32, RO, nson.U32(5000), []string{"sensor"}).
			Pins()

	// BasicInfoPins 基本信息
	BasicInfoPins = NewPinSet().
			Add("vendor_name", "厂商名称", dt.TypeString, RO, nson.String(""), []string{"meta"}).
			Add("product_name", "产品名称", dt.TypeString, RO, nson.String(""), []string{"meta"}).
			Add("serial_number", "序列号", dt.TypeString, RO, nson.String(""), []string{"meta"}).
			Pins()
)

// ============================================================================
// Wire 扩展：批量添加 Pin
// ============================================================================

// Pins 批量添加预定义的 Pin 集合
func (b *DeviceBuilder) Pins(pins []Pin) *DeviceBuilder {
	if b.currentWire == nil {
		panic("must call Wire() before Pins()")
	}
	b.currentWire.Pins = append(b.currentWire.Pins, pins...)
	return b
}
