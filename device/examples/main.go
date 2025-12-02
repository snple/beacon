// Package main 展示如何使用 device 包
package main

import (
	"fmt"

	"github.com/snple/beacon/device"
)

// Example1_QueryRegistry 示例：查询设备注册表
func Example1_QueryRegistry() {
	fmt.Println("=== 示例 1: 查询设备注册表 ===")

	// 1. 列出所有设备类别
	categories := device.GetCategories()
	fmt.Printf("设备类别数量: %d\n", len(categories))
	fmt.Printf("类别列表: %v\n\n", categories)

	// 2. 列出照明类设备
	fmt.Println("照明设备:")
	lightDevices := device.ListDevicesByCategory(device.CategoryLighting)
	for _, dev := range lightDevices {
		fmt.Printf("  - %s (%s)\n", dev.Name, dev.ID)
	}
	fmt.Println()

	// 3. 列出传感器设备
	fmt.Println("传感器设备:")
	sensorDevices := device.ListDevicesByCategory(device.CategorySensor)
	for _, dev := range sensorDevices {
		fmt.Printf("  - %s (%s)\n", dev.Name, dev.ID)
	}
	fmt.Println()

	// 4. 查询特定设备模板
	bulb := device.GetDevice("smart_bulb_color")
	if bulb != nil {
		fmt.Printf("设备: %s\n", bulb.Name)
		fmt.Printf("类别: %s\n", bulb.Category)
		fmt.Printf("厂商: %s\n", bulb.Manufacturer)
		fmt.Printf("型号: %s\n", bulb.Model)
		fmt.Println("Wire 列表:")
		for _, wire := range bulb.Wires {
			fmt.Printf("  - %s: %v (必需: %v)\n",
				wire.Name, wire.Clusters, wire.Required)
		}
	}
	fmt.Println()
}

// Example2_QuickBuild 示例：快速创建设备实例
func Example2_QuickBuild() {
	fmt.Println("=== 示例 2: 快速创建设备实例 ===")

	// 快速创建开关灯实例
	instance, err := device.QuickBuildDevice("smart_bulb_onoff", "客厅灯")
	if err != nil {
		panic(err)
	}

	fmt.Printf("设备实例: %s\n", instance.InstanceName)
	fmt.Printf("模板 ID: %s\n", instance.DeviceID)
	fmt.Printf("模板名称: %s\n\n", instance.Template.Name)

	// 遍历所有 Wire
	for _, wire := range instance.Wires {
		fmt.Printf("Wire: %s\n", wire.Name)
		fmt.Printf("  Clusters: %v\n", wire.Clusters)
		fmt.Printf("  Pins:\n")
		for _, pin := range wire.Pins {
			fmt.Printf("    - %s (Type: %d, Rw: %d, Addr: %s)\n",
				pin.Name, pin.Type, pin.Rw, pin.Addr)
		}
		fmt.Println()
	}
}

// Example3_BuildWithAddresses 示例：创建设备并配置地址
func Example3_BuildWithAddresses() {
	fmt.Println("=== 示例 3: 创建设备并配置地址 ===")

	// 创建构建器
	builder, err := device.NewDeviceBuilder("smart_bulb_color", "客厅彩灯")
	if err != nil {
		panic(err)
	}

	// 配置 Pin 物理地址
	builder.SetPinAddress("light", "onoff", "GPIO_1")
	builder.SetPinAddress("light", "level", "PWM_1")
	builder.SetPinAddress("light", "hue", "PWM_2")
	builder.SetPinAddress("light", "saturation", "PWM_3")

	// 构建实例
	instance, err := builder.Build()
	if err != nil {
		panic(err)
	}

	fmt.Printf("设备实例: %s\n", instance.InstanceName)
	fmt.Printf("设备类型: %s\n\n", instance.Template.Name)

	// 查找 light wire
	for _, wire := range instance.Wires {
		if wire.Name == "light" {
			fmt.Printf("Wire: %s\n", wire.Name)
			fmt.Printf("Clusters: %v\n", wire.Clusters)
			fmt.Println("Pins 配置:")
			for _, pin := range wire.Pins {
				fmt.Printf("  - %s -> %s (Type: %d, Rw: %d)\n",
					pin.Name, pin.Addr, pin.Type, pin.Rw)
			}
			break
		}
	}
	fmt.Println()
}

// Example4_BatchAddresses 示例：批量配置地址
func Example4_BatchAddresses() {
	fmt.Println("=== 示例 4: 批量配置地址 ===")

	// 使用快捷函数批量设置地址
	addresses := map[string]map[string]string{
		"light": {
			"onoff":      "GPIO_1",
			"level":      "PWM_1",
			"hue":        "PWM_2",
			"saturation": "PWM_3",
		},
	}

	instance, err := device.BuildDeviceWithAddresses(
		"smart_bulb_color",
		"卧室彩灯",
		addresses,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("设备实例: %s\n", instance.InstanceName)

	// 显示配置结果
	for _, wire := range instance.Wires {
		if wire.Name == "light" {
			fmt.Println("地址配置:")
			for _, pin := range wire.Pins {
				if pin.Addr != "" {
					fmt.Printf("  %s -> %s\n", pin.Name, pin.Addr)
				}
			}
			break
		}
	}
	fmt.Println()
}

// Example5_MultiWireDevice 示例：多 Wire 设备
func Example5_MultiWireDevice() {
	fmt.Println("=== 示例 5: 多 Wire 设备 (双路开关) ===")

	builder, err := device.NewDeviceBuilder("switch_2gang", "客厅双开")
	if err != nil {
		panic(err)
	}

	// 为每个开关配置地址
	builder.SetPinAddress("switch1", "onoff", "GPIO_1")
	builder.SetPinAddress("switch2", "onoff", "GPIO_2")

	instance, err := builder.Build()
	if err != nil {
		panic(err)
	}

	fmt.Printf("设备实例: %s\n", instance.InstanceName)
	fmt.Printf("Wire 数量: %d\n\n", len(instance.Wires))

	// 遍历所有 Wire
	for _, wire := range instance.Wires {
		fmt.Printf("Wire: %s\n", wire.Name)
		if len(wire.Pins) > 0 {
			for _, pin := range wire.Pins {
				fmt.Printf("  Pin: %s -> %s\n", pin.Name, pin.Addr)
			}
		}
	}
	fmt.Println()
}

// Example6_SensorDevice 示例：传感器设备
func Example6_SensorDevice() {
	fmt.Println("=== 示例 6: 传感器设备 ===")

	instance, err := device.QuickBuildDevice("temp_humi_sensor", "客厅温湿度")
	if err != nil {
		panic(err)
	}

	fmt.Printf("设备实例: %s\n", instance.InstanceName)
	fmt.Printf("设备类型: %s\n\n", instance.Template.Name)

	// 查找 sensor wire
	for _, wire := range instance.Wires {
		if wire.Name == "sensor" {
			fmt.Printf("传感器 Wire: %s\n", wire.Name)
			fmt.Printf("Clusters: %v\n", wire.Clusters)
			fmt.Println("传感器 Pins (只读):")
			for _, pin := range wire.Pins {
				rwStr := "只读"
				if pin.Rw == 1 {
					rwStr = "读写"
				}
				fmt.Printf("  - %s (Type: %d, %s)\n",
					pin.Name, pin.Type, rwStr)
			}
			break
		}
	}
	fmt.Println()
}

// Example7_RegisterCustomDevice 示例：注册自定义设备
func Example7_RegisterCustomDevice() {
	fmt.Println("=== 示例 7: 注册自定义设备 ===")

	// 定义自定义设备模板
	customDevice := &device.DeviceTemplate{
		ID:           "my_iot_device",
		Name:         "我的物联网设备",
		Category:     device.CategoryCustom,
		Description:  "具有特殊功能的物联网设备",
		Manufacturer: "MyCompany",
		Model:        "IOT-001",
		Version:      "1.0.0",
		Wires: []device.WireTemplate{
			{
				Name:     "root",
				Clusters: []string{"BasicInformation"},
				Required: true,
			},
			{
				Name:     "control",
				Clusters: []string{"OnOff", "LevelControl"},
				Required: true,
			},
			{
				Name:     "sensor",
				Clusters: []string{"TemperatureMeasurement"},
				Required: false, // 可选
			},
		},
	}

	// 注册到全局注册表
	err := device.RegisterDevice(customDevice)
	if err != nil {
		panic(err)
	}

	fmt.Printf("成功注册设备: %s (%s)\n", customDevice.Name, customDevice.ID)

	// 验证可以获取
	retrieved := device.GetDevice("my_iot_device")
	if retrieved != nil {
		fmt.Printf("设备名称: %s\n", retrieved.Name)
		fmt.Printf("类别: %s\n", retrieved.Category)
		fmt.Printf("厂商: %s\n", retrieved.Manufacturer)
		fmt.Printf("型号: %s\n", retrieved.Model)
		fmt.Println("Wire 列表:")
		for _, wire := range retrieved.Wires {
			fmt.Printf("  - %s (必需: %v, Clusters: %v)\n",
				wire.Name, wire.Required, wire.Clusters)
		}
	}

	fmt.Println()

	// 创建实例
	builder, _ := device.NewDeviceBuilder("my_iot_device", "书房设备")

	// 启用可选的 sensor wire
	builder.EnableWire("sensor")

	// 配置地址
	builder.SetPinAddress("control", "onoff", "GPIO_5")
	builder.SetPinAddress("control", "level", "PWM_5")

	instance, err := builder.Build()
	if err != nil {
		panic(err)
	}

	fmt.Printf("创建实例: %s\n", instance.InstanceName)
	fmt.Printf("Wire 数量: %d\n", len(instance.Wires))
	fmt.Println()
}

// Example8_ListAllDevices 示例：列出所有设备
func Example8_ListAllDevices() {
	fmt.Println("=== 示例 8: 列出所有设备 ===")

	devices := device.ListDevices()
	fmt.Printf("总设备数量: %d\n\n", len(devices))

	// 按类别分组显示
	categoryMap := make(map[string][]*device.DeviceTemplate)
	for _, dev := range devices {
		categoryMap[dev.Category] = append(categoryMap[dev.Category], dev)
	}

	for _, category := range device.GetCategories() {
		if devs, ok := categoryMap[category]; ok {
			fmt.Printf("%s (%d 个设备):\n", category, len(devs))
			for _, dev := range devs {
				fmt.Printf("  - %s (%s)\n", dev.Name, dev.ID)
			}
			fmt.Println()
		}
	}
}

func main() {
	Example1_QueryRegistry()
	Example2_QuickBuild()
	Example3_BuildWithAddresses()
	Example4_BatchAddresses()
	Example5_MultiWireDevice()
	Example6_SensorDevice()
	Example7_RegisterCustomDevice()
	Example8_ListAllDevices()

	fmt.Println("✅ 所有示例运行完成")
}
