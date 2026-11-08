# Beacon v4 协议规格

> **一句话**：每客户端队列 + 客户端主动拉取。生产者 `PUSH`，broker 按 channel 匹配入队；
> 消费者 `PULL` 取走、处理、`COMMIT`；服务端除响应外不发送任何数据包。

- 协议版本：`4`
- 传输：TCP / TLS / WebSocket（同一线格式）
- 存储：仅内存，进程重启后队列、会话、`delivered` 全部丢失

---

## 1. 固定头

所有包以 5 字节固定头开始：

| 字段 | 长度 | 说明 |
|---|---|---|
| Type | 1 字节 | 包类型 |
| Remaining Length | 4 字节 | 包体长度，大端序 |

包体上限由 `MaxPacketSize`（服务端配置，默认 16 MiB）约束，协议硬上限 256 MiB。

**帧级超限是连接级错误**：`Remaining Length` 超过本端上限时，包边界已不可信，
无法继续解析后续字节，因此直接终止连接（服务端发 `DISCONNECT(PacketTooLarge)`），
不会返回 `PUSH_OK` 级别的 `PacketTooLarge`。

---

## 2. 包类型

| 值 | 名称 | 方向 |
|---|---|---|
| 1 | `CONNECT` | C→S |
| 2 | `CONNACK` | S→C |
| 3 | `PUSH` | C→S |
| 4 | `PUSH_OK` | S→C |
| 5 | `PULL` | C→S |
| 6 | `PULL_OK` | S→C |
| 7 | `COMMIT` | C→S |
| 8 | `COMMIT_OK` | S→C |
| 9 | `PING` | C→S |
| 10 | `PONG` | S→C |
| 11 | `DISCONNECT` | 双向 |
| 12 | `DECLARE` | C→S |
| 13 | `DECLARE_RESP` | S→C |

> 14–21 保留，不得复用。

服务端唯一可能主动发送的包是 `CONNACK`、`PONG`、各请求的响应，以及**连接终止时的 `DISCONNECT`**。

### 2.1 基础类型

- `string`：`uint16` 长度 + UTF-8 字节
- `bytes`：`uint32` 长度 + 原始字节
- `stringmap`：`uint16` 条目数 + (`string` key, `string` value) × N，编码时按键排序
- `PacketID`：12 字节，布局为 `[6B 毫秒时间戳][2B 计数器][4B 随机数]`
- `RequestID`：`uint32`，由客户端生成并递增，服务端在响应中原样回显；回绕后可直接复用

---

## 3. Record

`PUSH` 与 `PULL_OK` 共用同一编码。

| 字段 | 类型 | 说明 |
|---|---|---|
| Flags | 1 字节 | 见下 |
| ID | PacketID | 服务端生成，客户端不可指定 |
| Channel | string | 分类标签；定向消息中不参与匹配 |
| Target | string | 定向投递目标 clientID；空 = 普通发布 |
| Source | string | 发布者 clientID，服务端填充，不可伪造 |
| Headers | stringmap | 自定义元数据 |
| Payload | bytes | 负载 |

Flags 位：

| 位 | 名称 | 说明 |
|---|---|---|
| 0 | Dup | 重复投递（此前已发出但未确认） |
| 1-2 | Priority | `0`=Low，`1`=Medium，`2`=High |
| 3 | HasHeaders | 携带 headers |
| 4 | HasPriority | 发送方显式指定了优先级 |

`HasHeaders` 必须与实际 `Headers` 一致：有 headers 时置位且条目数 > 0，
无 headers 时不置位且条目数为 0。不一致的包按 `MalformedPacket` 处理。

未设置 `HasPriority` 时，接收方按 **Medium** 处理。

---

## 4. 优先级与可靠性

v4 中优先级同时决定可靠性，不存在独立开关：

| 优先级 | 需要 COMMIT | 写出后 | 队列满时 |
|---|---|---|---|
| High | 是 | 留在队列直到确认 | 最后被丢 |
| Medium（默认） | 是 | 留在队列直到确认 | 其次被丢 |
| Low | 否 | 立即从队列删除 | 最先被丢 |

会话离线时（服务端 `SessionTimeout > 0`），队列**只保留 Medium / High**：
Low 消息在入队前直接丢弃，`PUSH_OK.Dropped` 会包含这些客户端；队列满后仍按上表淘汰。

---

## 5. 各包字段

### 5.1 CONNECT / CONNACK

```
CONNECT {
    Version        uint8      // 必须为 4
    Flags          uint8      // bit0 = CleanSession；其余保留，必须为 0
    ClientID       string     // 空 = 服务端分配
    KeepAlive      uint16     // 秒
    MaxPacketSize  uint32     // 客户端可接收的最大包
    WillFlags      uint8      // bit0 = HasWill
    WillChannel    string
    WillPayload    bytes
    WillHeaders    stringmap
    AuthMethod     string
    AuthData       bytes
}

CONNACK {
    Flags               uint8   // bit0 = SessionPresent；其余保留
    ReasonCode          uint8
    AssignedClientID    string
    ServerMaxPacketSize uint32
    KeepAlive           uint16   // 服务端要求的心跳间隔（秒），客户端必须以它为准
    SessionID           uint64   // 会话租约标识
    Pending             [3]uint32 // Low / Medium / High 的未投递积压（不含已投递未确认的那条）
}
```

已投递未确认的消息不计入 `Pending`；客户端重连后的 `PULL` 会以 `Dup=1`
重新返回这些消息，确认后才继续取新消息。

认证为**单轮**：`AuthMethod`/`AuthData` 在 `CONNECT` 中一次提交，失败时 `CONNACK` 携带错误原因码并关闭连接。

`CONNECT.Flags` 目前定义：

| 位 | 名称 | 说明 |
|---|---|---|
| 0 | `CleanSession` | 丢弃该 ClientID 的既有会话状态 |
| 1-7 | 保留 | 必须为 0 |

`CONNACK.Flags` 目前定义：

| 位 | 名称 | 说明 |
|---|---|---|
| 0 | `SessionPresent` | 复用了既有会话 |
| 1-7 | 保留 | 后续能力开关 |

会话保留时长完全由**服务端配置**，`CONNECT` 不携带该字段。`CleanSession` 置位时，
服务端丢弃该 ClientID 的既有 pending / delivered / channel 声明，以全新会话建立连接
（`SessionPresent=0`、新的 `SessionID`）。

订阅声明**不属于 `CONNECT`**，统一由 `DECLARE` 全量替换：

- 重连时不会携带过期声明；
- 恢复会话时服务端保留既有声明，但会按**本次认证得到的身份重新校验**，
  超出新身份授权范围的过滤器会被丢弃（`DECLARE_RESP` 不会回显这些项，
  需要确认时重新 `DECLARE` 即可拿到失败原因）。

**版本协商**：协议不做向后兼容。版本不是 `4`（或旧版布局根本无法解码）时，
服务端**静默关闭连接**，不返回 `CONNACK`。`ReasonUnsupportedProtocol` 仅为保留码，
当前实现不会发送。

`CONNACK.KeepAlive` 是服务端最终采用的间隔（可能覆盖客户端在 `CONNECT` 中请求的值）。
客户端必须用它计算发送 `PING` 的频率，否则在服务端配置更短时会周期性被判超时。

`SessionID` 的规则：

| 情况 | SessionPresent | SessionID | 动作 |
|---|---|---|---|
| 首次连接 / 新会话 | false | 新生成 | 新建会话 |
| `CleanSession` 置位 | false | 新生成 | 清理既有会话后新建 |
| 重连，旧连接已断开 | true | **沿用** | 恢复会话 |
| 重连，旧连接仍存活（含半开） | true | **新生成** | 接管，关闭旧连接 |

旧连接在接管时会收到 `DISCONNECT(SessionTakenOver)`，且不再产生任何状态变更。

### 5.2 PUSH / PUSH_OK

```
PUSH {
    RequestID uint32
    Flags     uint8   // bit0 = WantAck
    Record    Record
}

PUSH_OK {
    RequestID uint32
    Reason    uint8
    ID        PacketID
    Matched   uint32   // 入队成功的客户端数
    Dropped   uint32   // 目标离线且 Low 被丢弃，或因入队挤出旧消息的客户端数
}
```

- 每次 `PUSH` 只携带一条记录。
- `WantAck=0` 时服务端**不发送** `PUSH_OK`（发送即可，无需等待）。
- `PUSH_OK` 始终回显该记录的 `ID` 与处理结果，成功时 `Reason=Success`。
- 触发发布限流时返回 `Reason=RateLimited`，`ID` 仍由服务端生成。
- 原因码：`Success`、`ChannelInvalid`、`NotAuthorized`、`PacketTooLarge`、
  `TargetNotFound`、`NoMatch`、`ConsumerBacklog`、`RateLimited`、`QuotaExceeded`。
- `PacketTooLarge` 是**记录级**结果（单条记录超过 `MaxRecordBytes`）；帧级超限在
  读取阶段就终止连接，见 §1。

发布者权限：

- 客户端不得发布到 `$` 开头的保留 channel（返回 `ChannelInvalid`），只有 broker 内部
  可以产生这类消息。
- **定向投递**（`Target` 非空）需要额外的发布权限；未配置认证器时默认允许，
  配置认证器后要求 `Identity.AllowTargeted`。
- 定向投递还要求**目标已声明匹配该 `Channel` 的过滤器**，否则返回 `NoMatch`：
  定向投递不是绕过订阅/读权限的后门。只接收定向投递的声明（见 §5.5）同样满足该要求。

### 5.3 PULL / PULL_OK

```
PULL {
    RequestID uint32
    WaitMs    uint32   // 长轮询时长；0 = 立即返回
}

PULL_OK {
    RequestID uint32
    Reason    uint8
    Flags     uint8   // bit0 = HasRecord
    Pending   [3]uint32
    Record    Record  // 仅 HasRecord=1 时存在
}
```

拉取顺序：

1. `delivered` 为空：按 **High → Medium → Low** 取一条新消息；
2. `delivered` 非空且存在严格更高优先级的 pending 消息：投递该新消息；
3. 否则重发 `delivered` 中的未确认消息（`Dup=1`）。

每次 `PULL` 最多返回一条记录；队列为空且未命中长轮询数据时返回 `HasRecord=0`。

`delivered` 提供 2 个投递位（Medium / High 各一，Low 不占位）：

- `delivered` 为空时，按 High → Medium → Low 取一条新消息；
- 存在未确认消息时，只有**优先级严格更高**的新消息可以占用空闲投递位；
- 否则 `PULL` 以 `Dup=1` 重发未确认消息，不取同级或更低优先级的新消息。

因此未确认的 Medium 不会阻塞 High；同一优先级仍保持一次一条的确认节奏。

**同一连接同一时刻只允许一个在途 PULL。** 第二个并发的 `PULL` 会立刻收到
`PULL_OK(Reason=RateLimited)`，不返回任何记录；客户端应在约 1 秒后重试：
并发拉取会让同一条消息被重复投递，也会让客户端的未确认集合互相覆盖。

队列为空的处理完全由 `WaitMs` 决定：`0` 立即返回空结果，`>0` 最多等待该时长
（服务端按 `MaxWaitMs` 裁剪）。

`Reason=CommitRequired` 表示客户端在存在未确认消息时反复拉取，已超过软上限；
此时 `HasRecord=0`，客户端应先 `COMMIT`。

### 5.4 COMMIT / COMMIT_OK

```
COMMIT {
    RequestID uint32
    ID        PacketID
}

COMMIT_OK {
    RequestID uint32
    Pending   [3]uint32
}
```

- 每次 `COMMIT` 只确认一条消息。
- 只影响 `delivered`；对不存在或重复的 ID 是幂等空操作。
- 确认后释放对应投递位。
- Low 优先级消息永不进入 `delivered`，其 ID 无需提交。

### 5.5 DECLARE / DECLARE_RESP

```
DECLARE {
    RequestID uint32
    Count     uint16
    Channels  []{ Channel string, Flags uint8 }   // 全量替换；[] = 取消全部；全部需显式 "**"
}

DECLARE_RESP {
    RequestID uint32
    Accepted  uint16   // 被接受的声明数量
    Count     uint16   // Results 条数（只含失败项）
    Results   []{ Channel string, Reason uint8 }
    Pending   [3]uint32
    Limits {
        MaxPacketSize        uint32
        MaxWaitMs            uint32
        MaxChannelsPerClient uint16
    }
}
```

`DECLARE` 是**全量替换**且幂等。它**只影响之后的匹配**：已入队消息不会被清理，
因此不能用它「退订」已有积压。

每项声明的 `Flags`：

| 位 | 名称 | 语义 |
|---|---|---|
| bit0 | `TargetOnly` | 只接收**定向投递**：广播发布（`Target` 为空）不会命中该过滤器 |
| 其他 | — | 保留；置位时该声明被拒绝（`ChannelFilterInvalid`） |

未置位 `TargetOnly`（默认）时同时接收广播与定向投递。定向投递仍要求目标声明过
匹配该 `Channel` 的过滤器，`TargetOnly` 只是不参与广播匹配，不会扩大接收范围。

同一过滤器在一个 `DECLARE` 中重复声明时按**更宽松的模式**合并：只要有一次未置位
`TargetOnly`，该过滤器就参与广播匹配（重复项按接受计，不占用响应体积）。
`TargetOnly` 是逐项生效的：同一客户端可以同时声明 `jobs/**`（只定向投递）与
`jobs/public`（广播可达），广播只会命中后者。

响应压缩：`Results` **只回显被拒绝的项**，`Accepted` 给出接受数量（重复项按接受计）。
这样可以避免回显大量成功项把响应撑过对端声明的 `MaxPacketSize`。
声明数量超过 `MaxChannelsPerClient` 时**整批拒绝**（不部分生效），
此时 `Accepted=0` 且 `Results` 只有一条 `QuotaExceeded`（`Channel` 为空）。
失败项同样会按对端 `MaxPacketSize` 裁剪（至少保留一条）。

### 5.6 PING / PONG / DISCONNECT

```
PING       { Seq uint32, Timestamp uint64 }
PONG       { Seq uint32, Echo uint64 }
DISCONNECT { ReasonCode uint8, Flags uint8 }   // bit0 = SuppressWill
```

`DISCONNECT` 双向：

- C→S：优雅关闭；`ReasonNormalDisconnect` 或 `DisconnectWithWill`，并抑制遗嘱。
- S→C：终止通知；`SessionTakenOver`、`ServerShuttingDown`、`Kicked`、`CommitRequired`、
  `MalformedPacket`、`ProtocolError`、`PacketTooLarge`、`KeepAliveTimeout`。

服务端在因**协议错误**（非法包类型、记录解码失败、包体超限）关闭连接前，
一律先发送带对应原因码的 `DISCONNECT`，客户端据此区分「自己发错了」与「网络断开」。
客户端向服务端发送服务端方向的包（如 `CONNACK`、`PULL_OK`）同样触发
`DISCONNECT(ProtocolError)`。

客户端收到 `SessionTakenOver` 时**不得自动重连**，否则两个同 ClientID 的实例会无限互踢。


---

## 6. channel 语法

- 发布名：不含 `*`、`#`、`+`、`:`
- 过滤器：`*` 匹配单层（必须独占一层），`**` 匹配零或多层（必须在末尾）
- `**` 也匹配空后缀：`a/**` 匹配 `a`、`a/b`、`a/b/c`
- `$` 前缀保留给 broker 内部 channel：客户端不得发布或声明 `$` 开头的名字
  （发布返回 `ChannelInvalid`）；以通配符开头的过滤器（`*` / `**`）**不匹配**
  `$` 开头的 channel，只有显式以 `$` 开头的过滤器才能命中
- 长度：发布名与过滤器均不得超过服务端配置的上限，可配置范围 **256–1024 字节**
  （默认 256；线格式仍为 `uint16`）。超限时 `PUSH_OK` 返回 `ChannelInvalid`，
  `DECLARE_RESP` 返回 `ChannelFilterInvalid`

ACL 使用「模式包含」判定：声明过滤器 `F` 时要求存在允许模式 `A` 使得 `A` 覆盖 `F` 的全部可匹配集合。

---

## 7. 可靠性与已知限制

| 项 | 语义 |
|---|---|
| 投递语义 | at-least-once：未 `COMMIT` 的消息会重发（`Dup=1`） |
| 离线队列 | 只保留 Medium / High；Low 直接丢弃 |
| 顺序 | 同一优先级内 FIFO；跨优先级无序 |
| 历史回放 | 不支持（channel 是路由键，不存历史） |
| 重复流量 | 等于未确认集合规模；客户端应在拉取新数据前提交 |
| 重启 | 全部状态丢失，`SessionID` 变化 |
| Low 优先级 | 可能未经投递就被淘汰 |
| 队列满 | 丢弃最低优先级的旧消息；新消息优先级更低时丢弃新消息 |

---

## 8. 服务端默认上限

| 项 | 默认 |
|---|---|
| `QueueCap`（pending 条数） | 10000 |
| `MaxRecordBytes`（单条记录：payload + channel/target/source/headers） | 1 MiB |
| `MaxWaitMs`（服务端裁剪上限） | 300s |
| `MaxChannelsPerClient` | 1024 |
| `MaxChannelLength`（channel 名/过滤器字节数） | 256（可配 256–1024） |
| `MaxClients`（在线连接数） | 10000 |
| `MaxHandshakeConnections`（认证前并发连接） | 512 |
| `SessionTimeout`（离线会话保留时长） | 86400s（0 = 断开即清理） |
| `MaxSessions`（含离线保留） | 10000 |
| `MaxInflightBytes`（全局在途消息） | 256 MiB |
| `NoProgressSoft` / `Hard` | 4 / 16 |
| `UncommittedTimeout` | 关闭 |
