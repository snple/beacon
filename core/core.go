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

// Core 是 Beacon 消息总线的核心：管理连接、会话、订阅与内存消息队列。
//
// 消息只在 messageStore 中以 broker 生成的唯一 ID 保存一份，全局队列与各客户端
// 队列都只引用该 ID，因此 1→N 分发不复制消息内容，也不落盘。
type Core struct {
	options *CoreOptions
	logger  *zap.Logger

	// 客户端管理（在线 + 离线会话）
	clients   map[string]*Client
	clientsMu sync.RWMutex

	// registerMu 串行化客户端注册。
	// 丢弃旧会话时需要在锁外清理其队列与会话状态，
	// 若并发的同 ClientID 注册在此期间插入新客户端，清理会误删新会话的数据；
	// 因此注册过程整体串行，clientsMu 只用于保护 map 的读写。
	registerMu sync.Mutex

	// 离线会话过期管理
	offlineSessions   map[string]time.Time // clientID -> 过期时间
	offlineSessionsMu sync.Mutex

	// 订阅管理
	subTree *subTree

	// 共享消息表：key 为 broker 生成的唯一 ID
	msgs *messageStore

	// 全局待分发队列（只保存消息 ID）
	pending      *pendingQueue
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

	// 创建时间（管理面 Info/GetStats 的 StartedAt）。构造后只读，无需同步。
	startedAt time.Time

	// 连接速率限制
	connLimiter *tokenBucket

	// 握手阶段的并发上限：认证前的连接还没有进入 MaxClients 统计，
	// 必须单独设限，否则未认证连接可以无上限地占用 goroutine 与包缓冲。
	handshakeSlots chan struct{}

	// 握手阶段的连接：Stop 时主动关闭，避免优雅停机等待 ConnectTimeout。
	handshakeConns   map[net.Conn]struct{}
	handshakeConnsMu sync.Mutex

	// 生命周期
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running atomic.Bool
	stopped atomic.Bool

	// 临时任务的启动守卫：Stop 之后禁止再向 wg 添加任务，
	// 否则会与 wg.Wait 竞争。
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

	msgs := newMessageStore()

	b := &Core{
		options:         opts,
		logger:          opts.Logger,
		clients:         make(map[string]*Client),
		offlineSessions: make(map[string]time.Time),
		subTree:         newSubTree(),
		msgs:            msgs,
		pending:         newPendingQueue(),
		queueTrigger:    make(chan struct{}, 1),
		retainStore:     newRetainStore(msgs),
		handshakeConns:  make(map[net.Conn]struct{}),
		events:          newEventBus(),
		listeners:       make(map[uint64]listenerEntry),
		startedAt:       time.Now(),
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

	return b, nil
}

// New 创建新的 Core（使用默认配置）
func New() (*Core, error) {
	return NewWithOptions(NewCoreOptions())
}

// Start 启动 core
func (c *Core) Start() error {
	// Stop 之后不可重启：ctx 已取消、后台协程已退出。
	// 需要重启请重新 NewWithOptions。
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

	c.taskMu.Lock()
	c.tasksClosed = false
	c.taskMu.Unlock()

	// 启动后台协程
	c.startBackgroundWorkers()

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

		// 关闭所有事件流。订阅者可能使用 context.Background()，仅靠调用方 ctx
		// 无法退出，通道不关闭会让消费者永久阻塞。
		c.events.closeAll()

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
// 若 clientID 已存在会话（在线或离线），订阅会同时写入会话状态，
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

	c.emit(Event{
		Type:     EventSubscribed,
		ClientID: clientID,
		Topic:    sub.Topic,
		QoS:      sub.Options.QoS,
		Detail:   "api",
	})
}

// Unsubscribe 取消订阅（编程接口）。
//
// 与 Subscribe 对称：存在会话时同步移除会话状态。
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

	c.emit(Event{
		Type:     EventUnsubscribed,
		ClientID: clientID,
		Topic:    topic,
		Detail:   "api",
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
// SUBSCRIBE 处理路径与编程接口使用它：会话状态的维护由调用方负责。
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

// GetStats 获取统计信息快照
func (c *Core) GetStats() StatsSnapshot {
	snapshot := StatsSnapshot{
		StartedAt:          c.startedAt,
		Limits:             c.limits(),
		EventsDropped:      int64(c.events.dropped.Load()),
		ClientsConnected:   c.stats.ClientsConnected.Load(),
		ClientsTotal:       c.stats.ClientsTotal.Load(),
		MessagesReceived:   c.stats.MessagesReceived.Load(),
		MessagesSent:       c.stats.MessagesSent.Load(),
		MessagesDropped:    c.stats.MessagesDropped.Load(),
		MessagesRejected:   c.stats.MessagesRejected.Load(),
		BytesReceived:      c.stats.BytesReceived.Load(),
		BytesSent:          c.stats.BytesSent.Load(),
		SubscriptionsCount: c.stats.SubscriptionsCount.Load(),
	}

	snapshot.PendingQueueDepth = int64(c.pending.len())
	snapshot.InFlightMessages, snapshot.PendingBytes = func() (int64, int64) {
		count, bytes := c.msgs.stats()
		return int64(count), bytes
	}()

	c.offlineSessionsMu.Lock()
	snapshot.OfflineSessions = int64(len(c.offlineSessions))
	c.offlineSessionsMu.Unlock()

	c.clientsMu.RLock()
	for _, client := range c.clients {
		if client.queue != nil {
			snapshot.ClientQueueDepth += int64(client.queue.len())
		}
		snapshot.PendingAckTotal += int64(client.session.pendingAck.len())
	}
	c.clientsMu.RUnlock()

	return snapshot
}

// StatsSnapshot 统计信息快照 (可安全复制)
type StatsSnapshot struct {
	StartedAt time.Time // Core 构造时间（只读）
	Limits    Limits    // 配置上限，用于计算使用率
	// EventsDropped 是所有事件订阅者累计丢弃的事件数（单个订阅者用 EventStream.Dropped() 观察）
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

	// 队列与内存水位（用于容量规划与告警）
	PendingQueueDepth int64 // 全局待分发队列深度
	ClientQueueDepth  int64 // 所有客户端待发送队列深度之和
	PendingAckTotal   int64 // 所有客户端未确认（QoS1）消息总数
	OfflineSessions   int64 // 保留中（在线时间之外）的离线会话数
	InFlightMessages  int64 // 内存中尚未完成投递的共享消息条数
	PendingBytes      int64 // 上述消息内容的近似内存占用（字节）
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
