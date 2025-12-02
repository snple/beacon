// Package device 提供设备模板定义和注册表功能
// 支持常见的家庭物联网设备模板，并允许用户自定义设备
package device

import (
	"fmt"
	"sync"
)

// ============================================================================
// 设备模板定义
// ============================================================================

// DeviceTemplate 设备模板，定义一个完整的设备类型
type DeviceTemplate struct {
	ID           string         // 设备模板 ID (唯一标识)
	Name         string         // 设备名称 (如 "智能灯泡")
	Category     string         // 设备类别 (lighting, sensor, switch, etc.)
	Description  string         // 设备描述
	Manufacturer string         // 厂商名称
	Model        string         // 型号
	Version      string         // 版本
	Wires        []WireTemplate // Wire 模板列表
}

// WireTemplate Wire 模板定义
type WireTemplate struct {
	Name        string   // Wire 名称
	Description string   // Wire 描述
	Clusters    []string // 使用的 Cluster 列表
	Required    bool     // 是否必需（某些 Wire 可选）
}

// ============================================================================
// 设备注册表
// ============================================================================

// DeviceRegistry 设备模板注册表（线程安全）
type DeviceRegistry struct {
	mu         sync.RWMutex
	templates  map[string]*DeviceTemplate   // deviceID -> DeviceTemplate
	byCategory map[string][]*DeviceTemplate // category -> []*DeviceTemplate
}

var globalRegistry = &DeviceRegistry{
	templates:  make(map[string]*DeviceTemplate),
	byCategory: make(map[string][]*DeviceTemplate),
}

// Register 注册设备模板
func (r *DeviceRegistry) Register(template *DeviceTemplate) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if template.ID == "" {
		return fmt.Errorf("device template ID cannot be empty")
	}

	if _, exists := r.templates[template.ID]; exists {
		return fmt.Errorf("device template %s already registered", template.ID)
	}

	r.templates[template.ID] = template
	r.byCategory[template.Category] = append(r.byCategory[template.Category], template)
	return nil
}

// Get 获取设备模板
func (r *DeviceRegistry) Get(deviceID string) *DeviceTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.templates[deviceID]
}

// List 列出所有设备模板
func (r *DeviceRegistry) List() []*DeviceTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*DeviceTemplate, 0, len(r.templates))
	for _, t := range r.templates {
		result = append(result, t)
	}
	return result
}

// ListByCategory 按类别列出设备模板
func (r *DeviceRegistry) ListByCategory(category string) []*DeviceTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if templates, ok := r.byCategory[category]; ok {
		// 返回副本以避免并发修改
		result := make([]*DeviceTemplate, len(templates))
		copy(result, templates)
		return result
	}
	return []*DeviceTemplate{}
}

// Categories 获取所有设备类别
func (r *DeviceRegistry) Categories() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	categories := make([]string, 0, len(r.byCategory))
	for category := range r.byCategory {
		categories = append(categories, category)
	}
	return categories
}

// ============================================================================
// 全局注册表 API
// ============================================================================

// RegisterDevice 注册设备模板到全局注册表
func RegisterDevice(template *DeviceTemplate) error {
	return globalRegistry.Register(template)
}

// GetDevice 从全局注册表获取设备模板
func GetDevice(deviceID string) *DeviceTemplate {
	return globalRegistry.Get(deviceID)
}

// ListDevices 列出所有设备模板
func ListDevices() []*DeviceTemplate {
	return globalRegistry.List()
}

// ListDevicesByCategory 按类别列出设备模板
func ListDevicesByCategory(category string) []*DeviceTemplate {
	return globalRegistry.ListByCategory(category)
}

// GetCategories 获取所有设备类别
func GetCategories() []string {
	return globalRegistry.Categories()
}

// ============================================================================
// 设备类别常量
// ============================================================================

const (
	CategoryLighting       = "lighting"        // 照明设备
	CategorySensor         = "sensor"          // 传感器
	CategorySwitch         = "switch"          // 开关
	CategoryLock           = "lock"            // 门锁
	CategoryCamera         = "camera"          // 摄像头
	CategoryThermostat     = "thermostat"      // 温控器
	CategoryCurtain        = "curtain"         // 窗帘
	CategorySocket         = "socket"          // 插座
	CategoryAirConditioner = "air_conditioner" // 空调
	CategoryFan            = "fan"             // 风扇
	CategoryHumidifier     = "humidifier"      // 加湿器
	CategoryAirPurifier    = "air_purifier"    // 空气净化器
	CategorySecuritySensor = "security_sensor" // 安防传感器
	CategoryAlarm          = "alarm"           // 报警器
	CategoryDoorbell       = "doorbell"        // 门铃
	CategoryGateway        = "gateway"         // 网关
	CategoryCustom         = "custom"          // 自定义设备
)
