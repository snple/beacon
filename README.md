# Beacon

[![PkgGoDev](https://pkg.go.dev/badge/github.com/snple/beacon)](https://pkg.go.dev/github.com/snple/beacon)
[![Go Report Card](https://goreportcard.com/badge/github.com/snple/beacon)](https://goreportcard.com/report/github.com/snple/beacon)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

一个高性能、易部署的 IoT 开发框架，采用 Core-Edge 分布式架构。

## ✨ 特性

- **🚀 高性能**: 内存存储 + NSON 序列化，查询响应达微秒级
- **📦 易部署**: 纯 Go 实现，无 CGO 依赖，单一二进制文件
- **🔌 灵活扩展**: Wire/Pin 模型适配各种 IoT 设备
- **🎯 标准化**: Cluster 系统促进设备定义统一
- **💾 低资源**: Edge 端可运行在资源受限的嵌入式设备
- **🔒 安全**: 支持 TLS 双向认证，节点级别 Secret

## 🏗️ 架构

```
┌─────────────┐         gRPC/TLS        ┌─────────────┐
│  Core 端    │ ◄─────────────────────► │   Edge 端   │
│             │                          │             │
│ • 节点管理  │                          │ • 本地设备  │
│ • 数据汇聚  │                          │ • 数据采集  │
│ • API 服务  │                          │ • 配置同步  │
│             │                          │ • 离线缓存  │
└─────────────┘                          └─────────────┘
       │                                        │
       ▼                                        ▼
 ┌──────────┐                            ┌──────────┐
 │  Badger  │                            │  Badger  │
 │ (持久化) │                            │ (持久化) │
 └──────────┘                            └──────────┘
```

## 🚀 快速开始

### 环境要求

- Go 1.24.0 或更高版本
- Protocol Buffers Compiler (可选，仅用于开发)

### 安装

```bash
go get github.com/snple/beacon@latest
```

### 运行 Core 端

```go
package main

import (
    "github.com/dgraph-io/badger/v4"
    "github.com/snple/beacon/core"
    "google.golang.org/grpc"
    "net"
)

func main() {
    // 打开数据库
    opts := badger.DefaultOptions("/data/beacon/core")
    db, _ := badger.Open(opts)
    defer db.Close()

    // 创建 Core 服务
    cs, _ := core.Core(db)
    cs.Start()
    defer cs.Stop()

    // 启动 gRPC 服务器
    server := grpc.NewServer()
    cs.Register(server)

    lis, _ := net.Listen("tcp", ":50051")
    server.Serve(lis)
}
```

完整示例见 `bin/core/main.go`

### 运行 Edge 端

```go
package main

import (
    "github.com/snple/beacon/edge"
    "time"
)

func main() {
    // 创建 Edge 服务
    edgeOpts := []edge.EdgeOption{
        edge.WithNodeID("edge-001", "secret"),
        edge.WithNode(edge.NodeOptions{
            Enable: true,
            Addr:   "core-server:50051",
        }),
        edge.WithSync(edge.SyncOptions{
            Interval: time.Minute,
            Realtime: false,
        }),
    }

    es, _ := edge.Edge(edgeOpts...)
    es.Start()
    defer es.Stop()

    // ... 实现设备驱动逻辑
}
```

完整示例见 `bin/edge/main.go`

## 📖 核心概念

### Node (节点)

代表一个 IoT 设备或边缘节点。

### Wire (通道/端点)

代表设备的一个功能端点，例如：
- GPIO 端口组

### Pin (数据点)

代表具体的数据点或属性，例如：
- 温度传感器读数
- LED 开关状态
- 电机转速

### Cluster (集群/模板)

Pin 的逻辑分组，用于标准化设备定义，例如：
- `OnOff` - 开关控制
- `LevelControl` - 亮度/级别控制
- `TemperatureMeasurement` - 温度测量

## 🔧 使用示例

### 创建可调光灯

```go
import "github.com/snple/beacon/device"

// 使用预定义模板
result := device.BuildDimmableLightWire("bedroom_light")

// 设置类型和标签
result.Wire.Type = "DimmableLight"
result.Wire.Tags = []string{"category:light", "room:bedroom"}

// 配置 Pin 地址
for _, pin := range result.Pins {
    switch pin.Name {
    case "onoff":
        pin.Addr = "40001"  // Modbus 地址
    case "level":
        pin.Addr = "40002"
    }
}

// 保存到数据库或发送到 Core
// ...
```

### 读写 Pin 值

```go
import "github.com/snple/beacon/pb"

// 读取值
value, updated, err := storage.GetPinValue(nodeID, pinID)

// 写入值
nsonValue := &pb.NsonValue{
    Value: &pb.NsonValue_Bool{Bool: true},
}
err = storage.SetPinValue(ctx, nodeID, pinID, nsonValue, time.Now())
```

### 设备驱动示例

```go
// 定时采集温度
ticker := time.NewTicker(time.Second)
for range ticker.C {
    // 从传感器读取温度
    temp := readTemperature()

    // 保存到 Pin
    value := &pb.NsonValue{
        Value: &pb.NsonValue_F32{F32: temp},
    }
    storage.SetPinValue(ctx, pinID, value, time.Now())
}
```

## 📚 文档

- [架构设计](docs/ARCHITECTURE.md) - 深入了解系统架构
- [API 参考](docs/API_REFERENCE.md) - 完整的 API 文档
- [开发指南](docs/DEVELOPMENT.md) - 开发、测试、部署指南
- [项目分析](docs/PROJECT_ANALYSIS.md) - 代码质量分析和改进计划
- [设计文档](docs/) - 内存存储和 NSON 方案详解

## 🛠️ 开发

### 克隆项目

```bash
git clone https://github.com/snple/beacon.git
cd beacon
```

### 安装依赖

```bash
go mod download
```

### 编译 Protocol Buffers

```bash
make gen
```

### 运行测试

```bash
go test ./...
```

### 构建

```bash
# Core 端
go build -o bin/core/core ./bin/core

# Edge 端
go build -o bin/edge/edge ./bin/edge
```

## 🌟 适用场景

- ✅ 工业物联网数据采集
- ✅ 智能家居系统
- ✅ 边缘计算平台
- ✅ 设备管理系统
- ✅ 中小规模 IoT 项目

## ⚠️ 不适用场景

- ❌ 超大规模系统 (百万级设备)
- ❌ 需要复杂 SQL 查询的场景
- ❌ 强一致性要求的场景 (目前是最终一致性)

## 🤝 贡献

欢迎贡献代码、报告问题或提出建议！

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'feat: add some amazing feature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 提交 Pull Request

请参阅 [开发指南](docs/DEVELOPMENT.md) 了解更多详情。

## 📄 许可证

本项目采用 Apache 2.0 许可证。详见 [LICENSE](LICENSE) 文件。

## 🔗 相关项目

- [nson-go](https://github.com/danclive/nson-go) - NSON 序列化库
- [Badger](https://github.com/dgraph-io/badger) - 高性能嵌入式数据库

## 📧 联系方式

- Issues: [GitHub Issues](https://github.com/snple/beacon/issues)
- Discussions: [GitHub Discussions](https://github.com/snple/beacon/discussions)

---

⭐ 如果这个项目对你有帮助，请给个 Star！
