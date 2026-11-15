# Beacon

Beacon 是一个**面向 IoT 的拉取式消息服务**：生产者 `PUSH` 到 channel，broker 匹配后写入
每个客户端独立的队列；消费者按自己的节奏 `PULL` 并 `COMMIT`。

**服务端从不主动推送消息**：除了请求响应和连接终止通知，broker 不向客户端发送任何数据包。

- 协议规格：[`docs/protocol-v5.md`](./docs/protocol-v5.md)
- 嵌入式客户端实现要点：[`docs/embedded-client-guide.md`](./docs/embedded-client-guide.md)
- 进程内管理 API：[`docs/admin-api-design.md`](./docs/admin-api-design.md)

## 为什么是 pull

- **消费节奏由客户端决定**：按自身处理能力拉取，慢消费者只影响自己的队列。
- **队列有界**：`pending` 受 `QueueCap` 约束；`PULL` 批量返回记录，
  未确认可靠消息受每会话窗口 `ReceiveMaximum`（默认 32）约束。
- **确认即释放窗口**：`COMMIT` 批量移除已确认记录，无需独立 ack 表与重传定时器。
- **低延迟**：空队列时用长轮询（`WaitMs`）等待，有数据立即返回。

## 核心模型

```
PUSH ──匹配 channel──▶ 各客户端 pending[3][2]（优先级 × 可靠性，受 QueueCap 约束）
                                    │
PULL ──────────────────────────────▶│  窗口有空位时批量取出 → 写出
                                    │     ├─ 不可靠：写出即释放
                                    │     └─ 可靠  ：移入 delivered
COMMIT/NACK(ids) ──────────────────▶│  delivered 中按 ID 批量移除
```

- **优先级与可靠性独立**：`Reliable` 决定是否需要 `COMMIT`、离线是否保留；
  `Priority` 只决定投递与淘汰顺序。`Reliable` **默认关闭**，需要投递保证时必须
  显式 `WithReliable(true)`；`Low + Reliable` 表示「不急但必须送达」。
- **窗口内流水线**：最多 `ReceiveMaximum` 条可靠消息同时未确认；窗口有空位就继续取新消息，
  窗口满时 `PULL` 以 `Dup=1` 重发未确认消息。
- **定向投递**：`Record.Target` 指定 clientID，完全跳过 channel 匹配；
  声明可标记为只接收定向投递（`TargetOnly`），此时广播不会命中它。
- **会话**：`SessionID` 是会话租约；同 ClientID 的新连接会接管会话并关闭旧连接，
  旧连接的请求从此不产生任何状态变更（fencing）。
- **消息 TTL**：可设置绝对过期时间；过期消息不入队，入队后过期在 `PULL` 或会话清理时丢弃。
- **自定义属性**：`CONNECT` / `CONNACK` 及消息请求/响应支持 `Properties`，可在客户端、
  认证钩子与服务端之间双向传递；除 `error` 外 broker 不解释。
- **握手下发限制**：`CONNACK.Limits` 返回包大小、队列、批量拉取、会话保留等上限，
  `DECLARE_RESP` 会再次返回同一组值；客户端可提前按服务端限制调整行为。

## 快速开始

### 服务端

```bash
go run ./cmd/core --addr :5208 --metrics-addr 127.0.0.1:9101
```

常用参数：

| 参数 | 说明 |
|---|---|
| `--addr` | TCP 监听地址 |
| `--ws-addr` / `--ws-path` | WebSocket 监听地址与路径 |
| `--metrics-addr` | `/healthz`、`/metrics`、`/stats` 监听地址 |
| `--auth-file` | 设备密钥 TOML（`[[nodes]] serial=... secret=... allow_targeted=true`） |
| `--queue-cap` | 单客户端 pending 队列上限 |
| `--max-pending-bytes` | 单客户端 pending 字节上限（`0` = 不限） |
| `--max-keepalive` | 服务端最终心跳间隔上限秒数（默认 300，`65535` 约等于不限） |
| `--max-sessions` | 会话总数上限（含离线保留，`-1` = 不限） |
| `--session-timeout` | 离线会话保留秒数（`0` = 断开即清理） |
| `--max-inflight-bytes` | 全局在途消息字节上限（`-1` = 不限） |
| `--max-channel-length` | channel 名/过滤器最大字节数（256–1024，默认 256） |
| `--receive-max` | 每会话未确认可靠消息窗口（1–1024，默认 32） |
| `--max-pull-batch` | 单次 `PULL` 最多返回记录数（1–1024，默认 32） |

### 客户端（CLI）

```bash
# 订阅并消费（逐条拉取 + 确认）
go run ./cmd/client -core 127.0.0.1:5208 -client-id c1 -mode pull -channel 'sensors/**'

# 丢弃该 client-id 的既有会话（pending/delivered/声明）后重新连接
go run ./cmd/client -core 127.0.0.1:5208 -client-id c1 -mode pull -channel 'sensors/**' -clean-session

# 发布
go run ./cmd/client -core 127.0.0.1:5208 -client-id c2 -mode push -channel sensors/temp -message 21.5

# 高优先级 + 可靠投递 + 定向投递（发布默认不可靠，需要确认时加 -reliable）
go run ./cmd/client -mode push -channel cmd/reboot -priority high -reliable -target c1 -message now

# 连接启用了 --auth-file 的服务端
go run ./cmd/client -core 127.0.0.1:5208 -client-id device-1 -auth-data s3cret \
    -mode pull -channel 'sensors/**'
```

### Go 客户端

```go
import (
    "context"
    "time"

    "github.com/snple/beacon/client"
)

c, err := client.NewWithOptions(client.NewClientOptions().
    WithServer("127.0.0.1:5208").
    WithClientID("worker-1").
    WithProperty("model", "x1").                  // 随 CONNECT 发给服务端
    WithChannels(client.Filter("jobs/**")))       // 建连后自动 DECLARE（广播 + 定向投递）
if err != nil {
    return err
}
if err := c.ConnectContext(ctx); err != nil {
    return err
}
defer c.Close()

// 服务端可通过 WithServerProperties / AuthContext.ResponseProperties 在 CONNACK 中返回信息
region := c.ServerProperties()["region"]
// 握手时下发的限制，客户端可直接按此裁剪请求
limits := c.ServerLimits()

// 消费循环：批量拉取 → 处理 → 一个 RTT 批量确认
for {
    res, err := c.PullBatch(ctx, client.PullOptions{WaitMs: 30000, MaxCount: 32})
    if err != nil {
        return err
    }
    for _, m := range res.Messages {
        handle(m.Channel, m.Payload)
    }
    if len(res.Messages) > 0 {
        if _, err := c.CommitAll(ctx); err != nil {
            return err
        }
    }
}
```

发布：

```go
opts := client.PushOptions{}.
    WithPriority(client.PriorityHigh).
    WithReliable(true) // 需要 COMMIT；默认不可靠

// Push 返回 PushResult（ID/Matched/Dropped/Reason/Properties）与 error。
res, err := pub.Push(ctx, "jobs/image", payload, opts)
if err != nil { ... }

// 低紧急但必须送达：Low + Reliable
lowReliable := client.PushOptions{}.
    WithPriority(client.PriorityLow).
    WithReliable(true)

// 10 秒后过期；同时记录发布者时间戳
ttlOpts := client.PushOptions{}.
    WithReliable(true).
    WithTTL(10 * time.Second)

// 不等待确认（省一个 RTT，但发送失败不可知）
if _, err := pub.Push(ctx, "jobs/image", payload, opts.WithoutAck()); err != nil { ... }

// 定向投递（跳过 channel 匹配，直接进入目标客户端队列）；
// 目标存在但未声明匹配的过滤器时返回 client.ErrTargetNotSubscribed，
// 目标没有会话时返回匹配 client.ErrTargetNotFound 的错误。
if _, err := pub.PushTo(ctx, "worker-1", "cmd/stop", nil, client.PushOptions{}); err != nil { ... }
```

`WithMaxRecvPacketSize` 只限制**接收**；发送上限默认采用服务端
`CONNACK.Limits.MaxPacketSize`，可用 `WithMaxSendPacketSize` 进一步收紧。
运行时 `Declare` 的声明会被记住，并在每次重连后自动重放。

## 语义要点

### 空队列：默认等待，不是忙等

`PULL.WaitMs` 是唯一决定「等不等」的字段：`0` 立即返回，`>0` 最多等待该时长。
SDK 层的默认值不同——为了让朴素循环不会空转：

| 调用方式 | 行为 |
|---|---|
| `Pull(ctx, client.PullOptions{})` | 长轮询，最多等 `ClientOptions.DefaultPullWaitMs`（默认 30s） |
| `Pull(ctx, client.PullOptions{WaitMs: 500})` | 最多等 500ms |
| `Pull(ctx, client.PullOptions{Immediate: true})` | 立即返回，不等待 |
| `PollMessage(ctx, timeout)` | 内部逐条长轮询，直到取到消息或超时（不会自动确认） |

同一连接同一时刻只允许一个在途 `PULL`，第二个会收到 `PULL_OK(RateLimited)`；
SDK 已在 `Pull` 内部串行化。`PullBatch` 返回 `PullResult`：`Messages` 是本批消息，
`Properties` 是服务端随 `PULL_OK` 返回的属性（游标、限流提示等）。

### 窗口与确认

可靠消息在 `COMMIT`/`NACK` 之前一直占用窗口（默认 `ReceiveMaximum=32`）。
窗口有空位时 `PULL` 继续投递新消息；窗口满后只重发未确认消息（`Dup=1`）。
因此**只拉不确认最终会停止前进**。服务端会统计连续无进展的拉取次数：

- 超过 `NoProgressSoft`（默认 4）：返回 `PULL_OK(ReasonCommitRequired)`，不返回数据；
- 超过 `NoProgressHard`（默认 16）：断开连接（`DISCONNECT(CommitRequired)`）。

`client.CommitAll()` 用一个 `COMMIT` 确认当前所有未确认消息，返回
`client.CommitResult`（`Committed` / `Nacked` / `Unknown` / `Pending`）；
`Message.Ack()` 确认单条；`Message.Nack(ctx, reason)` 拒绝单条
（服务端不再重发，并触发 `Hooks.Nack`）。

### 响应内容

- `PUSH_OK` 始终回显该条记录的原因、ID 与匹配/丢弃数量；
- `COMMIT_OK` 分别给出 `Committed` / `Nacked` / `Unknown` 条数；
- `DECLARE_RESP.Results` 只包含被拒绝的声明，`Accepted` 给出接受数量。

### 消息 ID

`PacketID` 12 字节，前 6 字节是毫秒时间戳，因此**大致**按时间有序（时钟回拨除外）。
它用于追踪与消费端去重（重发消息沿用同一 ID）；**投递顺序以队列位置为准，不要依赖 ID 排序**。
生产者重试会生成新的 `PacketID`，如果需要在发布侧消除重复，请使用业务键并由消费端幂等处理。

### 优先级与丢弃

队列满时先淘汰不可靠消息，再按 Low → Medium → High 淘汰旧消息，同队列内丢最旧；
如果新消息的价值低于当前最低非空队列，则丢弃新消息本身。被丢弃的数量通过
`PUSH_OK.Dropped` 返回给发布者。可靠性高于优先级：`Low + Reliable` 比
`High + Unreliable` 更晚被淘汰。

单条记录如果超过**在线**目标客户端在 `CONNECT` 中声明的接收上限（`MaxPacketSize`），
服务端不会为该目标入队（否则它会永久堵住该客户端队列），同样计入 `Dropped`；
定向投递在唯一在线目标放不下时返回 `PacketTooLarge`。离线会话仍先入队，
重连后若仍放不下，会在 `PULL` 时被丢弃（见「已知限制」）。

会话离线时队列**保留全部可靠消息**，不可靠消息在入队前直接丢弃；
离线队列满后仍按上述策略淘汰。

### 定向投递

- 目标是 clientID；`Channel` 仍参与写权限校验，但不参与匹配；
- **发布者需要定向投递权限**：未配置认证器时默认允许，配置认证器后要求身份带
  `AllowTargeted`（`core.Identity`），否则返回 `NotAuthorized`；
- **目标必须声明过匹配该 `Channel` 的过滤器**，否则返回 `NoMatch`——定向投递
  不能绕过目标的订阅与读权限；客户端 SDK 的 `PushTo` 会将其映射为
  `client.ErrTargetNotSubscribed`；
- 目标离线但会话保留时仍然入队（仅可靠消息）；
- 目标没有会话时返回 `TargetNotFound`；
- 在线目标的接收上限放不下这条记录时返回 `PacketTooLarge`，不会入队。

声明可以逐项指定是否接收广播（见下节）：`TargetOnly` 声明只接收定向投递，
广播发布不会命中它，但定向投递仍可命中。

### 声明模式：广播 vs 只定向投递

```go
// 默认：同时接收广播与定向投递
c.Declare(ctx,
    client.Filter("jobs/**"),                 // 广播 + 定向投递
    client.TargetOnly("worker-1/inbox"),      // 只接收定向投递
)

// 建连后自动声明时同样可以区分
client.NewClientOptions().
    WithChannels(client.Filter("jobs/**"), client.TargetOnly("worker-1/inbox"))
```

`TargetOnly` 是**逐项生效**的：同一客户端可以同时声明 `jobs/**`（只定向投递）
与 `jobs/public`（广播可达），广播只会命中后者。同一过滤器重复声明时取更宽松的
模式（只要有一次广播可达，就参与广播匹配）。

`Declare` 返回完整的 `DeclareResult`：`Rejected` 只包含被拒绝的项，
另有 `Accepted`、`Pending` 与 `DECLARE_RESP.Properties`。需要随 DECLARE 携带属性时
使用 `DeclareWithProperties`。

### 保留 channel

`$` 前缀保留给 broker 内部 channel：客户端不能发布或声明 `$` 开头的名字；
以通配符开头的过滤器（`*` / `**`）不会匹配 `$` 开头的 channel（与 MQTT 一致）。

### 会话接管与清理

同一个 ClientID 再次连接会接管会话：新连接拿到新的 `SessionID`，旧连接收到
`DISCONNECT(SessionTakenOver)` 后被关闭，且旧连接之后的所有请求都不产生状态变更。
客户端收到该原因码后**不应自动重连**，否则两个实例会互相踢下线。

`CONNECT` 的 `Flags` bit0（`CleanSession`）置位时会先丢弃该 ClientID 的 pending / delivered / channel 声明，
再以新会话建立连接（`SessionPresent=0`）。会话保留时长由服务端
`--session-timeout` 统一配置（`0` = 断开即清理）。

订阅声明**只由 `DECLARE` 承担**（`CONNECT` 不携带）。恢复会话时服务端保留既有声明，
但会按本次认证身份重新校验，越权的过滤器被丢弃；SDK 在有 `WithChannels` 时
每次建连后都会重新 `DECLARE`，因此重连也能恢复订阅。

### 不落盘

所有队列与会话都在内存中，进程重启即丢失。需要跨重启可靠性时请在上层保障。

## 可观测性

| 端点 | 说明 |
|---|---|
| `GET /healthz` | 存活探针 |
| `GET /metrics` | Prometheus 文本格式 |
| `GET /stats` | JSON 统计快照 |

关键指标：`beacon_inflight_messages`、`beacon_inflight_bytes`、`beacon_messages_dropped_total`、
`beacon_messages_nacked_total`、`beacon_messages_expired_total`、
`beacon_commit_required_total`、`beacon_session_takeovers_total`、`beacon_wildcard_filters`。

## 管理 API（进程内）

```go
b.Info()                      // 版本、上限、监听器
b.ListClients()               // 会话概要（在线 + 离线保留）
b.GetClientInfo(clientID)     // 详情：队列深度、声明、身份、计数器
b.DisconnectClient(id, reason)
b.Push(channel, payload, opts)                 // 系统身份推送
b.PushToClient(clientID, channel, payload, opts)
b.GetStats()                  // 计数快照
```

详见 [`docs/admin-api-design.md`](./docs/admin-api-design.md)。

## 传输

- TCP：`core.ServeTCP(addr)`
- TLS：`core.ServeTLS(addr, tlsConfig)`
- WebSocket：`core.ServeWS(addr, path)`；客户端用 `client.WSDialer` 或 `ws://` / `wss://` 地址

WebSocket 默认只接受同源（无 `Origin` 的非浏览器客户端不受影响）。需要跨域时显式配置
`core.NewCoreOptions().WithWSCheckOrigin(fn)`。

所有传输共用同一线格式，行为完全一致。

## 开发

```bash
make fmt        # 检查格式
make fmt-fix    # 格式化
make vet
make lint       # golangci-lint v2
make test
make test-race
make check      # fmt + vet + lint + test-race
```

## 已知限制

| 限制 | 说明 |
|---|---|
| 无历史回放 | channel 是路由键，不存历史；新声明的客户端只收到之后的消息 |
| 未确认窗口有限 | 超过 `ReceiveMaximum` 后只重发未确认消息，直到客户端确认 |
| 不可靠消息可能未被投递就被淘汰 | 尽力而为的定义 |
| 队列满时仍会丢可靠消息 | 单队列容量上限的必然结果 |
| 生产者重试可能产生重复 | 发送后未收到 `PUSH_OK` 就重试会生成新消息；需要业务键 + 消费端幂等 |
| 无遗嘱消息 | 服务端不代发；可用服务端 `Hooks.Disconnect` 或客户端断线事件自行发布 |
| TTL 使用发布者时钟 | 设备与服务端时钟偏差过大会导致提前/延后过期，需要 NTP 或服务端校正 |
| 重启丢全部队列 | 纯内存 |
| `DECLARE` 不能减轻积压 | 只影响未来匹配 |
| 重连时调小 `MaxPacketSize` 会丢弃放不下的已入队消息 | 否则一条超限消息会永久堵住队列 |

## License

MIT License，Copyright (c) 2026 danc <dangcheng@hotmail.com>。
