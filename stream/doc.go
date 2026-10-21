// Package stream 实现 Beacon 之外的另一套“点对点流中继”协议。
//
// Beacon（core/client）是发布-订阅消息总线；stream 则是独立的字节流协议：
// 客户端通过 stream.Core 中继，与目标客户端建立双向流。
// 两者的线格式、包类型与语义互不兼容，也不共享 packet 包。
//
// 角色：
//   - Core：中继服务，负责认证、等待配对与双向转发；
//   - Client：主动方 OpenStream / 被动方 AcceptStream；
//   - Stream：实现 net.Conn 的字节流。
//
// 等待连接上限与停机行为见 WithMaxWaitingConns、WithWriteTimeout。
package stream
