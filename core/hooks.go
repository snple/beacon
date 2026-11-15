package core

import (
	"fmt"
	"maps"
	"sync/atomic"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// Identity 是认证器给出的身份。
type Identity struct {
	// ID 是身份的唯一标识，仅用于管理面展示与审计；ACL 只看下面的过滤器字段。
	ID string
	// Name 是人类可读名称，可为空。
	Name string
	// Attributes 是身份附带的键值信息，会透出到管理面与后续钩子。
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

// AuthContext 是单轮认证的上下文。
//
// 连接与认证字段来自本次 CONNECT（客户端声明值）；Identity 与
// ResponseProperties 由认证器填写。
type AuthContext struct {
	// ClientID 是客户端声明的 ID；为空时服务端已生成一个。
	ClientID string
	// RemoteAddr 是对端地址。
	RemoteAddr string

	// ProtocolVersion 是客户端声明的协议版本。
	ProtocolVersion uint8
	// CleanSession 是客户端声明的会话重建意图。
	CleanSession bool
	// KeepAlive 是客户端声明的心跳间隔（秒），尚未被服务端夹取。
	KeepAlive uint16
	// MaxPacketSize 是客户端声明的接收上限；0 表示未声明。
	MaxPacketSize uint32

	// AuthMethod 是客户端声明的认证方法。
	AuthMethod string
	// AuthData 是客户端提供的认证数据。
	AuthData []byte
	// Properties 是 CONNECT 携带的属性，必须视为只读。
	Properties map[string]string

	// Identity 由认证器填写，成功后透出到管理面与后续钩子。
	// 未填写时客户端仍可连接，但任何 channel 读写都会被拒绝。
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

// PushContext 是单条发布的上下文。
//
// Priority/Headers/Payload/ExpiresAt 可被钩子改写：服务端会在入队前应用，
// 并重新校验记录大小上限。Channel/Target/ID/Reliable 只读——路由与信任字段
// 已经校验过，改动会绕过 ACL。
//
// Identity/Properties 只读。
type PushContext struct {
	// ID 是服务端为这条记录分配的 ID（客户端不可指定）。
	ID packet.PacketID
	// ClientID 是发布者；FromServer 为 true 时为空。
	ClientID string
	// Identity 是发布者身份；FromServer 为 true 时为空。
	Identity *Identity
	// Channel 是消息发布的 channel（已通过名称与写权限校验）。
	Channel string
	// Target 是定向投递的目标 clientID；空表示广播发布。
	Target string

	// ===== 可改写：消息内容 =====
	// Priority 是消息优先级，改写后按新值入队与淘汰。
	Priority packet.Priority
	// Headers 是消息头；为 nil 表示没有消息头，写 header 请用 SetHeader。
	Headers map[string]string
	// Payload 是消息内容，改写后以新内容入队。
	Payload []byte
	// ExpiresAt 是过期时间（Unix 毫秒）；0 表示永不过期。
	ExpiresAt int64

	// ===== 只读：消息元数据与来源 =====
	// FromServer 表示消息由服务端管理接口发布，而不是客户端 PUSH。
	FromServer bool
	// Reliable 表示消息是否需要 COMMIT 确认。
	Reliable bool
	// PublishedAt 是发布者时间戳（Unix 毫秒）；0 表示未提供。
	PublishedAt int64
	// Properties 是 PUSH 携带的属性，必须视为只读。
	Properties map[string]string

	// ResponseProperties 会随 PUSH_OK 返回给发布者；仅在 WantAck=1 时生效。
	ResponseProperties map[string]string
}

// SetHeader 写入一个 header；Headers 为 nil 时自动创建。
func (c *PushContext) SetHeader(key, value string) {
	if c.Headers == nil {
		c.Headers = make(map[string]string, 1)
	}
	c.Headers[key] = value
}

// PushHandler 发布钩子。
type PushHandler interface {
	// OnPush 在消息入队前调用；返回错误则整条记录被拒绝。
	//
	// 返回 *HookError 可以指定拒绝原因码与回给发布者的诊断信息；
	// 其他错误按 NotAuthorized 处理。
	OnPush(ctx *PushContext) error
}

// PushHandlerFunc 适配函数式发布钩子。
type PushHandlerFunc func(ctx *PushContext) error

// OnPush 实现 PushHandler。
func (f PushHandlerFunc) OnPush(ctx *PushContext) error { return f(ctx) }

// DeliverContext 是单条投递的上下文。
//
// Identity 引用会话身份，Headers/Payload 引用共享消息内容，都必须视为只读：
// 修改会影响其他客户端、ACL 判断以及后续重发。
type DeliverContext struct {
	// ID 是被投递消息的 ID，重发时保持不变。
	ID packet.PacketID
	// ClientID 是接收者。
	ClientID string
	// Identity 是接收者身份。
	Identity *Identity
	// Channel 是消息所属 channel。
	Channel string
	// Source 是原始发布者；服务端发布时为空。
	Source string
	// Target 是定向投递的目标 clientID；空表示广播发布。
	Target string
	// Priority 是消息优先级。
	Priority packet.Priority
	// Reliable 表示消息是否需要 COMMIT 确认。
	Reliable bool
	// ExpiresAt 是过期时间（Unix 毫秒）；0 表示永不过期。
	ExpiresAt int64
	// PublishedAt 是发布者时间戳（Unix 毫秒）；0 表示未提供。
	PublishedAt int64
	// Headers 是发布时携带的消息头。
	Headers map[string]string
	// Payload 是消息内容。
	Payload []byte
	// Dup 表示这是未确认消息的重发。
	Dup bool
}

// DeliverHandler 投递钩子。
type DeliverHandler interface {
	// OnDeliver 在每次投递（含重发）前调用；返回 false 表示丢弃该条投递。
	OnDeliver(ctx *DeliverContext) bool
}

// DeliverHandlerFunc 适配函数式投递钩子。
type DeliverHandlerFunc func(ctx *DeliverContext) bool

// OnDeliver 实现 DeliverHandler。
func (f DeliverHandlerFunc) OnDeliver(ctx *DeliverContext) bool { return f(ctx) }

// NackContext 是客户端 NACK 一条消息的上下文。
//
// 可用于记录死信、告警或补偿；钩子在 COMMIT 处理路径中同步执行。
// Identity 引用会话身份，Headers/Payload 引用共享消息内容，都必须视为只读。
type NackContext struct {
	// ID 是被拒绝消息的 ID。
	ID packet.PacketID
	// ClientID 是拒绝消息的客户端。
	ClientID string
	// Identity 是拒绝消息的客户端身份。
	Identity *Identity
	// Channel 是消息所属 channel。
	Channel string
	// Source 是原始发布者；服务端发布时为空。
	Source string
	// Target 是定向投递的目标 clientID；空表示广播发布。
	Target string
	// Priority 是消息优先级。
	Priority packet.Priority
	// Reliable 表示消息是否需要 COMMIT 确认。
	Reliable bool
	// Reason 是客户端给出的原因码，协议本身不解释。
	Reason packet.ReasonCode
	// Headers 是发布时携带的消息头。
	Headers map[string]string
	// Payload 是消息内容。
	Payload []byte
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
//
// Identity 必须视为只读。
type ConnectContext struct {
	// ClientID 是客户端 ID（服务端生成时也已回填）。
	ClientID string
	// SessionID 是本次连接使用的会话租约标识。
	SessionID uint64
	// RemoteAddr 是对端地址。
	RemoteAddr string
	// SessionPresent 表示复用了既有会话。
	SessionPresent bool
	// CleanSession 表示本次连接清空了既有会话的队列与声明。
	CleanSession bool
	// Identity 是本次认证得到的身份；未配置认证器时为 nil。
	Identity *Identity
	// Properties 是 CONNECT 携带的属性，必须视为只读。
	Properties map[string]string
}

// ConnectHandler 连接建立钩子。
type ConnectHandler interface {
	OnConnect(ctx *ConnectContext)
}

// ConnectHandlerFunc 适配函数式连接建立钩子。
type ConnectHandlerFunc func(ctx *ConnectContext)

// OnConnect 实现 ConnectHandler。
func (f ConnectHandlerFunc) OnConnect(ctx *ConnectContext) { f(ctx) }

// DisconnectContext 连接断开上下文。
//
// Identity 必须视为只读。只有在 OnConnect 已触发过的连接上才会调用：
// 握手阶段被拒绝或接管、从未发出 CONNACK 的连接不会产生此事件。
type DisconnectContext struct {
	// ClientID 是该连接绑定的客户端 ID。
	ClientID string
	// SessionID 是该连接自己的会话租约标识，接管后不会变成新连接的。
	SessionID uint64
	// RemoteAddr 是对端地址。
	RemoteAddr string
	// Reason 是连接结束原因码；客户端主动断开时为 NormalDisconnect。
	Reason packet.ReasonCode
	// Err 是连接结束时的 I/O/协议错误；正常断开为 nil。
	Err error
	// Identity 是连接期间使用的身份；未配置认证器时为 nil。
	Identity *Identity
}

// DisconnectHandler 连接断开钩子。
type DisconnectHandler interface {
	OnDisconnect(ctx *DisconnectContext)
}

// DisconnectHandlerFunc 适配函数式连接断开钩子。
type DisconnectHandlerFunc func(ctx *DisconnectContext)

// OnDisconnect 实现 DisconnectHandler。
func (f DisconnectHandlerFunc) OnDisconnect(ctx *DisconnectContext) { f(ctx) }

// DeclareContext 是客户端声明被处理后的事件。
//
// 通知型钩子：不能否决声明（授权已由 Identity.ReadChannels 决定），
// 但可以设置 DECLARE_RESP 的属性。
type DeclareContext struct {
	// ClientID 是发起声明的客户端。
	ClientID string
	// Identity 是该客户端的身份；未配置认证器时为 nil。
	Identity *Identity
	// Properties 是 DECLARE 携带的属性，必须视为只读。
	Properties map[string]string
	// Requested 是客户端的原始请求（含重复项，未经过滤）。
	Requested []packet.ChannelDecl
	// Accepted 是实际生效的声明。
	Accepted []packet.ChannelDecl
	// Rejected 是被拒绝的项及其原因。
	Rejected []packet.ChannelResult
	// ResponseProperties 会随 DECLARE_RESP 返回给客户端。
	ResponseProperties map[string]string
}

// DeclareHandler 声明变更钩子。
type DeclareHandler interface {
	OnDeclare(ctx *DeclareContext)
}

// DeclareHandlerFunc 适配函数式声明钩子。
type DeclareHandlerFunc func(ctx *DeclareContext)

// OnDeclare 实现 DeclareHandler。
func (f DeclareHandlerFunc) OnDeclare(ctx *DeclareContext) { f(ctx) }

// SessionEndContext 是会话被销毁（队列已释放）的事件。
type SessionEndContext struct {
	// ClientID 是会话所属客户端。
	ClientID string
	// SessionID 是会话销毁前的租约标识。
	SessionID uint64
	// Identity 是会话身份；未配置认证器时为 nil。
	Identity *Identity
	// Reason 复用连接结束原因码：会话过期/断开即清理/服务停机。
	Reason packet.ReasonCode
	// MessagesDropped 是随会话一起释放的队列消息条数。
	MessagesDropped int
}

// SessionEndHandler 会话销毁钩子。
type SessionEndHandler interface {
	OnSessionEnd(ctx *SessionEndContext)
}

// SessionEndHandlerFunc 适配函数式会话销毁钩子。
type SessionEndHandlerFunc func(ctx *SessionEndContext)

// OnSessionEnd 实现 SessionEndHandler。
func (f SessionEndHandlerFunc) OnSessionEnd(ctx *SessionEndContext) { f(ctx) }

// PullContext 是一次 PULL 请求的上下文。
//
// 通知型钩子：不能否决拉取，只能读取请求信息并设置响应属性。
type PullContext struct {
	// ClientID 是发起拉取的客户端。
	ClientID string
	// Identity 是该客户端的身份；未配置认证器时为 nil。
	Identity *Identity
	// Properties 是 PULL 携带的属性，必须视为只读。
	Properties map[string]string
	// WaitMs 是客户端请求的长轮询时长，未经服务端裁剪。
	WaitMs uint32
	// MaxCount 是客户端请求的记录条数上限，未经服务端裁剪。
	MaxCount uint16
	// MaxBytes 是客户端请求的响应字节上限，未经服务端裁剪。
	MaxBytes uint32
	// ResponseProperties 会随 PULL_OK 返回给客户端。
	ResponseProperties map[string]string
}

// PullHandler 拉取钩子。
type PullHandler interface {
	OnPull(ctx *PullContext)
}

// PullHandlerFunc 适配函数式拉取钩子。
type PullHandlerFunc func(ctx *PullContext)

// OnPull 实现 PullHandler。
func (f PullHandlerFunc) OnPull(ctx *PullContext) { f(ctx) }

// Hooks 聚合全部钩子（均可为 nil）。
//
// Core 在创建时对 Hooks 做一次浅拷贝：之后替换某个钩子字段不会生效，
// 但钩子实现可以在运行期修改自身内部状态（由实现方保证并发安全）。
//
// 所有钩子都在调用方协程中同步执行，传入的 map/slice 一律只读
// （PushContext 的可写字段除外）。
type Hooks struct {
	// Auth 在握手阶段完成认证与授权。
	Auth AuthHandler
	// Connect 观察连接建立。
	Connect ConnectHandler
	// Disconnect 观察连接断开。
	Disconnect DisconnectHandler
	// Declare 观察声明变更，并设置 DECLARE_RESP 属性。
	Declare DeclareHandler
	// Push 在入队前审核/改写发布，并设置 PUSH_OK 属性。
	Push PushHandler
	// Pull 观察拉取请求，并设置 PULL_OK 属性。
	Pull PullHandler
	// Deliver 在每条投递（含重发）前过滤。
	Deliver DeliverHandler
	// Nack 处理客户端拒绝的消息。
	Nack NackHandler
	// SessionEnd 观察会话销毁。
	SessionEnd SessionEndHandler

	// Logger 用于记录钩子 panic；由 Core 自动填充。
	Logger *zap.Logger

	// stats 记录调用与 panic 次数；由 Core 填充。
	stats *hookStats
}

// hookKind 是钩子统计维度。
type hookKind int

const (
	hookAuth hookKind = iota
	hookPush
	hookDeliver
	hookConnect
	hookDisconnect
	hookDeclare
	hookSessionEnd
	hookPull
	hookNack
	hookKindCount
)

var hookKindNames = [hookKindCount]string{
	hookAuth:       "auth",
	hookPush:       "push",
	hookDeliver:    "deliver",
	hookConnect:    "connect",
	hookDisconnect: "disconnect",
	hookDeclare:    "declare",
	hookSessionEnd: "session-end",
	hookPull:       "pull",
	hookNack:       "nack",
}

// hookStats 是各钩子的调用/panic 计数。
type hookStats struct {
	calls  [hookKindCount]atomic.Int64
	panics [hookKindCount]atomic.Int64
}

func (h *Hooks) recordCall(k hookKind) {
	if h != nil && h.stats != nil {
		h.stats.calls[k].Add(1)
	}
}

func (h *Hooks) recordPanic(k hookKind, r any) {
	if h != nil && h.stats != nil {
		h.stats.panics[k].Add(1)
	}
	if h != nil && h.Logger != nil {
		h.Logger.Error("hook panicked",
			zap.String("hook", hookKindNames[k]),
			zap.Any("panic", r),
			zap.Stack("stack"))
	}
}

func (h *Hooks) callAuth(ctx *AuthContext) (err error) {
	if h == nil || h.Auth == nil {
		return nil
	}
	h.recordCall(hookAuth)
	defer func() {
		if r := recover(); r != nil {
			h.recordPanic(hookAuth, r)
			err = &HookError{Reason: packet.ReasonUnspecifiedError, Err: fmt.Errorf("auth hook panic: %v", r)}
		}
	}()
	return h.Auth.Authenticate(ctx)
}

func (h *Hooks) callOnPush(ctx *PushContext) (err error) {
	if h == nil || h.Push == nil {
		return nil
	}
	h.recordCall(hookPush)
	defer func() {
		if r := recover(); r != nil {
			h.recordPanic(hookPush, r)
			err = &HookError{Reason: packet.ReasonUnspecifiedError, Err: fmt.Errorf("push hook panic: %v", r)}
		}
	}()
	return h.Push.OnPush(ctx)
}

func (h *Hooks) callOnDeliver(ctx *DeliverContext) (ok bool) {
	if h == nil || h.Deliver == nil {
		return true
	}
	h.recordCall(hookDeliver)
	defer func() {
		if r := recover(); r != nil {
			h.recordPanic(hookDeliver, r)
			ok = false
		}
	}()
	return h.Deliver.OnDeliver(ctx)
}

func (h *Hooks) callOnConnect(ctx *ConnectContext) {
	if h == nil || h.Connect == nil {
		return
	}
	h.recordCall(hookConnect)
	defer func() {
		if r := recover(); r != nil {
			h.recordPanic(hookConnect, r)
		}
	}()
	h.Connect.OnConnect(ctx)
}

func (h *Hooks) callOnDisconnect(ctx *DisconnectContext) {
	if h == nil || h.Disconnect == nil {
		return
	}
	h.recordCall(hookDisconnect)
	defer func() {
		if r := recover(); r != nil {
			h.recordPanic(hookDisconnect, r)
		}
	}()
	h.Disconnect.OnDisconnect(ctx)
}

func (h *Hooks) callOnDeclare(ctx *DeclareContext) {
	if h == nil || h.Declare == nil {
		return
	}
	h.recordCall(hookDeclare)
	defer func() {
		if r := recover(); r != nil {
			h.recordPanic(hookDeclare, r)
		}
	}()
	h.Declare.OnDeclare(ctx)
}

func (h *Hooks) callOnSessionEnd(ctx *SessionEndContext) {
	if h == nil || h.SessionEnd == nil {
		return
	}
	h.recordCall(hookSessionEnd)
	defer func() {
		if r := recover(); r != nil {
			h.recordPanic(hookSessionEnd, r)
		}
	}()
	h.SessionEnd.OnSessionEnd(ctx)
}

func (h *Hooks) callOnNack(ctx *NackContext) {
	if h == nil || h.Nack == nil {
		return
	}
	h.recordCall(hookNack)
	defer func() {
		if r := recover(); r != nil {
			h.recordPanic(hookNack, r)
		}
	}()
	h.Nack.OnNack(ctx)
}

func (h *Hooks) callOnPull(ctx *PullContext) {
	if h == nil || h.Pull == nil {
		return
	}
	h.recordCall(hookPull)
	defer func() {
		if r := recover(); r != nil {
			h.recordPanic(hookPull, r)
		}
	}()
	h.Pull.OnPull(ctx)
}

// ============================================================================
// 组合器
// ============================================================================

// ChainPush 依次调用处理器，任一返回错误即中止。
func ChainPush(hs ...PushHandler) PushHandler {
	return PushHandlerFunc(func(ctx *PushContext) error {
		for _, h := range hs {
			if h == nil {
				continue
			}
			if err := h.OnPush(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

// ChainDeliver 依次调用处理器，任一返回 false 即丢弃该条投递。
func ChainDeliver(hs ...DeliverHandler) DeliverHandler {
	return DeliverHandlerFunc(func(ctx *DeliverContext) bool {
		for _, h := range hs {
			if h == nil {
				continue
			}
			if !h.OnDeliver(ctx) {
				return false
			}
		}
		return true
	})
}

// ChainConnect 依次调用全部处理器。
func ChainConnect(hs ...ConnectHandler) ConnectHandler {
	return ConnectHandlerFunc(func(ctx *ConnectContext) {
		for _, h := range hs {
			if h != nil {
				h.OnConnect(ctx)
			}
		}
	})
}

// ChainDisconnect 依次调用全部处理器。
func ChainDisconnect(hs ...DisconnectHandler) DisconnectHandler {
	return DisconnectHandlerFunc(func(ctx *DisconnectContext) {
		for _, h := range hs {
			if h != nil {
				h.OnDisconnect(ctx)
			}
		}
	})
}

// ChainDeclare 依次调用全部处理器。
func ChainDeclare(hs ...DeclareHandler) DeclareHandler {
	return DeclareHandlerFunc(func(ctx *DeclareContext) {
		for _, h := range hs {
			if h != nil {
				h.OnDeclare(ctx)
			}
		}
	})
}

// ChainSessionEnd 依次调用全部处理器。
func ChainSessionEnd(hs ...SessionEndHandler) SessionEndHandler {
	return SessionEndHandlerFunc(func(ctx *SessionEndContext) {
		for _, h := range hs {
			if h != nil {
				h.OnSessionEnd(ctx)
			}
		}
	})
}

// ChainNack 依次调用全部处理器。
func ChainNack(hs ...NackHandler) NackHandler {
	return NackHandlerFunc(func(ctx *NackContext) {
		for _, h := range hs {
			if h != nil {
				h.OnNack(ctx)
			}
		}
	})
}

// ChainPull 依次调用全部处理器。
func ChainPull(hs ...PullHandler) PullHandler {
	return PullHandlerFunc(func(ctx *PullContext) {
		for _, h := range hs {
			if h != nil {
				h.OnPull(ctx)
			}
		}
	})
}
