// Package device 提供 Wire 构建器，用于从 Cluster 模板创建 Wire 和 Pin
package device

import (
	"strings"
)

// ============================================================================
// 简化的 Wire/Pin 结构（本地定义，避免循环依赖）
// ============================================================================

// BuilderWire Wire 构建器结果
type BuilderWire struct {
	Name     string
	Type     string
	Clusters string
}

// BuilderPin Pin 构建器结果
type BuilderPin struct {
	Name string
	Type string
	Addr string
	Rw   int32
}

// SimpleBuildResult 简化的构建结果
type SimpleBuildResult struct {
	Wire *BuilderWire
	Pins []*BuilderPin
}

// ============================================================================
// WireBuilder - 从 Cluster 构建 Wire 及其 Pin（简化版）
// ============================================================================

// WireBuilder Wire 构建器
type WireBuilder struct {
	name         string
	clusterNames []string
	clusters     []*Cluster
}

// NewWireBuilder 创建 Wire 构建器
func NewWireBuilder(name string) *WireBuilder {
	return &WireBuilder{
		name:         name,
		clusterNames: make([]string, 0),
		clusters:     make([]*Cluster, 0),
	}
}

// WithCluster 添加 Cluster（通过名称）
func (b *WireBuilder) WithCluster(clusterName string) *WireBuilder {
	if cluster := GetCluster(clusterName); cluster != nil {
		b.clusterNames = append(b.clusterNames, clusterName)
		b.clusters = append(b.clusters, cluster)
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
	b.clusterNames = append(b.clusterNames, cluster.Name)
	b.clusters = append(b.clusters, cluster)
	return b
}

// Build 构建 Wire 和 Pin（简化版，不包含冗余字段）
func (b *WireBuilder) Build() *SimpleBuildResult {
	result := &SimpleBuildResult{
		Wire: &BuilderWire{
			Name:     b.name,
			Clusters: strings.Join(b.clusterNames, ","), // 存储 Cluster 列表
		},
		Pins: make([]*BuilderPin, 0),
	}

	// 收集所有 Cluster 的 Pin（只存储名称和 Rw，其他信息从注册表查）
	for _, cluster := range b.clusters {
		for _, pinTpl := range cluster.Pins {
			pin := &BuilderPin{
				Name: pinTpl.Name,
				Type: pinTpl.Type,
				Rw:   pinTpl.Rw, // 保留 Rw 字段
				// Addr 字段由用户在部署时设置
			}
			result.Pins = append(result.Pins, pin)
		}
	}

	return result
}

// ============================================================================
// 预定义的 Wire 模板（常见设备类型）
// ============================================================================

// BuildRootWire 构建根 Wire（包含设备基本信息）
// 每个 Node 必须有一个根 Wire
func BuildRootWire() *SimpleBuildResult {
	return NewWireBuilder("root").
		WithClusters("BasicInformation").
		Build()
}

// BuildOnOffLightWire 构建开关灯 Wire
func BuildOnOffLightWire(name string) *SimpleBuildResult {
	return NewWireBuilder(name).
		WithClusters("OnOff").
		Build()
}

// BuildDimmableLightWire 构建可调光灯 Wire
func BuildDimmableLightWire(name string) *SimpleBuildResult {
	return NewWireBuilder(name).
		WithClusters("OnOff", "LevelControl").
		Build()
}

// BuildColorLightWire 构建彩色灯 Wire
func BuildColorLightWire(name string) *SimpleBuildResult {
	return NewWireBuilder(name).
		WithClusters("OnOff", "LevelControl", "ColorControl").
		Build()
}

// BuildTemperatureSensorWire 构建温度传感器 Wire
func BuildTemperatureSensorWire(name string) *SimpleBuildResult {
	return NewWireBuilder(name).
		WithClusters("TemperatureMeasurement").
		Build()
}

// BuildTempHumiSensorWire 构建温湿度传感器 Wire
func BuildTempHumiSensorWire(name string) *SimpleBuildResult {
	return NewWireBuilder(name).
		WithClusters("TemperatureMeasurement", "HumidityMeasurement").
		Build()
}
