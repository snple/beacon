// Package device 提供 Wire Type 和 Tags 字段的使用示例
package device

import (
	"context"

	"github.com/snple/beacon/edge/model"
)

// ============================================================================
// Wire Type 和 Tags 字段使用示例
// ============================================================================

// Type 字段：标识 Wire 的设备类型/型号
// Tags 字段：提供额外的标签信息，支持多个键值对

// 使用示例：

// 示例 1：可调光灯
// BuildDimmableLightWithType 构建带类型标识的可调光灯 Wire
func BuildDimmableLightWithType(name string) *SimpleBuildResult {
	result := NewWireBuilder(name).
		WithClusters("OnOff", "LevelControl").
		Build()

	// 设置 Type 字段标识设备类型
	result.Wire.Type = "DimmableLight"

	// 设置 Tags 字段提供额外信息
	result.Wire.Tags = "category:light,feature:dimmable,room:bedroom"

	return result
}

// 示例 2：RGB 彩色灯
func BuildColorLightWithType(name string) *SimpleBuildResult {
	result := NewWireBuilder(name).
		WithClusters("OnOff", "LevelControl", "ColorControl").
		Build()

	result.Wire.Type = "ColorLight"
	result.Wire.Tags = "category:light,feature:color,feature:dimmable,vendor:philips"

	return result
}

// 示例 3：温湿度传感器
func BuildTempHumiSensorWithType(name string) *SimpleBuildResult {
	result := NewWireBuilder(name).
		WithClusters("TemperatureMeasurement", "HumidityMeasurement").
		Build()

	result.Wire.Type = "TempHumiditySensor"
	result.Wire.Tags = "category:sensor,function:temperature,function:humidity,location:outdoor"

	return result
}

// 示例 4：三联开关
func BuildThreeGangSwitchWithType(name string) *SimpleBuildResult {
	result := NewWireBuilder(name).
		WithClusters("OnOff"). // 每个开关都是一个 OnOff
		Build()

	result.Wire.Type = "ThreeGangSwitch"
	result.Wire.Tags = "category:switch,gangs:3,location:livingroom,position:wall"

	return result
}

// ============================================================================
// Tags 格式说明
// ============================================================================

// 推荐的 Tags 格式：key:value,key:value,...
//
// 常用 key 示例：
// - category: 设备大类（light, sensor, switch, actuator, etc.）
// - type: 具体类型（也可以使用 Type 字段）
// - feature: 功能特性（dimmable, color, motion, etc.）
// - vendor: 厂商名称
// - model: 型号
// - location: 位置（indoor, outdoor, bedroom, kitchen, etc.）
// - room: 房间
// - position: 安装位置（ceiling, wall, floor, etc.）
// - function: 功能描述
// - gangs: 开关联数
// - protocol: 通信协议（zigbee, wifi, ble, etc.）

// ============================================================================
// 查询和过滤示例
// ============================================================================

// FindWiresByType 根据 Type 查询 Wire
func (init *Initializer) FindWiresByType(ctx context.Context, wireType string) ([]model.Wire, error) {
	var wires []model.Wire
	err := init.db.NewSelect().
		Model(&wires).
		Where("type = ?", wireType).
		Scan(ctx)
	return wires, err
}

// FindWiresByTag 根据 Tags 包含的内容查询 Wire
func (init *Initializer) FindWiresByTag(ctx context.Context, tag string) ([]model.Wire, error) {
	var wires []model.Wire
	err := init.db.NewSelect().
		Model(&wires).
		Where("tags LIKE ?", "%"+tag+"%").
		Scan(ctx)
	return wires, err
}

// FindLightWires 查询所有灯光设备
func (init *Initializer) FindLightWires(ctx context.Context) ([]model.Wire, error) {
	return init.FindWiresByTag(ctx, "category:light")
}

// FindSensorWires 查询所有传感器设备
func (init *Initializer) FindSensorWires(ctx context.Context) ([]model.Wire, error) {
	return init.FindWiresByTag(ctx, "category:sensor")
}

// ============================================================================
// 实际使用示例
// ============================================================================

// ExampleCreateDeviceWithTypeAndTags 演示如何创建带类型和标签的设备
func ExampleCreateDeviceWithTypeAndTags(ctx context.Context, init *Initializer) error {
	// 创建卧室可调光灯
	bedroomLight := BuildDimmableLightWithType("bedroom_light")
	if err := init.CreateWireFromBuilder(ctx, bedroomLight); err != nil {
		return err
	}

	// 创建客厅彩色灯
	livingroomLight := BuildColorLightWithType("livingroom_light")
	livingroomLight.Wire.Tags = "category:light,feature:color,room:livingroom,position:ceiling"
	if err := init.CreateWireFromBuilder(ctx, livingroomLight); err != nil {
		return err
	}

	// 创建室外温湿度传感器
	outdoorSensor := BuildTempHumiSensorWithType("outdoor_sensor")
	if err := init.CreateWireFromBuilder(ctx, outdoorSensor); err != nil {
		return err
	}

	return nil
}
