# Beacon Client CLI

命令行客户端工具，用于与 Beacon Core 进行交互。

## 编译

```bash
cd cmd/client
go build -o client .
```

## 使用方法

### 基本选项

```bash
./client [options]
```

**通用选项:**
- `-core <address>`: Core 地址 (默认: `localhost:5208`)
- `-client-id <id>`: 客户端 ID (默认: 自动生成)
- `-keepalive <seconds>`: 心跳间隔，秒 (默认: 60)
- `-keep-session`: 保持会话状态 (默认: false)
- `-debug`: 启用调试日志
- `-version`: 显示版本信息

### 运行模式

使用 `-mode` 参数选择运行模式：

#### 1. 发布模式 (pub/publish)

向指定主题发布消息：

```bash
# 发布单条消息
./client -mode pub -topic test/topic -message "Hello, Beacon!"

# 发布多条消息，间隔 1 秒
./client -mode pub -topic sensor/temp -message "Temperature" -count 10 -interval 1s

# 连接到自定义 core 地址
./client -core 192.168.1.100:5208 -mode pub -topic test -message "Hello"
```

**发布选项:**
- `-topic <topic>`: 发布主题 (默认: `test/topic`)
- `-message <text>`: 消息内容 (默认: `Hello, Beacon!`)
- `-count <n>`: 发布消息数量 (默认: 1)
- `-interval <duration>`: 消息间隔时间 (默认: `1s`)

#### 2. 订阅模式 (sub/subscribe)

订阅主题并接收消息：

```bash
# 订阅单个主题
./client -mode sub -topic test/topic

# 订阅并保持会话
./client -mode sub -topic sensor/# -keep-session
```

**订阅选项:**
- `-topic <topic>`: 订阅主题 (默认: `test/topic`)

按 Ctrl+C 退出订阅。

#### 3. 请求模式 (req/request)

发送请求并等待响应：

```bash
# 发送单个请求
./client -mode req -action echo -message "Hello"

# 发送多个请求
./client -mode req -action calculate -message "2+2" -count 5 -interval 2s
```

**请求选项:**
- `-action <name>`: Action 名称 (默认: `echo`)
- `-message <text>`: 请求负载 (默认: `Hello, Beacon!`)
- `-count <n>`: 请求数量 (默认: 1)
- `-interval <duration>`: 请求间隔 (默认: `1s`)

#### 4. 轮询模式 (poll/polling)

注册 action 并处理传入的请求：

```bash
# 作为 echo 服务运行
./client -mode poll -action echo

# 作为计算服务运行
./client -mode poll -action calculator
```

**轮询选项:**
- `-action <name>`: 注册的 action 名称 (默认: `echo`)

轮询模式会自动响应收到的请求 (Echo 模式)。按 Ctrl+C 退出。

## 示例场景

### 场景 1: 简单的发布/订阅

终端 1 (订阅者):
```bash
./client -mode sub -topic sensors/temperature
```

终端 2 (发布者):
```bash
./client -mode pub -topic sensors/temperature -message "25.5°C" -count 100 -interval 1s
```

### 场景 2: 请求/响应服务

终端 1 (服务提供者):
```bash
./client -mode poll -action echo -client-id echo-server
```

终端 2 (客户端):
```bash
./client -mode req -action echo -message "Test message" -count 10
```

### 场景 3: 持久会话

```bash
# 首次连接，订阅主题
./client -mode sub -topic my/topic -keep-session -client-id my-client

# 断开后重新连接，会话会恢复
./client -mode sub -topic my/topic -keep-session -client-id my-client
```

## 测试

运行单元测试:

```bash
cd cmd/client
go test -v
```

运行集成测试:

```bash
# 需要先启动 core
cd cmd/core
./core &

# 运行集成测试脚本
cd ../../tests
./integration_test.sh
```

## 调试

启用调试日志查看详细信息:

```bash
./client -debug -mode sub -topic test
```

这将输出详细的连接、认证、消息传输等信息。

## 退出

在任何模式下，按 `Ctrl+C` 可优雅地断开连接并退出。
