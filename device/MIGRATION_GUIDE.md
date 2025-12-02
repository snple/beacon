# Builder API 迁移指南

## 概述

`builder.go` 现在作为**兼容层**存在，提供对旧版 `SimpleBuildResult` API 的支持。新代码应使用 `DeviceBuilder` API。

## 为什么要迁移？

### 旧版 API (builder.go) 的限制
- ❌ 只能构建单个 Wire，不能构建完整设备
- ❌ 没有设备模板概念，需要手动组合 Cluster
- ❌ 缺少设备类型标识，难以管理
- ❌ 不支持设备注册表查询

### 新版 API (device_builder.go) 的优势
- ✅ 27+ 种标准设备模板开箱即用
- ✅ 完整的设备模型（多个 Wire）
- ✅ 设备注册表支持查询和分类
- ✅ 更清晰的类型系统和文档

## 迁移对照表

### 场景 1: 创建根 Wire

**旧代码:**
```go
result := device.BuildRootWire()
// result.Wire.Name = "root"
// result.Pins = []{vendor_name, product_name, serial_number}
```

**新代码:**
```go
// 方式 1: 使用网关设备模板
instance, _ := device.QuickBuildDevice("smart_gateway", "root")
// instance.Wires[0].Name = "root"
// instance.Wires[0].Pins = []{vendor_name, product_name, serial_number}

// 方式 2: 使用设备构建器
builder, _ := device.NewDeviceBuilder("smart_gateway", "mydevice")
instance, _ := builder.Build()
rootWire := instance.Wires[0] // root wire
```

### 场景 2: 创建开关灯

**旧代码:**
```go
result := device.BuildOnOffLightWire("light1")
// result.Wire.Name = "light1"
// result.Pins = []{onoff}
```

**新代码:**
```go
instance, _ := device.QuickBuildDevice("smart_bulb_onoff", "light1")
// instance.Wires 包含 root + light
// lightWire := instance.Wires[1] // light wire
```

### 场景 3: 创建调光灯

**旧代码:**
```go
result := device.BuildDimmableLightWire("light2")
// result.Wire 包含 OnOff + LevelControl
```

**新代码:**
```go
instance, _ := device.QuickBuildDevice("smart_bulb_dimmable", "light2")
// instance.Wires 包含 root + light (OnOff + LevelControl)
```

### 场景 4: 创建彩色灯

**旧代码:**
```go
result := device.BuildColorLightWire("light3")
// result.Wire 包含 OnOff + LevelControl + ColorControl
```

**新代码:**
```go
instance, _ := device.QuickBuildDevice("smart_bulb_color", "light3")
// instance.Wires 包含 root + light (OnOff + LevelControl + ColorControl)
```

### 场景 5: 创建传感器

**旧代码:**
```go
result := device.BuildTemperatureSensorWire("sensor1")
// result.Wire 包含 TemperatureMeasurement
```

**新代码:**
```go
instance, _ := device.QuickBuildDevice("temperature_sensor", "sensor1")
// instance.Wires 包含 root + sensor (TemperatureMeasurement)
```

### 场景 6: 创建温湿度传感器

**旧代码:**
```go
result := device.BuildTempHumiSensorWire("sensor2")
// result.Wire 包含 TemperatureMeasurement + HumidityMeasurement
```

**新代码:**
```go
instance, _ := device.QuickBuildDevice("temp_humi_sensor", "sensor2")
// instance.Wires 包含 root + sensor (Temperature + Humidity)
```

### 场景 7: 配置 Pin 地址

**旧代码:**
```go
result := device.BuildColorLightWire("light")
// 手动遍历设置地址
for _, pin := range result.Pins {
    if pin.Name == "onoff" {
        pin.Addr = "GPIO_1"
    } else if pin.Name == "level" {
        pin.Addr = "PWM_1"
    }
}
```

**新代码:**
```go
builder, _ := device.NewDeviceBuilder("smart_bulb_color", "light")
builder.SetPinAddress("light", "onoff", "GPIO_1")
builder.SetPinAddress("light", "level", "PWM_1")
builder.SetPinAddress("light", "hue", "PWM_2")
builder.SetPinAddress("light", "saturation", "PWM_3")
instance, _ := builder.Build()

// 或者批量设置
addresses := map[string]map[string]string{
    "light": {
        "onoff": "GPIO_1",
        "level": "PWM_1",
        "hue":   "PWM_2",
        "saturation": "PWM_3",
    },
}
instance, _ := device.BuildDeviceWithAddresses("smart_bulb_color", "light", addresses)
```

### 场景 8: 自定义 Wire

**旧代码:**
```go
customCluster := &device.Cluster{
    ID: 0x9999,
    Name: "MyCluster",
    Pins: []device.PinTemplate{...},
}
result := device.NewWireBuilder("custom").
    WithCustomCluster(customCluster).
    Build()
```

**新代码:**
```go
// 1. 注册自定义 Cluster
customCluster := &device.Cluster{
    ID: 0x9999,
    Name: "MyCluster",
    Pins: []device.PinTemplate{...},
}
device.RegisterCluster(customCluster)

// 2. 创建自定义设备模板
customDevice := &device.DeviceTemplate{
    ID: "my_custom_device",
    Name: "我的自定义设备",
    Category: device.CategoryCustom,
    Wires: []device.WireTemplate{
        {Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
        {Name: "custom", Clusters: []string{"MyCluster"}, Required: true},
    },
}
device.RegisterDevice(customDevice)

// 3. 使用自定义设备
instance, _ := device.QuickBuildDevice("my_custom_device", "instance1")
```

## 结构对比

### 旧版结构

```go
SimpleBuildResult
├── Wire: *BuilderWire
│   ├── Name: string
│   ├── Type: string
│   └── Clusters: string (逗号分隔)
└── Pins: []*BuilderPin
    ├── Name: string
    ├── Type: uint32
    ├── Addr: string
    └── Rw: int32
```

### 新版结构

```go
DeviceInstance
├── DeviceID: string (设备模板 ID)
├── InstanceName: string (实例名称)
├── Template: *DeviceTemplate (设备模板引用)
└── Wires: []*WireInstance (多个 Wire)
    ├── Name: string
    ├── Clusters: []string (数组)
    └── Pins: []*PinInstance
        ├── Name: string
        ├── Type: uint32
        ├── Rw: int32
        └── Addr: string
```

## 关键差异

| 特性 | 旧版 API | 新版 API |
|------|---------|---------|
| 构建单位 | 单个 Wire | 完整设备（多个 Wire） |
| 设备模板 | 无 | 27+ 种标准模板 |
| 类型标识 | 无 | DeviceID + Category |
| 注册表查询 | 不支持 | 完整支持 |
| Cluster 格式 | 逗号分隔字符串 | 字符串数组 |
| 必需字段 | 不明确 | Required 标识 |
| 地址配置 | 手动修改 | 构建器 API |

## 迁移步骤

### 步骤 1: 识别使用场景

查找代码中使用旧版 API 的位置：
```bash
grep -r "BuildRootWire\|BuildOnOffLightWire\|BuildDimmableLightWire\|BuildColorLightWire\|BuildTemperatureSensorWire\|BuildTempHumiSensorWire\|NewWireBuilder" .
```

### 步骤 2: 确定对应的设备模板

| 旧函数 | 新设备模板 ID |
|--------|--------------|
| `BuildRootWire()` | `smart_gateway` |
| `BuildOnOffLightWire()` | `smart_bulb_onoff` |
| `BuildDimmableLightWire()` | `smart_bulb_dimmable` |
| `BuildColorLightWire()` | `smart_bulb_color` |
| `BuildTemperatureSensorWire()` | `temperature_sensor` |
| `BuildTempHumiSensorWire()` | `temp_humi_sensor` |

### 步骤 3: 替换代码

**示例迁移 (edge/seed.go):**

**旧代码:**
```go
result := device.BuildRootWire()

// 构建 Pins
pins := make([]storage.Pin, 0, len(result.Pins))
for _, builderPin := range result.Pins {
    pin := storage.Pin{
        ID:   util.RandomID(),
        Name: builderPin.Name,
        Type: builderPin.Type,
        Rw:   builderPin.Rw,
    }
    pins = append(pins, pin)
}

// 构建 Wire
wire := storage.Wire{
    ID:       util.RandomID(),
    Name:     result.Wire.Name,
    Clusters: parseClusterString(result.Wire.Clusters),
    Pins:     pins,
}
```

**新代码:**
```go
instance, _ := device.QuickBuildDevice("smart_gateway", "root")

// 获取 root wire
rootWire := instance.Wires[0]

// 构建 Pins
pins := make([]storage.Pin, 0, len(rootWire.Pins))
for _, pinInstance := range rootWire.Pins {
    pin := storage.Pin{
        ID:   util.RandomID(),
        Name: pinInstance.Name,
        Type: pinInstance.Type,
        Rw:   pinInstance.Rw,
        Addr: pinInstance.Addr,
    }
    pins = append(pins, pin)
}

// 构建 Wire
wire := storage.Wire{
    ID:       util.RandomID(),
    Name:     rootWire.Name,
    Clusters: rootWire.Clusters, // 已经是 []string
    Pins:     pins,
}
```

### 步骤 4: 测试验证

运行测试确保迁移正确：
```bash
go test ./...
```

## 兼容性说明

### 当前状态
- ✅ 旧版 API 仍然可用（builder.go 作为兼容层）
- ✅ edge/seed.go 等现有代码继续工作
- ✅ 新旧 API 可以共存

### 建议
- 🔄 **新代码**: 使用 `DeviceBuilder` API
- 🔄 **现有代码**: 可以继续使用，或逐步迁移
- ⚠️ **长期计划**: 旧版 API 可能在未来版本中标记为废弃

## 完整示例

### 示例 1: 创建完整的智能灯设备

**旧代码 (只能创建 Wire):**
```go
// 只能创建单个 Wire
lightWire := device.BuildColorLightWire("light")
// 缺少 root Wire，需要单独创建
```

**新代码 (创建完整设备):**
```go
// 一次性创建完整设备（包含 root + light）
instance, _ := device.QuickBuildDevice("smart_bulb_color", "客厅灯")

// 配置地址
builder, _ := device.NewDeviceBuilder("smart_bulb_color", "客厅灯")
builder.SetPinAddresses("light", map[string]string{
    "onoff": "GPIO_1",
    "level": "PWM_1",
    "hue":   "PWM_2",
    "saturation": "PWM_3",
})
instance, _ := builder.Build()

// instance.Wires[0] = root wire (BasicInformation)
// instance.Wires[1] = light wire (OnOff + LevelControl + ColorControl)
```

### 示例 2: 查询可用设备

**旧代码 (不支持):**
```go
// 无法查询可用设备类型
// 需要查阅文档或源代码
```

**新代码:**
```go
// 列出所有照明设备
lights := device.ListDevicesByCategory(device.CategoryLighting)
for _, dev := range lights {
    fmt.Printf("- %s (%s)\n", dev.Name, dev.ID)
}

// 获取设备详情
template := device.GetDevice("smart_bulb_color")
fmt.Printf("设备: %s\n", template.Name)
fmt.Printf("包含 Wire:\n")
for _, wire := range template.Wires {
    fmt.Printf("  - %s: %v\n", wire.Name, wire.Clusters)
}
```

## 需要帮助？

- 📖 查看新版 API 文档: [DEVICE_TEMPLATE_README.md](DEVICE_TEMPLATE_README.md)
- 🚀 快速入门指南: [QUICKSTART.md](QUICKSTART.md)
- 💻 示例代码: [examples/main.go](examples/main.go)
- 📝 重构总结: [REFACTOR_SUMMARY.md](REFACTOR_SUMMARY.md)

## 总结

| 维度 | 旧版 API | 新版 API |
|------|---------|---------|
| 易用性 | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| 功能完整性 | ⭐⭐ | ⭐⭐⭐⭐⭐ |
| 类型安全 | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| 文档完整性 | ⭐⭐ | ⭐⭐⭐⭐⭐ |
| 推荐程度 | 仅兼容 | ✅ 强烈推荐 |

**建议**: 新项目直接使用 `DeviceBuilder` API，现有项目可以逐步迁移。
