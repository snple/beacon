package core

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/danclive/nson-go"
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

// SetBuildInfo 由宿主注入构建信息（版本号默认 "devel"）。
func (c *Core) SetBuildInfo(version, commit, buildTime string) {
	c.buildMu.Lock()
	defer c.buildMu.Unlock()

	if version != "" {
		c.buildVer = version
	}
	c.buildCom = commit
	c.buildTime = buildTime
}

// Info 返回服务器信息快照。
func (c *Core) Info() ServerInfo {
	c.buildMu.RLock()
	startedAt := c.startedAt
	info := ServerInfo{
		Version:   c.buildVer,
		Commit:    c.buildCom,
		BuildTime: c.buildTime,
		StartedAt: startedAt,
	}
	c.buildMu.RUnlock()

	info.Protocol = "MX/2"
	if !startedAt.IsZero() {
		info.Uptime = time.Since(startedAt)
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

// ============================================================================
// 分页
// ============================================================================

const (
	defaultPageLimit = 100
	maxPageLimit     = 1000
)

// PageOptions 是分页/过滤参数。零值表示"第一页、默认条数、不过滤"。
type PageOptions struct {
	Limit  int    // 每页条数（默认 100，上限 1000）
	Cursor string // 上一页返回的 NextCursor
	Prefix string // 按 clientID / topic 前缀过滤
	Online *bool  // 仅客户端列表/会话列表：true=在线，false=离线
}

// Page 是分页结果。
type Page struct {
	NextCursor string // 空表示没有下一页
	Total      *int64 // nil 表示未计算（全量计数代价高，默认不返回）
}

func (o PageOptions) normalized() PageOptions {
	if o.Limit <= 0 {
		o.Limit = defaultPageLimit
	}
	if o.Limit > maxPageLimit {
		o.Limit = maxPageLimit
	}
	return o
}

// pageKeys 在已排序的 key 列表中截取一页（字典序游标）。
func pageKeys(keys []string, o PageOptions) ([]string, Page) {
	start := 0
	if o.Cursor != "" {
		start = sort.SearchStrings(keys, o.Cursor)
		// Cursor 指向上一页最后一个 key：跳过它本身。
		if start < len(keys) && keys[start] == o.Cursor {
			start++
		}
	}

	end := start + o.Limit
	if end > len(keys) {
		end = len(keys)
	}

	page := Page{}
	if end < len(keys) {
		page.NextCursor = keys[end-1]
	}
	return keys[start:end], page
}

// ============================================================================
// 客户端 / 会话查询
// ============================================================================

// GetClientInfo 获取客户端详细信息
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
// 以保证同一页内的字段来自同一时刻，不出现"在线但无连接"的撕裂视图。
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

// ListClients 列出所有客户端（含离线会话）。
//
// 大客户端量场景请使用 ListClientsPage：本方法一次性返回全部数据，
// 且每页内不保证一致性。
func (c *Core) ListClients() []*ClientInfo {
	c.clientsMu.RLock()
	infos := make([]*ClientInfo, 0, len(c.clients))
	for _, client := range c.clients {
		infos = append(infos, c.buildClientInfo(client))
	}
	c.clientsMu.RUnlock()

	sort.Slice(infos, func(i, j int) bool { return infos[i].ClientID < infos[j].ClientID })
	return infos
}

// ListClientsPage 分页列出客户端（含离线会话），按 ClientID 字典序。
//
// 整页在 clientsMu 读锁内完成快照，保证页内字段一致。
func (c *Core) ListClientsPage(o PageOptions) ([]*ClientInfo, Page, error) {
	o = o.normalized()

	c.clientsMu.RLock()
	defer c.clientsMu.RUnlock()

	keys := make([]string, 0, len(c.clients))
	for id, client := range c.clients {
		if !matchPrefix(id, o.Prefix) {
			continue
		}
		if o.Online != nil && client.Closed() == *o.Online {
			continue
		}
		keys = append(keys, id)
	}
	sort.Strings(keys)

	pageKeys, page := pageKeys(keys, o)
	infos := make([]*ClientInfo, 0, len(pageKeys))
	for _, id := range pageKeys {
		infos = append(infos, c.buildClientInfo(c.clients[id]))
	}

	return infos, page, nil
}

// SessionInfo 描述一个会话（在线或离线）。
type SessionInfo struct {
	ClientID      string
	Online        bool
	ExpiresAt     time.Time // 在线时为零值
	QueueSize     int64
	QueueBytes    int64
	PendingAck    int
	Subscriptions []ClientSubscriptionInfo
	Identity      *Identity
	LastSeenAt    time.Time
}

// ListSessions 分页列出会话（在线 + 保留中的离线会话）。
func (c *Core) ListSessions(o PageOptions) ([]SessionInfo, Page, error) {
	o = o.normalized()

	c.clientsMu.RLock()
	defer c.clientsMu.RUnlock()

	keys := make([]string, 0, len(c.clients))
	for id, client := range c.clients {
		if !matchPrefix(id, o.Prefix) {
			continue
		}
		if o.Online != nil && client.Closed() == *o.Online {
			continue
		}
		keys = append(keys, id)
	}
	sort.Strings(keys)

	pageKeys, page := pageKeys(keys, o)
	out := make([]SessionInfo, 0, len(pageKeys))
	for _, id := range pageKeys {
		client := c.clients[id]
		info := c.buildClientInfo(client)
		out = append(out, SessionInfo{
			ClientID:      info.ClientID,
			Online:        info.Connected,
			ExpiresAt:     info.ExpiresAt,
			QueueSize:     info.QueueSize,
			QueueBytes:    info.QueueBytes,
			PendingAck:    info.PendingAck,
			Subscriptions: info.Subscriptions,
			Identity:      info.Identity,
			LastSeenAt:    info.LastSeenAt,
		})
	}

	return out, page, nil
}

// GetClientSubscriptions 获取客户端的订阅列表
func (c *Core) GetClientSubscriptions(clientID string) ([]ClientSubscriptionInfo, error) {
	c.clientsMu.RLock()
	client, exists := c.clients[clientID]
	c.clientsMu.RUnlock()

	if !exists {
		return nil, NewClientNotFoundError(clientID)
	}

	client.session.subsMu.RLock()
	defer client.session.subsMu.RUnlock()

	subs := make([]ClientSubscriptionInfo, 0, len(client.session.subscriptions))
	for topic, opts := range client.session.subscriptions {
		subs = append(subs, ClientSubscriptionInfo{
			Topic:            topic,
			SubscribeOptions: opts,
		})
	}
	sort.Slice(subs, func(i, j int) bool { return subs[i].Topic < subs[j].Topic })

	return subs, nil
}

// GetTopicSubscribers 获取订阅指定主题的所有客户端
func (c *Core) GetTopicSubscribers(topic string) map[string]packet.SubscribeOptions {
	return c.subTree.matchTopic(topic)
}

// ============================================================================
// 订阅 / 主题
// ============================================================================

// SubscriptionEntry 是一条订阅记录。
type SubscriptionEntry struct {
	ClientID string
	Topic    string
	packet.SubscribeOptions
}

// TopicEntry 是一个主题（订阅过滤器）的聚合信息。
type TopicEntry struct {
	Topic           string
	SubscriberCount int
	Retained        bool
}

// ListSubscriptions 分页列出所有订阅（按 topic、clientID 字典序）。
func (c *Core) ListSubscriptions(o PageOptions) ([]SubscriptionEntry, Page, error) {
	o = o.normalized()

	all := c.subTree.snapshotSubscriptions()

	keys := make([]string, 0, len(all))
	byKey := make(map[string]SubscriptionEntry, len(all))
	for _, entry := range all {
		if !matchPrefix(entry.Topic, o.Prefix) {
			continue
		}
		key := entry.Topic + "\x00" + entry.ClientID
		keys = append(keys, key)
		byKey[key] = entry
	}
	sort.Strings(keys)

	pageKeys, page := pageKeys(keys, o)
	out := make([]SubscriptionEntry, 0, len(pageKeys))
	for _, k := range pageKeys {
		out = append(out, byKey[k])
	}

	return out, page, nil
}

// ListTopics 分页列出主题：订阅过滤器与保留消息主题的并集。
func (c *Core) ListTopics(o PageOptions) ([]TopicEntry, Page, error) {
	o = o.normalized()

	counts := make(map[string]int)
	for _, entry := range c.subTree.snapshotSubscriptions() {
		counts[entry.Topic]++
	}
	for _, entry := range c.retainStore.snapshot() {
		if _, ok := counts[entry.topic]; !ok {
			counts[entry.topic] = 0
		}
	}

	keys := make([]string, 0, len(counts))
	for topic := range counts {
		if !matchPrefix(topic, o.Prefix) {
			continue
		}
		keys = append(keys, topic)
	}
	sort.Strings(keys)

	pageKeys, page := pageKeys(keys, o)
	out := make([]TopicEntry, 0, len(pageKeys))
	for _, topic := range pageKeys {
		out = append(out, TopicEntry{
			Topic:           topic,
			SubscriberCount: counts[topic],
			Retained:        c.retainStore.exists(topic),
		})
	}

	return out, page, nil
}

// ============================================================================
// 保留消息
// ============================================================================

// RetainedInfo 是保留消息的元数据（不含负载）。
type RetainedInfo struct {
	Topic       string
	PayloadSize int
	QoS         packet.QoS
	ExpiresAt   int64 // 0 = 不过期
	UpdatedAt   time.Time
}

// ListRetainedPage 分页列出保留消息元数据，不读取消息负载。
func (c *Core) ListRetainedPage(o PageOptions) ([]RetainedInfo, Page, error) {
	o = o.normalized()

	entries := c.retainStore.snapshot()

	keys := make([]string, 0, len(entries))
	byTopic := make(map[string]retainEntry, len(entries))
	for _, entry := range entries {
		if !matchPrefix(entry.topic, o.Prefix) {
			continue
		}
		keys = append(keys, entry.topic)
		byTopic[entry.topic] = entry
	}
	sort.Strings(keys)

	pageKeys, page := pageKeys(keys, o)
	out := make([]RetainedInfo, 0, len(pageKeys))
	for _, topic := range pageKeys {
		entry := byTopic[topic]
		out = append(out, RetainedInfo{
			Topic:       entry.topic,
			PayloadSize: entry.payloadSize,
			QoS:         entry.qos,
			ExpiresAt:   entry.expiryTime,
			UpdatedAt:   unixOrZero(entry.updatedAt),
		})
	}

	return out, page, nil
}

// ============================================================================
// 在途消息
// ============================================================================

// PendingAckInfo 是一条在途（已发送未确认）的 QoS1 消息。
type PendingAckInfo struct {
	PacketID nson.Id
	Topic    string
	QoS      packet.QoS
	SentAt   time.Time
	Age      time.Duration
	Retries  int
}

// ListPendingAck 列出某客户端在途未确认的消息（排障"哪条卡住"）。
//
// 数量受客户端接收窗口约束（有界），可安全枚举。
func (c *Core) ListPendingAck(clientID string) ([]PendingAckInfo, error) {
	c.clientsMu.RLock()
	client, exists := c.clients[clientID]
	c.clientsMu.RUnlock()

	if !exists {
		return nil, NewClientNotFoundError(clientID)
	}

	now := time.Now()

	client.session.pendingAckMu.Lock()
	out := make([]PendingAckInfo, 0, len(client.session.pendingAck))
	for id, pending := range client.session.pendingAck {
		info := PendingAckInfo{
			PacketID: id,
			Retries:  pending.retries,
		}
		if pending.msg != nil && pending.msg.Packet != nil {
			info.Topic = pending.msg.Packet.Topic
			info.QoS = pending.msg.QoS
		}
		if pending.lastSentAt > 0 {
			info.SentAt = time.Unix(pending.lastSentAt, 0)
			info.Age = now.Sub(info.SentAt)
		}
		out = append(out, info)
	}
	client.session.pendingAckMu.Unlock()

	sort.Slice(out, func(i, j int) bool { return out[i].SentAt.Before(out[j].SentAt) })
	return out, nil
}

// ============================================================================
// 辅助
// ============================================================================

func matchPrefix(s, prefix string) bool {
	return prefix == "" || strings.HasPrefix(s, prefix)
}

func unixOrZero(sec int64) time.Time {
	if sec <= 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
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
