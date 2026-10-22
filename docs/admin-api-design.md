# Beacon 管理面 API（进程内）

管理程序与 broker 同进程，直接调用 `core` 的 Go 方法；HTTP、鉴权、前端由管理程序实现。

- **观测**：`ListClients`（概要）+ `GetClientInfo`（详情）+ `Info`/`GetStats` + 事件流
- **写**：只有踢下线；`Publish`/`PublishToClient`/`Broadcast` 属消息面
- **不提供**：删除会话、保留/主题/订阅枚举、分页、运行时改配置（替代做法见「常见配方」）

```go
import "github.com/snple/beacon/core"
```

## 快速开始

```go
c, err := core.NewWithOptions(core.NewCoreOptions().
    WithBuildInfo("1.2.3", "abc123", "2026-01-01T00:00:00Z"))
if err != nil {
    log.Fatal(err)
}
if err := c.Start(); err != nil {
    log.Fatal(err)
}
defer c.Stop()

go func() { _ = c.ServeTCP(":3883") }()
```

## 客户端列表

```go
type ClientSummary struct {
    ClientID    string
    Connected   bool      // false = 保留中的离线会话
    RemoteAddr  string    // 离线时为空
    ConnectedAt time.Time // 最近一次建立连接的时间
    LastSeenAt  time.Time // 最后一次活动时间
    ExpiresAt   time.Time // 离线会话过期时间；在线时为零值
}

func (c *Core) ListClients() []ClientSummary
```

```go
for _, s := range c.ListClients() {
    fmt.Printf("%s online=%v addr=%s lastSeen=%s\n",
        s.ClientID, s.Connected, s.RemoteAddr, s.LastSeenAt)
}
```

含离线会话，按 `ClientID` 字典序，**不分页**；只组装概要字段，不读队列/存储。

## 客户端详情

```go
func (c *Core) GetClientInfo(clientID string) (*ClientInfo, error) // 不存在：*ClientNotFoundError
```

```go
info, err := c.GetClientInfo("device-42")
if err != nil {
    return err
}
fmt.Println(info.Connected, info.QueueSize, info.PendingAck, info.MessagesSent)
for _, sub := range info.Subscriptions {
    fmt.Println(sub.Topic, sub.QoS)
}
```

`ClientInfo` 字段：

| 分组 | 字段 |
|---|---|
| 标识 | `ClientID`、`Transport`（tcp/tls/ws）、`ProtocolVersion`、`RemoteAddr` |
| 配置 | `KeepAlive`、`SessionTimeout`、`ReceiveWindow` |
| 状态 | `Connected`、`ConnectedAt`、`LastSeenAt`、`ExpiresAt`、`WillTopic`、`Identity`、`Subscriptions` |
| 积压/统计 | `QueueSize`、`QueueBytes`、`OldestQueuedAge`、`PendingAck`、`OldestPendingAckAge`、`MessagesSent/Dropped/Rejected`、`BytesSent/Received` |

`Subscriptions` 元素为 `ClientSubscriptionInfo{Topic string; packet.SubscribeOptions}`。

## 服务器信息与统计

```go
func (c *Core) Info() ServerInfo
func (c *Core) Ready() bool                 // 已 Start 且存储可用
func (c *Core) Listeners() []ListenerInfo
func (c *Core) GetStats() StatsSnapshot
func (c *Core) GetLogger() *zap.Logger
```

```go
info := c.Info()
fmt.Println(info.Version, info.Uptime, info.Listeners, info.Store.Bytes)

// 使用率 = 当前值 / 上限
stats, limits := c.GetStats(), c.Info().Limits
fmt.Printf("clients %d/%d, store %d/%d\n",
    stats.ClientsConnected, limits.MaxClients, stats.StoreBytes, limits.MaxStoreBytes)
```

- `ServerInfo`：`Version`/`Commit`/`BuildTime`（来自 `WithBuildInfo`）、`Protocol`（`"MX/2"`）、`StartedAt`（Core 构造时间）、`Uptime`、`Listeners`、`Limits`、`Store`。
- `StatsSnapshot`：`StartedAt`、`Limits`、`EventsDropped`、`ClientsConnected/Total`、`MessagesReceived/Sent/Dropped/Rejected`、`BytesReceived/Sent`、`SubscriptionsCount`、`PendingQueueDepth`、`ClientQueueDepth`、`PendingAckTotal`、`OfflineSessions`、`StoreBytes`。
- `Limits`：`MaxClients`、`MaxHandshakeConnections`、`MaxPacketSize`、`ReceiveWindow`、`MaxClientQueueSize`、`MaxPendingMessages`、`MaxStoreBytes`、`MaxOfflineSessions`、`MaxPublishRate`、`WriteTimeout`。

## 主题发布

```go
type PublishResult struct {
    MatchedSubscribers int  // 入队瞬间命中的订阅者数（含离线会话，估算值）
    Retained           bool // 是否写入保留存储
    Enqueued           bool // 是否进入全局分发队列
}

type PublishOptions struct {
    QoS             packet.QoS
    Retain          bool
    TraceID         string
    ContentType     string
    Expiry          uint32 // 秒；0 用 DefaultMessageExpiry
    UserProperties  map[string]string
    ResponseTopic   string
    CorrelationData []byte
    PublisherID     string // 发布身份（影响 NoLocal 与审计）；空 = 系统发布
}

func (c *Core) Publish(topic string, payload []byte, opts PublishOptions) (PublishResult, error)
```

```go
res, err := c.Publish("sensors/temperature", []byte(`{"v":25.6}`), core.PublishOptions{
    QoS:         packet.QoS0,
    PublisherID: "admin",
})
fmt.Println(res.MatchedSubscribers, res.Enqueued) // 只代表已入队
```

- **异步**：返回 nil ≠ 已投递；`MatchedSubscribers` 是入队瞬间的快照。
- **不调用 `OnPublish`**（管理面发布是受信来源，发布侧 ACL 不生效）；订阅侧 `OnDeliver` 仍生效。
- `Publish(topic, nil, PublishOptions{Retain: true})` = 清除保留消息。

| 错误 | 场景 |
|---|---|
| `ErrInvalidTopic` | topic 空/超长/含通配符 |
| `ErrInvalidQoS` | QoS 非 0/1 |
| `ErrQuotaExceeded` | 全局队列或存储超限 |
| `ErrCoreNotRunning` | 未启动/已停止 |

## 踢下线

```go
func (c *Core) DisconnectClient(clientID string, reason packet.ReasonCode) error
func (c *Core) DisconnectClientAndWait(ctx context.Context, clientID string, reason packet.ReasonCode) error
```

```go
if err := c.DisconnectClientAndWait(ctx, "device-42", packet.ReasonBanned); err != nil {
    var nf *core.ClientNotFoundError
    if !errors.As(err, &nf) {
        return err
    }
}
```

- `DisconnectClient` 异步；需要"确认已断开"用 `DisconnectClientAndWait`（等待发送协程退出，`ctx` 到期返回 `ctx.Err()`）。
- 踢下线**不删除会话**，客户端可能自动重连；要阻止重连需配合 `AuthHandler` 封禁（见「常见配方」）。
- 观测：`client.disconnected` 事件（`Reason`/`Duration`）与 `OnDisconnect` 钩子。

## 事件流

```go
type EventType string // client.connected / client.disconnected / subscription.added
                      // subscription.removed / message.published / message.dropped
                      // message.rejected / session.expired / admin.action

type Event struct {
    Seq  uint64    // 本订阅者内连续递增；跳号数量 == 丢弃数量
    Time time.Time
    Type EventType

    ClientID string // 客户端 / 发布者
    Actor    string // 管理面操作者（= PublisherID）

    RemoteAddr     string // 连接/断开
    SessionPresent bool   // 连接：是否恢复旧会话

    Topic    string
    Bytes    int
    QoS      packet.QoS
    Reason   string        // 原因码名或拒绝/丢弃原因
    Detail   string        // "pending_messages"、"store_bytes"、"api"
    Retained bool          // 保留消息
    Duration time.Duration // 断开：连接时长
}

type EventOptions struct {
    Types       []EventType // 空 = 全部；未知类型返回 ErrInvalidEventType
    ClientID    string      // 空 = 全部
    TopicPrefix string      // 空 = 全部
    Buffer      int         // 默认 1024，上限 8192
}

func (c *Core) SubscribeEvents(ctx context.Context, opts EventOptions) (*EventStream, error)

func (s *EventStream) Events() <-chan Event // Close/ctx 结束时关闭
func (s *EventStream) Dropped() uint64      // 本订阅者累计丢弃数
func (s *EventStream) Close()               // 幂等
```

```go
// SSE 桥接
stream, err := c.SubscribeEvents(r.Context(), core.EventOptions{
    Types:  []core.EventType{core.EventClientConnected, core.EventClientDisconnected},
    Buffer: 512,
})
if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
}
defer stream.Close()

for ev := range stream.Events() {
    if !writeSSE(w, ev) { // 客户端断开
        return
    }
}
```

各类型使用的字段：

| 类型 | 字段 |
|---|---|
| `client.connected` | `ClientID`、`RemoteAddr`、`SessionPresent` |
| `client.disconnected` | `ClientID`、`RemoteAddr`、`Reason`、`Duration` |
| `subscription.*` | `ClientID`、`Topic`、`QoS`、`Detail`（`"api"` = 编程接口发起） |
| `message.published` | `ClientID`（发布者）、`Topic`、`Bytes`、`QoS`、`Retained` |
| `message.dropped` / `.rejected` | `ClientID`、`Topic`、`Bytes`、`QoS`、`Reason`、`Detail` |
| `session.expired` | `ClientID`、`Reason` |
| `admin.action` | `Actor`、`Topic`、`Bytes`、`QoS`、`Reason`（`"publish"`）、`Retained` |

- 顺序保证：同一订阅者严格按 `Seq` 递增；未匹配事件不占号。
- 缓冲满即丢弃并累加 `Dropped()`（同时计入 `StatsSnapshot.EventsDropped`），不阻塞 broker。
- 无历史回放；`message.published` 与发布量同阶，按需订阅（消息内容用 `OnPublish` 采集）。
- 发现跳号或 `Dropped()` 增长时，重拉 `ListClients`/`GetClientInfo` 对账。

## 认证身份

```go
type Identity struct {
    Subject    string            // 设备序列号 / 用户名
    Roles      []string
    Attributes map[string]string
}
```

```go
// 写入：认证器设置，仅内存（重启后需重连）
opts.WithAuthHandler(&core.AuthHandlerFunc{
    ConnectFunc: func(ctx *core.AuthConnectContext) error {
        ctx.Identity = &core.Identity{Subject: "device-42", Roles: []string{"sensor"}}
        return nil
    },
})

// 读取：详情与事件
info.Identity.Subject
```

- 写入点：`AuthConnectContext.Identity`、`AuthContext.Identity`。
- ACL 读取点：`SubscribeContext.Identity`、`PublishContext.Identity`、`DeliverContext.Identity`。

```go
// 认证器指定 CONNACK 原因码
type AuthError struct {
    Reason packet.ReasonCode // 如 packet.ReasonBanned
    Err    error             // 仅服务端日志；支持 errors.As
}

// 断开原因
// DisconnectContext.Reason: NormalDisconnect / KeepAliveTimeout / SessionTakenOver
//                           / ServerShuttingDown / 管理员踢人时传入的 reason
// DisconnectContext.Err:    异常断开的底层错误（可能为 nil）
```

## 常见配方

### 封禁 + 踢下线

```go
opts.WithAuthHandler(&core.AuthHandlerFunc{
    ConnectFunc: func(ctx *core.AuthConnectContext) error {
        if bans.blocked(ctx.ClientID) {
            return &core.AuthError{Reason: packet.ReasonBanned}
        }
        return verifyCredential(ctx)
    },
})

// 先落库（bans 由管理程序自己维护），再踢下线
bans.add(id, time.Now().Add(ttl), reason)
err := c.DisconnectClientAndWait(ctx, id, packet.ReasonBanned)
var nf *core.ClientNotFoundError
if err != nil && !errors.As(err, &nf) {
    return err
}
```

### 日志 tail

```go
type logEntry struct {
    TS     time.Time
    Level  zapcore.Level
    Msg    string
    Fields map[string]any
}

type ringCore struct {
    zapcore.LevelEnabler
    mu   sync.Mutex
    buf  []logEntry
    next int
}

func (c *ringCore) With([]zapcore.Field) zapcore.Core { return c }
func (c *ringCore) Sync() error                       { return nil }

func (c *ringCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
    if c.Enabled(e.Level) {
        return ce.AddCore(e, c)
    }
    return ce
}

func (c *ringCore) Write(e zapcore.Entry, fs []zapcore.Field) error {
    enc := zapcore.NewMapObjectEncoder()
    for _, f := range fs {
        f.AddTo(enc)
    }
    c.mu.Lock()
    c.buf[c.next] = logEntry{TS: e.Time, Level: e.Level, Msg: e.Message, Fields: enc.Fields}
    c.next = (c.next + 1) % len(c.buf)
    c.mu.Unlock()
    return nil
}

baseLogger := zap.NewProduction()
ring := &ringCore{LevelEnabler: zapcore.InfoLevel, buf: make([]logEntry, 4096)}
opts.WithLogger(zap.New(zapcore.NewTee(baseLogger.Core(), ring)))
```

### 消息采集 / 流量 / 历史

```go
// OnPublish 在热路径同步执行：只做有界入队，不阻塞、不写库
type capture struct {
    ch      chan packet.PublishPacket
    dropped atomic.Int64
}

cap := &capture{ch: make(chan packet.PublishPacket, 1024)}

opts.WithMessageHandler(&core.MessageHandlerFunc{
    PublishFunc: func(ctx *core.PublishContext) error {
        select {
        case cap.ch <- ctx.Packet.DeepCopy(): // 必须深拷贝后再异步使用
        default:
            cap.dropped.Add(1)
        }
        return nil
    },
})

// 后台消费：for pkt := range cap.ch { writeDB(pkt.Topic, pkt.Payload) }
```

被拒绝/丢弃的消息看 `message.rejected` / `message.dropped` 事件。

### 其它

```go
// 清除保留消息
_, err := c.Publish(topic, nil, core.PublishOptions{Retain: true})

// 判断某客户端是否会收到某主题
info, err := c.GetClientInfo(clientID)
for _, s := range info.Subscriptions {
    if packet.MatchTopic(s.Topic, topic) { /* 会收到 */ }
}
```
