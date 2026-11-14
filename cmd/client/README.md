# beacon-client

Beacon v5 命令行客户端，用于调试 `PUSH` / `PULL` / `COMMIT`。

## 用法

```bash
# 订阅并批量消费（拉取一批 → 处理 → 一次确认）
go run ./cmd/client -core 127.0.0.1:5208 -client-id c1 -mode pull -channel 'sensors/**'

# 发布
go run ./cmd/client -core 127.0.0.1:5208 -client-id c2 -mode push -channel sensors/temp -message 21.5

# 高优先级 + 定向投递 + 带元数据
go run ./cmd/client -mode push -channel cmd/reboot -priority high -target c1 -message now

# 低紧急但必须送达
go run ./cmd/client -mode push -channel jobs/audit -priority low -reliable -message audit
```

## 参数

| 参数 | 默认 | 说明 |
|---|---|---|
| `-core` | `127.0.0.1:5208` | 服务端地址，支持 `ws://` / `wss://` |
| `-client-id` | 自动 | 客户端 ID |
| `-mode` | `pull` | `push` 或 `pull` |
| `-channel` | `test/channel` | channel 名 |
| `-message` | `Hello, Beacon!` | push 的负载 |
| `-count` | 1 | push 的消息条数 |
| `-priority` | `medium` | `low` / `medium` / `high` |
| `-reliable` | `false` | 需要 COMMIT 的可靠投递；默认不可靠 |
| `-target` | 空 | 定向投递的 clientID |
| `-channels` | 空 | 逗号分隔的声明过滤器（pull 模式） |
| `-wait-ms` | 30000 | 长轮询时长，0 = 立即返回 |
| `-max-count` | 32 | 单次拉取的最大记录数 |
| `-interval` | 0 | push 间隔 |
| `-ttl` | 0 | 消息存活时长（0 = 不过期） |

## 注意

`pull` 模式用 `PullBatch` 拉取一批，处理完后用 `CommitAll` 在一个 RTT 内确认整批。
可靠消息在确认前占用会话窗口（默认 `ReceiveMaximum=32`）；只拉不确认最终会停止前进，
并可能收到 `CommitRequired`。
