# Device 包架构图

## 系统架构

```
┌─────────────────────────────────────────────────────────────────┐
│                     Device Package                               │
│                                                                   │
│  ┌────────────────────────────────────────────────────────┐     │
│  │           Device Registry (线程安全)                    │     │
│  │                                                          │     │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │     │
│  │  │  Lighting    │  │   Sensor     │  │   Switch     │ │     │
│  │  │  (4 devices) │  │  (4 devices) │  │  (4 devices) │ │     │
│  │  └──────────────┘  └──────────────┘  └──────────────┘ │     │
│  │                                                          │     │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │     │
│  │  │  Security    │  │   Climate    │  │   Custom     │ │     │
│  │  │  (5 devices) │  │  (5 devices) │  │  (用户自定义) │ │     │
│  │  └──────────────┘  └──────────────┘  └──────────────┘ │     │
│  │                                                          │     │
│  │                 27+ Standard Devices                     │     │
│  └────────────────────────────────────────────────────────┘     │
│                                                                   │
│  ┌────────────────────────────────────────────────────────┐     │
│  │              Device Builder API                          │     │
│  │                                                          │     │
│  │  QuickBuildDevice()  ─────┐                            │     │
│  │  NewDeviceBuilder()  ─────┤                            │     │
│  │  BuildDeviceWithAddresses()├─> DeviceInstance          │     │
│  │                             │   - Wires                 │     │
│  │  Builder Pattern:           │   - Pins                 │     │
│  │  - SetPinAddress()          │   - Addresses            │     │
│  │  - EnableWire()             │                          │     │
│  │  - Build()                  │                          │     │
│  └────────────────────────────────────────────────────────┘     │
│                                                                   │
│  ┌────────────────────────────────────────────────────────┐     │
│  │              Cluster Definitions                         │     │
│  │                                                          │     │
│  │  OnOff  LevelControl  ColorControl  Temperature  ...    │     │
│  │    │         │             │              │              │     │
│  │    └─────────┴─────────────┴──────────────┘              │     │
│  │                     │                                    │     │
│  │              PinTemplates                                │     │
│  │              (name, type, rw, default)                  │     │
│  └────────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────┘
```

## 数据流

```
用户代码
   │
   ├─ 查询设备 ──────────────────────────────┐
   │  ListDevices()                          │
   │  GetDevice()                            ▼
   │  ListDevicesByCategory()        ┌──────────────┐
   │                                 │   Registry   │
   │                                 │  (27+ devices)│
   │                                 └──────────────┘
   │                                         │
   ├─ 创建设备实例 ───────────────────────────┤
   │  QuickBuildDevice()                     │
   │  NewDeviceBuilder()                     ▼
   │                                 ┌──────────────┐
   │                                 │   Builder    │
   │                                 │  - Template  │
   │                                 │  - Config    │
   │                                 └──────────────┘
   │                                         │
   ├─ 配置地址 ────────────────────────────────┤
   │  SetPinAddress()                        │
   │  SetPinAddresses()                      │
   │                                         ▼
   ├─ 构建实例 ─────────────────────────> Build()
   │                                         │
   │                                         ▼
   │                                 ┌──────────────┐
   └─ 使用实例 ───────────────────> │   Instance   │
      - Wires                        │  - Wires[]   │
      - Pins                         │  - Pins[]    │
      - Addresses                    │  - Addrs     │
                                     └──────────────┘
                                             │
                                             ▼
                                    创建数据库记录
                                    (应用程序负责)
```

## 类型关系

```
DeviceTemplate (设备模板 - 静态)
├── ID: string
├── Name: string
├── Category: string
├── Manufacturer: string
├── Model: string
└── Wires: []WireTemplate
        ├── Name: string
        ├── Clusters: []string
        └── Required: bool
                │
                ▼ 引用
        ┌───────────────┐
        │    Cluster    │ (功能集合定义)
        ├───────────────┤
        │ ID: ClusterID │
        │ Name: string  │
        └── Pins: []PinTemplate
                ├── Name: string
                ├── Type: uint32
                ├── Rw: int32
                └── Default: nson.Value

DeviceInstance (设备实例 - 运行时)
├── DeviceID: string
├── InstanceName: string
├── Template: *DeviceTemplate
└── Wires: []WireInstance
        ├── Name: string
        ├── Clusters: []string
        └── Pins: []PinInstance
                ├── Name: string
                ├── Type: uint32
                ├── Rw: int32
                └── Addr: string (物理地址)
```

## 模块组成

```
device/
│
├── 核心模块 (Core)
│   ├── device_template.go      # 设备模板定义
│   ├── standard_devices.go     # 标准设备库
│   ├── device_builder.go       # 设备构建器
│   └── cluster.go              # Cluster 定义
│
├── 兼容模块 (Legacy)
│   ├── builder.go              # SimpleBuildResult
│   ├── initializer.go          # 数据库初始化
│   └── config_manager.go       # 配置文件管理
│
├── 测试 (Tests)
│   ├── device_template_test.go # 模板测试
│   ├── device_builder_test.go  # 构建器测试
│   └── cluster_test.go         # Cluster 测试
│
├── 文档 (Docs)
│   ├── DEVICE_TEMPLATE_README.md  # 完整文档
│   ├── REFACTOR_SUMMARY.md        # 重构总结
│   ├── QUICKSTART.md              # 快速入门
│   ├── ARCHITECTURE.md            # 架构图
│   └── README.md                  # 原有文档
│
└── 示例 (Examples)
    └── examples/main.go           # 8 个使用示例
```

## 使用场景

```
┌─────────────────────────────────────────────────────────────┐
│                  典型使用场景                                 │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  场景 1: UI 显示设备列表                                      │
│  ┌──────────────────────────────────────┐                   │
│  │ categories := GetCategories()       │                   │
│  │ for _, cat := range categories {    │                   │
│  │   devices := ListDevicesByCategory(cat) │               │
│  │   // 显示设备列表                    │                   │
│  │ }                                    │                   │
│  └──────────────────────────────────────┘                   │
│                                                              │
│  场景 2: 用户选择设备并配置                                   │
│  ┌──────────────────────────────────────┐                   │
│  │ template := GetDevice("smart_bulb_color") │             │
│  │ builder := NewDeviceBuilder(id, name)     │             │
│  │ // UI 让用户配置 Pin 地址             │                   │
│  │ builder.SetPinAddress(...)            │                   │
│  │ instance := builder.Build()           │                   │
│  │ // 保存到数据库                       │                   │
│  └──────────────────────────────────────┘                   │
│                                                              │
│  场景 3: 批量初始化设备                                       │
│  ┌──────────────────────────────────────┐                   │
│  │ devices := []struct{                 │                   │
│  │   id   string                        │                   │
│  │   name string                        │                   │
│  │   addrs map[string]map[string]string │                   │
│  │ }{...}                               │                   │
│  │                                      │                   │
│  │ for _, dev := range devices {        │                   │
│  │   instance := BuildDeviceWithAddresses(...) │           │
│  │   // 保存到数据库                     │                   │
│  │ }                                    │                   │
│  └──────────────────────────────────────┘                   │
│                                                              │
│  场景 4: 注册厂商自定义设备                                   │
│  ┌──────────────────────────────────────┐                   │
│  │ custom := &DeviceTemplate{...}       │                   │
│  │ RegisterDevice(custom)               │                   │
│  │ // 后续像标准设备一样使用             │                   │
│  └──────────────────────────────────────┘                   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## 性能特性

```
┌─────────────────────────────────────────┐
│         性能特性                         │
├─────────────────────────────────────────┤
│                                          │
│  ✅ 线程安全                             │
│     - 注册表使用 RWMutex                 │
│     - 支持并发读取                       │
│     - 注册操作安全                       │
│                                          │
│  ✅ 内存效率                             │
│     - 模板共享，实例轻量                 │
│     - 按需构建设备实例                   │
│     - 不存储冗余数据                     │
│                                          │
│  ✅ 快速查询                             │
│     - 按 ID 查询: O(1)                   │
│     - 按类别查询: O(n) n=类别设备数       │
│     - 列出所有设备: O(n) n=总设备数      │
│                                          │
│  ✅ 初始化性能                           │
│     - 启动时一次性注册                   │
│     - 27 个设备注册耗时 < 1ms           │
│     - 不影响应用启动速度                 │
│                                          │
└─────────────────────────────────────────┘
```
