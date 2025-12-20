package main

import (
	"context"
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
				Type: uint32(nson.DataTypeBOOL),
				Rw:   device.RW,
			}).
			Pin(device.Pin{
				Name: "brightness",
				Type: uint32(nson.DataTypeU8),
				Rw:   device.RW,
			}),
		).Done()

	// 创建 Edge（自动使用 NoOpActuator）
	es, err := edge.Edge(
		edge.WithNodeID("NODE001", "secret"),
		edge.WithDevice(dev),
	)
	if err != nil {
		panic(err)
	}

	es.Start()
	defer es.Stop()

	// 获取设备管理器
	dm := es.GetDeviceManager()

	// 查看执行器信息
	infos := dm.ListActuatorInfos()
	for wireID, info := range infos {
		fmt.Printf("  Wire '%s': %s v%s\n", wireID, info.Name, info.Version)
	}

	// 模拟写入
	ctx := context.Background()
	if err := es.SetPinWrite(ctx, dt.PinValue{
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

// 示例 2：带预配置执行器
func runDeviceWithActuator() {
	// 创建 GPIO 执行器实例
	gpioActuator := &actuators.GPIOActuator{}

	// 定义设备并绑定执行器
	dev := device.DeviceBuilder("gpio_relay", "GPIO 继电器").
		Wire(device.WireBuilder("relay").
			Type("gpio").
			WithActuator(gpioActuator). // 预配置执行器
			ActuatorOptions(map[string]string{
				"port": "/dev/gpiochip0",
			}).
			Pin(device.Pin{
				Name: "ch1",
				Type: uint32(nson.DataTypeBOOL),
				Rw:   device.RW,
				Addr: "GPIO17",
			}),
		).Done()

	// 创建 Edge
	es, err := edge.Edge(
		edge.WithNodeID("RELAY001", "secret"),
		edge.WithDevice(dev),
	)
	if err != nil {
		panic(err)
	}

	es.Start()
	defer es.Stop()

	// 获取执行器信息
	dm := es.GetDeviceManager()
	info, _ := dm.GetActuatorInfo("relay")
	fmt.Printf("  Actuator: %s v%s\n", info.Name, info.Version)

	// 写入命令
	ctx := context.Background()
	if err := es.SetPinWrite(ctx, dt.PinValue{
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

// 示例 3：设备模板 + 后期配置
func runDeviceTemplate() {
	// 预定义设备模板（不配置执行器）
	templateDev := device.DeviceBuilder("temp_sensor", "温度传感器").
		Wire(device.WireBuilder("modbus").
			Type("modbus_rtu"). // 仅指定类型
			Pin(device.Pin{
				Name: "temp",
				Type: uint32(nson.DataTypeI16),
				Rw:   device.RO,
				Addr: "30001",
			}),
		).Done()

	// 复制模板并配置实际执行器
	dev := templateDev // 值复制，安全！

	// 为 Wire 配置执行器（使用 NoOpActuator 模拟）
	for i := range dev.Wires {
		if dev.Wires[i].Name == "modbus" {
			// 实际项目中使用: &actuators.ModbusRTUActuator{}
			// 这里用 nil 会自动使用 NoOpActuator
			dev.Wires[i].Actuator = nil
			dev.Wires[i].ActuatorConfig = map[string]string{
				"port":     "/dev/ttyUSB0",
				"baudrate": "9600",
				"slave_id": "1",
			}
		}
	}

	// 创建 Edge
	es, err := edge.Edge(
		edge.WithNodeID("SENSOR001", "secret"),
		edge.WithDevice(dev),
	)
	if err != nil {
		panic(err)
	}

	es.Start()
	defer es.Stop()

	// 查看执行器
	dm := es.GetDeviceManager()
	info, _ := dm.GetActuatorInfo("modbus")
	fmt.Printf("  Actuator: %s v%s\n", info.Name, info.Version)
	fmt.Println("  ✓ 设备模板可复制，执行器可后期配置（安全的值传递）")

	time.Sleep(100 * time.Millisecond)
}
