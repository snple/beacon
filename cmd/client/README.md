# Beacon Client CLI

命令行客户端工具，用于与 Beacon Core 进行交互。

## 编译

```bash
go build -o client ./cmd/client
```

## 使用方法

```bash
./client [options]
```

**通用选项：**

| 选项 | 默认值 | 说明 |
|---|---|---|
| `-core <address>` | `localhost:5208` | Core 地址；`ws://` / `wss://` 前缀自动走 WebSocket |
| `-client-id <id>` | 自动生成 | 客户端 ID（非空时由服务端校验） |
| `-keepalive <seconds>` | `60` | 心跳间隔（秒） |
| `-session-timeout <seconds>` | `0` | 会话保留时间（秒）；`0` 表示断开即清理 |
| `-debug` | `false` | 输出调试日志 |
| `-version` | | 显示版本后退出 |

**运行模式**（`-mode`，默认 `pub`）：

### 发布模式（`pub` / `publish`）

| 选项 | 默认值 | 说明 |
|---|---|---|
| `-topic <topic>` | `test/topic` | 发布主题（不允许通配符） |
| `-message <text>` | `Hello, Beacon!` | 消息内容，实际发送为 `<text> #<序号>` |
| `-count <n>` | `1` | 发布条数 |
| `-interval <duration>` | `1s` | 发布间隔 |

```bash
# 发布单条消息
./client -mode pub -topic test/topic -message "Hello, Beacon!"

# 发布 10 条，间隔 1 秒
./client -mode pub -topic sensor/temp -message "Temperature" -count 10 -interval 1s

# 连接自定义 core
./client -core 192.168.1.100:5208 -mode pub -topic test -message "Hello"
```

### 订阅模式（`sub` / `subscribe`）

| 选项 | 默认值 | 说明 |
|---|---|---|
| `-topic <topic>` | `test/topic` | 订阅主题过滤器（支持 `*` / `**`） |

```bash
./client -mode sub -topic test/topic
./client -mode sub -topic 'sensor/**'
```

消息以 `PollMessage` 轮询接收（5 秒超时后继续等待），Ctrl+C 优雅退出。

> 通配符语法为 `*`（单层）与 `**`（多层，只能位于末尾）；MQTT 风格的 `#` / `+` 会被服务端拒绝。

## 示例场景

### 发布/订阅

终端 1（订阅者）：

```bash
./client -mode sub -topic sensors/temperature
```

终端 2（发布者）：

```bash
./client -mode pub -topic sensors/temperature -message "25.5C" -count 100 -interval 1s
```

### WebSocket 传输

```bash
# 服务端：beacon-core --addr :5208 --ws-addr :8080 --ws-path /ws
./client -core ws://127.0.0.1:8080/ws -mode sub -topic 'test/**'
./client -core ws://127.0.0.1:8080/ws -mode pub -topic test/hello -message "hi"
```

### 持久会话

```bash
# 断开后 60 秒内用同一 ClientID 重连，服务端会恢复会话（订阅与待投递消息）
./client -mode sub -topic my/topic -client-id my-client -session-timeout 60
```

## 退出

任何模式下按 `Ctrl+C` 都会发送 DISCONNECT 并优雅关闭客户端。
