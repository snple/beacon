package core

import (
	"fmt"
	"maps"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// Identity 是认证器给出的身份。
type Identity struct {
	ID         string
	Name       string
	Attributes map[string]string

	// ReadChannels 允许声明的 channel 过滤器。nil 表示不允许任何读取；
	// 声明 "**" 等价于允许全部（仍受 MaxChannelsPerClient 约束）。
	ReadChannels []string
	// WriteChannels 允许发布的 channel 过滤器。
	WriteChannels []string
	// AllowTargeted 允许定向投递（Record.Target 非空）。
	// 定向投递跳过 channel 匹配，直接写入目标队列，默认应保持关闭。
	AllowTargeted bool
}

// Clone 深拷贝身份。
func (i *Identity) Clone() *Identity {
	if i == nil {
		return nil
	}
	out := &Identity{
		ID:            i.ID,
		Name:          i.Name,
		AllowTargeted: i.AllowTargeted,
		ReadChannels:  append([]string(nil), i.ReadChannels...),
		WriteChannels: append([]string(nil), i.WriteChannels...),
	}
	if i.Attributes != nil {
		out.Attributes = maps.Clone(i.Attributes)
	}
	return out
}

// AuthContext 认证上下文。
type AuthContext struct {
	ClientID   string
	RemoteAddr string
	Method     string
	Data       []byte
	// Properties 是客户端在 CONNECT 中发送的键值信息，必须视为只读。
	Properties map[string]string

	// Identity 由认证器填写，成功后透出到管理面。
	Identity *Identity
	// ResponseProperties 会随 CONNACK 返回给客户端；仅认证成功时生效。
	ResponseProperties map[string]string
}

// AuthHandler 认证钩子。
type AuthHandler interface {
	Authenticate(ctx *AuthContext) error
}

// AuthHandlerFunc 适配函数式认证器。
type AuthHandlerFunc func(ctx *AuthContext) error

// Authenticate 实现 AuthHandler。
func (f AuthHandlerFunc) Authenticate(ctx *AuthContext) error { return f(ctx) }

// PushContext 发布上下文。
//
// Headers 与 Payload 直接引用即将入队的共享消息，钩子必须视为只读：
// 修改会影响所有命中客户端以及后续重发。
type PushContext struct {
	ClientID    string
	Identity    *Identity
	Channel     string
	Target      string
	Priority    packet.Priority
	ExpiresAt   int64 // Unix 毫秒；0 表示永不过期
	PublishedAt int64 // Unix 毫秒；0 表示未提供
	Headers     map[string]string
	Payload     []byte
	// Properties 是 PUSH 携带的可选属性，必须视为只读。
	Properties map[string]string
}

// MessageHandler 消息面钩子。
type MessageHandler interface {
	// OnPush 在消息入队前调用；返回错误则整条记录被拒绝（PUSH_OK 携带错误原因）。
	OnPush(ctx *PushContext) error
	// OnDeliver 在投递前调用；返回 false 表示丢弃该条投递。
	OnDeliver(ctx *DeliverContext) bool
}

// DeliverContext 投递上下文。
//
// Payload 引用共享消息内容，钩子必须视为只读：修改会影响其他客户端
// 以及后续重发。
type DeliverContext struct {
	ClientID    string
	Channel     string
	Source      string
	Priority    packet.Priority
	ExpiresAt   int64
	PublishedAt int64
	Payload     []byte
}

// MessageHandlerFunc 适配函数式消息钩子。
type MessageHandlerFunc struct {
	PushFn    func(ctx *PushContext) error
	DeliverFn func(ctx *DeliverContext) bool
}

// OnPush 实现 MessageHandler。
func (f MessageHandlerFunc) OnPush(ctx *PushContext) error {
	if f.PushFn == nil {
		return nil
	}
	return f.PushFn(ctx)
}

// OnDeliver 实现 MessageHandler。
func (f MessageHandlerFunc) OnDeliver(ctx *DeliverContext) bool {
	if f.DeliverFn == nil {
		return true
	}
	return f.DeliverFn(ctx)
}

// NackContext 是客户端 NACK 一条消息的上下文。
//
// 可用于记录死信、告警或补偿；钩子在 COMMIT 处理路径中同步执行。
type NackContext struct {
	ClientID string
	ID       packet.PacketID
	Channel  string
	Source   string
	Priority packet.Priority
	Reliable bool
	Reason   packet.ReasonCode
}

// NackHandler 处理客户端 NACK。
type NackHandler interface {
	OnNack(ctx *NackContext)
}

// NackHandlerFunc 适配函数式 NACK 钩子。
type NackHandlerFunc func(ctx *NackContext)

// OnNack 实现 NackHandler。
func (f NackHandlerFunc) OnNack(ctx *NackContext) { f(ctx) }

// ConnectContext 连接建立上下文。
type ConnectContext struct {
	ClientID       string
	RemoteAddr     string
	SessionPresent bool
	// Properties 是客户端在 CONNECT 中发送的键值信息。
	Properties map[string]string
}

// DisconnectContext 连接断开上下文。
type DisconnectContext struct {
	ClientID string
	Reason   packet.ReasonCode
	Err      error
}

// ConnectionHandler 连接生命周期钩子。
type ConnectionHandler interface {
	OnConnect(ctx *ConnectContext)
	OnDisconnect(ctx *DisconnectContext)
}

// ConnectionHandlerFunc 适配函数式连接钩子。
type ConnectionHandlerFunc struct {
	ConnectFn    func(ctx *ConnectContext)
	DisconnectFn func(ctx *DisconnectContext)
}

// OnConnect 实现 ConnectionHandler。
func (f ConnectionHandlerFunc) OnConnect(ctx *ConnectContext) {
	if f.ConnectFn != nil {
		f.ConnectFn(ctx)
	}
}

// OnDisconnect 实现 ConnectionHandler。
func (f ConnectionHandlerFunc) OnDisconnect(ctx *DisconnectContext) {
	if f.DisconnectFn != nil {
		f.DisconnectFn(ctx)
	}
}

// Hooks 聚合全部钩子（均可为 nil）。
type Hooks struct {
	Auth       AuthHandler
	Message    MessageHandler
	Connection ConnectionHandler
	Nack       NackHandler

	// Logger 用于记录钩子 panic；由 Core 自动填充。
	Logger *zap.Logger
}

func (h *Hooks) logPanic(name string, r any) {
	if h != nil && h.Logger != nil {
		h.Logger.Error("hook panicked", zap.String("hook", name), zap.Any("panic", r))
	}
}

func (h *Hooks) callAuth(ctx *AuthContext) (err error) {
	if h == nil || h.Auth == nil {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			h.logPanic("OnAuth", r)
			err = fmt.Errorf("auth hook panic: %v", r)
		}
	}()
	return h.Auth.Authenticate(ctx)
}

func (h *Hooks) callOnPush(ctx *PushContext) (err error) {
	if h == nil || h.Message == nil {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			h.logPanic("OnPush", r)
			err = fmt.Errorf("push hook panic: %v", r)
		}
	}()
	return h.Message.OnPush(ctx)
}

func (h *Hooks) callOnDeliver(ctx *DeliverContext) (ok bool) {
	if h == nil || h.Message == nil {
		return true
	}
	defer func() {
		if r := recover(); r != nil {
			h.logPanic("OnDeliver", r)
			ok = false
		}
	}()
	return h.Message.OnDeliver(ctx)
}

// hasOnDeliver 报告是否注册了投递钩子。
func (h *Hooks) hasOnDeliver() bool {
	return h != nil && h.Message != nil
}

func (h *Hooks) callOnConnect(ctx *ConnectContext) {
	if h == nil || h.Connection == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			h.logPanic("OnConnect", r)
		}
	}()
	h.Connection.OnConnect(ctx)
}

func (h *Hooks) hasNack() bool {
	return h != nil && h.Nack != nil
}

func (h *Hooks) callOnNack(ctx *NackContext) {
	if h == nil || h.Nack == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			h.logPanic("OnNack", r)
		}
	}()
	h.Nack.OnNack(ctx)
}

func (h *Hooks) callOnDisconnect(ctx *DisconnectContext) {
	if h == nil || h.Connection == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			h.logPanic("OnDisconnect", r)
		}
	}()
	h.Connection.OnDisconnect(ctx)
}
