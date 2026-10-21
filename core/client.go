package core

import (
	"fmt"
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

	// 消息队列
	queue *Queue

	// 会话层（跨连接持久化的状态）
	session *session

	// 发布速率限制（每客户端，跨连接保留）
	publishLimiter *tokenBucket

	// 连接层（当前 TCP 连接）
	conn   *conn
	connMu sync.RWMutex

	// 管理面计数器（按客户端聚合）
	messagesSent     atomic.Int64
	messagesDropped  atomic.Int64
	messagesRejected atomic.Int64
	bytesSent        atomic.Int64
	bytesReceived    atomic.Int64

	// lastSeen 最后可见时间（unix 纳秒）：连接建立、收到包、断开时更新
	lastSeen atomic.Int64

	// counted 表示该客户端已计入 ClientsConnected，保证减计数恰好一次
	counted atomic.Bool

	// adminDeleted 表示会话已被管理面删除，断开处理不应再复活会话或发布遗嘱
	adminDeleted atomic.Bool

	// Core 引用
	core *Core
}

// newClient 创建新的客户端（全新会话）
func newClient(netConn net.Conn, connect *packet.ConnectPacket, core *Core) *Client {
	clientID := connect.ClientID

	// 创建会话
	session := newSession(clientID, connect, core)

	// 创建客户端
	c := &Client{
		ID:      clientID,
		session: session,
		core:    core,
	}

	// 每客户端发布速率限制（令牌桶）；0 表示不限制
	if rate := core.options.MaxPublishRate; rate > 0 {
		c.publishLimiter = newTokenBucket(rate, core.options.MaxPublishBurst)
	}

	// 初始化消息队列
	c.initQueue()

	// 附加连接
	c.attachConn(netConn, connect)

	// 提取遗嘱消息
	c.extractWillMessage(connect)

	c.touch()

	return c
}

// initQueue 初始化消息队列（NewQueue 不会失败）。
func (c *Client) initQueue() {
	c.queue = NewQueue(c.core.store.db(), fmt.Sprintf("client:%s:", c.ID))

	c.core.logger.Info("Message queue initialized")
}

// attachConn 附加新的网络连接到客户端
// 用于新连接创建或会话恢复时替换连接
func (c *Client) attachConn(netConn net.Conn, connect *packet.ConnectPacket) {
	conn := newConn(c, netConn, connect)

	c.connMu.Lock()
	// 替换连接
	c.conn = conn
	c.connMu.Unlock()
}

func (c *Client) getConn() *conn {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.conn
}

// serve 开始处理客户端消息（委托给 Conn）
// 返回实际服务的连接实例，便于调用方在连接被接管时避免清理新连接对应的会话。
func (c *Client) serve() *conn {
	conn := c.getConn()

	if conn != nil {
		conn.serve()
	}

	return conn
}

// writePacket 发送数据包（委托给 Conn）
func (c *Client) writePacket(pkt packet.Packet) error {
	conn := c.getConn()

	if conn == nil || conn.closed.Load() {
		return ErrClientClosed
	}

	return conn.writePacket(pkt)
}

// Close 关闭客户端连接
func (c *Client) Close(reason packet.ReasonCode) {
	conn := c.getConn()

	if conn != nil {
		conn.close(reason)
	}
}

// closeLocal 关闭客户端连接但不发送 DISCONNECT（用于服务端停机）。
func (c *Client) closeLocal(reason packet.ReasonCode) {
	conn := c.getConn()

	if conn != nil {
		conn.closeLocal(reason)
	}
}

// Closed 检查客户端连接是否已关闭。
func (c *Client) Closed() bool {
	conn := c.getConn()

	if conn == nil {
		return true
	}
	return conn.closed.Load()
}

// SessionTimeout 返回会话过期时间
func (c *Client) SessionTimeout() uint32 {
	return c.session.timeout.Load()
}

// WillPacket 返回遗嘱消息
func (c *Client) WillPacket() *packet.PublishPacket {
	c.session.willMu.Lock()
	defer c.session.willMu.Unlock()
	return c.session.willPacket
}

// applyConnect 用最新的 CONNECT 更新会话参数（会话恢复时调用）。
func (c *Client) applyConnect(connect *packet.ConnectPacket) {
	c.session.applyConnect(connect, c.core)
	// 遗嘱属于单次连接：先清空，再按本次 CONNECT 重新设置
	c.extractWillMessage(connect)
}

// SetWillPacket 设置遗嘱消息
func (c *Client) SetWillPacket(will *packet.PublishPacket) {
	c.setWillPacket(will)
}

// KeepAlive 返回心跳间隔
func (c *Client) KeepAlive() uint16 {
	conn := c.getConn()

	if conn != nil {
		return conn.keepAlive
	}
	return 0
}

// TraceID 返回跟踪 ID
func (c *Client) TraceID() string {
	conn := c.getConn()

	if conn != nil {
		return conn.traceID
	}
	return ""
}

// ConnectedAt 返回连接建立时间
func (c *Client) ConnectedAt() time.Time {
	conn := c.getConn()

	if conn != nil {
		return conn.connectedAt
	}
	return time.Time{}
}

// DisconnectPacket 返回断开连接包
func (c *Client) DisconnectPacket() *packet.DisconnectPacket {
	conn := c.getConn()

	if conn != nil {
		return conn.disconnectPacket
	}
	return nil
}

// touch 记录最后一次可见时间。
func (c *Client) touch() {
	c.lastSeen.Store(time.Now().UnixNano())
}

// lastSeenAt 返回最后一次可见时间（从未可见时返回零值）。
func (c *Client) lastSeenAt() time.Time {
	ns := c.lastSeen.Load()
	if ns <= 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}

// willTopic 返回当前遗嘱的主题（无遗嘱时为空）。
func (c *Client) willTopic() string {
	c.session.willMu.Lock()
	defer c.session.willMu.Unlock()

	if c.session.willPacket == nil {
		return ""
	}
	return c.session.willPacket.Topic
}
