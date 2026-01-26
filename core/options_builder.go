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
	MaxClients     uint32 // 最大客户端数
	MaxPacketSize  uint32 // 最大包大小
	ConnectTimeout uint32 // 连接超时 (秒)
	KeepAlive      uint16 // 服务器心跳间隔 (秒)，0 表示使用客户端的值

	// 会话管理
	MaxSessionTimeout uint32 // 最大会话保留时间（秒），0 表示断开即清理

	// 流量控制
	ReceiveWindow uint16 // core 可接收的消息窗口大小 (双向 FLOW 用)

	// 消息过期
	DefaultMessageExpiry time.Duration // 默认消息过期时间（0 表示永不过期）
	ExpiredCheckInterval time.Duration // 过期消息检查间隔

	// 请求超时
	DefaultRequestTimeout time.Duration // 默认请求超时（默认 30 秒）

	// QoS 重传
	RetransmitInterval time.Duration // QoS 1 消息重传间隔（默认 30 秒）

	// 钩子处理器
	Hooks Hooks // 钩子集合（包含认证处理器）

	// 功能开关
	RetainEnabled    bool   // 启用保留消息
	RequestQueueSize uint16 // REQUEST 轮询队列缓冲大小（默认 100）
	MessageQueueSize uint16 // MESSAGE 轮询队列缓冲大小（默认 100）

	// 持久化存储
	StoreOptions StoreOptions // 消息持久化配置

	// 日志
	Logger *zap.Logger
}

// NewCoreOptions 创建新的 Core 配置选项
func NewCoreOptions() *CoreOptions {
	return &CoreOptions{
		MaxClients:            10000,
		MaxPacketSize:         1048576, // 1MB
		ConnectTimeout:        10,
		KeepAlive:             60,
		MaxSessionTimeout:     3600,
		ReceiveWindow:         1000,
		DefaultMessageExpiry:  24 * time.Hour,
		ExpiredCheckInterval:  180 * time.Second,
		DefaultRequestTimeout: 30 * time.Second,
		RetransmitInterval:    30 * time.Second,
		RetainEnabled:         true,
		RequestQueueSize:      100,
		MessageQueueSize:      100,
		StoreOptions:          DefaultStoreOptions(),
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

// WithRequestHandler 设置请求处理钩子
func (o *CoreOptions) WithRequestHandler(handler RequestHandler) *CoreOptions {
	o.Hooks.RequestHandler = handler
	return o
}

// WithRetainEnabled 设置是否启用保留消息
func (o *CoreOptions) WithRetainEnabled(enabled bool) *CoreOptions {
	o.RetainEnabled = enabled
	return o
}

// WithRequestQueueSize 设置 REQUEST 轮询队列的缓冲大小
func (o *CoreOptions) WithRequestQueueSize(size uint16) *CoreOptions {
	o.RequestQueueSize = size
	return o
}

// WithMessageQueueSize 设置 MESSAGE 轮询队列的缓冲大小
func (o *CoreOptions) WithMessageQueueSize(size uint16) *CoreOptions {
	o.MessageQueueSize = size
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
}

// Validate 验证配置的有效性
func (o *CoreOptions) Validate() error {
	// 验证连接限制
	if o.MaxClients <= 0 {
		return fmt.Errorf("maxClients must be >= 0, got %d", o.MaxClients)
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

	return nil
}
