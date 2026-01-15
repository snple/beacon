package client

import (
	"context"
	"fmt"
	"time"

	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// Request 接收到的请求
type Request struct {
	RequestID      uint32            // 请求 ID
	Action         string            // action 名称
	Payload        []byte            // 请求负载
	SourceClientID string            // 来源客户端 ID
	Timeout        uint32            // 超时时间（相对时间，单位：秒）
	TraceID        string            // 追踪 ID
	ContentType    string            // 内容类型
	UserProperties map[string]string // 用户自定义属性

	// 内部字段
	client *Client
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

// Response 响应
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
// 该方法将响应发送给请求的来源
func (req *Request) Response(result *Response) error {
	if req.client == nil {
		return ErrRequestClientNotSet
	}

	if result == nil {
		return ErrResultNil
	}

	// 构建响应包
	resp := packet.NewResponsePacket(
		req.RequestID,
		req.SourceClientID,
		result.ReasonCode,
		result.Payload,
	)

	if result.ReasonString != "" {
		resp.Properties.ReasonString = result.ReasonString
	}
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
	return req.client.writePacket(resp)
}

// RespondSuccess 响应成功
func (req *Request) RespondSuccess(payload []byte) error {
	return req.Response(NewResponse(payload))
}

// RespondError 响应错误
func (req *Request) RespondError(code packet.ReasonCode, reason string) error {
	return req.Response(NewErrorResponse(code, reason))
}

// Request 发送请求并等待响应
func (c *Client) Request(ctx context.Context, action string, payload []byte, opts *RequestOptions) (*Response, error) {
	if !c.connected.Load() {
		return nil, ErrNotConnected
	}

	if opts == nil {
		opts = NewRequestOptions()
	}

	// 设置默认超时
	if opts.Timeout == 0 {
		opts.Timeout = defaultRequestTimeout
	}

	// 生成请求 ID
	requestID := c.nextRequestID.Add(1)

	// 创建响应通道
	respCh := make(chan *Response, 1)
	c.pendingReqMu.Lock()
	if c.pendingRequests == nil {
		c.pendingRequests = make(map[uint32]chan *Response)
	}
	c.pendingRequests[requestID] = respCh
	c.pendingReqMu.Unlock()

	// 确保清理
	defer func() {
		c.pendingReqMu.Lock()
		delete(c.pendingRequests, requestID)
		c.pendingReqMu.Unlock()
	}()

	// 构造 REQUEST 包
	reqPkt := packet.NewRequestPacket(requestID, action, payload)
	reqPkt.TargetClientID = opts.TargetClientID

	// 设置属性
	if opts.Timeout > 0 {
		// 将超时时间转换为秒数（相对时间）
		timeout := uint32(opts.Timeout.Seconds())
		reqPkt.Properties.Timeout = timeout
	}
	reqPkt.Properties.TraceID = opts.TraceID
	reqPkt.Properties.ContentType = opts.ContentType
	reqPkt.Properties.UserProperties = opts.UserProperties

	// 发送请求
	if err := c.writePacket(reqPkt); err != nil {
		return nil, fmt.Errorf("failed to send REQUEST: %w", err)
	}

	c.logger.Debug("Sent REQUEST",
		zap.Uint32("requestID", requestID),
		zap.String("action", action),
		zap.String("targetClientID", opts.TargetClientID))

	// 等待响应或超时
	timer := time.NewTimer(opts.Timeout)
	defer timer.Stop()

	select {
	case resp := <-respCh:
		return resp, nil
	case <-timer.C:
		return nil, NewRequestTimeoutError(opts.Timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, ErrClientClosed
	}
}

// RequestToClient 发送请求到指定客户端
func (c *Client) RequestToClient(ctx context.Context, targetClientID, action string, payload []byte, opts *RequestOptions) (*Response, error) {
	if opts == nil {
		opts = NewRequestOptions()
	}
	return c.Request(ctx, action, payload, opts.WithTargetClientID(targetClientID))
}

// RequestToCore 发送请求到 Core（使用 "core" 作为目标）
func (c *Client) RequestToCore(ctx context.Context, action string, payload []byte, opts *RequestOptions) (*Response, error) {
	if opts == nil {
		opts = NewRequestOptions()
	}
	return c.Request(ctx, action, payload, opts.WithTargetClientID(packet.TargetToCore))
}

// handleRequest 处理收到的 REQUEST 包
// 采用轮询模式：将请求放入队列，让外部代码通过 PollRequest() 接收并处理
func (c *Client) handleRequest(pkt *packet.RequestPacket) error {
	c.logger.Debug("Received REQUEST",
		zap.Uint32("requestID", pkt.RequestID),
		zap.String("action", pkt.Action),
		zap.String("sourceClientID", pkt.SourceClientID))

	// 构造 Request 对象
	req := &Request{
		RequestID:      pkt.RequestID,
		Action:         pkt.Action,
		Payload:        pkt.Payload,
		SourceClientID: pkt.SourceClientID,
		client:         c,
	}

	if pkt.Properties != nil {
		req.Timeout = pkt.Properties.Timeout
		req.TraceID = pkt.Properties.TraceID
		req.ContentType = pkt.Properties.ContentType
		req.UserProperties = pkt.Properties.UserProperties
	}

	// 调用 OnRequest 钩子（直接传递原始 packet）
	if !c.options.Hooks.callOnRequest(&RequestContext{ClientID: c.clientID, Packet: pkt}) {
		c.logger.Debug("Request rejected by hook",
			zap.Uint32("requestID", pkt.RequestID),
			zap.String("action", pkt.Action))
		resp := packet.NewResponsePacket(
			pkt.RequestID,
			pkt.SourceClientID,
			packet.ReasonNotAuthorized,
			nil,
		)
		resp.Properties.ReasonString = "Request rejected by hook"
		return c.writePacket(resp)
	}

	// 轮询模式：检查是否有请求队列，如果有则放入队列
	c.requestQueueMu.Lock()
	if c.requestQueue == nil {
		c.requestQueueMu.Unlock()

		// 没有请求队列，返回错误
		resp := packet.NewResponsePacket(
			pkt.RequestID,
			pkt.SourceClientID,
			packet.ReasonActionNotFound,
			nil,
		)
		resp.Properties.ReasonString = fmt.Sprintf("No polling handler for action: %s", pkt.Action)
		return c.writePacket(resp)
	}
	c.requestQueueMu.Unlock()

	// 非阻塞尝试放入队列
	select {
	case c.requestQueue <- req:
		// 成功放入队列
		c.logger.Debug("Enqueued request for polling",
			zap.Uint32("requestID", pkt.RequestID),
			zap.String("action", pkt.Action))
		return nil
	default:
		// 队列已满，返回服务器繁忙错误
		resp := packet.NewResponsePacket(
			pkt.RequestID,
			pkt.SourceClientID,
			packet.ReasonServerBusy,
			nil,
		)
		resp.Properties.ReasonString = "Client request queue full"
		return c.writePacket(resp)
	}
}

// handleResponse 处理收到的 RESPONSE 包
func (c *Client) handleResponse(pkt *packet.ResponsePacket) error {
	c.logger.Debug("Received RESPONSE",
		zap.Uint32("requestID", pkt.RequestID),
		zap.String("reasonCode", pkt.ReasonCode.String()),
		zap.String("targetClientID", pkt.TargetClientID))

	// 首先尝试查找来自客户端发送的请求（普通模式）
	c.pendingReqMu.Lock()
	respCh, exists := c.pendingRequests[pkt.RequestID]
	c.pendingReqMu.Unlock()

	if exists {
		// 这是对客户端发送请求的响应
		resp := &Response{
			ReasonCode: pkt.ReasonCode,
			Payload:    pkt.Payload,
		}

		if pkt.Properties != nil {
			resp.ReasonString = pkt.Properties.ReasonString
			resp.TraceID = pkt.Properties.TraceID
			resp.ContentType = pkt.Properties.ContentType
			resp.UserProperties = pkt.Properties.UserProperties
		}

		// 发送到等待的协程
		select {
		case respCh <- resp:
		default:
			c.logger.Warn("Response channel full or closed",
				zap.Uint32("requestID", pkt.RequestID))
		}

		return nil
	}

	// 如果没有找到，则可能是 core 发送的请求的响应
	// 尝试从 ActionRegistry 的 RequestTracker 中查找
	// 这部分需要访问 core 包中的全局响应等待器
	// 为了简单起见，我们可以直接发送到该通道
	// 但由于 request.go 在 client 包中，不能直接访问 core 的内部结构
	// 所以我们在这里做一个备注：core 发送的响应由 core 侧的 RequestTracker 处理

	c.logger.Warn("Response for unknown request",
		zap.Uint32("requestID", pkt.RequestID))
	return nil
}

// Register 向 core 注册一个 action
// 不存储回调函数，仅用于告知 core 本客户端支持该 action
func (c *Client) Register(action string) error {
	if !c.connected.Load() {
		return ErrNotConnected
	}

	return c.RegisterMultiple([]string{action})
}

// RegisterMultiple 向 core 批量注册 actions
// 不存储回调函数，仅用于告知 core 本客户端支持这些 actions
func (c *Client) RegisterMultiple(actions []string) error {
	if !c.connected.Load() {
		return ErrNotConnected
	}

	if len(actions) == 0 {
		return ErrActionsEmpty
	}

	// 记录已注册的 actions
	c.actionsMu.Lock()
	for _, action := range actions {
		c.registeredActions[action] = true
	}
	c.actionsMu.Unlock()

	registerPkt := packet.NewRegisterPacket(actions)
	return c.writePacket(registerPkt)
}

// Unregister 向 core 注销一个 action
func (c *Client) Unregister(action string) error {
	if !c.connected.Load() {
		return ErrNotConnected
	}

	return c.UnregisterMultiple([]string{action})
}

// UnregisterMultiple 向 core 批量注销 actions
func (c *Client) UnregisterMultiple(actions []string) error {
	if !c.connected.Load() {
		return ErrNotConnected
	}

	if len(actions) == 0 {
		return ErrActionsEmpty
	}

	// 移除已注册的 actions
	c.actionsMu.Lock()
	for _, action := range actions {
		delete(c.registeredActions, action)
	}
	c.actionsMu.Unlock()

	unregisterPkt := packet.NewUnregisterPacket(actions)
	return c.writePacket(unregisterPkt)
}

// handleRegack 处理 REGACK 包
// 由于我们不存储回调函数，这里仅记录日志
func (c *Client) handleRegack(p *packet.RegackPacket) {
	c.logger.Debug("Received REGACK",
		zap.Int("resultCount", len(p.Results)))

	// 可选：记录注册结果
	for _, result := range p.Results {
		if result.ReasonCode != packet.ReasonSuccess {
			c.logger.Warn("Failed to register action",
				zap.String("action", result.Action),
				zap.String("reasonCode", result.ReasonCode.String()))
		}
	}
}

// handleUnregack 处理 UNREGACK 包
// 由于我们不存储回调函数，这里仅记录日志
func (c *Client) handleUnregack(p *packet.UnregackPacket) {
	c.logger.Debug("Received UNREGACK",
		zap.Int("resultCount", len(p.Results)))

	// 可选：记录注销结果
	for _, result := range p.Results {
		if result.ReasonCode != packet.ReasonSuccess {
			c.logger.Warn("Failed to unregister action",
				zap.String("action", result.Action),
				zap.String("reasonCode", result.ReasonCode.String()))
		}
	}
}

// GetRegisteredActions 获取本客户端在 core 中注册的所有 actions
// 返回本地记录的已注册 actions 列表
func (c *Client) GetRegisteredActions() []string {
	c.actionsMu.RLock()
	defer c.actionsMu.RUnlock()

	actions := make([]string, 0, len(c.registeredActions))
	for action := range c.registeredActions {
		actions = append(actions, action)
	}
	return actions
}

// HasAction 检查本客户端是否注册了指定的 action
func (c *Client) HasAction(action string) bool {
	c.actionsMu.RLock()
	defer c.actionsMu.RUnlock()

	return c.registeredActions[action]
}
