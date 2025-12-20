# Edge Actuator 集成指南

## 架构概览

```
┌─────────────────────────────────────────────────────────────┐
│            Device（纯配置，可安全复制）                    │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Wire (gpio)                                         │  │
│  │    - Pins: [on, off, ...]                           │  │
│  │    - Actuator: GPIOActuator ◄─────────┐             │  │
│  │    - ActuatorConfig: {port: "/dev/..."} │            │  │
│  └──────────────────────────────────────────┼────────────┘  │
│  ✓ 无 mutex/channel（可值传递）             │               │
│  ✓ 可用作设备库模板                         │               │
└──────────────────────────────────────────────┼───────────────┘
                                               │
                                               │ 复制
                                               ▼
                        ┌───────────────────────────────────┐
                        │      EdgeService                  │
                        │  ┌─────────────────────────────┐ │
                        │  │  DeviceManager（指针）      │ │
                        │  │   - device: Device          │ │
                        │  │   - actuators: map[...]     │ │
                        │  │   - mu: sync.RWMutex        │ │
                        │  │   - 管理执行器生命周期      │ │
                        │  └─────────────────────────────┘ │
                        └───────────────────────────────────┘
```

**核心架构：**
- **Device**：纯配置（值类型），可安全复制，作为设备模板
- **DeviceManager**：运行时管理器（指针类型），管理执行器生命周期
- **Edge**：持有 DeviceManager 指针，调用其方法执行硬件操作

## 数据流

### 写入流程（Core → Edge → 硬件）

```
Core 发送 PinWrite
    ↓ Queen MQTT
QueenUpService.handlePinWrite
    ↓
EdgeService.SetPinWrite
    ├─→ Storage.SetPinWrite (保存)
    └─→ DeviceManager.Execute (硬件执行)
            ↓
        Actuator.Execute
            ↓
        硬件操作 (GPIO/Modbus/MQTT...)
```

### 读取流程（传感器 → Edge → Core）

```
DeviceManager.pollActuator (每5秒)
    ↓
Actuator.Read (读取硬件)
    ↓
EdgeService.SetPinValue (更新本地)
    ↓
NotifyPinValue → Queen → Core
```

## 使用示例

### 1. 基础示例：虚拟设备（预定义后替换）

```go
package main

import (
    "github.com/snple/beacon/device"
    "github.com/snple/beacon/edge"
    "github.com/danclive/nson-go"
)

func main() {
    // 定义设备（不指定 Actuator，自动使用 NoOpActuator）
    dev := device.DeviceBuilder("test_device", "测试设备").
        Wire(device.WireBuilder("ctrl").
            Pin(device.Pin{
                Name: "on",
                Type: uint32(nson.DataTypeBOOL),
                Rw:   device.RW,
            }),
        ).Done()

    // 创建 Edge（自动创建 DeviceManager）
    es, _ := edge.Edge(
        edge.WithNodeID("NODE001", "secret"),
        edge.WithDevice(dev), // Device 值传递，安全！
    )

    es.Start()
    defer es.Stop()

    // 获取 DeviceManager
    dm := es.GetDeviceManager()
    _ = dm // DeviceManager 管理执行器生命周期

    // NoOpActuator 只记录状态，不执行实际硬件操作
}
```

### 2. 在设备定义时绑定 Actuator：GPIO 控制

```go
package main

import (
    "github.com/snple/beacon/device"
    "github.com/snple/beacon/device/actuators"
    "github.com/snple/beacon/edge"
    "github.com/danclive/nson-go"
)

func main() {
    // 定义设备时直接绑定 GPIO Actuator
    dev := device.DeviceBuilder("relay_controller", "继电器控制器").
        Wire(device.WireBuilder("relay").
            Type("gpio"). // 类型标识（可选）
            Pin(device.Pin{
                Name: "ch1",
                Desc: "继电器通道1",
                Type: uint32(nson.DataTypeBOOL),
                Rw:   device.RW,
                Addr: "GPIO17",
            }).
            Pin(device.Pin{
                Name: "ch2",
                Desc: "继电器通道2",
                Type: uint32(nson.DataTypeBOOL),
                Rw:   device.RW,
                Addr: "GPIO27",
            }).
            // 🔥 在定义时绑定 Actuator
            WithActuator(&actuators.GPIOActuator{}).
            ActuatorOption("chip", "/dev/gpiochip0"),
        ).Done()

    // 创建 Edge（Device 值传递到 DeviceManager）
    es, _ := edge.Edge(
        edge.WithNodeID("RELAY001", "secret"),
        edge.WithDevice(dev),
    )

    es.Start()
    defer es.Stop()

    // Core 发送命令 → GPIOActuator → 控制实际 GPIO
}
```

### 3. 预定义设备 + 后期替换 Actuator

```go
package main

import (
    "github.com/snple/beacon/device"
    "github.com/snple/beacon/device/actuators"
    "github.com/snple/beacon/edge"
    "github.com/danclive/nson-go"
)

// 预定义设备库（使用 NoOpActuator）
func GetStandardRelay() device.Device {
    return device.DeviceBuilder("relay_2ch", "2路继电器").
        Wire(device.WireBuilder("relay").
            Pin(device.Pin{
                Name: "ch1",
                Type: uint32(nson.DataTypeBOOL),
                Rw:   device.RW,
                Addr: "GPIO17",
            }).
            Pin(device.Pin{
                Name: "ch2",
                Type: uint32(nson.DataTypeBOOL),
                Rw:   device.RW,
                Addr: "GPIO27",
            }),
            // 默认不设置 Actuator（自动使用 NoOp）
        ).Done()
}

func main() {
    // 获取预定义设备模板
    templateDev := GetStandardRelay()

    // 🔥 复制模板并配置执行器（Device 是值类型，可安全复制）
    dev := templateDev // 值复制
    dev.Wires[0].Actuator = &actuators.GPIOActuator{}
    dev.Wires[0].ActuatorConfig = map[string]string{
        "chip": "/dev/gpiochip0",
    }

    // 创建 Edge（DeviceManager 内部会复制一份 Device）
    es, _ := edge.Edge(
        edge.WithNodeID("RELAY001", "secret"),
        edge.WithDevice(dev),
    )

    es.Start()
    defer es.Stop()
}
```

### 4. Modbus 示例：配置与执行器绑定

```go
package main

import (
    "github.com/snple/beacon/device"
    "github.com/snple/beacon/device/actuators"
    "github.com/snple/beacon/edge"
    "github.com/danclive/nson-go"
)

func main() {
    // 定义 Modbus 温湿度传感器
    dev := device.DeviceBuilder("temp_sensor", "温度传感器").
        Wire(device.WireBuilder("modbus").
            Type("modbus_rtu").
            Pin(device.Pin{
                Name: "temp",
                Desc: "温度",
                Type: uint32(nson.DataTypeI16),
                Rw:   device.RO,
                Addr: "30001", // Input Register
            }).
            Pin(device.Pin{
                Name: "humi",
                Desc: "湿度",
                Type: uint32(nson.DataTypeU16),
                Rw:   device.RO,
                Addr: "30002",
            }).
            // 🔥 绑定 Modbus Actuator 及配置
            WithActuator(&actuators.ModbusRTUActuator{}).
            ActuatorOption("port", "/dev/ttyUSB0").
            ActuatorOption("baudrate", "9600").
            ActuatorOption("slave_id", "1"),
        ).Done()

    es, _ := edge.Edge(
        edge.WithNodeID("SENSOR001", "secret"),
        edge.WithDevice(dev),
    )

    es.Start()
    defer es.Stop()

    // DeviceManager 自动每5秒轮询 Modbus → 上报到 Core
}
```

### 5. 完整示例：温控系统（多种 Actuator）

```go
package main

import (
    "github.com/snple/beacon/device"
    "github.com/snple/beacon/device/actuators"
    "github.com/snple/beacon/edge"
    "github.com/danclive/nson-go"
)

func main() {
    // 定义复合设备（传感器 + 执行器）
    dev := device.DeviceBuilder("temp_control", "温控系统").
        // Wire 1: Modbus 温湿度传感器
        Wire(device.WireBuilder("sensor").
            Type("modbus_rtu").
            Pin(device.Pin{
                Name: "temp",
                Type: uint32(nson.DataTypeI16),
                Rw:   device.RO,
                Addr: "30001",
            }).
            Pin(device.Pin{
                Name: "humi",
                Type: uint32(nson.DataTypeU16),
                Rw:   device.RO,
                Addr: "30002",
            }).
            WithActuator(&actuators.ModbusRTUActuator{}).
            ActuatorOption("port", "/dev/ttyUSB0").
            ActuatorOption("baudrate", "9600").
            ActuatorOption("slave_id", "1"),
        ).
        // Wire 2: GPIO 继电器（加热器）
        Wire(device.WireBuilder("heater").
            Type("gpio").
            Pin(device.Pin{
                Name: "on",
                Desc: "加热器开关",
                Type: uint32(nson.DataTypeBOOL),
                Rw:   device.RW,
                Addr: "GPIO17",
            }).
            WithActuator(&actuators.GPIOActuator{}).
            ActuatorOption("chip", "/dev/gpiochip0"),
        ).
        // Wire 3: GPIO 风扇
        Wire(device.WireBuilder("fan").
            Type("gpio").
            Pin(device.Pin{
                Name: "on",
                Desc: "风扇开关",
                Type: uint32(nson.DataTypeBOOL),
                Rw:   device.RW,
                Addr: "GPIO27",
            }).
            WithActuator(&actuators.GPIOActuator{}).
            ActuatorOption("chip", "/dev/gpiochip0"),
        ).Done()

    // 创建 Edge（Device 值传递给 DeviceManager）
    es, err := edge.Edge(
        edge.WithNodeID("ROOM01", "secret"),
        edge.WithDevice(dev), // Device 可安全传递
    )
    if err != nil {
        panic(err)
    }

    es.Start()
    defer es.Stop()

    // 自动工作：
    // 1. DeviceManager 内的 ModbusRTUActuator 每5秒读取温湿度 → 上报 Core
    // 2. Core 根据温度发送加热/制冷命令
    // 3. GPIOActuator 控制继电器
}
```

### 6. MQTT 桥接：设备定义即配置

```go
package main

import (
    "github.com/snple/beacon/device"
    "github.com/snple/beacon/device/actuators"
    "github.com/snple/beacon/edge"
    "github.com/danclive/nson-go"
)

func main() {
    // Zigbee2MQTT 设备桥接
    dev := device.DeviceBuilder("zigbee_light", "Zigbee灯").
        Wire(device.WireBuilder("mqtt").
            Type("mqtt").
            Pin(device.Pin{
                Name: "on",
                Type: uint32(nson.DataTypeBOOL),
                Rw:   device.RW,
                Addr: "zigbee2mqtt/bedroom_light/set/state",
            }).
            Pin(device.Pin{
                Name: "brightness",
                Type: uint32(nson.DataTypeU8),
                Rw:   device.RW,
                Addr: "zigbee2mqtt/bedroom_light/set/brightness",
            }).
            WithActuator(&actuators.MQTTActuator{}).
            ActuatorOption("broker", "tcp://192.168.1.100:1883").
            ActuatorOption("client_id", "beacon_edge"),
        ).Done()

    es, _ := edge.Edge(
        edge.WithNodeID("ZIGBEE_GW", "secret"),
        edge.WithDevice(dev),
    )

    es.Start()
    defer es.Stop()

    // Core 命令 → MQTTActuator → Zigbee2MQTT → Zigbee 设备
}
```

## API 说明

### Device Builder API

```go
// 为 Wire 绑定 Actuator
wire := device.WireBuilder("gpio").
    Pin(...).
    WithActuator(&actuators.GPIOActuator{}). // 绑定执行器实例
    ActuatorOption("chip", "/dev/gpiochip0").   // 设置单个选项
    ActuatorOptions(map[string]string{          // 批量设置选项
        "mode": "output",
        "inverted": "true",
    })

// 预定义设备模板后修改（Device 是值类型，可安全复制）
templateDev := GetPreDefinedDevice("relay")
dev := templateDev // 值复制，安全！
dev.Wires[0].Actuator = &actuators.GPIOActuator{}
dev.Wires[0].ActuatorConfig["chip"] = "/dev/gpiochip0"
```

### DeviceManager API

```go
// EdgeService 提供 DeviceManager 访问
dm := es.GetDeviceManager()

// 获取设备配置（只读）
dev := dm.GetDevice()

// 执行 Pin 写入
err := dm.Execute(ctx, "wireID", "pinName", value)

// 读取 Wire 的所有可读 Pin
values, err := dm.Read(ctx, "wireID")

// 获取执行器信息
info, err := dm.GetActuatorInfo("wireID")
infos := dm.ListActuatorInfos()

// 设置轮询间隔
dm.SetPollInterval(10 * time.Second)
```

### EdgeService 配置

```go
// 推荐方式：在 Device 定义时绑定 Actuator
dev := device.DeviceBuilder(...).
    Wire(device.WireBuilder("gpio").
        WithActuator(&actuators.GPIOActuator{}).
        ActuatorOption("chip", "/dev/gpiochip0"),
    ).Done()

es, _ := edge.Edge(
    edge.WithNodeID("NODE001", "secret"),
    edge.WithDevice(dev), // Device 值传递，安全
)

// Device 是值类型，可以安全复制和传递
templateDev := GetDeviceTemplate("relay")
dev1 := templateDev // 复制给环境1
dev2 := templateDev // 复制给环境2
dev1.Wires[0].Actuator = &actuators.GPIOActuator{}
dev2.Wires[0].Actuator = &actuators.NoOpActuator{} // 测试环境用虚拟执行器
```

### Actuator 优先级

```
1. Device.Wire.Actuator（最高优先级）
   ↓ 如果为 nil
2. 根据 Wire.Type 从注册表查找
   ↓ 如果找不到
3. 使用 NoOpActuator（兜底）
```

### 配置选项优先级

```
1. Device.Wire.ActuatorConfig（设备定义）
   ↓ 如果为空
2. WithActuatorOptions（运行时配置）
   ↓ 如果找不到
3. 空 map（Actuator 使用默认值）
```

### 自定义 Actuator

```go
package myactuators

import (
    "context"
    "github.com/danclive/nson-go"
    "github.com/snple/beacon/device"
)

func init() {
    // 注册自定义 Actuator
    device.RegisterActuator("my_protocol", func() device.Actuator {
        return &MyActuator{}
    })
}

type MyActuator struct {
    // 你的字段
}

func (a *MyActuator) Initialize(ctx context.Context, config device.ActuatorConfig) error {
    // 初始化硬件
    return nil
}

func (a *MyActuator) Execute(ctx context.Context, pinName string, value nson.Value) error {
    // 执行硬件操作
    return nil
}

func (a *MyActuator) Read(ctx context.Context, pinNames []string) (map[string]nson.Value, error) {
    // 读取硬件状态
    return nil, nil
}

func (a *MyActuator) Close() error {
    // 清理资源
    return nil
}

func (a *MyActuator) Info() device.ActuatorInfo {
    return device.ActuatorInfo{
        Name:    "My Custom Actuator",
        Type:    "my_protocol",
        Version: "1.0.0",
    }
}
```

使用：
```go
import _ "myproject/myactuators" // 自动注册

dev := device.DeviceBuilder(...).
    Wire(device.WireBuilder("custom").
        Type("my_protocol"). // 自动使用 MyActuator
        ...
    ).Done()
```

## 配置文件支持（TODO）

未来可以支持从配置文件读取：

```toml
# config/edge.toml

[node]
id = "ROOM01"
secret = "secret123"
device_template = "temp_control"

# Actuator 配置
[actuators.sensor]
port = "/dev/ttyUSB0"
baudrate = "9600"
slave_id = "1"

[actuators.mqtt]
broker = "tcp://192.168.1.100:1883"
client_id = "edge_001"

# 轮询配置
[polling]
interval = "10s"
```

## 故障排查

### 1. Actuator 未执行

检查日志：
```
Actuator initialized: wire=ctrl, type=gpio, name=GPIO Actuator, version=1.0.0
```

如果看到：
```
No actuator for wire ctrl (type=gpio), using noop
```

说明 `actuators` 包未导入，添加：
```go
import _ "github.com/snple/beacon/device/actuators"
```

### 2. 硬件操作失败

SetPinWrite 会记录错误但不阻塞：
```
Execute actuator for pin ctrl.on: GPIO pin not found
```

检查 Pin.Addr 配置是否正确。

### 3. 传感器数据未上报

检查：
- Pin.Rw 是否为 RO 或 RW（只读或可读写才会轮询）
- 轮询是否启用：`pollEnabled[wireID] = true`
- Actuator.Read() 是否返回错误

## 架构设计说明

### Device vs DeviceManager

**Device（值类型）**：
- ✅ 纯配置结构，不包含 mutex/channel 等 NoCopy 字段
- ✅ 可以安全地值传递和复制
- ✅ 适合作为设备模板库
- ✅ 支持预定义后按环境配置

```go
type Device struct {
    ID    string
    Name  string
    Wires []Wire  // 包含 Actuator 配置
}
```

**DeviceManager（指针类型）**：
- ✅ 运行时管理器，包含 mutex/channel
- ✅ 必须通过指针使用
- ✅ 管理执行器生命周期
- ✅ 自动轮询传感器数据

```go
type DeviceManager struct {
    device       Device              // 设备配置（只读）
    actuators    map[string]Actuator // 运行时状态
    mu           sync.RWMutex
    pollWG       sync.WaitGroup
    // ...
}
```

### 使用流程

```go
// 1. 定义设备配置（可复制）
dev := device.DeviceBuilder("relay", "继电器").
    Wire(...).
    Done()

// 2. 创建 Edge（内部创建 DeviceManager）
es, _ := edge.Edge(
    edge.WithDevice(dev), // Device 值传递
)

// 3. DeviceManager 自动初始化执行器
// - 根据 Wire.Actuator 或 Wire.Type 选择执行器
// - 初始化每个执行器
// - 启动传感器轮询

// 4. 通过 DeviceManager 执行硬件操作
dm := es.GetDeviceManager()
dm.Execute(ctx, "wireID", "pinName", value)
```

## 总结

通过 Device + DeviceManager 架构，Edge 现在可以：

1. ✅ **类型安全** - Device 可值传递，DeviceManager 必须指针
2. ✅ **自动管理硬件** - DeviceManager 管理执行器生命周期
3. ✅ **无缝集成** - SetPinWrite 自动调用 DeviceManager.Execute
4. ✅ **自动轮询** - 传感器数据自动上报
5. ✅ **可扩展** - 支持自定义 Actuator
6. ✅ **零配置** - 虚拟设备开箱即用
7. ✅ **模板化** - Device 可作为可复制的设备模板

**Device 定义硬件配置，DeviceManager 管理执行器，Edge 负责通讯** - 职责清晰！
