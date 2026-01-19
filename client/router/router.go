package router

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// ============================================================================
// ROUTER - HTTP 风格的消息/请求路由器（客户端版本）
// ============================================================================

// MessageContext 路由消息上下文（类似 HTTP Request）
type MessageContext struct {
	Message  *client.Message   // 原始消息
	ClientID string            // 发送者客户端 ID
	Params   map[string]string // 主题参数（来自通配符匹配）

	// 内部
	router *Router
}

// Topic 获取消息主题
func (ctx *MessageContext) Topic() string {
	if ctx.Message != nil && ctx.Message.Packet != nil {
		return ctx.Message.Packet.Topic
	}
	return ""
}

// Payload 获取消息负载
func (ctx *MessageContext) Payload() []byte {
	if ctx.Message != nil && ctx.Message.Packet != nil {
		return ctx.Message.Packet.Payload
	}
	return nil
}

// RequestContext 路由请求上下文（类似 HTTP Request）
type RequestContext struct {
	Request   *client.Request   // 原始请求
	Params    map[string]string // Action 参数（来自通配符匹配）
	responded bool              // 是否已响应

	// 内部
	router *Router
}

// Respond 响应请求
func (ctx *RequestContext) Respond(result *client.Response) error {
	if ctx.responded {
		return fmt.Errorf("request already responded")
	}
	ctx.responded = true
	return ctx.Request.Response(result)
}

// RespondSuccess 响应成功
func (ctx *RequestContext) RespondSuccess(payload []byte) error {
	return ctx.Respond(client.NewResponse(payload))
}

// RespondError 响应错误
func (ctx *RequestContext) RespondError(code packet.ReasonCode, reason string) error {
	return ctx.Respond(client.NewErrorResponse(code, reason))
}

// Payload 获取请求负载
func (ctx *RequestContext) Payload() []byte {
	if ctx.Request != nil && ctx.Request.Packet != nil {
		return ctx.Request.Packet.Payload
	}
	return nil
}

// SourceClientID 获取请求的源客户端 ID
func (ctx *RequestContext) SourceClientID() string {
	if ctx.Request != nil && ctx.Request.Packet != nil {
		return ctx.Request.Packet.SourceClientID
	}
	return ""
}

// RouterMessageHandlerFunc 消息处理函数类型
type RouterMessageHandlerFunc func(ctx *MessageContext)

// RouterRequestHandlerFunc 请求处理函数类型
type RouterRequestHandlerFunc func(ctx *RequestContext)

// MessageMiddleware 消息中间件函数类型
type MessageMiddleware func(ctx *MessageContext, next func())

// RequestMiddleware 请求中间件函数类型
type RequestMiddleware func(ctx *RequestContext, next func())

// Router 消息/请求路由器
// 提供类似 HTTP 路由器的接口来处理消息和请求
type Router struct {
	client *client.Client
	logger *zap.Logger

	// 消息路由
	messageHandlers    []*messageRoute
	messageMiddlewares []MessageMiddleware
	messageMu          sync.RWMutex

	// 请求路由
	requestHandlers    []*requestRoute
	requestMiddlewares []RequestMiddleware
	requestMu          sync.RWMutex

	// 控制
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	pollDelay time.Duration
}

// messageRoute 消息路由条目
type messageRoute struct {
	pattern string                   // 主题模式（支持 + 和 # 通配符）
	handler RouterMessageHandlerFunc // 处理函数
	matcher *topicMatcher            // 主题匹配器
}

// requestRoute 请求路由条目
type requestRoute struct {
	pattern string                   // Action 模式（支持 * 和 ** 通配符）
	handler RouterRequestHandlerFunc // 处理函数
	matcher *actionMatcher           // Action 匹配器
}

// RouterOptions 路由器选项
type RouterOptions struct {
	PollTimeout time.Duration // 轮询超时（默认 5 秒）
	Logger      *zap.Logger   // 日志器
}

// DefaultRouterOptions 默认路由器选项
func DefaultRouterOptions() *RouterOptions {
	return &RouterOptions{
		PollTimeout: 5 * time.Second,
	}
}

// NewRouter 创建新的客户端路由器
func NewRouter(client *client.Client, opts *RouterOptions) *Router {
	if opts == nil {
		opts = DefaultRouterOptions()
	}

	logger := opts.Logger
	if logger == nil {
		logger = client.GetLogger()
		if logger == nil {
			logger, _ = zap.NewDevelopment()
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Router{
		client:    client,
		logger:    logger,
		pollDelay: opts.PollTimeout,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// ============================================================================
// 消息路由 API
// ============================================================================

// HandleMessage 注册消息处理器
// 主题模式支持通配符：
//   - * 匹配单层（如 "sensor/*/data" 匹配 "sensor/room1/data"）
//   - ** 匹配多层（如 "sensor/**" 匹配 "sensor/room1/temp"）
//
// 注意：调用此方法会自动订阅匹配的主题
//
// 示例：
//
//	router.HandleMessage("sensor/*/temperature", func(ctx *MessageContext) {
//	    roomID := ctx.Params["*1"] // 获取第一个 * 通配符匹配的值
//	    fmt.Printf("Room %s temperature: %s\n", roomID, ctx.Payload)
//	})
//
//	router.HandleMessage("device/**", func(ctx *MessageContext) {
//	    fmt.Printf("Device message on %s\n", ctx.Topic)
//	})
func (r *Router) HandleMessage(pattern string, handler RouterMessageHandlerFunc) {
	r.messageMu.Lock()
	defer r.messageMu.Unlock()

	route := &messageRoute{
		pattern: pattern,
		handler: handler,
		matcher: newTopicMatcher(pattern),
	}
	r.messageHandlers = append(r.messageHandlers, route)

	// 自动订阅主题
	if r.client.IsConnected() {
		r.client.Subscribe(pattern)
	}
}

// UseMessage 添加消息中间件
// 中间件按添加顺序执行，必须调用 next() 继续执行链
//
// 示例：
//
//	router.UseMessage(func(ctx *MessageContext, next func()) {
//	    start := time.Now()
//	    next() // 继续执行
//	    fmt.Printf("Message processed in %v\n", time.Since(start))
//	})
func (r *Router) UseMessage(middleware MessageMiddleware) {
	r.messageMu.Lock()
	defer r.messageMu.Unlock()
	r.messageMiddlewares = append(r.messageMiddlewares, middleware)
}

// ============================================================================
// 请求路由 API
// ============================================================================

// HandleRequest 注册请求处理器
// Action 模式支持类似路径的通配符：
//   - * 匹配单段（如 "user.*.get" 匹配 "user.123.get"）
//   - ** 匹配多段（如 "api.**" 匹配 "api.v1.user.list"）
//
// 注意：调用此方法会自动注册 action
//
// 示例：
//
//	router.HandleRequest("user.*.get", func(ctx *RequestContext) {
//	    userID := ctx.Params["*1"] // 获取第一个 * 通配符匹配的值
//	    // 处理请求...
//	    ctx.RespondSuccess([]byte(`{"id": "` + userID + `"}`))
//	})
//
//	router.HandleRequest("device.control", func(ctx *RequestContext) {
//	    // 处理设备控制请求...
//	    ctx.RespondSuccess(nil)
//	})
func (r *Router) HandleRequest(pattern string, handler RouterRequestHandlerFunc) {
	r.requestMu.Lock()
	defer r.requestMu.Unlock()

	route := &requestRoute{
		pattern: pattern,
		handler: handler,
		matcher: newActionMatcher(pattern),
	}
	r.requestHandlers = append(r.requestHandlers, route)

	// 自动注册 action（如果不是通配符模式）
	if !strings.Contains(pattern, "*") && r.client.IsConnected() {
		r.client.Register(pattern)
	}
}

// UseRequest 添加请求中间件
// 中间件按添加顺序执行，必须调用 next() 继续执行链
//
// 示例：
//
//	router.UseRequest(func(ctx *RequestContext, next func()) {
//	    // 认证检查
//	    if ctx.ClientID == "" {
//	        ctx.RespondError(0x87, "unauthorized")
//	        return
//	    }
//	    next()
//	})
func (r *Router) UseRequest(middleware RequestMiddleware) {
	r.requestMu.Lock()
	defer r.requestMu.Unlock()
	r.requestMiddlewares = append(r.requestMiddlewares, middleware)
}

// ============================================================================
// 路由器启动/停止
// ============================================================================

// Start 启动路由器
// 开始轮询客户端的消息和请求
func (r *Router) Start() error {
	r.logger.Info("Starting client router")

	// 确保订阅和注册已生效
	r.registerRoutes()

	// 启动消息轮询
	r.wg.Add(1)
	go r.messageLoop()

	// 启动请求轮询
	r.wg.Add(1)
	go r.requestLoop()

	return nil
}

// registerRoutes 注册所有路由的订阅和 action
func (r *Router) registerRoutes() {
	r.messageMu.RLock()
	for _, route := range r.messageHandlers {
		r.client.Subscribe(route.pattern)
	}
	r.messageMu.RUnlock()

	r.requestMu.RLock()
	for _, route := range r.requestHandlers {
		// 只注册非通配符模式
		if !strings.Contains(route.pattern, "*") {
			r.client.Register(route.pattern)
		}
	}
	r.requestMu.RUnlock()
}

// Stop 停止路由器
func (r *Router) Stop() {
	r.logger.Info("Stopping client router")
	r.cancel()
	r.wg.Wait()
}

// messageLoop 消息轮询循环
func (r *Router) messageLoop() {
	defer r.wg.Done()

	for {
		select {
		case <-r.ctx.Done():
			return
		default:
		}

		msg, err := r.client.PollMessage(r.ctx, r.pollDelay)
		if err != nil {
			if r.ctx.Err() != nil {
				return // 上下文已取消
			}
			if errors.Is(err, client.ErrNotConnected) {
				// 客户端未连接，继续轮询
				time.Sleep(500 * time.Millisecond)
				continue
			}
			if errors.Is(err, client.ErrPollTimeout) {
				continue // 超时，继续轮询
			}
			if errors.Is(err, client.ErrClientClosed) {
				return // core 停止，优雅退出
			}
			r.logger.Error("PollMessage error", zap.Error(err))
			continue
		}

		r.dispatchMessage(msg)
	}
}

// requestLoop 请求轮询循环
func (r *Router) requestLoop() {
	defer r.wg.Done()

	for {
		select {
		case <-r.ctx.Done():
			return
		default:
		}

		req, err := r.client.PollRequest(r.ctx, r.pollDelay)
		if err != nil {
			if r.ctx.Err() != nil {
				return // 上下文已取消
			}
			if errors.Is(err, client.ErrNotConnected) {
				// 客户端未连接，继续轮询
				time.Sleep(500 * time.Millisecond)
				continue
			}
			if errors.Is(err, client.ErrPollTimeout) {
				continue // 超时，继续轮询
			}
			if errors.Is(err, client.ErrClientClosed) {
				return // core 停止，优雅退出
			}
			r.logger.Error("PollRequest error", zap.Error(err))
			continue
		}

		r.dispatchRequest(req)
	}
}

// dispatchMessage 分发消息到匹配的处理器
func (r *Router) dispatchMessage(msg *client.Message) {
	r.messageMu.RLock()
	handlers := make([]*messageRoute, len(r.messageHandlers))
	copy(handlers, r.messageHandlers)
	middlewares := make([]MessageMiddleware, len(r.messageMiddlewares))
	copy(middlewares, r.messageMiddlewares)
	r.messageMu.RUnlock()

	// 查找匹配的处理器
	for _, route := range handlers {
		params, matched := route.matcher.match(msg.Packet.Topic)
		if !matched {
			continue
		}

		clientId := ""
		if msg != nil && msg.Packet.Properties != nil {
			clientId = msg.Packet.Properties.SourceClientID
		}

		ctx := &MessageContext{
			Message:  msg,
			ClientID: clientId,
			Params:   params,
			router:   r,
		}

		// 构建中间件链
		r.runMessageChain(ctx, middlewares, route.handler)
	}
}

// dispatchRequest 分发请求到匹配的处理器
func (r *Router) dispatchRequest(req *client.Request) {
	r.requestMu.RLock()
	handlers := make([]*requestRoute, len(r.requestHandlers))
	copy(handlers, r.requestHandlers)
	middlewares := make([]RequestMiddleware, len(r.requestMiddlewares))
	copy(middlewares, r.requestMiddlewares)
	r.requestMu.RUnlock()

	// 查找匹配的处理器
	var matchedRoute *requestRoute
	var params map[string]string

	for _, route := range handlers {
		p, matched := route.matcher.match(req.Packet.Action)
		if matched {
			matchedRoute = route
			params = p
			break
		}
	}

	ctx := &RequestContext{
		Request: req,
		Params:  params,
		router:  r,
	}

	if matchedRoute == nil {
		// 未找到处理器，返回错误
		r.logger.Debug("No handler found for action", zap.String("action", req.Packet.Action))
		ctx.RespondError(packet.ReasonUnspecifiedError, "action not found: "+req.Packet.Action)
		return
	}

	// 构建中间件链并执行
	r.runRequestChain(ctx, middlewares, matchedRoute.handler)

	// 如果处理器没有响应，返回默认错误
	if !ctx.responded {
		r.logger.Warn("Handler did not respond", zap.String("action", req.Packet.Action))
		ctx.RespondError(packet.ReasonUnspecifiedError, "handler did not respond")
	}
}

// runMessageChain 执行消息中间件链
func (r *Router) runMessageChain(ctx *MessageContext, middlewares []MessageMiddleware, handler RouterMessageHandlerFunc) {
	index := 0
	var next func()
	next = func() {
		if index < len(middlewares) {
			mw := middlewares[index]
			index++
			mw(ctx, next)
		} else {
			handler(ctx)
		}
	}
	next()
}

// runRequestChain 执行请求中间件链
func (r *Router) runRequestChain(ctx *RequestContext, middlewares []RequestMiddleware, handler RouterRequestHandlerFunc) {
	index := 0
	var next func()
	next = func() {
		if index < len(middlewares) {
			mw := middlewares[index]
			index++
			mw(ctx, next)
		} else {
			handler(ctx)
		}
	}
	next()
}

// ============================================================================
// 主题/Action 匹配器
// ============================================================================

// topicMatcher 主题匹配器
// 支持 * 和 ** 通配符
type topicMatcher struct {
	pattern  string
	segments []string
	hasMulti bool // 是否包含 **
}

func newTopicMatcher(pattern string) *topicMatcher {
	segments := strings.Split(pattern, "/")
	hasMulti := len(segments) > 0 && segments[len(segments)-1] == "**"

	return &topicMatcher{
		pattern:  pattern,
		segments: segments,
		hasMulti: hasMulti,
	}
}

func (m *topicMatcher) match(topic string) (map[string]string, bool) {
	topicSegments := strings.Split(topic, "/")
	params := make(map[string]string)

	starIndex := 0

	for i, seg := range m.segments {
		if seg == "**" {
			// ** 匹配剩余所有段
			if i < len(topicSegments) {
				params["**"] = strings.Join(topicSegments[i:], "/")
			}
			return params, true
		}

		if i >= len(topicSegments) {
			return nil, false
		}

		if seg == "*" {
			// * 匹配单段
			starIndex++
			params[fmt.Sprintf("*%d", starIndex)] = topicSegments[i]
		} else if seg != topicSegments[i] {
			return nil, false
		}
	}

	// 检查长度是否匹配（没有 ** 时必须完全匹配）
	if !m.hasMulti && len(topicSegments) != len(m.segments) {
		return nil, false
	}

	return params, true
}

// actionMatcher Action 匹配器
type actionMatcher struct {
	pattern  string
	segments []string
	hasMulti bool // 是否包含 **
}

func newActionMatcher(pattern string) *actionMatcher {
	segments := strings.Split(pattern, ".")
	hasMulti := len(segments) > 0 && segments[len(segments)-1] == "**"

	return &actionMatcher{
		pattern:  pattern,
		segments: segments,
		hasMulti: hasMulti,
	}
}

func (m *actionMatcher) match(action string) (map[string]string, bool) {
	actionSegments := strings.Split(action, ".")
	params := make(map[string]string)

	starIndex := 0

	for i, seg := range m.segments {
		if seg == "**" {
			// ** 匹配剩余所有段
			if i < len(actionSegments) {
				params["**"] = strings.Join(actionSegments[i:], ".")
			}
			return params, true
		}

		if i >= len(actionSegments) {
			return nil, false
		}

		if seg == "*" {
			// * 匹配单段
			starIndex++
			params[fmt.Sprintf("*%d", starIndex)] = actionSegments[i]
		} else if seg != actionSegments[i] {
			return nil, false
		}
	}

	// 检查长度是否匹配（没有 ** 时必须完全匹配）
	if !m.hasMulti && len(actionSegments) != len(m.segments) {
		return nil, false
	}

	return params, true
}

// ============================================================================
// 路由组（Group）支持
// ============================================================================

// RouterGroup 路由组，用于组织相关路由
type RouterGroup struct {
	router    *Router
	prefix    string // 消息前缀（主题）
	actionPfx string // 请求前缀（action）
}

// Group 创建路由组
// 示例：
//
//	api := router.Group("api/v1", "api.v1")
//	api.HandleMessage("/users", handler)  // 完整路径: api/v1/users
//	api.HandleRequest(".list", handler)   // 完整 action: api.v1.list
func (r *Router) Group(topicPrefix, actionPrefix string) *RouterGroup {
	return &RouterGroup{
		router:    r,
		prefix:    strings.TrimSuffix(topicPrefix, "/"),
		actionPfx: strings.TrimSuffix(actionPrefix, "."),
	}
}

// HandleMessage 在组内注册消息处理器
func (g *RouterGroup) HandleMessage(pattern string, handler RouterMessageHandlerFunc) {
	fullPattern := g.prefix
	if pattern != "" {
		if !strings.HasPrefix(pattern, "/") {
			fullPattern += "/"
		}
		fullPattern += pattern
	}
	g.router.HandleMessage(fullPattern, handler)
}

// HandleRequest 在组内注册请求处理器
func (g *RouterGroup) HandleRequest(pattern string, handler RouterRequestHandlerFunc) {
	fullPattern := g.actionPfx
	if pattern != "" {
		if !strings.HasPrefix(pattern, ".") {
			fullPattern += "."
		}
		fullPattern += pattern
	}
	g.router.HandleRequest(fullPattern, handler)
}

// Group 创建子组
func (g *RouterGroup) Group(topicSuffix, actionSuffix string) *RouterGroup {
	newTopicPrefix := g.prefix
	if topicSuffix != "" {
		if !strings.HasPrefix(topicSuffix, "/") {
			newTopicPrefix += "/"
		}
		newTopicPrefix += topicSuffix
	}

	newActionPrefix := g.actionPfx
	if actionSuffix != "" {
		if !strings.HasPrefix(actionSuffix, ".") {
			newActionPrefix += "."
		}
		newActionPrefix += actionSuffix
	}

	return &RouterGroup{
		router:    g.router,
		prefix:    newTopicPrefix,
		actionPfx: newActionPrefix,
	}
}
