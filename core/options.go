package core

import (
	"fmt"
	"net/http"
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// CoreOptions Core 配置选项（Builder 模式）
type CoreOptions struct {
	// ===== 连接 =====
	MaxClients     uint32        // 最大并发连接数
	MaxSessions    int           // 会话总数上限（含离线保留；<0 表示不限制）
	MaxPacketSize  uint32        // 服务端允许的最大包大小
	ConnectTimeout uint32        // 握手超时（秒）
	KeepAlive      uint16        // 服务端要求的心跳间隔（秒），0 表示沿用客户端值
	WriteTimeout   time.Duration // 单次写超时（0 表示使用默认值 10s）

	// WSCheckOrigin 校验 WebSocket 握手来源；nil 时只允许同源（或无 Origin 的非浏览器客户端）。
	WSCheckOrigin func(*http.Request) bool

	// MaxHandshakeConnections 认证前的并发连接上限（0 表示不限制）
	MaxHandshakeConnections int

	// 连接速率限制
	ConnectionRateLimit float64
	ConnectionRateBurst int

	// ===== 队列 =====
	QueueCap int // 单个客户端 pending 队列条数上限（0 表示不限制）

	MaxRecords uint16 // 单次 PULL 的记录数上限（同时约束 delivered 长度）
	MaxBytes   uint32 // 单次 PULL / PUSH 的字节上限（0 表示使用默认值 1 MiB）

	// MaxInflightBytes 全局在途消息字节上限（<0 表示不限制）
	MaxInflightBytes int64

	MaxWaitMs uint32 // 长轮询上限

	MaxChannelsPerClient int // 单客户端可声明的过滤器数量上限

	// MaxChannelLength channel 名/过滤器的最大字节数。
	// 取值范围 [packet.MinChannelLength, packet.MaxChannelLength]。
	MaxChannelLength int

	// MaxRecordBytes 单条记录（payload + 元数据）上限
	MaxRecordBytes uint32

	// ===== 会话 =====
	// SessionTimeout 会话保留时长（秒）；0 表示断开即清理。
	SessionTimeout       uint32
	SessionCheckInterval time.Duration

	// ===== 背压与惩罚 =====
	NoProgressSoft int // 连续无进展 pull 软上限（0 表示关闭）
	NoProgressHard int // 连续无进展 pull 硬上限（0 表示关闭）

	// UncommittedTimeout delivered 长期未确认的超时（0 表示关闭）
	UncommittedTimeout time.Duration

	// 发布速率限制（每客户端）
	PushRate  float64
	PushBurst int

	// ===== 杂项 =====
	Logger *zap.Logger
	Hooks  *Hooks
}

// NewCoreOptions 创建默认配置
func NewCoreOptions() *CoreOptions {
	return &CoreOptions{
		MaxClients:              10000,
		MaxSessions:             10000,
		MaxPacketSize:           16777216,
		ConnectTimeout:          10,
		WriteTimeout:            10 * time.Second,
		MaxHandshakeConnections: 512,

		QueueCap:         10000,
		MaxInflightBytes: 256 << 20,
		MaxRecords:       1000,
		MaxBytes:         1 << 20,

		MaxWaitMs:            300000,
		MaxChannelsPerClient: 1024,
		MaxChannelLength:     packet.MinChannelLength,
		MaxRecordBytes:       1 << 20,

		SessionTimeout:       86400,
		SessionCheckInterval: 30 * time.Second,

		NoProgressSoft: 4,
		NoProgressHard: 16,

		Logger: zap.NewNop(),
	}
}

func (o *CoreOptions) applyDefaults() {
	def := NewCoreOptions()

	if o.MaxClients == 0 {
		o.MaxClients = def.MaxClients
	}
	if o.MaxSessions == 0 {
		o.MaxSessions = def.MaxSessions
	}
	if o.MaxInflightBytes == 0 {
		o.MaxInflightBytes = def.MaxInflightBytes
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
	if o.MaxRecords == 0 {
		o.MaxRecords = def.MaxRecords
	}
	if o.MaxBytes == 0 {
		o.MaxBytes = def.MaxBytes
	}
	if o.MaxWaitMs == 0 {
		o.MaxWaitMs = def.MaxWaitMs
	}
	if o.MaxChannelsPerClient == 0 {
		o.MaxChannelsPerClient = def.MaxChannelsPerClient
	}
	if o.MaxChannelLength == 0 {
		o.MaxChannelLength = def.MaxChannelLength
	}
	if o.MaxRecordBytes == 0 {
		o.MaxRecordBytes = def.MaxRecordBytes
	}
	if o.SessionCheckInterval == 0 {
		o.SessionCheckInterval = def.SessionCheckInterval
	}
	if o.Logger == nil {
		o.Logger = def.Logger
	}
	if o.Hooks == nil {
		o.Hooks = &Hooks{}
	}
	if o.Hooks.Logger == nil {
		o.Hooks.Logger = o.Logger
	}
}

// Validate 校验配置
func (o *CoreOptions) Validate() error {
	if o.MaxPacketSize < 64 {
		return fmt.Errorf("maxPacketSize must be >= 64, got %d", o.MaxPacketSize)
	}
	if o.MaxBytes > o.MaxPacketSize {
		return fmt.Errorf("maxBytes (%d) must not exceed maxPacketSize (%d)", o.MaxBytes, o.MaxPacketSize)
	}
	if o.MaxRecordBytes > o.MaxPacketSize {
		return fmt.Errorf("maxRecordBytes (%d) must not exceed maxPacketSize (%d)", o.MaxRecordBytes, o.MaxPacketSize)
	}
	if o.MaxRecords == 0 {
		return fmt.Errorf("maxRecords must be > 0")
	}
	if o.QueueCap < 0 {
		return fmt.Errorf("queueCap must be >= 0")
	}
	if o.MaxWaitMs < 1000 {
		return fmt.Errorf("maxWaitMs must be >= 1000")
	}
	if o.MaxChannelLength < packet.MinChannelLength || o.MaxChannelLength > packet.MaxChannelLength {
		return fmt.Errorf("maxChannelLength must be in [%d, %d], got %d",
			packet.MinChannelLength, packet.MaxChannelLength, o.MaxChannelLength)
	}
	return nil
}

// WithMaxClients 设置最大并发连接数
func (o *CoreOptions) WithMaxClients(n uint32) *CoreOptions { o.MaxClients = n; return o }

// WithMaxSessions 设置会话总数上限（含离线保留）；n < 0 表示不限制。
func (o *CoreOptions) WithMaxSessions(n int) *CoreOptions { o.MaxSessions = n; return o }

// WithMaxInflightBytes 设置全局在途消息字节上限；n < 0 表示不限制。
func (o *CoreOptions) WithMaxInflightBytes(n int64) *CoreOptions {
	o.MaxInflightBytes = n
	return o
}

// WithWSCheckOrigin 设置 WebSocket Origin 校验函数；nil 使用默认同源策略。
func (o *CoreOptions) WithWSCheckOrigin(fn func(*http.Request) bool) *CoreOptions {
	o.WSCheckOrigin = fn
	return o
}

// WithMaxPacketSize 设置最大包大小
func (o *CoreOptions) WithMaxPacketSize(n uint32) *CoreOptions { o.MaxPacketSize = n; return o }

// WithConnectTimeout 设置握手超时（秒）
func (o *CoreOptions) WithConnectTimeout(s uint32) *CoreOptions { o.ConnectTimeout = s; return o }

// WithKeepAlive 设置服务端要求的心跳间隔（秒）
func (o *CoreOptions) WithKeepAlive(s uint16) *CoreOptions { o.KeepAlive = s; return o }

// WithWriteTimeout 设置单次网络写超时
func (o *CoreOptions) WithWriteTimeout(d time.Duration) *CoreOptions { o.WriteTimeout = d; return o }

// WithMaxHandshakeConnections 设置握手阶段并发上限
func (o *CoreOptions) WithMaxHandshakeConnections(n int) *CoreOptions {
	o.MaxHandshakeConnections = n
	return o
}

// WithConnectionRateLimit 设置连接速率限制
func (o *CoreOptions) WithConnectionRateLimit(rate float64, burst int) *CoreOptions {
	o.ConnectionRateLimit = rate
	o.ConnectionRateBurst = burst
	return o
}

// WithQueueCap 设置单客户端 pending 队列条数上限
func (o *CoreOptions) WithQueueCap(n int) *CoreOptions { o.QueueCap = n; return o }

// WithMaxRecords 设置单次 PULL 的记录数上限
func (o *CoreOptions) WithMaxRecords(n uint16) *CoreOptions { o.MaxRecords = n; return o }

// WithMaxBytes 设置单次 PULL / PUSH 的字节上限
func (o *CoreOptions) WithMaxBytes(n uint32) *CoreOptions { o.MaxBytes = n; return o }

// WithWaitLimit 设置长轮询上限（毫秒）
func (o *CoreOptions) WithWaitLimit(max uint32) *CoreOptions {
	o.MaxWaitMs = max
	return o
}

// WithMaxChannelsPerClient 设置单客户端声明数量上限
func (o *CoreOptions) WithMaxChannelsPerClient(n int) *CoreOptions {
	o.MaxChannelsPerClient = n
	return o
}

// WithMaxChannelLength 设置 channel 名/过滤器的最大字节数。
// 取值必须落在 [packet.MinChannelLength, packet.MaxChannelLength] 内。
func (o *CoreOptions) WithMaxChannelLength(n int) *CoreOptions {
	o.MaxChannelLength = n
	return o
}

// WithMaxRecordBytes 设置单条记录大小上限
func (o *CoreOptions) WithMaxRecordBytes(n uint32) *CoreOptions { o.MaxRecordBytes = n; return o }

// WithSessionTimeout 设置会话保留时长（秒）；0 表示断开即清理。
func (o *CoreOptions) WithSessionTimeout(seconds uint32) *CoreOptions {
	o.SessionTimeout = seconds
	return o
}

// WithSessionCheckInterval 设置过期会话检查间隔
func (o *CoreOptions) WithSessionCheckInterval(d time.Duration) *CoreOptions {
	o.SessionCheckInterval = d
	return o
}

// WithNoProgressLimits 设置无进展 pull 的软/硬上限（0 表示关闭）
func (o *CoreOptions) WithNoProgressLimits(soft, hard int) *CoreOptions {
	o.NoProgressSoft, o.NoProgressHard = soft, hard
	return o
}

// WithUncommittedTimeout 设置 delivered 长期未确认超时
func (o *CoreOptions) WithUncommittedTimeout(d time.Duration) *CoreOptions {
	o.UncommittedTimeout = d
	return o
}

// WithPushRateLimit 设置每客户端发布速率限制
func (o *CoreOptions) WithPushRateLimit(rate float64, burst int) *CoreOptions {
	o.PushRate, o.PushBurst = rate, burst
	return o
}

// WithLogger 设置日志器
func (o *CoreOptions) WithLogger(l *zap.Logger) *CoreOptions {
	if l != nil {
		o.Logger = l
	}
	return o
}

// WithHooks 设置钩子
func (o *CoreOptions) WithHooks(h *Hooks) *CoreOptions { o.Hooks = h; return o }
