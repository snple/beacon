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

	// Identity 由认证器填写，成功后透出到管理面。
	Identity *Identity
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
type PushContext struct {
	ClientID string
	Identity *Identity
	Channel  string
	Target   string
	Priority packet.Priority
	Headers  map[string]string
	Payload  []byte
}

// MessageHandler 消息面钩子。
type MessageHandler interface {
	// OnPush 在消息入队前调用；返回错误则整条记录被拒绝（PUSH_OK 携带错误原因）。
	OnPush(ctx *PushContext) error
	// OnDeliver 在投递前调用；返回 false 表示丢弃该条投递。
	OnDeliver(ctx *DeliverContext) bool
}

// DeliverContext 投递上下文。
type DeliverContext struct {
	ClientID string
	Channel  string
	Source   string
	Priority packet.Priority
	Payload  []byte
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

// ConnectContext 连接建立上下文。
type ConnectContext struct {
	ClientID       string
	RemoteAddr     string
	SessionPresent bool
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

// StreamOpenContext 是建流申请的授权上下文。
type StreamOpenContext struct {
	ClientID   string
	Identity   *Identity
	Target     string
	Topic      string
	Metadata   []byte
	Properties map[string]string
}

// StreamHandler 是 stream 建流策略钩子。
type StreamHandler interface {
	// OnStreamOpen 在服务端把请求转发给目标客户端之前调用；
	// 返回错误则拒绝本次建流（STREAM_OPEN_OK 携带错误原因）。
	OnStreamOpen(ctx *StreamOpenContext) error
}

// StreamHandlerFunc 适配函数式 stream 钩子。
type StreamHandlerFunc func(ctx *StreamOpenContext) error

// OnStreamOpen 实现 StreamHandler。
func (f StreamHandlerFunc) OnStreamOpen(ctx *StreamOpenContext) error { return f(ctx) }

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
	Stream     StreamHandler

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

func (h *Hooks) callOnStreamOpen(ctx *StreamOpenContext) (err error) {
	if h == nil || h.Stream == nil {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			h.logPanic("OnStreamOpen", r)
			err = fmt.Errorf("stream hook panic: %v", r)
		}
	}()
	return h.Stream.OnStreamOpen(ctx)
}
