package client

import (
	"github.com/snple/beacon/packet"
)

// ============================================================================
// 连接管理钩子 (ConnectionHandler)
// ============================================================================

// ConnectionHandler 连接生命周期处理器接口
type ConnectionHandler interface {
	// OnConnect 连接成功后调用
	// 使用原始 CONNACK 包，避免数据拷贝
	OnConnect(ctx *ConnectContext)

	// OnDisconnect 断开连接时调用
	OnDisconnect(ctx *DisconnectContext)
}

// ConnectContext 连接上下文
// 使用原始 packet 引用，避免数据拷贝
type ConnectContext struct {
	ClientID       string                // 客户端 ID
	SessionPresent bool                  // 是否恢复了旧会话
	Packet         *packet.ConnackPacket // 原始 CONNACK 包（引用，不要修改）
}

// DisconnectContext 断开连接上下文
type DisconnectContext struct {
	ClientID string                   // 客户端 ID
	Packet   *packet.DisconnectPacket // 原始 DISCONNECT 包（服务器发送的，可能为 nil）
	Err      error                    // 断开原因（nil 表示正常断开）
}

// ConnectionHandlerFunc 函数类型适配器
type ConnectionHandlerFunc struct {
	ConnectFunc    func(ctx *ConnectContext)
	DisconnectFunc func(ctx *DisconnectContext)
}

func (h *ConnectionHandlerFunc) OnConnect(ctx *ConnectContext) {
	if h.ConnectFunc != nil {
		h.ConnectFunc(ctx)
	}
}

func (h *ConnectionHandlerFunc) OnDisconnect(ctx *DisconnectContext) {
	if h.DisconnectFunc != nil {
		h.DisconnectFunc(ctx)
	}
}

// ============================================================================
// 认证钩子 (AuthHandler)
// ============================================================================

// AuthHandler 高级认证处理器接口
// 用于处理多轮认证流程（如 OAuth2, SASL 等）
type AuthHandler interface {
	// OnAuth 处理服务器发来的 AUTH 包
	// 使用原始 packet 引用，避免数据拷贝
	// 返回 (true, data, nil) 继续认证流程
	// 返回 (false, data, nil) 认证完成
	// 返回 error 认证失败
	OnAuth(ctx *AuthContext) (continued bool, authData []byte, err error)
}

// AuthContext 认证上下文
// 使用原始 packet 引用，避免数据拷贝
type AuthContext struct {
	ClientID string             // 客户端 ID
	Packet   *packet.AuthPacket // 原始 AUTH 包（引用，不要修改）
}

// AuthHandlerFunc 函数类型适配器
type AuthHandlerFunc func(ctx *AuthContext) (bool, []byte, error)

func (f AuthHandlerFunc) OnAuth(ctx *AuthContext) (bool, []byte, error) {
	return f(ctx)
}

// ============================================================================
// 追踪钩子 (TraceHandler)
// ============================================================================

// TraceHandler 追踪处理器接口
type TraceHandler interface {
	// OnTrace 处理追踪信息
	// 使用原始 packet 引用，避免数据拷贝
	OnTrace(ctx *TraceContext)
}

// TraceContext 追踪上下文
// 使用原始 packet 引用，避免数据拷贝
type TraceContext struct {
	ClientID string              // 客户端 ID
	Packet   *packet.TracePacket // 原始 TRACE 包（引用，不要修改）
}

// TraceHandlerFunc 函数类型适配器
type TraceHandlerFunc func(ctx *TraceContext)

func (f TraceHandlerFunc) OnTrace(ctx *TraceContext) {
	f(ctx)
}

// ============================================================================
// 消息钩子 (MessageHandler)
// ============================================================================

// MessageHandler 消息处理器接口
type MessageHandler interface {
	// OnPublish 收到消息时调用
	// 使用原始 packet 引用，避免数据拷贝
	// 返回 false 时消息不会入队
	OnPublish(ctx *PublishContext) bool
}

// PublishContext 消息上下文
// 使用原始 packet 引用，避免数据拷贝
type PublishContext struct {
	ClientID string                // 客户端 ID
	Packet   *packet.PublishPacket // 原始 PUBLISH 包（引用，不要修改）
}

// MessageHandlerFunc 函数类型适配器
type MessageHandlerFunc func(ctx *PublishContext) bool

func (f MessageHandlerFunc) OnPublish(ctx *PublishContext) bool {
	return f(ctx)
}

// ============================================================================
// 请求钩子 (RequestHandler)
// ============================================================================

// RequestHandler 请求处理器接口
type RequestHandler interface {
	// OnRequest 收到请求时调用（在请求入队之前）
	// 使用原始 packet 引用，避免数据拷贝
	// 返回 false 时请求不会入队
	OnRequest(ctx *RequestContext) bool
}

// RequestContext 请求上下文
// 使用原始 packet 引用，避免数据拷贝
type RequestContext struct {
	ClientID string                // 客户端 ID
	Packet   *packet.RequestPacket // 原始 REQUEST 包（引用，不要修改）
}

// RequestHandlerFunc 函数类型适配器
type RequestHandlerFunc func(ctx *RequestContext) bool

func (f RequestHandlerFunc) OnRequest(ctx *RequestContext) bool {
	return f(ctx)
}

// ============================================================================
// 钩子集合 (Hooks)
// ============================================================================

// Hooks 钩子集合
type Hooks struct {
	// 连接管理钩子
	ConnectionHandler ConnectionHandler

	// 认证钩子（用于高级认证流程）
	AuthHandler AuthHandler

	// 追踪钩子
	TraceHandler TraceHandler

	// 消息钩子
	MessageHandler MessageHandler

	// 请求钩子
	RequestHandler RequestHandler
}

// callOnConnect 调用 OnConnect 钩子
func (h *Hooks) callOnConnect(ctx *ConnectContext) {
	if h.ConnectionHandler != nil {
		h.ConnectionHandler.OnConnect(ctx)
	}
}

// callOnDisconnect 调用 OnDisconnect 钩子
func (h *Hooks) callOnDisconnect(ctx *DisconnectContext) {
	if h.ConnectionHandler != nil {
		h.ConnectionHandler.OnDisconnect(ctx)
	}
}

// callOnAuth 调用 OnAuth 钩子
func (h *Hooks) callOnAuth(ctx *AuthContext) (bool, []byte, error) {
	if h.AuthHandler != nil {
		return h.AuthHandler.OnAuth(ctx)
	}
	return false, nil, nil
}

// callOnTrace 调用 OnTrace 钩子
func (h *Hooks) callOnTrace(ctx *TraceContext) {
	if h.TraceHandler != nil {
		h.TraceHandler.OnTrace(ctx)
	}
}

// callOnPublish 调用 OnPublish 钩子
func (h *Hooks) callOnPublish(ctx *PublishContext) bool {
	if h.MessageHandler != nil {
		return h.MessageHandler.OnPublish(ctx)
	}
	return true
}

// callOnRequest 调用 OnRequest 钩子
func (h *Hooks) callOnRequest(ctx *RequestContext) bool {
	if h.RequestHandler != nil {
		return h.RequestHandler.OnRequest(ctx)
	}
	return true
}
