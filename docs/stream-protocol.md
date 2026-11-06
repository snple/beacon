# Beacon stream 数据面协议

stream 是 Beacon 的可选能力：两个已认证客户端经过 core 中继建立一条**点对点 TCP 通道**。
控制面（协商、票据）走主连接，见 [protocol-v4.md](./protocol-v4.md) 的 `STREAM_*`；
本文件只描述 core 独立 stream 端口上的数据面协议。

- 协议版本：`1`
- 传输：TCP 或 TLS（由 `STREAM_OPEN_OK` / `STREAM_REQUEST` 中的 `TLS` 告知）
- 端口：独立于消息服务端口，由 `Core.ServeStreamTCP/ServeStreamTLS` 启动
- 状态：纯内存，不排队、不持久化、不跨连接恢复

---

## 1. 角色

| 角色 | 说明 |
|---|---|
| initiator（A） | 调用 `OpenStream` 的一方 |
| acceptor（B） | 收到 `STREAM_REQUEST` 并接受的一方 |
| core | 负责校验票据、配对两条数据面连接并转发字节 |

一条流需要**两条**数据面连接：A 一条，B 一条。core 通过 `StreamID` + `Ticket` +
`Challenge` 把它们关联起来。

---

## 2. 固定头与握手包

固定头与主协议一致：

| 字段 | 长度 | 说明 |
|---|---|---|
| Type | 1 字节 | 包类型 |
| Length | 4 字节 | 包体长度，大端序 |

握手包上限 **64 KiB**（`stream/packet.MaxPacketSize`）。数据面只有两种包：

| 值 | 名称 | 方向 | 说明 |
|---|---|---|---|
| `0x01` | `BIND` | 客户端 → core | 绑定到某个待建流 |
| `0x02` | `BIND_OK` | core → 客户端 | 绑定结果；成功即表示流已建立 |

基础类型与主协议相同：`string`（uint16 长度）、`bytes`（uint32 长度）。

### 2.1 BIND

```
BIND {
    Version   uint8      // 必须为 1
    StreamID  uint64     // 控制面分配的流 ID
    Role      uint8      // 0 = initiator，1 = acceptor
    Ticket    bytes      // 服务端票据
    Challenge bytes      // 接受方生成的随机校验值
}
```

`BIND` 必须是数据面连接的第一个包。core 校验：

1. `StreamID` 对应一个已接受、未过期的待建流；
2. `Ticket` 与 `Challenge` 都和记录一致（常量时间比较）；
3. 双方主连接仍是所属会话的当前连接（防止会话接管后旧连接继续使用票据）；
4. 该角色尚未绑定。

### 2.2 BIND_OK

```
BIND_OK {
    StreamID   uint64
    Role       uint8
    ReasonCode uint8
    Message    string
}
```

- **校验失败**：立即返回 `BIND_OK(error)` 并关闭连接，不等待对端。
  原因包括 `StreamNotFound`、`StreamLimitExceeded`、`BindFailed`、`NotAuthorized`、
  `ServerUnavailable`。
- **校验通过**：不立即回复，连接挂起等待对端 `BIND`。
- **两端都校验通过**：core 向两端各发送一个 `BIND_OK(Success)`，随后进入裸字节转发。

所以正常建流只有一次往返：

```
B ──BIND──────────────▶ core    （校验通过后挂起）
A ──BIND──────────────▶ core
B ◀────────BIND_OK──── core    （两端已配对）
A ◀────────BIND_OK──── core
        （之后为裸字节流）
```

接受方在发出 `STREAM_ACCEPT` 后立即连接数据面；`ACCEPT` 与 `BIND` 走两条连接，
BIND 可能先到。core 会为 ACCEPT 保留最多 1 秒的追赶窗口，超时则本端收到
`BindFailed("stream not accepted")`。等待前会先校验票据，并且接受前只允许
acceptor 角色 BIND，避免无关连接占用窗口。

如果对端一直不出现，core 在 `BindTimeout` 到期后关闭挂起的连接：
已接受并已通知发起方的流，两端都会收到 `STREAM_ABORT(StreamTimeout)`；
尚未接受的请求则让发起方的 `STREAM_OPEN_OK` 直接返回失败。

---

## 3. 建立后的转发

- `BIND_OK(Success)` 之后，连接上不再有任何帧格式，双方直接收发原始字节。
- core 在两个方向各用一个 32 KiB 缓冲做阻塞式 `Read`/`Write`，天然提供 TCP 背压。
- 一端 `Read` 返回 `EOF` 时，core 对另一端执行 `CloseWrite`（半关闭），反向数据仍可继续。
- 任一端出现非 `EOF` 错误时，core 立即关闭两端。
- 可选 `StreamIdleTimeout`：每次读之前设置读截止时间，超时即关闭两端；默认关闭。
- 可选 `StreamWriteTimeout`：每次写之前设置写截止时间，超时即关闭两端；
  默认关闭，只依赖 TCP 背压。
- core 只统计：活跃流数、累计建立流数、被拒绝请求数、双向字节数。
  不缓存、不重放、不写入消息队列。

---

## 4. 生命周期

| 事件 | 行为 |
|---|---|
| 任一方主连接断开 | 关闭该客户端关联的全部待建流与活跃流 |
| 会话被同 ClientID 接管 | 旧连接关联的流全部关闭 |
| 目标拒绝 | 发起方收到 `STREAM_OPEN_OK(StreamRejected)` |
| 接受后未在 `BindTimeout` 内完成双端绑定 | 两端收到 `STREAM_ABORT(StreamTimeout)` |
| core 停止 | 关闭 stream 监听器与所有数据面连接 |

流不跨重连恢复：客户端重连后需要重新发起 `OpenStream`。

---

## 5. 安全要点

- `Ticket` + `Challenge` 是**一次性 bearer 凭证**；数据面连接在绑定前不做独立认证，
  因此建议在不可信网络使用 TLS（`TLS=1`）。
- 票据有效期短（默认 30s）、一次性、绑定 `StreamID` 与双方会话所有权。
- 建流策略由 `Hooks.Stream.OnStreamOpen` 决定；默认放行所有已认证客户端。
- 目标必须在线、声明过 stream 能力；发起方必须仍然是当前会话的连接所有者。
- 默认要求数据面连接的源 IP 与主连接一致（`StreamAllowAnySourceIP=false`），
  防止票据泄漏后被其他主机冒用；主连接经代理而数据面直连时可关闭。
