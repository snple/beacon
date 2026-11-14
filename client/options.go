package client

import (
	"fmt"
	"maps"
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// ClientOptions 客户端配置（Builder 模式）
type ClientOptions struct {
	// 服务端地址（TCP 或 ws:// / wss://），也可通过 WithDialer 指定
	Address string
	// Dialer 自定义连接方式（TCPAddress 会被忽略）
	Dialer Dialer

	ClientID     string
	KeepAlive    uint16 // 秒
	CleanSession bool   // 建连时要求服务端丢弃该 ClientID 的既有会话
	// MaxPacketSize 是本端可接收的最大包（CONNECT.MaxPacketSize），
	// 只限制读取；发送上限由 MaxSendPacketSize / 服务端 CONNACK 决定。
	MaxPacketSize uint32
	// MaxSendPacketSize 是发送上限；0 表示采用服务端 CONNACK.Limits.MaxPacketSize。
	MaxSendPacketSize uint32
	ConnectTimeout    time.Duration
	WriteTimeout      time.Duration

	// 重连
	AutoReconnect bool
	ReconnectMin  time.Duration
	ReconnectMax  time.Duration

	// Channels 是建连后自动 DECLARE 的声明（含重连）。
	// CONNECT 本身不携带订阅声明。
	Channels []Channel

	// 拉取默认长轮询时长（PollMessage 使用）
	DefaultWaitMs uint32

	// 认证
	AuthMethod string
	AuthData   []byte

	// Properties 是随 CONNECT 发送给服务端的键值信息，broker 本身不解释。
	Properties map[string]string

	Logger *zap.Logger
}

// NewClientOptions 创建默认配置。
func NewClientOptions() *ClientOptions {
	return &ClientOptions{
		KeepAlive:      packet.DefaultKeepAlive,
		MaxPacketSize:  packet.DefaultMaxPacketSize,
		ConnectTimeout: 10 * time.Second,
		WriteTimeout:   10 * time.Second,
		AutoReconnect:  true,
		ReconnectMin:   500 * time.Millisecond,
		ReconnectMax:   30 * time.Second,
		DefaultWaitMs:  30000,
		Logger:         zap.NewNop(),
	}
}

func (o *ClientOptions) applyDefaults() {
	def := NewClientOptions()
	if o.KeepAlive == 0 {
		o.KeepAlive = def.KeepAlive
	}
	if o.MaxPacketSize == 0 {
		o.MaxPacketSize = def.MaxPacketSize
	}
	if o.ConnectTimeout == 0 {
		o.ConnectTimeout = def.ConnectTimeout
	}
	if o.WriteTimeout == 0 {
		o.WriteTimeout = def.WriteTimeout
	}
	if o.ReconnectMin == 0 {
		o.ReconnectMin = def.ReconnectMin
	}
	if o.ReconnectMax == 0 {
		o.ReconnectMax = def.ReconnectMax
	}
	if o.Dialer == nil && o.Address == "" {
		o.Address = "127.0.0.1:5208"
	}
	if len(o.Channels) > 0 {
		o.Channels = append([]Channel(nil), o.Channels...)
	}
	if o.Logger == nil {
		o.Logger = def.Logger
	}
}

// Validate 校验配置。
//
// ClientID 允许为空（服务端会在 CONNACK 中分配 AssignedClientID）；
// 非空时必须符合协议约束。
func (o *ClientOptions) Validate() error {
	if o.ClientID != "" {
		if len(o.ClientID) > packet.MaxClientIDLength {
			return fmt.Errorf("clientID too long")
		}
		if !packet.ValidateClientID(o.ClientID) {
			return fmt.Errorf("invalid clientID %q", o.ClientID)
		}
	}
	if o.KeepAlive == 0 {
		return fmt.Errorf("keepAlive must be > 0")
	}
	if o.MaxPacketSize < packet.MinPacketSize || o.MaxPacketSize > packet.MaxPacketSize {
		return fmt.Errorf("maxPacketSize must be in [%d, %d]", packet.MinPacketSize, packet.MaxPacketSize)
	}
	if o.MaxSendPacketSize != 0 &&
		(o.MaxSendPacketSize < packet.MinPacketSize || o.MaxSendPacketSize > packet.MaxPacketSize) {
		return fmt.Errorf("maxSendPacketSize must be 0 or in [%d, %d]",
			packet.MinPacketSize, packet.MaxPacketSize)
	}
	return nil
}

// WithCore 设置服务端地址。
func (o *ClientOptions) WithCore(address string) *ClientOptions { o.Address = address; return o }

// WithDialer 设置自定义拨号器。
func (o *ClientOptions) WithDialer(d Dialer) *ClientOptions { o.Dialer = d; return o }

// WithClientID 设置客户端 ID。
func (o *ClientOptions) WithClientID(id string) *ClientOptions { o.ClientID = id; return o }

// WithKeepAlive 设置心跳间隔（秒）。
func (o *ClientOptions) WithKeepAlive(s uint16) *ClientOptions { o.KeepAlive = s; return o }

// WithCleanSession 设置建连时是否清理该 ClientID 的既有会话。
func (o *ClientOptions) WithCleanSession(clean bool) *ClientOptions {
	o.CleanSession = clean
	return o
}

// WithMaxPacketSize 设置本端可接收的最大包大小（只影响读取）。
func (o *ClientOptions) WithMaxPacketSize(n uint32) *ClientOptions { o.MaxPacketSize = n; return o }

// WithMaxSendPacketSize 设置发送上限（0 = 采用服务端 CONNACK 的接收上限）。
func (o *ClientOptions) WithMaxSendPacketSize(n uint32) *ClientOptions {
	o.MaxSendPacketSize = n
	return o
}

// WithProperties 设置随 CONNECT 发送给服务端的键值信息。
func (o *ClientOptions) WithProperties(props map[string]string) *ClientOptions {
	o.Properties = maps.Clone(props)
	return o
}

// WithProperty 设置一项随 CONNECT 发送的键值信息。
func (o *ClientOptions) WithProperty(key, value string) *ClientOptions {
	if o.Properties == nil {
		o.Properties = make(map[string]string)
	}
	o.Properties[key] = value
	return o
}

// WithConnectTimeout 设置连接超时。
func (o *ClientOptions) WithConnectTimeout(d time.Duration) *ClientOptions {
	o.ConnectTimeout = d
	return o
}

// WithWriteTimeout 设置写超时。
func (o *ClientOptions) WithWriteTimeout(d time.Duration) *ClientOptions {
	o.WriteTimeout = d
	return o
}

// WithAutoReconnect 设置是否自动重连。
func (o *ClientOptions) WithAutoReconnect(enable bool) *ClientOptions {
	o.AutoReconnect = enable
	return o
}

// WithReconnectBackoff 设置重连退避区间。
func (o *ClientOptions) WithReconnectBackoff(min, max time.Duration) *ClientOptions {
	o.ReconnectMin, o.ReconnectMax = min, max
	return o
}

// WithChannels 设置建连后自动声明的过滤器（每次连接/重连都会重新 DECLARE）。
//
// 这些过滤器同时接收广播与定向投递；只需要定向投递时用 WithTargetOnlyChannels。
func (o *ClientOptions) WithChannels(channels ...string) *ClientOptions {
	o.Channels = make([]Channel, len(channels))
	for i, ch := range channels {
		o.Channels[i] = Channel{Filter: ch}
	}
	return o
}

// WithTargetOnlyChannels 追加只接收定向投递的过滤器（不参与广播匹配）。
func (o *ClientOptions) WithTargetOnlyChannels(channels ...string) *ClientOptions {
	for _, ch := range channels {
		o.Channels = append(o.Channels, Channel{Filter: ch, TargetOnly: true})
	}
	return o
}

// WithDefaultWait 设置拉取的默认长轮询时长（毫秒）。
func (o *ClientOptions) WithDefaultWait(waitMs uint32) *ClientOptions {
	o.DefaultWaitMs = waitMs
	return o
}

// WithAuth 设置认证信息。
func (o *ClientOptions) WithAuth(method string, data []byte) *ClientOptions {
	o.AuthMethod, o.AuthData = method, data
	return o
}

// WithLogger 设置日志器。
func (o *ClientOptions) WithLogger(l *zap.Logger) *ClientOptions {
	if l != nil {
		o.Logger = l
	}
	return o
}
