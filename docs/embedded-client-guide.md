# 嵌入式客户端实现要点

面向用 C / Rust / 裸机等语言自行实现 Beacon v4 客户端的开发者。线格式的权威定义见
[`protocol-v4.md`](./protocol-v4.md)，这里只讲实现时容易出错的点。

## 1. 一个最小客户端需要实现什么

必需：

1. 固定头解析（1 字节类型 + 4 字节大端长度）
2. `CONNECT` 编码 / `CONNACK` 解析
3. `PULL` 编码 / `PULL_OK` 解析（含 Record）
4. `COMMIT` 编码 / `COMMIT_OK` 解析
5. `PUSH` 编码 / `PUSH_OK` 解析（只做消费者可以不实现）
6. `PING` / `PONG`

可选：`DECLARE`（改变关注的 channel）、`DISCONNECT`（优雅关闭）。

## 2. 握手顺序

```
客户端                                       服务端
  │ ── CONNECT ──────────────────────────────▶ │
  │ ◀───────────────────────────── CONNACK ── │
  │                                            │
  │ ── DECLARE（可选，也可放在 CONNECT 里）──▶ │
  │ ◀────────────────────────── DECLARE_RESP ─ │
  │                                            │
  │ ── PULL ─────────────────────────────────▶ │
  │ ◀────────────────────────────── PULL_OK ── │
  │ ── COMMIT ───────────────────────────────▶ │
  │ ◀───────────────────────────── COMMIT_OK ─ │
```

`CONNECT` 必须是第一个包。版本不是 `4` 时服务端回 `CONNACK(UnsupportedProtocol)` 并关闭。

## 3. Record 编解码

```
Flags      1 字节
ID         12 字节
Channel    2 字节长度 + 字节
Target     2 字节长度 + 字节（可为 0）
Source     2 字节长度 + 字节
Headers    2 字节条目数 + N × (2 字节 key 长度 + key + 2 字节 value 长度 + value)
Payload    4 字节长度 + 字节
```

注意：

- **`HasHeaders` 位与 Headers 必须一致**：条目数为 0 时不置位；非 0 时必须置位。
- 字符串用 `uint16` 长度，因此单条 channel / header 值上限 65535 字节；payload 用 `uint32`。
- `Headers` 的键在编码时排序，便于确定性输出；解码不要求有序。
- `Priority` 位为 0 且 `HasPriority` 未置位时，按 **Medium** 处理。

## 4. 必须遵守的消费循环

```c
for (;;) {
    pull(max_records, max_bytes, wait_ms);   // 阻塞直到有数据或超时
    for (each record in PULL_OK.records) {
        handle(record);
        collect(record.id);
    }
    commit(collect_ids);                     // 必须在下次 pull 之前
}
```

**关键点：`delivered` 非空时，`PULL` 只重发未确认的消息。**
如果没有 commit 就再次 pull，会收到一模一样的内容（`Dup=1`），并不会前进。
服务端会统计连续无进展的拉取次数，超过阈值会返回 `CommitRequired` 甚至断开连接。

因此：

- 不要「拉一批 → 慢慢处理 → 中途再拉」；
- 一次 pull 的 `MaxRecords` 应等于一轮能处理完并提交的量。

## 5. 优先级与确认

| 优先级 | 需要 commit | 说明 |
|---|---|---|
| Low (0) | 否 | 服务端写出后立即删除，可能丢失 |
| Medium (1) | 是 | 默认值 |
| High (2) | 是 | 队列满时最后被丢弃 |

Low 的消息 ID 不需要提交；提交了也没有副作用（服务端幂等忽略）。

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

服务端在 `1.5 × KeepAlive` 内没有收到任何包就会断开。因此：

- 空闲时每 `KeepAlive/2` 发一次 `PING`；
- 长轮询的 `PULL` 不算「已发送」，仍需要 `PING`。

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
QueueCap 条（pending）  +  MaxRecords 条（delivered）
每条约 24 字节索引 + 共享消息体
```

客户端侧只需保存当前一批未确认消息的 ID 列表。

## 10. 常见错误

| 现象 | 原因 |
|---|---|
| 反复收到同样的消息 | 没有 commit 就再次 pull |
| `PULL_OK.Reason = CommitRequired` | 连续无进展 pull 超过软上限 |
| 连接被服务端关闭且收到 `CommitRequired` | 超过硬上限 |
| `PUSH_OK` 全部 `NoMatch` | 没有客户端声明匹配的 channel |
| `PUSH_OK` 返回 `TargetNotFound` | 定向目标是 clientID，且该客户端没有会话 |
| `DECLARE_RESP` 返回 `NotAuthorized` | 声明的过滤器超出 ACL 允许范围 |
| 收不到任何消息 | 检查 `DECLARE` 是否真的成功，以及是否用了 `*` / `**` |
