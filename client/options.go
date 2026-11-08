package client

import (
	"fmt"
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

	ClientID       string
	KeepAlive      uint16 // 秒
	CleanSession   bool   // 建连时要求服务端丢弃该 ClientID 的既有会话
	MaxPacketSize  uint32
	ConnectTimeout time.Duration
	WriteTimeout   time.Duration

	// 重连
	AutoReconnect bool
	ReconnectMin  time.Duration
	ReconnectMax  time.Duration

	// Channels 是建连后自动 DECLARE 的过滤器（含重连）。
	// CONNECT 本身不携带订阅声明。
	Channels []string

	// 拉取默认长轮询时长（PollMessage 使用）
	DefaultWaitMs uint32

	// 认证
	AuthMethod string
	AuthData   []byte

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
		o.Channels = append([]string(nil), o.Channels...)
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
	if o.MaxPacketSize < 64 {
		return fmt.Errorf("maxPacketSize must be >= 64")
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

// WithMaxPacketSize 设置最大包大小。
func (o *ClientOptions) WithMaxPacketSize(n uint32) *ClientOptions { o.MaxPacketSize = n; return o }

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
func (o *ClientOptions) WithChannels(channels ...string) *ClientOptions {
	o.Channels = channels
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
