// 使用示例
package device

import (
	"fmt"
	"strings"
)

/*
使用示例：如何用 Cluster 模板定义设备

映射关系：
  - Node   = Device（设备）     -> 一个智能灯设备
  - Wire   = Endpoint（端点）   -> 灯光功能端点
  - Pin    = Attribute（属性）  -> onoff, level 等属性
  - Cluster = Pin 的分组模板    -> OnOff, LevelControl

简化后的数据模型：
  - Node: 只有 ID, Name, Secret, Status
  - Wire: 只有 ID, NodeID, Name, Clusters, Status
  - Pin:  只有 ID, NodeID, WireID, Name, Addr, Status
  - Const: 已移除（用根 Wire 的 BasicInformation Cluster 代替）

Pin 的 Desc, Type, Rw 等元信息从 Cluster 注册表获取
*/

// Example_dimmableLight 可调光灯示例
func Example_dimmableLight() {
	// 构建可调光灯 Wire
	result := BuildDimmableLightWire("living_room_light")

	fmt.Printf("Wire: %s\n", result.Wire.Name)
	fmt.Printf("Clusters: %s\n", result.Wire.Clusters)
	fmt.Printf("Pins:\n")

	// Pin 的元信息从 Cluster 注册表获取
	for _, pin := range result.Pins {
		// 根据 Pin.Name 查找元信息
		if tpl := findPinTemplate(result.Wire.Clusters, pin.Name); tpl != nil {
			fmt.Printf("  - %s (%s, rw=%d): %s\n", pin.Name, tpl.Type, tpl.Rw, tpl.Desc)
		}
	}
}

// findPinTemplate 从 Cluster 注册表查找 Pin 模板
func findPinTemplate(clusters string, pinName string) *PinTemplate {
	for _, clusterName := range strings.Split(clusters, ",") {
		if cluster := GetCluster(clusterName); cluster != nil {
			if tpl := cluster.GetPinTemplate(pinName); tpl != nil {
				return tpl
			}
		}
	}
	return nil
}

// Example_completeDevice 完整设备定义示例
func Example_completeDevice() {
	// 一个完整的设备定义
	// Node: 智能灯设备
	// - 根 Wire: 包含 BasicInformation
	// - 灯光 Wire: 包含 OnOff + LevelControl

	// 1. 根 Wire（必须）
	rootWire := BuildRootWire()
	fmt.Printf("Root Wire: %s, Clusters: %s\n", rootWire.Wire.Name, rootWire.Wire.Clusters)

	// 2. 灯光 Wire
	lightWire := BuildDimmableLightWire("light")
	fmt.Printf("Light Wire: %s, Clusters: %s\n", lightWire.Wire.Name, lightWire.Wire.Clusters)

	// 3. 部署时只需创建简化的记录
	fmt.Println("\n部署到数据库的记录：")
	fmt.Println("Node: { name: 'my_lamp' }")
	fmt.Println("Wire: { name: 'root', clusters: 'BasicInformation' }")
	fmt.Println("Wire: { name: 'light', clusters: 'OnOff,LevelControl' }")
	fmt.Println("Pin:  { name: 'vendor_name' }")
	fmt.Println("Pin:  { name: 'product_name' }")
	fmt.Println("Pin:  { name: 'onoff' }")
	fmt.Println("Pin:  { name: 'level' }")
	fmt.Println("Pin:  { name: 'min_level' }")
	fmt.Println("Pin:  { name: 'max_level' }")
}

// Example_customCluster 自定义 Cluster 示例
func Example_customCluster() {
	// 定义自定义 Cluster
	myCluster := &Cluster{
		ID:          0x1000,
		Name:        "FanControl",
		Description: "风扇控制",
		Pins: []PinTemplate{
			{Name: "power", Desc: "电源", Type: "bool", Rw: 1},
			{Name: "speed", Desc: "速度 (1-5)", Type: "u32", Rw: 1},
			{Name: "mode", Desc: "模式", Type: "u32", Rw: 1},
		},
	}

	// 注册
	RegisterCluster(myCluster)

	// 使用
	result := NewWireBuilder("fan").
		WithClusters("OnOff", "FanControl").
		Build()

	fmt.Printf("Wire: %s, Clusters: %s\n", result.Wire.Name, result.Wire.Clusters)
	fmt.Printf("Pins: ")
	for _, pin := range result.Pins {
		fmt.Printf("%s ", pin.Name)
	}
	fmt.Println()
}

/*
数据流示例：

1. Core 端设置 Pin 值：
   PinWrite { id: "pin_onoff", value: "true" }
   → 自动同步到 Edge

2. Edge 端接收到值变化：
   - 查询 Wire.Clusters = "OnOff,LevelControl"
   - 查询 Cluster 注册表获取 Pin "onoff" 的元信息
   - Type = "bool", Rw = 1
   - 执行硬件操作（如 GPIO 输出高电平）

3. Edge 端读取传感器：
   temperature := readSensor()
   PinValue { id: "pin_temp", value: "2500" }
   → 自动同步到 Core

4. Core/Edge 都可以查询 Cluster 注册表获取 Pin 的完整信息
*/
