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
	subTree *subTree

	// 消息队列 (按优先级)
	queues [4]*messageQueue

	// 消息通知 channel
	msgNotify chan struct{}

	// 保留消息
	retainStore *retainStore

	// 消息持久化存储
	messageStore *messageStore

	// REQUEST/RESPONSE 支持（轮询模式）
	actionRegistry *actionRegistry // action 注册表

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
		subTree:         newSubTree(),
		retainStore:     newRetainStore(),
		actionRegistry:  newActionRegistry(),
		msgNotify:       make(chan struct{}, 1),
		ctx:             ctx,
		cancel:          cancel,
	}

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

	store, err := newMessageStore(c.options.StorageConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize message store: %w", err)
	}
	c.messageStore = store
	return nil
}

// initPriorityQueues 初始化优先级队列
func (c *Core) initPriorityQueues() {
	for i := range c.queues {
		c.queues[i] = newMessageQueue(packet.Priority(i), priorityQueueCapacity)
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
	go c.accept()
}

// logStartupInfo 记录启动信息
func (c *Core) logStartupInfo() {
	c.logger.Info("core started",
		zap.String("address", c.options.Address),
		zap.Uint32("maxClients", c.options.MaxClients),
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

	// 最后关闭消息存储
	if c.messageStore != nil {
		c.messageStore.close()
	}

	c.logger.Info("core stopped")
	return nil
}

// Subscribe 订阅主题
func (c *Core) Subscribe(clientID string, sub packet.Subscription) {
	isNew := c.subTree.subscribe(clientID, sub.Topic, sub.Options.QoS)
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

	// 从 messageStore 加载消息
	if c.messageStore == nil {
		return nil
	}

	messages := make([]*Message, 0, len(topics))
	for _, t := range topics {
		msg, err := c.messageStore.getRetainMessage(t)
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

// ============================================================================
// 管理 API
// ============================================================================

// ClientInfo 客户端信息
type ClientInfo struct {
	ClientID       string                   `json:"client_id"`
	RemoteAddr     string                   `json:"remote_addr"`
	Connected      bool                     `json:"connected"`
	ConnectedAt    time.Time                `json:"connected_at"`
	KeepAlive      uint16                   `json:"keep_alive"`
	KeepSession    bool                     `json:"keep_session"`
	SessionTimeout uint32                   `json:"session_timeout"`
	Subscriptions  []ClientSubscriptionInfo `json:"subscriptions"`
	PendingAck     int                      `json:"pending_ack"`
	ReceiveWindow  uint16                   `json:"receive_window"`
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
	client.session.subsMu.RLock()
	subs := make([]ClientSubscriptionInfo, 0, len(client.session.subscriptions))
	for topic, opts := range client.session.subscriptions {
		subs = append(subs, ClientSubscriptionInfo{
			Topic: topic,
			QoS:   opts.QoS,
		})
	}
	client.session.subsMu.RUnlock()

	// 获取 ACK 信息
	client.session.pendingAckMu.Lock()
	pendingAck := len(client.session.pendingAck)
	client.session.pendingAckMu.Unlock()

	client.connMu.RLock()
	conn := client.conn
	client.connMu.RUnlock()

	info := &ClientInfo{
		ClientID:       client.ID,
		RemoteAddr:     conn.conn.RemoteAddr().String(), // 需要通过 conn 获取
		Connected:      !client.Closed(),
		ConnectedAt:    client.ConnectedAt(),
		KeepAlive:      client.KeepAlive(),
		KeepSession:    client.KeepSession(),
		SessionTimeout: client.SessionTimeout(),
		Subscriptions:  subs,
		PendingAck:     pendingAck,
		ReceiveWindow:  conn.sendWindow, // 需要通过 conn 获取
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
			Topic: topic,
			QoS:   opts.QoS,
		})
	}

	return subs, nil
}

// GetTopicSubscribers 获取订阅指定主题的所有客户端
func (c *Core) GetTopicSubscribers(topic string) map[string]packet.QoS {
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
