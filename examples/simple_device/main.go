package main

import (
	"fmt"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/device"
	"github.com/snple/beacon/device/actuators"
	"github.com/snple/beacon/dt"
	"github.com/snple/beacon/edge"
)

func main() {
	fmt.Println("=== Beacon Device Manager 示例 ===")

	// 示例 1：虚拟设备（NoOpActuator）
	fmt.Println("1. 虚拟设备（自动使用 NoOpActuator）")
	runVirtualDevice()

	// 示例 2：预配置执行器的设备
	fmt.Println("\n2. 带预配置执行器的设备")
	runDeviceWithActuator()

	// 示例 3：设备模板+后期配置执行器
	fmt.Println("\n3. 设备模板 + 运行时配置执行器")
	runDeviceTemplate()
}

// 示例 1：虚拟设备
func runVirtualDevice() {
	// 定义设备（不指定 Actuator）
	dev := device.DeviceBuilder("virtual_light", "虚拟灯").
		Wire(device.WireBuilder("ctrl").
			Pin(device.Pin{
				Name: "on",
				Type: nson.DataTypeBOOL,
				Rw:   device.RW,
			}).
			Pin(device.Pin{
				Name: "brightness",
				Type: nson.DataTypeU8,
				Rw:   device.RW,
			}),
		).Done()

	// 创建 DeviceManager
	dm := device.NewDeviceManager(dev)
	// 不设置执行器，Init 时会自动使用 NoOpActuator

	// 创建 Edge（传入 DeviceManager）
	es, err := edge.Edge(
		edge.WithNodeID("NODE001", "secret"),
		edge.WithDeviceManager(dm),
	)
	if err != nil {
		panic(err)
	}

	es.Start()
	defer es.Stop()

	// 查看执行器信息
	infos := dm.ListActuatorInfos()
	for wireID, info := range infos {
		fmt.Printf("  Wire '%s': %s v%s\n", wireID, info.Name, info.Version)
	}

	// 模拟写入
	if err := es.SetPinWrite(dt.PinValue{
		ID:      "ctrl.on",
		Value:   nson.Bool(true),
		Updated: time.Now(),
	}); err != nil {
		fmt.Printf("  错误: %v\n", err)
	} else {
		fmt.Println("  ✓ 写入成功（NoOpActuator 记录但不执行硬件操作）")
	}

	time.Sleep(100 * time.Millisecond)
}

// 示例 2：带执行器的设备（外部设置）
func runDeviceWithActuator() {
	// 定义设备（不包含执行器）
	dev := device.DeviceBuilder("gpio_relay", "GPIO 继电器").
		Wire(device.WireBuilder("relay").
			Type("gpio").
			Pin(device.Pin{
				Name: "ch1",
				Type: nson.DataTypeBOOL,
				Rw:   device.RW,
				Addr: "GPIO17",
			}),
		).Done()

	// 创建 DeviceManager 并设置执行器
	dm := device.NewDeviceManager(dev)
	gpioActuator := &actuators.GPIOActuator{}
	dm.SetActuator("relay", gpioActuator)

	// 创建 Edge（传入已配置执行器的 DeviceManager）
	es, err := edge.Edge(
		edge.WithNodeID("RELAY001", "secret"),
		edge.WithDeviceManager(dm),
	)
	if err != nil {
		panic(err)
	}

	es.Start()
	defer es.Stop()

	// 获取执行器信息
	info, _ := dm.GetActuatorInfo("relay")
	fmt.Printf("  Actuator: %s v%s\n", info.Name, info.Version)

	// 写入命令
	if err := es.SetPinWrite(dt.PinValue{
		ID:      "relay.ch1",
		Value:   nson.Bool(true),
		Updated: time.Now(),
	}); err != nil {
		fmt.Printf("  错误: %v\n", err)
	} else {
		fmt.Println("  ✓ GPIO 执行器已调用（实际硬件需要集成 periph.io）")
	}

	time.Sleep(100 * time.Millisecond)
}

// 示例 3：设备模板 + 后期配置执行器
func runDeviceTemplate() {
	// 预定义设备模板（不配置执行器）
	templateDev := device.DeviceBuilder("temp_sensor", "温度传感器").
		Wire(device.WireBuilder("modbus").
			Type("modbus_rtu"). // 仅指定类型
			Pin(device.Pin{
				Name: "temp",
				Type: nson.DataTypeI16,
				Rw:   device.RO,
				Addr: "30001",
			}),
		).Done()

	// 复制模板
	dev := templateDev // 值复制，安全！

	// 创建 DeviceManager
	dm := device.NewDeviceManager(dev)
	// 实际项目中使用: dm.SetActuator("modbus", &actuators.ModbusRTUActuator{})

	// 创建 Edge
	es, err := edge.Edge(
		edge.WithNodeID("SENSOR001", "secret"),
		edge.WithDeviceManager(dm),
	)
	if err != nil {
		panic(err)
	}

	es.Start()
	defer es.Stop()

	// 查看执行器
	info, _ := dm.GetActuatorInfo("modbus")
	fmt.Printf("  Actuator: %s v%s\n", info.Name, info.Version)
	fmt.Println("  ✓ 设备模板可复制，执行器可后期配置（安全的值传递）")

	time.Sleep(100 * time.Millisecond)
}
