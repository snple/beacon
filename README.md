# Beacon

Beacon 是一个轻量级消息总线协议，专为物联网和实时通信场景设计。

## 功能特性

- **发布/订阅模式**：支持主题订阅和消息发布
- **定向投递**：支持通过目标客户端 ID 将消息发送给指定客户端
- **PUBLISH 请求-响应元数据**：PUBLISH 可携带响应主题和关联数据，用于应用层实现请求-响应、异步回调和任务结果匹配

## QoS 语义

Beacon 的 QoS 机制强调“消息被处理”而非仅“协议层收到”：

- **QoS0**：尽力保证到达性（best-effort delivery）。消息会被持久化，在客户端离线或队列满时由重传机制补发，但**不需要 ACK**，不保证绝对不丢。
- **QoS1**：保证消息被处理（at-least-once processing）。接收方在**消息落入应用可见队列之后**才发送 ACK；
  未收到 ACK（处理中崩溃、断电、接收队列满被拒收）时，服务端会按 `RetransmitInterval` 重传，
  直到收到 ACK。需要“应用处理完才算确认”时，用 `WithAutoAck(false)` + `Message.Ack()` 手动确认（见下节）。

## QoS1 确认方式

客户端默认在收到 QoS1 消息**入队后自动发送 ACK**（`AutoAck=true`）。若需要严格保证“消息被应用处理”，可关闭自动确认，由应用处理完消息后显式调用 `Message.Ack()`：

```go
c, _ := client.NewWithOptions(
    client.NewClientOptions().
        WithCore("127.0.0.1:5208").
        WithClientID("c1").
        WithAutoAck(false), // 关闭自动确认，改为手动确认
)

msg, _ := c.PollMessage(ctx, timeout)
// ... 处理消息 ...
msg.Ack() // 处理完才确认；未确认的消息会被服务端重传
```

注意：手动确认模式下 QoS1 为 at-least-once——若应用在处理中崩溃/断电未确认，服务端会重传该消息并再次投递；应用如需不重复处理，请按 `msg.Packet.PacketID` 自行去重。

`PollMessage(ctx, timeout)` 的 `timeout=0` 表示一直等待；断线后仍可继续取走已缓冲的消息（队列为空时才返回 `ErrNotConnected`）。

## 可靠性边界

以下行为是 QoS 语义的一部分，部署时需要了解：

- **积压补货**：发送队列排空或收到 ACK 时会立即从持久化队列补货，`RetransmitInterval` 仅作为连接异常时的兜底重试间隔，
  不再决定正常投递的吞吐。
- **订阅端接收队列满**（`client.WithMessageQueueSize`）：QoS1 消息不会被确认，而是由服务端按 `RetransmitInterval`
  再次投递，形成端到端背压；QoS0 消息直接丢弃。
- **客户端离线时发布**：消息写入本地出站队列，`Publish` 返回 `nil` 表示"已入队"，连接建立后自动补发。
- **钩子拒绝**：`OnPublish` 钩子拒绝 QoS1 消息时，服务端/客户端会返回 `NotAuthorized`，发布方立即收到失败，不会无限重传。
- **保留消息标志**：因订阅而重放的保留消息始终带 `Retain=true`；`RetainAsPublished` 只控制实时转发是否保留发布时的 `Retain` 标志。
- **定向投递语义**：`TargetClientID` / `Core.PublishToClient` 与主题发布一致——目标客户端离线但会话保留时消息入队，重连后补发；
  `Core.Broadcast` 则是即时的、仅在线客户端的扇出，不为离线会话入队。
- **持久化默认关闭**：库默认使用内存存储（`StoreOptions.DataDir` 为空）。需要重启后仍保留消息，必须配置 `DataDir`，
  并建议同时设置 `SyncWrites=true`，否则进程/机器掉电时已确认的消息仍可能丢失。
- **写超时**：`core.WithWriteTimeout(d)` / `client.WithWriteTimeout(d)` 控制单次网络写超时（默认 10 秒，`0` 表示不设置）。
  写入超时或失败会关闭该连接，未确认的消息仍保留在持久化队列中，重连后补发；编码失败（如属性超长）不会断开连接。
  这保证了慢速/不读数据的对端不会永久占用发送协程，也不会拖住停机。

## 配额与背压

Beacon 的队列默认有上限，避免单个慢消费者或异常客户端把内存/磁盘写满：

| 配置 | 默认值 | 超限行为 |
|---|---|---|
| `WithMaxClientQueueSize(n, policy)` | 10000 条 | 每个客户端的待发送队列达到上限后，按策略丢弃新消息（`QueueOverflowDropNewest`，默认）或最旧消息（`QueueOverflowDropOldest`），计入 `MessagesDropped` |
| `WithMaxPendingMessages(n)` | 100000 条 | 全局待分发队列达到上限后，新的发布被拒绝；QoS1 发布端收到 `PUBACK(QuotaExceeded)` |
| `WithMaxPublishRate(rate, burst)` | 不限制 | 单客户端发布速率超限；QoS1 收到 `PUBACK(MessageRateTooHigh)`，QoS0 被丢弃 |
| `WithMaxStoreBytes(bytes)` | 不限制 | 存储占用超过水位后拒绝新发布（`QuotaExceeded`）。基于 badger 上报的磁盘占用，有数秒滞后 |
| `WithMaxHandshakeConnections(n)` | 512 | 认证前（CONNECT 尚未解析）的并发连接上限。超限直接 `CONNACK(ServerBusy)` 并关闭，避免未认证连接无上限占用 goroutine 与包缓冲 |
| `WithMaxOfflineSessions(n)` | 10000 个 | 保留中的离线会话超过上限时，驱逐**最早过期**的会话（会话记录、订阅与待投递队列一并清理） |
| `client.WithMaxQueueSize(n)` | 10000 条 | 客户端本地出站队列满时，`Publish` 返回 `ErrQueueFull` |

所有限制都可以用 `0` 关闭。拒绝/丢弃数量可通过 `core.GetStats()` 的 `MessagesRejected`（接收侧拒绝）
与 `MessagesDropped`（投递侧丢弃）观察；`Core.GetClientInfo(id).QueueSize` 可查看单个客户端的积压量。

注意：

- 客户端队列配额按**消息条数**统计。单条消息的大小由 `MaxPacketSize` 限制，生产环境建议同时设置 `MaxStoreBytes`
  作为磁盘水位的兜底。
- 发布速率限制按客户端会话统计：使用全新会话（`SessionTimeout=0`）重连会获得新的令牌桶。
  如需防止重连绕过，请配合 `WithConnectionRateLimit` 与认证使用。

## 会话管理

- `SessionTimeout`：会话保留时间（秒），0 表示断开即清理；服务端可用 `MaxSessionTimeout` 限制上限。
- `client.WithCleanStart(true)`：要求服务端丢弃已有会话（订阅 + 待投递消息）并新建会话；
  CONNACK 的 `SessionPresent` 指示本次连接是否恢复了旧会话。
- **ClientID 约束**：非空、不超过 256 字节、合法 UTF-8，且不能包含 `:` 或控制字符
  （`:` 是服务端持久化队列的 key 分隔符）。服务端用 `CONNACK(ReasonClientIDNotValid)` 拒绝非法 ID；
  留空则由服务端自动生成。
- 会话元数据（订阅、会话超时）与待投递消息都会持久化：core 重启后未过期的会话会被恢复，
  客户端重连即可拿到 `SessionPresent=true`，无需重新订阅并能继续收到积压消息。
  **认证数据不随会话持久化**，每次建立连接都必须重新认证。
- **订阅重放**：客户端记录每个订阅的完整选项（QoS/NoLocal/RetainAsPublished/RetainHandling）。
  连接建立后若 `SessionPresent=false`（CleanStart、会话过期或换了服务端），客户端会按原选项自动重放，
  手动 `Disconnect()` 后重新 `Connect()` 与自动重连行为一致；`SessionPresent=true` 时不会重复订阅。
- 停机期间过期的会话，以及没有会话记录的孤儿队列，会在启动时自动清理。
- 遗嘱消息不随会话持久化：它属于单次连接，重连时由新的 CONNECT 提供。
- **会话接管不发布遗嘱**：同一 ClientID 的新连接替换旧连接（旧连接收到 `DISCONNECT(SessionTakenOver)`）
  视为客户端主动行为，旧连接的遗嘱既不会发布也不会残留；新会话的遗嘱以新 CONNECT 为准。

## 安全（认证与 TLS）

**认证**：在 `AuthHandler` 中校验 CONNECT 携带的 `AuthData`（建议用恒定时间比较）。`beacon-core` 已内置
`--auth-file`（TOML，支持 SIGHUP 热重载）：

```toml
[[nodes]]
serial = "device-1"
secret = "s3cr3t"
```

```bash
beacon-core --addr :5208 --auth-file nodes.toml
```

客户端在 CONNECT 时提供认证数据：

```go
opts := client.NewClientOptions().
    WithCore("127.0.0.1:5208").
    WithClientID("device-1").
    WithAuth("token", []byte("s3cr3t")) // AuthMethod + AuthData
```

**TLS**：服务端用 `core.ServeTLS(addr, tlsConfig)`（或自行传入 `tls.Listener`）；客户端地址使用 `tls://` 前缀即可：

```go
dialer := &client.TCPDialer{
    Address:   "tls://broker.example.com:8883",
    TLSConfig: &tls.Config{RootCAs: pool}, // 可选
}
```

未显式设置 `ServerName` 且未关闭校验时，客户端会从地址推导 SNI 名称（IP 字面量则依赖证书 IP SAN），
不会因为缺少 `ServerName` 而被迫使用 `InsecureSkipVerify`。

完整的加固配置示例见 [`examples/secure_core`](./examples/secure_core/main.go)（认证 + TLS + 配额 + 运维端点）。

## 可观测性

`GetStats()` 返回 `StatsSnapshot`，包含计数器（消息收发/丢弃/拒绝、字节数、订阅数）与运行时水位
（`PendingQueueDepth` 全局待分发队列、`ClientQueueDepth` 所有客户端待发送队列之和、`PendingAckTotal`、
`OfflineSessions`、`StoreBytes`）。

需要 HTTP 端点时启动 `ServeMetrics`（`beacon-core` 对应 `--metrics-addr`）：

| 端点 | 说明 |
|---|---|
| `GET /healthz` | 存活探针：运行中返回 `200 ok`，已停止返回 `503` |
| `GET /metrics` | Prometheus 文本格式指标（`beacon_*`；`beacon_pending_queue_depth`、`beacon_pending_ack`、`beacon_store_bytes` 等用于容量告警） |
| `GET /stats` | JSON 格式统计快照，便于人工排查 |

```bash
beacon-core --addr :5208 --metrics-addr :9101
curl -s localhost:9101/metrics | grep pending
```

`/metrics` 与 `/stats` 共用 1 秒快照缓存，避免高频抓取时重复全量遍历客户端；
需要实时数值时直接调用 `core.GetStats()`。运维端点默认不鉴权，生产环境请绑定 loopback 或置于内网。

## 管理 API（进程内）

面向外部管理程序（另一个仓库）嵌入使用：broker 只暴露 Go 方法，HTTP/鉴权/前端由管理程序自行实现。

| 能力 | API |
|---|---|
| 服务器信息 | `Info()`（版本/uptime/监听器/上限/存储，版本由 `WithBuildInfo` 构造期注入）、`Listeners()`、`Ready()`、`GetStats()` |
| 客户端列表 | `ListClients()`：轻量概要（ClientID/在线/地址/时间/离线过期时间），**不分页** |
| 客户端详情 | `GetClientInfo(id)`：配置、运行状态、队列与在途水位、订阅、每客户端计数器 |
| 踢下线 | `DisconnectClient()` / `DisconnectClientAndWait(ctx, ...)`（唯一改变会话状态的写操作） |
| 发布 | `Publish(topic, payload, opts)` 返回 `PublishResult`（命中订阅者数；**仅表示已入队**） |
| 事件流 | `SubscribeEvents(ctx, EventOptions) (*EventStream, error)`：连接/断开/订阅/发布/丢弃/拒绝/会话过期/管理操作 |
| 认证身份 | `AuthConnectContext.Identity`（仅内存），透出到 `ClientInfo.Identity` 与事件 |
| 错误语义 | 认证器返回 `*AuthError{Reason}` 可指定 CONNACK 原因码（如 `ReasonBanned`）；`DisconnectContext` 含 `Reason`/`Err` |

**观测面只有"客户端列表 + 客户端详情"两个入口**，话题/订阅/保留消息不再提供枚举 API；
需要时用 `GetClientInfo(id).Subscriptions` 查看单个客户端的订阅。

**改变会话/配置状态的写操作只有"踢下线"一条**：`DisconnectClient` / `DisconnectClientAndWait`。
`Publish` 只产生消息流，不算状态变更。删除会话、删除保留消息、清空积压一律不由 broker 提供
（保留消息用协议语义的 `Publish(topic, nil, PublishOptions{Retain: true})` 清除）。

事件流是**尽力而为**的：订阅者消费过慢时丢弃新事件，`EventStream.Dropped()` 给出本订阅者的累计丢弃数，
`Event.Seq` 是**每订阅者连续序号**（未匹配事件不占号），跳号数量 == 丢弃数量。
需要精确状态时以 `ListClients` / `GetClientInfo` 的快照为准。

封禁、ACL、审计留存、日志 tail、消息历史等属于业务策略，由管理程序用 `AuthHandler` /
`OnSubscribe` / `OnPublish` / zap sink 自行组合，broker 不内建（设计取舍见
[`docs/admin-api-design.md`](./docs/admin-api-design.md)）。

## 使用方法

导入包：

```go
import "github.com/snple/beacon/client"
import "github.com/snple/beacon/core"
```

更多示例：

- [`examples/core_api`](./examples/core_api/main.go)：Core 管理 API（分页/会话/保留消息/事件流/发布）
- [`examples/flexible_listeners`](./examples/flexible_listeners/main.go)：自定义监听器（TCP/TLS/Unix Socket）
- [`examples/secure_core`](./examples/secure_core/main.go)：认证 + TLS + 配额 + metrics 端点
- [`docs/embedded-client-guide.md`](./docs/embedded-client-guide.md)：CONNECT 报文布局与嵌入式客户端实现要点
- [`docs/admin-api-design.md`](./docs/admin-api-design.md)：进程内管理 API 设计（供外部 Web 管理程序嵌入调用；broker 不提供 HTTP 管理面）

## 流中继（独立的 stream 协议）

`stream/` 是**另一套协议**：点对点字节流中继，与 Beacon 消息总线的线格式和语义互不兼容，也不共享 `packet` 包。

- `stream.Core`：监听并中继；`stream.Client.OpenStream`（主动方）/ `AcceptStream`（被动方）建立双向流；
- `stream.Stream` 实现 `net.Conn`，可直接作为普通字节流使用；
- 等待中的接收连接总数受 `stream.WithMaxWaitingConns`（默认 10000）限制；
- 控制包（CONNACK/PONG/STREAM_ACK）与数据转发都受 `stream.WithWriteTimeout`（默认 10s）约束：
  对端长期不读数据时写超时会结束该流，而不是让转发协程永久阻塞；
- `Core.Stop()` 会关闭等待连接与活跃流，不会因转发协程阻塞而挂起；认证被拒时 `AcceptStream` 立即返回 `ErrAuthRejected`。

## WebSocket 支持

Beacon 同时支持 TCP 与 WebSocket 两种传输方式，二者完全互通（消息、订阅、QoS 语义一致）。

**服务端**：为 core 增加 `--ws-addr` 参数即可同时监听 WebSocket（默认路径 `/ws`）：

```bash
beacon-core --addr :5208 --ws-addr :8080 --ws-path /ws
```

**客户端**：地址以 `ws://` 或 `wss://` 开头时自动走 WebSocket 传输：

```bash
beacon-client --core ws://127.0.0.1:8080/ws --client-id c1 --mode sub --topic "test/#"
beacon-client --core ws://127.0.0.1:8080/ws --client-id c2 --mode pub --topic test/hello --message "hi"
```

需要 `wss://` 时，给 `core.ServeWSListener` 传入 `tls.NewListener(...)`，或把明文 WS 端点放在反向代理之后
（`beacon-core --ws-addr` 本身只提供明文 HTTP）。WS 升级默认接受任意 `Origin`（协议在 CONNECT 中认证）；
浏览器侧的来源限制请在反向代理或网关层配置。

**Go 代码**：使用 `client.WSDialer` 连接：

```go
c, _ := client.NewClient(client.NewClientOptions().
    WithClientID("browser-client").
    WithLogger(logger))
dialer := &client.WSDialer{Address: "ws://127.0.0.1:8080/ws", DialTimeout: 10 * time.Second}
err := c.ConnectWithDialer(dialer)
```

浏览器/前端应用可直接用原生 WebSocket 或 gorilla/websocket 与 core 的 `/ws` 端点通信。

## 开发与验证

```bash
make fmt        # 检查格式（不修改文件）
make fmt-fix    # 格式化
make vet
make lint       # golangci-lint，配置见 .golangci.yml
make test
make test-race
make check      # fmt + vet + lint + test-race
```

`make lint` 需要本地安装 golangci-lint v2：

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

CI（`.github/workflows/ci.yml`）会执行同样的检查。

## License

MIT
