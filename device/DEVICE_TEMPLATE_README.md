# Device 包 - 设备模板和注册表

## 概述

`device` 包提供标准化的家庭物联网设备模板定义和运行时注册表功能。

### 核心功能

1. **设备模板注册表**：定义和管理常见的家庭物联网设备模板
2. **运行时查询**：作为注册表供应用程序查询设备信息
3. **自定义设备**：允许用户注册自定义设备类型
4. **设备实例构建**：从模板创建具体的设备实例

## 核心概念

### 1. DeviceTemplate（设备模板）

设备模板定义了一类设备的标准结构，包含：
- 设备基本信息（ID、名称、类别、厂商等）
- Wire 模板列表（定义设备包含哪些功能端点）
- 每个 Wire 使用的 Cluster（功能集合）

```go
type DeviceTemplate struct {
    ID          string         // 设备模板 ID (唯一标识)
    Name        string         // 设备名称
    Category    string         // 设备类别
    Description string         // 描述
    Manufacturer string        // 厂商名称
    Model       string         // 型号
    Version     string         // 版本
    Wires       []WireTemplate // Wire 模板列表
}
```

### 2. Cluster（功能集合）

Cluster 定义了一组标准的 Pin（属性），如：
- `OnOff`: 开关控制
- `LevelControl`: 亮度/级别控制
- `ColorControl`: 颜色控制
- `TemperatureMeasurement`: 温度测量
- `HumidityMeasurement`: 湿度测量
- `BasicInformation`: 设备基本信息

### 3. DeviceInstance（设备实例）

从设备模板创建的具体实例，可以配置：
- 实例名称（用户自定义）
- Pin 的物理地址（GPIO、PWM 等）
- 启用/禁用可选的 Wire

## 标准设备类别

| 类别 | 说明 | 示例设备 |
|------|------|----------|
| `lighting` | 照明设备 | 智能灯泡、LED 灯带 |
| `sensor` | 传感器 | 温湿度传感器、空气质量传感器 |
| `switch` | 开关 | 1/2/3 路开关 |
| `socket` | 插座 | 智能插座 |
| `lock` | 门锁 | 智能门锁 |
| `camera` | 摄像头 | 智能摄像头 |
| `thermostat` | 温控器 | 空调控制器 |
| `curtain` | 窗帘 | 电动窗帘 |
| `security_sensor` | 安防传感器 | 门窗传感器、人体传感器、烟雾传感器 |
| `alarm` | 报警器 | 智能报警器 |
| `doorbell` | 门铃 | 智能门铃 |
| `gateway` | 网关 | 智能网关 |
| `custom` | 自定义 | 用户自定义设备 |

## 标准设备模板列表

### 照明设备

| 设备 ID | 名称 | 功能 |
|---------|------|------|
| `smart_bulb_onoff` | 智能开关灯泡 | 开关控制 |
| `smart_bulb_dimmable` | 可调光智能灯泡 | 开关 + 亮度调节 |
| `smart_bulb_color` | 彩色智能灯泡 | 开关 + 亮度 + 颜色 |
| `led_strip` | LED 智能灯带 | 开关 + 亮度 + 颜色 |

### 传感器设备

| 设备 ID | 名称 | 功能 |
|---------|------|------|
| `temp_humi_sensor` | 温湿度传感器 | 温度 + 湿度测量 |
| `temperature_sensor` | 温度传感器 | 温度测量 |
| `humidity_sensor` | 湿度传感器 | 湿度测量 |
| `air_quality_sensor` | 空气质量传感器 | PM2.5、CO2 等 |

### 开关和插座

| 设备 ID | 名称 | 功能 |
|---------|------|------|
| `switch_1gang` | 单路智能开关 | 1 路开关控制 |
| `switch_2gang` | 双路智能开关 | 2 路开关控制 |
| `switch_3gang` | 三路智能开关 | 3 路开关控制 |
| `smart_socket` | 智能插座 | 远程开关控制 |

### 安防设备

| 设备 ID | 名称 | 功能 |
|---------|------|------|
| `door_window_sensor` | 门窗传感器 | 检测开关状态 |
| `motion_sensor` | 人体传感器 | 检测人体移动 |
| `smoke_sensor` | 烟雾传感器 | 检测烟雾 |
| `water_leak_sensor` | 水浸传感器 | 检测漏水 |
| `smart_lock` | 智能门锁 | 锁定/解锁控制 |

### 环境控制

| 设备 ID | 名称 | 功能 |
|---------|------|------|
| `smart_curtain` | 智能窗帘 | 开合控制 |
| `ac_controller` | 空调控制器 | 温度控制 |
| `smart_fan` | 智能风扇 | 风速控制 |
| `humidifier` | 智能加湿器 | 湿度控制 |
| `air_purifier` | 空气净化器 | 档位控制 |

### 其他设备

| 设备 ID | 名称 | 功能 |
|---------|------|------|
| `smart_gateway` | 智能网关 | 子设备管理 |
| `smart_doorbell` | 智能门铃 | 按铃检测 |
| `smart_camera` | 智能摄像头 | 视频监控 |
| `smart_alarm` | 智能报警器 | 报警控制 |

## 使用示例

### 1. 查询设备注册表

```go
package main

import (
    "fmt"
    "github.com/snple/beacon/device"
)

func main() {
    // 列出所有设备类别
    categories := device.GetCategories()
    fmt.Println("设备类别:", categories)

    // 列出照明类设备
    lightDevices := device.ListDevicesByCategory(device.CategoryLighting)
    for _, dev := range lightDevices {
        fmt.Printf("- %s: %s\n", dev.ID, dev.Name)
    }

    // 获取特定设备模板
    bulb := device.GetDevice("smart_bulb_color")
    if bulb != nil {
        fmt.Printf("设备: %s\n", bulb.Name)
        fmt.Printf("类别: %s\n", bulb.Category)
        for _, wire := range bulb.Wires {
            fmt.Printf("  Wire: %s, Clusters: %v\n", wire.Name, wire.Clusters)
        }
    }
}
```

### 2. 创建设备实例（简单方式）

```go
package main

import (
    "fmt"
    "github.com/snple/beacon/device"
)

func main() {
    // 快速创建设备实例（使用默认配置）
    instance, err := device.QuickBuildDevice("smart_bulb_onoff", "客厅灯")
    if err != nil {
        panic(err)
    }

    fmt.Printf("设备实例: %s\n", instance.InstanceName)
    fmt.Printf("模板 ID: %s\n", instance.DeviceID)

    for _, wire := range instance.Wires {
        fmt.Printf("  Wire: %s\n", wire.Name)
        for _, pin := range wire.Pins {
            fmt.Printf("    Pin: %s (Type: %d, Rw: %d)\n",
                pin.Name, pin.Type, pin.Rw)
        }
    }
}
```

### 3. 创建设备实例（配置 Pin 地址）

```go
package main

import (
    "github.com/snple/beacon/device"
)

func main() {
    // 使用构建器创建设备实例
    builder, err := device.NewDeviceBuilder("smart_bulb_color", "客厅彩灯")
    if err != nil {
        panic(err)
    }

    // 配置 Pin 的物理地址
    builder.SetPinAddress("light", "onoff", "GPIO_1")
    builder.SetPinAddress("light", "level", "PWM_1")
    builder.SetPinAddress("light", "hue", "PWM_2")
    builder.SetPinAddress("light", "saturation", "PWM_3")

    // 构建实例
    instance, err := builder.Build()
    if err != nil {
        panic(err)
    }

    // 使用实例...
}
```

### 4. 批量设置 Pin 地址

```go
package main

import (
    "github.com/snple/beacon/device"
)

func main() {
    // 使用 BuildDeviceWithAddresses 快捷函数
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
        "客厅彩灯",
        addresses,
    )
    if err != nil {
        panic(err)
    }

    // 使用实例...
}
```

### 5. 注册自定义设备

```go
package main

import (
    "github.com/snple/beacon/device"
)

func main() {
    // 定义自定义设备模板
    customDevice := &device.DeviceTemplate{
        ID:           "my_custom_light",
        Name:         "我的自定义灯",
        Category:     device.CategoryCustom,
        Description:  "具有特殊功能的灯",
        Manufacturer: "MyCompany",
        Model:        "CL-001",
        Version:      "1.0.0",
        Wires: []device.WireTemplate{
            {
                Name:     "root",
                Clusters: []string{"BasicInformation"},
                Required: true,
            },
            {
                Name:     "light",
                Clusters: []string{"OnOff", "LevelControl"},
                Required: true,
            },
            {
                Name:     "special",
                Clusters: []string{"CustomCluster"}, // 自定义 Cluster
                Required: false,
            },
        },
    }

    // 注册到全局注册表
    err := device.RegisterDevice(customDevice)
    if err != nil {
        panic(err)
    }

    // 后续可以像标准设备一样使用
    instance, err := device.QuickBuildDevice("my_custom_light", "书房灯")
    if err != nil {
        panic(err)
    }

    // 使用实例...
}
```

### 6. 创建多 Wire 设备

```go
package main

import (
    "github.com/snple/beacon/device"
)

func main() {
    // 创建双路开关实例
    builder, _ := device.NewDeviceBuilder("switch_2gang", "客厅双开")

    // 为每个开关配置地址
    builder.SetPinAddress("switch1", "onoff", "GPIO_1")
    builder.SetPinAddress("switch2", "onoff", "GPIO_2")

    instance, err := builder.Build()
    if err != nil {
        panic(err)
    }

    // instance 包含 3 个 Wire: root, switch1, switch2
}
```

## API 参考

### 全局注册表 API

| 函数 | 说明 |
|------|------|
| `RegisterDevice(template)` | 注册设备模板 |
| `GetDevice(deviceID)` | 获取设备模板 |
| `ListDevices()` | 列出所有设备模板 |
| `ListDevicesByCategory(category)` | 按类别列出设备 |
| `GetCategories()` | 获取所有类别 |

### 设备构建 API

| 函数 | 说明 |
|------|------|
| `NewDeviceBuilder(deviceID, name)` | 创建设备构建器 |
| `QuickBuildDevice(deviceID, name)` | 快速构建设备实例 |
| `BuildDeviceWithAddresses(...)` | 构建并配置地址 |

### DeviceBuilder 方法

| 方法 | 说明 |
|------|------|
| `SetPinAddress(wire, pin, addr)` | 设置单个 Pin 地址 |
| `SetPinAddresses(wire, addrs)` | 批量设置地址 |
| `EnableWire(wireName)` | 启用可选 Wire |
| `DisableWire(wireName)` | 禁用非必需 Wire |
| `Build()` | 构建设备实例 |

## 与其他模块的关系

```
device (设备模板和注册表)
   │
   ├─ 定义标准设备模板 (DeviceTemplate)
   ├─ 提供运行时查询接口
   └─ 构建设备实例 (DeviceInstance)

   ↓ 使用 Cluster 定义

cluster (功能集合定义)
   │
   ├─ OnOff, LevelControl, ColorControl
   ├─ TemperatureMeasurement, HumidityMeasurement
   └─ BasicInformation

   ↓ 包含 Pin 模板

PinTemplate (属性模板)
   │
   ├─ name, type, rw
   └─ default value, tags
```

## 工作流程

```
1. 应用启动时，注册表自动初始化
   ↓
2. 用户可以查询设备类别和模板
   device.ListDevicesByCategory("lighting")
   ↓
3. 选择设备模板创建实例
   builder := device.NewDeviceBuilder("smart_bulb_color", "客厅灯")
   ↓
4. 配置 Pin 物理地址
   builder.SetPinAddress("light", "onoff", "GPIO_1")
   ↓
5. 构建设备实例
   instance := builder.Build()
   ↓
6. 使用实例信息创建数据库记录
   (由 initializer 或其他模块负责)
```

## 最佳实践

1. **优先使用标准设备**：标准设备模板经过验证，功能完整
2. **自定义设备使用 CategoryCustom**：便于区分标准设备和自定义设备
3. **模板注册在初始化时完成**：避免运行时注册影响性能
4. **设备实例名称要有意义**：如 "客厅灯"、"卧室温度传感器" 等
5. **Pin 地址配置清晰**：使用标准命名如 "GPIO_1"、"PWM_2" 等
6. **只读 Pin 可以不配置地址**：某些只读属性可能不需要物理地址

## 注意事项

- ⚠️ **设备模板是只读的**：注册后不应修改设备模板
- ⚠️ **设备 ID 必须唯一**：重复注册会返回错误
- ⚠️ **必需 Wire 不能禁用**：Required=true 的 Wire 总是会被创建
- ⚠️ **线程安全**：注册表是线程安全的，可以并发查询
- ⚠️ **模板 vs 实例**：DeviceTemplate 是模板，DeviceInstance 是实例

## 总结

重新设计的 device 包：

✅ **清晰的职责**：定义标准设备模板 + 运行时注册表
✅ **丰富的标准设备**：25+ 种常见家庭物联网设备
✅ **灵活的自定义**：支持注册自定义设备类型
✅ **易用的构建器**：链式 API 轻松创建设备实例
✅ **完善的测试**：每个功能都有单元测试覆盖
✅ **线程安全**：注册表支持并发访问
✅ **运行时查询**：可作为注册表供应用查询设备信息
