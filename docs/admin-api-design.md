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
    Channels          []ChannelInfo
    Identity          *Identity
    MessagesSent      int64
    MessagesDropped   int64
    MessagesRejected  int64
    MessagesNacked    int64
    MessagesExpired   int64
    ReceiveMaximum    uint32 // 当前生效的未确认可靠消息窗口
}

type ChannelInfo struct {
    Filter     string
    TargetOnly bool // 只接收定向投递，不参与广播匹配
}

func (c *Core) GetClientInfo(clientID string) (*ClientInfo, error) // 不存在：*ClientNotFoundError
```

### 统计

```go
func (c *Core) GetStats() StatsSnapshot
```

含计数器（收发/丢弃/拒绝/NACK/过期/重复投递/接管次数）、内存水位
（`InFlightMessages`、`InFlightBytes`）与通配符过滤器数量。

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
    Priority    packet.Priority // 默认 Medium
    Reliable    bool            // 仅 WithReliable 显式设置时生效
    Headers     map[string]string
    TTL         time.Duration   // WithTTL
    ExpiresAt   int64           // WithExpiresAt，Unix 毫秒
    PublishedAt int64           // WithPublishedAt，Unix 毫秒
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
  因队列满被挤出的旧消息数（或新消息被丢弃）、以及在线目标接收上限放不下的记录数。
- `PushToClient` 在目标不存在、channel 非法、目标在线且接收上限放不下记录等情况下
  返回 error；对应 `PushResult.Reason` 同样会带上原因码（如 `PacketTooLarge`）。
- 优先级只能通过 `WithPriority` 设置：未设置时按 Medium；要显式使用 Low 请写成
  `core.PushOptions{}.WithPriority(packet.PriorityLow)`。
- 可靠性独立于优先级：`WithReliable(true)` 表示必须 `COMMIT`（离线保留、占用窗口），
  `WithReliable(false)` 表示尽力而为。**默认不可靠**，需要投递保证时必须显式设置。
- `PushToClient` 要求目标已声明匹配该 channel 的过滤器，否则返回
  `ErrTargetNotSubscribed`（`PushResult.Reason = NoMatch`）。
- `Identity.AllowTargeted` 控制客户端是否允许定向投递；管理面 `Push`/`PushToClient`
  走系统身份，不受该限制。
- `WithTTL` / `WithExpiresAt` 设置过期时间；过期消息不会入队。
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

没有内建事件流。需要审计时使用 `Hooks`（每个钩子独立注册，互不依赖）：

```go
type AuthHandler interface {
    Authenticate(ctx *AuthContext) error      // 返回错误则拒绝连接
}

type PushHandler interface {
    OnPush(ctx *PushContext) error            // 返回错误则拒绝该条消息
}

type DeliverHandler interface {
    OnDeliver(ctx *DeliverContext) bool       // 返回 false 则丢弃该次投递
}

type ConnectHandler interface     { OnConnect(ctx *ConnectContext) }
type DisconnectHandler interface  { OnDisconnect(ctx *DisconnectContext) }
type DeclareHandler interface     { OnDeclare(ctx *DeclareContext) }         // 通知型
type SessionEndHandler interface  { OnSessionEnd(ctx *SessionEndContext) }   // 通知型
type PullHandler interface        { OnPull(ctx *PullContext) }               // 通知 + 响应属性
type NackHandler interface        { OnNack(ctx *NackContext) }
```

函数适配器为 `AuthHandlerFunc`、`PushHandlerFunc`、`DeliverHandlerFunc`、
`ConnectHandlerFunc`、`DisconnectHandlerFunc`、`DeclareHandlerFunc`、
`SessionEndHandlerFunc`、`PullHandlerFunc`、`NackHandlerFunc`。

`OnPush` 可用于 ACL、计费、审计日志；`OnDeliver` 可用于按客户端过滤消息内容；
`OnNack` 可用于死信、告警或补偿；`OnDeclare` 可审计声明变更；`OnSessionEnd`
补齐会话过期/断开即清理的观测闭环。

### 改写消息

`PushContext` 的 `Headers`、`Payload`、`Priority`、`ExpiresAt` 可被钩子改写，
服务端会在入队前应用并**重新校验** `MaxRecordBytes`（超限整条拒绝）。
`Channel`、`Target`、`Source`、`ID`、`Reliable` 只读——路由与信任字段已经校验过，
改动会绕过 ACL；需要改路由请在钩子内重新发布。

```go
Push: core.PushHandlerFunc(func(ctx *core.PushContext) error {
    ctx.SetHeader("trace-id", traceID()) // Headers 可能为 nil
    if !allowed(ctx.Channel) {
        return &core.HookError{Reason: packet.ReasonNotAuthorized}
    }
    return nil
}),
```

### 拒绝原因

`OnPush` 返回 `*HookError`（`*AuthError` 是它的别名）可以指定拒绝原因码，并可选地
通过 `Detail` 回传诊断信息（写入 `PUSH_OK` 的 `error` 属性）。普通 `error` 一律映射为
`NotAuthorized`，且不向客户端泄露错误文本；`Reason` 不是失败码时同样回退为
`NotAuthorized`，避免“拒绝却回 Success”：

```go
return &core.HookError{
    Reason: packet.ReasonQuotaExceeded,
    Detail: "rate limit 100/s",
}
```

### 响应属性

响应属性由**对应操作的钩子自己设置**，不引入额外的“响应钩子”——每个钩子只处理
自己那次操作的请求与响应，不需要再按响应类型分派：

| 响应 | 由谁设置 | 说明 |
|---|---|---|
| `CONNACK` | `AuthContext.ResponseProperties` | 与 `WithServerProperties` 合并，钩子同名键覆盖；仅认证成功时生效 |
| `PUSH_OK` | `PushContext.ResponseProperties` | 仅在 `WantAck=1` 时会发出响应；与 `error` 细节合并 |
| `PULL_OK` | `PullContext.ResponseProperties` | `OnPull` 每个 PULL 请求调用一次（长轮询复用，不会重复触发） |
| `DECLARE_RESP` | `DeclareContext.ResponseProperties` | 与 `OnDeclare` 同一次回调 |

```go
hooks := &core.Hooks{
    Push: core.PushHandlerFunc(func(ctx *core.PushContext) error {
        ctx.SetHeader("trace-id", traceID())
        ctx.ResponseProperties = map[string]string{"trace-id": traceID()}
        return nil
    }),
    Pull: core.PullHandlerFunc(func(ctx *core.PullContext) {
        ctx.ResponseProperties = map[string]string{"server-time": now()}
    }),
}
```

`OnPull` 还能读到请求侧的 `Properties`、`WaitMs`、`MaxCount`、`MaxBytes`
（均为客户端原始值，未经服务端裁剪），便于审计与按请求返回提示。

大小约束：

- `PUSH_OK` / `PULL_OK` / `DECLARE_RESP` 的属性放不下客户端声明的 `MaxPacketSize` 时
  **整组丢弃**并记 Warn，不影响响应本身发出；PULL 的属性大小已计入批量预算，
  DECLARE_RESP 的属性会先从 `Results` 裁剪预算里扣除。
- `CONNACK` 保持原有语义：最终 `CONNACK`（含属性）超限时返回 `PacketTooLarge` 并关闭
  连接，此时 `PUSH_OK` 之类的“丢属性”降级不适用——握手阶段必须显式失败。
- 客户端读取：`ServerProperties()`（CONNACK）、`PushResult.Properties`（PUSH_OK）、
  `PullBatchResult`（PULL_OK）、`DeclareChannelsResponse`（DECLARE_RESP）。

### 上下文字段

命名约定：`Properties` 是**本次请求**携带的属性（只读），`ResponseProperties`
是钩子填写、**随响应**返回的属性；两者在所有上下文里保持同名同义。

| 钩子 | 可用字段 |
|---|---|
| `AuthContext` | `ClientID`、`RemoteAddr`、`ProtocolVersion`、`CleanSession`、`KeepAlive`、`MaxPacketSize`、`AuthMethod`、`AuthData`、`Properties`、`Identity`（出参）、`ResponseProperties`（出参） |
| `PushContext` | 只读：`ID`、`ClientID`、`Identity`、`Channel`、`Target`、`FromServer`、`Reliable`、`PublishedAt`、`Properties`；可写：`Headers`、`Payload`、`Priority`、`ExpiresAt`、`ResponseProperties` |
| `DeliverContext` | `ID`、`ClientID`、`Identity`、`Channel`、`Source`、`Target`、`Priority`、`Reliable`、`ExpiresAt`、`PublishedAt`、`Headers`、`Payload`、`Dup` |
| `NackContext` | `ID`、`ClientID`、`Identity`、`Channel`、`Source`、`Target`、`Priority`、`Reliable`、`Reason`、`Headers`、`Payload` |
| `ConnectContext` | `ClientID`、`SessionID`、`RemoteAddr`、`SessionPresent`、`CleanSession`、`Identity`、`Properties` |
| `DisconnectContext` | `ClientID`、`SessionID`、`RemoteAddr`、`Reason`、`Err`、`Identity` |
| `DeclareContext` | `ClientID`、`Identity`、`Properties`、`Requested`、`Accepted`、`Rejected`、`ResponseProperties` |
| `SessionEndContext` | `ClientID`、`SessionID`、`Identity`、`Reason`、`MessagesDropped` |
| `PullContext` | `ClientID`、`Identity`、`Properties`、`WaitMs`、`MaxCount`、`MaxBytes`、`ResponseProperties` |

`SessionID` 是会话租约标识，同一会话在重连后保持不变，接管或 `CleanSession` 时更换；
`OnConnect`/`OnDisconnect` 携带的是该连接自己的租约快照，接管事件不会串到新会话。
`OnDeliver` 对新消息与**重发**都会调用（`Dup=true` 表示重发），因此运行期改变投递策略
会立即生效；被拒绝的消息从队列中丢弃（计入 `MessagesDropped`），不会反复重发。
`OnDeclare`/`OnSessionEnd`/`OnPull` 是通知型钩子，不能否决操作。
`OnSessionEnd.MessagesDropped` 是随会话一起释放的队列消息条数。
`OnDisconnect` 只在 `OnConnect` 已触发过的连接上调用，二者一一对应。

### 组合与统计

需要多个处理器时用组合器，顺序由调用方决定：

```go
Push: core.ChainPush(metricsHook, aclHook, auditHook),   // 任一返回错误即中止
Connect: core.ChainConnect(metricsHook, auditHook),      // 全部执行
```

- `ChainPush` / `ChainDeliver` 短路：首个拒绝生效；
- `ChainConnect` / `ChainDisconnect` / `ChainDeclare` / `ChainSessionEnd` /
  `ChainPull` / `ChainNack` 全部执行；
- 组合器内部 panic 会中止整条链，由 Core 统一捕获并记为一次 panic；
- 不提供 `ChainAuth`：认证是单一仲裁，串联语义容易误解。

各钩子的调用次数与 panic 次数计入 `Stats.Hooks`（键为钩子名），
并通过 `/metrics` 暴露为 `beacon_hook_calls_total{hook="push"}` 与
`beacon_hook_panics_total{hook="push"}`。

### 执行模型

所有钩子都在调用方协程中同步执行，**不要在其中做阻塞 IO**，也不要在 `OnDeliver`
中调用 `DisconnectClient`/`DisconnectClientAndWait` 处理同一个客户端（会等待自身退出）。
`Identity`、`Headers`、`Payload`、`Properties` 直接引用会话状态或共享消息内容，
除 `PushContext` 标注为可写的字段外**必须视为只读**；修改会影响其他客户端、ACL 判断
和后续重发。`OnDeliver` 对每条记录 × 每个命中客户端 × 每次投递调用一次，
适合做内存内的快速判断，重活应丢进内部队列。
`DisconnectContext.Err` 会带上连接结束时的 I/O/协议错误（正常断开时为 nil）。
钩子 panic 会被捕获并记录（含堆栈），不会带崩进程：`OnPush`/`Authenticate` 视为拒绝，
`OnDeliver` 视为丢弃，事件类钩子只记录日志。

`Core` 在创建时对 `Hooks` 做一次浅拷贝，因此需要在 `NewWithOptions` 之前注册完毕；
运行期替换钩子字段不会生效，但钩子实现可以自行维护可变状态（需保证并发安全）。

## 配额一览

| 选项 | 默认 | 作用 |
|---|---|---|
| `WithQueueCap(n)` | 10000 | 单客户端 pending 队列上限（0 = 不限） |
| `WithMaxPendingBytesPerClient(n)` | 不限 | 单客户端 pending 近似字节上限（0 = 不限） |
| `WithMaxChannelsPerClient(n)` | 1024 | 单客户端声明数量上限 |
| `WithMaxChannelLength(n)` | 256 | channel 名/过滤器最大字节数（256–1024） |
| `WithPushRateLimit(rate, burst)` | 不限 | 每客户端发布速率 |
| `WithConnectionRateLimit(rate, burst)` | 不限 | 连接建立速率 |
| `WithMaxHandshakeConnections(n)` | 512 | 认证前并发连接上限 |
| `WithMaxClients(n)` | 10000 | 在线连接上限（握手阶段原子预占，不会超卖） |
| `WithMaxSessions(n)` | 10000 | 会话总数上限（含离线保留；<0 = 不限） |
| `WithMaxInflightBytes(n)` | 256 MiB | 全局在途消息字节上限（<0 = 不限） |
| `WithWaitLimit(max)` | 300s | 长轮询时长上限 |
| `WithMaxKeepAlive(s)` | 300s | 最终心跳间隔上限（防止超长 KeepAlive 占用连接名额） |
| `WithMaxRecordBytes(n)` | 1 MiB | 单条记录上限（payload + channel/target/source/headers） |
| `WithSessionTimeout(seconds)` | 86400s | 会话保留时长（0 = 断开即清理） |
| `WithNoProgressLimits(soft, hard)` | 4 / 16 | 无进展 pull 的惩罚阈值（0 = 关闭） |
| `WithUncommittedTimeout(d)` | 关闭 | delivered 长期未确认超时 |
| `WithReceiveMaximum(n)` | 32 | 每会话未确认可靠消息窗口（1–1024，服务端决定） |
| `WithServerProperties(map)` | 空 | 随 CONNACK 返回给客户端的键值信息 |
| `WithMaxPullBatch(n)` | 32 | 单次 PULL 最多返回记录数（1–1024） |

`PULL` / `COMMIT` 支持批量；未确认可靠消息受每会话窗口 `ReceiveMaximum` 约束（默认 32）。
连接/会话相关的限制（包大小、记录大小、长轮询、channel 数/长度、批量拉取、队列容量、
会话保留）同时通过 `CONNACK.Limits` 下发，`DECLARE_RESP.Limits` 会再次返回。
