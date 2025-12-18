# Queen 通信集成测试

这个目录包含了验证 Edge-Core Queen 通信的集成测试。

## 测试覆盖范围

### 1. 基本通信测试 (`TestQueenCommunication_Basic`)
验证 Edge 和 Core 之间通过 Queen 建立基本连接。

**测试内容**:
- Core 启动 Queen Broker
- Edge 使用 Queen 客户端连接
- 验证节点在线状态

### 2. 配置推送测试 (`TestQueenCommunication_ConfigPush`)
验证 Edge 将本地配置推送到 Core。

**测试内容**:
- Edge 创建 Wire 和 Pin 配置
- 通过 `beacon/push` 主题推送配置
- Core 接收并保存配置
- 验证 Core 端配置完整性

### 3. Pin 值同步测试 (`TestQueenCommunication_PinValue`)
验证 Pin 值从 Edge 同步到 Core。

**测试内容**:
- Edge 更新 Pin 值
- 通过 `beacon/pin/value` 主题上报
- Core 接收并保存 Pin 值
- 验证数据一致性

### 4. Pin 写入命令测试 (`TestQueenCommunication_PinWrite`)
验证 Core 向 Edge 发送写入命令。

**测试内容**:
- Core 创建 Pin 写入命令
- 通过 `beacon/pin/write` 通知 Edge
- Edge 通过 `beacon.pin.write.pull` 拉取命令
- Edge 执行写入操作
- 验证命令传递和执行

### 5. 重连功能测试 (`TestQueenCommunication_Reconnect`)
验证 Edge 在 Core 重启后能自动重连。

**测试内容**:
- 建立初始连接
- 模拟 Core 服务重启
- Edge 自动重连
- 验证重连后功能正常

### 6. 认证测试 (`TestQueenCommunication_Authentication`)
验证 Queen 认证机制的安全性。

**测试内容**:
- 使用错误 secret 连接（应失败）
- 使用正确 secret 连接（应成功）
- 验证认证逻辑正确性

## 运行测试

### 运行所有 Queen 测试

```bash
cd /home/danc/code/beacon
go test -v ./tests/integration/... -run TestQueenCommunication
```

### 运行单个测试

```bash
# 基本通信测试
go test -v ./tests/integration/... -run TestQueenCommunication_Basic

# 配置推送测试
go test -v ./tests/integration/... -run TestQueenCommunication_ConfigPush

# Pin 值同步测试
go test -v ./tests/integration/... -run TestQueenCommunication_PinValue

# Pin 写入命令测试
go test -v ./tests/integration/... -run TestQueenCommunication_PinWrite

# 重连功能测试
go test -v ./tests/integration/... -run TestQueenCommunication_Reconnect

# 认证测试
go test -v ./tests/integration/... -run TestQueenCommunication_Authentication
```

### 运行并显示详细日志

```bash
go test -v ./tests/integration/... -run TestQueenCommunication 2>&1 | tee test.log
```

## 测试架构

```
┌─────────────────────────────────────────┐
│         Integration Test                │
│                                         │
│  ┌─────────────┐      ┌──────────────┐ │
│  │ Core Service│◄────►│ Edge Service │ │
│  │             │      │              │ │
│  │ ┌─────────┐ │      │ ┌──────────┐ │ │
│  │ │  Queen  │ │      │ │  Queen   │ │ │
│  │ │  Broker │ │      │ │  Client  │ │ │
│  │ └────┬────┘ │      │ └────┬─────┘ │ │
│  │      │      │      │      │       │ │
│  │      │ TCP  │      │      │ TCP   │ │
│  │      └──────┼──────┼──────┘       │ │
│  │   :13883-888│      │              │ │
│  └─────────────┘      └──────────────┘ │
└─────────────────────────────────────────┘
```

## 测试数据流

### 配置推送流程
```
Edge Storage ─┐
              ├─► NSON Encode ─► beacon/push ─► Core Handler ─► Core Storage
              │
           Wire, Pin
```

### Pin 值上报流程
```
Edge Pin Value ─► NSON Array ─► beacon/pin/value ─► Core Handler ─► Core PinValue
```

### Pin 写入流程
```
Core SetWrite ─► beacon/pin/write (notify) ─► Edge Subscribe
                                               │
                                               ▼
                         Edge Pull ◄─ beacon.pin.write.pull (request/response)
                                               │
                                               ▼
                                          Execute Write
```

## 注意事项

1. **端口使用**: 测试使用端口 13883-13888，确保这些端口未被占用
2. **并发执行**: 测试使用不同端口，可以并发运行
3. **清理**: 测试使用内存存储，无需手动清理
4. **超时设置**: 测试中有适当的等待时间以确保异步操作完成

## 测试环境要求

- Go 1.24+
- Queen 包: `snple.com/queen`
- 测试工具: `github.com/stretchr/testify`
- NSON 编码: `github.com/danclive/nson-go`

## 故障排查

### 测试失败：连接超时

**可能原因**:
- 端口被占用
- 防火墙阻止连接
- Queen Broker 启动失败

**解决方法**:
```bash
# 检查端口占用
netstat -an | grep 13883

# 查看详细日志
go test -v ./tests/integration/... -run TestQueenCommunication_Basic 2>&1
```

### 测试失败：认证失败

**可能原因**:
- TOKEN_SALT 环境变量未设置
- Node Secret 不匹配

**解决方法**:
```bash
# 设置环境变量
export TOKEN_SALT="test-secret-key-for-jwt-signing"

# 重新运行测试
go test -v ./tests/integration/...
```

### 测试失败：数据未同步

**可能原因**:
- 等待时间不足
- 消息丢失（网络问题）

**解决方法**:
- 增加测试中的等待时间
- 检查 Queen 的 QoS 设置
- 查看 Edge 和 Core 的日志

## 扩展测试

如果需要添加新的测试用例：

1. 在 `queen_test.go` 中添加新的测试函数
2. 使用不同的端口避免冲突
3. 遵循现有的测试模式（Setup → Action → Assert）
4. 更新本 README 文档

示例:

```go
func TestQueenCommunication_YourFeature(t *testing.T) {
    // 1. Setup: 创建 Core 和 Edge
    // 2. Action: 执行待测试的操作
    // 3. Assert: 验证结果
}
```

## 性能测试

如果需要进行性能测试：

```bash
# 运行基准测试
go test -bench=. ./tests/integration/...

# 带内存分析
go test -bench=. -benchmem ./tests/integration/...

# 生成 CPU profile
go test -bench=. -cpuprofile=cpu.prof ./tests/integration/...
go tool pprof cpu.prof
```

## 参考文档

- [Queen 通信协议文档](../../docs/queen_communication.md)
- [Core Queen 实现](../../core/queen.go)
- [Edge Queen 实现](../../edge/queen_up.go)
- [Queen 协议处理器](../../core/queen_handlers.go)
