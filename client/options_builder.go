package client

import (
	"crypto/tls"
	"time"

	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// ClientOptions 客户端配置选项（Builder 模式）
type ClientOptions struct {
	// 连接信息
	Core     string // 地址 (如 "localhost:3883" 或 "tls://localhost:8883")
	ClientID string

	// 认证信息
	AuthMethod string // 认证方法 (如 "plain", "jwt", "oauth2" 等)
	AuthData   []byte // 认证数据 (如密码、token 等)

	// TLS 配置
	TLSConfig *tls.Config

	// 连接选项
	KeepAlive      uint16            // 心跳间隔 (秒)
	CleanSession   bool              // 清理会话
	SessionExpiry  uint32            // 会话过期时间 (秒)，0 表示断开即清理
	ConnectTimeout time.Duration     // 连接超时
	PublishTimeout time.Duration     // 发布超时 (默认 30 秒)
	TraceID        string            // 追踪 ID
	UserProperties map[string]string // 连接时发送的用户属性

	// 遗嘱消息（连接非正常断开时由 core 发送）
	Will *WillMessage

	// 消息持久化配置
	StoreConfig *StoreConfig // 消息持久化存储配置，nil 表示不启用持久化

	// 重传配置
	RetransmitInterval time.Duration // QoS 1 消息重传间隔（默认 5 秒）

	// 轮询配置
	RequestQueueSize int // 请求队列缓冲大小（默认 100）
	MessageQueueSize int // 消息队列缓冲大小（默认 100）

	// 日志配置
	Logger *zap.Logger // 可选的日志器，nil 表示静默

	// 钩子处理器
	Hooks Hooks
}

// NewClientOptions 创建新的客户端配置选项
func NewClientOptions() *ClientOptions {
	return &ClientOptions{
		Core:             "localhost:3883",
		KeepAlive:        defaultKeepAlive,
		CleanSession:     true,
		ConnectTimeout:   defaultConnectTimeout,
		PublishTimeout:   defaultPublishTimeout,
		RequestQueueSize: 100,
		MessageQueueSize: 100,
	}
}

// WithCore 设置 Core 地址
func (o *ClientOptions) WithCore(addr string) *ClientOptions {
	o.Core = addr
	return o
}

// WithClientID 设置客户端 ID
func (o *ClientOptions) WithClientID(id string) *ClientOptions {
	o.ClientID = id
	return o
}

// WithAuth 设置认证信息
func (o *ClientOptions) WithAuth(method string, data []byte) *ClientOptions {
	o.AuthMethod = method
	o.AuthData = data
	return o
}

// WithTLSConfig 设置 TLS 配置
func (o *ClientOptions) WithTLSConfig(cfg *tls.Config) *ClientOptions {
	o.TLSConfig = cfg
	return o
}

// WithKeepAlive 设置心跳间隔 (秒)
func (o *ClientOptions) WithKeepAlive(seconds uint16) *ClientOptions {
	o.KeepAlive = seconds
	return o
}

// WithCleanSession 设置是否清理会话
func (o *ClientOptions) WithCleanSession(clean bool) *ClientOptions {
	o.CleanSession = clean
	return o
}

// WithSessionExpiry 设置会话过期时间 (秒)
func (o *ClientOptions) WithSessionExpiry(seconds uint32) *ClientOptions {
	o.SessionExpiry = seconds
	return o
}

// WithConnectTimeout 设置连接超时
func (o *ClientOptions) WithConnectTimeout(d time.Duration) *ClientOptions {
	o.ConnectTimeout = d
	return o
}

// WithPublishTimeout 设置发布超时
func (o *ClientOptions) WithPublishTimeout(d time.Duration) *ClientOptions {
	o.PublishTimeout = d
	return o
}

// WithTraceID 设置 CONNECT 的 TraceID
func (o *ClientOptions) WithTraceID(traceID string) *ClientOptions {
	o.TraceID = traceID
	return o
}

// WithUserProperties 设置 CONNECT 的用户属性
func (o *ClientOptions) WithUserProperties(props map[string]string) *ClientOptions {
	o.UserProperties = props
	return o
}

// WithUserProperty 设置单个 CONNECT 用户属性
func (o *ClientOptions) WithUserProperty(key, value string) *ClientOptions {
	if o.UserProperties == nil {
		o.UserProperties = make(map[string]string)
	}
	o.UserProperties[key] = value
	return o
}

// WithLogger 设置日志器
func (o *ClientOptions) WithLogger(logger *zap.Logger) *ClientOptions {
	o.Logger = logger
	return o
}

// WithHooks 设置钩子集合
func (o *ClientOptions) WithHooks(hooks Hooks) *ClientOptions {
	o.Hooks = hooks
	return o
}

// WithConnectionHandler 设置连接管理钩子
func (o *ClientOptions) WithConnectionHandler(handler ConnectionHandler) *ClientOptions {
	o.Hooks.ConnectionHandler = handler
	return o
}

// WithAuthHandler 设置认证钩子
func (o *ClientOptions) WithAuthHandler(handler AuthHandler) *ClientOptions {
	o.Hooks.AuthHandler = handler
	return o
}

// WithTraceHandler 设置追踪钩子
func (o *ClientOptions) WithTraceHandler(handler TraceHandler) *ClientOptions {
	o.Hooks.TraceHandler = handler
	return o
}

// WithMessageHandler 设置消息钩子
func (o *ClientOptions) WithMessageHandler(handler MessageHandler) *ClientOptions {
	o.Hooks.MessageHandler = handler
	return o
}

// WithRequestHandler 设置请求钩子
func (o *ClientOptions) WithRequestHandler(handler RequestHandler) *ClientOptions {
	o.Hooks.RequestHandler = handler
	return o
}

// WithWill 设置遗嘱消息
func (o *ClientOptions) WithWill(will *WillMessage) *ClientOptions {
	o.Will = will
	return o
}

// WithWillSimple 便捷设置遗嘱消息（最常用字段）
func (o *ClientOptions) WithWillSimple(topic string, payload []byte, qos packet.QoS, retain bool) *ClientOptions {
	o.Will = &WillMessage{
		Packet: &packet.PublishPacket{
			Topic:   topic,
			Payload: payload,
			QoS:     qos,
			Retain:  retain,
		},
	}
	return o
}

// WithStore 设置消息持久化存储配置
func (o *ClientOptions) WithStore(cfg *StoreConfig) *ClientOptions {
	o.StoreConfig = cfg
	return o
}

// WithStoreDataDir 便捷设置持久化存储目录
func (o *ClientOptions) WithStoreDataDir(dataDir string) *ClientOptions {
	if o.StoreConfig == nil {
		cfg := DefaultStoreConfig()
		o.StoreConfig = &cfg
	}
	o.StoreConfig.DataDir = dataDir
	return o
}

// WithRetransmitInterval 设置 QoS1 消息重传间隔
func (o *ClientOptions) WithRetransmitInterval(d time.Duration) *ClientOptions {
	o.RetransmitInterval = d
	return o
}

// WithRequestQueueSize 设置请求队列缓冲大小
func (o *ClientOptions) WithRequestQueueSize(size int) *ClientOptions {
	o.RequestQueueSize = size
	return o
}

// WithMessageQueueSize 设置消息队列缓冲大小
func (o *ClientOptions) WithMessageQueueSize(size int) *ClientOptions {
	o.MessageQueueSize = size
	return o
}

// applyDefaults 应用默认值
func (o *ClientOptions) applyDefaults() {
	if o.Core == "" {
		o.Core = "localhost:3883"
	}
	if o.KeepAlive == 0 {
		o.KeepAlive = defaultKeepAlive
	}
	if o.ConnectTimeout == 0 {
		o.ConnectTimeout = defaultConnectTimeout
	}
	if o.PublishTimeout == 0 {
		o.PublishTimeout = defaultPublishTimeout
	}
	if o.RetransmitInterval == 0 {
		o.RetransmitInterval = defaultRetransmitInterval
	}
	if o.RequestQueueSize == 0 {
		o.RequestQueueSize = 100
	}
	if o.MessageQueueSize == 0 {
		o.MessageQueueSize = 100
	}
}

// PublishOptions 发布选项（Builder 模式）
type PublishOptions struct {
	QoS             packet.QoS
	Retain          bool
	Priority        packet.Priority
	TraceID         string
	ContentType     string
	Expiry          uint32
	Timeout         time.Duration // 超时时间 (0 表示使用配置默认值)
	UserProperties  map[string]string
	TargetClientID  string // 目标客户端ID
	ResponseTopic   string // 响应主题
	CorrelationData []byte // 关联数据
}

// NewPublishOptions 创建新的发布选项
func NewPublishOptions() *PublishOptions {
	return &PublishOptions{
		QoS:      packet.QoS0,
		Priority: packet.PriorityNormal,
	}
}

// WithQoS 设置 QoS 级别
func (o *PublishOptions) WithQoS(qos packet.QoS) *PublishOptions {
	o.QoS = qos
	return o
}

// WithRetain 设置保留标志
func (o *PublishOptions) WithRetain(retain bool) *PublishOptions {
	o.Retain = retain
	return o
}

// WithPriority 设置消息优先级
func (o *PublishOptions) WithPriority(priority packet.Priority) *PublishOptions {
	o.Priority = priority
	return o
}

// WithTraceID 设置追踪 ID
func (o *PublishOptions) WithTraceID(traceID string) *PublishOptions {
	o.TraceID = traceID
	return o
}

// WithContentType 设置内容类型
func (o *PublishOptions) WithContentType(contentType string) *PublishOptions {
	o.ContentType = contentType
	return o
}

// WithExpiry 设置消息过期时间 (秒)
func (o *PublishOptions) WithExpiry(seconds uint32) *PublishOptions {
	o.Expiry = seconds
	return o
}

// WithTimeout 设置发布超时时间
func (o *PublishOptions) WithTimeout(timeout time.Duration) *PublishOptions {
	o.Timeout = timeout
	return o
}

// WithUserProperties 设置用户属性
func (o *PublishOptions) WithUserProperties(props map[string]string) *PublishOptions {
	o.UserProperties = props
	return o
}

// WithUserProperty 设置单个用户属性
func (o *PublishOptions) WithUserProperty(key, value string) *PublishOptions {
	if o.UserProperties == nil {
		o.UserProperties = make(map[string]string)
	}
	o.UserProperties[key] = value
	return o
}

// WithTargetClientID 设置目标客户端ID（用于点对点消息）
func (o *PublishOptions) WithTargetClientID(clientID string) *PublishOptions {
	o.TargetClientID = clientID
	return o
}

// WithResponseTopic 设置响应主题
func (o *PublishOptions) WithResponseTopic(topic string) *PublishOptions {
	o.ResponseTopic = topic
	return o
}

// WithCorrelationData 设置关联数据
func (o *PublishOptions) WithCorrelationData(data []byte) *PublishOptions {
	o.CorrelationData = data
	return o
}

// SubscribeOptions 订阅选项（Builder 模式）
type SubscribeOptions struct {
	QoS               packet.QoS
	NoLocal           bool
	RetainAsPublished bool
}

// NewSubscribeOptions 创建新的订阅选项
func NewSubscribeOptions() *SubscribeOptions {
	return &SubscribeOptions{
		QoS: packet.QoS0,
	}
}

// WithQoS 设置订阅 QoS
func (o *SubscribeOptions) WithQoS(qos packet.QoS) *SubscribeOptions {
	o.QoS = qos
	return o
}

// WithNoLocal 不接收自己发布的消息
func (o *SubscribeOptions) WithNoLocal(noLocal bool) *SubscribeOptions {
	o.NoLocal = noLocal
	return o
}

// WithRetainAsPublished 保留消息按原样发送
func (o *SubscribeOptions) WithRetainAsPublished(rap bool) *SubscribeOptions {
	o.RetainAsPublished = rap
	return o
}
