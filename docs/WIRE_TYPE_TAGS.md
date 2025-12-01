# Wire Type 和 Tags 字段说明

## 概述

在 Wire 模型中新增了两个字段，用于更好地标识和分类设备端点：

- **Type**: 设备类型/型号标识
- **Tags**: 灵活的标签系统，支持多个键值对

## 字段定义

### Type 字段

**用途**: 明确标识 Wire 的设备类型或型号

**示例**:
```
Type = "DimmableLight"      // 可调光灯
Type = "ColorLight"         // RGB 彩色灯
Type = "TempHumiditySensor" // 温湿度传感器
Type = "ThreeGangSwitch"    // 三联开关
Type = "MotionSensor"       // 人体感应传感器
```

### Tags 字段

**用途**: 提供额外的分类和属性信息

**格式**: `key:value,key:value,...`

**常用 Keys**:
```
category  - 设备大类 (light, sensor, switch, actuator, etc.)
feature   - 功能特性 (dimmable, color, motion, etc.)
vendor    - 厂商名称
model     - 型号
location  - 位置 (indoor, outdoor, bedroom, kitchen, etc.)
room      - 房间
position  - 安装位置 (ceiling, wall, floor, etc.)
function  - 功能描述
protocol  - 通信协议 (zigbee, wifi, ble, etc.)
```

**示例**:
```
Tags = "category:light,feature:dimmable,room:bedroom"
Tags = "category:sensor,function:temperature,location:outdoor"
Tags = "category:switch,gangs:3,position:wall"
```

## 与 Clusters 的关系

这三个字段在 Wire 中各司其职，相互补充：

| 字段 | 用途 | 示例 |
|------|------|------|
| **Clusters** | 定义功能结构（包含哪些 Pin） | `"OnOff,LevelControl"` |
| **Type** | 标识设备类型/型号 | `"DimmableLight"` |
| **Tags** | 额外的分类和属性信息 | `"category:light,room:bedroom"` |

**完整示例**:
```go
Wire{
    Name:     "bedroom_main_light",
    Type:     "DimmableLight",
    Tags:     "category:light,feature:dimmable,room:bedroom,position:ceiling",
    Clusters: "OnOff,LevelControl",
}
```

## 使用场景

### 1. 设备识别

通过 Type 字段快速识别设备类型：

```go
if wire.Type == "DimmableLight" {
    // 处理可调光灯的逻辑
}
```

### 2. 设备查询

根据 Type 或 Tags 查询特定类型的设备：

```go
// 查询所有可调光灯
wires, err := db.NewSelect().
    Model((*model.Wire)(nil)).
    Where("type = ?", "DimmableLight").
    Scan(ctx)

// 查询所有灯光设备
wires, err := db.NewSelect().
    Model((*model.Wire)(nil)).
    Where("tags LIKE ?", "%category:light%").
    Scan(ctx)

// 查询卧室的所有设备
wires, err := db.NewSelect().
    Model((*model.Wire)(nil)).
    Where("tags LIKE ?", "%room:bedroom%").
    Scan(ctx)
```

### 3. 设备分组

按房间、位置或功能对设备进行分组：

```go
// 按房间分组
SELECT * FROM wire WHERE tags LIKE '%room:bedroom%'
SELECT * FROM wire WHERE tags LIKE '%room:livingroom%'

// 按类别分组
SELECT * FROM wire WHERE tags LIKE '%category:light%'
SELECT * FROM wire WHERE tags LIKE '%category:sensor%'
```

### 4. UI 展示

在用户界面中显示设备信息：

```go
// 显示设备类型
fmt.Printf("设备类型: %s\n", wire.Type)

// 解析并显示标签
tags := parseWireTags(wire.Tags)
fmt.Printf("类别: %s\n", tags["category"])
fmt.Printf("房间: %s\n", tags["room"])
fmt.Printf("位置: %s\n", tags["position"])
```

## 代码示例

### 创建带类型和标签的 Wire

```go
// 方式 1: 使用 WireBuilder
result := NewWireBuilder("main_light").
    WithClusters("OnOff", "LevelControl").
    Build()

result.Wire.Type = "DimmableLight"
result.Wire.Tags = "category:light,feature:dimmable,room:bedroom,position:ceiling"

err := init.CreateWireFromBuilder(ctx, result)
```

```go
// 方式 2: 直接创建
wire := &model.Wire{
    ID:       util.RandomID(),
    Name:     "temp_sensor",
    Type:     "TemperatureSensor",
    Tags:     "category:sensor,function:temperature,location:outdoor",
    Clusters: "TemperatureMeasurement",
    Updated:  time.Now(),
}

_, err := db.NewInsert().Model(wire).Exec(ctx)
```

### 查询和过滤

```go
// 查询特定类型的设备
func FindWiresByType(ctx context.Context, db *bun.DB, wireType string) ([]model.Wire, error) {
    var wires []model.Wire
    err := db.NewSelect().
        Model(&wires).
        Where("type = ?", wireType).
        Scan(ctx)
    return wires, err
}

// 查询包含特定标签的设备
func FindWiresByTag(ctx context.Context, db *bun.DB, tag string) ([]model.Wire, error) {
    var wires []model.Wire
    err := db.NewSelect().
        Model(&wires).
        Where("tags LIKE ?", "%"+tag+"%").
        Scan(ctx)
    return wires, err
}
```

### Tags 解析工具

```go
// ParseWireTags 解析 Wire 的 Tags 字段
func ParseWireTags(tags string) map[string]string {
    result := make(map[string]string)
    if tags == "" {
        return result
    }

    pairs := strings.Split(tags, ",")
    for _, pair := range pairs {
        kv := strings.SplitN(pair, ":", 2)
        if len(kv) == 2 {
            result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
        }
    }

    return result
}

// HasTag 检查 Wire 是否包含指定的 tag
func HasTag(wire *model.Wire, key, value string) bool {
    searchTag := key + ":" + value
    return strings.Contains(wire.Tags, searchTag)
}
```

## 预定义的设备类型

建议使用以下标准的 Type 值：

### 灯光设备
- `OnOffLight` - 开关灯
- `DimmableLight` - 可调光灯
- `ColorLight` - RGB 彩色灯
- `ColorTemperatureLight` - 色温可调灯

### 传感器
- `TemperatureSensor` - 温度传感器
- `HumiditySensor` - 湿度传感器
- `TempHumiditySensor` - 温湿度传感器
- `MotionSensor` - 人体感应传感器
- `DoorSensor` - 门磁传感器
- `SmokeSensor` - 烟雾传感器

### 开关
- `OnOffSwitch` - 单联开关
- `TwoGangSwitch` - 双联开关
- `ThreeGangSwitch` - 三联开关

### 执行器
- `Relay` - 继电器
- `Valve` - 阀门控制器
- `Motor` - 电机控制器

## 数据库迁移

如果需要为现有数据库添加这两个字段，请执行以下 SQL：

```sql
-- SQLite
ALTER TABLE wire ADD COLUMN type TEXT DEFAULT '';
ALTER TABLE wire ADD COLUMN tags TEXT DEFAULT '';

-- 为现有数据设置默认值（可选）
UPDATE wire SET type = 'Unknown' WHERE type = '';
UPDATE wire SET tags = 'category:unknown' WHERE tags = '';
```

## 最佳实践

1. **Type 字段**
   - 使用驼峰命名（PascalCase）
   - 保持简洁明了
   - 使用标准的预定义值

2. **Tags 字段**
   - 使用小写字母和下划线
   - 格式保持一致：`key:value,key:value`
   - 不要在 value 中使用逗号或冒号
   - 使用有意义的 key 名称

3. **查询优化**
   - 如果经常按 Type 查询，考虑添加索引
   - Tags 使用 LIKE 查询，对于大数据集可能较慢
   - 考虑使用全文搜索或专门的标签表

## 参考

更多示例请查看：
- `device/wire_type_tags_example.go` - 完整的使用示例
- `device/cluster.go` - Cluster 定义和注册
- `device/builder.go` - WireBuilder 实现
