// Package wsconn 提供 WebSocket 到 net.Conn 的适配层。
//
// beacon 的 core.HandleConn 与 client 拨号都基于 net.Conn 抽象，
// 通过本包将 WebSocket 消息流适配为字节流，即可让两端复用现有的
// 连接处理逻辑（读包、心跳、超时等），无需改动协议层。
package wsconn

import (
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Conn 将 WebSocket 连接包装为 net.Conn。
//
// WebSocket 是消息协议，net.Conn 是字节流协议，因此 Read 需要维护
// 内部缓冲：一次读入一个二进制消息，消费不完的部分留在缓冲中，
// 下次 Read 继续消费。Write 将每次调用作为一个二进制消息发出。
type Conn struct {
	ws *websocket.Conn

	// 读缓冲：当前未消费完的消息内容
	readBuf []byte

	// gorilla 内部对并发写有互斥保护，但 Close 与 Write 交错时
	// 仍可能产生竞态，这里再加一把写锁统一串行化
	writeMu sync.Mutex
}

// New 包装一个已建立的 WebSocket 连接
func New(ws *websocket.Conn) *Conn {
	return &Conn{ws: ws}
}

// Read 从 WebSocket 消息流中读取字节。
// 实现 io.Reader 语义：尽量填满 b，但允许返回少于 len(b) 的字节。
func (c *Conn) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}

	for len(c.readBuf) == 0 {
		msgType, data, err := c.ws.ReadMessage()
		if err != nil {
			return 0, err
		}
		// 只接受二进制消息；文本消息直接丢弃（协议是二进制的，
		// 但浏览器等客户端可能误发文本，丢弃比报错更宽容）
		if msgType != websocket.BinaryMessage {
			continue
		}
		c.readBuf = data
	}

	n := copy(b, c.readBuf)
	c.readBuf = c.readBuf[n:]
	return n, nil
}

// Write 将字节作为单个二进制消息发送。
func (c *Conn) Write(b []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.ws.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return 0, err
	}
	return len(b), nil
}

// Close 关闭 WebSocket 连接（发送关闭帧并关闭底层连接）。
//
// 这里不获取 writeMu：关闭必须能打断正在阻塞的写操作，
// 否则写超时/停机时会被卡在进行中的写里。gorilla/websocket 的
// Conn.Close 可以安全地与其它方法并发调用。
func (c *Conn) Close() error {
	return c.ws.Close()
}

// SetReadLimit 限制单个 WebSocket 消息的最大字节数。
// limit <= 0 表示不限制。
func (c *Conn) SetReadLimit(limit int64) {
	c.ws.SetReadLimit(limit)
}

// LocalAddr 返回本地地址
func (c *Conn) LocalAddr() net.Addr {
	return c.ws.LocalAddr()
}

// RemoteAddr 返回远端地址
func (c *Conn) RemoteAddr() net.Addr {
	return c.ws.RemoteAddr()
}

// SetDeadline 同时设置读写截止时间
func (c *Conn) SetDeadline(t time.Time) error {
	if err := c.ws.SetReadDeadline(t); err != nil {
		return err
	}
	return c.ws.SetWriteDeadline(t)
}

// SetReadDeadline 设置读取截止时间
func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.ws.SetReadDeadline(t)
}

// SetWriteDeadline 设置写入截止时间
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.ws.SetWriteDeadline(t)
}
