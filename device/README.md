# Beacon Device 包 - 设备初始化和配置管理

## 概述

`device` 包提供设备初始化和配置管理功能。这些功能在设备生命周期中**只执行一次**（首次初始化），或在**版本升级时执行合并操作**。

### 核心设计理念

1. **配置文件驱动**：设备定义通过 JSON 配置文件管理，版本号写死在配置中
2. **一次性初始化**：创建数据库后只需执行一次，后续不需要重复执行
3. **智能合并**：升级时保留现有数据，只添加新的 Pin，不删除现有 Pin
4. **Edge 只读**：Edge 服务只提供查询和数据读写，不提供结构修改

## 核心组件

### 1. Initializer（初始化器）
负责创建和管理 Wire、Pin 的底层操作。

```go
init := device.NewInitializer(db)

// 创建 Wire（如果已存在则跳过）
init.CreateWireFromTemplate(ctx, "light1", []string{"OnOff", "LevelControl"})

// 合并 Wire（添加新 Pin，保留现有 Pin）
init.MergeWireFromTemplate(ctx, "light1", []string{"OnOff", "LevelControl", "ColorControl"})
```

### 2. ConfigManager（配置管理器）
提供配置文件的加载、应用、导出功能。

```go
configMgr := device.NewConfigManager(db)

// 从文件加载并应用配置
config, _ := configMgr.LoadFromFile("device.json")
configMgr.ApplyConfig(ctx, config, "create") // 或 "merge"
```

## 使用场景

### 场景 1：首次初始化设备

#### 步骤 1：创建配置文件 `device.json`

```json
{
  "version": "1.0.0",
  "wires": [
    {
      "name": "root",
      "clusters": ["BasicInformation"]
    },
    {
      "name": "light1",
      "clusters": ["OnOff", "LevelControl", "ColorControl"],
      "pins": [
        {"name": "onoff", "addr": "GPIO_1"},
        {"name": "level", "addr": "PWM_1"},
        {"name": "hue", "addr": "PWM_2"},
        {"name": "saturation", "addr": "PWM_3"}
      ]
    }
  ]
}
```

#### 步骤 2：在应用启动时初始化

```go
package main

import (
    "context"
    "github.com/snple/beacon/device"
    "github.com/snple/beacon/edge"
)

func main() {
    // 1. 创建数据库
    db := setupDatabase()

    // 2. 创建 schema
    edge.CreateSchema(db)

    // 3. 初始化设备（只执行一次）
    ctx := context.Background()
    configMgr := device.NewConfigManager(db)
    config, err := configMgr.LoadFromFile("device.json")
    if err != nil {
        panic(err)
    }

    // 使用 create 模式初始化
    err = configMgr.ApplyConfig(ctx, config, "create")
    if err != nil {
        panic(err)
    }

    // 4. 启动 Edge 服务（只读查询 + 数据读写）
    es, _ := edge.Edge(db)
    defer es.Stop()
    es.Start()

    // 后续只使用 Edge 服务进行查询和数据操作
    // 不再需要调用 device 包的初始化功能
}
```

### 场景 2：设备升级（添加新功能）

假设需要升级设备，为 `light1` 添加 `PowerMeasurement` 功能。

#### 步骤 1：更新配置文件 `device_v1.1.json`

```json
{
  "version": "1.1.0",
  "wires": [
    {
      "name": "root",
      "clusters": ["BasicInformation"]
    },
    {
      "name": "light1",
      "clusters": ["OnOff", "LevelControl", "ColorControl", "PowerMeasurement"],
      "pins": [
        {"name": "onoff", "addr": "GPIO_1"},
        {"name": "level", "addr": "PWM_1"},
        {"name": "hue", "addr": "PWM_2"},
        {"name": "saturation", "addr": "PWM_3"},
        {"name": "power", "addr": "ADC_1"}
      ]
    }
  ]
}
```

#### 步骤 2：执行升级脚本

```go
package main

import (
    "context"
    "github.com/snple/beacon/device"
)

func upgradeDevice(db *bun.DB) error {
    ctx := context.Background()
    configMgr := device.NewConfigManager(db)

    // 加载新版本配置
    newConfig, err := configMgr.LoadFromFile("device_v1.1.json")
    if err != nil {
        return err
    }

    // 使用 merge 模式升级（保留现有 Pin，添加新 Pin）
    return configMgr.ApplyConfig(ctx, newConfig, "merge")
}
```

**merge 模式的行为：**
- ✅ 保留所有现有的 Pin
- ✅ 添加新 Cluster 的 Pin（如 `power`）
- ✅ 更新 Wire 的 Clusters 字段
- ❌ 不删除任何现有数据

### 场景 3：编程方式初始化（不使用配置文件）

```go
package main

import (
    "context"
    "github.com/snple/beacon/device"
)

func initDeviceProgrammatically(db *bun.DB) error {
    ctx := context.Background()
    init := device.NewInitializer(db)

    // 使用预构建模板
    devices := []*device.SimpleBuildResult{
        device.BuildRootWire(),
        device.BuildColorLightWire("light1"),
        device.BuildTempHumiSensorWire("sensor1"),
    }

    for _, dev := range devices {
        err := init.CreateWireFromBuilder(ctx, dev)
        if err != nil {
            return err
        }
    }

    return nil
}
```

### 场景 4：导出当前配置

如果需要将数据库中的设备配置导出为配置文件：

```go
package main

import (
    "context"
    "github.com/snple/beacon/device"
)

func exportConfig(db *bun.DB) error {
    ctx := context.Background()
    configMgr := device.NewConfigManager(db)

    // 导出配置
    config, err := configMgr.ExportConfig(ctx, "1.0.0")
    if err != nil {
        return err
    }

    // 保存到文件
    return configMgr.SaveToFile(config, "exported_device.json")
}
```

## API 参考

### Initializer

| 方法 | 说明 |
|------|------|
| `CreateWireFromTemplate` | 从 Cluster 模板创建 Wire，如果已存在则跳过 |
| `CreateWireFromBuilder` | 从 Builder 创建 Wire |
| `MergeWireFromTemplate` | 合并 Wire 配置（升级用），保留现有 Pin |
| `DeleteWire` | 删除 Wire 及其所有 Pin |
| `UpdatePinAddress` | 更新 Pin 的物理地址 |

### ConfigManager

| 方法 | 说明 |
|------|------|
| `LoadFromFile` | 从 JSON 文件加载配置 |
| `ApplyConfig` | 应用配置（mode: "create" 或 "merge"） |
| `ExportConfig` | 从数据库导出配置 |
| `SaveToFile` | 保存配置到 JSON 文件 |

### 预构建设备模板

| 函数 | 说明 |
|------|------|
| `BuildRootWire()` | 根 Wire（BasicInformation） |
| `BuildOnOffLightWire(name)` | 开关灯 |
| `BuildDimmableLightWire(name)` | 可调光灯 |
| `BuildColorLightWire(name)` | 彩色灯 |
| `BuildTemperatureSensorWire(name)` | 温度传感器 |
| `BuildTempHumiSensorWire(name)` | 温湿度传感器 |

## 配置文件格式

```json
{
  "version": "1.0.0",           // 设备模型版本（写死在配置中）
  "wires": [                     // Wire 列表
    {
      "name": "wire_name",       // Wire 名称
      "clusters": ["Cluster1"],  // Cluster 列表
      "pins": [                  // 可选：Pin 地址配置
        {
          "name": "pin_name",    // Pin 名称
          "addr": "GPIO_1"       // 物理地址
        }
      ]
    }
  ]
}
```

## 标准 Cluster 列表

| Cluster | 描述 | Pin |
|---------|------|-----|
| `BasicInformation` | 设备基本信息 | vendor_name, product_name, serial_number |
| `OnOff` | 开关控制 | onoff |
| `LevelControl` | 亮度控制 | level, min_level, max_level |
| `ColorControl` | 颜色控制 | hue, saturation |
| `TemperatureMeasurement` | 温度测量 | temperature, temp_min, temp_max |
| `HumidityMeasurement` | 湿度测量 | humidity |

## 工作流程

```
首次部署:
┌─────────────┐
│ device.json │ ──> device.ConfigManager.ApplyConfig(mode="create")
└─────────────┘                    │
                                   ▼
                          ┌──────────────────┐
                          │  SQLite Database │
                          │  (Wire + Pin)    │
                          └──────────────────┘
                                   │
                                   ▼
                          ┌──────────────────┐
                          │  Edge Service    │  (只读查询 + 数据读写)
                          │  - View/List     │
                          │  - GetValue/     │
                          │    SetValue      │
                          └──────────────────┘

版本升级:
┌───────────────────┐
│ device_v1.1.json  │ ──> device.ConfigManager.ApplyConfig(mode="merge")
└───────────────────┘                    │
                                         ▼
                               ┌──────────────────┐
                               │  合并操作:       │
                               │  - 保留现有 Pin  │
                               │  - 添加新 Pin    │
                               │  - 更新 Clusters │
                               └──────────────────┘
```

## 最佳实践

1. **使用配置文件**：推荐使用 JSON 配置文件定义设备，便于版本控制
2. **版本管理**：每次修改设备定义时更新 `version` 字段
3. **首次初始化用 create**：第一次部署使用 `mode="create"`
4. **升级时用 merge**：版本升级使用 `mode="merge"`，保护现有数据
5. **一次性执行**：初始化代码在设备生命周期中只需执行一次
6. **Edge 只做查询**：运行时只使用 Edge 服务进行查询和数据操作

## 注意事项

- ⚠️ **初始化只执行一次**：设备初始化在首次部署时执行，后续运行不需要重复执行
- ⚠️ **升级时使用 merge**：升级时必须使用 merge 模式，避免数据丢失
- ⚠️ **版本号管理**：版本号写死在配置文件中，不存储在数据库
- ⚠️ **Edge 服务只读**：运行时的 Edge 服务不提供结构修改功能
- ⚠️ **配置文件备份**：配置文件是唯一的设备定义来源，需要妥善保管

## 总结

通过将设备初始化逻辑从 Edge 包分离到 device 包，实现了：

- ✅ **关注点分离**：device 负责初始化，edge 负责运行时操作
- ✅ **简化 Edge**：Edge 服务保持简洁，只提供查询和数据操作
- ✅ **配置驱动**：设备定义通过配置文件管理，易于维护
- ✅ **安全升级**：merge 模式确保升级时不丢失现有数据
- ✅ **一次性执行**：初始化代码只在需要时执行，不影响运行时性能
