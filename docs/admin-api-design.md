# 进程内管理 API

管理程序与 broker 同进程，直接调用 `core` 的 Go 方法。HTTP、鉴权、前端由管理程序实现。

```go
import "github.com/snple/beacon/core"
```

## 观测

### Info

```go
type ServerInfo struct {
    ProtocolVersion uint8
    StartedAt       time.Time
    Uptime          time.Duration
    MaxClients      uint32
    Connected       int64
    Sessions        int64
    Listeners       []ListenerInfo
    Limits          packet.Limits
    MaxChannelLength int // channel 名/过滤器最大字节数（256–1024）
    Storage         string // 固定 "memory"
}

func (c *Core) Info() ServerInfo
func (c *Core) Listeners() []ListenerInfo
func (c *Core) Ready() bool
```

### 会话列表

```go
type ClientSummary struct {
    ClientID    string
    Connected   bool
    RemoteAddr  string
    ConnectedAt time.Time
    LastSeenAt  time.Time
    ExpiresAt   time.Time // 离线会话的过期时间；在线时为零值
    Pending     [3]uint32 // Low / Medium / High 未投递积压
    Delivered   uint32    // 已投递未确认
}

func (c *Core) ListClients() []ClientSummary // 按 ClientID 字典序，不分页
```

### 会话详情

```go
type ClientInfo struct {
    ClientID          string
    Connected         bool
    RemoteAddr        string
    ClientSessionID   uint64
    CreatedAt         time.Time
    ConnectedAt       time.Time
    LastSeenAt        time.Time
    ExpiresAt         time.Time
    SessionTimeout    uint32
    Pending           [3]uint32
    Delivered         uint32
    OldestUncommitted packet.PacketID
    Channels          []string
    Identity          *Identity
    MessagesSent      int64
    MessagesDropped   int64
    MessagesRejected  int64
}

func (c *Core) GetClientInfo(clientID string) (*ClientInfo, error) // 不存在：*ClientNotFoundError
```

### 统计

```go
func (c *Core) GetStats() StatsSnapshot
```

含计数器（收发/丢弃/拒绝/重复投递/接管次数）、内存水位（`InFlightMessages`、`InFlightBytes`）
与通配符过滤器数量。

## 写操作

### 断开连接

```go
func (c *Core) DisconnectClient(clientID string, reason packet.ReasonCode) error
func (c *Core) DisconnectClientAndWait(ctx context.Context, clientID string, reason packet.ReasonCode) error
```

服务端会先向客户端发送 `DISCONNECT(reason)` 再关闭连接。`reason` 建议用
`packet.ReasonKicked`，客户端据此不会自动重连。

### 推送

```go
type PushOptions struct {
    Priority packet.Priority // 默认 Medium
    Headers  map[string]string
}

type PushResult struct {
    ID      packet.PacketID
    Reason  packet.ReasonCode
    Matched uint32
    Dropped uint32
}

func (c *Core) Push(channel string, payload []byte, opts PushOptions) PushResult
func (c *Core) PushToClient(clientID, channel string, payload []byte, opts PushOptions) (PushResult, error)
```

- `Push` 以系统身份推送，**不做写权限校验**。
- `PushToClient` 设置 `Record.Target`，跳过 channel 匹配直接入该客户端队列；
  目标没有会话时返回 `*ClientNotFoundError`。
- `Matched` 表示入队成功的客户端数；`Dropped` 表示目标离线时的 Low 丢弃、
  因队列满被挤出的旧消息数（或新消息被丢弃）。
- `PushToClient` 在目标不存在、channel 非法等情况下返回 error；channel 非法时 `PushResult.Reason`
  同样会带上原因码。
- 优先级只能通过 `WithPriority` 设置：未设置时按 Medium；要显式使用 Low 请写成
  `core.PushOptions{}.WithPriority(packet.PriorityLow)`。
- `PushToClient` 要求目标已声明匹配该 channel 的过滤器，否则返回
  `ErrTargetNotSubscribed`（`PushResult.Reason = NoMatch`）。
- `Identity.AllowTargeted` 控制客户端是否允许定向投递；管理面 `Push`/`PushToClient`
  走系统身份，不受该限制。
- payload 与 headers 会被拷贝，调用方可以复用缓冲区。

## 不提供的操作

| 不提供 | 原因 / 替代做法 |
|---|---|
| 删除会话 | 断开连接后等待服务端 `SessionTimeout` 过期 |
| 枚举 channel | channel 是路由键，broker 不维护目录 |
| 清空某客户端队列 | 只能通过消费 + commit 消化，或断开后等会话过期 |
| 运行时改配置 | 需要重新创建 Core |
| 事件流 / 推送通知 | 用 `/metrics` + 结构化日志 + 轮询 `ListClients` |

## 事件与审计

没有内建事件流。需要审计时使用 `Hooks`：

```go
type MessageHandler interface {
    OnPush(ctx *PushContext) error        // 返回错误则拒绝该条消息
    OnDeliver(ctx *DeliverContext) bool   // 返回 false 则丢弃该次投递
}

type ConnectionHandler interface {
    OnConnect(ctx *ConnectContext)
    OnDisconnect(ctx *DisconnectContext)
}
```

`OnPush` 可用于 ACL、计费、审计日志；`OnDeliver` 可用于按客户端过滤消息内容。
两者都在调用方协程中同步执行，**不要在其中做阻塞 IO**，也不要在 `OnDeliver`
中调用 `DisconnectClient`/`DisconnectClientAndWait` 处理同一个客户端（会等待自身退出）。
钩子 panic 会被捕获并记录，不会带崩进程。

## 配额一览

| 选项 | 默认 | 作用 |
|---|---|---|
| `WithQueueCap(n)` | 10000 | 单客户端 pending 队列上限（0 = 不限） |
| `WithMaxRecords(n)` | 1000 | 单次 PULL 条数上限，同时约束 delivered 长度 |
| `WithMaxBytes(n)` | 1 MiB | 单次 PULL / PUSH 字节上限 |
| `WithMaxChannelsPerClient(n)` | 1024 | 单客户端声明数量上限 |
| `WithMaxChannelLength(n)` | 256 | channel 名/过滤器最大字节数（256–1024） |
| `WithPushRateLimit(rate, burst)` | 不限 | 每客户端发布速率 |
| `WithConnectionRateLimit(rate, burst)` | 不限 | 连接建立速率 |
| `WithMaxHandshakeConnections(n)` | 512 | 认证前并发连接上限 |
| `WithMaxClients(n)` | 10000 | 在线连接上限（握手阶段原子预占，不会超卖） |
| `WithMaxSessions(n)` | 10000 | 会话总数上限（含离线保留；<0 = 不限） |
| `WithMaxInflightBytes(n)` | 256 MiB | 全局在途消息字节上限（<0 = 不限） |
| `WithWaitLimit(max)` | 300s | 长轮询时长上限 |
| `WithMaxBatch(n)` | 16 | 单次 PUSH 记录数上限（1..128） |
| `WithMaxRecordBytes(n)` | 1 MiB | 单条记录上限（payload + channel/target/source/headers） |
| `WithSessionTimeout(seconds)` | 86400s | 会话保留时长（0 = 断开即清理） |
| `WithNoProgressLimits(soft, hard)` | 4 / 16 | 无进展 pull 的惩罚阈值（0 = 关闭） |
| `WithUncommittedTimeout(d)` | 关闭 | delivered 长期未确认超时 |
