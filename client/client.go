// Package client 提供 Beacon 协议客户端库
package client

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// 客户端相关常量
const (
	// 超时相关
	defaultKeepAlive          = 60 // 默认心跳间隔（秒）
	defaultConnectTimeout     = 10 * time.Second
	defaultPublishTimeout     = 30 * time.Second
	defaultRetransmitInterval = 5 * time.Second // QoS1 消息重传间隔

	// 流量控制
	defaultMaxPacketSize = 1048576 // 默认最大包大小（1 MB）
	defaultReceiveWindow = 100     // 默认接收窗口大小

	// 重传相关
	retransmitBatchSize = 50 // 每次从持久化加载的最大消息数
)

// Message 内部消息表示
//
// 设计原则：
// - 内容层：直接引用原始 *packet.PublishPacket（视为只读、可共享）
// - 投递层（PacketID/QoS/Dup）由发送队列/持久化层单独管理
type Message struct {
	// 原始 PUBLISH 包（视为只读）
	Packet *packet.PublishPacket `nson:"pkt"`

	// 固定头部标志
	Dup bool `nson:"dup"` // 重发标志

	// 时间相关（非协议字段，仅用于调试/观察）
	Timestamp int64 `nson:"ts"`

	// 手动确认回调：仅当 AutoAck=false 且消息为 QoS1 时非 nil。
	// 不导出且不参与 nson 序列化，仅内存中生效。
	ack func() error
}

// Ack 手动确认消息已处理（仅当客户端 AutoAck=false 且消息为 QoS1 时有效）。
// 应用在收到消息并处理完成后调用；返回错误表示确认发送失败
// （例如连接已断开，消息可能被服务端重传）。
//
// 注意：手动确认模式下 QoS1 为 at-least-once——若应用在处理中崩溃/断电未确认，
// 服务端会重传该消息并再次投递。应用如需不重复处理，请按 Packet.PacketID 自行去重。
func (m *Message) Ack() error {
	if m.ack == nil {
		return ErrNoPendingAck
	}
	return m.ack()
}

func (m *Message) IsExpired() bool {
	if m.Packet == nil || m.Packet.Properties == nil || m.Packet.Properties.ExpiryTime == 0 {
		return false
	}
	return time.Now().Unix() > m.Packet.Properties.ExpiryTime
}

// Topic 返回消息主题
func (m *Message) Topic() string {
	if m.Packet != nil {
		return m.Packet.Topic
	}
	return ""
}

// Payload 返回消息负载
func (m *Message) Payload() []byte {
	if m.Packet != nil {
		return m.Packet.Payload
	}
	return nil
}

// Client 客户端（会话管理器）
// 负责管理跨连接的会话状态：订阅、消息队列、持久化存储等
type Client struct {
	options *ClientOptions

	// 订阅主题列表
	subscribedTopics   map[string]bool
	subscribedTopicsMu sync.RWMutex

	// 当前活跃的连接（可能为 nil）
	conn      *Conn
	connMu    sync.RWMutex
	connectMu sync.Mutex

	// 连接拨号器（用于自动重连）
	dialer Dialer

	// 连接状态
	connected atomic.Bool

	// 轮询模式队列
	messageQueue chan *Message

	// 自动重连
	autoReconnect atomic.Bool
	reconnecting  atomic.Bool
	connLostCh    chan struct{} // 仅用于唤醒重连循环，不携带错误值

	// 后台 goroutine 跟踪（reconnectLoop / retransmitLoop / 按需重传）
	wg sync.WaitGroup

	// 临时任务的启动守卫：Close 之后禁止再向 wg 添加任务，
	// 否则会与 Close 中的 wg.Wait 竞争（Wait 期间 Add 会 panic）。
	taskMu      sync.Mutex
	tasksClosed bool

	// 消息持久化存储
	store *store

	// 消息队列（持久化）
	queue *Queue

	retransmitting atomic.Bool

	// 重传协程启动守卫：与 retransmitting（TriggerRetransmit 一次性守卫）分离，
	// 避免标志冲突导致 retransmitLoop 永不启动
	retransmitLoopStarted atomic.Bool

	// 生命周期
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once

	logger *zap.Logger
}

// NewWithOptions 使用 Builder 模式创建新的客户端（推荐）
//
// 用法：
//
//	c, err := client.NewWithOptions(
//	    client.NewClientOptions().
//	        WithCore("127.0.0.1:3883").
//	        WithClientID("c1").
//	        WithKeepAlive(60),
//	)
func NewWithOptions(opts *ClientOptions) (*Client, error) {
	if opts == nil {
		opts = NewClientOptions()
	}
	opts.applyDefaults()
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 使用用户提供的 logger，或创建默认 logger
	logger := opts.Logger
	if logger == nil {
		logger, _ = zap.NewDevelopment()
	}

	c := &Client{
		options:          opts,
		subscribedTopics: make(map[string]bool),
		ctx:              ctx,
		cancel:           cancel,
		connLostCh:       make(chan struct{}, 1),
		logger:           logger,
		messageQueue:     make(chan *Message, opts.MessageQueueSize),
	}

	// 初始化消息存储

	if opts.StoreOptions.Logger == nil {
		opts.StoreOptions.Logger = logger
	}
	store, err := newStore(opts.StoreOptions)
	if err != nil {
		logger.Error("Failed to initialize message store", zap.Error(err))
		return nil, err
	}

	c.store = store

	// 初始化消息队列
	c.queue = NewQueue(store.db(), "queue:")

	logger.Info("Message queue initialized")

	// 启动统一的重连循环（内部会检查 autoReconnect 开关）
	c.wg.Add(1)
	go c.reconnectLoop()

	return c, nil
}

// spawnTask 启动受 wg 跟踪的后台任务。客户端已关闭时返回 false。
func (c *Client) spawnTask(fn func()) bool {
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

// New 创建新的客户端（使用默认配置）
func New() (*Client, error) {
	return NewWithOptions(NewClientOptions())
}

// onConnLost 当连接断开时由 Connection 调用
func (c *Client) onConnLost(err error) {
	c.connected.Store(false)

	// 获取当前连接的 clientID
	var clientID string
	c.connMu.RLock()
	if c.conn != nil {
		clientID = c.conn.clientID
	}
	c.connMu.RUnlock()

	// 调用 OnDisconnect 钩子
	c.options.Hooks.callOnDisconnect(&DisconnectContext{
		ClientID: clientID,
		Packet:   nil,
		Err:      err,
	})

	// 通知重连循环（错误已通过 OnDisconnect 钩子暴露，这里只需要信号）
	select {
	case c.connLostCh <- struct{}{}:
	default:
	}
}

func (c *Client) close(err error) error {
	c.closeOnce.Do(func() {
		c.connected.Store(false)

		// 关闭当前连接
		c.connMu.Lock()
		conn := c.conn
		c.conn = nil
		c.connMu.Unlock()

		if conn != nil {
			conn.close(err)
			conn.wait()
		}

	})
	return nil
}

// Close 关闭客户端并释放所有资源
func (c *Client) Close() error {
	// 禁用自动重连
	c.autoReconnect.Store(false)

	err := c.Disconnect()
	// 关闭整个客户端生命周期，唤醒所有等待 ctx.Done() 的 goroutine
	c.cancel()

	// 先禁止新的临时任务，避免与下面的 wg.Wait 竞争（Wait 期间 Add 会 panic）
	c.taskMu.Lock()
	c.tasksClosed = true
	c.taskMu.Unlock()

	// 等待后台 goroutine（reconnectLoop、retransmitLoop、按需重传）退出后再清理资源
	c.wg.Wait()

	// 若重连协程在关闭期间恰好创建了新连接，等待其完全退出，
	// 确保不再有任何 goroutine 访问即将关闭的存储（H3/M3 边界）。
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()
	if conn != nil {
		conn.close(ErrClientClosed)
		conn.wait()
	}

	// 关闭消息存储
	if c.store != nil {
		if storeErr := c.store.Close(); storeErr != nil {
			c.logger.Warn("Failed to close message store", zap.Error(storeErr))
		}
		c.store = nil
	}

	return err
}

// IsConnected 检查是否已连接
func (c *Client) IsConnected() bool {
	return c.connected.Load()
}

// SessionPresent 返回服务端是否恢复了会话
// 当 CleanSession=false 且服务端有旧会话时返回 true
func (c *Client) SessionPresent() bool {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	if c.conn != nil {
		return c.conn.sessionPresent
	}
	return false
}

// ClientID 返回客户端 ID
func (c *Client) ClientID() string {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	if c.conn != nil {
		return c.conn.clientID
	}
	return c.options.ClientID
}

// MaxPacketSize 返回客户端允许接收的最大包大小
func (c *Client) MaxPacketSize() uint32 {
	return c.options.MaxPacketSize
}

// GetSendWindow 返回 core 的接收窗口大小（客户端的发送窗口）
func (c *Client) GetSendWindow() uint16 {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	if c.conn != nil {
		return c.conn.sendWindow
	}
	return defaultReceiveWindow
}

// GetLogger 返回客户端使用的日志记录器
func (c *Client) GetLogger() *zap.Logger {
	return c.logger
}

// ForceClose 强制关闭连接（不发送 DISCONNECT 包）
// 用于测试遗嘱消息等场景
// 注意：这是一个测试辅助方法，生产环境请使用 Disconnect() 或 Close()
func (c *Client) ForceClose() error {
	return c.close(nil)
}

// LastRTT 返回最近一次心跳 RTT（0 表示未知）
func (c *Client) LastRTT() time.Duration {
	c.connMu.RLock()
	defer c.connMu.RUnlock()

	if c.conn == nil {
		return 0
	}
	return c.conn.LastRTT()
}

// allocatePacketID 分配新的 PacketID（使用 nson.Id 以保证全局唯一性和有序性）
func (c *Client) allocatePacketID() nson.Id {
	return nson.NewId()
}

// getConn 获取当前活跃的连接
func (c *Client) getConn() *Conn {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.conn
}

// writePacket 发送数据包（通过当前连接）
func (c *Client) writePacket(pkt packet.Packet) error {
	conn := c.getConn()
	if conn == nil || conn.closed.Load() {
		return ErrNotConnected
	}
	return conn.writePacket(pkt)
}

// GetSubscribedTopics 获取当前订阅的主题列表
func (c *Client) GetSubscribedTopics() []string {
	c.subscribedTopicsMu.RLock()
	defer c.subscribedTopicsMu.RUnlock()

	topics := make([]string, 0, len(c.subscribedTopics))
	for topic := range c.subscribedTopics {
		topics = append(topics, topic)
	}
	return topics
}

// HasSubscription 检查是否订阅了指定主题
func (c *Client) HasSubscription(topic string) bool {
	c.subscribedTopicsMu.RLock()
	defer c.subscribedTopicsMu.RUnlock()

	return c.subscribedTopics[topic]
}
