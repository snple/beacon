package core

import (
	"context"
	"sort"
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// ============================================================================
// 服务器信息
// ============================================================================

// Limits 是配置上限快照，供管理面计算使用率（当前值/上限）。
type Limits struct {
	MaxClients              uint32
	MaxHandshakeConnections uint32
	MaxPacketSize           uint32
	ReceiveWindow           uint16
	MaxClientQueueSize      int
	MaxPendingMessages      int
	MaxStoreBytes           int64
	MaxOfflineSessions      int
	MaxPublishRate          float64
	WriteTimeout            time.Duration
}

func (c *Core) limits() Limits {
	o := c.options
	return Limits{
		MaxClients:              o.MaxClients,
		MaxHandshakeConnections: o.MaxHandshakeConnections,
		MaxPacketSize:           o.MaxPacketSize,
		ReceiveWindow:           o.ReceiveWindow,
		MaxClientQueueSize:      o.MaxClientQueueSize,
		MaxPendingMessages:      o.MaxPendingMessages,
		MaxStoreBytes:           o.MaxStoreBytes,
		MaxOfflineSessions:      o.MaxOfflineSessions,
		MaxPublishRate:          o.MaxPublishRate,
		WriteTimeout:            o.WriteTimeout,
	}
}

// ListenerInfo 描述一个正在服务的监听器。
type ListenerInfo struct {
	Address  string // 实际监听地址
	Protocol string // "tcp" / "tls" / "ws" / "metrics" / "custom"
	Active   bool
}

// StoreInfo 描述消息存储状态。
type StoreInfo struct {
	Path  string // 空字符串表示内存模式
	Ready bool
	Bytes int64
}

// ServerInfo 是进程级信息快照。
//
// 构建信息来自构造期注入的 CoreOptions.BuildInfo（WithBuildInfo），
// 运行期不可变，因此读取无需加锁。
type ServerInfo struct {
	Version   string
	Commit    string
	BuildTime string
	Protocol  string // "MX/2"
	StartedAt time.Time
	Uptime    time.Duration
	Listeners []ListenerInfo
	Limits    Limits
	Store     StoreInfo
}

// Info 返回服务器信息快照。
//
// StartedAt 是 Core 的创建时间（宿主通常在进程启动时创建），构造后只读，
// 因此这里不加锁。
func (c *Core) Info() ServerInfo {
	info := ServerInfo{
		Version:   c.options.BuildInfo.Version,
		Commit:    c.options.BuildInfo.Commit,
		BuildTime: c.options.BuildInfo.BuildTime,
		StartedAt: c.startedAt,
	}

	info.Protocol = "MX/2"
	if !info.StartedAt.IsZero() {
		info.Uptime = time.Since(info.StartedAt)
	}
	info.Listeners = c.Listeners()
	info.Limits = c.limits()
	info.Store = StoreInfo{
		Path:  c.options.StoreOptions.DataDir,
		Ready: c.Ready(),
		Bytes: c.storeBytes(),
	}

	return info
}

// Ready 报告 broker 是否已启动且存储可用（可用于就绪探针）。
func (c *Core) Ready() bool {
	if !c.running.Load() || c.stopped.Load() {
		return false
	}
	return c.store != nil
}

// ============================================================================
// 监听器注册表
// ============================================================================

type listenerEntry struct {
	address  string
	protocol string
}

func (c *Core) registerListener(address, protocol string) uint64 {
	c.listenersMu.Lock()
	defer c.listenersMu.Unlock()

	c.listenerSeq++
	id := c.listenerSeq
	c.listeners[id] = listenerEntry{address: address, protocol: protocol}
	return id
}

func (c *Core) unregisterListener(id uint64) {
	c.listenersMu.Lock()
	delete(c.listeners, id)
	c.listenersMu.Unlock()
}

// Listeners 返回当前所有监听器（按注册顺序）。
func (c *Core) Listeners() []ListenerInfo {
	c.listenersMu.Lock()
	defer c.listenersMu.Unlock()

	out := make([]ListenerInfo, 0, len(c.listeners))
	for id := uint64(1); id <= c.listenerSeq; id++ {
		entry, ok := c.listeners[id]
		if !ok {
			continue
		}
		out = append(out, ListenerInfo{Address: entry.address, Protocol: entry.protocol, Active: true})
	}
	return out
}

func connAddress(conn *conn) string {
	if conn == nil || conn.conn == nil {
		return ""
	}
	return conn.conn.RemoteAddr().String()
}

// offlineExpiry 返回离线会话的过期时间（不在离线表中时返回零值）。
func (c *Core) offlineExpiry(clientID string) time.Time {
	c.offlineSessionsMu.Lock()
	defer c.offlineSessionsMu.Unlock()

	if expiry, ok := c.offlineSessions[clientID]; ok {
		return expiry
	}
	return time.Time{}
}

// ============================================================================
// 客户端列表（轻量）
// ============================================================================

// ClientSummary 是客户端列表项：只含概要字段，不读取队列、订阅等内部状态。
// 需要配置、运行状态与统计时用 GetClientInfo（按单个客户端读取，代价可控）。
type ClientSummary struct {
	ClientID    string
	Connected   bool      // 在线（false 表示保留中的离线会话）
	RemoteAddr  string    // 离线时为空
	ConnectedAt time.Time // 最近一次建立连接的时间（进程重启后恢复的离线会话为零值）
	LastSeenAt  time.Time // 最后一次活动时间（连接、收包、断开时更新）
	ExpiresAt   time.Time // 离线会话的过期时间；在线时为零值
}

// ListClients 列出所有客户端（含保留中的离线会话），按 ClientID 字典序。
//
// 不做分页：客户端总量受 MaxClients 约束，列表只组装概要字段（无存储读取）；
// 需要更多内容请对单个客户端调用 GetClientInfo。
func (c *Core) ListClients() []ClientSummary {
	c.clientsMu.RLock()
	out := make([]ClientSummary, 0, len(c.clients))
	for _, client := range c.clients {
		conn := client.getConn()
		online := conn != nil && !conn.closed.Load()

		summary := ClientSummary{
			ClientID:    client.ID,
			Connected:   online,
			ConnectedAt: client.ConnectedAt(),
			LastSeenAt:  client.lastSeenAt(),
		}
		if online {
			summary.RemoteAddr = connAddress(conn)
			summary.LastSeenAt = conn.lastActiveAt()
		} else {
			summary.ExpiresAt = c.offlineExpiry(client.ID)
		}
		out = append(out, summary)
	}
	c.clientsMu.RUnlock()

	sort.Slice(out, func(i, j int) bool { return out[i].ClientID < out[j].ClientID })
	return out
}

// ============================================================================
// 客户端详情
// ============================================================================

// GetClientInfo 获取单个客户端的详细信息：配置、运行状态、队列/在途水位与计数器。
func (c *Core) GetClientInfo(clientID string) (*ClientInfo, error) {
	c.clientsMu.RLock()
	client, exists := c.clients[clientID]
	if exists {
		info := c.buildClientInfo(client)
		c.clientsMu.RUnlock()
		return info, nil
	}
	c.clientsMu.RUnlock()

	return nil, NewClientNotFoundError(clientID)
}

// buildClientInfo 组装 ClientInfo。调用方必须持有 clientsMu（读锁即可），
// 以保证同一快照内的字段来自同一时刻，不出现"在线但无连接"的撕裂视图。
func (c *Core) buildClientInfo(client *Client) *ClientInfo {
	client.session.subsMu.RLock()
	subs := make([]ClientSubscriptionInfo, 0, len(client.session.subscriptions))
	for topic, opts := range client.session.subscriptions {
		subs = append(subs, ClientSubscriptionInfo{
			Topic:            topic,
			SubscribeOptions: opts,
		})
	}
	client.session.subsMu.RUnlock()
	sort.Slice(subs, func(i, j int) bool { return subs[i].Topic < subs[j].Topic })

	client.session.pendingAckMu.Lock()
	pendingAck := len(client.session.pendingAck)
	var oldestAck time.Time
	for _, p := range client.session.pendingAck {
		if p.lastSentAt <= 0 {
			continue
		}
		sentAt := time.Unix(p.lastSentAt, 0)
		if oldestAck.IsZero() || sentAt.Before(oldestAck) {
			oldestAck = sentAt
		}
	}
	client.session.pendingAckMu.Unlock()

	conn := client.getConn()
	online := conn != nil && !conn.closed.Load()

	info := &ClientInfo{
		ClientID:       client.ID,
		Connected:      online,
		ConnectedAt:    client.ConnectedAt(),
		KeepAlive:      client.KeepAlive(),
		SessionTimeout: client.SessionTimeout(),
		Subscriptions:  subs,
		PendingAck:     pendingAck,
		Identity:       client.session.getIdentity().Clone(),
		WillTopic:      client.willTopic(),

		MessagesSent:     client.messagesSent.Load(),
		MessagesDropped:  client.messagesDropped.Load(),
		MessagesRejected: client.messagesRejected.Load(),
		BytesSent:        client.bytesSent.Load(),
		BytesReceived:    client.bytesReceived.Load(),
		LastSeenAt:       client.lastSeenAt(),
	}

	if !oldestAck.IsZero() {
		info.OldestPendingAckAge = time.Since(oldestAck)
	}

	if client.queue != nil {
		info.QueueSize = client.queue.Count()
		info.QueueBytes = client.queue.Bytes()
		if msg, err := client.queue.Peek(); err == nil && msg != nil && msg.Timestamp > 0 {
			info.OldestQueuedAge = time.Since(time.Unix(msg.Timestamp, 0))
		}
	}

	if online {
		info.RemoteAddr = connAddress(conn)
		info.ReceiveWindow = conn.sendWindow
		info.Transport = conn.transport
		info.ProtocolVersion = conn.protocol
		info.LastSeenAt = conn.lastActiveAt()
		info.ExpiresAt = time.Time{}
	} else {
		info.ExpiresAt = c.offlineExpiry(client.ID)
	}

	return info
}

// DisconnectClientAndWait 断开指定客户端并等待连接与发送协程退出。
//
// 与 DisconnectClient 的区别：本方法在连接关闭后才返回，便于管理面立即刷新状态；
// 等待受 ctx 约束，ctx 到期返回 ctx.Err()。
func (c *Core) DisconnectClientAndWait(ctx context.Context, clientID string, reason packet.ReasonCode) error {
	if ctx == nil {
		ctx = context.Background()
	}

	c.clientsMu.RLock()
	client, exists := c.clients[clientID]
	c.clientsMu.RUnlock()

	if !exists {
		return NewClientNotFoundError(clientID)
	}

	conn := client.getConn()
	if conn == nil || conn.closed.Load() {
		return nil
	}

	client.Close(reason)

	done := make(chan struct{})
	go func() {
		conn.wait()
		close(done)
	}()

	select {
	case <-done:
		c.logger.Info("Client disconnected by admin",
			zap.String("clientID", clientID), zap.Uint8("reason", uint8(reason)))
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
