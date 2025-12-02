# Beacon 开发指南

## 📋 目录

1. [环境准备](#环境准备)
2. [项目结构](#项目结构)
3. [开发流程](#开发流程)
4. [构建和运行](#构建和运行)
5. [测试](#测试)
6. [代码规范](#代码规范)
7. [调试技巧](#调试技巧)
8. [常见问题](#常见问题)

## 环境准备

### 必需软件

- **Go**: 1.24.0 或更高版本
- **Protocol Buffers Compiler**: 用于生成 gRPC 代码
- **Make**: 用于运行构建脚本
- **Git**: 版本控制

### 安装 Go

```bash
# Linux/macOS
wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# 验证安装
go version
```

### 安装 Protocol Buffers

```bash
# Linux
sudo apt-get install protobuf-compiler

# macOS
brew install protobuf

# 验证安装
protoc --version
```

### 安装 Go 插件

```bash
# Protocol Buffers Go 插件
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 确保插件在 PATH 中
export PATH=$PATH:$(go env GOPATH)/bin
```

### 克隆项目

```bash
git clone https://github.com/snple/beacon.git
cd beacon
```

### 安装依赖

```bash
# 下载所有依赖
go mod download

# 整理依赖
go mod tidy
```

## 项目结构

```
beacon/
├── bin/                    # 可执行文件和示例
│   ├── core/              # Core 端示例
│   │   ├── main.go
│   │   ├── config/
│   │   └── log/
│   └── edge/              # Edge 端示例
│       ├── main.go
│       ├── config/
│       └── log/
├── client/                # 客户端库
│   ├── core/             # Core 客户端
│   └── edge/             # Edge 客户端
├── core/                  # Core 端实现
│   ├── core.go           # Core 服务
│   ├── node.go           # Node 服务
│   ├── wire.go           # Wire 服务
│   ├── pin.go            # Pin 服务
│   ├── pin_value.go      # PinValue 服务
│   ├── pin_write.go      # PinWrite 服务
│   ├── sync.go           # 同步服务
│   ├── node/             # Node 管理服务
│   └── storage/          # 存储层
│       └── storage.go
├── edge/                  # Edge 端实现
│   ├── edge.go           # Edge 服务
│   ├── node.go
│   ├── wire.go
│   ├── pin.go
│   ├── sync.go
│   ├── node_up.go        # 连接 Core 的客户端
│   ├── badger.go         # Badger 管理
│   └── storage/          # 存储层
│       └── storage.go
├── device/                # 设备抽象层
│   ├── cluster.go        # Cluster 定义
│   ├── builder.go        # Wire 构建器
│   └── README.md
├── dt/                    # 数据类型定义
│   ├── dt.go
│   └── nson.go
├── pb/                    # Protocol Buffers 生成代码
│   ├── cores/
│   ├── edges/
│   └── nodes/
├── proto/                 # Protocol Buffers 定义
│   ├── common.proto
│   ├── cores/
│   ├── edges/
│   └── nodes/
├── tcp/                   # TCP 协议支持
│   └── node/
├── util/                  # 工具函数
│   ├── compress/         # 压缩算法
│   └── tls.go           # TLS 配置
├── docs/                  # 文档
│   ├── ARCHITECTURE.md
│   ├── API_REFERENCE.md
│   ├── DEVELOPMENT.md
│   └── PROJECT_ANALYSIS.md
├── go.mod                 # Go 模块定义
├── go.sum
├── makefile              # 构建脚本
├── version.go            # 版本信息
├── LICENSE
└── README.md
```

## 开发流程

### 1. 分支管理

```bash
# 创建功能分支
git checkout -b feature/my-feature

# 创建修复分支
git checkout -b fix/my-fix

# 提交代码
git add .
git commit -m "feat: add new feature"

# 推送到远程
git push origin feature/my-feature
```

### 2. 编译 Protocol Buffers

修改 `.proto` 文件后需要重新生成代码:

```bash
make gen
```

这会执行以下操作:
1. 编译所有 `.proto` 文件
2. 生成 Go 代码到 `github.com/snple/beacon/pb/`
3. 复制到 `pb/` 目录
4. 清理临时文件

### 3. 添加新功能

#### 添加新的 gRPC 服务

1. 在 `proto/` 中定义 `.proto` 文件
2. 运行 `make gen` 生成代码
3. 在 `core/` 或 `edge/` 中实现服务
4. 在 `core.go` 或 `edge.go` 中注册服务

示例:

```protobuf
// proto/cores/my_service.proto
syntax = "proto3";
package cores;
option go_package = "github.com/snple/beacon/pb/cores";

service MyService {
  rpc MyMethod(MyRequest) returns (MyResponse);
}

message MyRequest {
  string id = 1;
}

message MyResponse {
  string result = 1;
}
```

```go
// core/my_service.go
package core

import (
    "context"
    "github.com/snple/beacon/pb/cores"
)

type MyServiceServer struct {
    cores.UnimplementedMyServiceServer
    cs *CoreService
}

func newMyService(cs *CoreService) *MyServiceServer {
    return &MyServiceServer{cs: cs}
}

func (s *MyServiceServer) MyMethod(ctx context.Context, in *cores.MyRequest) (*cores.MyResponse, error) {
    // 实现逻辑
    return &cores.MyResponse{Result: "ok"}, nil
}

// 在 core.go 中注册
func (cs *CoreService) Register(server *grpc.Server) {
    // ... 现有注册
    cores.RegisterMyServiceServer(server, cs.myService)
}
```

#### 添加新的 Cluster

```go
// device/cluster.go

// 定义 Cluster
var MyDeviceCluster = Cluster{
    ID:          0x9999,
    Name:        "MyDevice",
    Description: "我的自定义设备",
    Pins: []PinTemplate{
        {
            Name:    "value",
            Desc:    "数值",
            Type:    dt.TypeI32,
            Rw:      1,
            Default: nson.I32(0),
            Tags:    "custom",
        },
    },
}

// 注册到全局注册表
func init() {
    RegisterCluster(&MyDeviceCluster)
}

// 创建构建器函数
func BuildMyDeviceWire(name string) *SimpleBuildResult {
    return NewWireBuilder(name).
        WithCluster("MyDevice").
        Build()
}
```

## 构建和运行

### 构建 Core 端

```bash
# 构建
go build -o bin/core/core ./bin/core

# 运行
./bin/core/core

# 指定配置文件
./bin/core/core -config config.toml
```

### 构建 Edge 端

```bash
# 构建
go build -o bin/edge/edge ./bin/edge

# 首次运行需要 seed (初始化节点)
./bin/edge/edge seed "EdgeNode01"

# 运行
./bin/edge/edge

# 手动推送配置
./bin/edge/edge push
```

### 交叉编译

```bash
# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o bin/edge/edge-linux-arm64 ./bin/edge

# Linux ARM (32位)
GOOS=linux GOARCH=arm GOARM=7 go build -o bin/edge/edge-linux-arm ./bin/edge

# Windows
GOOS=windows GOARCH=amd64 go build -o bin/edge/edge.exe ./bin/edge

# macOS
GOOS=darwin GOARCH=arm64 go build -o bin/edge/edge-darwin-arm64 ./bin/edge
```

### Docker 构建

```bash
# 创建 Dockerfile (Core)
cat > Dockerfile.core <<EOF
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -o /beacon-core ./bin/core

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /beacon-core /usr/local/bin/
EXPOSE 50051
CMD ["beacon-core"]
EOF

# 构建镜像
docker build -f Dockerfile.core -t beacon-core:latest .

# 运行
docker run -d -p 50051:50051 -v /data:/data beacon-core
```

## 测试

### 运行单元测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./core/storage

# 运行特定测试函数
go test -run TestGetNode ./core/storage

# 显示详细输出
go test -v ./...

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### 编写测试

#### 单元测试示例

```go
// core/storage/storage_test.go
package storage_test

import (
    "testing"
    "time"

    "github.com/dgraph-io/badger/v4"
    "github.com/snple/beacon/core/storage"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestStorage_GetNode(t *testing.T) {
    // 准备测试数据库
    opts := badger.DefaultOptions("").WithInMemory(true)
    db, err := badger.Open(opts)
    require.NoError(t, err)
    defer db.Close()

    // 创建存储
    s := storage.New(db)

    // 准备测试数据
    node := &storage.Node{
        ID:      "test-node",
        Name:    "TestNode",
        Status:  1,
        Updated: time.Now(),
        Wires:   []storage.Wire{},
    }

    // 推送数据
    data, err := encodeNode(node)
    require.NoError(t, err)

    err = s.Push(context.Background(), data)
    require.NoError(t, err)

    // 测试查询
    result, err := s.GetNode("test-node")
    assert.NoError(t, err)
    assert.Equal(t, "test-node", result.ID)
    assert.Equal(t, "TestNode", result.Name)

    // 测试不存在的节点
    _, err = s.GetNode("non-existent")
    assert.Error(t, err)
}
```

#### 集成测试示例

```go
// integration_test.go
package beacon_test

import (
    "context"
    "testing"
    "time"

    "github.com/snple/beacon/core"
    "github.com/snple/beacon/edge"
    "google.golang.org/grpc"
)

func TestCoreEdgeIntegration(t *testing.T) {
    // 启动 Core
    coreService := startTestCore(t)
    defer coreService.Stop()

    // 启动 Edge
    edgeService := startTestEdge(t, coreService.Address())
    defer edgeService.Stop()

    // 测试同步
    err := edgeService.Push()
    assert.NoError(t, err)

    // 验证数据
    node, err := coreService.GetStorage().GetNode(edgeService.GetStorage().GetNodeID())
    assert.NoError(t, err)
    assert.NotNil(t, node)
}
```

### 基准测试

```go
// core/storage/storage_bench_test.go
package storage_test

import (
    "testing"
    "github.com/snple/beacon/core/storage"
)

func BenchmarkStorage_GetNode(b *testing.B) {
    s := setupTestStorage(b)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = s.GetNode("test-node")
    }
}

func BenchmarkStorage_GetPinByID(b *testing.B) {
    s := setupTestStorage(b)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = s.GetPinByID("test-pin")
    }
}
```

运行基准测试:

```bash
go test -bench=. -benchmem ./core/storage
```

### 竞态检测

```bash
# 检测竞态条件
go test -race ./...

# 构建时启用竞态检测
go build -race -o bin/core/core-race ./bin/core
```

## 代码规范

### Go 代码规范

遵循官方 Go 代码规范:

1. **格式化**: 使用 `gofmt` 或 `goimports`

```bash
# 格式化所有代码
gofmt -w .

# 使用 goimports (自动管理 import)
go install golang.org/x/tools/cmd/goimports@latest
goimports -w .
```

2. **命名规范**:
   - 包名: 小写单词,无下划线
   - 导出函数: 大写开头,驼峰命名
   - 私有函数: 小写开头,驼峰命名
   - 常量: 驼峰命名或全大写+下划线

3. **注释规范**:

```go
// Package core 提供 Beacon Core 端服务实现
//
// Core 服务管理多个 Edge 节点,负责配置管理和数据汇聚。
package core

// CoreService Core 端主服务
//
// 管理所有 Edge 节点的连接、配置和数据同步。
type CoreService struct {
    // ...
}

// Start 启动 Core 服务
//
// 此方法会加载所有持久化数据到内存,启动 gRPC 服务器。
// 如果启动失败,返回错误。
func (cs *CoreService) Start() error {
    // ...
}
```

### Linting

使用 golangci-lint 进行代码检查:

```bash
# 安装 golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# 运行检查
golangci-lint run

# 自动修复
golangci-lint run --fix
```

配置 `.golangci.yml`:

```yaml
linters:
  enable:
    - gofmt
    - govet
    - errcheck
    - staticcheck
    - unused
    - gosimple
    - structcheck
    - varcheck
    - ineffassign
    - deadcode
    - typecheck

linters-settings:
  errcheck:
    check-blank: true
```

### 提交规范

使用 Conventional Commits:

```
类型(范围): 简短描述

详细描述

相关 Issue: #123
```

类型:
- `feat`: 新功能
- `fix`: 修复 bug
- `docs`: 文档更新
- `style`: 代码格式调整
- `refactor`: 重构
- `test`: 测试相关
- `chore`: 构建/工具链更新

示例:

```
feat(core): 添加节点批量导入功能

实现了从 JSON 文件批量导入节点配置的功能,
支持验证和回滚机制。

相关 Issue: #45
```

## 调试技巧

### 日志调试

```go
import "go.uber.org/zap"

// 使用结构化日志
logger.Info("processing node",
    zap.String("node_id", nodeID),
    zap.String("node_name", node.Name),
    zap.Int("wire_count", len(node.Wires)),
)

logger.Error("failed to save node",
    zap.String("node_id", nodeID),
    zap.Error(err),
)

// 开发环境使用 Debug 级别
logger.Debug("pin value updated",
    zap.String("pin_id", pinID),
    zap.Any("value", value),
)
```

### Delve 调试器

```bash
# 安装 Delve
go install github.com/go-delve/delve/cmd/dlv@latest

# 调试程序
dlv debug ./bin/core

# 在调试器中
(dlv) break main.main
(dlv) continue
(dlv) print nodeID
(dlv) step
(dlv) quit
```

### pprof 性能分析

```go
import _ "net/http/pprof"

// 在 main 中启动 pprof 服务器
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

访问性能分析:

```bash
# CPU 分析
go tool pprof http://localhost:6060/debug/pprof/profile

# 内存分析
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutine 分析
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

### gRPC 调试

使用 grpcurl 测试 gRPC API:

```bash
# 安装 grpcurl
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# 列出所有服务
grpcurl -plaintext localhost:50051 list

# 列出服务的方法
grpcurl -plaintext localhost:50051 list nodes.NodeService

# 调用方法
grpcurl -plaintext -d '{"id": "node-001"}' \
    localhost:50051 nodes.NodeService/View
```

## 常见问题

### Q1: 编译 proto 文件失败

**问题**: 运行 `make gen` 时报错

**解决方案**:
1. 确认 protoc 已安装并在 PATH 中
2. 确认 Go 插件已安装:
   ```bash
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
   ```
3. 确认 `$GOPATH/bin` 在 PATH 中

### Q2: Badger 数据库锁定

**问题**: 程序启动时报 "database locked"

**解决方案**:
1. 确认没有其他实例正在运行
2. 删除锁文件:
   ```bash
   rm /data/beacon/LOCK
   ```
3. 使用 defer 确保数据库正确关闭:
   ```go
   db, _ := badger.Open(opts)
   defer db.Close()
   ```

### Q3: gRPC 连接失败

**问题**: Edge 无法连接到 Core

**解决方案**:
1. 检查网络连通性
2. 检查防火墙规则
3. 验证 TLS 证书配置
4. 使用 `grpcurl` 测试连接
5. 检查 Core 服务是否正在监听正确的端口

### Q4: 内存占用过高

**问题**: 运行一段时间后内存持续增长

**解决方案**:
1. 使用 pprof 分析内存:
   ```bash
   go tool pprof http://localhost:6060/debug/pprof/heap
   ```
2. 检查是否有 goroutine 泄漏:
   ```bash
   go tool pprof http://localhost:6060/debug/pprof/goroutine
   ```
3. 确认 Badger GC 正常运行
4. 考虑清理不常用的索引

### Q5: 测试覆盖率低

**问题**: 代码覆盖率不足

**解决方案**:
1. 为核心逻辑添加单元测试
2. 为 API 添加集成测试
3. 使用 table-driven tests 减少重复
4. 目标覆盖率 70%+

### Q6: 性能不达标

**问题**: 查询响应慢

**解决方案**:
1. 使用基准测试定位瓶颈
2. 确认索引正确构建
3. 考虑增加缓存
4. 优化数据结构
5. 使用 profiling 工具分析

## 发布流程

### 1. 版本号管理

编辑 `version.go`:

```go
package beacon

const Version = "v1.2.3"
```

### 2. 更新 CHANGELOG

记录版本变更:

```markdown
## [1.2.3] - 2025-12-02

### Added
- 新增批量导入功能
- 添加性能监控指标

### Fixed
- 修复内存泄漏问题
- 修复并发竞态条件

### Changed
- 优化查询性能
- 更新依赖版本
```

### 3. 打 Tag

```bash
git tag -a v1.2.3 -m "Release v1.2.3"
git push origin v1.2.3
```

### 4. 构建发布包

```bash
# 构建多平台二进制
./scripts/build-release.sh v1.2.3

# 生成校验和
sha256sum dist/* > dist/checksums.txt
```

### 5. GitHub Release

在 GitHub 上创建 Release,上传构建产物。

## 相关资源

- [Go 官方文档](https://golang.org/doc/)
- [gRPC Go 教程](https://grpc.io/docs/languages/go/)
- [Badger 文档](https://dgraph.io/docs/badger/)
- [NSON 格式](https://github.com/danclive/nson)
- [项目架构](ARCHITECTURE.md)
- [API 参考](API_REFERENCE.md)
