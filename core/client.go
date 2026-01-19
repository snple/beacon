package core

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/snple/beacon/packet"
)

// 客户端相关常量
const (
	// 超时相关
	keepAliveMultiplier     = 1.5 // KeepAlive * 1.5 作为读取超时
	defaultKeepAliveTimeout = 60 * time.Second

	// PacketID 相关
	minPacketID = 1
	maxPacketID = 65535

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
	conn    *Conn
	oldConn *Conn // 会话恢复时暂存旧连接，用于迁移 sendQueue
	connMu  sync.RWMutex

	// Core 引用
	core *Core

	// 生命周期
	closeOnce sync.Once
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
	// 重置 closeOnce，允许新连接关闭
	c.closeOnce = sync.Once{}
	c.connMu.Unlock()
}

// Serve 开始处理客户端消息（委托给 Conn）
func (c *Client) Serve() {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn != nil {
		// 处理持久化消息
		if !c.session.keep {
			// KeepSession=false: 清空该客户端在 BadgerDB 中的旧消息
			c.core.cleanupClient(c.ID)
		}

		conn.Serve()
	}
}

// WritePacket 发送数据包（委托给 Conn）
func (c *Client) WritePacket(pkt packet.Packet) error {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil || conn.IsClosed() {
		return errors.New("client not connected")
	}

	return conn.writePacket(pkt)
}

// Close 关闭客户端连接
// cleanup: true 表示执行清理逻辑，false 表示跳过清理（会话被接管时）
func (c *Client) Close(reason packet.ReasonCode, cleanup bool) {
	c.closeOnce.Do(func() {
		c.connMu.RLock()
		conn := c.conn
		c.connMu.RUnlock()

		if conn != nil {
			conn.close(reason, cleanup)
		}
	})
}

// IsClosed 检查客户端是否已关闭
func (c *Client) IsClosed() bool {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil {
		return true
	}
	return conn.IsClosed()
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
		return conn.KeepAlive
	}
	return 0
}

// TraceID 返回跟踪 ID
func (c *Client) TraceID() string {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn != nil {
		return conn.TraceID
	}
	return ""
}

// ConnectedAt 返回连接建立时间
func (c *Client) ConnectedAt() time.Time {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn != nil {
		return conn.ConnectedAt
	}
	return time.Time{}
}

// DisconnectPacket 返回断开连接包
func (c *Client) DisconnectPacket() *packet.DisconnectPacket {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn != nil {
		return conn.DisconnectPacket
	}
	return nil
}

// allocatePacketID 分配新的 PacketID
func (c *Client) allocatePacketID() uint16 {
	return c.session.allocatePacketID()
}

// triggerSend 触发发送协程（委托给 Conn）
func (c *Client) triggerSend() {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn != nil && !conn.IsClosed() {
		conn.triggerSend()
	}
}
