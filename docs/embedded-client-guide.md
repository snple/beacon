# 嵌入式客户端实现要点

面向用 C / Rust / 裸机等语言自行实现 Beacon v5 客户端的开发者。线格式的权威定义见
[`protocol-v5.md`](./protocol-v5.md)，这里只讲实现时容易出错的点。

## 1. 一个最小客户端需要实现什么

必需：

1. 固定头解析（1 字节类型 + 4 字节大端长度）
2. `CONNECT` 编码 / `CONNACK` 解析（含可选 `Properties`、`Limits` 与 `ReceiveMaximum`）
3. `PULL` 编码 / `PULL_OK` 解析（含一批 Record）
4. `COMMIT` 编码 / `COMMIT_OK` 解析（支持批量与 NACK）
5. `PUSH` 编码 / `PUSH_OK` 解析（只做消费者可以不实现）
6. `PING` / `PONG`

必需（消费者）：`DECLARE`（唯一声明订阅的途径，`CONNECT` 不携带）。
可选：`DISCONNECT`（优雅关闭）。

## 2. 握手顺序

```
客户端                                       服务端
  │ ── CONNECT ──────────────────────────────▶ │
  │ ◀───────────────────────────── CONNACK ── │
  │                                            │
  │ ── DECLARE（订阅声明，必需）──────────────▶ │
  │ ◀────────────────────────── DECLARE_RESP ─ │
  │                                            │
  │ ── PULL(MaxCount=N) ──────────────────────▶ │
  │ ◀───────────────── PULL_OK(N 条 Records) ─ │
  │ ── COMMIT(N 条 Entries) ──────────────────▶ │
  │ ◀───────────────────────────── COMMIT_OK ─ │
```

`CONNECT` 必须是第一个包。版本不是 `5` 时**服务端静默关闭连接**，不返回 `CONNACK`
（协议不向后兼容）。

### 2.1 会话与 CleanSession

- 会话保留时长由**服务端配置**，`CONNECT` 不携带 `SessionTimeout`。
- 订阅声明**不在 `CONNECT` 中**：连接建立后必须用 `DECLARE` 全量声明。
  重连后如依赖既有订阅，建议无条件重发 `DECLARE`（服务端会按新身份重新校验）。
- 每项声明带 1 字节 `Flags`：bit0 `TargetOnly` 表示只接收定向投递（广播发布不命中），
  默认同时接收广播与定向投递；置位保留位会被拒绝（`ChannelFilterInvalid`）。
- `CleanSession`（`CONNECT.Flags` bit0）置位：服务端丢弃该 ClientID 的既有 pending / delivered / channel 声明，
  `CONNACK.SessionPresent=0` 并返回新的 `SessionID`。
- `CleanSession` 未置位：若会话仍被保留，则复用队列与声明（`SessionPresent=1`）。
- 会话离线期间队列保留**全部可靠消息**，不可靠消息直接丢弃；队列满后按可靠性/优先级淘汰。

### 2.2 Properties 与 Limits

`Properties` 是可选块，只有对应 `HasProperties` 标志置位时才出现在包尾：

```
Properties = uint16 条目数 + N × (string key, string value)
```

- 没有属性时**不要**写入该块，也不要只写 2 字节的 `0`；
- 编码时按键排序，解码不要求有序；
- 除保留键外，broker 不解释键值，只透传给钩子或对端；
- `CONNECT`：`Flags` bit1，布局 `... AuthData, [Properties]`；
- `CONNACK`：`Flags` bit1，布局 `... Pending, Limits, [Properties]`；
- `PUSH`：bit1；`PUSH_OK`：bit0；`PULL`：bit0；`PULL_OK`：bit0；
  `DECLARE`：bit0；`DECLARE_RESP`：bit0。各自的 `Properties` 都在包尾。
- 除保留键 `PUSH_OK.Properties.error`（错误细节）外，`Properties` 的键值由应用自定义。

`CONNACK.Limits` 是**固定 26 字节**，即使全为 0 也必须存在：

```
MaxPacketSize        uint32
MaxRecordBytes       uint32
MaxWaitMs            uint32
MaxChannelsPerClient uint16
MaxChannelLength     uint16
MaxPullBatch         uint16
MaxQueueCap          uint32
SessionTimeout       uint32
```

`DECLARE_RESP` 在 `Pending` 之后使用完全相同布局的 `Limits`。客户端应以最近一次
收到的值为准，不要假设服务端默认值。

## 3. Record 编解码

```
Flags      1 字节
ID         12 字节
Channel    2 字节长度 + 字节
Target     2 字节长度 + 字节（可为 0）
Source     2 字节长度 + 字节
Headers    2 字节条目数 + N × (2 字节 key 长度 + key + 2 字节 value 长度 + value)
Payload    4 字节长度 + 字节
ExpiresAt  8 字节 uint64（仅 HasExpiry bit6）
PublishedAt 8 字节 uint64（仅 HasPublishedAt bit7）
```

Flags 位：bit0 Dup，bit1-2 Priority，bit3 HasHeaders，bit4 保留，
bit5 Reliable，bit6 HasExpiry，bit7 HasPublishedAt。

注意：

- **`HasHeaders` 位与 Headers 必须一致**：条目数为 0 时不置位；非 0 时必须置位。
  不一致会被服务端判定为 `MalformedPacket`（并收到 `DISCONNECT`）。
- 字符串用 `uint16` 长度，因此 header key/value 上限 65535 字节；payload 用 `uint32`。
- **channel 名/过滤器另受服务端配置的长度上限约束**：可配置 256–1024 字节，默认 256
  （线格式仍是 `uint16`）。超限时 `PUSH_OK` 返回 `ChannelInvalid`，
  `DECLARE_RESP` 返回 `ChannelFilterInvalid`。
- `Headers` 的键在编码时排序，便于确定性输出；解码不要求有序。
- `Priority` 位：`00`=Medium（默认），`01`=Low，`10`=High；`11` 非法
  （`MalformedPacket`）。
- `Reliable` 取 bit5，默认 `0`（不可靠）；需要 `COMMIT` 时必须显式置位，
  不会从优先级推导。`Dup` 由服务端设置，发布方不应设置。
- `HasExpiry` 置位时，Payload 之后是 8 字节的绝对过期时间（Unix 毫秒）；
  `HasPublishedAt` 同理。两者默认不发送，因此不产生额外带宽。
- **`CONNECT.MaxPacketSize` 最小 64，且必须能容纳 CONNACK**（固定 59 字节 +
  `AssignedClientID` + 可选 `Properties`）；否则服务端返回 `CONNACK(PacketTooLarge)` 后关闭连接。
  超过该上限的记录不会为**在线**目标入队（`PUSH_OK.Dropped` 增加）；重连时调小
  上限会让已入队但放不下的消息被丢弃，以免永久堵住队列。

## 4. 必须遵守的消费循环

```c
for (;;) {
    pull(wait_ms, max_count);            // 阻塞直到有数据或超时
    for (i = 0; i < PULL_OK.Count; i++) {
        handle(PULL_OK.Records[i]);
    }
    commit(PULL_OK.Records, Count);      // 一个 RTT 确认整批
}
```

一次 `PULL` 返回 0..`MaxCount` 条记录，`MaxCount` 会被服务端裁剪到 `MaxPullBatch`
（默认 32）。记录可能包含新消息与 `Dup=1` 的重发消息。

**关键点：窗口。** 未确认的**可靠**消息占用 `CONNACK.ReceiveMaximum` 个槽位
（默认 32）。窗口有空位时 `PULL` 继续投递新消息；窗口满后只重发未确认消息。
不可靠消息不占窗口，但窗口已满时同样会暂停新消息投递（优先重发未确认消息）。

服务端会统计连续无进展的拉取次数，超过阈值会返回 `CommitRequired` 甚至断开连接。
因此：**拉取后尽快批量确认**，不要在窗口里长期积压已处理的消息。

## 5. 优先级与确认

可靠性与优先级是两个独立的位：

| 字段 | 说明 |
|---|---|
| `Reliable=0` | 服务端写出后立即释放，不占窗口、不重发、离线丢弃 |
| `Reliable=1` | 需要 `COMMIT` 或 `NACK`；离线保留；受窗口约束 |
| `Priority` | Low/Medium/High，只影响投递顺序与队列满时的淘汰顺序 |

可靠性默认关闭：不置位 `Reliable`（bit5）的消息一律不可靠，
不会被重发、不占窗口、离线时不保留。需要投递保证时必须显式置位。

`COMMIT` 的每项带 `Action`：`0=Commit`（确认，不再重发）、`1=Nack`（拒绝，不再重发，
服务端可配置钩子做死信）。不可靠消息的 ID 提交后计入 `Unknown`，无副作用。

## 6. 长轮询

**同一连接同一时刻只能有一个在途 PULL。** 还没有收到上一条 `PULL_OK` 就发下一条，
会立刻收到 `PULL_OK(Reason=RateLimited)`（不返回数据）。这是有意为之：并发拉取只会
拿到重复内容，并让本地未确认列表错乱。

队列为空时是否等待由 `WaitMs` 决定（`0` = 立即返回）。`WaitMs > 0` 时服务端会让请求
挂起，直到：

- 有新消息入队；
- 超时 → 返回空 `PULL_OK`（这是**成功**，不是错误）；
- 连接关闭。

建议 `WaitMs` 小于 `KeepAlive * 1000`，并在等待期间照常发 `PING`（不同请求可以在同一连接上并行）。

## 7. 心跳

服务端采用 `CONNACK.KeepAlive` 作为最终心跳间隔（可能覆盖 `CONNECT` 中的值），
并在 `1.5 × KeepAlive` 内没有收到任何包时断开。因此：

- 以 `CONNACK.KeepAlive` 为准，空闲时每 `KeepAlive/2` 发一次 `PING`；
- 长轮询的 `PULL` 不算「已发送」，仍需要 `PING`；
- 客户端也应设置读超时（约 `2 × KeepAlive`）：超过该时长没有任何包（包括 `PONG`）
  就主动断开并重连，否则半开连接会一直表现为“在线”。

## 8. 断线处理

| 收到/发生 | 处理 |
|---|---|
| `DISCONNECT(SessionTakenOver)` | **不要自动重连**，上报给应用（另一个实例抢了同一个 ClientID） |
| `DISCONNECT(Kicked)` | 不要自动重连 |
| `DISCONNECT(ServerShuttingDown)` | 指数退避后重连 |
| `DISCONNECT(CommitRequired)` | 先修好消费循环，再退避重连 |
| TCP 断开 | 退避重连 |

重连后对比 `CONNACK.SessionID`：变化说明会话被重建（或发生过接管），
本地缓存的未确认状态应作废。

## 9. 内存估算

单个客户端在服务端的占用上界：

```
QueueCap 条（pending）  +  ReceiveMaximum 条（delivered，默认 32）
每条约 24 字节索引 + 共享消息体
```

客户端侧最多保存服务端 `CONNACK.ReceiveMaximum` 条未确认消息的 ID。

## 10. 常见错误

| 现象 | 原因 |
|---|---|
| 反复收到同样的消息（`Dup=1`） | 窗口已满且未确认，先 `COMMIT` |
| `PULL_OK.Count=0` 且 `Reason=Success` | 长轮询超时，不是错误 |
| `PULL_OK.Reason = CommitRequired` | 连续无进展 pull 超过软上限 |
| 连接被服务端关闭且收到 `CommitRequired` | 超过硬上限 |
| `PUSH_OK` 返回 `NoMatch` | 没有客户端声明匹配的 channel |
| `PUSH_OK` 返回 `TargetNotFound` | 定向目标是 clientID，且该客户端没有会话 |
| `PUSH_OK` 返回 `NoMatch`（定向） | 目标存在但未声明匹配该 channel 的过滤器 |
| `PUSH_OK` 返回 `NotAuthorized`（定向） | 发布方没有定向投递权限 |
| `PUSH_OK` 返回 `ChannelInvalid`（`$` 前缀） | `$` 前缀保留给 broker，客户端不可发布 |
| `DECLARE_RESP` 返回 `NotAuthorized` | 声明的过滤器超出 ACL 允许范围 |
| `PUSH_OK` 返回 `ChannelInvalid` / `DECLARE_RESP` 返回 `ChannelFilterInvalid` | channel 名或过滤器超过服务端长度上限（默认 256 字节） |
| 收到 `DISCONNECT(MalformedPacket)` | 包体解码失败（长度、优先级位、HasHeaders 不一致等） |
| 收到 `DISCONNECT(ProtocolError)` | 发送了服务端方向的包或重复 `CONNECT` |
| 收不到任何消息 | 检查 `DECLARE` 是否真的成功，以及是否用了 `*` / `**` |
