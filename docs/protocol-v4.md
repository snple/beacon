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

服务端唯一可能主动发送的包是 `CONNACK`、`PONG`、各请求的响应，以及**连接终止时的 `DISCONNECT`**。

### 2.1 基础类型

- `string`：`uint16` 长度 + UTF-8 字节
- `bytes`：`uint32` 长度 + 原始字节
- `stringmap`：`uint16` 条目数 + (`string` key, `string` value) × N，编码时按键排序
- `PacketID`：12 字节，布局为 `[6B 毫秒时间戳][2B 计数器][4B 随机数]`

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
    CleanSession   uint8      // 1 = 丢弃该 ClientID 的既有会话状态
    ClientID       string     // 空 = 服务端分配
    KeepAlive      uint16     // 秒
    MaxPacketSize  uint32     // 客户端可接收的最大包
    WillFlags      uint8      // bit0 = HasWill
    WillChannel    string
    WillPayload    bytes
    WillHeaders    stringmap
    AuthMethod     string
    AuthData       bytes
    ChannelCount   uint16
    Channels       []string   // 初始声明（可选）
}

CONNACK {
    SessionPresent      uint8
    ReasonCode          uint8
    AssignedClientID    string
    ServerMaxPacketSize uint32
    KeepAlive           uint16   // 服务端要求的心跳间隔（秒），客户端必须以它为准
    SessionID           uint64   // 会话租约标识
    Pending             [3]uint32 // Low / Medium / High 的未投递积压
    Delivered           uint32    // 已投递未确认条数
    OldestUncommitted   PacketID  // delivered 队头；空则全零
}
```

认证为**单轮**：`AuthMethod`/`AuthData` 在 `CONNECT` 中一次提交，失败时 `CONNACK` 携带错误原因码并关闭连接。

会话保留时长完全由**服务端配置**，`CONNECT` 不携带该字段。`CleanSession=1` 时，
服务端丢弃该 ClientID 的既有 pending / delivered / channel 声明，以全新会话建立连接
（`SessionPresent=0`、新的 `SessionID`）。

`CONNACK.KeepAlive` 是服务端最终采用的间隔（可能覆盖客户端在 `CONNECT` 中请求的值）。
客户端必须用它计算发送 `PING` 的频率，否则在服务端配置更短时会周期性被判超时。

`SessionID` 的规则：

| 情况 | SessionPresent | SessionID | 动作 |
|---|---|---|---|
| 首次连接 / 新会话 | false | 新生成 | 新建会话 |
| `CleanSession=1` | false | 新生成 | 清理既有会话后新建 |
| 重连，旧连接已断开 | true | **沿用** | 恢复会话 |
| 重连，旧连接仍存活（含半开） | true | **新生成** | 接管，关闭旧连接 |

旧连接在接管时会收到 `DISCONNECT(SessionTakenOver)`，且不再产生任何状态变更。

### 5.2 PUSH / PUSH_OK

```
PUSH {
    RequestID uint64
    Flags     uint8   // bit0 = WantAck
    Count     uint16
    Records   []Record
}

PUSH_OK {
    RequestID uint64
    Count     uint16
    Results   []{
        Reason  uint8
        ID      PacketID
        Matched uint32   // 入队成功的客户端数
        Dropped uint32   // 目标离线且 Low 被丢弃，或因入队挤出旧消息的客户端数
    }
}
```

- 批量**不做整批原子**，逐条返回结果。
- `WantAck=0` 时服务端**不发送** `PUSH_OK`（发送即可，无需等待）。
- 单条结果原因码：`Success`、`ChannelInvalid`、`NotAuthorized`、`PacketTooLarge`、
  `TargetNotFound`、`NoMatch`、`ConsumerBacklog`、`RateLimited`、`QuotaExceeded`。

### 5.3 PULL / PULL_OK

```
PULL {
    RequestID  uint64
    MaxRecords uint16   // 0 或超上限时使用服务端上限
    MaxBytes   uint32
    WaitMs     uint32   // 长轮询时长；0 = 立即返回
}

PULL_OK {
    RequestID    uint64
    Reason       uint8
    Flags        uint8   // bit0 = More
    RetryAfterMs uint32
    Pending      [3]uint32
    Delivered    uint32
    OldestUncommitted PacketID
    Count        uint16
    Records      []Record
}
```

拉取顺序：

1. **`delivered` 非空时只返回 `delivered`**（`Dup=1`），不混入新消息；
2. 否则按 **High → Medium → Low** 取值，存在更低优先级非空队列时，
   本级最多占剩余预算的 80%（保证下级不会被完全饿死）。

`More=1` 表示被 `MaxRecords`/`MaxBytes` 截断。单条记录大于 `MaxBytes` 时仍至少返回一条。

**同一连接同一时刻只允许一个在途 PULL。** 第二个并发的 `PULL` 会立刻收到
`PULL_OK(Reason=RateLimited, RetryAfterMs=1000)`，不返回任何记录：
并发拉取会让同一条消息被重复投递，也会让客户端的未确认集合互相覆盖。

队列为空的处理完全由 `WaitMs` 决定：`0` 立即返回空结果，`>0` 最多等待该时长
（服务端按 `MaxWaitMs` 裁剪）。

`Reason=CommitRequired` 表示客户端在存在未确认消息时反复拉取，已超过软上限；
此时 `Records` 为空，客户端应先 `COMMIT`。

### 5.4 COMMIT / COMMIT_OK

```
COMMIT {
    RequestID uint64
    Count     uint16
    IDs       []PacketID   // 数量受 MaxPacketSize 约束
}

COMMIT_OK {
    RequestID uint64
    Committed uint16
    Pending   [3]uint32
    Delivered uint32
}
```

- 只影响 `delivered`；对不存在或重复的 ID 是幂等空操作。
- 支持乱序确认（按 ID 匹配），确认后从头部压缩连续已确认的前缀。
- Low 优先级消息永不进入 `delivered`，其 ID 无需提交。

### 5.5 DECLARE / DECLARE_RESP

```
DECLARE {
    RequestID uint64
    Count     uint16
    Channels  []string   // 全量替换；[] = 取消全部；全部需显式 "**"
}

DECLARE_RESP {
    RequestID uint64
    Count     uint16
    Results   []{ Channel string, Reason uint8 }
    Pending   [3]uint32
    Delivered uint32
    Limits {
        MaxPacketSize        uint32
        MaxRecords           uint16
        MaxWaitMs            uint32
        MaxBatch             uint32
        MaxChannelsPerClient uint16
    }
}
```

`DECLARE` 是**全量替换**且幂等。它**只影响之后的匹配**：已入队消息不会被清理，
因此不能用它「退订」已有积压。

### 5.6 PING / PONG / DISCONNECT

```
PING       { Seq uint32, Timestamp uint64 }
PONG       { Seq uint32, Echo uint64, Load uint8 }
DISCONNECT { ReasonCode uint8, Flags uint8 }   // bit0 = SuppressWill
```

`DISCONNECT` 双向：

- C→S：优雅关闭；`ReasonNormalDisconnect` 或 `DisconnectWithWill`，并抑制遗嘱。
- S→C：终止通知；`SessionTakenOver`、`ServerShuttingDown`、`Kicked`、`CommitRequired`。

客户端收到 `SessionTakenOver` 时**不得自动重连**，否则两个同 ClientID 的实例会无限互踢。

---

## 6. channel 语法

- 发布名：不含 `*`、`#`、`+`、`:`
- 过滤器：`*` 匹配单层（必须独占一层），`**` 匹配零或多层（必须在末尾）
- `**` 也匹配空后缀：`a/**` 匹配 `a`、`a/b`、`a/b/c`
- `$` 前缀保留给 broker 内部 channel
- 长度：发布名与过滤器均不得超过服务端配置的上限，可配置范围 **256–1024 字节**
  （默认 256；线格式仍为 `uint16`）。超限时 `PUSH_OK` 返回 `ChannelInvalid`，
  `DECLARE_RESP` 返回 `ChannelFilterInvalid`；`CONNECT` 中的初始声明若超限会被忽略，
  需要逐项结果时请使用 `DECLARE`

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
| `MaxRecords` | 1000 |
| `MaxBytes` | 1 MiB |
| `MaxRecordBytes` | 1 MiB |
| `MaxWaitMs`（服务端裁剪上限） | 300s |
| `MaxChannelsPerClient` | 1024 |
| `MaxChannelLength`（channel 名/过滤器字节数） | 256（可配 256–1024） |
| `SessionTimeout`（离线会话保留时长） | 86400s（0 = 断开即清理） |
| `MaxSessions`（含离线保留） | 10000 |
| `MaxInflightBytes`（全局在途消息） | 256 MiB |
| `NoProgressSoft` / `Hard` | 4 / 16 |
| `UncommittedTimeout` | 关闭 |
