package core

import (
	"errors"

	"github.com/snple/beacon/packet"
)

// ============================================================================
// 消息处理钩子 (MessageHandler)
// ============================================================================

// MessageHandler 消息生命周期处理器接口
// 用于在消息的不同生命周期阶段执行自定义逻辑
type MessageHandler interface {
	// OnPublish 消息发布时调用
	// 在消息进入 core 发布队列之前调用，此时可以访问原始 PUBLISH 包
	// 可用于：消息验证、消息转换、审计日志、访问控制等
	// 返回 error 时消息会被拒绝
	OnPublish(ctx *PublishContext) error

	// OnDeliver 消息投递时调用
	// 在消息即将投递给订阅者之前调用
	// 可用于：消息过滤、投递日志等
	// 返回 false 时消息不会投递给该订阅者
	OnDeliver(ctx *DeliverContext) bool
}

// PublishContext 发布消息上下文
// 使用原始 packet 引用，避免数据拷贝
type PublishContext struct {
	ClientID string                // 发布者客户端 ID
	Packet   *packet.PublishPacket // 原始 PUBLISH 包（引用，不要修改）
}

// DeliverContext 消息投递上下文
// 使用原始 packet 引用，避免数据拷贝
type DeliverContext struct {
	ClientID string                // 订阅者客户端 ID
	Packet   *packet.PublishPacket // 原始 PUBLISH 包（引用，不要修改）
}

// MessageHandlerFunc 函数类型适配器，用于简化只需实现部分方法的场景
type MessageHandlerFunc struct {
	PublishFunc func(ctx *PublishContext) error
	DeliverFunc func(ctx *DeliverContext) bool
}

func (h *MessageHandlerFunc) OnPublish(ctx *PublishContext) error {
	if h.PublishFunc != nil {
		return h.PublishFunc(ctx)
	}
	return nil
}

func (h *MessageHandlerFunc) OnDeliver(ctx *DeliverContext) bool {
	if h.DeliverFunc != nil {
		return h.DeliverFunc(ctx)
	}
	return true
}

// ============================================================================
// 连接管理钩子 (ConnectionHandler)
// ============================================================================

// ConnectionHandler 连接生命周期处理器接口
// 用于在客户端连接的不同阶段执行自定义逻辑
type ConnectionHandler interface {
	// OnConnect 客户端连接成功后调用
	// 在认证通过、发送 CONNACK 之后调用
	// 可用于：连接日志、资源初始化、通知其他系统等
	OnConnect(ctx *ConnectContext)

	// OnDisconnect 客户端断开连接时调用
	// 在客户端断开（正常或异常）后调用
	// 可用于：断开日志、资源清理、通知其他系统等
	OnDisconnect(ctx *DisconnectContext)
}

// ConnectContext 连接上下文
// 使用原始 packet 引用，避免数据拷贝
type ConnectContext struct {
	ClientID       string                // 客户端 ID
	RemoteAddr     string                // 远程地址
	Packet         *packet.ConnectPacket // 原始 CONNECT 包（引用，不要修改）
	SessionPresent bool                  // 是否恢复了旧会话
}

// DisconnectContext 断开连接上下文
// 使用原始 packet 引用，避免数据拷贝
type DisconnectContext struct {
	ClientID string                   // 客户端 ID
	Packet   *packet.DisconnectPacket // 原始 DISCONNECT 包（可能为 nil，如连接异常断开）
	Duration int64                    // 连接持续时间（秒）
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
// 订阅管理钩子 (SubscriptionHandler)
// ============================================================================

// SubscriptionHandler 订阅管理处理器接口
// 用于在订阅操作时执行自定义逻辑
type SubscriptionHandler interface {
	// OnSubscribe 客户端订阅主题时调用
	// 在订阅注册到 core 之前调用
	// 可用于：订阅权限检查、订阅日志等
	// 返回 error 时订阅会被拒绝
	OnSubscribe(ctx *SubscribeContext) error

	// OnUnsubscribe 客户端取消订阅时调用
	// 在取消订阅之前调用
	// 可用于：取消订阅日志等
	OnUnsubscribe(ctx *UnsubscribeContext)
}

// SubscribeContext 订阅上下文
// 使用原始 packet 引用，避免数据拷贝
type SubscribeContext struct {
	ClientID     string                  // 客户端 ID
	Packet       *packet.SubscribePacket // 原始 SUBSCRIBE 包（引用，不要修改）
	Subscription *packet.Subscription    // 当前正在处理的订阅项
}

// UnsubscribeContext 取消订阅上下文
// 使用原始 packet 引用，避免数据拷贝
type UnsubscribeContext struct {
	ClientID string                    // 客户端 ID
	Packet   *packet.UnsubscribePacket // 原始 UNSUBSCRIBE 包（引用，不要修改）
	Topic    string                    // 当前正在取消的主题
}

// SubscriptionHandlerFunc 函数类型适配器
type SubscriptionHandlerFunc struct {
	SubscribeFunc   func(ctx *SubscribeContext) error
	UnsubscribeFunc func(ctx *UnsubscribeContext)
}

func (h *SubscriptionHandlerFunc) OnSubscribe(ctx *SubscribeContext) error {
	if h.SubscribeFunc != nil {
		return h.SubscribeFunc(ctx)
	}
	return nil
}

func (h *SubscriptionHandlerFunc) OnUnsubscribe(ctx *UnsubscribeContext) {
	if h.UnsubscribeFunc != nil {
		h.UnsubscribeFunc(ctx)
	}
}

// ============================================================================
// 认证钩子 (AuthHandler)
// ============================================================================

// AuthHandler 认证处理器接口（合并了原 Authenticator 功能）
// 用于实现认证流程，包括：
// 1. 连接时的初始认证（CONNECT 包）
// 2. 高级多轮认证（AUTH 包，如 OAuth2, SASL 等）
// 3. 重新认证
type AuthHandler interface {
	// OnConnect 处理连接时的认证（CONNECT 包）
	// 返回 nil 表示认证成功
	// 返回 error 表示认证失败
	OnConnect(ctx *AuthConnectContext) error

	// OnAuth 处理 AUTH 包的认证交互
	// 客户端发送 AUTH 包时调用
	// 返回 (continue=true, authData, nil) 继续认证流程
	// 返回 (continue=false, authData, nil) 认证成功
	// 返回 error 认证失败
	OnAuth(ctx *AuthContext) (continued bool, authData []byte, err error)
}

// AuthConnectContext 连接认证上下文（用于 OnConnect）
// 使用原始 packet 引用，避免数据拷贝
type AuthConnectContext struct {
	ClientID   string                // 客户端 ID
	RemoteAddr string                // 远程地址
	Packet     *packet.ConnectPacket // 原始 CONNECT 包（引用，不要修改）

	// 认证器可以设置以下字段，用于返回给客户端
	ResponseProperties map[string]string // 返回给客户端的用户属性
}

// AuthContext 高级认证上下文（用于 OnAuth）
// 使用原始 packet 引用，避免数据拷贝
type AuthContext struct {
	ClientID   string             // 客户端 ID
	RemoteAddr string             // 远程地址
	Packet     *packet.AuthPacket // 原始 AUTH 包（引用，不要修改）

	// 认证器可以设置以下字段，用于返回给客户端
	ResponseProperties map[string]string // 返回给客户端的用户属性
}

// AuthHandlerFunc 函数类型适配器（简化版，只需实现部分方法）
type AuthHandlerFunc struct {
	ConnectFunc func(ctx *AuthConnectContext) error
	AuthFunc    func(ctx *AuthContext) (bool, []byte, error)
}

func (h *AuthHandlerFunc) OnConnect(ctx *AuthConnectContext) error {
	if h.ConnectFunc != nil {
		return h.ConnectFunc(ctx)
	}
	// 默认：允许所有连接
	return nil
}

func (h *AuthHandlerFunc) OnAuth(ctx *AuthContext) (bool, []byte, error) {
	if h.AuthFunc != nil {
		return h.AuthFunc(ctx)
	}
	// 默认：不支持高级认证
	return false, nil, errors.New("authentication not supported")
}

// ============================================================================
// 追踪钩子 (TraceHandler)
// ============================================================================

// TraceHandler 追踪处理器接口
// 用于收集和处理分布式追踪信息
type TraceHandler interface {
	// OnTrace 处理 TRACE 包
	// 收到追踪信息时调用
	// 可用于：分布式追踪、性能监控、调试等
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
// 请求钩子 (RequestHandler)
// ============================================================================

// RequestHandler 请求处理器接口
// 用于在请求的不同生命周期阶段执行自定义逻辑
type RequestHandler interface {
	// OnRequest 收到客户端请求时调用
	// 在请求进入队列之前调用
	// 可用于：请求验证、访问控制、审计日志等
	// 返回 error 时请求会被拒绝
	OnRequest(ctx *RequestContext) error

	// OnResponse 向客户端发送响应时调用
	// 在响应发送之前调用
	// 可用于：响应日志、响应转换等
	OnResponse(ctx *ResponseContext)
}

// RequestContext 请求钩子上下文
// 使用原始 packet 引用，避免数据拷贝
type RequestContext struct {
	ClientID string                // 请求来源客户端 ID
	Packet   *packet.RequestPacket // 原始 REQUEST 包（引用，不要修改）
}

// ResponseContext 响应钩子上下文
// 使用原始 packet 引用，避免数据拷贝
type ResponseContext struct {
	ClientID string                 // 目标客户端 ID
	Packet   *packet.ResponsePacket // 原始 RESPONSE 包（引用，不要修改）
}

// RequestHandlerFunc 函数类型适配器
type RequestHandlerFunc struct {
	RequestFunc  func(ctx *RequestContext) error
	ResponseFunc func(ctx *ResponseContext)
}

func (h *RequestHandlerFunc) OnRequest(ctx *RequestContext) error {
	if h.RequestFunc != nil {
		return h.RequestFunc(ctx)
	}
	return nil
}

func (h *RequestHandlerFunc) OnResponse(ctx *ResponseContext) {
	if h.ResponseFunc != nil {
		h.ResponseFunc(ctx)
	}
}

// ============================================================================
// 钩子管理器 (HookManager)
// ============================================================================

// Hooks 钩子集合
// 用于集中管理所有钩子处理器
type Hooks struct {
	// 消息处理钩子
	MessageHandler MessageHandler

	// 连接管理钩子
	ConnectionHandler ConnectionHandler

	// 订阅管理钩子
	SubscriptionHandler SubscriptionHandler

	// 认证钩子
	AuthHandler AuthHandler

	// 追踪钩子
	TraceHandler TraceHandler

	// 请求钩子
	RequestHandler RequestHandler
}

// callOnPublish 调用 OnPublish 钩子
func (h *Hooks) callOnPublish(ctx *PublishContext) error {
	if h.MessageHandler != nil {
		return h.MessageHandler.OnPublish(ctx)
	}
	return nil
}

// callOnDeliver 调用 OnDeliver 钩子
func (h *Hooks) callOnDeliver(ctx *DeliverContext) bool {
	if h.MessageHandler != nil {
		return h.MessageHandler.OnDeliver(ctx)
	}
	return true
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

// callOnSubscribe 调用 OnSubscribe 钩子
func (h *Hooks) callOnSubscribe(ctx *SubscribeContext) error {
	if h.SubscriptionHandler != nil {
		return h.SubscriptionHandler.OnSubscribe(ctx)
	}
	return nil
}

// callOnUnsubscribe 调用 OnUnsubscribe 钩子
func (h *Hooks) callOnUnsubscribe(ctx *UnsubscribeContext) {
	if h.SubscriptionHandler != nil {
		h.SubscriptionHandler.OnUnsubscribe(ctx)
	}
}

// callAuthOnConnect 调用 AuthHandler.OnConnect 钩子
func (h *Hooks) callAuthOnConnect(ctx *AuthConnectContext) error {
	if h.AuthHandler != nil {
		return h.AuthHandler.OnConnect(ctx)
	}
	// 默认：允许所有连接
	return nil
}

// callOnAuth 调用 OnAuth 钩子
func (h *Hooks) callOnAuth(ctx *AuthContext) (bool, []byte, error) {
	if h.AuthHandler != nil {
		return h.AuthHandler.OnAuth(ctx)
	}
	// 默认：不支持高级认证
	return false, nil, errors.New("authentication not supported")
}

// callOnTrace 调用 OnTrace 钩子
func (h *Hooks) callOnTrace(ctx *TraceContext) {
	if h.TraceHandler != nil {
		h.TraceHandler.OnTrace(ctx)
	}
}

// callOnRequest 调用 OnRequest 钩子
func (h *Hooks) callOnRequest(ctx *RequestContext) error {
	if h.RequestHandler != nil {
		return h.RequestHandler.OnRequest(ctx)
	}
	return nil
}

// callOnResponse 调用 OnResponse 钩子
func (h *Hooks) callOnResponse(ctx *ResponseContext) {
	if h.RequestHandler != nil {
		h.RequestHandler.OnResponse(ctx)
	}
}
