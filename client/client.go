// Package client 提供 Queen 协议客户端库
package client

import (
	"context"
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
	defaultRequestTimeout     = 30 * time.Second
	defaultRetransmitInterval = 5 * time.Second // QoS1 消息重传间隔

	// 流量控制
	defaultReceiveWindow = 100 // 默认接收窗口大小

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
// 负责管理跨连接的会话状态：订阅、注册的 actions、消息队列、持久化存储等
type Client struct {
	options *ClientOptions

	// 订阅主题列表
	subscribedTopics   map[string]bool
	subscribedTopicsMu sync.RWMutex

	// 已注册的 actions
	registeredActions map[string]bool
	actionsMu         sync.RWMutex

	// 当前活跃的连接（可能为 nil）
	conn      *Conn
	connMu    sync.RWMutex
	connectMu sync.Mutex

	// 连接状态
	connected atomic.Bool

	// REQUEST/RESPONSE 支持
	nextRequestID   atomic.Uint32
	pendingRequests map[uint32]chan *Response
	pendingReqMu    sync.Mutex

	// 轮询模式队列
	requestQueue chan *Request
	messageQueue chan *Message

	// 自动重连
	autoReconnect atomic.Bool
	reconnecting  atomic.Bool
	reconnectWG   sync.WaitGroup
	connLostCh    chan error

	// 消息持久化存储
	store *store

	// 消息队列（持久化）
	queue *Queue

	retransmitting atomic.Bool

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

	ctx, cancel := context.WithCancel(context.Background())

	// 使用用户提供的 logger，或创建默认 logger
	logger := opts.Logger
	if logger == nil {
		logger, _ = zap.NewDevelopment()
	}

	c := &Client{
		options:           opts,
		subscribedTopics:  make(map[string]bool),
		registeredActions: make(map[string]bool),
		ctx:               ctx,
		cancel:            cancel,
		connLostCh:        make(chan error, 1),
		logger:            logger,
		messageQueue:      make(chan *Message, opts.MessageQueueSize),
		requestQueue:      make(chan *Request, opts.RequestQueueSize),
		pendingRequests:   make(map[uint32]chan *Response),
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

	// 初始化消息队列（基于 store.db）
	c.queue = NewQueue(store.db, "queue:")

	logger.Info("Message queue initialized")

	// 启动统一的重连循环（内部会检查 autoReconnect 开关）
	go c.reconnectLoop()

	return c, nil
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

	// 通知重连循环
	select {
	case c.connLostCh <- err:
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

		// 清理等待中的请求
		c.pendingReqMu.Lock()
		for _, ch := range c.pendingRequests {
			select {
			case ch <- &Response{
				Packet: &packet.ResponsePacket{
					ReasonCode: packet.ReasonServerShuttingDown,
				},
			}:
			default:
			}
		}
		c.pendingRequests = make(map[uint32]chan *Response)
		c.pendingReqMu.Unlock()
	})
	return nil
}

// Close 关闭客户端并释放所有资源
func (c *Client) Close() error {
	// 禁用自动重连
	c.autoReconnect.Store(false)

	err := c.Disconnect()
	// 关闭整个客户端生命周期
	c.cancel()

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
	if conn == nil {
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

// sendMessage 发送消息到 core（从 sendQueue 调用）
func (c *Client) sendMessage(msg *Message) error {
	conn := c.getConn()
	if conn == nil {
		return ErrNotConnected
	}
	return conn.sendMessage(msg)
}

// matchTopic 检查主题是否匹配模式
// 支持 * (单层通配符) 和 ** (多层通配符)
func matchTopic(pattern, topic string) bool {
	pi, ti := 0, 0
	plen, tlen := len(pattern), len(topic)

	for pi < plen && ti < tlen {
		// 找 pattern 当前部分
		pEnd := -1
		for i := pi; i < plen; i++ {
			if pattern[i] == '/' {
				pEnd = i
				break
			}
		}
		var pPart string
		if pEnd == -1 {
			pPart = pattern[pi:]
			pEnd = plen
		} else {
			pPart = pattern[pi:pEnd]
		}

		// 找 topic 当前部分
		tEnd := -1
		for i := ti; i < tlen; i++ {
			if topic[i] == '/' {
				tEnd = i
				break
			}
		}
		var tPart string
		if tEnd == -1 {
			tPart = topic[ti:]
			tEnd = tlen
		} else {
			tPart = topic[ti:tEnd]
		}

		switch pPart {
		case "**":
			return true
		case "*":
			// 匹配当前层级
		default:
			if pPart != tPart {
				return false
			}
		}

		pi = pEnd + 1
		ti = tEnd + 1
	}

	// 检查是否都处理完
	if pi >= plen && ti >= tlen {
		return true
	}

	// 模式以 ** 结尾可以匹配空
	if pi < plen && pattern[pi:] == "**" {
		return true
	}

	return false
}
