package device

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewDeviceBuilder 测试创建设备构建器
func TestNewDeviceBuilder(t *testing.T) {
	// 测试有效的设备 ID
	builder, err := NewDeviceBuilder("smart_bulb_color", "客厅彩灯")
	require.NoError(t, err)
	assert.NotNil(t, builder)
	assert.Equal(t, "smart_bulb_color", builder.deviceID)
	assert.Equal(t, "客厅彩灯", builder.instanceName)

	// 测试无效的设备 ID
	_, err = NewDeviceBuilder("non_existent_device", "test")
	assert.Error(t, err)
}

// TestDeviceBuilder_Build 测试构建设备实例
func TestDeviceBuilder_Build(t *testing.T) {
	builder, err := NewDeviceBuilder("smart_bulb_onoff", "卧室灯")
	require.NoError(t, err)

	instance, err := builder.Build()
	require.NoError(t, err)
	assert.NotNil(t, instance)
	assert.Equal(t, "smart_bulb_onoff", instance.DeviceID)
	assert.Equal(t, "卧室灯", instance.InstanceName)

	// 验证 Wire 数量（至少有 root）
	assert.GreaterOrEqual(t, len(instance.Wires), 1)

	// 验证 root wire
	var rootWire *WireInstance
	for _, wire := range instance.Wires {
		if wire.Name == "root" {
			rootWire = wire
			break
		}
	}
	require.NotNil(t, rootWire)
	assert.Contains(t, rootWire.Clusters, "BasicInformation")
}

// TestDeviceBuilder_SetPinAddress 测试设置 Pin 地址
func TestDeviceBuilder_SetPinAddress(t *testing.T) {
	builder, err := NewDeviceBuilder("smart_bulb_dimmable", "客厅灯")
	require.NoError(t, err)

	// 设置 Pin 地址
	builder.SetPinAddress("light", "onoff", "GPIO_1")
	builder.SetPinAddress("light", "level", "PWM_1")

	instance, err := builder.Build()
	require.NoError(t, err)

	// 查找 light wire
	var lightWire *WireInstance
	for _, wire := range instance.Wires {
		if wire.Name == "light" {
			lightWire = wire
			break
		}
	}
	require.NotNil(t, lightWire)

	// 验证 Pin 地址
	for _, pin := range lightWire.Pins {
		if pin.Name == "onoff" {
			assert.Equal(t, "GPIO_1", pin.Addr)
		} else if pin.Name == "level" {
			assert.Equal(t, "PWM_1", pin.Addr)
		}
	}
}

// TestDeviceBuilder_SetPinAddresses 测试批量设置地址
func TestDeviceBuilder_SetPinAddresses(t *testing.T) {
	builder, err := NewDeviceBuilder("smart_bulb_color", "客厅彩灯")
	require.NoError(t, err)

	// 批量设置地址
	builder.SetPinAddresses("light", map[string]string{
		"onoff":      "GPIO_1",
		"level":      "PWM_1",
		"hue":        "PWM_2",
		"saturation": "PWM_3",
	})

	instance, err := builder.Build()
	require.NoError(t, err)

	// 查找 light wire
	var lightWire *WireInstance
	for _, wire := range instance.Wires {
		if wire.Name == "light" {
			lightWire = wire
			break
		}
	}
	require.NotNil(t, lightWire)

	// 验证所有 Pin 都有地址
	pinAddrs := make(map[string]string)
	for _, pin := range lightWire.Pins {
		pinAddrs[pin.Name] = pin.Addr
	}

	assert.Equal(t, "GPIO_1", pinAddrs["onoff"])
	assert.Equal(t, "PWM_1", pinAddrs["level"])
	assert.Equal(t, "PWM_2", pinAddrs["hue"])
	assert.Equal(t, "PWM_3", pinAddrs["saturation"])
}

// TestDeviceBuilder_EnableDisableWire 测试启用/禁用 Wire
func TestDeviceBuilder_EnableDisableWire(t *testing.T) {
	// 注册一个有可选 Wire 的测试设备
	testDevice := &DeviceTemplate{
		ID:       "test_device_optional",
		Name:     "测试设备",
		Category: CategoryCustom,
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "optional_wire", Clusters: []string{"OnOff"}, Required: false},
		},
	}
	RegisterDevice(testDevice)

	builder, err := NewDeviceBuilder("test_device_optional", "测试实例")
	require.NoError(t, err)

	// 默认情况下，可选 Wire 不启用
	instance, err := builder.Build()
	require.NoError(t, err)

	hasOptional := false
	for _, wire := range instance.Wires {
		if wire.Name == "optional_wire" {
			hasOptional = true
			break
		}
	}
	assert.False(t, hasOptional, "Optional wire should not be enabled by default")

	// 启用可选 Wire
	builder.EnableWire("optional_wire")
	instance, err = builder.Build()
	require.NoError(t, err)

	hasOptional = false
	for _, wire := range instance.Wires {
		if wire.Name == "optional_wire" {
			hasOptional = true
			break
		}
	}
	assert.True(t, hasOptional, "Optional wire should be enabled after EnableWire()")
}

// TestQuickBuildDevice 测试快速构建设备
func TestQuickBuildDevice(t *testing.T) {
	instance, err := QuickBuildDevice("smart_socket", "客厅插座")
	require.NoError(t, err)
	assert.NotNil(t, instance)
	assert.Equal(t, "smart_socket", instance.DeviceID)
	assert.Equal(t, "客厅插座", instance.InstanceName)
}

// TestBuildDeviceWithAddresses 测试带地址构建
func TestBuildDeviceWithAddresses(t *testing.T) {
	addresses := map[string]map[string]string{
		"light": {
			"onoff": "GPIO_1",
			"level": "PWM_1",
		},
	}

	instance, err := BuildDeviceWithAddresses("smart_bulb_dimmable", "卧室灯", addresses)
	require.NoError(t, err)
	assert.NotNil(t, instance)

	// 验证地址设置正确
	for _, wire := range instance.Wires {
		if wire.Name == "light" {
			for _, pin := range wire.Pins {
				if pin.Name == "onoff" {
					assert.Equal(t, "GPIO_1", pin.Addr)
				} else if pin.Name == "level" {
					assert.Equal(t, "PWM_1", pin.Addr)
				}
			}
		}
	}
}

// TestDeviceInstance_MultiWire 测试多 Wire 设备
func TestDeviceInstance_MultiWire(t *testing.T) {
	builder, err := NewDeviceBuilder("switch_2gang", "客厅双开")
	require.NoError(t, err)

	// 设置两个开关的地址
	builder.SetPinAddress("switch1", "onoff", "GPIO_1")
	builder.SetPinAddress("switch2", "onoff", "GPIO_2")

	instance, err := builder.Build()
	require.NoError(t, err)

	// 验证有 3 个 Wire: root, switch1, switch2
	assert.GreaterOrEqual(t, len(instance.Wires), 3)

	// 验证 switch1 和 switch2
	switchCount := 0
	for _, wire := range instance.Wires {
		if wire.Name == "switch1" || wire.Name == "switch2" {
			switchCount++
			assert.Contains(t, wire.Clusters, "OnOff")
			assert.GreaterOrEqual(t, len(wire.Pins), 1)
		}
	}
	assert.Equal(t, 2, switchCount, "Should have 2 switch wires")
}

// TestDeviceInstance_ComplexDevice 测试复杂设备
func TestDeviceInstance_ComplexDevice(t *testing.T) {
	builder, err := NewDeviceBuilder("smart_bulb_color", "客厅彩灯")
	require.NoError(t, err)

	// 设置所有 Pin 的地址
	addresses := map[string]string{
		"onoff":      "GPIO_1",
		"level":      "PWM_1",
		"hue":        "PWM_2",
		"saturation": "PWM_3",
		"min_level":  "", // 只读 Pin 可能不需要物理地址
		"max_level":  "",
	}
	builder.SetPinAddresses("light", addresses)

	instance, err := builder.Build()
	require.NoError(t, err)

	// 验证 light wire
	var lightWire *WireInstance
	for _, wire := range instance.Wires {
		if wire.Name == "light" {
			lightWire = wire
			break
		}
	}
	require.NotNil(t, lightWire)

	// 验证包含所有必要的 Cluster
	assert.Contains(t, lightWire.Clusters, "OnOff")
	assert.Contains(t, lightWire.Clusters, "LevelControl")
	assert.Contains(t, lightWire.Clusters, "ColorControl")

	// 验证 Pin 数量（OnOff:1 + LevelControl:3 + ColorControl:2 = 6）
	assert.GreaterOrEqual(t, len(lightWire.Pins), 4)

	// 验证 Pin 类型和权限
	for _, pin := range lightWire.Pins {
		assert.NotZero(t, pin.Type, "Pin type should be set")
		if pin.Name == "onoff" || pin.Name == "level" || pin.Name == "hue" || pin.Name == "saturation" {
			assert.Equal(t, int32(1), pin.Rw, "Should be read-write")
		}
	}
}

// TestDeviceInstance_Sensor 测试传感器设备
func TestDeviceInstance_Sensor(t *testing.T) {
	instance, err := QuickBuildDevice("temp_humi_sensor", "客厅温湿度")
	require.NoError(t, err)

	// 查找 sensor wire
	var sensorWire *WireInstance
	for _, wire := range instance.Wires {
		if wire.Name == "sensor" {
			sensorWire = wire
			break
		}
	}
	require.NotNil(t, sensorWire)

	// 验证包含温湿度 Cluster
	assert.Contains(t, sensorWire.Clusters, "TemperatureMeasurement")
	assert.Contains(t, sensorWire.Clusters, "HumidityMeasurement")

	// 验证所有传感器 Pin 都是只读的
	for _, pin := range sensorWire.Pins {
		assert.Equal(t, int32(0), pin.Rw, "Sensor pins should be read-only")
	}
}
