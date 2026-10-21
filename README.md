# Beacon

Beacon 是一个轻量级消息总线协议，专为物联网和实时通信场景设计。

## 功能特性

- **发布/订阅模式**：支持主题订阅和消息发布
- **定向投递**：支持通过目标客户端 ID 将消息发送给指定客户端
- **PUBLISH 请求-响应元数据**：PUBLISH 可携带响应主题和关联数据，用于应用层实现请求-响应、异步回调和任务结果匹配

## QoS 语义

Beacon 的 QoS 机制强调“消息被处理”而非仅“协议层收到”：

- **QoS0**：尽力保证到达性（best-effort delivery）。消息会被持久化，在客户端离线或队列满时由重传机制补发，但**不需要 ACK**，不保证绝对不丢。
- **QoS1**：保证消息被处理（at-least-once processing）。接收方**处理完消息后才发送 ACK**；若接收方在处理过程中发生意外（断电、崩溃）未能发送 ACK，发送方会重传该消息，直到收到 ACK。

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

## 可靠性边界

以下行为是 QoS 语义的一部分，部署时需要了解：

- **积压补货**：发送队列排空或收到 ACK 时会立即从持久化队列补货，`RetransmitInterval` 仅作为连接异常时的兜底重试间隔，
  不再决定正常投递的吞吐。
- **订阅端接收队列满**（`WithMessageQueueSize`）：QoS1 消息不会被确认，而是由服务端按 `RetransmitInterval` 再次投递，
  形成端到端背压；QoS0 消息直接丢弃。
- **客户端离线时发布**：消息写入本地出站队列，`Publish` 返回 `nil` 表示"已入队"，连接建立后自动补发。
- **钩子拒绝**：`OnPublish` 钩子拒绝 QoS1 消息时，服务端/客户端会返回 `NotAuthorized`，发布方立即收到失败，不会无限重传。
- **保留消息标志**：因订阅而重放的保留消息始终带 `Retain=true`；`RetainAsPublished` 只控制实时转发是否保留发布时的 `Retain` 标志。
- **持久化默认关闭**：库默认使用内存存储（`StoreOptions.DataDir` 为空）。需要重启后仍保留消息，必须配置 `DataDir`，
  并建议同时设置 `SyncWrites=true`，否则进程/机器掉电时已确认的消息仍可能丢失。

## 配额与背压

Beacon 的队列默认有上限，避免单个慢消费者或异常客户端把内存/磁盘写满：

| 配置 | 默认值 | 超限行为 |
|---|---|---|
| `WithMaxClientQueueSize(n, policy)` | 10000 条 | 每个客户端的待发送队列达到上限后，按策略丢弃新消息（`QueueOverflowDropNewest`，默认）或最旧消息（`QueueOverflowDropOldest`），计入 `MessagesDropped` |
| `WithMaxPendingMessages(n)` | 100000 条 | 全局待分发队列达到上限后，新的发布被拒绝；QoS1 发布端收到 `PUBACK(QuotaExceeded)` |
| `WithMaxPublishRate(rate, burst)` | 不限制 | 单客户端发布速率超限；QoS1 收到 `PUBACK(MessageRateTooHigh)`，QoS0 被丢弃 |
| `WithMaxStoreBytes(bytes)` | 不限制 | 存储占用超过水位后拒绝新发布（`QuotaExceeded`）。基于 badger 上报的磁盘占用，有数秒滞后 |
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
- 会话元数据（订阅、超时、认证信息）与待投递消息都会持久化：core 重启后未过期的会话会被恢复，
  客户端重连即可拿到 `SessionPresent=true`，无需重新订阅并能继续收到积压消息。
- 停机期间过期的会话，以及没有会话记录的孤儿队列，会在启动时自动清理。
- 遗嘱消息不随会话持久化：它属于单次连接，重连时由新的 CONNECT 提供。

## 使用方法

导入包：

```go
import "github.com/snple/beacon/client"
import "github.com/snple/beacon/core"
```

更多使用示例请参考 `examples/` 目录。

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

**Go 代码**：使用 `client.WSDialer` 连接：

```go
c, _ := client.NewClient(client.NewClientOptions().
    WithClientID("browser-client").
    WithLogger(logger))
dialer := &client.WSDialer{Address: "ws://127.0.0.1:8080/ws", DialTimeout: 10 * time.Second}
err := c.ConnectWithDialer(dialer)
```

浏览器/前端应用可直接用原生 WebSocket 或 gorilla/websocket 与 core 的 `/ws` 端点通信。

## License

MIT
