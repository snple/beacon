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
