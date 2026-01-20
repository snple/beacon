package core

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snple/beacon/packet"
)

// 客户端相关常量
const (
	// 超时相关
	keepAliveMultiplier     = 1.5 // KeepAlive * 1.5 作为读取超时
	defaultKeepAliveTimeout = 60 * time.Second

	// 重传相关
	overflowBatchSize = 100 // 从持久化加载的每批最大条数
)

// Client 表示一个已连接的客户端（组合 Session 和 Conn）
// 提供统一的对外接口，内部委托给 Session 和 Conn 处理
type Client struct {
	// 客户端标识
	ID string

	// 会话层（跨连接持久化的状态）
	session *session

	// 连接层（当前 TCP 连接）
	conn    *conn
	oldConn *conn // 会话恢复时暂存旧连接，用于迁移 sendQueue
	connMu  sync.RWMutex

	// Core 引用
	core *Core

	// 其他状态
	skipHandle atomic.Bool // true: 跳过断开处理（会话被接管时）
}

// newClient 创建新的客户端（全新会话）
func newClient(netConn net.Conn, connect *packet.ConnectPacket, core *Core) *Client {
	// 创建会话
	session := newSession(connect.ClientID, connect, core)

	// 创建客户端
	c := &Client{
		ID:      connect.ClientID,
		session: session,
		core:    core,
	}

	// 附加连接
	c.attachConn(netConn, connect)

	// 提取遗嘱消息
	c.extractWillMessage(connect)

	return c
}

// attachConn 附加新的网络连接到客户端
// 用于新连接创建或会话恢复时替换连接
func (c *Client) attachConn(netConn net.Conn, connect *packet.ConnectPacket) {
	conn := newConn(c, netConn, connect)

	c.connMu.Lock()
	// 保存旧连接引用，用于迁移 sendQueue
	if c.conn != nil {
		c.oldConn = c.conn
	}
	c.conn = conn
	// 重置跳过断开处理标志
	c.skipHandle.Store(false)
	c.connMu.Unlock()
}

// serve 开始处理客户端消息（委托给 Conn）
func (c *Client) serve() {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn != nil {
		conn.serve()
	}
}

// writePacket 发送数据包（委托给 Conn）
func (c *Client) writePacket(pkt packet.Packet) error {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil || conn.closed.Load() {
		return errors.New("client not connected")
	}

	return conn.writePacket(pkt)
}

// Close 关闭客户端连接
func (c *Client) Close(reason packet.ReasonCode) {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn != nil {
		conn.close(reason)
	}
}

// closeAndSkipHandle 关闭客户端连接且跳过断开处理
// 用于会话被接管时
func (c *Client) closeAndSkipHandle(reason packet.ReasonCode) {
	c.skipHandle.Store(true)

	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn != nil {
		conn.close(reason)
	}
}

// IsClosed 检查客户端是否已关闭
func (c *Client) Closed() bool {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil {
		return true
	}
	return conn.closed.Load()
}

// KeepSession 返回是否保持会话
func (c *Client) KeepSession() bool {
	return c.session.keep
}

// SessionTimeout 返回会话过期时间
func (c *Client) SessionTimeout() uint32 {
	return c.session.timeout
}

// WillPacket 返回遗嘱消息
func (c *Client) WillPacket() *packet.PublishPacket {
	return c.session.willPacket
}

// SetWillPacket 设置遗嘱消息
func (c *Client) SetWillPacket(will *packet.PublishPacket) {
	c.session.willPacket = will
}

// KeepAlive 返回心跳间隔
func (c *Client) KeepAlive() uint16 {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn != nil {
		return conn.keepAlive
	}
	return 0
}

// TraceID 返回跟踪 ID
func (c *Client) TraceID() string {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn != nil {
		return conn.traceID
	}
	return ""
}

// ConnectedAt 返回连接建立时间
func (c *Client) ConnectedAt() time.Time {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn != nil {
		return conn.connectedAt
	}
	return time.Time{}
}

// DisconnectPacket 返回断开连接包
func (c *Client) DisconnectPacket() *packet.DisconnectPacket {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn != nil {
		return conn.disconnectPacket
	}
	return nil
}
