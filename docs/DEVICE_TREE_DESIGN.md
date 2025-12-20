# Edge 设备树机制设计方案

## 当前架构分析

### 1. PinWrite 处理流程

当前 Edge 接收 Core 的 PinWrite 命令流程如下：

```
Core (SetPinWrite)
  ↓ Queen MQTT
QueenUpService.handlePinWrite/handlePinWriteBatch
  ↓
setPinWrite (解析 nodeID.wireName.pinName)
  ↓
EdgeService.SetPinWrite (验证权限和类型)
  ↓
Storage.SetPinWrite (保存到 Badger)
  ↓
【缺失】硬件执行层 ❌
```

**问题：**
- ✅ 设备定义（Device Template）已完善
- ✅ 数据验证和存储完整
- ❌ **缺少硬件执行抽象层**
- ❌ 没有 Driver/Handler 机制将 PinWrite 映射到实际硬件操作

### 2. 当前设备定义（简单设备树）

```go
// device/devices.go 中的预定义设备
SmartBulb = DeviceBuilder("bulb", "智能灯泡").
    Wire(WireBuilder("ctrl").
        Pin(OnOffPin).        // 开关
        Pin(DimPin).          // 亮度
        Pin(CCTPin),          // 色温
    ).Done()
```

这已经是一个**简单的设备树**：
- Device → Wire → Pin 的层级结构
- 包含 Type、Rw、Range、Unit 等元数据
- 类似 Linux Device Tree 的声明式配置

## 设计方案：设备树 + Driver 机制

### 核心思想

借鉴 Linux 设备树机制，增加以下层次：

```
Device Template (设备树声明)
    ↓
Wire Driver (驱动层)
    ↓
Hardware Abstraction (硬件抽象)
    ↓
Physical Device (实际硬件)
```

### 1. Wire Driver 接口设计

```go
// edge/driver/driver.go

// WireDriver 是 Wire 级别的驱动接口
// 每个 Wire 对应一个物理设备或协议端点
type WireDriver interface {
    // Initialize 初始化驱动（如打开串口、连接Modbus等）
    Initialize(ctx context.Context, config WireConfig) error

    // Close 关闭驱动
    Close() error

    // OnPinWrite 处理 Pin 写入（核心方法）
    // 将逻辑 Pin 映射到硬件操作
    OnPinWrite(ctx context.Context, pinID string, value nson.Value) error

    // ReadPins 定期读取 Pin 值（可选，用于传感器）
    ReadPins(ctx context.Context) ([]dt.PinValue, error)

    // GetInfo 获取驱动信息
    GetInfo() DriverInfo
}

// WireConfig Wire 配置
type WireConfig struct {
    WireID   string            // Wire ID
    WireName string            // Wire 名称
    WireType string            // Wire 类型（用于匹配驱动）
    Pins     []dt.Pin          // Pin 列表
    Options  map[string]string // 自定义选项（如串口路径、Modbus地址等）
}

// DriverInfo 驱动信息
type DriverInfo struct {
    Name        string   // 驱动名称
    Version     string   // 驱动版本
    SupportTypes []string // 支持的 Wire.Type
}
```

### 2. Driver Registry（驱动注册表）

```go
// edge/driver/registry.go

var driverRegistry = make(map[string]DriverFactory)

// DriverFactory 驱动工厂函数
type DriverFactory func() WireDriver

// RegisterDriver 注册驱动
func RegisterDriver(wireType string, factory DriverFactory) {
    driverRegistry[wireType] = factory
}

// GetDriver 根据 Wire.Type 获取驱动实例
func GetDriver(wireType string) (WireDriver, error) {
    factory, ok := driverRegistry[wireType]
    if !ok {
        return nil, fmt.Errorf("no driver for wire type: %s", wireType)
    }
    return factory(), nil
}
```

### 3. 内置驱动示例

#### Modbus RTU 驱动

```go
// edge/driver/modbus_rtu.go

func init() {
    RegisterDriver("modbus_rtu", func() WireDriver {
        return &ModbusRTUDriver{}
    })
}

type ModbusRTUDriver struct {
    client  *modbus.RTUClient
    config  WireConfig
    addrMap map[string]uint16 // pinID → Modbus地址
}

func (d *ModbusRTUDriver) Initialize(ctx context.Context, cfg WireConfig) error {
    // 从 Options 获取串口配置
    port := cfg.Options["port"]       // /dev/ttyUSB0
    slave := cfg.Options["slave_id"]  // 从站地址

    // 建立 Modbus 连接
    handler := modbus.NewRTUClientHandler(port)
    handler.BaudRate = 9600
    handler.SlaveId = byte(slave)
    d.client = modbus.NewClient(handler)

    // 构建 Pin → Modbus 地址映射
    d.addrMap = make(map[string]uint16)
    for _, pin := range cfg.Pins {
        if addr := pin.Addr; addr != "" {
            // Addr 格式: "40001" (Holding Register)
            d.addrMap[pin.ID], _ = parseModbusAddr(addr)
        }
    }

    return handler.Connect()
}

func (d *ModbusRTUDriver) OnPinWrite(ctx context.Context, pinID string, value nson.Value) error {
    addr, ok := d.addrMap[pinID]
    if !ok {
        return fmt.Errorf("no modbus address for pin: %s", pinID)
    }

    // 根据数据类型写入
    switch value.DataType() {
    case nson.DataTypeBOOL:
        // 写单个线圈
        return d.client.WriteSingleCoil(addr, boolToUint16(value.Bool()))
    case nson.DataTypeU16:
        // 写单个寄存器
        return d.client.WriteSingleRegister(addr, value.U16())
    case nson.DataTypeI32:
        // 写多个寄存器
        bytes := int32ToBytes(value.I32())
        return d.client.WriteMultipleRegisters(addr, 2, bytes)
    default:
        return fmt.Errorf("unsupported type for modbus: %v", value.DataType())
    }
}

func (d *ModbusRTUDriver) ReadPins(ctx context.Context) ([]dt.PinValue, error) {
    var values []dt.PinValue

    for pinID, addr := range d.addrMap {
        // 读取寄存器
        data, err := d.client.ReadHoldingRegisters(addr, 1)
        if err != nil {
            continue
        }

        values = append(values, dt.PinValue{
            ID:      pinID,
            Value:   nson.U16(binary.BigEndian.Uint16(data)),
            Updated: time.Now(),
        })
    }

    return values, nil
}
```

#### GPIO 驱动（树莓派等）

```go
// edge/driver/gpio.go

func init() {
    RegisterDriver("gpio", func() WireDriver {
        return &GPIODriver{}
    })
}

type GPIODriver struct {
    pins map[string]*gpio.Pin // pinID → GPIO Pin
}

func (d *GPIODriver) Initialize(ctx context.Context, cfg WireConfig) error {
    d.pins = make(map[string]*gpio.Pin)

    for _, pin := range cfg.Pins {
        if addr := pin.Addr; addr != "" {
            // Addr 格式: "GPIO17"
            gpioNum, _ := parseGPIOAddr(addr)
            gpioPin := gpio.NewPin(gpioNum)

            if pin.Rw == device.RO {
                gpioPin.Input()
            } else {
                gpioPin.Output()
            }

            d.pins[pin.ID] = gpioPin
        }
    }

    return nil
}

func (d *GPIODriver) OnPinWrite(ctx context.Context, pinID string, value nson.Value) error {
    pin, ok := d.pins[pinID]
    if !ok {
        return fmt.Errorf("gpio pin not found: %s", pinID)
    }

    if value.Bool() {
        pin.High()
    } else {
        pin.Low()
    }

    return nil
}
```

#### MQTT 驱动（桥接其他 MQTT 设备）

```go
// edge/driver/mqtt.go

func init() {
    RegisterDriver("mqtt", func() WireDriver {
        return &MQTTDriver{}
    })
}

type MQTTDriver struct {
    client   mqtt.Client
    topicMap map[string]string // pinID → MQTT Topic
}

func (d *MQTTDriver) OnPinWrite(ctx context.Context, pinID string, value nson.Value) error {
    topic, ok := d.topicMap[pinID]
    if !ok {
        return fmt.Errorf("no mqtt topic for pin: %s", pinID)
    }

    // 发布到 MQTT
    payload, _ := json.Marshal(map[string]interface{}{
        "value": value,
        "ts":    time.Now().Unix(),
    })

    token := d.client.Publish(topic, 1, false, payload)
    return token.Error()
}
```

### 4. Edge Service 集成

```go
// edge/edge.go

type EdgeService struct {
    // ... 现有字段

    drivers  map[string]WireDriver // wireID → Driver
    driverMu sync.RWMutex
}

// startDrivers 启动所有 Wire 驱动
func (es *EdgeService) startDrivers(ctx context.Context) error {
    es.drivers = make(map[string]WireDriver)

    node := es.storage.GetNode()

    for _, wire := range node.Wires {
        // 根据 Wire.Type 获取驱动
        driver, err := driver.GetDriver(wire.Type)
        if err != nil {
            // 没有驱动的 Wire 跳过（如虚拟设备）
            es.Logger().Sugar().Warnf("No driver for wire %s (type=%s)", wire.ID, wire.Type)
            continue
        }

        // 准备配置
        config := driver.WireConfig{
            WireID:   wire.ID,
            WireName: wire.Name,
            WireType: wire.Type,
            Pins:     wire.Pins,
            Options:  es.getWireOptions(wire.ID), // 从配置文件读取
        }

        // 初始化驱动
        if err := driver.Initialize(ctx, config); err != nil {
            return fmt.Errorf("init driver for wire %s: %w", wire.ID, err)
        }

        es.drivers[wire.ID] = driver

        // 如果驱动支持读取，启动轮询
        if _, ok := driver.(driver.ReadableDriver); ok {
            go es.pollDriver(ctx, wire.ID, driver)
        }
    }

    return nil
}

// 修改 SetPinWrite，增加硬件执行
func (es *EdgeService) SetPinWrite(ctx context.Context, value dt.PinValue) error {
    // ... 现有验证逻辑 ...

    // 保存到存储
    if err := es.storage.SetPinWrite(ctx, value); err != nil {
        return err
    }

    // 🔥 新增：执行硬件操作
    if err := es.executeHardware(ctx, value); err != nil {
        es.Logger().Sugar().Errorf("Execute hardware failed: %v", err)
        // 注意：硬件执行失败不影响数据保存
    }

    return nil
}

// executeHardware 执行硬件操作
func (es *EdgeService) executeHardware(ctx context.Context, value dt.PinValue) error {
    // 获取 Pin 所属的 Wire
    wireID, err := es.storage.GetPinWireID(value.ID)
    if err != nil {
        return err
    }

    // 获取对应的驱动
    es.driverMu.RLock()
    driver, ok := es.drivers[wireID]
    es.driverMu.RUnlock()

    if !ok {
        return fmt.Errorf("no driver for wire: %s", wireID)
    }

    // 调用驱动执行硬件操作
    return driver.OnPinWrite(ctx, value.ID, value.Value)
}

// pollDriver 轮询驱动读取传感器数据
func (es *EdgeService) pollDriver(ctx context.Context, wireID string, drv WireDriver) {
    ticker := time.NewTicker(5 * time.Second) // 可配置
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            values, err := drv.ReadPins(ctx)
            if err != nil {
                es.Logger().Sugar().Errorf("Read driver %s: %v", wireID, err)
                continue
            }

            // 更新 PinValue
            for _, v := range values {
                es.SetPinValue(ctx, v, false)
            }
        }
    }
}
```

### 5. 设备配置文件（设备树实例化）

```toml
# config/device.toml

[device]
id = "SN001"
name = "车间监控站"
template = "industrial_monitor"  # 使用预定义模板

# Wire 配置（实例化设备树）
[[wires]]
name = "temp_humi"
type = "modbus_rtu"
[wires.options]
port = "/dev/ttyUSB0"
slave_id = "1"
baudrate = "9600"

# Pin 地址映射
[[wires.pins]]
name = "temp"
addr = "40001"  # Modbus Holding Register 地址

[[wires.pins]]
name = "humi"
addr = "40002"

[[wires]]
name = "relay"
type = "gpio"
[wires.options]
# 树莓派 GPIO

[[wires.pins]]
name = "on"
addr = "GPIO17"

[[wires]]
name = "alarm"
type = "mqtt"
[wires.options]
broker = "tcp://localhost:1883"

[[wires.pins]]
name = "trigger"
addr = "alarm/station1/trigger"  # MQTT Topic
```

### 6. 使用示例

```go
// 创建 Edge 节点
es, err := edge.Edge(
    edge.WithNodeID("SN001", "secret"),
    edge.WithDeviceTemplate(device.IndustrialMonitor),
    edge.WithDriverConfig("config/device.toml"),
)

// 启动时会自动：
// 1. 加载设备模板（设备树声明）
// 2. 加载 Wire 配置（实例化）
// 3. 为每个 Wire 创建并初始化 Driver
// 4. 建立 Pin → 硬件地址的映射

// 当 Core 发送 PinWrite:
// Core → Queen → Edge.SetPinWrite → Driver.OnPinWrite → 硬件
```

## 优势

### 1. **分层清晰**
- Device Template: 逻辑定义（what）
- Wire Config: 实例配置（where）
- Driver: 硬件实现（how）

### 2. **可扩展**
- 新增硬件协议只需实现 `WireDriver` 接口
- 不影响核心代码

### 3. **可复用**
- Device Template 可共享
- Driver 可跨项目使用

### 4. **类型安全**
- Pin 类型在模板中声明
- Driver 层自动转换

### 5. **热插拔**
- Driver 可动态加载/卸载
- 支持设备重启

## 实现步骤

1. ✅ Device Template 已完成
2. 🔲 定义 WireDriver 接口
3. 🔲 实现 Driver Registry
4. 🔲 实现基础驱动（Modbus、GPIO、MQTT）
5. 🔲 修改 EdgeService 集成 Driver
6. 🔲 设计配置文件格式
7. 🔲 编写示例和文档

## 扩展方向

### 1. 驱动发现机制
```go
// 自动扫描可用驱动
drivers := driver.Discover()
for _, d := range drivers {
    fmt.Printf("Found driver: %s (supports: %v)\n", d.Name, d.SupportTypes)
}
```

### 2. 驱动配置验证
```go
type WireDriver interface {
    // ValidateConfig 验证配置是否合法
    ValidateConfig(cfg WireConfig) error
}
```

### 3. 驱动状态监控
```go
type WireDriver interface {
    // GetStatus 获取驱动状态
    GetStatus() DriverStatus
}

type DriverStatus struct {
    State       string    // running, error, stopped
    LastError   error
    LastSuccess time.Time
    Stats       map[string]interface{}
}
```

### 4. 驱动链（类似 Linux I/O 调度）
```go
// 允许多层驱动组合
// 例如: GPIO → I2C → 传感器芯片
type ChainDriver struct {
    Next WireDriver
}
```

## 总结

当前的 Device 定义已经是很好的"设备树"声明，但缺少**驱动执行层**。

建议增加：
1. **WireDriver 接口** - 硬件抽象
2. **Driver Registry** - 驱动管理
3. **executeHardware()** - 执行桥接

这样就能实现完整的：**设备树声明 → 驱动映射 → 硬件执行** 闭环。
