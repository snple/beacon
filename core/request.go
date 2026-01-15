package core

import (
	"context"
	"fmt"
	"time"

	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// ============================================================================
// REQUEST/RESPONSE API
// ============================================================================

// Request 接收到的来自客户端的请求（轮询模式）
type Request struct {
	RequestID      uint32            // 请求 ID（对于同一源客户端唯一）
	Action         string            // action 名称
	Payload        []byte            // 请求负载
	SourceClientID string            // 来源客户端 ID
	Timeout        uint32            // 超时时间（秒）
	TraceID        string            // 追踪 ID
	ContentType    string            // 内容类型
	UserProperties map[string]string // 用户自定义属性

	// 内部字段
	core *Core
}

// NewRequest 创建新的请求
func NewRequest(action string, payload []byte) *Request {
	return &Request{
		Action:  action,
		Payload: payload,
	}
}

// WithTimeout 设置请求超时（秒）
func (r *Request) WithTimeout(seconds uint32) *Request {
	r.Timeout = seconds
	return r
}

// WithTraceID 设置追踪 ID
func (r *Request) WithTraceID(traceID string) *Request {
	r.TraceID = traceID
	return r
}

// WithContentType 设置内容类型
func (r *Request) WithContentType(contentType string) *Request {
	r.ContentType = contentType
	return r
}

// WithUserProperties 设置用户属性
func (r *Request) WithUserProperties(props map[string]string) *Request {
	r.UserProperties = props
	return r
}

// WithUserProperty 设置单个用户属性
func (r *Request) WithUserProperty(key, value string) *Request {
	if r.UserProperties == nil {
		r.UserProperties = make(map[string]string)
	}
	r.UserProperties[key] = value
	return r
}

// Response 处理请求的结果
type Response struct {
	ReasonCode     packet.ReasonCode // 原因码
	Payload        []byte            // 响应负载
	ReasonString   string            // 原因字符串
	TraceID        string            // 追踪 ID
	ContentType    string            // 内容类型
	UserProperties map[string]string // 用户自定义属性
}

// NewResponse 创建新的响应（成功）
func NewResponse(payload []byte) *Response {
	return &Response{
		ReasonCode: packet.ReasonSuccess,
		Payload:    payload,
	}
}

// NewResponseWithCode 创建带有原因码的响应
func NewResponseWithCode(code packet.ReasonCode, payload []byte) *Response {
	return &Response{
		ReasonCode: code,
		Payload:    payload,
	}
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(code packet.ReasonCode, reason string) *Response {
	return &Response{
		ReasonCode:   code,
		ReasonString: reason,
	}
}

// WithReasonCode 设置原因码
func (r *Response) WithReasonCode(code packet.ReasonCode) *Response {
	r.ReasonCode = code
	return r
}

// WithReasonString 设置原因字符串
func (r *Response) WithReasonString(reason string) *Response {
	r.ReasonString = reason
	return r
}

// WithTraceID 设置追踪 ID
func (r *Response) WithTraceID(traceID string) *Response {
	r.TraceID = traceID
	return r
}

// WithContentType 设置内容类型
func (r *Response) WithContentType(contentType string) *Response {
	r.ContentType = contentType
	return r
}

// WithUserProperties 设置用户属性
func (r *Response) WithUserProperties(props map[string]string) *Response {
	r.UserProperties = props
	return r
}

// WithUserProperty 设置单个用户属性
func (r *Response) WithUserProperty(key, value string) *Response {
	if r.UserProperties == nil {
		r.UserProperties = make(map[string]string)
	}
	r.UserProperties[key] = value
	return r
}

// IsSuccess 检查响应是否成功
func (r *Response) IsSuccess() bool {
	return r.ReasonCode == packet.ReasonSuccess
}

// Error 返回错误信息（如果有）
func (r *Response) Error() string {
	if r.ReasonCode == packet.ReasonSuccess {
		return ""
	}
	if r.ReasonString != "" {
		return r.ReasonString
	}
	return r.ReasonCode.String()
}

// Response 响应一个接收到的请求
// 该方法将响应发送给请求的来源客户端
func (breq *Request) Response(result *Response) error {
	if breq.core == nil {
		return ErrRequestCoreNotSet
	}

	if result == nil {
		return ErrResultNil
	}

	return breq.core.respondToRequest(breq.SourceClientID, breq.RequestID, result)
}

// RespondSuccess 响应成功
func (breq *Request) RespondSuccess(payload []byte) error {
	return breq.Response(NewResponse(payload))
}

// RespondError 响应错误
func (breq *Request) RespondError(code packet.ReasonCode, reason string) error {
	return breq.Response(NewErrorResponse(code, reason))
}

// RequestOptions 请求选项（用于 core.RequestWithOptions）
type RequestOptions struct {
	TargetClientID string            // 目标客户端ID
	Timeout        time.Duration     // 超时时间（默认 30 秒）
	TraceID        string            // 追踪 ID
	ContentType    string            // 内容类型
	UserProperties map[string]string // 用户自定义属性
}

// DefaultRequestOptions 返回默认请求选项
func DefaultRequestOptions() *RequestOptions {
	return &RequestOptions{
		Timeout: 30 * time.Second,
	}
}

// WithTargetClientID 设置目标客户端ID
func (opts *RequestOptions) WithTargetClientID(clientID string) *RequestOptions {
	opts.TargetClientID = clientID
	return opts
}

// WithTimeout 设置超时时间
func (opts *RequestOptions) WithTimeout(timeout time.Duration) *RequestOptions {
	opts.Timeout = timeout
	return opts
}

// WithTraceID 设置追踪ID
func (opts *RequestOptions) WithTraceID(traceID string) *RequestOptions {
	opts.TraceID = traceID
	return opts
}

// WithContentType 设置内容类型
func (opts *RequestOptions) WithContentType(contentType string) *RequestOptions {
	opts.ContentType = contentType
	return opts
}

// WithUserProperties 设置用户属性
func (opts *RequestOptions) WithUserProperties(props map[string]string) *RequestOptions {
	opts.UserProperties = props
	return opts
}

// WithUserProperty 设置单个用户属性
func (opts *RequestOptions) WithUserProperty(key, value string) *RequestOptions {
	if opts.UserProperties == nil {
		opts.UserProperties = make(map[string]string)
	}
	opts.UserProperties[key] = value
	return opts
}

// Request 向客户端发送请求并等待响应
// 该方法会阻塞直到收到响应或超时
//
// 参数说明：
// - ctx: 上下文
// - action: 请求的 action 名称
// - payload: 请求负载数据
// - opts: 请求选项（可选），如果为 nil 则使用默认选项
//
// 路由逻辑：
// - 如果指定了 TargetClientID，则直接发送给该客户端
// - 如果未指定 TargetClientID，则通过 action 路由选择合适的客户端
//
// 返回值：
// - *Response: 响应结果（包含原因码、响应数据等）
// - error: 发生的错误（包括客户端不存在、超时等）
func (c *Core) Request(ctx context.Context, action string, payload []byte, opts *RequestOptions) (*Response, error) {
	if opts == nil {
		opts = DefaultRequestOptions()
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	var targetClientID string
	var client *Client

	if opts.TargetClientID != "" {
		// 指定了目标客户端，直接使用
		targetClientID = opts.TargetClientID
		c.clientsMu.RLock()
		client = c.clients[targetClientID]
		c.clientsMu.RUnlock()

		if client == nil {
			return nil, NewClientNotFoundError(targetClientID)
		}
	} else {
		// 未指定目标客户端，通过 action 路由选择
		var reasonCode packet.ReasonCode
		targetClientID, reasonCode = c.actionRegistry.SelectHandler(
			action,
			"",
			func(id string) *Client {
				c.clientsMu.RLock()
				defer c.clientsMu.RUnlock()
				return c.clients[id]
			},
		)

		if reasonCode != packet.ReasonSuccess {
			switch reasonCode {
			case packet.ReasonActionNotFound:
				return nil, NewActionNotFoundError(action)
			case packet.ReasonNoAvailableHandler:
				return nil, NewNoAvailableHandlerError(action)
			default:
				return nil, fmt.Errorf("failed to select handler: %s", reasonCode.String())
			}
		}

		c.clientsMu.RLock()
		client = c.clients[targetClientID]
		c.clientsMu.RUnlock()
	}

	if client == nil || client.IsClosed() {
		return nil, NewClientNotAvailableError(targetClientID)
	}

	// 生成请求 ID（使用 core 的请求 ID 生成器，避免与客户端冲突）
	coreRequestID := c.requestTracker.GenerateRequestID()

	// 创建响应通道
	respCh := make(chan *Response, 1)

	// 注册请求等待器
	c.requestTracker.RegisterResponseWaiter(coreRequestID, respCh)
	defer c.requestTracker.UnregisterResponseWaiter(coreRequestID)

	// 构建 REQUEST 包
	reqPkt := packet.NewRequestPacket(coreRequestID, action, payload)
	reqPkt.TargetClientID = targetClientID
	// 关键：设置 SourceClientID 为 "core" 来标记这是一个 core-initiated 请求
	// 客户端会在响应中将 TargetClientID 设置为此值，从而我们可以识别出来
	reqPkt.SourceClientID = "core"

	// 设置属性
	if timeout > 0 {
		timeoutSeconds := uint32(timeout.Seconds())
		if timeoutSeconds == 0 {
			timeoutSeconds = 1 // 最小 1 秒
		}
		reqPkt.Properties.Timeout = timeoutSeconds
	}
	reqPkt.Properties.TraceID = opts.TraceID
	reqPkt.Properties.ContentType = opts.ContentType
	reqPkt.Properties.UserProperties = opts.UserProperties

	// 发送请求包
	if err := client.WritePacket(reqPkt); err != nil {
		return nil, fmt.Errorf("failed to send REQUEST: %w", err)
	}

	c.logger.Debug("Sent REQUEST from core",
		zap.Uint32("requestID", coreRequestID),
		zap.String("action", action),
		zap.String("targetClientID", targetClientID))

	// 等待响应或超时
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case resp := <-respCh:
		if resp == nil {
			return nil, ErrResponseHandlerClosed
		}
		return resp, nil
	case <-timer.C:
		return nil, NewRequestTimeoutError(timeout)
	case <-c.ctx.Done():
		return nil, ErrCoreShutdown
	}
}

// RequestToClient 向指定客户端发送请求
func (c *Core) RequestToClient(ctx context.Context, targetClientID string, action string, payload []byte, opts *RequestOptions) (*Response, error) {
	if opts == nil {
		opts = DefaultRequestOptions()
	}

	return c.Request(ctx, action, payload, opts.WithTargetClientID(targetClientID))
}

// GetActionHandlers 获取指定 action 的所有处理者（客户端）
func (c *Core) GetActionHandlers(action string) []string {
	handlers := c.actionRegistry.GetHandlers(action)
	result := make([]string, 0, len(handlers))
	for _, h := range handlers {
		result = append(result, h.ClientID)
	}
	return result
}

// GetActionsForClient 获取指定客户端注册的所有 action
func (c *Core) GetActionsForClient(clientID string) []string {
	return c.actionRegistry.GetActionsForClient(clientID)
}

// respondToRequest 响应一个请求
// 用于向发送请求的客户端发送响应
func (c *Core) respondToRequest(sourceClientID string, requestID uint32, result *Response) error {
	if !c.running.Load() {
		return ErrCoreNotRunning
	}

	if sourceClientID == "" {
		return ErrSourceClientIDEmpty
	}

	// 查找目标客户端
	c.clientsMu.RLock()
	client := c.clients[sourceClientID]
	c.clientsMu.RUnlock()

	if client == nil {
		return NewClientNotFoundError(sourceClientID)
	}

	// 构建响应包
	resp := packet.NewResponsePacket(
		requestID,
		"",
		result.ReasonCode,
		result.Payload,
	)
	resp.Properties.ReasonString = result.ReasonString

	if result.TraceID != "" {
		resp.Properties.TraceID = result.TraceID
	}
	if result.ContentType != "" {
		resp.Properties.ContentType = result.ContentType
	}
	if len(result.UserProperties) > 0 {
		resp.Properties.UserProperties = result.UserProperties
	}

	// 发送响应
	return client.WritePacket(resp)
}

// ============================================================================
//  client 请求处理函数
// ============================================================================

// handleRegister 处理 REGISTER 包
func (c *Client) handleRegister(p *packet.RegisterPacket) error {
	c.core.logger.Debug("Handle REGISTER",
		zap.String("clientID", c.ID),
		zap.Strings("actions", p.Actions))

	// 获取并发处理能力
	var concurrency uint16 = 1
	if p.Properties != nil && p.Properties.Concurrency != nil {
		concurrency = *p.Properties.Concurrency
	}

	// 注册 actions
	results := c.core.actionRegistry.Register(c.ID, p.Actions, concurrency)

	// 构造响应
	regResults := make([]packet.RegisterResult, 0, len(results))
	for action, code := range results {
		regResults = append(regResults, packet.RegisterResult{
			Action:     action,
			ReasonCode: code,
		})
	}

	regack := packet.NewRegackPacket(regResults)
	return c.WritePacket(regack)
}

// handleUnregister 处理 UNREGISTER 包
func (c *Client) handleUnregister(p *packet.UnregisterPacket) error {
	c.core.logger.Debug("Handle UNREGISTER",
		zap.String("clientID", c.ID),
		zap.Strings("actions", p.Actions))

	// 注销 actions
	results := c.core.actionRegistry.Unregister(c.ID, p.Actions)

	// 构造响应
	unregResults := make([]packet.RegisterResult, 0, len(results))
	for action, code := range results {
		unregResults = append(unregResults, packet.RegisterResult{
			Action:     action,
			ReasonCode: code,
		})
	}

	unregack := packet.NewUnregackPacket(unregResults)
	return c.WritePacket(unregack)
}

// handleRequest 处理 REQUEST 包
func (c *Client) handleRequest(p *packet.RequestPacket) error {
	c.core.logger.Debug("Handle REQUEST",
		zap.String("clientID", c.ID),
		zap.Uint32("requestID", p.RequestID),
		zap.String("action", p.Action),
		zap.String("targetClientID", p.TargetClientID))

	// 调用 OnRequest 钩子（直接传递原始 packet）
	reqCtx := &RequestContext{
		ClientID: c.ID,
		Packet:   p,
	}
	if err := c.core.options.Hooks.callOnRequest(reqCtx); err != nil {
		c.core.logger.Debug("Request rejected by hook",
			zap.String("clientID", c.ID),
			zap.String("action", p.Action),
			zap.Error(err))
		resp := packet.NewResponsePacket(p.RequestID, c.ID, packet.ReasonNotAuthorized, nil)
		resp.Properties.ReasonString = err.Error()
		return c.WritePacket(resp)
	}

	// 验证 Action 名称
	if !validateActionName(p.Action) {
		resp := packet.NewResponsePacket(
			p.RequestID,
			c.ID,
			packet.ReasonActionInvalid,
			nil,
		)
		resp.Properties.ReasonString = fmt.Sprintf("Invalid action name: %s", p.Action)
		return c.WritePacket(resp)
	}

	// 填充 SourceClientID（由 core 填充，确保可信）
	p.SourceClientID = c.ID

	// 路径1：显式指定由 core 处理
	if p.TargetClientID == packet.TargetToCore {
		return c.enqueueRequest(p)
	}

	// 路径2：由指定的客户端或动态选择的客户端处理
	return c.handleRequestToClient(p)
}

// handleRequestToClient 处理发送给客户端的请求（路径2：由指定或动态选择的客户端处理）
func (c *Client) handleRequestToClient(p *packet.RequestPacket) error {
	var targetClientID string
	var reasonCode packet.ReasonCode

	if p.TargetClientID != "" {
		// 指定了目标客户端，检查是否存在
		targetClientID = p.TargetClientID
		c.core.clientsMu.RLock()
		client := c.core.clients[targetClientID]
		c.core.clientsMu.RUnlock()

		if client != nil {
			reasonCode = packet.ReasonSuccess
		} else {
			reasonCode = packet.ReasonClientNotFound
		}
	} else {
		// 通过 action 路由选择处理者
		targetClientID, reasonCode = c.core.actionRegistry.SelectHandler(
			p.Action,
			"",
			func(id string) *Client {
				c.core.clientsMu.RLock()
				defer c.core.clientsMu.RUnlock()
				return c.core.clients[id]
			},
		)
	}

	// 处理选择失败的情况
	if reasonCode != packet.ReasonSuccess {
		resp := packet.NewResponsePacket(p.RequestID, c.ID, reasonCode, nil)
		switch reasonCode {
		case packet.ReasonActionNotFound:
			resp.Properties.ReasonString = fmt.Sprintf("Action not found: %s", p.Action)
		case packet.ReasonClientNotFound:
			resp.Properties.ReasonString = fmt.Sprintf("Target client not found: %s", p.TargetClientID)
		case packet.ReasonNoAvailableHandler:
			resp.Properties.ReasonString = fmt.Sprintf("No available handler for action: %s", p.Action)
		}
		return c.WritePacket(resp)
	}

	// 追踪请求
	coreReqID := c.core.requestTracker.Track(p, targetClientID)

	// 获取目标客户端并发送请求
	c.core.clientsMu.RLock()
	targetClient := c.core.clients[targetClientID]
	c.core.clientsMu.RUnlock()

	if targetClient == nil {
		// 目标客户端已断开
		c.core.requestTracker.CompleteByCoreID(coreReqID)
		resp := packet.NewResponsePacket(p.RequestID, c.ID, packet.ReasonClientNotFound, nil)
		resp.Properties.ReasonString = "Target client not found"
		return c.WritePacket(resp)
	}

	// 构造发送给目标客户端的请求包（使用 coreRequestID）
	forwardPkt := packet.NewRequestPacket(uint32(coreReqID), p.Action, p.Payload)
	forwardPkt.SourceClientID = p.SourceClientID
	forwardPkt.TargetClientID = p.TargetClientID
	if p.Properties != nil {
		forwardPkt.Properties.Timeout = p.Properties.Timeout
		forwardPkt.Properties.TraceID = p.Properties.TraceID
		forwardPkt.Properties.ContentType = p.Properties.ContentType
	}

	if err := targetClient.WritePacket(forwardPkt); err != nil {
		// 发送失败，完成追踪并返回错误
		c.core.requestTracker.CompleteByCoreID(coreReqID)
		resp := packet.NewResponsePacket(p.RequestID, c.ID, packet.ReasonClientNotFound, nil)
		resp.Properties.ReasonString = "Target client disconnected"
		return c.WritePacket(resp)
	}

	return nil
}

// handleResponse 处理 RESPONSE 包
// 注意：收到的 p.RequestID 实际上是 coreRequestID（由 core 在转发 REQUEST 时设置）
func (c *Client) handleResponse(p *packet.ResponsePacket) error {
	c.core.logger.Debug("Handle RESPONSE",
		zap.String("clientID", c.ID),
		zap.Uint32("requestID", p.RequestID),
		zap.String("targetClientID", p.TargetClientID),
		zap.Uint8("reasonCode", uint8(p.ReasonCode)))

	// p.RequestID 在此场景中实际上是 coreRequestID
	coreRequestID := uint64(p.RequestID)

	// 首先检查这是否是 core-initiated 请求的响应
	// core-initiated 请求的 TargetClientID 会被设置为 "core"（因为源客户端设置为了 "core"）
	if p.TargetClientID == "core" {
		return c.handlecoreInitiatedResponse(p, coreRequestID)
	}

	// 通过 coreRequestID 完成请求追踪
	pr := c.core.requestTracker.CompleteByCoreID(coreRequestID)
	if pr == nil {
		c.core.logger.Warn("Response for unknown request",
			zap.String("clientID", c.ID),
			zap.Uint64("coreRequestID", coreRequestID))
		return nil
	}

	// 验证响应来源
	if pr.TargetClientID != c.ID {
		c.core.logger.Warn("Response from wrong client",
			zap.String("clientID", c.ID),
			zap.String("expectedClient", pr.TargetClientID),
			zap.Uint64("coreRequestID", coreRequestID))
		return nil
	}

	// 获取源客户端
	c.core.clientsMu.RLock()
	sourceClient := c.core.clients[pr.SourceClientID]
	c.core.clientsMu.RUnlock()

	if sourceClient == nil {
		c.core.logger.Debug("Source client not found",
			zap.String("sourceClientID", pr.SourceClientID),
			zap.Uint64("coreRequestID", coreRequestID))
		return nil
	}

	// 构造响应包，使用客户端原始 RequestID
	resp := packet.NewResponsePacket(
		pr.ClientRequestID,
		pr.SourceClientID,
		p.ReasonCode,
		p.Payload,
	)
	if p.Properties != nil {
		resp.Properties.ReasonString = p.Properties.ReasonString
		resp.Properties.TraceID = p.Properties.TraceID
		resp.Properties.ContentType = p.Properties.ContentType
		resp.Properties.UserProperties = p.Properties.UserProperties
	}

	// 调用 OnResponse 钩子（直接传递原始 packet）
	respCtx := &ResponseContext{
		ClientID: pr.SourceClientID,
		Packet:   resp,
		Action:   pr.Action,
	}
	c.core.options.Hooks.callOnResponse(respCtx)

	return sourceClient.WritePacket(resp)
}

// handlecoreInitiatedResponse 处理来自客户端对 core-initiated 请求的响应
func (c *Client) handlecoreInitiatedResponse(p *packet.ResponsePacket, coreRequestID uint64) error {
	// 获取响应等待器
	respCh := c.core.requestTracker.GetResponseWaiter(uint32(coreRequestID))
	if respCh == nil {
		c.core.logger.Debug("No waiter for core-initiated response",
			zap.Uint64("requestID", coreRequestID))
		return nil
	}

	// 构造结果对象
	result := &Response{
		ReasonCode:     p.ReasonCode,
		Payload:        p.Payload,
		ContentType:    p.Properties.ContentType,
		UserProperties: p.Properties.UserProperties,
		TraceID:        p.Properties.TraceID,
	}
	if p.Properties != nil {
		result.ReasonString = p.Properties.ReasonString
	}

	// 发送到等待器
	select {
	case respCh <- result:
		c.core.logger.Debug("Sent core-initiated response to waiter",
			zap.Uint64("requestID", coreRequestID))
	default:
		c.core.logger.Warn("Response waiter channel full or closed",
			zap.Uint64("requestID", coreRequestID))
	}

	return nil
}
