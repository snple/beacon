# Flexible Listener API

Core 提供了最大灵活性的 API 设计，将连接监听和连接处理完全解耦。

## 核心 API

Core 只提供一个主要API：
```go
func (c *Core) HandleConn(conn net.Conn) error
```

这意味着：
- ✅ Core 不管理监听器（listener），只处理已建立的连接
- ✅ 用户完全控制如何监听（TCP、TLS、WebSocket、QUIC、Unix Socket等）
- ✅ 最大的灵活性和可扩展性

## 使用方式

### 1. 使用辅助方法 ServeTCP（最简单）

```go
c, _ := core.NewWithOptions(core.NewCoreOptions())
c.Start()

// 阻塞式启动TCP服务
if err := c.ServeTCP(":3883"); err != nil {
    log.Fatal(err)
}
```

### 2. 使用辅助方法 ServeTLS

```go
cert, _ := tls.LoadX509KeyPair("cert.pem", "key.pem")
tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}

c, _ := core.NewWithOptions(core.NewCoreOptions())
c.Start()

// 阻塞式启动TLS服务
if err := c.ServeTLS(":8883", tlsConfig); err != nil {
    log.Fatal(err)
}
```

### 3. 使用 Serve 方法（任意 Listener）

```go
// 创建任意类型的listener
listener, _ := net.Listen("unix", "/tmp/beacon.sock")

c, _ := core.NewWithOptions(core.NewCoreOptions())
c.Start()

// 阻塞式处理连接
if err := c.Serve(listener); err != nil {
    log.Fatal(err)
}
```

### 4. 完全手动控制（最大灵活性）

```go
listener, _ := net.Listen("tcp", ":3883")

c, _ := core.NewWithOptions(core.NewCoreOptions())
c.Start()

// 自己实现accept循环
for {
    conn, err := listener.Accept()
    if err != nil {
        log.Fatal(err)
    }

    // 将连接交给Core处理
    if err := c.HandleConn(conn); err != nil {
        log.Printf("HandleConn error: %v", err)
    }
}
```

### 5. 多监听器（不同端口/协议）

```go
c, _ := core.NewWithOptions(core.NewCoreOptions())
c.Start()

// TCP on 3883
go c.ServeTCP(":3883")

// TLS on 8883
go c.ServeTLS(":8883", tlsConfig)

// Unix Socket
go func() {
    listener, _ := net.Listen("unix", "/tmp/beacon.sock")
    c.Serve(listener)
}()

// WebSocket (需要额外库)
go func() {
    // 创建WebSocket listener (伪代码)
    wsListener := createWebSocketListener(":9883")
    c.Serve(wsListener)
}()

// QUIC (需要额外库)
go func() {
    // 创建QUIC listener (伪代码)
    quicListener := createQUICListener(":10883")
    c.Serve(quicListener)
}()

// 等待信号
<-sigChan
c.Stop()
```

## 扩展到其他协议

由于 Core 只需要 `net.Conn`，你可以轻松支持任何协议：

### WebSocket 示例（概念）
```go
// 使用 gorilla/websocket
func serveWebSocket(c *core.Core) {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        conn, _ := upgrader.Upgrade(w, r, nil)
        wsConn := &websocketConn{conn} // 实现 net.Conn 接口
        c.HandleConn(wsConn)
    })
    http.ListenAndServe(":9883", nil)
}
```

### QUIC 示例（概念）
```go
// 使用 quic-go
func serveQUIC(c *core.Core) {
    listener, _ := quic.ListenAddr(":10883", tlsConfig, nil)
    for {
        session, _ := listener.Accept(context.Background())
        stream, _ := session.AcceptStream(context.Background())
        quicConn := &quicConn{stream} // 实现 net.Conn 接口
        c.HandleConn(quicConn)
    }
}
```

## 优势

1. **解耦**: Core 专注于协议逻辑，不关心传输层
2. **灵活**: 支持任何实现 `net.Conn` 的连接
3. **简单**: API极其简单，只有一个核心方法
4. **可扩展**: 轻松添加新的传输协议
5. **可测试**: 容易模拟和测试

## 完整示例

见 [examples/flexible_listeners/main.go](../examples/flexible_listeners/main.go)
