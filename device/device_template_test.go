package device

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeviceRegistry_Register 测试设备注册
func TestDeviceRegistry_Register(t *testing.T) {
	registry := &DeviceRegistry{
		templates:  make(map[string]*DeviceTemplate),
		byCategory: make(map[string][]*DeviceTemplate),
	}

	template := &DeviceTemplate{
		ID:       "test_device",
		Name:     "测试设备",
		Category: CategoryLighting,
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
		},
	}

	err := registry.Register(template)
	require.NoError(t, err)

	// 验证可以获取
	retrieved := registry.Get("test_device")
	require.NotNil(t, retrieved)
	assert.Equal(t, "test_device", retrieved.ID)
	assert.Equal(t, "测试设备", retrieved.Name)

	// 测试重复注册
	err = registry.Register(template)
	assert.Error(t, err)
}

// TestDeviceRegistry_ListByCategory 测试按类别列出设备
func TestDeviceRegistry_ListByCategory(t *testing.T) {
	registry := &DeviceRegistry{
		templates:  make(map[string]*DeviceTemplate),
		byCategory: make(map[string][]*DeviceTemplate),
	}

	// 注册多个设备
	devices := []*DeviceTemplate{
		{ID: "light1", Name: "灯1", Category: CategoryLighting, Wires: []WireTemplate{}},
		{ID: "light2", Name: "灯2", Category: CategoryLighting, Wires: []WireTemplate{}},
		{ID: "sensor1", Name: "传感器1", Category: CategorySensor, Wires: []WireTemplate{}},
	}

	for _, dev := range devices {
		err := registry.Register(dev)
		require.NoError(t, err)
	}

	// 获取照明类别
	lights := registry.ListByCategory(CategoryLighting)
	assert.Equal(t, 2, len(lights))

	// 获取传感器类别
	sensors := registry.ListByCategory(CategorySensor)
	assert.Equal(t, 1, len(sensors))
}

// TestStandardDevices 测试标准设备是否正确注册
func TestStandardDevices(t *testing.T) {
	// 验证常见设备已注册
	testCases := []struct {
		deviceID string
		category string
	}{
		{"smart_bulb_onoff", CategoryLighting},
		{"smart_bulb_dimmable", CategoryLighting},
		{"smart_bulb_color", CategoryLighting},
		{"temp_humi_sensor", CategorySensor},
		{"switch_1gang", CategorySwitch},
		{"switch_2gang", CategorySwitch},
		{"smart_socket", CategorySocket},
		{"door_window_sensor", CategorySecuritySensor},
		{"motion_sensor", CategorySecuritySensor},
		{"smart_curtain", CategoryCurtain},
		{"smart_gateway", CategoryGateway},
	}

	for _, tc := range testCases {
		t.Run(tc.deviceID, func(t *testing.T) {
			device := GetDevice(tc.deviceID)
			require.NotNil(t, device, "Device %s should be registered", tc.deviceID)
			assert.Equal(t, tc.deviceID, device.ID)
			assert.Equal(t, tc.category, device.Category)
			assert.NotEmpty(t, device.Name)
			assert.NotEmpty(t, device.Wires)

			// 验证必需的 root wire
			hasRoot := false
			for _, wire := range device.Wires {
				if wire.Name == "root" && wire.Required {
					hasRoot = true
					break
				}
			}
			assert.True(t, hasRoot, "Device should have required root wire")
		})
	}
}

// TestListDevices 测试列出所有设备
func TestListDevices(t *testing.T) {
	devices := ListDevices()
	assert.Greater(t, len(devices), 20, "Should have at least 20 standard devices")

	// 验证每个设备都有必需字段
	for _, device := range devices {
		assert.NotEmpty(t, device.ID)
		assert.NotEmpty(t, device.Name)
		assert.NotEmpty(t, device.Category)
		assert.NotEmpty(t, device.Wires)
	}
}

// TestGetCategories 测试获取所有类别
func TestGetCategories(t *testing.T) {
	categories := GetCategories()
	assert.Greater(t, len(categories), 5, "Should have multiple categories")

	// 验证包含主要类别
	categoryMap := make(map[string]bool)
	for _, cat := range categories {
		categoryMap[cat] = true
	}

	expectedCategories := []string{
		CategoryLighting,
		CategorySensor,
		CategorySwitch,
		CategorySecuritySensor,
	}

	for _, expected := range expectedCategories {
		assert.True(t, categoryMap[expected], "Should contain category %s", expected)
	}
}

// TestListDevicesByCategory 测试按类别列出设备
func TestListDevicesByCategory(t *testing.T) {
	// 测试照明设备
	lightDevices := ListDevicesByCategory(CategoryLighting)
	assert.Greater(t, len(lightDevices), 0)
	for _, device := range lightDevices {
		assert.Equal(t, CategoryLighting, device.Category)
	}

	// 测试传感器设备
	sensorDevices := ListDevicesByCategory(CategorySensor)
	assert.Greater(t, len(sensorDevices), 0)
	for _, device := range sensorDevices {
		assert.Equal(t, CategorySensor, device.Category)
	}
}

// TestRegisterCustomDevice 测试注册自定义设备
func TestRegisterCustomDevice(t *testing.T) {
	customDevice := &DeviceTemplate{
		ID:           "custom_test_device",
		Name:         "自定义测试设备",
		Category:     CategoryCustom,
		Description:  "用于测试的自定义设备",
		Manufacturer: "TestCompany",
		Model:        "TEST-001",
		Version:      "1.0.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "custom_wire", Clusters: []string{"OnOff"}, Required: false},
		},
	}

	err := RegisterDevice(customDevice)
	require.NoError(t, err)

	// 验证可以获取
	retrieved := GetDevice("custom_test_device")
	require.NotNil(t, retrieved)
	assert.Equal(t, "custom_test_device", retrieved.ID)
	assert.Equal(t, "自定义测试设备", retrieved.Name)
	assert.Equal(t, CategoryCustom, retrieved.Category)
}

// TestDeviceTemplate_WireValidation 测试设备模板的 Wire 验证
func TestDeviceTemplate_WireValidation(t *testing.T) {
	device := GetDevice("smart_bulb_color")
	require.NotNil(t, device)

	// 验证 Wire 结构
	assert.GreaterOrEqual(t, len(device.Wires), 2) // 至少有 root 和 light

	// 验证 root wire
	var rootWire *WireTemplate
	for _, wire := range device.Wires {
		if wire.Name == "root" {
			rootWire = &wire
			break
		}
	}
	require.NotNil(t, rootWire)
	assert.True(t, rootWire.Required)
	assert.Contains(t, rootWire.Clusters, "BasicInformation")

	// 验证 light wire
	var lightWire *WireTemplate
	for _, wire := range device.Wires {
		if wire.Name == "light" {
			lightWire = &wire
			break
		}
	}
	require.NotNil(t, lightWire)
	assert.Contains(t, lightWire.Clusters, "OnOff")
	assert.Contains(t, lightWire.Clusters, "LevelControl")
	assert.Contains(t, lightWire.Clusters, "ColorControl")
}
