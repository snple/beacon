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

// Core 是 Beacon v4 的核心：维护每客户端队列、channel 匹配与会话生命周期。
//
// 消息只保存一份（*Message），各客户端队列只持有指针与投递 ID，
// 因此 1→N 分发不复制消息内容，也不落盘。
type Core struct {
	options *CoreOptions
	logger  *zap.Logger

	// 客户端会话（在线 + 离线保留）
	clients   map[string]*Client
	clientsMu sync.RWMutex

	// registerMu 串行化会话注册与接管
	registerMu sync.Mutex
	// onlineConns 是已占用在线名额的连接数，受 registerMu 保护。
	// 用它而不是 CONNACK 之后才自增的统计值做 MaxClients 判定，避免并发握手超卖。
	onlineConns int

	matcher *matcher

	stats Stats

	// 内存中尚未完成投递的消息条数与近似字节数
	inflightMsgs  atomic.Int64
	inflightBytes atomic.Int64

	listenersMu sync.Mutex
	listeners   map[uint64]listenerEntry
	listenerSeq uint64

	startedAt time.Time

	connLimiter *tokenBucket

	handshakeSlots chan struct{}

	handshakeConns   map[net.Conn]struct{}
	handshakeConnsMu sync.Mutex

	// ===== stream =====
	streamEnabled atomic.Bool
	streamIDGen   atomic.Uint64

	streamMu      sync.Mutex
	streamPending map[uint64]*streamPending
	streams       map[uint64]*streamSession
	// streamPerClient 记录每个客户端参与的 pending+active 流数量（受 streamMu 保护）。
	streamPerClient map[string]int

	streamHandshakeSlots chan struct{}

	streamListenersMu sync.Mutex
	streamListeners   map[uint64]streamListenerEntry
	streamListenerSeq uint64

	streamConnsMu sync.Mutex
	streamConns   map[net.Conn]struct{}

	statsCacheMu sync.Mutex
	statsCache   StatsSnapshot
	statsCacheAt time.Time

	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running atomic.Bool
	stopped atomic.Bool

	taskMu      sync.Mutex
	tasksClosed bool
}

// Stats core 统计信息
type Stats struct {
	ClientsConnected atomic.Int64
	ClientsTotal     atomic.Int64
	MessagesReceived atomic.Int64 // PUSH 接收的记录数
	MessagesSent     atomic.Int64 // PULL 发出的记录数
	MessagesDropped  atomic.Int64 // 队列溢出丢弃
	MessagesRejected atomic.Int64 // 接收侧拒绝（非法、ACL、配额、钩子）
	MessagesNoMatch  atomic.Int64 // 无匹配客户端
	BytesReceived    atomic.Int64
	BytesSent        atomic.Int64
	DuplicatesSent   atomic.Int64
	CommitRequired   atomic.Int64
	SessionTakeovers atomic.Int64
	TargetMisses     atomic.Int64

	// stream
	StreamsActive   atomic.Int64
	StreamsTotal    atomic.Int64
	StreamsRejected atomic.Int64
	StreamBytesAToB atomic.Int64
	StreamBytesBToA atomic.Int64
}

// NewWithOptions 使用 Builder 模式创建新的 core
func NewWithOptions(opts *CoreOptions) (*Core, error) {
	if opts == nil {
		opts = NewCoreOptions()
	}
	opts.applyDefaults()

	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Core{
		options:        opts,
		logger:         opts.Logger,
		clients:        make(map[string]*Client),
		matcher:        newMatcher(),
		handshakeConns: make(map[net.Conn]struct{}),
		listeners:      make(map[uint64]listenerEntry),
		startedAt:      time.Now(),
		ctx:            ctx,
		cancel:         cancel,

		streamPending:   make(map[uint64]*streamPending),
		streams:         make(map[uint64]*streamSession),
		streamPerClient: make(map[string]int),
		streamListeners: make(map[uint64]streamListenerEntry),
		streamConns:     make(map[net.Conn]struct{}),
	}
	c.streamEnabled.Store(opts.StreamEnabled)

	if opts.MaxHandshakeConnections > 0 {
		c.handshakeSlots = make(chan struct{}, opts.MaxHandshakeConnections)
	}
	if opts.StreamMaxHandshakeConnections > 0 {
		c.streamHandshakeSlots = make(chan struct{}, opts.StreamMaxHandshakeConnections)
	}
	if opts.ConnectionRateLimit > 0 {
		c.connLimiter = newTokenBucket(opts.ConnectionRateLimit, opts.ConnectionRateBurst)
	}

	return c, nil
}

// New 创建新的 Core（使用默认配置）
func New() (*Core, error) { return NewWithOptions(NewCoreOptions()) }

// Start 启动 core
func (c *Core) Start() error {
	if c.stopped.Load() {
		return ErrCoreStopped
	}
	if !c.running.CompareAndSwap(false, true) {
		return ErrCoreAlreadyRunning
	}
	if c.stopped.Load() {
		c.running.Store(false)
		return ErrCoreStopped
	}

	c.taskMu.Lock()
	c.tasksClosed = false
	c.taskMu.Unlock()

	if c.options.SessionCheckInterval > 0 {
		c.wg.Add(1)
		go c.sessionCleanupLoop()
	}

	c.wg.Add(1)
	go c.streamCleanupLoop()

	c.logger.Info("core started",
		zap.Uint32("maxClients", c.options.MaxClients),
		zap.Int("queueCap", c.options.QueueCap),
		zap.Uint16("maxRecords", c.options.MaxRecords))
	return nil
}

// Stop 停止 core
func (c *Core) Stop() error {
	if c.stopped.Swap(true) {
		return nil
	}
	wasRunning := c.running.Swap(false)
	if !wasRunning {
		return nil
	}

	c.logger.Info("stopping core...")

	c.taskMu.Lock()
	c.tasksClosed = true
	c.taskMu.Unlock()

	c.cancel()
	c.closeHandshakeConns()
	c.closeStreamListeners()
	c.closeStreamConns()

	c.clientsMu.Lock()
	clients := make([]*Client, 0, len(c.clients))
	for _, cl := range c.clients {
		clients = append(clients, cl)
	}
	c.clientsMu.Unlock()

	// 并发通知，避免 N 个慢连接各等一个写超时导致停机挂起
	var notifyWG sync.WaitGroup
	for _, cl := range clients {
		cn := cl.currentConn()
		if cn == nil {
			continue
		}
		notifyWG.Add(1)
		go func(cn *conn) {
			defer notifyWG.Done()
			cn.terminate(packet.ReasonServerShuttingDown, true)
		}(cn)
	}
	notifyWG.Wait()

	c.wg.Wait()

	c.streamMu.Lock()
	c.streamPending = make(map[uint64]*streamPending)
	c.streams = make(map[uint64]*streamSession)
	c.streamPerClient = make(map[string]int)
	c.streamMu.Unlock()

	c.logger.Info("core stopped")
	return nil
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

// releaseMessage 释放一个消息引用；归零时扣减在途统计。
func (c *Core) releaseMessage(m *Message) {
	if m == nil {
		return
	}
	if m.release() {
		c.inflightMsgs.Add(-1)
		c.inflightBytes.Add(-m.Size())
	}
}

// takeOwnership 为新建的消息登记初始引用（由发布路径持有）。
func (c *Core) takeOwnership(m *Message) {
	m.retain()
	c.inflightMsgs.Add(1)
	c.inflightBytes.Add(m.Size())
}

// sessionCleanupLoop 定期清理过期会话。
func (c *Core) sessionCleanupLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.options.SessionCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case now := <-ticker.C:
			c.expireSessions(now)
		}
	}
}

func (c *Core) expireSessions(now time.Time) {
	uncommitted := c.options.UncommittedTimeout
	nowMs := now.UnixMilli()

	c.clientsMu.RLock()
	var (
		expired []*Client
		stale   []*Client
	)
	for _, cl := range c.clients {
		at := cl.expiresAt.Load()
		if at > 0 && nowMs >= at && cl.currentConn() == nil {
			expired = append(expired, cl)
			continue
		}
		if uncommitted > 0 {
			if since := cl.deliveredSince.Load(); since > 0 && nowMs-since >= uncommitted.Milliseconds() {
				stale = append(stale, cl)
			}
		}
	}
	c.clientsMu.RUnlock()

	for _, cl := range expired {
		c.destroySession(cl, packet.ReasonKeepAliveTimeout)
	}
	for _, cl := range stale {
		if cn := cl.currentConn(); cn != nil {
			c.logger.Debug("uncommitted timeout", zap.String("clientID", cl.ID))
			cn.terminate(packet.ReasonCommitRequired, true)
		}
	}
}

// destroySession 释放会话及其队列。
func (c *Core) destroySession(cl *Client, reason packet.ReasonCode) {
	c.registerMu.Lock()
	c.clientsMu.Lock()
	cur, ok := c.clients[cl.ID]
	if ok && cur == cl {
		delete(c.clients, cl.ID)
	} else {
		ok = false
	}
	c.clientsMu.Unlock()

	if ok {
		c.matcher.remove(cl.ID)
	}
	c.registerMu.Unlock()

	if !ok {
		return
	}

	cl.mb.releaseAll()
	c.logger.Debug("session destroyed",
		zap.String("clientID", cl.ID),
		zap.String("reason", reason.String()))
}

// canRead 校验某身份是否可以声明该过滤器。
func (c *Core) canRead(id *Identity, filter string) bool {
	if !c.authEnabled() {
		return true
	}
	if id == nil {
		return false
	}
	for _, allow := range id.ReadChannels {
		if packet.FilterSubsumes(allow, filter) {
			return true
		}
	}
	return false
}

// canWrite 校验某身份是否可以发布到该 channel。
func (c *Core) canWrite(id *Identity, channel string) bool {
	if !c.authEnabled() {
		return true
	}
	if id == nil {
		return false
	}
	for _, allow := range id.WriteChannels {
		if packet.MatchChannel(allow, channel) {
			return true
		}
	}
	return false
}

// canTarget 校验某身份是否允许定向投递（Record.Target 非空）。
//
// 定向投递会跳过 channel 匹配，直接写入目标队列，因此需要单独授权；
// 未配置认证器时不做限制。
func (c *Core) canTarget(id *Identity) bool {
	if !c.authEnabled() {
		return true
	}
	return id != nil && id.AllowTargeted
}

// authEnabled 报告是否配置了认证器（未配置时不做 ACL 校验）。
func (c *Core) authEnabled() bool {
	return c.options.Hooks != nil && c.options.Hooks.Auth != nil
}

// StatsSnapshot 是 GetStats 的返回快照。
type StatsSnapshot struct {
	StartedAt        time.Time
	ClientsConnected int64
	ClientsTotal     int64
	Sessions         int64
	MessagesReceived int64
	MessagesSent     int64
	MessagesDropped  int64
	MessagesRejected int64
	MessagesNoMatch  int64
	BytesReceived    int64
	BytesSent        int64
	DuplicatesSent   int64
	CommitRequired   int64
	SessionTakeovers int64
	TargetMisses     int64
	InFlightMessages int64
	InFlightBytes    int64
	WildcardFilters  int

	StreamsActive   int64
	StreamsPending  int64
	StreamsTotal    int64
	StreamsRejected int64
	StreamBytesAToB int64
	StreamBytesBToA int64
}

// GetStats 返回统计快照。
func (c *Core) GetStats() StatsSnapshot {
	c.clientsMu.RLock()
	sessions := int64(len(c.clients))
	c.clientsMu.RUnlock()

	c.streamMu.Lock()
	pending := int64(len(c.streamPending))
	c.streamMu.Unlock()

	return StatsSnapshot{
		StartedAt:        c.startedAt,
		ClientsConnected: c.stats.ClientsConnected.Load(),
		ClientsTotal:     c.stats.ClientsTotal.Load(),
		Sessions:         sessions,
		MessagesReceived: c.stats.MessagesReceived.Load(),
		MessagesSent:     c.stats.MessagesSent.Load(),
		MessagesDropped:  c.stats.MessagesDropped.Load(),
		MessagesRejected: c.stats.MessagesRejected.Load(),
		MessagesNoMatch:  c.stats.MessagesNoMatch.Load(),
		BytesReceived:    c.stats.BytesReceived.Load(),
		BytesSent:        c.stats.BytesSent.Load(),
		DuplicatesSent:   c.stats.DuplicatesSent.Load(),
		CommitRequired:   c.stats.CommitRequired.Load(),
		SessionTakeovers: c.stats.SessionTakeovers.Load(),
		TargetMisses:     c.stats.TargetMisses.Load(),
		InFlightMessages: c.inflightMsgs.Load(),
		InFlightBytes:    c.inflightBytes.Load(),
		WildcardFilters:  c.matcher.wildcardCount(),

		StreamsActive:   c.stats.StreamsActive.Load(),
		StreamsPending:  pending,
		StreamsTotal:    c.stats.StreamsTotal.Load(),
		StreamsRejected: c.stats.StreamsRejected.Load(),
		StreamBytesAToB: c.stats.StreamBytesAToB.Load(),
		StreamBytesBToA: c.stats.StreamBytesBToA.Load(),
	}
}
