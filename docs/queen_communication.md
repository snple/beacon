# Queen 通信协议集成指南

## 概述

Beacon 现已支持使用 Queen 消息代理协议进行 Edge-Core 通信，作为 gRPC 的替代方案。Queen 是一个高性能的 MQTT 风格的消息代理，支持发布/订阅和请求/响应模式。

## 架构变更

### 之前 (gRPC)
```
Edge Node ←─gRPC Stream─→ Core Service
  - nodes.proto (Push/KeepAlive)
  - pins.proto (PushValue/PullWrite)
```

### 现在 (Queen)
```
Edge Node ←─Queen Pub/Sub─→ Queen Broker (in Core)
  - beacon/push (配置推送)
  - beacon/pin/value (Pin 值上报)
  - beacon/pin/write (写入命令通知)
  - beacon.pin.write.pull (拉取写入命令 - Request/Response)
```

## 配置方式

### Core 端配置

在 Core 初始化时启用 Queen Broker：

```go
import (
    "github.com/snple/beacon/core"
)

// 创建 CoreService 时添加 Queen 支持
cs, err := core.NewCoreService(
    dataPath,
    core.WithQueenBroker(&core.QueenBrokerConfig{
        Addr: ":3883",        // Queen Broker 监听地址
        TLS:  false,          // 是否启用 TLS
    }),
)
if err != nil {
    log.Fatal(err)
}

// 启动服务
if err := cs.Start(); err != nil {
    log.Fatal(err)
}
```

### Edge 端配置

Edge 可以选择使用 gRPC 或 Queen 协议：

#### 方式 1: 使用 Queen 通信 (推荐)

```go
import (
    "github.com/snple/beacon/edge"
)

// 创建 EdgeService 时指定使用 Queen
es, err := edge.NewEdgeService(
    dataPath,
    edge.WithNode(&edge.NodeOptions{
        UseQueen:  true,              // 启用 Queen 通信
        QueenAddr: "localhost:3883",  // Queen Broker 地址
        QueenTLS:  false,             // 是否使用 TLS
        ID:        nodeID,
        Secret:    nodeSecret,
    }),
)
if err != nil {
    log.Fatal(err)
}

// 启动服务
if err := es.Start(); err != nil {
    log.Fatal(err)
}
```

#### 方式 2: 使用 gRPC 通信 (传统方式)

```go
// UseQueen 设为 false 或不设置（默认）
es, err := edge.NewEdgeService(
    dataPath,
    edge.WithNode(&edge.NodeOptions{
        UseQueen: false,              // 使用 gRPC
        Addr:     "localhost:50051",  // gRPC 地址
        ID:       nodeID,
        Secret:   nodeSecret,
    }),
)
```

## 通信主题说明

### 1. beacon/push - 配置推送

**方向**: Edge → Core
**QoS**: 1 (至少一次)
**数据格式**: NSON (Node Storage 导出的配置数据)

Edge 将本地配置（Devices、Pins、Options 等）序列化为 NSON 格式，通过此主题推送到 Core。

### 2. beacon/pin/value - Pin 值上报

**方向**: Edge → Core
**QoS**: 1 (至少一次)
**数据格式**: NSON Array，每个元素包含：
- `id` (string): Pin ID
- `value` (Message): Pin 值
- `created` (int64): 创建时间 (UnixMilli)
- `updated` (int64): 更新时间 (UnixMilli)

Edge 定期批量上报 Pin 值的变化。

### 3. beacon/pin/write - 写入命令通知

**方向**: Core → Edge
**QoS**: 1 (至少一次)
**数据格式**: NSON Map，每个 Pin ID 对应一个写入命令数组

Core 通过此主题通知 Edge 有新的 Pin 写入命令需要执行。Edge 收到通知后会立即通过 `beacon.pin.write.pull` 拉取完整的写入命令列表。

### 4. beacon.pin.write.pull - 拉取写入命令

**方向**: Edge → Core (Request/Response)
**数据格式**:
- Request: NSON Map 包含 `updated` 字段 (上次同步时间)
- Response: NSON Array，包含所有待执行的写入命令

Edge 主动拉取需要执行的 Pin 写入命令列表。

## 数据同步流程

### 配置同步 (Edge → Core)

```
1. Edge 检测到本地配置变更 (nodeUpdated > nodeUpdated2)
2. Edge 导出配置为 NSON: ExportConfig()
3. Edge 发布到 beacon/push 主题 (QoS 1)
4. Core 接收并解析配置
5. Core 更新数据库
```

### Pin 值同步 (Edge → Core)

```
1. Edge 定期检查 Pin 值变化 (pinValueUpdated > pinValueUpdated2)
2. Edge 批量序列化变化的 Pin 值为 NSON Array
3. Edge 发布到 beacon/pin/value 主题 (QoS 1)
4. Core 接收并批量更新 Pin 值
```

### Pin 写入同步 (Core → Edge)

```
方式1: 推送通知 + 拉取
1. Core 创建 Pin 写入命令
2. Core 发布通知到 beacon/pin/write (QoS 1)
3. Edge 收到通知
4. Edge 发送 Request 到 beacon.pin.write.pull
5. Core 返回完整的写入命令列表
6. Edge 执行写入命令

方式2: 定期轮询
1. Edge 定期发送 Request 到 beacon.pin.write.pull
2. Core 返回新的写入命令
3. Edge 执行写入命令
```

## 认证机制

Queen 使用 ClientID + AuthData 进行认证：

```go
// Edge 连接时的认证信息
client := queen.New(queen.Config{
    Broker:     addr,
    ClientID:   nodeID,      // Node ID
    AuthMethod: "plain",
    AuthData:   []byte(nodeSecret),  // Node Secret
})
```

Core 的 Queen Broker 会验证 ClientID 和 AuthData：
```go
// 在 core/queen.go 中
func (cs *CoreService) authenticateBrokerClient(clientID string, authData []byte) bool {
    node, err := cs.GetNode(clientID)
    if err != nil {
        return false
    }
    return node.GetSecret() == string(authData)
}
```

## 优势对比

### Queen vs gRPC

| 特性 | Queen | gRPC |
|------|-------|------|
| 协议复杂度 | 简单 (类 MQTT) | 复杂 (HTTP/2) |
| 依赖 | 无需 protobuf | 需要 protoc 生成代码 |
| 消息模式 | Pub/Sub + Request/Response | Stream + RPC |
| 流量控制 | 内置窗口机制 | 需要手动实现 |
| 消息持久化 | 客户端支持 | 需要自行实现 |
| 连接保持 | KeepAlive + 自动重连 | 需要手动维护 |
| 优先级 | 支持消息优先级 | 不支持 |
| 轻量级 | ✅ | ❌ |

### 适用场景

**推荐使用 Queen**:
- 新项目或新部署
- 需要消息持久化和可靠传输
- 边缘设备资源受限
- 需要消息优先级
- 网络不稳定需要自动重连

**继续使用 gRPC**:
- 已有 gRPC 基础设施
- 需要 gRPC 特有功能（如复杂的流式处理）
- 与其他 gRPC 服务集成

## 未来规划

1. **移除 node 模块**: 当所有 Edge 节点都切换到 Queen 后，可以移除 `core/node` 中的 gRPC 服务定义
2. **扩展主题**: 添加更多主题支持设备管理、日志上报等功能
3. **集群支持**: 利用 Queen 的多 Broker 支持实现 Core 集群
4. **安全增强**: 添加 TLS/SSL、证书认证等安全机制

## 示例代码

完整的示例代码请参考：
- Core 端: [core/queen.go](../core/queen.go)
- Edge 端: [edge/queen_up.go](../edge/queen_up.go)
- 协议处理: [core/queen_handlers.go](../core/queen_handlers.go)

## 故障排查

### Edge 无法连接到 Queen Broker

检查：
1. Core 是否启用了 Queen Broker (`WithQueenBroker`)
2. 网络连接是否正常 (`telnet <addr>`)
3. 认证信息是否正确 (NodeID 和 Secret)

### 消息未收到

检查：
1. 是否已成功订阅主题
2. QoS 级别是否正确
3. 查看日志中的错误信息

### 性能问题

优化建议：
1. 批量发送 Pin 值而不是逐个发送
2. 使用 QoS 0 减少确认开销（适用于高频但不重要的数据）
3. 调整同步间隔避免过于频繁

## 总结

通过集成 Queen 协议，Beacon 获得了更简洁、可靠的 Edge-Core 通信方案。Queen 的 Pub/Sub 模式天然契合 IoT 场景的需求，同时减少了对 gRPC 和 Protobuf 的依赖，降低了系统的复杂度。
