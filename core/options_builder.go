package core

import (
	"fmt"
	"time"

	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// CoreOptions Core 配置选项（Builder 模式）
type CoreOptions struct {
	// 连接限制
	MaxClients     uint32        // 最大客户端数
	MaxPacketSize  uint32        // 最大包大小
	ConnectTimeout uint32        // 连接超时 (秒)
	KeepAlive      uint16        // 服务器心跳间隔 (秒)，0 表示使用客户端的值
	WriteTimeout   time.Duration // 单次写超时（默认 10 秒，0 表示不设置写截止时间）

	// 会话管理
	MaxSessionTimeout uint32 // 会话保留时间的上限（秒），0 表示不限制（使用客户端请求值）

	// 流量控制
	ReceiveWindow uint16 // core 可接收的消息窗口大小 (双向 FLOW 用)

	// 消息过期
	DefaultMessageExpiry time.Duration // 默认消息过期时间（0 表示永不过期）
	ExpiredCheckInterval time.Duration // 过期消息检查间隔

	// QoS 重传
	RetransmitInterval time.Duration // QoS 1 消息重传间隔（默认 30 秒）

	// 钩子处理器
	Hooks Hooks // 钩子集合（包含认证处理器）

	// 功能开关
	RetainEnabled bool // 启用保留消息

	// 连接速率限制
	ConnectionRateLimit float64 // 每秒最大新连接数（0 表示不限制）
	ConnectionRateBurst int     // 连接速率突发容量（默认等于 ConnectionRateLimit）

	// 配额与背压
	MaxClientQueueSize        int                 // 每个客户端待发送队列上限（条），0 表示不限制
	ClientQueueOverflowPolicy QueueOverflowPolicy // 客户端队列满时的溢出策略
	MaxPendingMessages        int                 // 全局分发队列上限（条），0 表示不限制
	MaxPublishRate            float64             // 每客户端每秒最大发布消息数，0 表示不限制
	MaxPublishBurst           int                 // 每客户端发布速率突发容量（默认取 rate）
	MaxStoreBytes             int64               // 存储占用水位（字节），0 表示不限制
	MaxOfflineSessions        int                 // 保留中的离线会话上限（个），0 表示不限制

	// 持久化存储
	StoreOptions StoreOptions // 消息持久化配置

	// 日志
	Logger *zap.Logger
}

// QueueOverflowPolicy 客户端待发送队列满时的溢出策略
type QueueOverflowPolicy uint8

const (
	// QueueOverflowDropNewest 丢弃新到达的消息（默认）
	QueueOverflowDropNewest QueueOverflowPolicy = iota
	// QueueOverflowDropOldest 丢弃队列中最旧的消息，保留最新消息
	QueueOverflowDropOldest
)

func (p QueueOverflowPolicy) String() string {
	switch p {
	case QueueOverflowDropOldest:
		return "drop-oldest"
	default:
		return "drop-newest"
	}
}

// NewCoreOptions 创建新的 Core 配置选项
func NewCoreOptions() *CoreOptions {
	return &CoreOptions{
		MaxClients:           10000,
		MaxPacketSize:        1048576, // 1MB
		ConnectTimeout:       10,
		KeepAlive:            60,
		WriteTimeout:         10 * time.Second,
		MaxSessionTimeout:    3600,
		ReceiveWindow:        1000,
		DefaultMessageExpiry: 24 * time.Hour,
		ExpiredCheckInterval: 180 * time.Second,
		RetransmitInterval:   30 * time.Second,
		RetainEnabled:        true,
		StoreOptions:         DefaultStoreOptions(),

		// 配额与背压默认值
		MaxClientQueueSize:        10000,
		ClientQueueOverflowPolicy: QueueOverflowDropNewest,
		MaxPendingMessages:        100000,
		MaxOfflineSessions:        10000,
	}
}

// WithMaxClients 设置最大客户端数
func (o *CoreOptions) WithMaxClients(n uint32) *CoreOptions {
	o.MaxClients = n
	return o
}

// WithMaxPacketSize 设置最大包大小
func (o *CoreOptions) WithMaxPacketSize(size uint32) *CoreOptions {
	o.MaxPacketSize = size
	return o
}

// WithConnectTimeout 设置连接超时 (秒)
func (o *CoreOptions) WithConnectTimeout(seconds uint32) *CoreOptions {
	o.ConnectTimeout = seconds
	return o
}

// WithKeepAlive 设置服务器心跳间隔 (秒)
func (o *CoreOptions) WithKeepAlive(seconds uint16) *CoreOptions {
	o.KeepAlive = seconds
	return o
}

// WithWriteTimeout 设置单次网络写超时。超时或写失败会关闭该连接，
// 未确认的消息仍保留在持久化队列中等待重连补发。0 表示不设置写截止时间。
func (o *CoreOptions) WithWriteTimeout(d time.Duration) *CoreOptions {
	o.WriteTimeout = d
	return o
}

// WithMaxSessionTimeout 设置最大会话保留时间 (秒)
func (o *CoreOptions) WithMaxSessionTimeout(seconds uint32) *CoreOptions {
	o.MaxSessionTimeout = seconds
	return o
}

// WithReceiveWindow 设置 core 可接收的消息窗口大小
func (o *CoreOptions) WithReceiveWindow(size uint16) *CoreOptions {
	o.ReceiveWindow = size
	return o
}

// WithDefaultMessageExpiry 设置默认消息过期时间
func (o *CoreOptions) WithDefaultMessageExpiry(d time.Duration) *CoreOptions {
	o.DefaultMessageExpiry = d
	return o
}

// WithExpiredCheckInterval 设置过期消息检查间隔
func (o *CoreOptions) WithExpiredCheckInterval(d time.Duration) *CoreOptions {
	o.ExpiredCheckInterval = d
	return o
}

// WithRetransmitInterval 设置 QoS 1 消息重传间隔
func (o *CoreOptions) WithRetransmitInterval(d time.Duration) *CoreOptions {
	o.RetransmitInterval = d
	return o
}

// WithHooks 设置钩子集合
func (o *CoreOptions) WithHooks(hooks Hooks) *CoreOptions {
	o.Hooks = hooks
	return o
}

// WithMessageHandler 设置消息处理钩子
func (o *CoreOptions) WithMessageHandler(handler MessageHandler) *CoreOptions {
	o.Hooks.MessageHandler = handler
	return o
}

// WithConnectionHandler 设置连接管理钩子
func (o *CoreOptions) WithConnectionHandler(handler ConnectionHandler) *CoreOptions {
	o.Hooks.ConnectionHandler = handler
	return o
}

// WithSubscriptionHandler 设置订阅管理钩子
func (o *CoreOptions) WithSubscriptionHandler(handler SubscriptionHandler) *CoreOptions {
	o.Hooks.SubscriptionHandler = handler
	return o
}

// WithAuthHandler 设置认证钩子
func (o *CoreOptions) WithAuthHandler(handler AuthHandler) *CoreOptions {
	o.Hooks.AuthHandler = handler
	return o
}

// WithTraceHandler 设置追踪钩子
func (o *CoreOptions) WithTraceHandler(handler TraceHandler) *CoreOptions {
	o.Hooks.TraceHandler = handler
	return o
}

// WithRetainEnabled 设置是否启用保留消息
func (o *CoreOptions) WithRetainEnabled(enabled bool) *CoreOptions {
	o.RetainEnabled = enabled
	return o
}

// WithConnectionRateLimit 设置每秒最大新连接数（0 表示不限制）
func (o *CoreOptions) WithConnectionRateLimit(rate float64, burst int) *CoreOptions {
	o.ConnectionRateLimit = rate
	o.ConnectionRateBurst = burst
	return o
}

// WithMaxClientQueueSize 设置每个客户端待发送队列上限（条），并指定溢出策略。
// size 为 0 表示不限制；默认 10000、丢弃新消息。
func (o *CoreOptions) WithMaxClientQueueSize(size int, policy QueueOverflowPolicy) *CoreOptions {
	o.MaxClientQueueSize = size
	o.ClientQueueOverflowPolicy = policy
	return o
}

// WithMaxPendingMessages 设置全局分发队列上限（条），0 表示不限制。
// 超过上限时新的发布会被拒绝（QoS1 返回 QuotaExceeded）。
func (o *CoreOptions) WithMaxPendingMessages(n int) *CoreOptions {
	o.MaxPendingMessages = n
	return o
}

// WithMaxOfflineSessions 设置保留中的离线会话上限，0 表示不限制。
// 超限时驱逐最早过期的离线会话（会话记录、订阅与待投递队列一并清理）。
func (o *CoreOptions) WithMaxOfflineSessions(n int) *CoreOptions {
	o.MaxOfflineSessions = n
	return o
}

// WithMaxPublishRate 设置每个客户端每秒最大发布消息数，0 表示不限制。
// 超限时 QoS1 消息返回 MessageRateTooHigh，QoS0 消息被丢弃。
func (o *CoreOptions) WithMaxPublishRate(rate float64, burst int) *CoreOptions {
	o.MaxPublishRate = rate
	o.MaxPublishBurst = burst
	return o
}

// WithMaxStoreBytes 设置存储占用水位（字节），0 表示不限制。
// 超过水位后新的发布会被拒绝（QoS1 返回 QuotaExceeded），避免写满磁盘。
// 水位基于 badger 上报的磁盘占用，存在数秒滞后，属于粗粒度保护而非精确配额。
func (o *CoreOptions) WithMaxStoreBytes(bytes int64) *CoreOptions {
	o.MaxStoreBytes = bytes
	return o
}

// WithStore 设置持久化存储配置
func (o *CoreOptions) WithStore(storeOptions StoreOptions) *CoreOptions {
	o.StoreOptions = storeOptions
	return o
}

// WithStoreDir 设置持久化存储目录
func (o *CoreOptions) WithStoreDir(dir string) *CoreOptions {
	o.StoreOptions.DataDir = dir
	return o
}

// WithLogger 设置日志器
func (o *CoreOptions) WithLogger(logger *zap.Logger) *CoreOptions {
	o.Logger = logger
	return o
}

// applyDefaults 应用默认值
func (o *CoreOptions) applyDefaults() {
	if o.Logger == nil {
		o.Logger, _ = zap.NewDevelopment()
	}

	if o.MaxPublishRate > 0 && o.MaxPublishBurst <= 0 {
		o.MaxPublishBurst = max(1, int(o.MaxPublishRate))
	}
}

// Validate 验证配置的有效性
func (o *CoreOptions) Validate() error {
	// 验证连接限制
	if o.MaxClients <= 0 {
		return fmt.Errorf("maxClients must be > 0, got %d", o.MaxClients)
	}
	if o.MaxPacketSize <= 0 {
		return fmt.Errorf("maxPacketSize must be > 0, got %d", o.MaxPacketSize)
	}
	if o.MaxPacketSize > packet.MaxPacketSize {
		return fmt.Errorf("maxPacketSize must be <= %d, got %d", packet.MaxPacketSize, o.MaxPacketSize)
	}
	if o.ConnectTimeout <= 0 {
		return fmt.Errorf("connectTimeout must be >= 0, got %d", o.ConnectTimeout)
	}

	// 验证流量控制
	if o.ReceiveWindow <= 0 {
		return fmt.Errorf("receiveWindow must be > 0, got %d", o.ReceiveWindow)
	}

	// 验证写超时
	if o.WriteTimeout < 0 {
		return fmt.Errorf("writeTimeout must be >= 0, got %v", o.WriteTimeout)
	}

	// 验证消息过期设置
	if o.ExpiredCheckInterval < 0 {
		return fmt.Errorf("expiredCheckInterval must be >= 0, got %v", o.ExpiredCheckInterval)
	}
	if o.ExpiredCheckInterval > 0 && o.ExpiredCheckInterval < 10*time.Second {
		return fmt.Errorf("expiredCheckInterval must be >= 10s or 0 to disable, got %v", o.ExpiredCheckInterval)
	}

	// 验证 QoS 重传设置
	if o.RetransmitInterval <= 0 {
		return fmt.Errorf("retransmitInterval must be > 0, got %v", o.RetransmitInterval)
	}
	if o.RetransmitInterval < time.Second {
		return fmt.Errorf("retransmitInterval must be >= 1s, got %v", o.RetransmitInterval)
	}

	// 验证存储配置
	if err := o.StoreOptions.Validate(); err != nil {
		return fmt.Errorf("store options validation failed: %w", err)
	}

	// 验证配额配置
	if o.MaxClientQueueSize < 0 {
		return fmt.Errorf("maxClientQueueSize must be >= 0, got %d", o.MaxClientQueueSize)
	}
	if o.MaxPendingMessages < 0 {
		return fmt.Errorf("maxPendingMessages must be >= 0, got %d", o.MaxPendingMessages)
	}
	if o.MaxPublishRate < 0 {
		return fmt.Errorf("maxPublishRate must be >= 0, got %v", o.MaxPublishRate)
	}
	if o.MaxPublishBurst < 0 {
		return fmt.Errorf("maxPublishBurst must be >= 0, got %d", o.MaxPublishBurst)
	}
	if o.MaxStoreBytes < 0 {
		return fmt.Errorf("maxStoreBytes must be >= 0, got %d", o.MaxStoreBytes)
	}
	if o.MaxOfflineSessions < 0 {
		return fmt.Errorf("maxOfflineSessions must be >= 0, got %d", o.MaxOfflineSessions)
	}
	switch o.ClientQueueOverflowPolicy {
	case QueueOverflowDropNewest, QueueOverflowDropOldest:
	default:
		return fmt.Errorf("unknown clientQueueOverflowPolicy: %d", o.ClientQueueOverflowPolicy)
	}

	return nil
}
