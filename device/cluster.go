// Package device 提供标准化的设备定义能力
// Cluster 是 Pin 的逻辑分组/模板，用于标准化设备定义
//
// 映射关系：
//   - Node   = Device（设备）
//   - Wire   = Endpoint（端点/功能实例）
//   - Pin    = Attribute（属性）
//   - Cluster = Pin 的逻辑分组（模板）
//
// 使用方式：
//  1. 定义 Cluster 模板（包含一组标准 Pin 定义）
//  2. 创建 Wire 时，指定包含哪些 Cluster
//  3. 系统自动将 Cluster 中定义的 Pin 添加到 Wire
package device

import (
	"github.com/danclive/nson-go"
	"github.com/snple/beacon/dt"
)

// ============================================================================
// 基础类型
// ============================================================================

type ClusterID uint32

// ============================================================================
// PinTemplate - Pin 的模板定义
// ============================================================================

// PinTemplate Pin 模板，定义 Pin 的标准结构
type PinTemplate struct {
	Name    string     // Pin 名称（在 Wire 中唯一）
	Desc    string     // 描述
	Type    uint32     // 数据类型: dt.TypeBool, dt.TypeI32, etc.
	Rw      int32      // 0: 只读, 1: 读写
	Default nson.Value // 默认值
	Tags    string     // 标签
}

// ============================================================================
// Cluster - Pin 的逻辑分组模板
// ============================================================================

// Cluster 定义一组标准的 Pin，作为模板使用
// 当 Wire 包含某个 Cluster 时，该 Cluster 的所有 Pin 将添加到 Wire
type Cluster struct {
	ID          ClusterID     // Cluster ID（可选，用于标识）
	Name        string        // Cluster 名称
	Description string        // 描述
	Pins        []PinTemplate // 包含的 Pin 模板
}

// GetPinTemplates 获取所有 Pin 模板
func (c *Cluster) GetPinTemplates() []PinTemplate {
	return c.Pins
}

// GetPinTemplate 根据名称获取 Pin 模板
func (c *Cluster) GetPinTemplate(name string) *PinTemplate {
	for i := range c.Pins {
		if c.Pins[i].Name == name {
			return &c.Pins[i]
		}
	}
	return nil
}

// ============================================================================
// 标准 Cluster 定义
// ============================================================================

// OnOff Cluster - 开关控制
var OnOffCluster = Cluster{
	ID:          0x0006,
	Name:        "OnOff",
	Description: "开关控制",
	Pins: []PinTemplate{
		{
			Name:    "onoff",
			Desc:    "开关状态",
			Type:    dt.TypeBool,
			Rw:      1, // 读写
			Default: nson.Bool(false),
			Tags:    "cluster:OnOff",
		},
	},
}

// LevelControl Cluster - 亮度/级别控制
var LevelControlCluster = Cluster{
	ID:          0x0008,
	Name:        "LevelControl",
	Description: "亮度/级别控制",
	Pins: []PinTemplate{
		{
			Name:    "level",
			Desc:    "当前级别 (0-254)",
			Type:    dt.TypeU32,
			Rw:      1,
			Default: nson.U32(0),
			Tags:    "cluster:LevelControl",
		},
		{
			Name:    "min_level",
			Desc:    "最小级别",
			Type:    dt.TypeU32,
			Rw:      0, // 只读
			Default: nson.U32(0),
			Tags:    "cluster:LevelControl",
		},
		{
			Name:    "max_level",
			Desc:    "最大级别",
			Type:    dt.TypeU32,
			Rw:      0,
			Default: nson.U32(254),
			Tags:    "cluster:LevelControl",
		},
	},
}

// ColorControl Cluster - 颜色控制
var ColorControlCluster = Cluster{
	ID:          0x0300,
	Name:        "ColorControl",
	Description: "颜色控制 (HSV)",
	Pins: []PinTemplate{
		{
			Name:    "hue",
			Desc:    "色相 (0-254)",
			Type:    dt.TypeU32,
			Rw:      1,
			Default: nson.U32(0),
			Tags:    "cluster:ColorControl",
		},
		{
			Name:    "saturation",
			Desc:    "饱和度 (0-254)",
			Type:    dt.TypeU32,
			Rw:      1,
			Default: nson.U32(0),
			Tags:    "cluster:ColorControl",
		},
	},
}

// TemperatureMeasurement Cluster - 温度测量
var TemperatureMeasurementCluster = Cluster{
	ID:          0x0402,
	Name:        "TemperatureMeasurement",
	Description: "温度测量 (单位: 0.01°C)",
	Pins: []PinTemplate{
		{
			Name:    "temperature",
			Desc:    "当前温度 (0.01°C)",
			Type:    dt.TypeI32,
			Rw:      0,              // 只读，传感器数据
			Default: nson.I32(2500), // 25.00°C
			Tags:    "cluster:TemperatureMeasurement",
		},
		{
			Name:    "temp_min",
			Desc:    "最小可测温度",
			Type:    dt.TypeI32,
			Rw:      0,
			Default: nson.I32(-4000), // -40°C
			Tags:    "cluster:TemperatureMeasurement",
		},
		{
			Name:    "temp_max",
			Desc:    "最大可测温度",
			Type:    dt.TypeI32,
			Rw:      0,
			Default: nson.I32(12500), // 125°C
			Tags:    "cluster:TemperatureMeasurement",
		},
	},
}

// HumidityMeasurement Cluster - 湿度测量
var HumidityMeasurementCluster = Cluster{
	ID:          0x0405,
	Name:        "HumidityMeasurement",
	Description: "湿度测量 (单位: 0.01%)",
	Pins: []PinTemplate{
		{
			Name:    "humidity",
			Desc:    "当前湿度 (0.01%)",
			Type:    dt.TypeU32,
			Rw:      0,
			Default: nson.U32(5000), // 50.00%
			Tags:    "cluster:HumidityMeasurement",
		},
	},
}

// BasicInformation Cluster - 基本信息
var BasicInformationCluster = Cluster{
	ID:          0x0028,
	Name:        "BasicInformation",
	Description: "设备基本信息",
	Pins: []PinTemplate{
		{
			Name:    "vendor_name",
			Desc:    "厂商名称",
			Type:    dt.TypeString,
			Rw:      0,
			Default: nson.String(""),
			Tags:    "cluster:BasicInformation",
		},
		{
			Name:    "product_name",
			Desc:    "产品名称",
			Type:    dt.TypeString,
			Rw:      0,
			Default: nson.String(""),
			Tags:    "cluster:BasicInformation",
		},
		{
			Name:    "serial_number",
			Desc:    "序列号",
			Type:    dt.TypeString,
			Rw:      0,
			Default: nson.String(""),
			Tags:    "cluster:BasicInformation",
		},
	},
}

// ============================================================================
// Cluster 注册表
// ============================================================================

// ClusterRegistry Cluster 注册表
var ClusterRegistry = map[string]*Cluster{
	"OnOff":                  &OnOffCluster,
	"LevelControl":           &LevelControlCluster,
	"ColorControl":           &ColorControlCluster,
	"TemperatureMeasurement": &TemperatureMeasurementCluster,
	"HumidityMeasurement":    &HumidityMeasurementCluster,
	"BasicInformation":       &BasicInformationCluster,
}

// GetCluster 根据名称获取 Cluster
func GetCluster(name string) *Cluster {
	return ClusterRegistry[name]
}

// RegisterCluster 注册自定义 Cluster
func RegisterCluster(cluster *Cluster) {
	ClusterRegistry[cluster.Name] = cluster
}
