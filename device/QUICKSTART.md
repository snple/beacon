# Device 包快速入门

## 5 分钟上手指南

### 场景 1：我想知道有哪些设备可用

```go
package main

import (
    "fmt"
    "github.com/snple/beacon/device"
)

func main() {
    // 查看所有设备类别
    categories := device.GetCategories()
    fmt.Println("设备类别:", categories)

    // 查看照明类设备
    lights := device.ListDevicesByCategory(device.CategoryLighting)
    for _, dev := range lights {
        fmt.Printf("- %s\n", dev.Name)
    }
}
```

### 场景 2：我想创建一个简单的设备

```go
package main

import (
    "github.com/snple/beacon/device"
)

func main() {
    // 一行代码创建智能灯泡实例
    instance, _ := device.QuickBuildDevice("smart_bulb_onoff", "客厅灯")

    // instance.Wires 包含所有 Wire
    // 每个 Wire.Pins 包含所有 Pin
    // 可以用来创建数据库记录
}
```

### 场景 3：我需要配置 GPIO 地址

```go
package main

import (
    "github.com/snple/beacon/device"
)

func main() {
    // 使用构建器模式
    builder, _ := device.NewDeviceBuilder("smart_bulb_dimmable", "卧室灯")

    // 配置 Pin 的物理地址
    builder.SetPinAddress("light", "onoff", "GPIO_1")
    builder.SetPinAddress("light", "level", "PWM_1")

    // 构建实例
    instance, _ := builder.Build()

    // 使用 instance...
}
```

### 场景 4：我想批量配置多个设备

```go
package main

import (
    "github.com/snple/beacon/device"
)

func main() {
    // 定义地址映射
    addresses := map[string]map[string]string{
        "light": {
            "onoff": "GPIO_1",
            "level": "PWM_1",
        },
    }

    // 一次性构建并配置
    instance, _ := device.BuildDeviceWithAddresses(
        "smart_bulb_dimmable",
        "客厅灯",
        addresses,
    )

    // 使用 instance...
}
```

### 场景 5：我需要自定义设备类型

```go
package main

import (
    "github.com/snple/beacon/device"
)

func main() {
    // 定义自定义设备
    custom := &device.DeviceTemplate{
        ID:       "my_custom_device",
        Name:     "我的自定义设备",
        Category: device.CategoryCustom,
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
        },
    }

    // 注册
    device.RegisterDevice(custom)

    // 后续像标准设备一样使用
    instance, _ := device.QuickBuildDevice("my_custom_device", "实例1")
}
```

## 常用设备 ID 速查

### 照明
- `smart_bulb_onoff` - 开关灯
- `smart_bulb_dimmable` - 调光灯
- `smart_bulb_color` - 彩色灯
- `led_strip` - LED 灯带

### 传感器
- `temp_humi_sensor` - 温湿度传感器
- `temperature_sensor` - 温度传感器
- `motion_sensor` - 人体传感器
- `door_window_sensor` - 门窗传感器

### 开关
- `switch_1gang` - 单路开关
- `switch_2gang` - 双路开关
- `switch_3gang` - 三路开关
- `smart_socket` - 智能插座

### 环境控制
- `smart_curtain` - 智能窗帘
- `ac_controller` - 空调控制器
- `smart_fan` - 智能风扇
- `air_purifier` - 空气净化器

### 安防
- `smart_lock` - 智能门锁
- `smart_camera` - 智能摄像头
- `smoke_sensor` - 烟雾传感器
- `water_leak_sensor` - 水浸传感器

完整列表请参考 [DEVICE_TEMPLATE_README.md](DEVICE_TEMPLATE_README.md)

## API 速查

| 功能 | API |
|------|-----|
| 获取设备模板 | `device.GetDevice("device_id")` |
| 列出所有设备 | `device.ListDevices()` |
| 按类别列出 | `device.ListDevicesByCategory(category)` |
| 获取所有类别 | `device.GetCategories()` |
| 快速构建设备 | `device.QuickBuildDevice(id, name)` |
| 构建器模式 | `device.NewDeviceBuilder(id, name)` |
| 设置 Pin 地址 | `builder.SetPinAddress(wire, pin, addr)` |
| 批量设置地址 | `builder.SetPinAddresses(wire, addrs)` |
| 注册自定义设备 | `device.RegisterDevice(template)` |

## 典型工作流

```
1. 查询可用设备
   device.ListDevicesByCategory("lighting")

   ↓

2. 选择设备模板
   device.GetDevice("smart_bulb_color")

   ↓

3. 创建设备实例
   builder := device.NewDeviceBuilder("smart_bulb_color", "客厅灯")

   ↓

4. 配置 Pin 地址
   builder.SetPinAddress("light", "onoff", "GPIO_1")
   builder.SetPinAddress("light", "level", "PWM_1")

   ↓

5. 构建实例
   instance := builder.Build()

   ↓

6. 使用实例创建数据库记录
   (由应用程序负责)
```

## 下一步

- 📖 完整文档：[DEVICE_TEMPLATE_README.md](DEVICE_TEMPLATE_README.md)
- 💻 示例代码：[examples/main.go](examples/main.go)
- 📝 重构总结：[REFACTOR_SUMMARY.md](REFACTOR_SUMMARY.md)

## 需要帮助？

- 查看 27 种标准设备列表：`device.ListDevices()`
- 运行示例程序：`go run examples/main.go`
- 查看测试用例：`*_test.go` 文件
