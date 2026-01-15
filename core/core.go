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

// Core 相关常量
const (
	// 优先级队列容量
	priorityQueueCapacity = 10000
)

// Core
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
	subscriptions *SubscriptionTree

	// 消息队列 (按优先级)
	queues [4]*MessageQueue

	// 消息通知 channel
	msgNotify chan struct{}

	// 保留消息
	retainStore *RetainStore

	// 消息持久化存储
	messageStore *MessageStore

	// REQUEST/RESPONSE 支持（轮询模式）
	actionRegistry   *ActionRegistry // action 注册表
	requestTracker   *RequestTracker // 请求追踪器
	requestQueue     chan *Request
	requestQueueMu   sync.Mutex
	requestQueueInit bool // 请求队列是否已初始化

	// MESSAGE 轮询支持
	messageQueue     chan *Message
	messageQueueMu   sync.Mutex
	messageQueueInit bool // 消息队列是否已初始化

	// 统计信息
	stats Stats

	// 生命周期
	listener net.Listener
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	running  atomic.Bool
}

// Stats core 统计信息
type Stats struct {
	ClientsConnected   atomic.Int64
	ClientsTotal       atomic.Int64
	MessagesReceived   atomic.Int64
	MessagesSent       atomic.Int64
	MessagesDropped    atomic.Int64
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
		subscriptions:   NewSubscriptionTree(),
		retainStore:     NewRetainStore(),
		actionRegistry:  NewActionRegistry(),
		msgNotify:       make(chan struct{}, 1),
		ctx:             ctx,
		cancel:          cancel,
	}

	// 初始化请求追踪器（需要 core 引用）
	b.requestTracker = NewRequestTracker(b)

	// 初始化消息持久化存储
	if err := b.initMessageStore(); err != nil {
		return nil, err
	}

	// 初始化优先级队列
	b.initPriorityQueues()

	return b, nil
}

// New 创建新的 Core（使用默认配置）
func New() (*Core, error) {
	return NewWithOptions(NewCoreOptions())
}

// initMessageStore 初始化消息存储
func (c *Core) initMessageStore() error {
	if !c.options.StorageConfig.Enabled {
		return nil
	}

	// 确保 Logger 正确传递
	if c.options.StorageConfig.Logger == nil {
		c.options.StorageConfig.Logger = c.options.Logger
	}

	store, err := NewMessageStore(c.options.StorageConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize message store: %w", err)
	}
	c.messageStore = store
	return nil
}

// initPriorityQueues 初始化优先级队列
func (c *Core) initPriorityQueues() {
	for i := range c.queues {
		c.queues[i] = NewMessageQueue(packet.Priority(i), priorityQueueCapacity)
	}
}

// Start 启动 core
func (c *Core) Start() error {
	if c.running.Load() {
		return ErrCoreAlreadyRunning
	}

	// 启动网络监听
	listener, err := c.startListener()
	if err != nil {
		return err
	}
	c.listener = listener
	c.running.Store(true)

	// 恢复持久化的消息
	if c.messageStore != nil {
		c.recoverMessages()
	}

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

	// 接受连接
	c.wg.Add(1)
	go c.acceptLoop()
}

// logStartupInfo 记录启动信息
func (c *Core) logStartupInfo() {
	c.logger.Info("core started",
		zap.String("address", c.options.Address),
		zap.Int("maxClients", c.options.MaxClients),
		zap.Bool("tls", c.options.TLSConfig != nil),
		zap.Bool("persistenceEnabled", c.messageStore != nil))
}

// Stop 停止 core
func (c *Core) Stop() error {
	if !c.running.Load() {
		return nil
	}

	c.logger.Info("Stopping core...")
	c.running.Store(false)
	c.cancel()

	// 关闭监听器，停止接受新连接
	if c.listener != nil {
		c.listener.Close()
	}

	// 断开所有普通客户端
	c.clientsMu.Lock()
	for _, client := range c.clients {
		client.Close(packet.ReasonServerShuttingDown)
	}
	c.clientsMu.Unlock()

	// 等待所有协程退出
	c.wg.Wait()

	// 最后关闭消息存储（确保客户端清理完成后再关闭）
	if c.messageStore != nil {
		c.messageStore.Close()
	}

	c.logger.Info("core stopped")
	return nil
}

// recoverMessages 恢复持久化的消息
func (c *Core) recoverMessages() {
	if c.messageStore == nil {
		return
	}

	// 只恢复保留消息到内存 (数量通常较少)
	retainMsgs, err := c.messageStore.GetAllRetainMessages()
	if err != nil {
		c.logger.Error("Failed to recover retain messages", zap.Error(err))
	} else {
		for _, stored := range retainMsgs {
			msg := storedToMessage(stored)
			c.retainStore.Set(msg.Topic, msg)
		}
		if len(retainMsgs) > 0 {
			c.logger.Info("Recovered retain messages", zap.Int("count", len(retainMsgs)))
		}
	}

	// 统计待投递消息数量 (不加载到内存，客户端重连时按需加载)
	stats, err := c.messageStore.GetStats()
	if err != nil {
		c.logger.Error("Failed to get storage stats", zap.Error(err))
	} else if stats.TotalMessages > 0 {
		c.logger.Info("Pending messages in storage (lazy load on client reconnect)",
			zap.Int64("count", stats.TotalMessages))
	}
}

// Subscribe 订阅主题
func (c *Core) Subscribe(clientID string, sub packet.Subscription) {
	isNew := c.subscriptions.Add(clientID, sub.Topic, sub.Options.QoS)
	if isNew {
		c.stats.SubscriptionsCount.Add(1)
	}
}

// Unsubscribe 取消订阅
func (c *Core) Unsubscribe(clientID, topic string) {
	if removed := c.subscriptions.Remove(clientID, topic); removed {
		c.stats.SubscriptionsCount.Add(-1)
	}
}

// GetRetainedMessages 获取与主题模式匹配的保留消息
func (c *Core) GetRetainedMessages(topic string) []*Message {
	return c.retainStore.Match(topic)
}

// GetStats 获取统计信息快照
func (c *Core) GetStats() StatsSnapshot {
	return StatsSnapshot{
		ClientsConnected:   c.stats.ClientsConnected.Load(),
		ClientsTotal:       c.stats.ClientsTotal.Load(),
		MessagesReceived:   c.stats.MessagesReceived.Load(),
		MessagesSent:       c.stats.MessagesSent.Load(),
		MessagesDropped:    c.stats.MessagesDropped.Load(),
		BytesReceived:      c.stats.BytesReceived.Load(),
		BytesSent:          c.stats.BytesSent.Load(),
		SubscriptionsCount: c.stats.SubscriptionsCount.Load(),
	}
}

// StatsSnapshot 统计信息快照 (可安全复制)
type StatsSnapshot struct {
	ClientsConnected   int64
	ClientsTotal       int64
	MessagesReceived   int64
	MessagesSent       int64
	MessagesDropped    int64
	BytesReceived      int64
	BytesSent          int64
	SubscriptionsCount int64
}

// 生成随机 Client ID
func generateClientID() string {
	return fmt.Sprintf("queen-%d", time.Now().UnixNano())
}

// ============================================================================
// 管理 API
// ============================================================================

// ClientInfo 客户端信息
type ClientInfo struct {
	ClientID      string                   `json:"client_id"`
	RemoteAddr    string                   `json:"remote_addr"`
	Connected     bool                     `json:"connected"`
	ConnectedAt   time.Time                `json:"connected_at"`
	KeepAlive     uint16                   `json:"keep_alive"`
	CleanSession  bool                     `json:"clean_start"`
	SessionExpiry uint32                   `json:"session_expiry"`
	Subscriptions []ClientSubscriptionInfo `json:"subscriptions"`
	PendingAck    int                      `json:"pending_ack"`
	ReceiveWindow uint16                   `json:"receive_window"`
}

// ClientSubscriptionInfo 客户端订阅信息（不含 ClientID）
type ClientSubscriptionInfo struct {
	Topic string     `json:"topic"`
	QoS   packet.QoS `json:"qos"`
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
	client.subsMu.RLock()
	subs := make([]ClientSubscriptionInfo, 0, len(client.subscriptions))
	for topic, opts := range client.subscriptions {
		subs = append(subs, ClientSubscriptionInfo{
			Topic: topic,
			QoS:   opts.QoS,
		})
	}
	client.subsMu.RUnlock()

	// 获取 ACK 信息
	client.qosMu.Lock()
	pendingAck := len(client.pendingAck)
	client.qosMu.Unlock()

	info := &ClientInfo{
		ClientID:      client.ID,
		RemoteAddr:    client.conn.RemoteAddr().String(),
		Connected:     !client.IsClosed(),
		ConnectedAt:   client.ConnectedAt,
		KeepAlive:     client.KeepAlive,
		CleanSession:  client.CleanSession,
		SessionExpiry: client.SessionExpiry,
		Subscriptions: subs,
		PendingAck:    pendingAck,
		ReceiveWindow: client.sendWindow,
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

	if exists && !client.IsClosed() {
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

	client.subsMu.RLock()
	defer client.subsMu.RUnlock()

	subs := make([]ClientSubscriptionInfo, 0, len(client.subscriptions))
	for topic, opts := range client.subscriptions {
		subs = append(subs, ClientSubscriptionInfo{
			Topic: topic,
			QoS:   opts.QoS,
		})
	}

	return subs, nil
}

// GetTopicSubscribers 获取订阅指定主题的所有客户端
func (c *Core) GetTopicSubscribers(topic string) []string {
	subscribers := c.subscriptions.Match(topic)
	clientIDs := make([]string, 0, len(subscribers))
	for _, sub := range subscribers {
		clientIDs = append(clientIDs, sub.ClientID)
	}
	return clientIDs
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

// GetAddress 返回 core 监听的地址
// 如果使用了随机端口（:0），则返回实际分配的地址
func (c *Core) GetAddress() string {
	if c.listener != nil {
		return c.listener.Addr().String()
	}
	return c.options.Address
}

// GetLogger 获取当前使用的 Logger
func (c *Core) GetLogger() *zap.Logger {
	return c.logger
}
