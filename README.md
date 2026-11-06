# Beacon

Beacon 是一个**面向 IoT 的拉取式消息服务**：生产者 `PUSH` 到 channel，broker 匹配后写入
每个客户端独立的队列；消费者按自己的节奏 `PULL` 并 `COMMIT`。

**服务端从不主动推送消息**：除了请求响应和连接终止通知，broker 不向客户端发送任何数据包。

- 协议规格：[`docs/protocol-v4.md`](./docs/protocol-v4.md)
- stream 数据面协议：[`docs/stream-protocol.md`](./docs/stream-protocol.md)
- 嵌入式客户端实现要点：[`docs/embedded-client-guide.md`](./docs/embedded-client-guide.md)
- 进程内管理 API：[`docs/admin-api-design.md`](./docs/admin-api-design.md)

## 为什么是 pull

- **消费节奏由客户端决定**：按自身处理能力拉取，慢消费者只影响自己的队列。
- **队列有界**：`pending` 受 `QueueCap` 约束，`delivered` 受 `MaxRecords` 约束
  （`delivered` 非空时 PULL 只重发未确认消息，不取新数据，因此长度天然不超过一次
  拉取量）。
- **确认即前缀压缩**：`COMMIT` 直接推进 `delivered` 的 FIFO 头，无需独立 ack 表与重传定时器。
- **低延迟**：空队列时用长轮询（`WaitMs`）等待，有数据立即返回。

## 核心模型

```
PUSH ──匹配 channel──▶ 各客户端 pending[3]（Low/Medium/High，受 QueueCap 约束）
                                    │
PULL ──────────────────────────────▶│  取出 → 写出
                                    │     ├─ Low      ：写出即删
                                    │     └─ Medium/High：移入 delivered
COMMIT(ids) ───────────────────────▶│  delivered 中按 ID 删除
```

- **优先级即可靠性**：`High`/`Medium` 必须 `COMMIT`；`Low` 尽力而为，写出即删。
- **`delivered` 优先**：存在未确认消息时，`PULL` 只重发这些消息（`Dup=1`），不取新数据，
  因此客户端必须先 `COMMIT` 再拉取。
- **定向投递**：`Record.Target` 指定 clientID，完全跳过 channel 匹配。
- **会话**：`SessionID` 是会话租约；同 ClientID 的新连接会接管会话并关闭旧连接，
  旧连接的请求从此不产生任何状态变更（fencing）。

## 快速开始

### 服务端

```bash
go run ./cmd/core --addr :5208 --metrics-addr :9101
```

常用参数：

| 参数 | 说明 |
|---|---|
| `--addr` | TCP 监听地址 |
| `--ws-addr` / `--ws-path` | WebSocket 监听地址与路径 |
| `--metrics-addr` | `/healthz`、`/metrics`、`/stats` 监听地址 |
| `--auth-file` | 设备密钥 TOML（`[[nodes]] serial=... secret=...`） |
| `--queue-cap` | 单客户端 pending 队列上限 |
| `--max-records` / `--max-bytes` | 单次 PULL 的记录数/字节上限 |
| `--max-batch` | 单次 PUSH 的记录数上限（默认 16，上限 128） |
| `--max-sessions` | 会话总数上限（含离线保留，`-1` = 不限） |
| `--session-timeout` | 离线会话保留秒数（`0` = 断开即清理） |
| `--max-inflight-bytes` | 全局在途消息字节上限（`-1` = 不限） |
| `--max-channel-length` | channel 名/过滤器最大字节数（256–1024，默认 256） |
| `--stream-addr` | stream 数据面 TCP 监听地址（空 = 不启动） |
| `--stream-enabled` | 是否开启 stream 能力（默认关闭） |
| `--stream-advertise` | 告知客户端的 stream 地址（默认使用监听器地址） |

### 客户端（CLI）

```bash
# 订阅并消费（逐批拉取 + 整批确认）
go run ./cmd/client -core 127.0.0.1:5208 -client-id c1 -mode pull -channel 'sensors/**'

# 丢弃该 client-id 的既有会话（pending/delivered/声明）后重新连接
go run ./cmd/client -core 127.0.0.1:5208 -client-id c1 -mode pull -channel 'sensors/**' -clean-session

# 发布
go run ./cmd/client -core 127.0.0.1:5208 -client-id c2 -mode push -channel sensors/temp -message 21.5

# 高优先级 + 定向投递
go run ./cmd/client -mode push -channel cmd/reboot -priority high -target c1 -message now
```

### Go 客户端

```go
import (
    "context"

    "github.com/snple/beacon/client"
)

c, err := client.NewWithOptions(client.NewClientOptions().
    WithCore("127.0.0.1:5208").
    WithClientID("worker-1").
    WithChannels("jobs/**"))     // 建连时随 CONNECT 一起声明
if err != nil {
    return err
}
if err := c.ConnectContext(ctx); err != nil {
    return err
}
defer c.Close()

// 消费循环：拉取 → 处理 → 整批确认
for {
    msgs, err := c.Pull(ctx, client.PullOptions{MaxRecords: 100, WaitMs: 30000})
    if err != nil {
        return err
    }
    for _, m := range msgs {
        handle(m.Channel, m.Payload)
    }
    if _, err := c.CommitAll(ctx); err != nil {
        return err
    }
}
```

发布：

```go
opts := client.PushOptions{}.WithPriority(packet.PriorityHigh)

if err := pub.Push(ctx, "jobs/image", payload, opts); err != nil { ... }

// 不等待确认（省一个 RTT，但发送失败不可知）
if err := pub.Push(ctx, "jobs/image", payload, opts.WithoutAck()); err != nil { ... }

// 定向投递（跳过 channel 匹配，直接进入目标客户端队列）
if err := pub.PushTo(ctx, "worker-1", "cmd/stop", nil, client.PushOptions{}); err != nil { ... }
```

## 语义要点

### 空队列：默认等待，不是忙等

`PULL.WaitMs` 是唯一决定「等不等」的字段：`0` 立即返回，`>0` 最多等待该时长。
SDK 层的默认值不同——为了让朴素循环不会空转：

| 调用方式 | 行为 |
|---|---|
| `Pull(ctx, client.PullOptions{})` | 长轮询，最多等 `ClientOptions.DefaultWaitMs`（默认 30s） |
| `Pull(ctx, client.PullOptions{WaitMs: 500})` | 最多等 500ms |
| `Pull(ctx, client.PullOptions{Immediate: true})` | 立即返回，不等待 |
| `PollMessage(ctx, timeout)` | 内部按批长轮询，攒够或者超时 |

同一连接同一时刻只允许一个在途 `PULL`，第二个会收到 `PULL_OK(RateLimited)`；
SDK 已在 `Pull` 内部串行化。

### 必须「先 commit 再 pull」

`PULL` 在存在未确认消息时只重发这些消息。因此**客户端不 commit 就再 pull，只会反复收到同样的内容**。
服务端会统计连续无进展的拉取次数：

- 超过 `NoProgressSoft`（默认 4）：返回 `PULL_OK(ReasonCommitRequired)`，不返回数据；
- 超过 `NoProgressHard`（默认 16）：断开连接（`DISCONNECT(CommitRequired)`）。

`client.CommitAll()` / `Message.Ack()` 负责提交。

### 响应只回显失败项

为了不让响应超过对端声明的 `MaxPacketSize`：

- `PUSH_OK.Results` 只包含失败或发生丢弃的记录，全部成功时为空；
- `DECLARE_RESP.Results` 只包含被拒绝的声明，`Accepted` 给出接受数量。

### 消息 ID

`PacketID` 12 字节，前 6 字节是毫秒时间戳，因此**大致**按时间有序（时钟回拨除外）。
它用于去重与追踪；**投递顺序以队列位置为准，不要依赖 ID 排序**。

### 优先级与丢弃

队列满时按「低优先级优先丢弃、同优先级丢最旧」处理；如果新消息的优先级低于当前最低非空优先级，
则丢弃新消息本身。被丢弃的数量通过 `PUSH_OK.Results[].Dropped` 返回给发布者。

会话离线时队列**只保留 Medium / High**：Low 消息在入队前直接丢弃，不占用离线队列；
离线队列满后仍按上述优先级策略淘汰。

### 定向投递

- 目标是 clientID；`Channel` 仍参与写权限校验，但不参与匹配；
- **发布者需要定向投递权限**：未配置认证器时默认允许，配置认证器后要求身份带
  `AllowTargeted`（`core.Identity`），否则返回 `NotAuthorized`；
- **目标必须声明过匹配该 `Channel` 的过滤器**，否则返回 `NoMatch`——定向投递
  不能绕过目标的订阅与读权限；
- 目标离线但会话保留时仍然入队（Low 除外）；
- 目标没有会话时返回 `TargetNotFound`。

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

### 遗嘱

`CONNECT` 可携带遗嘱（channel + payload + headers），仅在**异常断开**时发布；
`DISCONNECT` 或正常关闭会清除它。会话被接管时遗嘱**不触发**（设备仍在线，只是换了连接）。

### 不落盘

所有队列与会话都在内存中，进程重启即丢失。需要跨重启可靠性时请在上层保障。

## 可观测性

| 端点 | 说明 |
|---|---|
| `GET /healthz` | 存活探针 |
| `GET /metrics` | Prometheus 文本格式 |
| `GET /stats` | JSON 统计快照 |

关键指标：`beacon_inflight_messages`、`beacon_inflight_bytes`、`beacon_messages_dropped_total`、
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

## 点对点流（可选能力）

stream 用于在两个已认证客户端之间打通一条 TCP 通道。控制面复用主连接完成协商，
数据面走 core 的独立端口；建立后只做字节转发，不进入消息队列。

能力默认关闭，双方都需要显式开启。能力声明复用 `CONNECT`/`CONNACK` 的 spare bit，
没有额外的 HELLO 往返；旧客户端/旧服务端忽略该位，线格式保持兼容。

```go
// 服务端
opts := core.NewCoreOptions().
    WithStreamEnabled(true)          // 默认 false，core 构造时固定
opts.WithStreamAdvertiseAddr("beacon.example.com:5209")
go b.ServeStreamTCP(":5209")          // 或 ServeStreamTLS(addr, tlsConfig)

// 客户端
opts := client.NewClientOptions().
    WithCore("beacon.example.com:5208").
    WithClientID("device-a").
    WithStream(true)                  // 在 CONNECT 中声明支持 stream
```

接收方：

```go
b.OnStreamRequest(func(req *client.StreamRequest) {
    s, err := req.Accept(ctx, nil, nil)   // s 实现 net.Conn / io.ReadWriteCloser
    if err != nil {
        return
    }
    defer s.Close()
    io.Copy(s, s)
})

// 或者阻塞式
s, err := b.AcceptStream(ctx)
```

发起方：

```go
s, err := a.OpenStream(ctx, "device-b", "chat/1", []byte("meta"), map[string]string{"k": "v"})
```

数据面票据一次性、默认 30s 有效；任一方主连接断开或被接管，关联的流会立即关闭。
服务端只统计活跃流数与双向字节数，见 `GetStats().Streams*` / `Core.Streams()`。
协议细节：[`docs/stream-protocol.md`](./docs/stream-protocol.md)。

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
| 无 pipelining | `delivered` 非空时不取新数据（stop-and-wait） |
| Low 可能未被投递就被淘汰 | 尽力而为的定义 |
| High 溢出时仍会丢可靠消息 | 单队列容量上限的必然结果 |
| 无法表达「低紧急但必须送达」 | 只能用 Medium |
| 重启丢全部队列 | 纯内存 |
| `DECLARE` 不能减轻积压 | 只影响未来匹配 |

## License

MIT License，Copyright (c) 2026 danc <dangcheng@hotmail.com>。
