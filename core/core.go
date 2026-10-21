package core

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// Core 是 Beacon 消息总线的核心：管理连接、会话、订阅与持久化消息队列。
type Core struct {
	options *CoreOptions
	logger  *zap.Logger

	// 客户端管理（在线 + 离线会话）
	clients   map[string]*Client
	clientsMu sync.RWMutex

	// 离线会话过期管理
	offlineSessions   map[string]time.Time // clientID -> 过期时间
	offlineSessionsMu sync.Mutex

	// 订阅管理
	subTree *subTree

	// 消息持久化存储
	store *store

	// 消息队列
	queue        *Queue
	queueTrigger chan struct{}

	// 保留消息
	retainStore *retainStore

	// 统计信息
	stats Stats

	// /metrics 与 /stats 的快照缓存（GetStats 需要遍历所有客户端）
	statsCacheMu sync.Mutex
	statsCache   StatsSnapshot
	statsCacheAt time.Time

	// 连接速率限制
	connLimiter *tokenBucket

	// 存储占用水位缓存（避免每条消息都统计 badger 大小）
	storeSizeMu sync.Mutex
	storeSize   int64
	storeSizeAt time.Time

	// 生命周期
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running atomic.Bool

	// 临时任务（如按需重传）的启动守卫：Stop 之后禁止再向 wg 添加任务，
	// 否则会与 wg.Wait 竞争，且任务可能在 store 关闭后访问它。
	taskMu      sync.Mutex
	tasksClosed bool
}

// spawnTask 启动受 wg 跟踪的后台任务。core 已停止时返回 false。
func (c *Core) spawnTask(fn func()) bool {
	c.taskMu.Lock()
	defer c.taskMu.Unlock()
	if c.tasksClosed {
		return false
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		fn()
	}()
	return true
}

// Stats core 统计信息
type Stats struct {
	ClientsConnected atomic.Int64
	// ClientsTotal 累计新建的会话数（单调递增，包含已清理的历史会话）。
	ClientsTotal       atomic.Int64
	MessagesReceived   atomic.Int64
	MessagesSent       atomic.Int64
	MessagesDropped    atomic.Int64 // 投递侧丢弃（过期、客户端队列配额溢出、订阅端拒收等）
	MessagesRejected   atomic.Int64 // 接收侧拒绝（发布速率超限、全局队列/存储配额超限、钩子拒绝等）
	BytesReceived      atomic.Int64
	BytesSent          atomic.Int64
	SubscriptionsCount atomic.Int64
}

// NewWithOptions 使用 Builder 模式创建新的 core（推荐）
//
// 用法：
//
//	b, err := core.NewWithOptions(
//	    core.NewCoreOptions().
//	        WithAddress(":3883").
//	        WithMaxClients(10000).
//	        WithRetainEnabled(true),
//	)
func NewWithOptions(opts *CoreOptions) (*Core, error) {
	if opts == nil {
		opts = NewCoreOptions()
	}
	opts.applyDefaults()

	// 验证配置
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	b := &Core{
		options:         opts,
		logger:          opts.Logger,
		clients:         make(map[string]*Client),
		offlineSessions: make(map[string]time.Time),
		subTree:         newSubTree(),
		queueTrigger:    make(chan struct{}, 1),
		retainStore:     newRetainStore(),
		ctx:             ctx,
		cancel:          cancel,
	}

	// 初始化连接速率限制器
	if opts.ConnectionRateLimit > 0 {
		b.connLimiter = newTokenBucket(opts.ConnectionRateLimit, opts.ConnectionRateBurst)
	}

	// 初始化消息持久化存储（必须先于 initQueue）
	if err := b.initStore(); err != nil {
		return nil, err
	}

	// 初始化消息队列（使用 InMemory badger）
	if err := b.initQueue(); err != nil {
		return nil, err
	}

	return b, nil
}

// New 创建新的 Core（使用默认配置）
func New() (*Core, error) {
	return NewWithOptions(NewCoreOptions())
}

// initStore 初始化消息存储
func (c *Core) initStore() error {
	// 确保 Logger 正确传递
	if c.options.StoreOptions.Logger == nil {
		c.options.StoreOptions.Logger = c.options.Logger
	}

	store, err := newStore(c.options.StoreOptions)
	if err != nil {
		return fmt.Errorf("failed to initialize message store: %w", err)
	}
	c.store = store
	return nil
}

// initQueue 初始化消息队列
func (c *Core) initQueue() error {
	c.queue = NewQueue(c.store.db(), "core:")

	c.logger.Info("Message queue initialized (InMemory)")
	return nil
}

// Start 启动 core
func (c *Core) Start() error {
	if c.running.Load() {
		return ErrCoreAlreadyRunning
	}

	c.running.Store(true)

	c.taskMu.Lock()
	c.tasksClosed = false
	c.taskMu.Unlock()

	// 恢复持久化的消息与会话（订阅/过期时间），并清理孤儿队列
	c.recoverMessages()
	c.recoverSessions()

	// 启动后台协程
	c.startBackgroundWorkers()

	// 上次停机时可能仍有未分发的积压消息（已持久化在全局队列中）。
	// 必须主动触发一次排空：dispatchLoop 只在发布时被唤醒，
	// 否则积压要等下一次发布才会被处理，甚至在此期间因过期被丢弃。
	if c.queue != nil {
		if depth := c.queue.Count(); depth > 0 {
			c.logger.Info("Dispatching persisted backlog", zap.Int64("pending", depth))
			c.triggerDispatch()
		}
	}

	c.logStartupInfo()
	return nil
}

// startBackgroundWorkers 启动后台工作协程
func (c *Core) startBackgroundWorkers() {
	// 启动消息分发协程
	c.wg.Add(1)
	go c.dispatchLoop()

	// 启动过期消息清理协程
	if c.options.ExpiredCheckInterval > 0 {
		c.wg.Add(1)
		go c.expiredCleanupLoop()
	}

	// 启动消息重传协程
	if c.options.RetransmitInterval > 0 {
		c.wg.Add(1)
		go c.retransmitLoop()
	}
}

// logStartupInfo 记录启动信息
func (c *Core) logStartupInfo() {
	c.logger.Info("core started",
		zap.Uint32("maxClients", c.options.MaxClients))
}

// Stop 停止 core
func (c *Core) Stop() error {
	if !c.running.Load() {
		return nil
	}

	c.logger.Info("Stopping core...")
	c.running.Store(false)

	// 先禁止新的临时任务，再 cancel：保证 Stop 期间的 wg.Wait 不会与 wg.Add 竞争
	c.taskMu.Lock()
	c.tasksClosed = true
	c.taskMu.Unlock()

	c.cancel()

	// 断开所有普通客户端
	c.clientsMu.Lock()
	clients := make([]*Client, 0, len(c.clients))
	for _, client := range c.clients {
		clients = append(clients, client)
	}
	c.clientsMu.Unlock()

	// 关闭每个客户端的连接，并在锁外等待其发送协程退出。
	// 注意：不能持有 clientsMu 等待——发送协程可能通过用户 OnDeliver 钩子回调 core API。
	// 停机路径不逐个发送 DISCONNECT：避免慢连接让 N 个客户端各等一个写超时。
	for _, client := range clients {
		client.closeLocal(packet.ReasonServerShuttingDown)
		if conn := client.getConn(); conn != nil {
			conn.wait()
		}
	}

	// 等待所有协程退出
	c.wg.Wait()

	// 最后关闭消息存储
	if err := c.store.close(); err != nil {
		c.logger.Warn("Failed to close message store", zap.Error(err))
	}

	c.logger.Info("core stopped")
	return nil
}

// MaxPacketSize 返回服务端允许的最大包大小。
func (c *Core) MaxPacketSize() uint32 {
	return c.options.MaxPacketSize
}

// Subscribe 订阅主题
func (c *Core) Subscribe(clientID string, sub packet.Subscription) {
	// 统一通配符规则：** 只能作为最后一层。非法过滤器直接忽略，
	// 避免通过编程接口绕过 SUBSCRIBE 校验后与匹配语义不一致。
	if !packet.ValidateTopicFilter(sub.Topic) {
		c.logger.Warn("Ignoring subscription with invalid topic filter",
			zap.String("clientID", clientID),
			zap.String("topic", sub.Topic))
		return
	}

	isNew := c.subTree.subscribe(clientID, sub.Topic, sub.Options)
	if isNew {
		c.stats.SubscriptionsCount.Add(1)
	}
}

// Unsubscribe 取消订阅
func (c *Core) Unsubscribe(clientID, topic string) {
	if removed := c.subTree.unsubscribe(clientID, topic); removed {
		c.stats.SubscriptionsCount.Add(-1)
	}
}

// GetRetainedMessages 获取与主题模式匹配的保留消息
// 从 messageStore 中按需加载消息内容
func (c *Core) GetRetainedMessages(topic string) []*Message {
	// 获取匹配的主题列表
	topics := c.retainStore.matchTopics(topic)
	if len(topics) == 0 {
		return nil
	}

	messages := make([]*Message, 0, len(topics))
	for _, t := range topics {
		msg, err := c.store.getRetain(t)
		if err != nil {
			c.logger.Debug("Failed to get retain message from store",
				zap.String("topic", t),
				zap.Error(err))
			continue
		}
		if msg != nil {
			messages = append(messages, msg)
		}
	}
	return messages
}

// GetStats 获取统计信息快照
func (c *Core) GetStats() StatsSnapshot {
	snapshot := StatsSnapshot{
		ClientsConnected:   c.stats.ClientsConnected.Load(),
		ClientsTotal:       c.stats.ClientsTotal.Load(),
		MessagesReceived:   c.stats.MessagesReceived.Load(),
		MessagesSent:       c.stats.MessagesSent.Load(),
		MessagesDropped:    c.stats.MessagesDropped.Load(),
		MessagesRejected:   c.stats.MessagesRejected.Load(),
		BytesReceived:      c.stats.BytesReceived.Load(),
		BytesSent:          c.stats.BytesSent.Load(),
		SubscriptionsCount: c.stats.SubscriptionsCount.Load(),
		StoreBytes:         c.storeBytes(),
	}

	// 队列深度只在有队列时统计（NewWithOptions 之前/测试场景可能为 nil）
	if c.queue != nil {
		snapshot.PendingQueueDepth = c.queue.Count()
	}

	c.offlineSessionsMu.Lock()
	snapshot.OfflineSessions = int64(len(c.offlineSessions))
	c.offlineSessionsMu.Unlock()

	c.clientsMu.RLock()
	for _, client := range c.clients {
		if client.queue != nil {
			snapshot.ClientQueueDepth += client.queue.Count()
		}
		client.session.pendingAckMu.Lock()
		snapshot.PendingAckTotal += int64(len(client.session.pendingAck))
		client.session.pendingAckMu.Unlock()
	}
	c.clientsMu.RUnlock()

	return snapshot
}

// StatsSnapshot 统计信息快照 (可安全复制)
type StatsSnapshot struct {
	ClientsConnected   int64
	ClientsTotal       int64 // 累计新建的会话数（单调递增）
	MessagesReceived   int64
	MessagesSent       int64
	MessagesDropped    int64
	MessagesRejected   int64
	BytesReceived      int64
	BytesSent          int64
	SubscriptionsCount int64

	// 队列与存储水位（用于容量规划与告警）
	PendingQueueDepth int64 // 全局待分发队列深度
	ClientQueueDepth  int64 // 所有客户端待发送队列深度之和
	PendingAckTotal   int64 // 所有客户端未确认（QoS1）消息总数
	OfflineSessions   int64 // 保留中（在线时间之外）的离线会话数
	StoreBytes        int64 // 存储占用（字节）
}

// storeBytes 返回存储当前占用的近似字节数。
// 结果缓存 1 秒，避免高频发布时每条消息都统计 badger 大小。
func (c *Core) storeBytes() int64 {
	if c.store == nil {
		return 0
	}

	c.storeSizeMu.Lock()
	defer c.storeSizeMu.Unlock()

	if !c.storeSizeAt.IsZero() && time.Since(c.storeSizeAt) < time.Second {
		return c.storeSize
	}

	lsm, vlog := c.store.db().Size()
	c.storeSize = lsm + vlog
	c.storeSizeAt = time.Now()
	return c.storeSize
}

// ============================================================================
// 管理 API
// ============================================================================

// ClientInfo 客户端信息
type ClientInfo struct {
	ClientID       string
	RemoteAddr     string
	Connected      bool
	ConnectedAt    time.Time
	KeepAlive      uint16
	SessionTimeout uint32
	Subscriptions  []ClientSubscriptionInfo
	PendingAck     int
	ReceiveWindow  uint16
	QueueSize      int64 // 待发送队列中的消息条数
}

// ClientSubscriptionInfo 客户端订阅信息（不含 ClientID）
type ClientSubscriptionInfo struct {
	Topic string
	packet.SubscribeOptions
}

// GetClientInfo 获取客户端详细信息
func (c *Core) GetClientInfo(clientID string) (*ClientInfo, error) {
	c.clientsMu.RLock()
	client, exists := c.clients[clientID]
	c.clientsMu.RUnlock()

	if !exists {
		return nil, NewClientNotFoundError(clientID)
	}

	// 获取订阅信息
	client.session.subsMu.RLock()
	subs := make([]ClientSubscriptionInfo, 0, len(client.session.subscriptions))
	for topic, opts := range client.session.subscriptions {
		subs = append(subs, ClientSubscriptionInfo{
			Topic:            topic,
			SubscribeOptions: opts,
		})
	}
	client.session.subsMu.RUnlock()

	// 获取 ACK 信息
	client.session.pendingAckMu.Lock()
	pendingAck := len(client.session.pendingAck)
	client.session.pendingAckMu.Unlock()

	conn := client.getConn()

	info := &ClientInfo{
		ClientID:       client.ID,
		Connected:      !client.Closed(),
		ConnectedAt:    client.ConnectedAt(),
		KeepAlive:      client.KeepAlive(),
		SessionTimeout: client.SessionTimeout(),
		Subscriptions:  subs,
		PendingAck:     pendingAck,
	}
	if client.queue != nil {
		info.QueueSize = client.queue.Count()
	}

	// conn 可能为 nil（例如离线但会话保留的客户端），需要防御性处理，避免 nil 解引用
	if conn != nil && conn.conn != nil {
		info.RemoteAddr = conn.conn.RemoteAddr().String()
		info.ReceiveWindow = conn.sendWindow
	}

	return info, nil
}

// ListClients 列出所有客户端
func (c *Core) ListClients() []*ClientInfo {
	c.clientsMu.RLock()
	clientIDs := make([]string, 0, len(c.clients))
	for id := range c.clients {
		clientIDs = append(clientIDs, id)
	}
	c.clientsMu.RUnlock()

	infos := make([]*ClientInfo, 0, len(clientIDs))
	for _, id := range clientIDs {
		if info, err := c.GetClientInfo(id); err == nil {
			infos = append(infos, info)
		}
	}

	return infos
}

// IsClientOnline 检查客户端是否在线
func (c *Core) IsClientOnline(clientID string) bool {
	c.clientsMu.RLock()
	client, exists := c.clients[clientID]
	c.clientsMu.RUnlock()

	if exists && !client.Closed() {
		return true
	}

	return false
}

// DisconnectClient 断开指定客户端连接
func (c *Core) DisconnectClient(clientID string, reason packet.ReasonCode) error {
	c.clientsMu.RLock()
	client, exists := c.clients[clientID]
	c.clientsMu.RUnlock()

	if !exists {
		return NewClientNotFoundError(clientID)
	}

	client.Close(reason)
	c.logger.Info("Client disconnected by admin", zap.String("clientID", clientID), zap.Uint8("reason", uint8(reason)))
	return nil
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

	return subs, nil
}

// GetTopicSubscribers 获取订阅指定主题的所有客户端
func (c *Core) GetTopicSubscribers(topic string) map[string]packet.SubscribeOptions {
	return c.subTree.matchTopic(topic)
}

// GetOnlineClientsCount 获取在线客户端数量
func (c *Core) GetOnlineClientsCount() int {
	return int(c.stats.ClientsConnected.Load())
}

// GetTotalClientsCount 获取总客户端数（包括离线会话）
func (c *Core) GetTotalClientsCount() int {
	c.clientsMu.RLock()
	count := len(c.clients)
	c.clientsMu.RUnlock()
	return count
}

// GetLogger 获取当前使用的 Logger
func (c *Core) GetLogger() *zap.Logger {
	return c.logger
}
