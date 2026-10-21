package core

import (
	"context"
	"fmt"
	"net"
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

	// 事件总线（管理面 SubscribeEvents）
	events *eventBus

	// 监听器注册表（管理面 Info）
	listenersMu sync.Mutex
	listeners   map[uint64]listenerEntry
	listenerSeq uint64

	// 构建信息与启动时间（管理面 Info）
	buildMu   sync.RWMutex
	buildVer  string
	buildCom  string
	buildTime string
	startedAt time.Time

	// 连接速率限制
	connLimiter *tokenBucket

	// 握手阶段的并发上限：认证前的连接还没有进入 MaxClients 统计，
	// 必须单独设限，否则未认证连接可以无上限地占用 goroutine 与包缓冲。
	handshakeSlots chan struct{}

	// 握手阶段的连接：Stop 时主动关闭，避免优雅停机等待 ConnectTimeout。
	handshakeConns   map[net.Conn]struct{}
	handshakeConnsMu sync.Mutex

	// 存储占用水位缓存（避免每条消息都统计 badger 大小）
	storeSizeMu sync.Mutex
	storeSize   int64
	storeSizeAt time.Time

	// 生命周期
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running atomic.Bool
	stopped atomic.Bool

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
		handshakeConns:  make(map[net.Conn]struct{}),
		events:          newEventBus(),
		listeners:       make(map[uint64]listenerEntry),
		buildVer:        "devel",
		ctx:             ctx,
		cancel:          cancel,
	}

	if opts.MaxHandshakeConnections > 0 {
		b.handshakeSlots = make(chan struct{}, opts.MaxHandshakeConnections)
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
	// Stop 之后不可重启：ctx 已取消、存储已关闭，重启只会得到一个"看起来在运行
	// 但无法收发消息"的实例。需要重启请重新 NewWithOptions。
	if c.stopped.Load() {
		return ErrCoreStopped
	}
	if !c.running.CompareAndSwap(false, true) {
		return ErrCoreAlreadyRunning
	}
	// 与并发的 Stop 竞争：Stop 可能刚把 stopped 置位，这里复查并回滚。
	if c.stopped.Load() {
		c.running.Store(false)
		return ErrCoreStopped
	}

	c.buildMu.Lock()
	c.startedAt = time.Now()
	c.buildMu.Unlock()

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
	// 幂等：重复 Stop 直接返回；同时标记 stopped，禁止同一实例再次 Start。
	if c.stopped.Swap(true) {
		return nil
	}

	wasRunning := c.running.Swap(false)
	if wasRunning {
		c.logger.Info("Stopping core...")

		// 先禁止新的临时任务，再 cancel：保证 Stop 期间的 wg.Wait 不会与 wg.Add 竞争
		c.taskMu.Lock()
		c.tasksClosed = true
		c.taskMu.Unlock()

		c.cancel()

		// 关闭仍在握手的连接：它们还没注册进 clients，wg.Wait 会被 ConnectTimeout 拖住。
		c.closeHandshakeConns()

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
	}

	// 最后关闭消息存储。未 Start 就 Stop 时同样要关：否则 badger 句柄与文件锁会泄漏。
	if c.store != nil {
		if err := c.store.close(); err != nil {
			c.logger.Warn("Failed to close message store", zap.Error(err))
		}
	}

	if wasRunning {
		c.logger.Info("core stopped")
	}
	return nil
}

// MaxPacketSize 返回服务端允许的最大包大小。
func (c *Core) MaxPacketSize() uint32 {
	return c.options.MaxPacketSize
}

// Subscribe 订阅主题（编程接口）。
//
// 若 clientID 已存在会话（在线或离线），订阅会同时写入会话状态并持久化，
// 与客户端自己发送 SUBSCRIBE 的效果一致；没有会话时只影响全局订阅树。
func (c *Core) Subscribe(clientID string, sub packet.Subscription) {
	// 统一通配符规则：** 只能作为最后一层。非法过滤器直接忽略，
	// 避免通过编程接口绕过 SUBSCRIBE 校验后与匹配语义不一致。
	if !c.addSubscription(clientID, sub) {
		return
	}

	c.clientsMu.RLock()
	client := c.clients[clientID]
	c.clientsMu.RUnlock()
	if client == nil || client.session == nil {
		return
	}

	client.session.subsMu.Lock()
	client.session.subscriptions[sub.Topic] = sub.Options
	client.session.subsMu.Unlock()

	c.saveSessionPreservingExpiry(client)

	c.emit(Event{
		Type:     EventSubscribed,
		ClientID: clientID,
		Topic:    sub.Topic,
		QoS:      sub.Options.QoS,
		Extra:    map[string]string{"source": "api"},
	})
}

// Unsubscribe 取消订阅（编程接口）。
//
// 与 Subscribe 对称：存在会话时同步移除会话状态并持久化。
func (c *Core) Unsubscribe(clientID, topic string) {
	c.removeSubscription(clientID, topic)

	c.clientsMu.RLock()
	client := c.clients[clientID]
	c.clientsMu.RUnlock()
	if client == nil || client.session == nil {
		return
	}

	client.session.subsMu.Lock()
	delete(client.session.subscriptions, topic)
	client.session.subsMu.Unlock()

	c.saveSessionPreservingExpiry(client)

	c.emit(Event{
		Type:     EventUnsubscribed,
		ClientID: clientID,
		Topic:    topic,
		Extra:    map[string]string{"source": "api"},
	})
}

// removeSubscription 只更新全局订阅树与统计。
func (c *Core) removeSubscription(clientID, topic string) {
	if removed := c.subTree.unsubscribe(clientID, topic); removed {
		c.stats.SubscriptionsCount.Add(-1)
	}
}

// addSubscription 只更新全局订阅树与统计，返回订阅是否被接受。
//
// SUBSCRIBE 处理路径与启动恢复使用它：会话状态的维护与持久化由调用方
// 负责（订阅路径是批量写一次，恢复路径不需要写），避免每主题一次 store 写入。
func (c *Core) addSubscription(clientID string, sub packet.Subscription) bool {
	if !packet.ValidateTopicFilter(sub.Topic) {
		c.logger.Warn("Ignoring subscription with invalid topic filter",
			zap.String("clientID", clientID),
			zap.String("topic", sub.Topic))
		return false
	}
	if !sub.Options.ValidRetainHandling() {
		c.logger.Warn("Ignoring subscription with invalid retain handling",
			zap.String("clientID", clientID),
			zap.String("topic", sub.Topic),
			zap.Uint8("retainHandling", sub.Options.RetainHandling))
		return false
	}

	if isNew := c.subTree.subscribe(clientID, sub.Topic, sub.Options); isNew {
		c.stats.SubscriptionsCount.Add(1)
	}
	return true
}

// saveSessionPreservingExpiry 持久化会话元数据；离线会话保留原有过期时间，
// 避免编程接口写回时把离线会话误标为在线（expiresAt=0）。
func (c *Core) saveSessionPreservingExpiry(client *Client) {
	expiresAt := int64(0)
	if client.Closed() {
		c.offlineSessionsMu.Lock()
		expiry, ok := c.offlineSessions[client.ID]
		c.offlineSessionsMu.Unlock()
		if ok {
			expiresAt = expiry.Unix()
		}
	}
	c.saveSession(client, expiresAt)
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
	c.buildMu.RLock()
	startedAt := c.startedAt
	c.buildMu.RUnlock()

	snapshot := StatsSnapshot{
		StartedAt:          startedAt,
		Limits:             c.limits(),
		EventsDropped:      c.events.dropped.Load(),
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
	StartedAt time.Time // 进程启动时间
	Limits    Limits    // 配置上限，用于计算使用率
	// EventsDropped 是因订阅者消费过慢而被丢弃的事件数
	EventsDropped int64

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

// ClientInfo 客户端信息。
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

	// 会话与连接
	ExpiresAt       time.Time // 离线会话过期时间（在线时为零值）
	LastSeenAt      time.Time // 最后一次可见/活动时间
	Transport       string    // "tcp" / "tls" / "ws"
	ProtocolVersion string    // 例如 "MX/2"
	Identity        *Identity // 认证器给出的身份（可能为 nil）
	WillTopic       string    // 遗嘱主题（无遗嘱时为空）

	// 积压
	QueueSize           int64         // 待发送队列消息条数
	QueueBytes          int64         // 待发送队列编码后字节数（近似）
	OldestQueuedAge     time.Duration // 队列中最老消息的存在时长
	OldestPendingAckAge time.Duration // 最老的未确认 QoS1 消息存在时长

	// 每客户端计数器
	MessagesSent     int64
	MessagesDropped  int64
	MessagesRejected int64
	BytesSent        int64
	BytesReceived    int64
}

// ClientSubscriptionInfo 客户端订阅信息（不含 ClientID）
type ClientSubscriptionInfo struct {
	Topic string
	packet.SubscribeOptions
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
