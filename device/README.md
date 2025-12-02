# Device 包 - Builder 链式设备定义

## 设计理念

**大道至简** - 使用流畅的 Builder 链式 API 定义设备

## 快速开始

### 使用标准设备

```go
import "github.com/snple/beacon/device"

// 获取标准设备
dev, ok := device.Get("smart_bulb_onoff")

// 按标签查询
lights := device.ByTag("lighting")

// 列出所有设备
all := device.List()

// 获取所有标签
tags := device.AllTags()
```

### 定义自定义设备

```go
// 简单设备
myDevice := device.New("my_device", "我的设备").
    Tags("custom", "demo").
    Wire("control").
        Pin("switch", dt.TypeBool, device.RW).
        Pin("value", dt.TypeI32, device.RO).
    Done()

// 复杂设备：使用预定义 Pin 集合
smartLight := device.New("my_smart_light", "我的智能灯").
    Tags("lighting", "smart_home").
    Wire("root").
        Pins(device.BasicInfoPins).
    Wire("light").
        WireTags("control", "light").
        Pins(device.OnOffPins).
        Pins(device.LevelControlPins).
        Pins(device.ColorControlPins).
    Done()

// 注册自定义设备
device.Register(myDevice)
```

### 多 Wire 设备

```go
switch3Gang := device.New("switch_3gang", "三路开关").
    Tags("switch", "smart_home").
    Wire("switch1").
        Pin("onoff", dt.TypeBool, device.RW).
    Wire("switch2").
        Pin("onoff", dt.TypeBool, device.RW).
    Wire("switch3").
        Pin("onoff", dt.TypeBool, device.RW).
    Done()
```

## 核心概念

### 数据结构（与 storage 兼容）

```
Device (设备模板)
  ├─ ID, Name, Desc
  ├─ Tags[] (设备标签，用于分类和查询)
  └─ Wires[] (端点模板)
       ├─ Name, Desc, Type
       ├─ Tags[] (Wire 标签)
       └─ Pins[] (属性模板)
            ├─ Name, Desc, Type, Rw
            ├─ Default (默认值)
            └─ Tags[] (Pin 标签数组)
```

**设计理念：**
- Device 包定义的是"设备模板"，不是实例
- 数据结构与 `core/storage` 和 `edge/storage` 中的 Node/Wire/Pin 兼容
- Tags 在所有层级（Device/Wire/Pin）都使用数组，便于多标签查询
- Storage.Node 通过 Device 字段指向设备模板 ID

### Builder API

**设备级别：**
- `New(id, name)` - 创建设备构建器
- `.Tags(tags...)` - 设置设备标签
- `.Desc(desc)` - 设置设备描述
- `.Done()` - 完成构建

**Wire 级别：**
- `.Wire(name)` - 创建新 Wire
- `.WireType(type)` - 设置 Wire 类型
- `.WireDesc(desc)` - 设置 Wire 描述
- `.WireTags(tags...)` - 设置 Wire 标签

**Pin 级别：**
- `.Pin(name, type, rw)` - 添加 Pin
- `.Pins([]Pin)` - 批量添加 Pin
- `.PinWithDesc(name, desc, type, rw)` - 添加带描述的 Pin
- `.PinFull(name, desc, type, rw, default, tags)` - 添加完整配置的 Pin

### 注册表 API

- `Register(device)` - 注册设备
- `Get(id)` - 获取设备（返回 Device, bool）
- `List()` - 列出所有设备
- `ByTag(tag)` - 按标签查询设备
- `AllTags()` - 获取所有标签

### 辅助函数

- `device.ToStorageNode()` - 转换为 storage 兼容的格式
- `ClonePins(pins)` - 深拷贝 Pin 列表
- `CloneWires(wires)` - 深拷贝 Wire 列表

### 预定义 Pin 集合

- `OnOffPins` - 开关控制
- `LevelControlPins` - 级别控制
- `ColorControlPins` - 颜色控制
- `TemperaturePins` - 温度测量
- `HumidityPins` - 湿度测量
- `BasicInfoPins` - 基本信息

## 标准设备

| ID | 名称 | 类别 |
|----|------|------|
| `smart_bulb_onoff` | 智能开关灯泡 | lighting |
| `smart_bulb_dimmable` | 可调光智能灯泡 | lighting |
| `smart_bulb_color` | 彩色智能灯泡 | lighting |
| `temp_humi_sensor` | 温湿度传感器 | sensor |
| `switch_2gang` | 双路智能开关 | switch |
| `smart_socket` | 智能插座 | socket |

## 特点

✅ **流畅的链式 API** - 代码即文档
✅ **极简设计** - 只有核心概念
✅ **类型安全** - 编译时检查
✅ **可扩展** - 轻松添加自定义设备
✅ **零依赖** - 纯 Go 实现
✅ **线程安全** - 并发访问无忧
