# beacon-client

Beacon v4 命令行客户端，用于调试 `PUSH` / `PULL`。

## 用法

```bash
# 订阅并消费（拉取 → 处理 → 整批确认）
go run ./cmd/client -core 127.0.0.1:5208 -client-id c1 -mode pull -channel 'sensors/**'

# 发布
go run ./cmd/client -core 127.0.0.1:5208 -client-id c2 -mode push -channel sensors/temp -message 21.5

# 高优先级 + 定向投递 + 带元数据
go run ./cmd/client -mode push -channel cmd/reboot -priority high -target c1 -message now
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
| `-target` | 空 | 定向投递的 clientID |
| `-channels` | 空 | 逗号分隔的声明过滤器（pull 模式） |
| `-max-records` | 100 | 每次 pull 的最大条数 |
| `-wait-ms` | 30000 | 长轮询时长，0 = 立即返回 |
| `-interval` | 0 | push 间隔 |

## 注意

`pull` 模式在每批消息处理完后会立即调用 `CommitAll`。这是 v4 的必需步骤：
未确认的消息会在下一次 `PULL` 时被重发，反复不确认会被服务端限速甚至断开。
