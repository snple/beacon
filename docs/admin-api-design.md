# Beacon 管理面 API 设计（进程内）

> **状态**：已实现（M1 + M2 + M3）。实现与设计稿的差异见 §0。
> **目标读者**：要实现 Web 管理程序的开发者（另一个仓库，运行时与 broker 同进程）。
> **范围**：只讨论 **broker 需要补的进程内 Go API**。HTTP 路由、鉴权、RBAC、前端、审计存储全部由 Web 项目实现。

---

## 0. 实现状态（与设计稿的差异）

| 项 | 状态 |
|---|---|
| M1：`Publish`/`PublishResult`、`ClientInfo`/`StatsSnapshot` 扩展、`ListSessions`/`DeleteSession`、分页 + 页内一致快照 | ✅ 已实现 |
| M2：`SubscribeEvents`（含 `Seq`/`EventsDropped`）、保留/订阅/主题枚举、`Info`/`Ready`/`SetBuildInfo`/`Listeners`、`Identity`、`AuthError`、`DisconnectContext.Reason/Err`、`DisconnectClientAndWait`、`OldestQueuedAge` | ✅ 已实现 |
| M3：`PurgeClientQueue`、`ListPendingAck` | ✅ 已实现 |
| `Event.MessageDelivered` | ❌ 按决定**不实现**（量级过大，需要时再加开关） |
| `Page.Total` | ❌ 暂不计算（字段保留为 `nil`） |
| `Drain()`、运行时改配置、日志环形缓冲、消息历史、每主题计数 | ❌ 保持"暂缓"（§7） |

实现时的补充：

- `PublishOptions` 增加 `PublisherID`；`Publish` 会用它填充 `SourceClientID`（影响 `NoLocal` 与审计）。
- `RetainedInfo` 增加 `QoS`：保留消息索引现在额外记录 `payloadSize`/`updatedAt`/`qos`，列元数据不再读取负载。
- `ListClientsPage`/`ListSessions` 整页持 `clientsMu.RLock`，保证页内字段一致。
- 在线计数改为 `Client.counted` 的 CAS 扣减，`DeleteSession` 与正常断开不会再双重扣减。
- `PublishToClient` 与 `Broadcast` 补上主题校验（**行为变更**，见 §8）。

---

## 1. 定位与设计原则

### 1.1 部署形态

**单进程、双仓库**：

```
web 管理程序（另一个仓库）            本项目（github.com/snple/beacon）
  go.mod 依赖 ────────────────────►  core / client / packet
  main(): core.NewWithOptions(...)    ↓
          broker.Start()              管理 API 以 Go 方法形式暴露
          http.ListenAndServe(...)    （无 HTTP、无 admin 鉴权、无管理中间件）
```

Web 项目通过 `import "github.com/snple/beacon/core"` 嵌入 broker，自己实现 HTTP 层。
**本项目不新增任何网络管理面。**

### 1.2 保持项目干净的五条原则

| 原则 | 含义 |
|---|---|
| **不加传输层** | 不提供 `ServeAdmin`、不引入 HTTP/gRPC 管理端点、不加 Token/TLS/CORS/`/readyz` |
| **不加策略** | 封禁、ACL、RBAC、审计留存、告警规则都属于业务策略，留在 Web 项目侧，由已有钩子实现 |
| **不加依赖** | 不引入任何新的第三方依赖；管理相关能力只用标准库 + 现有 `packet`/`zap` |
| **不自持管理状态** | broker 不持久化封禁名单、审计日志、事件历史；这些是 Web 项目的数据 |
| **增量且只补缺** | 只补"无法组合"或"组合代价明显过高"的能力（见 §3 判定标准）；其余一律不加 |

### 1.3 非目标

- 不做跨进程管理 API（明确不需要）。
- 不做集群/多节点纳管。
- 不内建消息历史、日志缓冲、运行时改配置。
- 不把管理面塞进 Beacon 协议（不引入 `$SYS` 之类保留主题）。

---

## 2. 现状盘点（进程内）

| 能力 | 现有 API | 缺口 |
|---|---|---|
| 聚合统计 | `GetStats() StatsSnapshot` | 缺 `StartedAt`、`Limits`，前端画不出使用率 |
| 客户端列表 | `ListClients()` | 全量返回、无排序/过滤/分页 |
| 客户端详情 | `GetClientInfo(id)` | 字段不足（见 §4） |
| 客户端订阅 | `GetClientSubscriptions(id)` | 可用 |
| 主题→订阅者 | `GetTopicSubscribers(topic)` | 可用（需已知主题，无法反向枚举） |
| 保留消息读取 | `GetRetainedMessages(pattern)` | 会**全量读取 payload**，无法只列元数据 |
| 在线/总数 | `GetOnlineClientsCount()` / `GetTotalClientsCount()` | 可用 |
| 踢下线 | `DisconnectClient(id, reason)` | 可用（封禁需 Web 侧配合，见 §3.2） |
| 定向发布 | `PublishToClient(...)` | 可投递，但不校验 topic |
| 广播 | `Broadcast(...)` | **绕过订阅树**，不校验 topic |
| 主题发布（按订阅投递） | **无** | 内部 `publish()` 未导出 |
| 离线会话 | 只有 count | 列表/详情/删除全无 |
| 编程式订阅 | `Subscribe()` / `Unsubscribe()` | 可用（已同步会话并持久化） |
| 事件观测 | `Hooks`（每种**只能注册一个**，同步执行） | 无多消费者事件流 |
| 认证身份 | 无输出通道 | 无法展示"是谁"、无法按身份审计 |
| 日志 | `GetLogger() *zap.Logger` | 可用（需在 `NewWithOptions` 前挂 sink） |

---

## 3. 判定标准：什么该进 broker，什么不该

### 3.1 三问

1. Web 项目能否用现有 API + 钩子自己组合出来？**能 → 不加。**
2. 是否只需读取 broker 内部状态？**→ 加只读 accessor（零策略、零副作用）。**
3. 是否必须改 broker 内部状态，且没有副作用等价的公开路径？**→ 加最小写 API。**

### 3.2 明确**不**由本项目提供的（附 Web 项目实现方式）

| 需求 | 为什么不做 | Web 项目怎么做 |
|---|---|---|
| 封禁 / 黑名单 | 属策略；且单进程单宿主下 `AuthHandler` 归 Web 项目所有，组合无障碍 | 在 `AuthHandler.OnConnect` 里查自己的黑名单，命中返回 error；踢人用 `DisconnectClient`。名单由 Web 项目持久化 |
| 发布/订阅 ACL | 同上 | `OnPublish` / `OnSubscribe` 钩子（配合 §5.6 的 `Identity`）；规则引擎属业务，不应内置 |
| admin HTTP / Token / TLS / CORS / RBAC | Web 项目职责 | 自己的 Web 框架 |
| `/readyz`、`/metrics` 暴露 | HTTP 是 Web 项目的事 | `if !broker.Ready() { 503 }`、`broker.GetStats()` 自己出端点 |
| 审计留存 | broker 只产生事件，不存 | 消费 `SubscribeEvents` 写自己的库 |
| 日志采集/tail | 属宿主基础设施 | 构造 `zap.Logger` 时挂自定义 `zapcore`（环形缓冲），再传给 `WithLogger` |
| 消息历史 / 流量检索 | 不是 broker 职责 | `OnPublish`/`OnDeliver` 写库，或消费事件流 |
| 运行时改配置（配额/限速/日志级别） | 破坏"配置在构造时确定"的简单性 | 改配置文件 + 重启进程（由 supervisor 负责） |
| drain / 单个 listener 停止 | 进程生命周期由 supervisor 管 | 直接停进程；会话已落盘，重启即恢复 |
| 集群纳管 | 单实例 broker | 不在本期范围 |

> 一句话：**broker 只提供"内部状态的窗口"和"必须由它执行的内部变更"，其余全部留在 Web 项目。**

---

### 3.3 可组合但繁琐：逐项评估

**前提修正**：本方案是**单进程、单宿主**——`AuthHandler` 与四类钩子都归 Web 项目所有，不存在"多个宿主抢钩子"的问题。因此"能否组合"几乎总是 YES；真正要判断的是**组合代价是否高到值得让 broker 提供一个更简单的入口**。

#### 3.3.1 评分维度

| 维度 | 0 分 | 1 分 | 2 分 |
|---|---|---|---|
| **D** 组合难度 | 一行调用 | 十几行循环/拼装 | 需要额外状态机、并发或时序处理 |
| **R** 出错风险 | 写错立刻可见 | 容易漏边界 | 容易写出竞态或语义误解（如把"已入队"当"已投递"） |
| **F** 使用频率 | 偶发运维动作 | 常见操作 | 每个页面/每次交互都用 |
| **I** 触达内部状态 | 公开状态即可 | 部分内部 | 纯内部，必须 broker 开放 |
| **C** broker 侧成本（反向） | 几乎零成本 | 几十行 + 单测 | 需要新数据结构/新通道/新索引 |
| **P** 策略泄漏（反向） | 无策略 | 轻微倾向 | 引入业务策略与状态 |

**判定规则**：

- `D+R+F+I ≥ 5` 且 `C ≤ 1` 且 `P = 0` → **提供 API**；
- 满足分数但 `C = 2` 或 `P > 0` → **提供文档片段**（可复制即用），不进 broker；
- 总分 < 5 → **不提供**，写进文档即可。

#### 3.3.2 评估结果

| # | 候选项 | D | R | F | I | C | P | 合计 | 结论 |
|---|---|---|---|---|---|---|---|---|---|
| 1 | `Publish` 返回命中/入队信息 | 1 | 2 | 2 | 1 | 0 | 0 | 6 | ✅ **提供**（P0，见 §5.2） |
| 2 | 分页结果的一致性快照 | 2 | 2 | 2 | 2 | 0 | 0 | 8 | ✅ **提供**（P0，内部实现，不改签名） |
| 3 | "踢下线并等待真正断开" | 2 | 1 | 1 | 2 | 1 | 0 | 6 | ✅ **提供** `DisconnectClientAndWait`（P1） |
| 4 | 事件带消息 payload（流量页预览） | 1 | 1 | 1 | 1 | 2 | 0 | 4 | 📄 **文档**：用 `OnPublish` 钩子采集；事件只带 `Bytes` |
| 5 | 事件缺口可检测（序号） | 2 | 1 | 1 | 2 | 0 | 0 | 6 | ✅ **提供** `Event.Seq`（P1） |
| 6 | 批量删除会话 `DeleteSessions(filter)` | 1 | 0 | 0 | 1 | 1 | 0 | 2 | ❌ 不提供：`ListSessions` + 循环即可 |
| 7 | 按主题前缀批量删除保留消息 | 1 | 0 | 0 | 1 | 1 | 0 | 2 | ❌ 不提供：`ListRetainedPage` + 循环 |
| 8 | 每主题消息计数 | 1 | 1 | 1 | 2 | 2 | 0 | 5 | ❌ 不提供：基数爆炸；文档给事件聚合方案 |
| 9 | 日志 tail 缓冲 | 2 | 2 | 1 | 0 | 2 | 0 | 5 | 📄 **文档**：给 `zapcore` 组合片段（约 30 行） |
| 10 | 封禁 / ACL | 1 | 1 | 1 | 0 | 1 | 2 | 5 | 📄 **文档**：`AuthHandler` + `OnSubscribe`/`OnPublish` 组合 |
| 11 | 判断"某客户端是否会收到某 topic" | 0 | 0 | 1 | 0 | 0 | 0 | 1 | ❌ 不提供：`GetClientSubscriptions` + `packet.MatchTopic` |
| 12 | 主题树/层级展示 | 0 | 0 | 1 | 0 | 0 | 0 | 1 | ❌ 不提供：`ListTopics` + 按 `/` 切分 |
| 13 | 保留消息重放给指定客户端 | 1 | 1 | 0 | 0 | 1 | 0 | 2 | ❌ 不提供：`GetRetainedMessages` + `PublishToClient` 循环 |
| 14 | 立即触发过期清理 `CleanupNow()` | 2 | 0 | 0 | 2 | 1 | 0 | 4 | ❌ 不提供：清理周期可用 `WithExpiredCheckInterval` 调到 10s |
| 15 | 统计时间序列 / 速率曲线 | 1 | 1 | 2 | 0 | 1 | 0 | 4 | ❌ 不提供：Web 侧采样自家数据库是本职 |
| 16 | 会话"重置"（清空订阅+积压，保留会话） | 1 | 0 | 0 | 1 | 1 | 0 | 2 | ❌ 不提供：`Unsubscribe` 全量 + `PurgeClientQueue`（P2）组合 |
| 17 | 状态变更的可靠通知（不漏事件） | 2 | 2 | 2 | 2 | 2 | 0 | 8 | ⚠️ **部分提供**：`Event.Seq` + `EventsDropped` 供对账，**不保证不丢**；UI 以列表 API 为准 |
| 18 | 让客户端知道"是被封禁"而不是"凭证错" | 2 | 1 | 1 | 2 | 0 | 0 | 6 | ✅ **提供** `AuthError{Reason}`（P1） |
| 19 | 断开原因（keepalive 超时/被踢/接管/关机） | 2 | 0 | 2 | 2 | 0 | 0 | 6 | ✅ **提供** `DisconnectContext.Reason`（P1） |
| 20 | 待发送队列"最老消息多久了" | 2 | 0 | 1 | 2 | 0 | 0 | 5 | ✅ **提供** `ClientInfo.OldestQueuedAge`（P1） |
| 21 | 哪条 QoS1 消息卡住了（packetID/topic/发了多久） | 2 | 1 | 1 | 2 | 1 | 0 | 6 | ✅ **提供** `ListPendingAck`（P2） |

#### 3.3.3 几条值得强调的取舍

1. **#2 分页一致性**看起来只是实现细节，但它是纯内部状态问题：`ListClients` 现在对每个客户端单独加锁，跨条目可能出现"`Connected=true` 但连接已被替换"的撕裂视图。在 `ListClientsPage` 内部改为**整页持 `clientsMu.RLock`** 即可解决，成本为零，所以必须做。

2. **#4 事件塞 payload 明确拒绝**。事件通道一旦承载数据，就会有人拿它当消息总线用，背压、内存上限、大包拷贝全部变成 broker 的责任。消息内容本来就该由 `OnPublish` 钩子在 Web 项目侧采集（钩子归它所有，且可自行异步化）。

3. **#8 每主题计数不提供**。主题基数不可控（客户端可随意发布），broker 内建计数器等于给了个内存泄漏入口。若确实需要，Web 项目消费 `message.published` 事件做聚合，并自行实现 TopN + 淘汰。

4. **#18/#19 是"组合不可能"，不是"组合麻烦"**。认证失败当前一律回 `ReasonNotAuthorized`，钩子无法表达"被封禁"；断开时 `DisconnectContext` 只有一个可空的 `Packet`，异常断开的真实原因（keepalive 超时/被踢/被接管/关机）藏在内部 `conn.closeReason` 里。这两处各补一个字段/类型即可，却是排障台的核心信息。

5. **#20/#21 是把"内部状态"变成"可展示状态"**。队列与 `pendingAck` 都是私有结构，Web 侧无法窥探；补上后"这个客户端为什么不动了"才可回答。`OldestQueuedAge` 复用现成的 `Queue.Peek()`，几乎没有成本。

## 4. 数据模型缺口

> 交付节奏：标 **P0** 的字段随 M1，其余（`Identity`、`OldestQueuedAge`、`OldestPendingAckAge`）随 M2；字段本身都是同一批改动，分两批只是为了让 M1 尽早可用。

### 4.1 `ClientInfo`

现有字段：`ClientID`、`RemoteAddr`、`Connected`、`ConnectedAt`、`KeepAlive`、`SessionTimeout`、`Subscriptions`、`PendingAck`、`ReceiveWindow`、`QueueSize`。

| 需补字段 | 现状问题 | 管理台用途 |
|---|---|---|
| `ExpiresAt time.Time` | 离线会话过期时间只在内部 `offlineSessions` | 会话页"剩余有效期" |
| `LastSeenAt time.Time` | 无。`ConnectedAt`/`KeepAlive`/`RemoteAddr` 都取自 `conn`，**客户端一离线全部归零** | 离线客户端"上次在线时间" |
| `Identity *Identity` | 认证器无身份输出通道 | 显示"是谁"、按身份过滤/审计 |
| `Transport string` | 无 | 区分 TCP/WS/TLS |
| `ProtocolVersion string` | 无 | 老客户端识别 |
| `WillTopic string` | 遗嘱不对外 | 排查"掉线会发什么" |
| `MessagesSent/Dropped/Rejected int64` | 只有全局计数 | 定位单个客户端丢消息 |
| `BytesSent/Received int64` | 只有全局 | 单客户端流量 |
| `QueueBytes int64` | 只有条数 | 一条消息可能 1MB |
| `OldestPendingAckAge time.Duration` | 只有数量 | 判断 QoS1 是否卡死 |
| `OldestQueuedAge time.Duration` | 只有条数 | 判断积压是否在动（复用 `Queue.Peek()`） |

### 4.2 `StatsSnapshot`

```go
type StatsSnapshot struct {
    // ... 现有字段保持不变 ...
    StartedAt time.Time // 进程启动时间，用于 uptime
    Limits    Limits    // 上限，前端据此画"使用率"
}

type Limits struct {
    MaxClients              uint32
    MaxHandshakeConnections uint32
    MaxPacketSize           uint32
    ReceiveWindow           uint16
    MaxClientQueueSize      int
    MaxPendingMessages      int
    MaxStoreBytes           int64
    MaxOfflineSessions      int
    MaxPublishRate          int
    WriteTimeout            time.Duration
}
```

> 补齐这些字段后，`/metrics` 与 `/stats` 的内容可由 Web 项目自行渲染，broker 不需要改 HTTP 层。

---

## 5. API 定义

> 全部为**增量**接口，不修改现有签名；现有 HTTP 运维端点保持原样（供简单部署使用，不扩展）。

### 5.1 服务器信息与就绪

```go
type ListenerInfo struct {
    Address  string // 实际监听地址
    Protocol string // "tcp" | "tls" | "ws" | "custom"
    Active   bool
}

type StoreInfo struct {
    Path  string
    Ready bool
    Bytes int64
}

type ServerInfo struct {
    Version   string // 由宿主注入，默认为空
    Protocol  string // "MX/2"
    StartedAt time.Time
    Uptime    time.Duration
    Listeners []ListenerInfo // 需要 listener 注册表，见 §5.1.1
    Limits    Limits
    Store     StoreInfo
}

func (c *Core) Info() ServerInfo

// Ready 报告 broker 是否已 Start 且存储可用。
func (c *Core) Ready() bool

// SetBuildInfo 由宿主注入构建信息（版本号目前只存在于 cmd/core 的包级常量）。
func (c *Core) SetBuildInfo(version, commit, buildTime string)
```

#### 5.1.1 `Listeners()` 的实现前提

`ServeTCP`/`ServeTLS`/`ServeWS`/`ServeMetrics` 目前都是 `net.Listen` 后直接阻塞，没有句柄。要支持 `Info().Listeners`，需内部登记：

```go
type listenerEntry struct {
    addr, proto string
    closeFn     func() error // TCP: listener.Close；WS/Metrics: http.Server.Close
}
```

这是**纯内部重构**，不新增对外概念，也不引入 drain 语义（不做 `Drain()`，见 §7）。

### 5.2 主题发布（补齐文档承诺）

现有 [`Broadcast`](../core/deliver.go) 的注释写"需要持久化/按订阅关系投递时请使用主题发布（Publish）"，但该 API **从未导出**。这是文档 bug + 真实缺口。

```go
// PublishResult 描述本次发布的**入队结果**（不是投递结果）。
type PublishResult struct {
    MatchedSubscribers int  // 入队瞬间匹配到的订阅者数（含离线会话，估算值）
    Retained           bool // 是否已写入保留存储
    Enqueued           bool // 是否进入全局分发队列
}

// Publish 以 broker 身份向主题发布消息。
// 语义与客户端发布一致：校验 topic、进入全局分发队列、按订阅树投递、
// 处理 retain、为离线会话入队、受全局配额约束。
// opts.PublisherID 为空表示"系统发布"，用于 NoLocal 判断与审计。
func (c *Core) Publish(topic string, payload []byte, opts PublishOptions) (PublishResult, error)
```

**为什么一开始就返回结构体而不是 `error`**（评估 #1）：`Publish` 是新增 API，此刻改签名零成本；一旦发布成 `error`，将来想加"命中几个订阅者"就是破坏性变更。而 `MatchedSubscribers` 只在入队瞬间算一次（复用订阅树查询），成本可忽略。

**必须写进 godoc 的语义**：分发是**异步**的（先入全局队列，由 `dispatchLoop` 投递）。因此：

- `Publish` 返回 nil 表示"已入队"，**不表示已投递**；
- `MatchedSubscribers` 是入队瞬间的快照，与后续订阅变更存在竞态，只作 UI 提示；
- QoS1 在这里没有 PUBACK，不存在"等确认"的概念。

`PublishOptions` 追加一个字段：

```go
type PublishOptions struct {
    // ... 现有字段 ...
    PublisherID string // 以哪个身份发布（影响 NoLocal 与审计），默认空=系统
}
```

错误语义：

| 情况 | 返回 |
|---|---|
| topic 非法（空/超长/含通配符） | `ErrInvalidTopic`（新增） |
| 全局队列/存储配额超限 | `ErrQuotaExceeded`（新增） |
| `OnPublish` 钩子拒绝 | `ErrDeliveryRejected` |

**顺带修正**：`PublishToClient` / `Broadcast` 补 `packet.ValidateTopicName` 校验（当前缺失，可写入客户端永远订阅不到的非法主题）。

> 说明：`PublishToClient`/`Broadcast` **保持现有签名不变**（避免破坏），只有新增的 `Publish` 返回 `PublishResult`。若后续希望统一，可在下一个大版本再收敛。

### 5.3 分页与枚举

```go
type PageOptions struct {
    Limit  int    // 默认 100，上限 1000
    Cursor string // 上一页返回的 NextCursor
    Prefix string // 按 clientID / topic 前缀过滤
    Online *bool  // 仅客户端列表：true=在线，false=离线
}

type Page struct {
    NextCursor string // 空表示没有下一页
    Total      *int64 // nil=未计算（全量计数代价高，按需开启）
}

func (c *Core) ListClientsPage(o PageOptions) ([]*ClientInfo, Page, error)
func (c *Core) ListSessions(o PageOptions) ([]SessionInfo, Page, error)
func (c *Core) ListRetainedPage(o PageOptions) ([]RetainedInfo, Page, error)
func (c *Core) ListSubscriptions(o PageOptions) ([]SubscriptionEntry, Page, error)
func (c *Core) ListTopics(o PageOptions) ([]TopicEntry, Page, error)
```

**游标设计**：按主键字典序（clientID / topic），`Cursor` 表示"从该 key 之后继续"。

理由：`clients` 是 map，天然无序；序号游标在并发增删时会重复/漏项。字典序游标最坏只漏掉"新插入且排序在游标之前"的项，对管理台可接受。

**页内一致性**（评估 #2）：实现时**整页持有 `clientsMu.RLock`** 完成"取快照 + 拷贝字段"，而不是像现在 `ListClients` 那样对每个客户端单独加锁。否则同一页里可能出现 `Connected=true` 但连接已被替换的撕裂视图 —— 这类问题 Web 侧无法自行修正，成本却为零，必须由 broker 保证。

> 注意：页内一致 ≠ 跨页一致。翻页期间数据可能变化，这是游标分页的固有特性。

```go
type SessionInfo struct {
    ClientID      string
    Online        bool
    ExpiresAt     time.Time // 在线时为零值
    QueueSize     int64
    QueueBytes    int64
    PendingAck    int
    Subscriptions []ClientSubscriptionInfo
    Identity      *Identity
    LastSeenAt    time.Time
}

type RetainedInfo struct {
    Topic       string
    PayloadSize int
    QoS         packet.QoS
    ExpiresAt   int64     // 0=不过期
    UpdatedAt   time.Time
}

type SubscriptionEntry struct {
    ClientID string
    Topic    string
    packet.SubscribeOptions
}

type TopicEntry struct {
    Topic           string
    SubscriberCount int
    Retained        bool
}
```

> `ListRetainedPage` 只返回元数据；要看内容再调现有的 `GetRetainedMessages(topic)`（单条主题，代价可控）。

### 5.4 内部状态变更（最小集）

```go
// DeleteSession 删除会话记录 + 订阅 + 待投递队列（"忘记该客户端"）。
// 在线客户端会先被踢下线。不可由 Web 项目组合实现。
func (c *Core) DeleteSession(clientID string) error

// DeleteRetained 删除单条保留消息。
// 现有绕路（发布空 payload + Retain）会经 Broadcast 广播给所有在线客户端，副作用不可接受。
func (c *Core) DeleteRetained(topic string) error

// PurgeClientQueue 清空某客户端的待发送积压，返回清除条数。（P2）
func (c *Core) PurgeClientQueue(clientID string) (int64, error)

// DisconnectClientAndWait 踢下线并等待连接与发送协程真正退出。
// 现有 DisconnectClient 是异步的（内部只触发关闭），Web 项目若想确认
// "已经断开"必须自己轮询 IsClientOnline，既繁琐又有竞态。
func (c *Core) DisconnectClientAndWait(ctx context.Context, clientID string, reason packet.ReasonCode) error

// PendingAckInfo 一条在途（已发送未确认）的 QoS1 消息。
type PendingAckInfo struct {
    PacketID  nson.Id
    Topic     string
    QoS       packet.QoS
    SentAt    time.Time
    Age       time.Duration // now - SentAt，超过重传间隔说明在反复重发
    Retries   int           // 已重发次数
}

// ListPendingAck 列出某客户端在途未确认的消息（排障"哪条卡住了"）。（P2）
func (c *Core) ListPendingAck(clientID string) ([]PendingAckInfo, error)
```

### 5.5 事件流（把观测移出热路径）

**为什么需要它**（钩子本身是可用的，问题不在"能不能拿到事件"）：

1. **钩子在热路径同步执行**：在里面写库、推 SSE 会直接拖慢发布，Web 项目必须自己再搭一层异步队列 —— 而这件事每个使用方都要重写一遍。
2. **钩子覆盖不全**：消息被丢弃、被拒绝、会话过期、管理操作（删会话/删保留/发布）都没有对应钩子。
3. **需要扇出**：实时页、审计、告警要各自消费同一批事件，钩子做不到多消费者。

> 事件流不是"抢钩子"的替代品，而是**把观测从热路径剥离出来**，并补齐钩子覆盖不到的事件。

```go
type EventType string

const (
    EventClientConnected    EventType = "client.connected"
    EventClientDisconnected EventType = "client.disconnected"
    EventSubscribed         EventType = "subscription.added"
    EventUnsubscribed       EventType = "subscription.removed"
    EventMessagePublished   EventType = "message.published"
    // EventMessageDelivered 未实现：投递事件量级 = 发布量 × 订阅者数，需要时再加开关。
    EventMessageDropped     EventType = "message.dropped"
    EventMessageRejected    EventType = "message.rejected"
    EventSessionExpired     EventType = "session.expired"
    EventAdminAction        EventType = "admin.action"
)

type Event struct {
    Seq      uint64 // 单调递增；配合 Stats.EventsDropped 做缺口对账
    Time     time.Time
    Type     EventType
    ClientID string
    Topic    string
    Bytes    int
    QoS      packet.QoS
    Reason   string            // reason code 名 / 丢弃原因
    Actor    string            // 管理操作的操作者
    Extra    map[string]string // 预留扩展
}

type EventFilter struct {
    Types       []EventType
    ClientID    string
    TopicPrefix string
    Buffer      int // 每订阅者缓冲，默认 1024
}

// SubscribeEvents 返回只读事件通道。消费者过慢时按"丢弃并计数"处理，
// 绝不允许阻塞 broker 主路径。
func (c *Core) SubscribeEvents(ctx context.Context, f EventFilter) (<-chan Event, error)
```

实现要点：

- **单生产者扇出**：内部产生一次事件，写入各订阅者的有界环形缓冲；
- **绝不阻塞**：缓冲满则丢弃并累加 `Stats.EventsDropped`（新增指标）；
- **`admin.action`**：`DeleteSession`/`DeleteRetained`/`PurgeClientQueue`/`Publish(PublisherID!= "")` 等写操作都发一条，带 `Actor`（由 Web 项目通过 `opts` 或上下文提供）；
- 事件**不落盘**；需要留存由 Web 项目写自己的库。
- **事件不携带消息 payload**（评估 #4）：一旦让事件通道承载数据，背压/内存上限/大包拷贝都会变成 broker 的责任。需要消息内容请在 `OnPublish` 钩子里采集（钩子归 Web 项目所有，可自行异步化、自行采样）。
- **不保证可靠投递**：靠 `Seq` 跳号 + `Stats.EventsDropped` 判断是否丢过；UI 需以 `List*` 列表 API 为准做对账（评估 #17）。

> 兼容性：`Hooks` 保留不动。实现上让内部产生事件 + 继续调用用户钩子，二者并存。

### 5.6 认证身份

```go
type Identity struct {
    Subject    string            // 设备序列号 / 用户名
    Roles      []string          // 角色，供 ACL 与前端展示
    Attributes map[string]string // 租户、型号等
}

// AuthConnectContext / AuthContext 追加：
//   Identity *Identity   // 认证器成功时写入
```

约束：

- **仅内存**：与既有"认证数据不随会话持久化"的决定一致（见 [README「会话管理」](../README.md)）。进程重启后需重连才有身份。
- `ClientInfo.Identity`、`SessionInfo.Identity`、事件中的身份都由它填充。
- ACL 不在 broker 内实现；broker 只负责透出身份，鉴权继续由 `OnSubscribe` / `OnPublish` 决定。
- 建议同时把 `Identity` 放进 `SubscribeContext` / `PublishContext`，否则按身份的 ACL 无法落地（见 §9）。

### 5.7 钩子可表达性（两处小补充）

这两项不是新能力，而是**让已有钩子能表达它本来就知道的事实**（评估 #18/#19）。

```go
// AuthError 让认证器指定 CONNACK 原因码。
// 当前所有认证失败一律回 ReasonNotAuthorized，钩子无法区分
// "凭证错误"（客户端应停止重试）与"被封禁""服务器忙"等语义。
type AuthError struct {
    Reason packet.ReasonCode // 如 ReasonBanned(0x8A) / ReasonBadAuthMethodOrAuthData / ReasonServerBusy
    Err    error             // 可选：仅用于服务端日志，不下发
}

func (e *AuthError) Error() string
func (e *AuthError) Unwrap() error

// DisconnectContext 追加两个字段：
//   Reason packet.ReasonCode // NormalDisconnect / KeepAliveTimeout / SessionTakenOver
//                             // / ServerShuttingDown / 管理员踢人时传入的 reason
//   Err    error             // 异常断开的底层错误（读错误等），可能为 nil
```

实现要点：

- `authClient` 用 `errors.As` 取 `*AuthError`，取不到则沿用 `ReasonNotAuthorized`（完全向后兼容）；
- 断开原因直接读现成的 `conn.closeReason`（`keepAliveTimeout`、`closeLocal(ServerShuttingDown)`、`detach(SessionTakenOver)`、管理员 `Close(reason)` 都已写入），无需新增状态；
- Web 项目据此可显示"该客户端最后一次是因为心跳超时断开的"，而不是只能看到"断开了"。

---

## 6. Web 项目如何用这些 API 组装页面

| 页面/功能 | 使用的 API | 说明 |
|---|---|---|
| 总览大盘 | `Info()` + `GetStats()` + `Ready()` | 计数器 + limits 画使用率 |
| 客户端列表 | `ListClientsPage()` | 分页/过滤/搜索在服务端完成 |
| 客户端详情 | `GetClientInfo()` + `GetClientSubscriptions()` | 字段已补全（§4.1） |
| 会话页 | `ListSessions()` / `DeleteSession()` | 含离线会话与过期时间 |
| 订阅页 | `ListSubscriptions()` / `ListTopics()` / `GetTopicSubscribers()` | 支持全局浏览与反向查询 |
| 保留消息页 | `ListRetainedPage()` + `GetRetainedMessages()` + `DeleteRetained()` | 列表不载 payload，详情按需取 |
| 测试发布 | `Publish()` / `PublishToClient()` | 返回 `PublishResult`，可提示"命中 N 个订阅者"；注意只代表已入队 |
| 踢下线 | `DisconnectClient()` / `DisconnectClientAndWait()` | 需要"确认已断开"时用后者 |
| 封禁 | `DisconnectClient()` + `AuthHandler` 自查黑名单 | 见 §3.2 |
| 断开原因展示 | `DisconnectContext.Reason` / `Event.Reason` | 区分心跳超时/被踢/接管/关机 |
| 在途消息排障 | `ListPendingAck()`（P2） | 定位"哪条 QoS1 卡住" |
| 实时事件页 | `SubscribeEvents()` | SSE/WebSocket 由 Web 项目实现 |
| 审计 | `SubscribeEvents()` 过滤 `admin.action` | Web 项目落库 |
| 日志 tail | 自定义 `zapcore` 环形缓冲 → `WithLogger` | 已有能力，无需新 API |
| 消息历史 | `OnPublish`/`OnDeliver` 或事件流 → Web 项目自己的库 | 见 §3.2 |

---

## 7. 明确暂缓（除非后续有明确需求）

| 项 | 暂缓理由 |
|---|---|
| `Drain()` / 单 listener 停止 | 进程生命周期交给 supervisor；`Stop()` 已能把会话全部落盘 |
| 运行时改配置 | 配置构造期确定，重启即可；避免引入可变全局状态 |
| 日志环形缓冲 | Web 项目挂 zap sink 即可，broker 不应持有日志缓冲 |
| 消息历史存储 | 数据量级与保留策略属业务决策，放 Web 项目 |
| 待发送队列**内容**明细 | 可能上万条且无界，导出意义不大；先用 `QueueSize`/`QueueBytes`/`OldestQueuedAge` 定位。注意与 `ListPendingAck` 的区别：在途未确认消息受 `ReceiveWindow` 约束（有界，默认 100 条），可以安全枚举 |

> `PurgeClientQueue` 虽已给出签名（§5.4），但归入 P2/M3，不必随 M1 交付。

---

## 8. 兼容性与影响面

| 变更 | 兼容性 |
|---|---|
| 新增 Go API | 完全增量，无影响 |
| `ClientInfo` / `StatsSnapshot` / `PublishOptions` 追加字段 | 源码兼容；使用**无 key 结构体字面量**的外部代码会编译失败，需在 CHANGELOG 注明 |
| `Broadcast` / `PublishToClient` 增加 topic 校验 | **行为变更**：非法 topic 从"静默接受"变为返回错误。属修 bug，需标注 |
| `AuthError` | 增量；未返回 `*AuthError` 时仍回 `ReasonNotAuthorized`，行为不变 |
| `DisconnectContext` 追加 `Reason`/`Err` | 结构体由 broker 填充、用户只读，安全 |
| 事件总线接入钩子路径 | 需保证不阻塞；`Hooks` 对外行为不变 |
| listener 注册表 | 纯内部重构，无 API 变化 |

**依赖**：不新增任何第三方依赖。

---

## 9. 待确认

1. **`Total` 是否返回**：全量计数需遍历 map。建议默认不返回（前端用"加载更多"）。
2. **`Identity` 是否进入 `SubscribeContext` / `PublishContext`**：不进则按身份的 ACL 无法实现。建议进。
3. **`Actor` 如何传入**：建议写操作统一带一个 `AdminContext{Actor}` 参数，或 `Core.SetActorResolver(func() string)`；前者更显式。
4. **封禁是否要 broker 提供**：当前评估结论是**不提供**（`AuthHandler` 组合，见 §3.3 #10）。若 Web 项目希望"重启后封禁仍生效且无需自己存库"，需要额外讨论——那意味着 broker 要持久化管理状态，与本方案的干净原则冲突。
5. **`ListRetainedPage` 的 `UpdatedAt`**：现有存储只保存消息体。推荐用 `Message.PacketID.Time()` 推导（nson.Id 前 48 位是毫秒时间戳，库已提供 `Timestamp()`/`Time()`），零存储成本；若要求"更新时刻"而非"发布时刻"，才需额外索引字段。
6. ~~**`Event.MessageDelivered` 是否要实现**~~：**已决定不实现**。
7. ~~**`DisconnectClientAndWait` 的超时语义**~~：**已决定带 `ctx` 由调用方控制**；实现为"等待发送协程退出"，`ctx` 到期返回 `ctx.Err()`。

---

## 10. 里程碑

### M1 — 核心页面可落地（P0）

- `Core.Publish` → `(PublishResult, error)` + topic 校验（含修正 `PublishToClient`/`Broadcast` 与 `Broadcast` 的错误注释）
- `ClientInfo` 补 `ExpiresAt`、`LastSeenAt`、`QueueBytes`、`OldestPendingAckAge`、每客户端计数器
- `StatsSnapshot` 补 `StartedAt`、`Limits`
- `ListSessions` / `DeleteSession`
- `PageOptions`/`Page` + `ListClientsPage`
- 分页**页内一致性**（整页持 `clientsMu.RLock`，见 §5.3）

**验收**：Web 项目仅依赖公开 API 即可完成「客户端列表 / 客户端详情 / 会话列表 / 测试发布」四个页面。

### M2 — 可观测与治理（P1）

- `SubscribeEvents`（含 `admin.action`、`Event.Seq`）+ `Stats.EventsDropped`
- `ListRetainedPage` / `DeleteRetained` / `ListSubscriptions` / `ListTopics`
- `Info()` / `Ready()` / `SetBuildInfo()` + listener 注册表
- `Identity` 透出（`ClientInfo` / `SessionInfo` / 事件）
- `DisconnectClientAndWait`
- `AuthError{Reason}` + `DisconnectContext.Reason`（§5.7）
- `ClientInfo.OldestQueuedAge`

**验收**：实时事件页、保留消息页、订阅浏览页、服务器信息页可落地；踢人后可确定性地刷新页面状态。

### M3 — 补充（P2）

- `PurgeClientQueue`
- `ListPendingAck`
- 视实际需要再评估 §7 中暂缓项

---

## 11. 测试计划

| 项 | 方法 |
|---|---|
| `Publish` 与客户端发布语义一致 | 同一 topic/payload 分别经客户端与 `Core.Publish` 发出，断言订阅者收到的 QoS/Retain/属性一致 |
| 分页正确性 | 并发增删客户端时用游标翻页，断言无重复项；覆盖空结果、整页边界、cursor 越界 |
| `LastSeenAt` / `ExpiresAt` | 在线→离线→重连→过期的时间线断言 |
| 事件流不阻塞 | 慢消费者（不读 channel）+ 高频发布，断言发布延迟不退化、`EventsDropped` 增长 |
| 事件扇出 | 多订阅者 + 过滤器（类型/客户端/前缀）分别断言 |
| `DeleteSession` | 在线/离线两种状态；断言订阅树、队列、会话记录、离线表全部清理 |
| `DeleteRetained` | 断言不再匹配、不影响其他主题、不触发广播 |
| topic 校验 | 空/超长/含 `*`、`**`、控制字符全被拒绝 |
| `PublishResult.MatchedSubscribers` | 0/1/多订阅者、含离线会话、含 `NoLocal` 自订阅，分别断言计数 |
| 分页页内一致性 | 并发替换连接（同 ClientID 重连）时翻页，断言同一页内不出现 `Connected=true` 却无连接地址的撕裂项 |
| `DisconnectClientAndWait` | 正常断开、ctx 超时、客户端不存在三种路径；断言返回后 `IsClientOnline` 恒为 false |
| `Event.Seq` | 慢消费者导致丢弃时，序号跳号与 `EventsDropped` 增量一致 |
| `AuthError` 原因码 | 返回 `ReasonBanned` 时客户端收到对应 CONNACK；返回普通 error 时仍为 `NotAuthorized`（向后兼容） |
| `DisconnectContext.Reason` | 正常断开/心跳超时/被踢/被接管/服务端停机五条路径逐一断言 |
| `OldestQueuedAge` / `ListPendingAck` | 空队列为 0；积压后年龄单调增长；重传后 `Retries`/`SentAt` 更新 |
| 竞态 | 新增 API 全部纳入 `go test -race`，列表/事件做并发压测 |

---

## 12. 与现有文档的关系

- [README.md](../README.md)：新增 API 落地后，在「会话管理」一节点出 `Admin API` 小节，并说明管理台由外部项目通过库调用实现。
- [docs/embedded-client-guide.md](./embedded-client-guide.md)：与嵌入式客户端无关，无需改动。
- `examples/core_api`：M1 完成后更新，展示 `Publish`、分页与会话管理。

---

## 13. 附录：组合片段（不进 broker）

以下需求评估结论是"不提供 API"，给可复制的实现片段，Web 项目直接拿去用。

### 13.1 封禁 / 黑名单（评估 #10 + #18）

```go
type banEntry struct {
    Until  time.Time
    Reason string
}

type banList struct {
    mu sync.RWMutex
    m  map[string]banEntry
}

func (b *banList) blocked(id string) bool {
    b.mu.RLock()
    e, ok := b.m[id]
    b.mu.RUnlock()
    if !ok {
        return false
    }
    return e.Until.IsZero() || time.Now().Before(e.Until)
}

// 注册认证钩子：先查黑名单，再走真实认证
opts.WithAuthHandler(&core.AuthHandlerFunc{
    ConnectFunc: func(ctx *core.AuthConnectContext) error {
        if bans.blocked(ctx.ClientID) {
            // 让客户端明确知道"被封禁"，而不是"凭证错误"
            return &core.AuthError{Reason: packet.ReasonBanned}
        }
        return verifyCredential(ctx) // Web 项目自己的认证
    },
})

// 封禁动作：落库 → 立即踢下线
func ban(ctx context.Context, broker *core.Core, bans *banList, id, reason string, ttl time.Duration) error {
    bans.add(id, time.Now().Add(ttl), reason)          // Web 项目自己持久化
    if err := broker.DisconnectClientAndWait(ctx, id, packet.ReasonBanned); err != nil {
        var nf *core.ClientNotFoundError
        if !errors.As(err, &nf) {
            return err // 客户端不存在可以忽略
        }
    }
    return nil
}
```

> 评估 #10 的结论：broker 不内建封禁，因为那要求 broker 持久化管理状态。上面前 3 行就是"策略"，属于 Web 项目。

### 13.2 日志 tail（评估 #9）

```go
type logEntry struct {
    TS     time.Time
    Level  zapcore.Level
    Msg    string
    Fields map[string]any
}

// ringCore 把日志写入定长环形缓冲，MB 级内存即可保留最近 N 条。
type ringCore struct {
    zapcore.LevelEnabler
    mu   sync.Mutex
    buf  []logEntry
    next int
    n    int
}

func (c *ringCore) With([]zapcore.Field) zapcore.Core { return c }

func (c *ringCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
    if c.Enabled(e.Level) {
        return ce.AddCore(e, c)
    }
    return ce
}

func (c *ringCore) Write(e zapcore.Entry, fs []zapcore.Field) error {
    enc := zapcore.NewMapObjectEncoder() // 只用于取字段，本身是值拷贝
    for _, f := range fs {
        f.AddTo(enc)
    }
    c.mu.Lock()
    defer c.mu.Unlock()
    c.buf[c.next] = logEntry{e.Time, e.Level, e.Message, enc.Fields}
    c.next = (c.next + 1) % len(c.buf)
    if c.n < len(c.buf) {
        c.n++
    }
    return nil
}

func (c *ringCore) Sync() error { return nil }

// 组合：既写标准输出，又进环形缓冲
ring := &ringCore{LevelEnabler: zapcore.InfoLevel, buf: make([]logEntry, 4096)}
logger := zap.New(zapcore.NewTee(baseLogger.Core(), ring))
opts.WithLogger(logger)
```

`ring` 再提供一个 `Snapshot() []logEntry` 给 `GET /api/v1/logs` 用即可。

### 13.3 判断"某客户端是否会收到某主题"（评估 #11）

```go
subs, err := broker.GetClientSubscriptions(clientID)
if err != nil {
    return err
}
for _, s := range subs {
    if packet.MatchTopic(s.Topic, topic) { // 与 broker 内部匹配语义完全一致
        return true
    }
}
```

### 13.4 主题树 / 层级展示（评估 #12）

`ListTopics()` 返回扁平列表，按 `/` 切分自行建树即可（前端的树组件基本都吃这个结构）。

### 13.5 消息采集 / 流量分析 / 历史（评估 #4、#8、#15）

```go
// OnPublish 在热路径同步执行：只做有界入队，绝不阻塞、绝不写库。
type capture struct {
    ch      chan *packet.PublishPacket
    dropped atomic.Int64
}

func (c *capture) OnPublish(p *packet.PublishPacket) {
    // 必须深拷贝：钩子拿到的是内部引用的包，异步使用前要隔离
    select {
    case c.ch <- p.DeepCopy():
    default:
        c.dropped.Add(1) // 后台消费不过来就丢，由采样/告警兜底
    }
}

opts.WithMessageHandler(&core.MessageHandlerFunc{
    PublishFunc: func(ctx *core.PublishContext) error {
        cap.OnPublish(ctx.Packet)
        return nil
    },
})
```

后台从 `c.ch` 取消息写库（Web 项目的数据库），即可支撑消息历史、流量图、TopN 主题统计。
**注意**：`OnPublish` 只覆盖"被接受的发布"；被拒绝/丢弃的消息请看 `message.rejected` / `message.dropped` 事件。

