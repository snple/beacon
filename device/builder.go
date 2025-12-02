// Package device 提供 Wire 构建器适配器
//
// 此文件为旧版 SimpleBuildResult API 提供适配器支持
// 新代码应使用 DeviceBuilder API (device_builder.go)
//
// 迁移指南:
//
//	旧: result := device.BuildRootWire()
//	新: instance, _ := device.QuickBuildDevice("smart_gateway", "root")
package device

import (
	"strings"
)

// ============================================================================
// 简化的 Wire/Pin 结构（兼容旧版 API）
// ============================================================================

// BuilderWire Wire 构建器结果（兼容层）
type BuilderWire struct {
	Name     string
	Type     string
	Clusters string
}

// BuilderPin Pin 构建器结果（兼容层）
type BuilderPin struct {
	Name string
	Type uint32 // nson.DataType
	Addr string
	Rw   int32
}

// SimpleBuildResult 简化的构建结果（兼容层）
type SimpleBuildResult struct {
	Wire *BuilderWire
	Pins []*BuilderPin
}

// ============================================================================
// WireBuilder - Wire 构建器（兼容层）
// ============================================================================

// WireBuilder Wire 构建器（兼容旧版 API）
// 新代码应使用 DeviceBuilder
type WireBuilder struct {
	name         string
	clusterNames []string
}

// NewWireBuilder 创建 Wire 构建器
//
// 已废弃: 新代码应使用 device.NewDeviceBuilder()
func NewWireBuilder(name string) *WireBuilder {
	return &WireBuilder{
		name:         name,
		clusterNames: make([]string, 0),
	}
}

// WithCluster 添加 Cluster（通过名称）
func (b *WireBuilder) WithCluster(clusterName string) *WireBuilder {
	if GetCluster(clusterName) != nil {
		b.clusterNames = append(b.clusterNames, clusterName)
	}
	return b
}

// WithClusters 添加多个 Cluster
func (b *WireBuilder) WithClusters(clusterNames ...string) *WireBuilder {
	for _, name := range clusterNames {
		b.WithCluster(name)
	}
	return b
}

// WithCustomCluster 添加自定义 Cluster
func (b *WireBuilder) WithCustomCluster(cluster *Cluster) *WireBuilder {
	RegisterCluster(cluster)
	b.clusterNames = append(b.clusterNames, cluster.Name)
	return b
}

// Build 构建 Wire 和 Pin
func (b *WireBuilder) Build() *SimpleBuildResult {
	result := &SimpleBuildResult{
		Wire: &BuilderWire{
			Name:     b.name,
			Clusters: strings.Join(b.clusterNames, ","),
		},
		Pins: make([]*BuilderPin, 0),
	}

	// 从 Cluster 收集 Pin
	for _, clusterName := range b.clusterNames {
		cluster := GetCluster(clusterName)
		if cluster == nil {
			continue
		}

		for _, pinTpl := range cluster.Pins {
			pin := &BuilderPin{
				Name: pinTpl.Name,
				Type: pinTpl.Type,
				Rw:   pinTpl.Rw,
				Addr: "", // 地址由用户配置
			}
			result.Pins = append(result.Pins, pin)
		}
	}

	return result
}

// ============================================================================
// 快捷构建函数（兼容层）
// ============================================================================

// BuildRootWire 构建根 Wire
//
// 已废弃: 新代码推荐使用:
//
//	instance, _ := device.QuickBuildDevice("smart_gateway", "root")
func BuildRootWire() *SimpleBuildResult {
	return NewWireBuilder("root").
		WithClusters("BasicInformation").
		Build()
}

// BuildOnOffLightWire 构建开关灯 Wire
//
// 已废弃: 新代码推荐使用:
//
//	instance, _ := device.QuickBuildDevice("smart_bulb_onoff", name)
func BuildOnOffLightWire(name string) *SimpleBuildResult {
	return NewWireBuilder(name).
		WithClusters("OnOff").
		Build()
}

// BuildDimmableLightWire 构建可调光灯 Wire
//
// 已废弃: 新代码推荐使用:
//
//	instance, _ := device.QuickBuildDevice("smart_bulb_dimmable", name)
func BuildDimmableLightWire(name string) *SimpleBuildResult {
	return NewWireBuilder(name).
		WithClusters("OnOff", "LevelControl").
		Build()
}

// BuildColorLightWire 构建彩色灯 Wire
//
// 已废弃: 新代码推荐使用:
//
//	instance, _ := device.QuickBuildDevice("smart_bulb_color", name)
func BuildColorLightWire(name string) *SimpleBuildResult {
	return NewWireBuilder(name).
		WithClusters("OnOff", "LevelControl", "ColorControl").
		Build()
}

// BuildTemperatureSensorWire 构建温度传感器 Wire
//
// 已废弃: 新代码推荐使用:
//
//	instance, _ := device.QuickBuildDevice("temperature_sensor", name)
func BuildTemperatureSensorWire(name string) *SimpleBuildResult {
	return NewWireBuilder(name).
		WithClusters("TemperatureMeasurement").
		Build()
}

// BuildTempHumiSensorWire 构建温湿度传感器 Wire
//
// 已废弃: 新代码推荐使用:
//
//	instance, _ := device.QuickBuildDevice("temp_humi_sensor", name)
func BuildTempHumiSensorWire(name string) *SimpleBuildResult {
	return NewWireBuilder(name).
		WithClusters("TemperatureMeasurement", "HumidityMeasurement").
		Build()
}
