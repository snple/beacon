package client

import (
	"crypto/tls"
	"fmt"
	"maps"
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// ClientOptions 是客户端配置。
//
// NewWithOptions 会克隆该配置后再使用，调用方在构造后改动它不会影响已创建的客户端。
type ClientOptions struct {
	// Address 是服务端地址：host:port、tls://host:port、ws://… 或 wss://…
	Address string
	// Dialer 是自定义连接方式；非 nil 时忽略 Address。
	Dialer Dialer
	// TLSConfig 是 tls:// 与 wss:// 连接的 TLS 参数；nil 使用系统默认。
	// 未显式设置 ServerName 时按地址推导。
	TLSConfig *tls.Config

	// ClientID 是客户端 ID；为空时由服务端分配，并在重连时沿用。
	ClientID string
	// KeepAlive 是心跳间隔（秒）。
	KeepAlive uint16
	// CleanSession 为 true 时要求服务端丢弃该 ClientID 的既有会话。
	CleanSession bool
	// MaxRecvPacketSize 是本端可接收的最大包（CONNECT.MaxPacketSize）。
	MaxRecvPacketSize uint32
	// MaxSendPacketSize 是本端发送上限；0 表示采用服务端 CONNACK 声明的限制。
	MaxSendPacketSize uint32
	// ConnectTimeout 是建立连接（含 TLS/WebSocket 握手）的超时。
	ConnectTimeout time.Duration
	// WriteTimeout 是单次写超时。
	WriteTimeout time.Duration

	// AutoReconnect 为 true 时断线后自动重连。
	AutoReconnect bool
	// ReconnectMin 是首次重连退避。
	ReconnectMin time.Duration
	// ReconnectMax 是重连退避上限。
	ReconnectMax time.Duration

	// Channels 是建连与每次重连后自动 DECLARE 的声明。
	Channels []Channel

	// DefaultPullWaitMs 是 Pull 的默认长轮询时长（毫秒）；0 表示不等待。
	DefaultPullWaitMs uint32

	// AuthMethod 是 CONNECT 携带的认证方法。
	AuthMethod string
	// AuthData 是 CONNECT 携带的认证数据。
	AuthData []byte
	// Properties 是 CONNECT 携带的属性。
	Properties map[string]string

	// Logger 是客户端日志器；nil 使用空实现。
	Logger *zap.Logger
}

// NewClientOptions 创建默认配置。
func NewClientOptions() *ClientOptions {
	return &ClientOptions{
		KeepAlive:         packet.DefaultKeepAlive,
		MaxRecvPacketSize: packet.DefaultMaxPacketSize,
		ConnectTimeout:    10 * time.Second,
		WriteTimeout:      10 * time.Second,
		AutoReconnect:     true,
		ReconnectMin:      500 * time.Millisecond,
		ReconnectMax:      30 * time.Second,
		DefaultPullWaitMs: 30000,
		Logger:            zap.NewNop(),
	}
}

// clone 深拷贝配置，保证客户端不受调用方的后续改动影响。
func (o *ClientOptions) clone() *ClientOptions {
	c := *o
	c.Channels = append([]Channel(nil), o.Channels...)
	c.Properties = maps.Clone(o.Properties)
	c.AuthData = append([]byte(nil), o.AuthData...)
	if o.TLSConfig != nil {
		c.TLSConfig = o.TLSConfig.Clone()
	}
	return &c
}

func (o *ClientOptions) applyDefaults() {
	def := NewClientOptions()
	if o.KeepAlive == 0 {
		o.KeepAlive = def.KeepAlive
	}
	if o.MaxRecvPacketSize == 0 {
		o.MaxRecvPacketSize = def.MaxRecvPacketSize
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
	if o.Logger == nil {
		o.Logger = def.Logger
	}
	o.Channels = mergeChannels(o.Channels)
}

// Validate 校验配置。
//
// ClientID 允许为空（服务端会在 CONNACK 中分配 AssignedClientID）；
// 非空时必须符合协议约束。声明过滤器在本地先行校验，避免把明显非法
// 的请求发到服务端。
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
	if o.MaxRecvPacketSize < packet.MinPacketSize || o.MaxRecvPacketSize > packet.MaxPacketSize {
		return fmt.Errorf("maxRecvPacketSize must be in [%d, %d]", packet.MinPacketSize, packet.MaxPacketSize)
	}
	if o.MaxSendPacketSize != 0 &&
		(o.MaxSendPacketSize < packet.MinPacketSize || o.MaxSendPacketSize > packet.MaxPacketSize) {
		return fmt.Errorf("maxSendPacketSize must be 0 or in [%d, %d]",
			packet.MinPacketSize, packet.MaxPacketSize)
	}
	if o.ConnectTimeout < 0 || o.WriteTimeout < 0 {
		return fmt.Errorf("timeouts must not be negative")
	}
	if o.AutoReconnect {
		if o.ReconnectMin <= 0 || o.ReconnectMax <= 0 {
			return fmt.Errorf("reconnect backoff must be > 0")
		}
		if o.ReconnectMin > o.ReconnectMax {
			return fmt.Errorf("reconnectMin must not exceed reconnectMax")
		}
	}
	for _, ch := range o.Channels {
		if !packet.ValidateChannelFilterLimit(ch.Filter, packet.MaxChannelLength) {
			return fmt.Errorf("invalid channel filter %q", ch.Filter)
		}
	}
	return nil
}

// WithServer 设置服务端地址。
func (o *ClientOptions) WithServer(address string) *ClientOptions { o.Address = address; return o }

// WithDialer 设置自定义拨号器（非 nil 时忽略 Address）。
func (o *ClientOptions) WithDialer(d Dialer) *ClientOptions { o.Dialer = d; return o }

// WithTLSConfig 设置 tls:// 与 wss:// 连接使用的 TLS 配置。
func (o *ClientOptions) WithTLSConfig(cfg *tls.Config) *ClientOptions {
	if cfg != nil {
		o.TLSConfig = cfg
	}
	return o
}

// WithClientID 设置客户端 ID。
func (o *ClientOptions) WithClientID(id string) *ClientOptions { o.ClientID = id; return o }

// WithKeepAlive 设置心跳间隔（秒）。
func (o *ClientOptions) WithKeepAlive(s uint16) *ClientOptions { o.KeepAlive = s; return o }

// WithCleanSession 设置建连时是否清理该 ClientID 的既有会话。
func (o *ClientOptions) WithCleanSession(clean bool) *ClientOptions {
	o.CleanSession = clean
	return o
}

// WithMaxRecvPacketSize 设置本端可接收的最大包大小。
func (o *ClientOptions) WithMaxRecvPacketSize(n uint32) *ClientOptions {
	o.MaxRecvPacketSize = n
	return o
}

// WithMaxSendPacketSize 设置发送上限（0 = 采用服务端 CONNACK 的接收上限）。
func (o *ClientOptions) WithMaxSendPacketSize(n uint32) *ClientOptions {
	o.MaxSendPacketSize = n
	return o
}

// WithConnectTimeout 设置建连超时。
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

// WithChannels 用给定声明替换默认声明（建连与每次重连后自动 DECLARE）。
//
// 只需要追加时用 AddChannels。可用 Filter/TargetOnly 构造声明。
func (o *ClientOptions) WithChannels(channels ...Channel) *ClientOptions {
	o.Channels = append([]Channel(nil), channels...)
	return o
}

// AddChannels 追加默认声明。
func (o *ClientOptions) AddChannels(channels ...Channel) *ClientOptions {
	o.Channels = append(o.Channels, channels...)
	return o
}

// WithDefaultPullWait 设置 Pull 的默认长轮询时长（毫秒）。
func (o *ClientOptions) WithDefaultPullWait(waitMs uint32) *ClientOptions {
	o.DefaultPullWaitMs = waitMs
	return o
}

// WithAuth 设置认证信息。
func (o *ClientOptions) WithAuth(method string, data []byte) *ClientOptions {
	o.AuthMethod = method
	o.AuthData = append([]byte(nil), data...)
	return o
}

// WithProperties 设置随 CONNECT 发送的属性。
func (o *ClientOptions) WithProperties(props map[string]string) *ClientOptions {
	o.Properties = maps.Clone(props)
	return o
}

// WithProperty 设置一项随 CONNECT 发送的属性。
func (o *ClientOptions) WithProperty(key, value string) *ClientOptions {
	if o.Properties == nil {
		o.Properties = make(map[string]string)
	}
	o.Properties[key] = value
	return o
}

// WithLogger 设置日志器。
func (o *ClientOptions) WithLogger(l *zap.Logger) *ClientOptions {
	if l != nil {
		o.Logger = l
	}
	return o
}
