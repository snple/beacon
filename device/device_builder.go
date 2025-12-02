package device

import (
	"fmt"
)

// ============================================================================
// 设备实例构建器
// ============================================================================

// DeviceInstance 设备实例（运行时）
type DeviceInstance struct {
	DeviceID     string          // 设备模板 ID
	InstanceName string          // 设备实例名称（用户指定）
	Template     *DeviceTemplate // 设备模板
	Wires        []*WireInstance // Wire 实例列表
}

// WireInstance Wire 实例
type WireInstance struct {
	Name     string         // Wire 名称
	Clusters []string       // Cluster 列表
	Pins     []*PinInstance // Pin 实例列表
}

// PinInstance Pin 实例
type PinInstance struct {
	Name string // Pin 名称
	Addr string // 物理地址（如 GPIO_1, PWM_1 等）
	Type uint32 // 数据类型
	Rw   int32  // 读写权限
}

// ============================================================================
// DeviceBuilder - 设备实例构建器
// ============================================================================

// DeviceBuilder 设备构建器，用于从模板创建设备实例
type DeviceBuilder struct {
	deviceID     string
	instanceName string
	template     *DeviceTemplate
	wireConfigs  map[string]*WireConfig // wireName -> config
}

// WireConfig Wire 配置（Pin 地址映射）
type WireConfig struct {
	Enabled  bool              // 是否启用此 Wire
	PinAddrs map[string]string // pinName -> address
}

// NewDeviceBuilder 创建设备构建器
func NewDeviceBuilder(deviceID, instanceName string) (*DeviceBuilder, error) {
	template := GetDevice(deviceID)
	if template == nil {
		return nil, fmt.Errorf("device template %s not found", deviceID)
	}

	builder := &DeviceBuilder{
		deviceID:     deviceID,
		instanceName: instanceName,
		template:     template,
		wireConfigs:  make(map[string]*WireConfig),
	}

	// 初始化默认配置：所有必需的 Wire 启用
	for _, wireTemplate := range template.Wires {
		builder.wireConfigs[wireTemplate.Name] = &WireConfig{
			Enabled:  wireTemplate.Required,
			PinAddrs: make(map[string]string),
		}
	}

	return builder, nil
}

// EnableWire 启用某个 Wire（对于可选 Wire）
func (b *DeviceBuilder) EnableWire(wireName string) *DeviceBuilder {
	if config, ok := b.wireConfigs[wireName]; ok {
		config.Enabled = true
	}
	return b
}

// DisableWire 禁用某个 Wire（仅对非必需 Wire 有效）
func (b *DeviceBuilder) DisableWire(wireName string) *DeviceBuilder {
	if config, ok := b.wireConfigs[wireName]; ok {
		// 检查是否为必需 Wire
		for _, wireTemplate := range b.template.Wires {
			if wireTemplate.Name == wireName && !wireTemplate.Required {
				config.Enabled = false
				break
			}
		}
	}
	return b
}

// SetPinAddress 设置 Pin 的物理地址
func (b *DeviceBuilder) SetPinAddress(wireName, pinName, address string) *DeviceBuilder {
	if config, ok := b.wireConfigs[wireName]; ok {
		config.PinAddrs[pinName] = address
	}
	return b
}

// SetPinAddresses 批量设置 Pin 地址
func (b *DeviceBuilder) SetPinAddresses(wireName string, addrs map[string]string) *DeviceBuilder {
	if config, ok := b.wireConfigs[wireName]; ok {
		for pinName, addr := range addrs {
			config.PinAddrs[pinName] = addr
		}
	}
	return b
}

// Build 构建设备实例
func (b *DeviceBuilder) Build() (*DeviceInstance, error) {
	instance := &DeviceInstance{
		DeviceID:     b.deviceID,
		InstanceName: b.instanceName,
		Template:     b.template,
		Wires:        make([]*WireInstance, 0),
	}

	// 构建每个启用的 Wire
	for _, wireTemplate := range b.template.Wires {
		config, ok := b.wireConfigs[wireTemplate.Name]
		if !ok || !config.Enabled {
			continue
		}

		wireInstance := &WireInstance{
			Name:     wireTemplate.Name,
			Clusters: wireTemplate.Clusters,
			Pins:     make([]*PinInstance, 0),
		}

		// 从 Cluster 模板构建 Pin
		for _, clusterName := range wireTemplate.Clusters {
			cluster := GetCluster(clusterName)
			if cluster == nil {
				continue
			}

			for _, pinTemplate := range cluster.Pins {
				// 获取配置的地址（如果有）
				addr := config.PinAddrs[pinTemplate.Name]

				pinInstance := &PinInstance{
					Name: pinTemplate.Name,
					Addr: addr,
					Type: pinTemplate.Type,
					Rw:   pinTemplate.Rw,
				}
				wireInstance.Pins = append(wireInstance.Pins, pinInstance)
			}
		}

		instance.Wires = append(instance.Wires, wireInstance)
	}

	return instance, nil
}

// ============================================================================
// 快捷构建函数
// ============================================================================

// QuickBuildDevice 快速构建设备实例（使用默认配置）
func QuickBuildDevice(deviceID, instanceName string) (*DeviceInstance, error) {
	builder, err := NewDeviceBuilder(deviceID, instanceName)
	if err != nil {
		return nil, err
	}
	return builder.Build()
}

// BuildDeviceWithAddresses 构建设备实例并设置地址
func BuildDeviceWithAddresses(deviceID, instanceName string, addresses map[string]map[string]string) (*DeviceInstance, error) {
	builder, err := NewDeviceBuilder(deviceID, instanceName)
	if err != nil {
		return nil, err
	}

	// 设置地址
	for wireName, pinAddrs := range addresses {
		builder.SetPinAddresses(wireName, pinAddrs)
	}

	return builder.Build()
}
