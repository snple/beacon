package device

import (
	"testing"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/dt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCluster_GetPinTemplates 测试获取所有 Pin 模板
func TestCluster_GetPinTemplates(t *testing.T) {
	cluster := Cluster{
		ID:          1,
		Name:        "TestCluster",
		Description: "Test cluster",
		Pins: []PinTemplate{
			{Name: "pin1", Type: dt.TypeBool, Rw: 0},
			{Name: "pin2", Type: dt.TypeI32, Rw: 1},
		},
	}

	templates := cluster.GetPinTemplates()
	assert.Equal(t, 2, len(templates))
	assert.Equal(t, "pin1", templates[0].Name)
	assert.Equal(t, "pin2", templates[1].Name)
}

// TestCluster_GetPinTemplate 测试按名称获取 Pin 模板
func TestCluster_GetPinTemplate(t *testing.T) {
	cluster := Cluster{
		ID:   1,
		Name: "TestCluster",
		Pins: []PinTemplate{
			{Name: "pin1", Type: dt.TypeBool, Rw: 0},
			{Name: "pin2", Type: dt.TypeI32, Rw: 1},
		},
	}

	// 获取存在的 Pin
	template := cluster.GetPinTemplate("pin1")
	require.NotNil(t, template)
	assert.Equal(t, "pin1", template.Name)
	assert.Equal(t, dt.TypeBool, template.Type)

	// 获取不存在的 Pin
	template = cluster.GetPinTemplate("non-existent")
	assert.Nil(t, template)
}

// TestStandardClusters 测试标准 Cluster 定义
func TestStandardClusters(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *Cluster
		expectedID  ClusterID
		minPinCount int
	}{
		{"OnOff", &OnOffCluster, 0x0006, 1},
		{"LevelControl", &LevelControlCluster, 0x0008, 1},
		{"ColorControl", &ColorControlCluster, 0x0300, 1},
		{"TemperatureMeasurement", &TemperatureMeasurementCluster, 0x0402, 1},
		{"HumidityMeasurement", &HumidityMeasurementCluster, 0x0405, 1},
		{"BasicInformation", &BasicInformationCluster, 0x0028, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedID, tt.cluster.ID)
			assert.NotEmpty(t, tt.cluster.Name)
			assert.NotEmpty(t, tt.cluster.Description)
			assert.GreaterOrEqual(t, len(tt.cluster.Pins), tt.minPinCount,
				"Cluster should have at least %d pins", tt.minPinCount)

			// 验证每个 Pin 模板的完整性
			for _, pin := range tt.cluster.Pins {
				assert.NotEmpty(t, pin.Name, "Pin name should not be empty")
				assert.NotEmpty(t, pin.Desc, "Pin description should not be empty")
				assert.NotZero(t, pin.Type, "Pin type should be set")
			}
		})
	}
}

// TestOnOffCluster 测试 OnOff Cluster
func TestOnOffCluster(t *testing.T) {
	assert.Equal(t, ClusterID(0x0006), OnOffCluster.ID)
	assert.Equal(t, "OnOff", OnOffCluster.Name)

	onoffPin := OnOffCluster.GetPinTemplate("onoff")
	require.NotNil(t, onoffPin)
	assert.Equal(t, "onoff", onoffPin.Name)
	assert.Equal(t, dt.TypeBool, onoffPin.Type)
}

// TestLevelControlCluster 测试 LevelControl Cluster
func TestLevelControlCluster(t *testing.T) {
	assert.Equal(t, ClusterID(0x0008), LevelControlCluster.ID)
	assert.Equal(t, "LevelControl", LevelControlCluster.Name)

	levelPin := LevelControlCluster.GetPinTemplate("level")
	require.NotNil(t, levelPin)
	assert.Equal(t, "level", levelPin.Name)
	// 实际类型是 U32
	assert.Equal(t, dt.TypeU32, levelPin.Type)
}

// TestColorControlCluster 测试 ColorControl Cluster
func TestColorControlCluster(t *testing.T) {
	assert.Equal(t, ClusterID(0x0300), ColorControlCluster.ID)
	assert.Equal(t, "ColorControl", ColorControlCluster.Name)

	pins := ColorControlCluster.GetPinTemplates()
	// ColorControl cluster 应该包含多个颜色相关的 pins
	assert.GreaterOrEqual(t, len(pins), 1, "Should have at least 1 color-related pin")
}

// TestTemperatureMeasurementCluster 测试温度测量 Cluster
func TestTemperatureMeasurementCluster(t *testing.T) {
	assert.Equal(t, ClusterID(0x0402), TemperatureMeasurementCluster.ID)
	assert.Equal(t, "TemperatureMeasurement", TemperatureMeasurementCluster.Name)

	tempPin := TemperatureMeasurementCluster.GetPinTemplate("temperature")
	require.NotNil(t, tempPin)
	assert.Equal(t, "temperature", tempPin.Name)
}

// TestGetCluster 测试按名称获取 Cluster
func TestGetCluster(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		expectFound bool
	}{
		{"GetOnOff", "OnOff", true},
		{"GetLevelControl", "LevelControl", true},
		{"GetColorControl", "ColorControl", true},
		{"GetTemperatureMeasurement", "TemperatureMeasurement", true},
		{"GetNonExistent", "NonExistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := GetCluster(tt.clusterName)
			if tt.expectFound {
				require.NotNil(t, cluster, "Expected to find cluster %s", tt.clusterName)
				assert.Equal(t, tt.clusterName, cluster.Name)
			} else {
				assert.Nil(t, cluster, "Should not find cluster %s", tt.clusterName)
			}
		})
	}
}

// TestRegisterCluster 测试注册新 Cluster
func TestRegisterCluster(t *testing.T) {
	customCluster := &Cluster{
		ID:          0x8888,
		Name:        "CustomTestCluster",
		Description: "Custom test cluster",
		Pins: []PinTemplate{
			{
				Name: "custom_pin",
				Desc: "Custom pin",
				Type: dt.TypeString,
				Rw:   1,
			},
		},
	}

	// 注册 Cluster
	RegisterCluster(customCluster)

	// 验证可以通过名称获取
	retrieved := GetCluster("CustomTestCluster")
	require.NotNil(t, retrieved)
	assert.Equal(t, "CustomTestCluster", retrieved.Name)
	assert.Equal(t, ClusterID(0x8888), retrieved.ID)
}

// TestPinTemplate_DefaultValue 测试 Pin 模板的默认值
func TestPinTemplate_DefaultValue(t *testing.T) {
	tests := []struct {
		name         string
		template     PinTemplate
		checkDefault func(*testing.T, nson.Value)
	}{
		{
			name: "BoolDefault",
			template: PinTemplate{
				Name:    "test_bool",
				Type:    dt.TypeBool,
				Default: nson.Bool(false),
			},
			checkDefault: func(t *testing.T, val nson.Value) {
				b, ok := val.(nson.Bool)
				require.True(t, ok)
				assert.False(t, bool(b))
			},
		},
		{
			name: "I32Default",
			template: PinTemplate{
				Name:    "test_i32",
				Type:    dt.TypeI32,
				Default: nson.I32(100),
			},
			checkDefault: func(t *testing.T, val nson.Value) {
				i, ok := val.(nson.I32)
				require.True(t, ok)
				assert.Equal(t, int32(100), int32(i))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.template.Default)
			tt.checkDefault(t, tt.template.Default)
		})
	}
}

// TestClusterRegistry 测试 Cluster 注册表
func TestClusterRegistry(t *testing.T) {
	// 验证注册表不为空
	assert.NotEmpty(t, ClusterRegistry)

	// 验证包含预期的标准 Cluster
	expectedClusters := []string{
		"OnOff",
		"LevelControl",
		"ColorControl",
		"TemperatureMeasurement",
		"HumidityMeasurement",
		"BasicInformation",
	}

	for _, name := range expectedClusters {
		cluster, ok := ClusterRegistry[name]
		assert.True(t, ok, "Should contain %s cluster", name)
		assert.NotNil(t, cluster)
		assert.Equal(t, name, cluster.Name)
	}
}

// TestCluster_PinNamesUnique 测试 Cluster 中 Pin 名称的唯一性
func TestCluster_PinNamesUnique(t *testing.T) {
	for name, cluster := range ClusterRegistry {
		t.Run(name, func(t *testing.T) {
			names := make(map[string]bool)
			for _, pin := range cluster.Pins {
				assert.False(t, names[pin.Name],
					"Duplicate pin name '%s' in cluster '%s'", pin.Name, name)
				names[pin.Name] = true
			}
		})
	}
}
