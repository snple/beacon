package device

import (
	"testing"

	"github.com/snple/beacon/dt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWireBuilder_Build 测试 WireBuilder 兼容层
func TestWireBuilder_Build(t *testing.T) {
	builder := NewWireBuilder("test_wire")
	builder.WithClusters("OnOff", "LevelControl")

	result := builder.Build()
	require.NotNil(t, result)
	require.NotNil(t, result.Wire)

	assert.Equal(t, "test_wire", result.Wire.Name)
	assert.Equal(t, "OnOff,LevelControl", result.Wire.Clusters)

	// 验证 Pin 数量（OnOff: 1, LevelControl: 3）
	assert.GreaterOrEqual(t, len(result.Pins), 4)

	// 验证 onoff pin
	var onoffPin *BuilderPin
	for _, pin := range result.Pins {
		if pin.Name == "onoff" {
			onoffPin = pin
			break
		}
	}
	require.NotNil(t, onoffPin)
	assert.Equal(t, dt.TypeBool, onoffPin.Type)
	assert.Equal(t, int32(1), onoffPin.Rw)
}

// TestWireBuilder_ChainedCalls 测试链式调用
func TestWireBuilder_ChainedCalls(t *testing.T) {
	result := NewWireBuilder("light").
		WithCluster("OnOff").
		WithCluster("LevelControl").
		Build()

	require.NotNil(t, result)
	assert.Equal(t, "light", result.Wire.Name)
	assert.Contains(t, result.Wire.Clusters, "OnOff")
	assert.Contains(t, result.Wire.Clusters, "LevelControl")
}

// TestBuildRootWire 测试构建根 Wire
func TestBuildRootWire(t *testing.T) {
	result := BuildRootWire()
	require.NotNil(t, result)

	assert.Equal(t, "root", result.Wire.Name)
	assert.Contains(t, result.Wire.Clusters, "BasicInformation")

	// 验证包含 BasicInformation 的 Pin
	assert.GreaterOrEqual(t, len(result.Pins), 3)

	pinNames := make(map[string]bool)
	for _, pin := range result.Pins {
		pinNames[pin.Name] = true
	}

	assert.True(t, pinNames["vendor_name"])
	assert.True(t, pinNames["product_name"])
	assert.True(t, pinNames["serial_number"])
}

// TestBuildOnOffLightWire 测试构建开关灯
func TestBuildOnOffLightWire(t *testing.T) {
	result := BuildOnOffLightWire("light1")
	require.NotNil(t, result)

	assert.Equal(t, "light1", result.Wire.Name)
	assert.Contains(t, result.Wire.Clusters, "OnOff")
	assert.GreaterOrEqual(t, len(result.Pins), 1)

	// 验证 onoff pin
	var found bool
	for _, pin := range result.Pins {
		if pin.Name == "onoff" {
			found = true
			assert.Equal(t, dt.TypeBool, pin.Type)
			assert.Equal(t, int32(1), pin.Rw)
			break
		}
	}
	assert.True(t, found, "Should have onoff pin")
}

// TestBuildDimmableLightWire 测试构建调光灯
func TestBuildDimmableLightWire(t *testing.T) {
	result := BuildDimmableLightWire("light2")
	require.NotNil(t, result)

	assert.Equal(t, "light2", result.Wire.Name)
	assert.Contains(t, result.Wire.Clusters, "OnOff")
	assert.Contains(t, result.Wire.Clusters, "LevelControl")

	// 验证包含必要的 Pin
	pinNames := make(map[string]bool)
	for _, pin := range result.Pins {
		pinNames[pin.Name] = true
	}

	assert.True(t, pinNames["onoff"])
	assert.True(t, pinNames["level"])
}

// TestBuildColorLightWire 测试构建彩色灯
func TestBuildColorLightWire(t *testing.T) {
	result := BuildColorLightWire("light3")
	require.NotNil(t, result)

	assert.Equal(t, "light3", result.Wire.Name)
	assert.Contains(t, result.Wire.Clusters, "OnOff")
	assert.Contains(t, result.Wire.Clusters, "LevelControl")
	assert.Contains(t, result.Wire.Clusters, "ColorControl")

	// 验证包含彩色控制的 Pin
	pinNames := make(map[string]bool)
	for _, pin := range result.Pins {
		pinNames[pin.Name] = true
	}

	assert.True(t, pinNames["onoff"])
	assert.True(t, pinNames["level"])
	assert.True(t, pinNames["hue"])
	assert.True(t, pinNames["saturation"])
}

// TestBuildTemperatureSensorWire 测试构建温度传感器
func TestBuildTemperatureSensorWire(t *testing.T) {
	result := BuildTemperatureSensorWire("sensor1")
	require.NotNil(t, result)

	assert.Equal(t, "sensor1", result.Wire.Name)
	assert.Contains(t, result.Wire.Clusters, "TemperatureMeasurement")

	// 验证 temperature pin 是只读的
	var tempPin *BuilderPin
	for _, pin := range result.Pins {
		if pin.Name == "temperature" {
			tempPin = pin
			break
		}
	}
	require.NotNil(t, tempPin)
	assert.Equal(t, int32(0), tempPin.Rw, "Temperature pin should be read-only")
}

// TestBuildTempHumiSensorWire 测试构建温湿度传感器
func TestBuildTempHumiSensorWire(t *testing.T) {
	result := BuildTempHumiSensorWire("sensor2")
	require.NotNil(t, result)

	assert.Equal(t, "sensor2", result.Wire.Name)
	assert.Contains(t, result.Wire.Clusters, "TemperatureMeasurement")
	assert.Contains(t, result.Wire.Clusters, "HumidityMeasurement")

	// 验证包含温湿度 Pin
	pinNames := make(map[string]bool)
	for _, pin := range result.Pins {
		pinNames[pin.Name] = true
	}

	assert.True(t, pinNames["temperature"])
	assert.True(t, pinNames["humidity"])
}

// TestWireBuilder_WithCustomCluster 测试自定义 Cluster
func TestWireBuilder_WithCustomCluster(t *testing.T) {
	customCluster := &Cluster{
		ID:   0x9999,
		Name: "CustomTestCluster2",
		Pins: []PinTemplate{
			{Name: "custom1", Type: dt.TypeBool, Rw: 1},
			{Name: "custom2", Type: dt.TypeI32, Rw: 0},
		},
	}

	result := NewWireBuilder("custom_wire").
		WithCustomCluster(customCluster).
		Build()

	require.NotNil(t, result)
	assert.Contains(t, result.Wire.Clusters, "CustomTestCluster2")
	assert.Equal(t, 2, len(result.Pins))

	// 验证自定义 Cluster 已注册
	registered := GetCluster("CustomTestCluster2")
	require.NotNil(t, registered)
	assert.Equal(t, ClusterID(0x9999), registered.ID)
}

// TestSimpleBuildResult_Structure 测试 SimpleBuildResult 结构
func TestSimpleBuildResult_Structure(t *testing.T) {
	result := BuildOnOffLightWire("test")

	// 验证结构完整性
	require.NotNil(t, result)
	require.NotNil(t, result.Wire)
	require.NotNil(t, result.Pins)

	// 验证 Wire 字段
	assert.NotEmpty(t, result.Wire.Name)
	assert.NotEmpty(t, result.Wire.Clusters)

	// 验证 Pin 字段
	for _, pin := range result.Pins {
		assert.NotEmpty(t, pin.Name)
		assert.NotZero(t, pin.Type)
		// Addr 可以为空（由用户配置）
		// Rw 可以是 0 或 1
	}
}

// TestBackwardCompatibility 测试向后兼容性
func TestBackwardCompatibility(t *testing.T) {
	// 模拟 edge/seed.go 中的使用方式
	result := BuildRootWire()
	require.NotNil(t, result)

	// 验证可以像旧代码一样访问字段
	wireName := result.Wire.Name
	clusters := result.Wire.Clusters

	assert.Equal(t, "root", wireName)
	assert.NotEmpty(t, clusters)

	// 验证可以遍历 Pins
	for _, pin := range result.Pins {
		_ = pin.Name
		_ = pin.Type
		_ = pin.Addr
		_ = pin.Rw
	}
}
