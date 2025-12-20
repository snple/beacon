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
	// 示例 1：虚拟设备（使用 NoOpActuator）
	fmt.Println("=== 示例 1：虚拟设备 ===")
	runVirtualDevice()

	// 示例 2：带预配置执行器的设备
	fmt.Println("\n=== 示例 2：带预配置执行器的设备 ===")
	runDeviceWithActuator()
}

// ============================================================================
// 示例 1：虚拟设备（Wire.Actuator 为 nil，自动使用 NoOpActuator）
// ============================================================================

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

	// 创建 Edge（设备会自动使用 NoOpActuator）
	es, err := edge.Edge(
		edge.WithNodeID("NODE001", "secret"),
		edge.WithDevice(dev),
	)
	if err != nil {
		panic(err)
	}

	es.Start()
	defer es.Stop()

	// 获取设备并查看执行器信息
	if device := es.GetDeviceManager(); device != nil {
		infos := device.ListActuatorInfos()
		for wireID, info := range infos {
			fmt.Printf("  Wire %s: %s v%s\n", wireID, info.Name, info.Version)
		}
	}

	// 模拟写入
	ctx := context.Background()
	if err := es.SetPinWrite(ctx, dt.PinValue{
		ID:    "ctrl.on",
		Value: nson.Bool(true),
	}); err != nil {
		fmt.Printf("  SetPinWrite error: %v\n", err)
	} else {
		fmt.Println("  ✓ NoOpActuator 记录了状态（无实际硬件操作）")
	}

	time.Sleep(100 * time.Millisecond)
}

// ============================================================================
// 示例 2：带预配置执行器的设备
// ============================================================================

func runDeviceWithActuator() {
	// 创建 GPIO 执行器（带配置）
	gpioActuator := &actuators.GPIOActuator{}

	// 定义设备（Wire 预配置 Actuator）
	dev := device.DeviceBuilder("gpio_relay", "GPIO 继电器").
		Wire(device.WireBuilder("relay").
			Type("gpio"). // Type 仅用于识别，Actuator 已指定
			WithActuator(gpioActuator).
			ActuatorOptions(map[string]string{
				"port": "/dev/gpiochip0",
			}).
			Pin(device.Pin{
				Name: "ch1",
				Desc: "通道1",
				Type: uint32(nson.DataTypeBOOL),
				Rw:   device.RW,
				Addr: "GPIO17", // GPIO 引脚号
			}).
			Pin(device.Pin{
				Name: "ch2",
				Desc: "通道2",
				Type: uint32(nson.DataTypeBOOL),
				Rw:   device.RW,
				Addr: "GPIO27",
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

	// 执行器信息
	if device := es.GetDeviceManager(); device != nil {
		if info, err := device.GetActuatorInfo("relay"); err == nil {
			fmt.Printf("  Actuator: %s v%s\n", info.Name, info.Version)
		}
	}

	// 写入命令（会调用 GPIOActuator.Execute）
	ctx := context.Background()
	if err := es.SetPinWrite(ctx, dt.PinValue{
		ID:    "relay.ch1",
		Value: nson.Bool(true),
	}); err != nil {
		fmt.Printf("  SetPinWrite error: %v\n", err)
	} else {
		fmt.Println("  ✓ GPIOActuator.Execute 被调用（TODO: 实际 GPIO 库）")
	}

	time.Sleep(100 * time.Millisecond)
}

// ============================================================================
// 示例 3：预定义设备库模式（先定义模板，再替换执行器）
// ============================================================================

// 预定义设备库（使用 NoOpActuator，便于测试和部署前验证）
func GetStandardTempSensor() device.Device {
	return device.DeviceBuilder("temp_sensor_std", "标准温湿度传感器").
		Desc("Modbus RTU 温湿度传感器").
		Tags("sensor", "modbus", "temperature", "humidity").
		Wire(device.WireBuilder("modbus").
			Type("modbus_rtu").
			Pin(device.Pin{
				Name: "temp",
				Desc: "温度",
				Type: uint32(nson.DataTypeI16),
				Rw:   device.RO,
				Addr: "30001",
				Unit: "°C",
			}).
			Pin(device.Pin{
				Name: "humi",
				Desc: "湿度",
				Type: uint32(nson.DataTypeU16),
				Rw:   device.RO,
				Addr: "30002",
				Unit: "%",
			}),
		).Done()
}
