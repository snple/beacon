// Package client 提供 Queen 协议客户端库
package client

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

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

	// PacketID 相关
	minPacketID = 1
	maxPacketID = 65535

	// 重传相关
	retransmitBatchSize = 50 // 每次从持久化加载的最大消息数
)

// Message 接收到的消息
type Message struct {
	// 基础消息属性
	Topic    string
	Payload  []byte
	QoS      packet.QoS
	Retain   bool
	PacketID uint16

	// 消息元数据
	Priority       packet.Priority
	TraceID        string
	ContentType    string
	UserProperties map[string]string

	// 时间相关
	ExpiryTime int64 // 消息过期时间戳（Unix 秒），0 表示不过期

	// 请求-响应模式属性
	TargetClientID  string // 目标客户端ID
	SourceClientID  string // 来源客户端ID，标识发送者
	ResponseTopic   string // 响应主题
	CorrelationData []byte // 关联数据
}

// WillMessage 遗嘱消息（连接异常断开时由 core 代发）
type WillMessage struct {
	Topic   string
	Payload []byte
	QoS     packet.QoS
	Retain  bool

	Priority       *packet.Priority
	TraceID        string
	ContentType    string
	UserProperties map[string]string

	Expiry time.Duration // 相对过期时间（如 30*time.Second），0 表示不过期

	// 请求-响应模式属性（可选）
	TargetClientID  string
	ResponseTopic   string
	CorrelationData []byte
}

// Client 客户端
type Client struct {
	options *ClientOptions
	conn    net.Conn
	reader  *bufio.Reader
	writer  *bufio.Writer
	writeMu sync.Mutex

	// 状态
	connected      atomic.Bool
	clientID       string
	keepAlive      uint16 // 实际使用的心跳间隔（由服务器确定）
	sessionPresent bool   // 服务端是否恢复了会话（CleanSession=false 且有旧会话时为 true）

	// 包 ID 管理
	nextPacketID atomic.Uint32

	// 订阅主题（轮询模式，不再存储回调）
	subscribedTopics   map[string]bool // topic -> true
	subscribedTopicsMu sync.RWMutex

	// 已注册的 actions（用于重连时重新注册）
	registeredActions map[string]bool // action name -> true
	actionsMu         sync.RWMutex

	// QoS 状态
	pendingAck   map[uint16]chan error
	pendingAckMu sync.Mutex

	// 生命周期
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closeOnce sync.Once

	// 流量控制
	sendWindow uint16      // core 的接收窗口大小（客户端的发送窗口）
	sendQueue  *SendQueue  // 发送队列（基于 core 的 ReceiveWindow）
	processing atomic.Bool // true: 正在处理发送队列

	// REQUEST/RESPONSE 支持
	nextRequestID    atomic.Uint32             // 请求ID生成器
	pendingRequests  map[uint32]chan *Response // requestID -> response channel
	pendingReqMu     sync.Mutex                // 保护 pendingRequests
	requestQueue     chan *Request             // 轮询模式：接收请求的队列
	requestQueueMu   sync.Mutex
	requestQueueInit bool // 请求队列是否已初始化

	// 消息轮询
	messageQueue     chan *Message // 轮询模式：接收消息的队列
	messageQueueMu   sync.Mutex
	messageQueueInit bool // 消息队列是否已初始化

	// 自动重连
	autoReconnect atomic.Bool    // 是否启用自动重连
	reconnecting  atomic.Bool    // 是否正在重连中
	reconnectWG   sync.WaitGroup // 等待重连协程完成

	// 心跳 RTT
	pingSeq      atomic.Uint32 // PING 序号生成器
	lastRTTNanos atomic.Int64  // 最近一次 RTT（纳秒），0 表示未知
	lastPingSeq  atomic.Uint32 // 最近一次发送的 PING 序号
	lastPingSent atomic.Int64  // 最近一次发送 PING 的本地时间戳（Unix 纳秒）

	// QoS1 消息持久化
	store          *MessageStore // 消息持久化存储
	retransmitting atomic.Bool   // true: 正在重传消息

	// 日志
	logger *zap.Logger
}

// LastRTT 返回最近一次心跳 RTT（0 表示未知）
func (c *Client) LastRTT() time.Duration {
	ns := c.lastRTTNanos.Load()
	if ns <= 0 {
		return 0
	}
	return time.Duration(ns)
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
		pendingAck:        make(map[uint16]chan error),
		ctx:               ctx,
		cancel:            cancel,
		logger:            logger,
	}

	// 初始化消息存储
	if opts.StoreConfig != nil && opts.StoreConfig.Enabled {
		if opts.StoreConfig.Logger == nil {
			opts.StoreConfig.Logger = logger
		}
		store, err := NewMessageStore(*opts.StoreConfig)
		if err != nil {
			logger.Error("Failed to initialize message store", zap.Error(err))
			return nil, err
		}

		c.store = store
	}

	return c, nil
}

// New 创建新的客户端（使用默认配置）
func New() (*Client, error) {
	return NewWithOptions(NewClientOptions())
}

func (c *Client) close(err error) error {
	c.closeOnce.Do(func() {
		c.connected.Store(false)
		c.cancel()

		if c.conn != nil {
			c.conn.Close()
		}

		// 清理等待中的 pendingRequests，让它们立即返回错误
		c.pendingReqMu.Lock()
		for _, ch := range c.pendingRequests {
			select {
			case ch <- &Response{
				ReasonCode: packet.ReasonImplementationError,
			}:
			default:
			}
		}
		c.pendingRequests = nil // 清空 map
		c.pendingReqMu.Unlock()

		// 清理等待中的 pendingAck
		c.pendingAckMu.Lock()
		for _, ch := range c.pendingAck {
			select {
			case ch <- errors.New("connection closed"):
			default:
			}
		}
		c.pendingAck = make(map[uint16]chan error) // 重置 map
		c.pendingAckMu.Unlock()

		// 在新协程中等待并执行回调和重连
		go func() {
			c.wg.Wait()

			// 调用 OnDisconnect 钩子
			c.options.Hooks.callOnDisconnect(&DisconnectContext{
				ClientID: c.clientID,
				Packet:   nil, // 客户端主动断开或网络错误时没有 DISCONNECT 包
				Err:      err,
			})

			// 检查是否需要自动重连
			if c.autoReconnect.Load() && !c.reconnecting.Load() {
				c.reconnectWG.Add(1)
				go c.reconnect()
			}
		}()
	})
	return nil
}

// Close 关闭客户端并释放所有资源
func (c *Client) Close() error {
	// 禁用自动重连
	c.autoReconnect.Store(false)

	err := c.Disconnect()

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
	return c.sessionPresent
}

// ClientID 返回客户端 ID
func (c *Client) ClientID() string {
	return c.clientID
}

// GetSendWindow 返回 core 的接收窗口大小（客户端的发送窗口）
func (c *Client) GetSendWindow() uint16 {
	return c.sendWindow
}

// GetSendQueue 返回发送队列（用于监控）
func (c *Client) GetSendQueue() *SendQueue {
	return c.sendQueue
}

// GetMessageStore 返回消息存储（用于监控和测试）
func (c *Client) GetMessageStore() *MessageStore {
	return c.store
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

// allocatePacketID 分配新的 PacketID
func (c *Client) allocatePacketID() uint16 {
	id := c.nextPacketID.Add(1)
	if id == 0 || id > maxPacketID {
		c.nextPacketID.Store(minPacketID)
		id = minPacketID
	}
	return uint16(id)
}

func (c *Client) receiveLoop() {
	defer c.wg.Done()

	for c.connected.Load() {
		// 设置读取超时
		timeout := time.Duration(c.keepAlive) * time.Second * 2
		c.conn.SetReadDeadline(time.Now().Add(timeout))

		pkt, err := packet.ReadPacket(c.reader)
		if err != nil {
			if c.connected.Load() {
				if errors.Is(err, io.EOF) {
					c.close(nil)
				} else {
					c.close(err)
				}
			}
			return
		}

		c.handlePacket(pkt)
	}
}

func (c *Client) handlePacket(pkt packet.Packet) {
	switch p := pkt.(type) {
	case *packet.PublishPacket:
		c.handlePublish(p)
	case *packet.PubackPacket:
		c.handlePuback(p)
	case *packet.SubackPacket:
		c.handleSuback(p)
	case *packet.UnsubackPacket:
		c.handleUnsuback(p)
	case *packet.PongPacket:
		// 心跳响应：优先用 Seq 关联本地发送时间，回退到 Echo
		now := time.Now().UnixNano()
		if p.Seq != 0 && p.Seq == c.lastPingSeq.Load() {
			sent := c.lastPingSent.Load()
			if sent > 0 && now > sent {
				c.lastRTTNanos.Store(now - sent)
				break
			}
		}
		if p.Echo > 0 && uint64(now) > p.Echo {
			c.lastRTTNanos.Store(int64(uint64(now) - p.Echo))
		}
	case *packet.DisconnectPacket:
		c.close(NewServerDisconnectError(p.ReasonCode.String()))
	case *packet.RequestPacket:
		c.handleRequest(p)
	case *packet.ResponsePacket:
		c.handleResponse(p)
	case *packet.RegackPacket:
		c.handleRegack(p)
	case *packet.UnregackPacket:
		c.handleUnregack(p)
	case *packet.AuthPacket:
		c.handleAuth(p)
	case *packet.TracePacket:
		c.handleTrace(p)
	}
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

func (c *Client) keepAliveLoop() {
	defer c.wg.Done()

	interval := time.Duration(c.keepAlive) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if !c.connected.Load() {
				return
			}
			now := time.Now().UnixNano()
			seq := c.pingSeq.Add(1)
			c.lastPingSeq.Store(seq)
			c.lastPingSent.Store(now)
			ping := packet.NewPingPacket(seq, uint64(now))
			if err := c.writePacket(ping); err != nil {
				c.close(err)
				return
			}
		}
	}
}

// sendMessage 发送消息到 core
func (c *Client) sendMessage(qm *QueuedMessage) error {
	pub := packet.NewPublishPacket(qm.Message.Topic, qm.Message.Payload)
	pub.QoS = qm.Message.QoS
	pub.Retain = qm.Message.Retain
	pub.PacketID = qm.Message.PacketID

	// 设置属性
	pub.Properties.Priority = &qm.Message.Priority
	pub.Properties.TraceID = qm.Message.TraceID
	pub.Properties.ContentType = qm.Message.ContentType
	pub.Properties.ExpiryTime = qm.Message.ExpiryTime
	pub.Properties.UserProperties = qm.Message.UserProperties
	pub.Properties.TargetClientID = qm.Message.TargetClientID
	pub.Properties.ResponseTopic = qm.Message.ResponseTopic
	pub.Properties.CorrelationData = qm.Message.CorrelationData

	return c.writePacket(pub)
}

func (c *Client) writePacket(pkt packet.Packet) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if err := packet.WritePacket(c.writer, pkt); err != nil {
		return err
	}
	return c.writer.Flush()
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
