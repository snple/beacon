# Beacon v5 协议规格

> **一句话**：每客户端队列 + 客户端主动拉取。生产者 `PUSH`，broker 按 channel 匹配入队；
> 消费者 `PULL` 一批、处理、`COMMIT` 或 `NACK`；服务端除响应外不发送任何数据包。

- 协议版本：`5`
- 传输：TCP / TLS / WebSocket（同一线格式）
- 存储：仅内存，进程重启后队列、会话、`delivered` 全部丢失

v5 与 v4 **二进制不兼容**。主要变化：

- 可靠性（`Reliable`）与优先级（`Priority`）相互独立，且默认不可靠；
- `PULL` / `PULL_OK` 与 `COMMIT` / `COMMIT_OK` 支持批量；
- 每会话有未确认可靠消息窗口（`ReceiveMaximum`），允许流水线消费；
- `COMMIT` 支持 `NACK`；
- `CONNECT` / `CONNACK` / `PUSH` / `PUSH_OK` / `PULL` / `PULL_OK` / `DECLARE` /
  `DECLARE_RESP` 均可携带可选 `Properties`，并在 `CONNACK` 中返回服务端上限。

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
- `RequestID`：`uint32`，由客户端生成并递增，服务端在响应中原样回显；回绕后可直接复用。
  `0` 保留，客户端应使用非 0 值（Go SDK 会忽略 `RequestID=0` 的响应）

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
| ExpiresAt | uint64 | 可选，仅 HasExpiry=1：过期时间（Unix 毫秒） |
| PublishedAt | uint64 | 可选，仅 HasPublishedAt=1：发布者时间戳（Unix 毫秒） |

Flags 位：

| 位 | 名称 | 说明 |
|---|---|---|
| 0 | Dup | 重复投递（此前已发出但未确认） |
| 1-2 | Priority | `00`=Medium（默认），`01`=Low，`10`=High，`11`=非法 |
| 3 | HasHeaders | 携带 headers |
| 4 | 保留 | 必须为 0 |
| 5 | Reliable | 需要 `COMMIT` 确认；`0` = 不可靠（默认） |
| 6 | HasExpiry | 携带 ExpiresAt |
| 7 | HasPublishedAt | 携带 PublishedAt |

- `HasHeaders` 必须与实际 `Headers` 一致：有 headers 时置位且条目数 > 0，
  无 headers 时不置位且条目数为 0。不一致的包按 `MalformedPacket` 处理。
- `Priority` 的 `00` 是默认值 Medium，因此全 0 的 Flags 表示 Medium + 不可靠；
  `11` 是非法编码，按 `MalformedPacket` 处理。
- `Reliable` 默认是 `0`（不可靠）。**可靠性不会从优先级推导**：需要 `COMMIT`
  的消息必须显式置位 bit5；只设置优先级不会让它变成可靠消息。
- `ExpiresAt` 是绝对时间（Unix 毫秒）。已过期的消息不会入队，入队后过期会在
  `PULL` 或会话清理时丢弃；`ExpiresAt=0` 表示永不过期。
- `PublishedAt` 是发布者提供的时间戳，broker 只透传、不用于过期判断；
  使用 `WithTTL` 时 SDK 会自动同时设置它。

### 3.1 属性约定

`CONNECT` / `CONNACK` / `PUSH` / `PUSH_OK` / `PULL` / `PULL_OK` / `DECLARE` /
`DECLARE_RESP` 都可以携带可选的 `Properties`：

- 批量 PULL、批量 COMMIT/NACK、NACK、TTL 都是 v5 的固定能力，不通过能力协商开关；
- 除保留键 `error`（`PUSH_OK` 拒绝时的可读细节）外，broker 不解释键值，原样透传；
- `Properties` 仅当对应包的 `HasProperties` 标志置位时出现，没有属性时省略整块。

---

## 4. 优先级与可靠性

两个维度相互独立：

| 维度 | 取值 | 作用 |
|---|---|---|
| `Priority` | Low / Medium / High | 投递顺序、队列满时的淘汰顺序 |
| `Reliable` | true / false | 是否需要 `COMMIT`、离线是否保留、是否占用窗口；默认 false |

| 组合 | 需要 COMMIT | 离线队列 | 占 `ReceiveMaximum` |
|---|---|---|---|
| 任意优先级 + Reliable | 是 | 保留 | 是 |
| 任意优先级 + Unreliable | 否 | 丢弃 | 否 |

默认所有消息都不可靠。需要投递保证时必须显式设置 `Reliable=true`。
例如「低紧急但必须送达」可以用 `Low + Reliable`：平时排在 High/Medium 之后，
但不会被淘汰、离线也保留，直到客户端确认。

队列满时按下列顺序淘汰（先淘汰价值最低的）：

1. 不可靠消息优先于可靠消息；
2. 同可靠性下 Low → Medium → High；
3. 同队列内淘汰最旧的一条。

若新消息的价值低于当前最低非空队列，则丢弃新消息本身，而不是挤出旧消息。

会话离线时（服务端 `SessionTimeout > 0`），**可靠消息全部保留**，不可靠消息直接丢弃，
`PUSH_OK.Dropped` 会包含这些客户端。

---

## 5. 各包字段

### 5.1 CONNECT / CONNACK

```
CONNECT {
    Version         uint8      // 必须为 5
    Flags           uint8      // bit0 = CleanSession，bit1 = HasProperties
    ClientID        string     // 空 = 服务端分配
    KeepAlive       uint16     // 秒
    MaxPacketSize   uint32     // 客户端可接收的最大包；0 = 按服务端上限；最小 64 且必须容纳 CONNACK
    AuthMethod      string
    AuthData        bytes
    Properties      stringmap  // 可选，仅 HasProperties=1；客户端 → 服务端
}

CONNACK {
    Flags            uint8     // bit0 = SessionPresent，bit1 = HasProperties
    ReasonCode       uint8
    AssignedClientID string
    SessionID        uint64    // 会话租约标识
    KeepAlive        uint16    // 服务端要求的心跳间隔（秒），可能被服务端上限裁剪
    ReceiveMaximum   uint16    // 服务端为该会话采用的未确认窗口
    Pending          [3]uint32 // Low / Medium / High 的未投递积压（不含已投递未确认的那些）
    Limits           Limits    // 服务端施加的主要限制，见下
    Properties       stringmap // 可选，仅 HasProperties=1；服务端 → 客户端
}

Limits {
    MaxPacketSize        uint32
    MaxRecordBytes       uint32
    MaxWaitMs            uint32
    MaxChannelsPerClient uint16
    MaxChannelLength     uint16
    MaxPullBatch         uint16
    MaxQueueCap          uint32
    SessionTimeout       uint32   // 秒；0 = 断开即清理
}
```

`ReceiveMaximum` 是**未确认可靠消息的窗口**，完全由服务端决定：

- `CONNECT` 不携带该字段，客户端不能请求窗口大小；
- `CONNACK.ReceiveMaximum` 是服务端配置的最终值，客户端必须以它为准；
- 不可靠消息不占用窗口；但窗口已满时服务端会暂停投递新消息（包括不可靠消息），
  优先重发未确认的可靠消息。

`Properties` 是双向的 `stringmap`，用于交换连接期的附加信息：

- 键和值都是 UTF-8 字符串；除保留键 `error` 外，broker 不解释具体含义；
- `CONNECT.Properties` 可在认证钩子（`AuthContext.Properties`）与连接钩子
  （`ConnectContext.Properties`）中读取；
- `CONNACK.Properties` 由服务端配置或认证钩子（`AuthContext.ResponseProperties`）提供，
  客户端连接完成后可读取；
- 编码时按键排序；大小受对端 `MaxPacketSize` 约束，不做额外限制；
- 属性只属于本次连接，不写入会话，也不会跨重连保留；
- 如果最终 `CONNACK`（含属性）超过客户端声明的 `MaxPacketSize`，服务端返回
  `CONNACK(PacketTooLarge)` 并关闭连接，不会写出超限响应。

`CONNACK.Limits` 让客户端在握手阶段就得到服务端限制，避免靠猜测或反复试错：

| 字段 | 含义 |
|---|---|
| `MaxPacketSize` | 服务端可接收的单个包上限；客户端发送包也不得超过该值 |
| `MaxRecordBytes` | 单条 Record 的编码上限 |
| `MaxWaitMs` | `PULL.WaitMs` 的服务端裁剪上限 |
| `MaxChannelsPerClient` | `DECLARE` 最多可声明的 channel 数 |
| `MaxChannelLength` | channel 名/过滤器的最大字节数 |
| `MaxPullBatch` | 单次 `PULL_OK` 最多返回的记录数 |
| `MaxQueueCap` | 每会话 pending 队列容量 |
| `SessionTimeout` | 离线会话保留时长（秒） |

`Limits.MaxPacketSize` 同时是客户端发送上限，不再单独提供 `ServerMaxPacketSize`。
这些值也可能随后续 `DECLARE_RESP.Limits` 更新；客户端应以最近一次收到的值为准。

已投递未确认的消息不计入 `Pending`；客户端重连后的 `PULL` 会以 `Dup=1`
重新返回这些消息，确认后才继续取新消息。

认证为**单轮**：`AuthMethod`/`AuthData` 在 `CONNECT` 中一次提交，失败时 `CONNACK` 携带错误原因码并关闭连接。

`CONNECT.Flags` 目前定义：

| 位 | 名称 | 说明 |
|---|---|---|
| 0 | `CleanSession` | 丢弃该 ClientID 的既有会话状态 |
| 1 | `HasProperties` | 末尾携带 `Properties` |
| 2-7 | 保留 | 必须为 0 |

`CONNACK.Flags` 目前定义：

| 位 | 名称 | 说明 |
|---|---|---|
| 0 | `SessionPresent` | 复用了既有会话 |
| 1 | `HasProperties` | 末尾携带 `Properties` |
| 2-7 | 保留 | 后续能力开关 |

会话保留时长完全由**服务端配置**，`CONNECT` 不携带该字段。`CleanSession` 置位时，
服务端丢弃该 ClientID 的既有 pending / delivered / channel 声明，以全新会话建立连接
（`SessionPresent=0`、新的 `SessionID`）。

订阅声明**不属于 `CONNECT`**，统一由 `DECLARE` 全量替换：

- 重连时不会携带过期声明；
- 恢复会话时服务端保留既有声明，但会按**本次认证得到的身份重新校验**，
  超出新身份授权范围的过滤器会被丢弃（`DECLARE_RESP` 不会回显这些项，
  需要确认时重新 `DECLARE` 即可拿到失败原因）。

**版本协商**：协议不做向后兼容。版本不是 `5`（或旧版布局根本无法解码）时，
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
    RequestID  uint32
    Flags      uint8      // bit0 = WantAck，bit1 = HasProperties
    Record     Record
    Properties stringmap  // 可选，仅 HasProperties=1
}

PUSH_OK {
    RequestID  uint32
    Flags      uint8      // bit0 = HasProperties
    Reason     uint8
    ID         PacketID
    Matched    uint32     // 入队成功的客户端数
    Dropped    uint32     // 未入队的客户端数
    Properties stringmap  // 可选，仅 HasProperties=1；错误时含 `error` 细节
}
```

- 每次 `PUSH` 只携带一条记录。
- `WantAck=0` 时服务端**不发送** `PUSH_OK`（发送即可，无需等待）。
- `WantAck=1` 时 `PUSH_OK` 始终回显该记录的 `ID` 与处理结果，成功时 `Reason=Success`。
- 触发发布限流时返回 `Reason=RateLimited`，`ID` 仍由服务端生成。
- 原因码：`Success`、`ChannelInvalid`、`NotAuthorized`、`PacketTooLarge`、
  `TargetNotFound`、`NoMatch`、`ConsumerBacklog`、`RateLimited`、`QuotaExceeded`、
  `Expired`。
- `PUSH.Properties` 可携带发布侧的附加信息，服务端在 `OnPush` 钩子中通过
  `PushContext.Properties` 读取；broker 不解释键值。
- `PUSH_OK.Properties` 在失败时提供 `error` 细节，客户端可直接用于日志。
- `PacketTooLarge` 是**记录级**结果：单条记录超过 `MaxRecordBytes`，或定向投递的
  唯一在线目标在 `CONNECT` 中声明的接收上限放不下该记录（此时不会入队）；
  帧级超限在读取阶段就终止连接，见 §1。
- 广播发布时，若某个**在线**目标声明的接收上限放不下该记录，该目标不会入队，
  计入 `Dropped`，不影响其他目标；若所有命中的目标都放不下，则返回
  `Reason=PacketTooLarge`。
- 离线会话没有可用的当前连接上限：**可靠**记录仍会入队，不可靠记录直接丢弃；
  如果重连后可靠记录仍放不下，会在 `PULL` 时为该客户端丢弃（见 §5.3）。

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
    RequestID  uint32
    Flags      uint8      // bit0 = HasProperties
    WaitMs     uint32     // 长轮询时长；0 = 立即返回
    MaxCount   uint16     // 本次最多返回的记录数；0 与 1 等价
    MaxBytes   uint32     // 本次响应中记录编码大小之和上限；0 = 不额外限制
    Properties stringmap  // 可选，仅 HasProperties=1
}

PULL_OK {
    RequestID  uint32
    Flags      uint8      // bit0 = HasProperties
    Reason     uint8
    Count      uint16     // Records 条数
    Pending    [3]uint32
    Records    []Record   // 0..Count 条
    Properties stringmap  // 可选，仅 HasProperties=1
}
```

服务端会把 `MaxCount` 裁剪到 `min(客户端请求, MaxPullBatch, 1024)`。
`MaxBytes` 只统计记录本身的编码大小，不包含包头；放不下下一条时会提前结束本批。

拉取顺序：

1. **窗口有空位时先取新消息**：按 Priority `High → Medium → Low`，
   同优先级内 `Reliable → Unreliable`、FIFO；
2. 可靠消息每取一条就占用一个窗口槽位，不可靠消息不占；
3. 窗口已满或没有新消息时，**重发** `delivered` 中未确认的可靠消息（`Dup=1`），
   同样受 `MaxCount` / `MaxBytes` 限制。

重发只发生在**没有新消息可投递**时，因此一次 `PULL_OK` 不会同时包含新消息与重发消息。
客户端应在处理完后批量 `COMMIT`，以尽快腾出窗口。

如果某条 pending / delivered 记录编码后超过客户端声明的接收上限（例如客户端
重连时调小了 `MaxPacketSize`），服务端会为该客户端丢弃这条记录并继续尝试下一条
（计入丢弃指标），而不是让它永久堵住队头；`PULL_OK` 本身不会因此返回
`PacketTooLarge`。

**同一连接同一时刻只允许一个在途 PULL。** 第二个并发的 `PULL` 会立刻收到
`PULL_OK(Reason=RateLimited)`，不返回任何记录；客户端应在约 1 秒后重试：
并发拉取会让同一条消息被重复投递，也会让客户端的未确认集合互相覆盖。

队列为空的处理完全由 `WaitMs` 决定：`0` 立即返回空结果，`>0` 最多等待该时长
（服务端按 `MaxWaitMs` 裁剪）。`PULL_OK.Count=0` 且 `Reason=Success` 表示超时，不是错误。

`Reason=CommitRequired` 表示客户端在存在未确认消息时反复拉取，已超过软上限；
此时 `Count=0`，客户端应先 `COMMIT`。

### 5.4 COMMIT / COMMIT_OK

```
COMMIT {
    RequestID uint32
    Count     uint16
    Entries   []{
        ID     PacketID
        Action uint8      // 0 = Commit，1 = Nack
        Reason uint8      // 仅 Nack 时有意义
    }
}

COMMIT_OK {
    RequestID uint32
    Committed uint16   // 成功确认的条数
    Nacked    uint16   // 成功拒绝的条数
    Unknown   uint16   // 不在 delivered 中的 ID（幂等空操作）
    Pending   [3]uint32
}
```

- `Action=Commit`：从 `delivered` 移除并释放对应窗口槽位，消息不再重发。
- `Action=Nack`：同样从 `delivered` 移除并释放窗口槽位，但**不再重发**；
  服务端会调用 `Hooks.Nack`（若配置），随后丢弃该消息并计入 `MessagesNacked`。
  `Reason` 供应用层死信/告警使用，协议本身不改变处理方式。
- 同一批中重复的 ID 按第一次出现处理，后续计入 `Unknown`。
- 对不存在或已确认的 ID 是幂等空操作，计入 `Unknown`。
- 不可靠消息永不进入 `delivered`，其 ID 提交后计为 `Unknown`。

批量 `COMMIT` 让客户端可以用一个 RTT 确认整批消息，把吞吐从「每条一个 RTT」
提升到「每窗口一个 RTT」。

### 5.5 DECLARE / DECLARE_RESP

```
DECLARE {
    RequestID  uint32
    Flags      uint8      // bit0 = HasProperties
    Count      uint16
    Channels   []{ Channel string, Flags uint8 }  // 全量替换；[] = 取消全部
    Properties stringmap  // 可选，仅 HasProperties=1
}

DECLARE_RESP {
    RequestID  uint32
    Flags      uint8      // bit0 = HasProperties
    Accepted   uint16     // 被接受的声明数量
    Count      uint16     // Results 条数（只含失败项）
    Results    []{ Channel string, Reason uint8 }
    Pending    [3]uint32
    Limits     Limits     // 与 CONNACK.Limits 相同布局
    Properties stringmap  // 可选，仅 HasProperties=1
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
DISCONNECT { ReasonCode uint8 }
```

`DISCONNECT` 双向：

- C→S：优雅关闭；`ReasonNormalDisconnect`。
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
| 投递语义 | 可靠消息 at-least-once：未 `COMMIT` 会重发（`Dup=1`）；`NACK` 后不再重发 |
| 发布重复 | 生产者重试会生成新的 `PacketID`；broker 不做发布去重，需要时由应用业务键 + 消费端幂等处理 |
| 离线队列 | 保留全部可靠消息；不可靠消息直接丢弃 |
| 顺序 | 按 Priority High→Medium→Low，同优先级内可靠先于不可靠、各自 FIFO |
| 窗口 | 未确认可靠消息 ≤ `ReceiveMaximum`；不可靠消息不占窗口 |
| 吞吐 | 一次 `PULL` 最多 `MaxPullBatch` 条，一次 `COMMIT` 可确认整批 |
| TTL | ExpiresAt 由发布者提供（Unix 毫秒）；过期消息不入队、不投递 |
| 历史回放 | 不支持（channel 是路由键，不存历史） |
| 重启 | 全部状态丢失，`SessionID` 变化 |
| 队列满 | 先淘汰不可靠，再按 Low→High 淘汰旧消息；新消息价值更低时丢弃新消息 |

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
| `ReceiveMaximum`（每会话未确认可靠消息窗口） | 32（可配 1–1024） |
| `MaxPullBatch`（单次 PULL 最多记录数） | 32（上限 1024） |
| `SessionTimeout`（离线会话保留时长） | 86400s（0 = 断开即清理） |
| `MaxSessions`（含离线保留） | 10000 |
| `MaxInflightBytes`（全局在途消息） | 256 MiB |
| `NoProgressSoft` / `Hard` | 4 / 16 |
| `UncommittedTimeout` | 关闭 |

除 `MaxClients`、`MaxHandshakeConnections`、`MaxSessions`、`MaxInflightBytes` 等全局
资源限制外，上表与连接/会话相关的值都会通过 `CONNACK.Limits` 返回；`DECLARE_RESP`
会再次返回同一组 `Limits`。
