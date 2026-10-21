# Beacon 嵌入式/资源受限客户端实现指导

> 本文档指导在**嵌入式设备、MCU、资源受限设备**（仅几百 KB 甚至几十 KB 内存）上实现 Beacon 客户端。
> 采用**分层递进**的方式：层级越低，功能越少、实现越简单；**层级 0 的发布/订阅核心机制是所有设备都必须实现的**。
> 存储与重传属于"可靠性增强"，可按资源情况选择实现或简化。

---

## 1. 总体思路

Beacon 是一个**精简的二进制协议**（协议名 `MX`，版本 2），固定头仅 5 字节，属性采用位掩码紧凑编码，比文本协议更适合资源受限场景。

但标准客户端（参考 `client/` 包）为满足**更强的**可靠投递，引入了：
- **持久化存储**（BadgerDB，内存占用大）
- **发送/接收队列**
- **超时重传 + 去重**
- **会话恢复、轮询缓冲**

这些在嵌入式设备上往往**不可承受**。本文档的要点：**协议本身很轻；QoS1 确认式收发也不依赖存储，是低成本的核心能力；真正重的是持久化、队列、复杂重传管理这些“增强设施”**。因此按“能否放弃某些增强设施”划分层级。

```
层级 0   ──►  层级 1   ──►  层级 2   ──►  层级 3
最小核心      +可靠性增强    +有限持久化     完整客户端
(必选:含QoS1) (推荐)        (可选)         (参考/对照)
```

**核心决策原则**：QoS1 确认式收发**不需要存储**——它只是“发后等 ACK、收到回 ACK”，重传由 broker 负责。因此**层级 0 就应实现 QoS1**（内存极小的设备同样需要可靠性，不能因为“小”就放弃）。持久化与复杂的重传管理才是可按资源裁剪的部分。

---

## 2. 协议线格式速查（实现者对照）

### 2.1 固定头（所有包）
```
[0]    Type    1 字节：包类型
[1-4]  Remaining  4 字节大端：后续包体长度
固定头 = 5 字节
```
包类型：`CONNECT=1, CONNACK=2, PUBLISH=3, PUBACK=4, SUBSCRIBE=5, SUBACK=6, UNSUBSCRIBE=7, UNSUBACK=8, PING=9, PONG=10, DISCONNECT=11, AUTH=12, TRACE=13`（以 `packet/constants.go` 为准）。

**字符串**：`[Len(2B 大端)][UTF-8 字节]`；**Id**：12 字节（见 §5.2）；**属性**：`[Bitmask(2B)]` + 按位序的属性值，无属性时仅写 2 字节 `0x0000`。

### 2.2 各包最小 Remaining（可据此快速拒绝畸形包）
| 包 | 最小 Remaining | 说明 |
|---|---|---|
| CONNECT | 17 | 协议名(2+2)+版本(1)+ClientID(2)+KeepAlive(2)+SessionTimeout(4)+SessionFlags(1)+属性(2) |
| CONNACK | 6 | SessionPresent(1)+ReasonCode(1)+属性(2)+固定 |
| PUBLISH | 19 | Flags(1)+Topic(2)+PacketID(12)+属性(2)+固定 |
| PUBACK | 12 | PacketID(12) |
| SUBSCRIBE | 19 | PacketID(12)+属性(2)+Topic(2)+Options(1)+固定 |
| SUBACK | 17 | PacketID(12)+属性(2)+ReasonCode(1) |
| UNSUBSCRIBE | 18 | PacketID(12)+属性(2)+Topic(2) |
| UNSUBACK | 17 | PacketID(12)+属性(2)+ReasonCode(1) |
| PING | 12 | Seq(4)+Timestamp(8) |
| PONG | 13 | Seq(4)+Echo(8)+Load(1) |
| DISCONNECT/AUTH | 0 | 可无包体 |

> 建议实现者**先校验 Remaining 是否大于本地缓冲区上限**再继续读取，防止被畸形包撑爆内存（参考标准实现中对包上限的溢出安全校验）。

### 2.3 关键包结构

**CONNECT**（客户端 → 服务端）
```
Type=CONNECT, Remaining=...
  ProtocolName: string("MX")
  ProtocolVersion: 2 (1B)
  ClientID: string
  KeepAlive: uint16（秒，0 或过大时服务端会用默认值并回写）
  SessionTimeout: uint32（秒；0 = 断开即清理会话）
  SessionFlags: 1B（bit0=CleanStart，其余保留）
  Properties: bitmask(2B) + 可选值（MaxPacketSize/ReceiveWindow/AuthMethod/AuthData/TraceID/UserProperties）
  WillFlags(1B): bit0=QoS, bit1=Retain, bit2=Dup, bit3=Will
  [Will]: WillTopic(string) + WillProperties + WillPayload（仅 bit3=1 时存在）
```

> `CleanStart=1` 表示要求服务端丢弃已有会话（订阅 + 待投递消息）并新建会话；为 0 时，
> 服务端在 `SessionTimeout>0` 且存在未过期会话的情况下恢复会话（CONNACK 的 `SessionPresent=1`）。

**CONNACK**（服务端 → 客户端）
```
  SessionPresent(1B)：是否恢复会话
  ReasonCode(1B)：0x00=成功
  Properties：可含 KeepAlive(覆盖值)、MaxPacketSize、ReceiveWindow、ClientID(服务端分配时) 等
```

**PUBLISH**（双向）
```
  Flags(1B)：bit0=QoS(0/1)，bit1=Retain，bit2=Dup(重发标志)
  Topic: string
  PacketID: 12B（QoS0/QoS1 都携带）
  Properties: bitmask(2B) + 可选值（ExpiryTime/ContentType/TraceID/TargetClientID/ResponseTopic/CorrelationData/UserProperties）
  Payload: 剩余全部字节
```

**SUBSCRIBE / UNSUBSCRIBE**
```
  PacketID(12B) + Properties(2B) + 主题表
  SUBSCRIBE 每条：Topic(string) + Options(1B)
  Options(1B)：bit0=QoS，bit1=NoLocal，bit2=RetainAsPublished，bit3-4=RetainHandling
  SUBACK 返回：ReasonCode×N（0x03=授予QoS0，0x04=授予QoS1）
```

**PING / PONG**（心跳）
```
  PING:  Seq(4B) + Timestamp(8B, Unix 纳秒)
  PONG:  Seq(4B, 回显) + Echo(8B, 回显时间戳) + Load(1B, 服务端负载)
```

---

## 3. 层级 0：最小客户端（必选核心）

> **目标**：内存极小的设备（8~32KB RAM），需要最基本的发布/订阅，以及**不依赖存储的 QoS1 确认式可靠性**。
> **定位**：任何 Beacon 设备都至少要达到此层级。

### 3.1 功能范围
| 必须实现 | 说明 |
|---|---|
| TCP 连接 + CONNECT/CONNACK | 建立会话的基础 |
| SUBSCRIBE/SUBACK、UNSUBSCRIBE | 订阅与退订 |
| PUBLISH（QoS0）收发 | 核心消息通道 |
| PUBLISH（QoS1）发送（确认式） | 发后等待 PUBACK；在途仅 1 条，存于 RAM |
| PUBLISH（QoS1）接收（自动确认） | 收到 QoS1 消息即回 PUBACK |
| PUBACK 解析 | 处理对端对已发 QoS1 的确认 |
| PING/PONG 心跳 | 保持连接、检测断线 |
| DISCONNECT | 正常断开 |
| 消息解析 + 事件回调 | 收到消息后立即处理（不缓冲或极小缓冲） |

### 3.2 明确不做（可简化项）
- ❌ **不做持久化存储**（无 flash 或不用 flash）——QoS1 可靠性靠“RAM 单条在途 + broker 重传”，不需要落盘
- ❌ **不做离线消息队列**（离线消息依赖服务端 `SessionTimeout` 缓冲，见层级 1）
- ❌ **不做去重表 / 复杂重传管理**（仅内存中保存当前在途 1 条；重传由 broker 主导）
- ❌ **不做会话恢复**（每次连接都是新会话；`SessionTimeout=0`）
- ❌ **不做手动确认**（处理完才 ACK，见层级 1；层级 0 用“收到即 ACK”）
- ❌ **不做 retain / will / 定向投递 / 请求-响应**（服务端这些能力对客户端透明，客户端只需能解析属性即可跳过）

### 3.3 实现要点
1. **固定缓冲、零动态分配**：用一块静态环形/双缓冲（如 1~4KB）接收并解析包；一条消息处理完再读下一条。Payload 过大时直接丢弃（或按 Topic 过滤）。
2. **逐字段解析 + 强校验**：读固定头 → 校验 Remaining ≤ 缓冲区 → 校验最小长度 → 解析。任何不满足直接断开，不要试图"容忍"畸形包。
3. **QoS1 确认式收发（无需存储）**：
   - 发送：发出 QoS1 后，把该消息**保留在 RAM 中**（同一时刻仅 1 条在途），等待 PUBACK；收到后释放该缓冲。超时（如 `KeepAlive` 或固定值）未确认则重发并置 `Dup=1`，最多 N 次（如 3 次）后丢弃并上报错误。
   - 接收：收到 QoS1 消息，**处理（或入极小缓冲）后立即回 PUBACK**。若不回（如断电/崩溃），broker 会重传——这就是 QoS1 的可靠性来源，设备端**不需要任何存储**。
4. **QoS0 即回调**：QoS0 收到即回调、不确认；若设备正在处理上一条可能丢消息，这是 QoS0 的可接受行为（尽力到达）。
5. **心跳**：以 CONNACK 回写的 KeepAlive 为准（若未回写则用 CONNECT 里发的值）；建议在 `1.5×KeepAlive` 内未收到任何包则判定断线；空闲时发 PING。
6. **属性解析**：多数设备可**跳过/忽略不认识或不需要的属性**（位掩码告诉你有哪些属性，逐个跳过其长度即可）。只需解析 `TargetClientID`（定向消息）与 `ExpiryTime`（过期）等少数几个。

### 3.4 最小交互时序
```
connect:   CONNECT ──► CONNACK
subscribe: SUBSCRIBE ──► SUBACK
publish0:  PUBLISH(QoS0)            （无需确认）
publish1:  PUBLISH(QoS1) ──► PUBACK （等待确认）
receive0:  ──► PUBLISH(QoS0)（回调处理）
receive1:  ──► PUBLISH(QoS1) ──► PUBACK（自动确认）
keepalive: PING ──► PONG
disconnect: DISCONNECT
```

---

## 4. 层级 1：+ 可靠性增强（推荐）

> **目标**：中等资源设备（32~96KB RAM），在层级 0 的 QoS1 基础上，进一步增强**连接级可靠性**。
> **核心增量**：断线重连 + 会话恢复 + 遗嘱 + 手动确认（处理完才 ACK）+ 更完善的重传管理。

### 4.1 新增功能
| 必须实现 | 说明 |
|---|---|
| 断线重连 | 检测到断线后按固定/指数退避重连，重连后恢复订阅 |
| 会话恢复（SessionTimeout>0） | 请求服务端保留会话；重连后 `SessionPresent=true` 时由服务端补发离线消息 |
| Will 遗嘱 | CONNECT 携带遗嘱，异常断开时由服务端发布 |
| 重传管理增强 | 重连后重发本地未确认消息；重试上限与退避策略 |
| 手动确认（可选） | QoS1 接收**处理完才 ACK**，需要应用显式确认；配合小型 PacketID 去重应对 broker 重传 |

### 4.2 简化策略（关键）
1. **离线消息依赖服务端**：利用 `SessionTimeout>0`，让服务端在设备离线时缓冲消息；设备重连（`SessionPresent=true`）后服务端会补发。设备端**仍不需要自己的持久化**。
2. **重连后恢复订阅**：重连成功（或 `SessionPresent=false`）后，重新下发内存中保存的订阅列表（若干主题即可）。
3. **手动确认（可选）**：默认仍用“收到即 ACK”；如需“处理完才 ACK”，则应用处理完显式回 ACK，并用一个很小的“最近已处理 PacketID”表（如 16 项）去重，应对 broker 重传。
4. **重传仍不落盘**：重传与重试都基于 RAM；重启即失，可靠性上限由服务端会话/离线缓冲兜底。

### 4.3 在途消息状态机（建议；层级 0 的 QoS1 发送亦适用）
```
IDLE ──发送QoS1──► WAIT_ACK ──收到PUBACK──► IDLE
                    │
                    ├──超时──► 重发(Dup=1)，重试计数+1
                    │            └─ 超过N次 ──► 丢弃 + 上报错误
                    └──连接断开──► 若启用会话保留：重连后补发；
                                  否则丢弃（重启即失）
```

---

## 5. 层级 2：+ 有限持久化与重传（可选增强）

> **目标**：资源较充足或带 flash 的设备（128KB~512KB RAM / 或外部 flash）。
> **核心增量**：把"在途 + 待发"消息持久化到非易失存储，实现断电续传。

### 5.1 新增功能
| 功能 | 说明 |
|---|---|
| 待发队列持久化 | 把待发送（含未确认）消息写入 flash，重启后可恢复 |
| 接收缓冲持久化 | 可选：收到的 QoS1 消息在应用处理前落 flash，处理后再删 |
| PacketID 去重 | 收到重传消息（同 PacketID + Dup=1）时去重，避免重复处理 |
| 会话恢复 + 补发 | 重连后先补发本地未确认消息，再处理服务端补发 |

### 5.2 简化策略
1. **限制队列深度**：如 16~64 条，超出则拒绝新发布（返回"队列满"）。用固定数组 + 序号，不用动态链表。
2. **简单存储格式**：每条记录 `[Len(2B)][CRC(2B)][序列化消息字节]`，线性追加 + 环形覆盖；启动时扫描恢复。
3. **序列化只需够用**：不必完整实现属性编解码器，只保存 Topic、Payload、PacketID、QoS、Dup、属性位掩码原文即可。
4. **去重表很小**：只需记住"最近 N 个已处理 PacketID"，而非完整历史。

---

## 6. 层级 3：完整客户端（对照参考）

> 标准实现：`client/` 包 + `core/` 服务端。具备：完整持久化（BadgerDB）、收发队列、超时重传、会话接管、轮询缓冲、手动确认（`AutoAck=false` + `Message.Ack()`）等。嵌入式设备一般**不需要**达到此层级，此处仅作对照与"能力上限"参考。

---

## 7. 层级对比总表

| 能力 | 层级0 | 层级1 | 层级2 | 层级3(参考) |
|---|---|---|---|---|
| 内存预算(RAM) | 8~32KB | 32~96KB | 128~512KB | 不限 |
| 连接 + CONNECT/CONNACK | ✅ | ✅ | ✅ | ✅ |
| SUBSCRIBE/UNSUBSCRIBE | ✅ | ✅ | ✅ | ✅ |
| PUBLISH QoS0 收发 | ✅ | ✅ | ✅ | ✅ |
| PUBLISH QoS1 收发 | ✅ | ✅ | ✅ | ✅ |
| PING/PONG 心跳 | ✅ | ✅ | ✅ | ✅ |
| 遗嘱 Will | ❌ | ✅ | ✅ | ✅ |
| 断线重连 | ❌ | ✅ | ✅ | ✅ |
| 在途单条重传 | ✅(RAM单条) | ✅(增强) | ✅ | ✅ |
| 持久化存储 | ❌ | ❌ | ✅(有限) | ✅(完整) |
| 离线消息缓冲 | 依赖服务端 | 依赖服务端 | 本端+服务端 | 本端完整 |
| PacketID 去重 | ❌ | ✅(若手动确认) | ✅(小表) | ✅ |
| 手动确认(处理后才ACK) | ❌ | ✅(可选) | ✅(可选) | ✅ |
| retain/定向/请求响应 | 透传可忽略 | 透传可忽略 | 可选 | ✅ |

---

## 8. 关键实现注意事项

### 8.1 QoS 语义与
- **QoS0**：尽力到达。服务端/完整客户端会持久化 QoS0 并尽力补发，但**无需 ACK**。嵌入式端把 QoS0 当作"发了就行"，是最省资源的通道。
- **QoS1**：保证“消息被处理”。**接收方确认后才算送达**；若不确认，broker 会重传。**实现 QoS1 不需要存储**——只需“发后等 ACK、收到回 ACK”，因此**层级 0 也应实现**（可靠性对极小设备同样重要）。“收到即 ACK”（自动确认）最省事；“处理完才 ACK”（手动确认，层级 1 可选）能更强地保证被应用处理，但需配合小型 PacketID 去重应对重传。
- 结论：**能少用 QoS1 就少用**；传感器上报这类高频数据用 QoS0，控制指令/关键事件才用 QoS1。层级 0 已具备 QoS1 能力，代价仅是少量 RAM 与一条在途状态。

### 8.2 PacketID 生成
- PacketID 是 **12 字节**（标准实现用 `nson.Id` = 6B 毫秒时间戳 + 2B 计数器 + 4B 随机）。
- 嵌入式端**不必**严格复刻该格式，但需满足：
  1. **全局唯一**（在同一会话内不重复即可；服务端以它做去重/确认依据）；
  2. **单调递增**更好（利于 FIFO 排序）。
- 简单做法：`[4B 启动随机数][4B 单调计数器][4B 时间戳]`，或直接 `[8B 计数器+4B 随机]`。
- 注意：**QoS0 消息也携带 PacketID**，不要省略（否则部分服务端逻辑会出问题）。

### 8.3 心跳与超时
- CONNECT 里发 `KeepAlive`（秒）；服务端可能回写覆盖值（CONNACK 属性）。
- 建议：客户端在 `KeepAlive` 秒内无数据可发时发 PING；若 `1.5×KeepAlive` 内未收到任何字节则判定断线。
- PING 用递增 Seq 关联 PONG，可用于 RTT 测量（可选）。

### 8.4 流控（ReceiveWindow / MaxPacketSize）
- CONNECT 属性可声明 `MaxPacketSize`（能接收的最大包，建议设备按自身缓冲设定，如 4KB/16KB）与 `ReceiveWindow`（接收窗口，服务端据此控制并发在途数）。
- 服务端 CONNACK 也会下发 `MaxPacketSize`/`ReceiveWindow` 作为发送上限，设备应遵守（否则可能被服务端丢弃或断开）。
- 层级1 设备的 `ReceiveWindow` 设为 1 即可（一次只处理一条，天然流控）。

### 8.5 属性（Properties）编码
- 所有属性集合都是 `[Bitmask(2B)]` + 按位序的值。**不认识/不需要的属性直接按类型长度跳过**（字符串=2B长度+内容、uint32=4B、uint16=2B、binary=2B长度+内容），不必完整实现。
- 嵌入式最值得解析的属性：PUBLISH 的 `TargetClientID`（判断是否本机定向消息）、`ExpiryTime`（过期丢弃）、CONNACK 的 `KeepAlive`/`MaxPacketSize`/`ReceiveWindow`。

### 8.6 主题与通配符
- 通配符：`*`（单层）、`**`（多层，须在末尾）。发布主题**不允许**通配符。
- 嵌入式只需发送/订阅普通字符串主题；通配符匹配是**服务端**的职责，客户端无需实现匹配逻辑。

### 8.7 内存优化建议（通用）
- 固定/静态缓冲，避免堆分配；单包解析器逐段消费输入缓冲。
- 大 Payload 分块读入，或按需丢弃（如只关心特定 Topic 时先解析 Topic 再决定是否读完整 Payload）。
- 用 `ReceiveWindow`/`MaxPacketSize` 主动告知服务端"我很小"，让服务端配合。
- 心跳用硬件/OS 定时器，避免忙等。

---

## 9. 推荐路线

1. **先做层级 0**：连上 broker、订阅主题、QoS0/QoS1 收发（QoS1 用 RAM 单条在途 + 自动确认）。验证链路与基本可靠性可用。
2. **加层级 1**：断线重连 + 会话恢复 + 遗嘱 +（可选）处理完才确认。覆盖绝大多数可靠性需求。
3. **按需加层级 2**：只有需要“断电不丢”时，才引入 flash 持久化与去重。
4. 层级 3 只作对照，一般不在 MCU 上实现。

> 核心原则：**协议解析与 QoS1 确认是固定成本，不可省；持久化与复杂重传管理才是可变成本，按资源裁剪。** 用层级 0 即可让大部分嵌入式设备以几十 KB 内存获得可靠的发布/订阅能力。
