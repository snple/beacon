// Package stream 提供 core stream 功能的客户端数据类型。
//
// stream 是可选能力：客户端在连接后通过主连接声明支持，服务端可按需开启
// （默认关闭）。协商成功后，双方通过 core 的独立 stream 端口建立一条
// 点对点 TCP 通道；通道建立后只做字节转发，不经过消息队列。
//
// 服务端的协商与转发逻辑在 core 包，客户端 API 在 client 包。
// 数据面握手包定义在 stream/packet。
package stream
