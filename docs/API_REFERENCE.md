# Beacon API 参考文档

## 📋 目录

1. [Core 端 API](#core-端-api)
2. [Edge 端 API](#edge-端-api)
3. [Storage API](#storage-api)
4. [Device Builder API](#device-builder-api)
5. [数据类型](#数据类型)
6. [错误码](#错误码)

## Core 端 API

### CoreService

Core 端主服务，管理所有节点配置和数据。

#### 创建 CoreService

```go
import (
    "github.com/snple/beacon/core"
    "github.com/dgraph-io/badger/v4"
)

// 创建 Core 服务
func NewCore() (*core.CoreService, error) {
    // 打开 Badger 数据库
    opts := badger.DefaultOptions("/data/beacon/core")
    db, err := badger.Open(opts)
    if err != nil {
        return nil, err
    }

    // 创建 Core 服务
    coreOpts := []core.CoreOption{
        core.WithLogger(logger),
        core.WithLinkTTL(3 * time.Minute),
    }

    cs, err := core.Core(db, coreOpts...)
    if err != nil {
        return nil, err
    }

    // 启动服务
    if err := cs.Start(); err != nil {
        return nil, err
    }

    return cs, nil
}
```

#### 配置选项

```go
// WithLogger 设置日志器
func WithLogger(logger *zap.Logger) CoreOption

// WithLinkTTL 设置链接超时时间 (默认 3 分钟)
func WithLinkTTL(d time.Duration) CoreOption
```

#### 方法

```go
// Start 启动服务，加载所有存储数据
func (cs *CoreService) Start() error

// Stop 停止服务，清理资源
func (cs *CoreService) Stop()

// GetStorage 获取存储实例
func (cs *CoreService) GetStorage() *storage.Storage

// GetSync 获取同步服务
func (cs *CoreService) GetSync() *SyncService

// GetNode 获取节点服务
func (cs *CoreService) GetNode() *NodeService

// GetWire 获取 Wire 服务
func (cs *CoreService) GetWire() *WireService

// GetPin 获取 Pin 服务
func (cs *CoreService) GetPin() *PinService

// GetPinValue 获取 PinValue 服务
func (cs *CoreService) GetPinValue() *PinValueService

// GetPinWrite 获取 PinWrite 服务
func (cs *CoreService) GetPinWrite() *PinWriteService

// Register 注册 gRPC 服务
func (cs *CoreService) Register(server *grpc.Server)
```

### NodeService

节点管理服务。

#### gRPC API

```protobuf
service NodeService {
  // 创建节点
  rpc Create(Node) returns (Node);

  // 更新节点
  rpc Update(Node) returns (Node);

  // 查看节点
  rpc View(Id) returns (Node);

  // 按名称查询节点
  rpc Name(Name) returns (Node);

  // 删除节点
  rpc Delete(Id) returns (MyBool);

  // 列出节点
  rpc List(NodeListRequest) returns (NodeListResponse);

  // 节点链接状态流
  rpc Link(Id) returns (stream NodeLinkResponse);
}
```

#### 使用示例

```go
import (
    "github.com/snple/beacon/pb/nodes"
    "google.golang.org/grpc"
)

// 创建 gRPC 客户端
conn, _ := grpc.Dial("localhost:50051", grpc.WithInsecure())
client := nodes.NewNodeServiceClient(conn)

// 创建节点
node := &pb.Node{
    Name:   "edge-001",
    Status: 1,
}
result, err := client.Create(ctx, node)

// 查询节点
result, err := client.View(ctx, &pb.Id{Id: "node-id"})

// 按名称查询
result, err := client.Name(ctx, &pb.Name{Name: "edge-001"})

// 列出所有节点
req := &nodes.NodeListRequest{
    Page:     &pb.Page{Limit: 20, Offset: 0},
    NodeId:   "",
    Name:     "",
    Tags:     "",
    Type:     "",
    Status:   nodes.NodeStatus_ON,
}
resp, err := client.List(ctx, req)

// 监听节点状态
stream, err := client.Link(ctx, &pb.Id{Id: "node-id"})
for {
    link, err := stream.Recv()
    if err != nil {
        break
    }
    fmt.Printf("Node: %s, Status: %v\n", link.Name, link.Status)
}
```

### PinValueService

Pin 读取值服务。

#### gRPC API

```protobuf
service PinValueService {
  // 按 ID 获取值
  rpc GetValue(Id) returns (PinValue);

  // 按 ID 设置值
  rpc SetValue(PinValue) returns (MyBool);

  // 按名称获取值 (NodeID + Pin 全名)
  rpc GetValueByName(PinNameRequest) returns (PinNameValue);

  // 按名称设置值
  rpc SetValueByName(PinNameValueRequest) returns (MyBool);
}
```

#### 使用示例

```go
import (
    "github.com/snple/beacon/pb/cores"
)

client := cores.NewPinValueServiceClient(conn)

// 按 ID 获取值
value, err := client.GetValue(ctx, &pb.Id{Id: "pin-id"})
fmt.Printf("Value: %v, Updated: %v\n", value.Value, value.Updated)

// 按名称获取值 (格式: NodeName.WireName.PinName)
req := &cores.PinNameRequest{
    NodeId: "node-id",
    Name:   "modbus.temp_sensor",  // wire.pin
}
result, err := client.GetValueByName(ctx, req)

// 设置值
nsonValue := &pb.NsonValue{
    Value: &pb.NsonValue_F32{F32: 25.5},
}
pinValue := &pb.PinValue{
    Id:      "pin-id",
    Value:   nsonValue,
    Updated: timestamppb.Now(),
}
_, err = client.SetValue(ctx, pinValue)
```

### PinWriteService

Pin 写入指令服务。

#### gRPC API

```protobuf
service PinWriteService {
  // 获取写入指令
  rpc GetWrite(Id) returns (PinValue);

  // 设置写入指令
  rpc SetWrite(PinValue) returns (MyBool);

  // 删除写入指令
  rpc DeleteWrite(Id) returns (MyBool);

  // 拉取节点的所有待写入指令 (流式)
  rpc PullWrite(PinPullWriteRequest) returns (stream PinValue);

  // 按名称操作
  rpc GetWriteByName(PinNameRequest) returns (PinNameValue);
  rpc SetWriteByName(PinNameValueRequest) returns (MyBool);
}
```

#### 使用示例

```go
client := cores.NewPinWriteServiceClient(conn)

// 设置写入指令
nsonValue := &pb.NsonValue{
    Value: &pb.NsonValue_Bool{Bool: true},
}
pinValue := &pb.PinValue{
    Id:      "pin-id",
    Value:   nsonValue,
    Updated: timestamppb.Now(),
}
_, err := client.SetWrite(ctx, pinValue)

// 按名称设置写入
req := &cores.PinNameValueRequest{
    NodeId: "node-id",
    Name:   "gpio.led1",
    Value:  nsonValue,
}
_, err = client.SetWriteByName(ctx, req)

// Edge 端拉取写入指令
pullReq := &cores.PinPullWriteRequest{
    NodeId: "node-id",
    After:  timestamppb.New(lastSyncTime),
}
stream, err := client.PullWrite(ctx, pullReq)
for {
    write, err := stream.Recv()
    if err != nil {
        break
    }

    // 执行写入操作
    executeWrite(write)

    // 写入成功后删除指令
    client.DeleteWrite(ctx, &pb.Id{Id: write.Id})
}
```

## Edge 端 API

### EdgeService

Edge 端主服务，管理本地设备。

#### 创建 EdgeService

```go
import (
    "github.com/snple/beacon/edge"
)

// 创建 Edge 服务
func NewEdge() (*edge.EdgeService, error) {
    edgeOpts := []edge.EdgeOption{
        edge.WithNodeID(nodeID, secret),
        edge.WithLogger(logger),
        edge.WithSync(edge.SyncOptions{
            TokenRefresh: 3 * time.Minute,
            Link:         time.Minute,
            Interval:     time.Minute,
            Realtime:     false,
        }),
    }

    // 配置 Node 客户端 (连接到 Core)
    if coreAddr != "" {
        grpcOpts := []grpc.DialOption{
            grpc.WithInsecure(),
        }

        edgeOpts = append(edgeOpts, edge.WithNode(edge.NodeOptions{
            Enable:      true,
            Addr:        coreAddr,
            GRPCOptions: grpcOpts,
        }))
    }

    es, err := edge.Edge(edgeOpts...)
    if err != nil {
        return nil, err
    }

    // 启动服务
    es.Start()

    return es, nil
}
```

#### 配置选项

```go
// WithNodeID 设置节点 ID 和 Secret
func WithNodeID(id, secret string) EdgeOption

// WithLogger 设置日志器
func WithLogger(logger *zap.Logger) EdgeOption

// WithSync 设置同步选项
func WithSync(options SyncOptions) EdgeOption

// WithNode 设置 Node 客户端 (连接到 Core)
func WithNode(options NodeOptions) EdgeOption

// WithBadger 设置 Badger 选项
func WithBadger(options badger.Options) EdgeOption

// WithLinkTTL 设置链接超时
func WithLinkTTL(d time.Duration) EdgeOption
```

#### 同步选项

```go
type SyncOptions struct {
    TokenRefresh time.Duration // Token 刷新间隔
    Link         time.Duration // 链接状态报告间隔
    Interval     time.Duration // 数据同步间隔
    Realtime     bool          // 是否实时同步
    Retry        time.Duration // 重试间隔
}
```

#### 方法

```go
// Start 启动服务
func (es *EdgeService) Start()

// Stop 停止服务
func (es *EdgeService) Stop()

// Push 手动推送配置到 Core
func (es *EdgeService) Push() error

// GetStorage 获取存储实例
func (es *EdgeService) GetStorage() *storage.Storage

// GetSync 获取同步服务
func (es *EdgeService) GetSync() *SyncService

// Register 注册 gRPC 服务 (本地 API)
func (es *EdgeService) Register(server *grpc.Server)
```

#### 使用示例

```go
// 获取本地 Pin 值
storage := es.GetStorage()
value, updated, err := storage.GetPinValue("pin-id")

// 设置本地 Pin 值
nsonValue := &pb.NsonValue{
    Value: &pb.NsonValue_F32{F32: 25.5},
}
err = storage.SetPinValue(ctx, "pin-id", nsonValue, time.Now())

// 手动推送配置到 Core
err = es.Push()
```

## Storage API

### Core Storage

#### 节点操作

```go
// GetNode 获取节点
func (s *Storage) GetNode(nodeID string) (*Node, error)

// GetNodeByName 按名称获取节点
func (s *Storage) GetNodeByName(name string) (*Node, error)

// ListNodes 列出所有节点
func (s *Storage) ListNodes() []*Node

// Push 接收 Edge 推送的配置 (NSON 格式)
func (s *Storage) Push(ctx context.Context, data []byte) error

// DeleteNode 删除节点
func (s *Storage) DeleteNode(ctx context.Context, nodeID string) error
```

#### Wire 操作

```go
// GetWireByID 按 ID 获取 Wire
func (s *Storage) GetWireByID(wireID string) (*Wire, error)

// GetWireByName 按名称获取 Wire
func (s *Storage) GetWireByName(nodeID, wireName string) (*Wire, error)

// GetWireByFullName 按全名获取 (NodeName.WireName)
func (s *Storage) GetWireByFullName(fullName string) (*Wire, error)

// ListWires 获取节点的所有 Wire
func (s *Storage) ListWires(nodeID string) ([]*Wire, error)
```

#### Pin 操作

```go
// GetPinByID 按 ID 获取 Pin
func (s *Storage) GetPinByID(pinID string) (*Pin, error)

// GetPinByName 按名称获取 Pin (支持 "wire.pin")
func (s *Storage) GetPinByName(nodeID, pinName string) (*Pin, error)

// GetPinByFullName 按全名获取 (NodeName.WireName.PinName)
func (s *Storage) GetPinByFullName(fullName string) (*Pin, error)

// GetPinNodeID 获取 Pin 所属的 Node ID
func (s *Storage) GetPinNodeID(pinID string) (string, error)

// ListPins 获取 Wire 的所有 Pin
func (s *Storage) ListPins(wireID string) ([]*Pin, error)

// ListPinsByNode 获取节点的所有 Pin
func (s *Storage) ListPinsByNode(nodeID string) ([]*Pin, error)
```

#### PinValue 操作

```go
// GetPinValue 获取点位值
func (s *Storage) GetPinValue(nodeID, pinID string) (*pb.NsonValue, time.Time, error)

// SetPinValue 设置点位值
func (s *Storage) SetPinValue(ctx context.Context, nodeID, pinID string, value *pb.NsonValue, updated time.Time) error

// ListPinValues 列出节点的点位值
func (s *Storage) ListPinValues(nodeID string, after time.Time, limit int) ([]PinValueEntry, error)
```

#### PinWrite 操作

```go
// GetPinWrite 获取点位写入值
func (s *Storage) GetPinWrite(nodeID, pinID string) (*pb.NsonValue, time.Time, error)

// SetPinWrite 设置点位写入值
func (s *Storage) SetPinWrite(ctx context.Context, nodeID, pinID string, value *pb.NsonValue, updated time.Time) error

// DeletePinWrite 删除点位写入值
func (s *Storage) DeletePinWrite(ctx context.Context, nodeID, pinID string) error

// ListPinWrites 列出节点的写入值
func (s *Storage) ListPinWrites(nodeID string, after time.Time, limit int) ([]PinValueEntry, error)
```

#### Secret 操作

```go
// GetSecret 获取节点 Secret
func (s *Storage) GetSecret(nodeID string) (string, error)

// SetSecret 设置节点 Secret
func (s *Storage) SetSecret(ctx context.Context, nodeID, secret string) error
```

### Edge Storage

#### 节点操作

```go
// GetNode 获取节点
func (s *Storage) GetNode() (*Node, error)

// GetNodeID 获取节点 ID
func (s *Storage) GetNodeID() string

// SetNode 设置/更新节点配置
func (s *Storage) SetNode(ctx context.Context, node *Node) error

// UpdateNodeName 更新节点名称
func (s *Storage) UpdateNodeName(ctx context.Context, name string) error

// UpdateNodeStatus 更新节点状态
func (s *Storage) UpdateNodeStatus(ctx context.Context, status int32) error
```

#### 配置导入/导出

```go
// ExportConfig 导出节点配置为 NSON 字节
func (s *Storage) ExportConfig() ([]byte, error)

// ImportConfig 从 NSON 字节导入节点配置
func (s *Storage) ImportConfig(ctx context.Context, data []byte) error
```

#### 同步时间戳

```go
// GetSyncTime 获取同步时间戳
func (s *Storage) GetSyncTime(key string) (time.Time, error)

// SetSyncTime 设置同步时间戳
func (s *Storage) SetSyncTime(key string, t time.Time) error

// 预定义的 Sync Key:
const (
    SYNC_NODE                  = "sync:node"      // 本地配置最新时间
    SYNC_PIN_VALUE             = "sync:pin_value" // 本地 PinValue 最新时间
    SYNC_PIN_WRITE             = "sync:pin_write" // 本地 PinWrite 最新时间
    SYNC_NODE_TO_REMOTE        = "sync:node_ltr"  // 已同步到 Core 的时间
    SYNC_PIN_VALUE_TO_REMOTE   = "sync:pv_ltr"    // PinValue 已同步到 Core
    SYNC_PIN_WRITE_FROM_REMOTE = "sync:pw_rtl"    // PinWrite 已从 Core 拉取
)
```

## Device Builder API

### Cluster

#### 预定义 Cluster

```go
import "github.com/snple/beacon/device"

// 预定义的 Cluster
var (
    OnOffCluster                  *Cluster // 开关控制
    LevelControlCluster           *Cluster // 亮度/级别控制
    ColorControlCluster           *Cluster // 颜色控制 (HSV)
    TemperatureMeasurementCluster *Cluster // 温度测量
    HumidityMeasurementCluster    *Cluster // 湿度测量
    BasicInformationCluster       *Cluster // 设备基本信息
)

// 获取 Cluster
cluster := device.GetCluster("OnOff")

// 注册自定义 Cluster
device.RegisterCluster(&myCluster)
```

#### 定义 Cluster

```go
var MyCluster = device.Cluster{
    ID:          0x9999,
    Name:        "MyDevice",
    Description: "自定义设备",
    Pins: []device.PinTemplate{
        {
            Name:    "value",
            Desc:    "当前值",
            Type:    dt.TypeI32,
            Rw:      1, // 读写
            Default: nson.I32(0),
            Tags:    "custom",
        },
        {
            Name:    "status",
            Desc:    "状态",
            Type:    dt.TypeString,
            Rw:      0, // 只读
            Default: nson.String("ok"),
            Tags:    "custom",
        },
    },
}
```

### WireBuilder

#### 创建 Wire

```go
import "github.com/snple/beacon/device"

// 方式 1: 使用预定义模板
result := device.BuildDimmableLightWire("main_light")

// 方式 2: 使用 Builder
result := device.NewWireBuilder("my_wire").
    WithClusters("OnOff", "LevelControl").
    Build()

// 方式 3: 自定义 Cluster
result := device.NewWireBuilder("custom_device").
    WithCustomCluster(&myCluster).
    Build()

// result.Wire 包含 Wire 配置
// result.Pins 包含所有 Pin 模板
```

#### 预定义模板

```go
// 灯光设备
BuildOnOffLightWire(name)          // 开关灯
BuildDimmableLightWire(name)       // 可调光灯
BuildColorLightWire(name)          // RGB 彩灯

// 传感器
BuildTemperatureSensorWire(name)   // 温度传感器
BuildTempHumiSensorWire(name)      // 温湿度传感器

// 特殊
BuildRootWire()                    // 根 Wire (每个 Node 必须有)
```

#### 使用示例

```go
// 1. 创建 Wire
result := device.BuildDimmableLightWire("bedroom_light")

// 2. 设置 Wire 的 Type 和 Tags
result.Wire.Type = "DimmableLight"
result.Wire.Tags = []string{"category:light", "room:bedroom"}

// 3. 为每个 Pin 设置地址
for _, pin := range result.Pins {
    switch pin.Name {
    case "onoff":
        pin.Addr = "40001"
    case "level":
        pin.Addr = "40002"
    }
}

// 4. 保存到数据库或发送到 Core
// (具体实现取决于你的应用逻辑)
```

## 数据类型

### NSON DataType

```go
const (
    TypeNull uint32 = iota
    TypeBool

    // 整数
    TypeI8
    TypeI16
    TypeI32
    TypeI64

    TypeU8
    TypeU16
    TypeU32
    TypeU64

    // 浮点数
    TypeF32
    TypeF64

    // 字符串和二进制
    TypeString
    TypeBinary

    // 复杂类型
    TypeArray
    TypeMap
    TypeMessageId
    TypeTimestamp
)
```

### NsonValue

```protobuf
message NsonValue {
  oneof value {
    bool bool = 1;

    int32 i8 = 2;
    int32 i16 = 3;
    int32 i32 = 4;
    int64 i64 = 5;

    uint32 u8 = 6;
    uint32 u16 = 7;
    uint32 u32 = 8;
    uint64 u64 = 9;

    float f32 = 10;
    double f64 = 11;

    string string = 12;
    bytes binary = 13;
  }
}
```

### 使用示例

```go
// 创建 NSON 值
boolVal := &pb.NsonValue{Value: &pb.NsonValue_Bool{Bool: true}}
i32Val := &pb.NsonValue{Value: &pb.NsonValue_I32{I32: 123}}
f32Val := &pb.NsonValue{Value: &pb.NsonValue_F32{F32: 25.5}}
strVal := &pb.NsonValue{Value: &pb.NsonValue_String_{String_: "hello"}}

// 读取值
switch v := nsonVal.Value.(type) {
case *pb.NsonValue_Bool:
    fmt.Println("Bool:", v.Bool)
case *pb.NsonValue_I32:
    fmt.Println("I32:", v.I32)
case *pb.NsonValue_F32:
    fmt.Println("F32:", v.F32)
case *pb.NsonValue_String_:
    fmt.Println("String:", v.String_)
}
```

## 错误码

### gRPC 错误码

Beacon 使用标准 gRPC 状态码:

```go
import "google.golang.org/grpc/codes"

codes.OK                 // 成功
codes.InvalidArgument    // 参数错误
codes.NotFound           // 资源不存在
codes.AlreadyExists      // 资源已存在
codes.FailedPrecondition // 前置条件不满足
codes.Internal           // 内部错误
codes.Unavailable        // 服务不可用
codes.Unauthenticated    // 未认证
codes.PermissionDenied   // 权限不足
```

### 错误处理示例

```go
import (
    "google.golang.org/grpc/status"
    "google.golang.org/grpc/codes"
)

// 服务端返回错误
if nodeID == "" {
    return nil, status.Error(codes.InvalidArgument, "node ID is required")
}

if _, err := storage.GetNode(nodeID); err != nil {
    return nil, status.Errorf(codes.NotFound, "node not found: %v", err)
}

// 客户端处理错误
result, err := client.View(ctx, &pb.Id{Id: nodeID})
if err != nil {
    st, ok := status.FromError(err)
    if ok {
        switch st.Code() {
        case codes.NotFound:
            fmt.Println("Node not found")
        case codes.InvalidArgument:
            fmt.Println("Invalid argument:", st.Message())
        default:
            fmt.Println("Error:", st.Message())
        }
    }
}
```

## 最佳实践

### 1. 使用 Context

始终传递 context 进行超时和取消控制:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := client.View(ctx, &pb.Id{Id: nodeID})
```

### 2. 连接池

复用 gRPC 连接:

```go
// 创建一次，复用多次
conn, err := grpc.Dial(addr, opts...)
if err != nil {
    return err
}
defer conn.Close()

nodeClient := nodes.NewNodeServiceClient(conn)
wireClient := nodes.NewWireServiceClient(conn)
```

### 3. 流式 API

对于大量数据，使用流式 API:

```go
stream, err := client.PullWrite(ctx, req)
for {
    item, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }

    // 处理 item
    process(item)
}
```

### 4. 错误处理

始终检查错误并妥善处理:

```go
value, err := storage.GetPinValue(nodeID, pinID)
if err != nil {
    if errors.Is(err, badger.ErrKeyNotFound) {
        // Pin 值不存在，使用默认值
        value = defaultValue
    } else {
        // 其他错误，记录并返回
        logger.Error("get pin value failed", zap.Error(err))
        return err
    }
}
```

### 5. 资源清理

使用 defer 确保资源被正确清理:

```go
cs, err := core.Core(db, opts...)
if err != nil {
    return err
}
defer cs.Stop()

cs.Start()
// ... 使用服务
```

## 相关文档

- [架构设计](ARCHITECTURE.md)
- [开发指南](DEVELOPMENT.md)
- [项目分析](PROJECT_ANALYSIS.md)
