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
	Packet *packet.RequestPacket

	// 内部字段
	client *Client
}

// Response 响应
type Response struct {
	Packet *packet.ResponsePacket
}

// ReasonCode 返回响应的原因码
func (r *Response) ReasonCode() packet.ReasonCode {
	if r.Packet != nil {
		return r.Packet.ReasonCode
	}
	return packet.ReasonUnspecifiedError
}

// ReasonString 返回响应的原因字符串
func (r *Response) ReasonString() string {
	if r.Packet != nil && r.Packet.Properties != nil {
		return r.Packet.Properties.ReasonString
	}
	return ""
}

// Payload 返回响应的负载
func (r *Response) Payload() []byte {
	if r.Packet != nil {
		return r.Packet.Payload
	}
	return nil
}

// handleRequest 处理收到的 REQUEST 包
// 采用轮询模式：将请求放入队列，让外部代码通过 PollRequest() 接收并处理
func (c *Client) handleRequest(p *packet.RequestPacket) error {
	c.logger.Debug("Received REQUEST",
		zap.Uint32("requestID", p.RequestID),
		zap.String("action", p.Action),
		zap.String("sourceClientID", p.SourceClientID))

	// 调用 OnRequest 钩子（直接传递原始 packet）
	if !c.options.Hooks.callOnRequest(&RequestContext{Packet: p}) {
		c.logger.Debug("Request rejected by hook",
			zap.Uint32("requestID", p.RequestID),
			zap.String("action", p.Action))
		resp := packet.NewResponsePacket(
			p.RequestID,
			p.SourceClientID,
			packet.ReasonNotAuthorized,
			nil,
		)
		resp.Properties.ReasonString = "Request rejected by hook"
		return c.writePacket(resp)
	}

	req := &Request{
		Packet: p,
		client: c,
	}

	// 非阻塞尝试放入队列
	select {
	case c.requestQueue <- req:
		// 成功放入队列
		c.logger.Debug("Enqueued request for polling",
			zap.Uint32("requestID", p.RequestID),
			zap.String("action", p.Action))
		return nil
	default:
		// 队列已满，返回服务器繁忙错误
		resp := packet.NewResponsePacket(
			p.RequestID,
			p.SourceClientID,
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

	// 尝试查找来自客户端发送的请求
	c.pendingReqMu.Lock()
	respCh, exists := c.pendingRequests[pkt.RequestID]
	c.pendingReqMu.Unlock()

	if exists {
		// 这是对客户端发送请求的响应
		resp := &Response{
			Packet: pkt,
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

	// 如果没有找到,可能是消息超时或未知请求，丢弃并记录日志
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

// Response 响应一个接收到的请求
// 该方法将响应发送给请求的来源
func (req *Request) Response(res *Response) error {
	if req.client == nil {
		return ErrRequestClientNotSet
	}

	if res == nil || res.Packet == nil {
		return ErrResultNil
	}

	// 填充响应的 RequestID 和 TargetClientID
	res.Packet.RequestID = req.Packet.RequestID
	res.Packet.TargetClientID = req.Packet.SourceClientID

	// 发送响应
	return req.client.writePacket(res.Packet)
}

// Request 发送请求并等待响应
func (c *Client) Request(ctx context.Context, req *Request) (*Response, error) {
	if !c.connected.Load() {
		return nil, ErrNotConnected
	}

	if req == nil || req.Packet == nil {
		return nil, ErrRequestNil
	}

	if req.Packet.Action == "" {
		return nil, ErrActionEmpty
	}

	if req.Packet.Properties == nil {
		req.Packet.Properties = packet.NewRequestProperties()
	}

	// 处理超时设置
	timeout := defaultRequestTimeout
	if req.Packet.Properties.Timeout == 0 {
		req.Packet.Properties.Timeout = uint32(timeout.Seconds())
	} else {
		timeout = time.Duration(req.Packet.Properties.Timeout) * time.Second
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

	// 填充请求 ID
	req.Packet.RequestID = requestID

	// 发送请求
	if err := c.writePacket(req.Packet); err != nil {
		return nil, fmt.Errorf("failed to send REQUEST: %w", err)
	}

	c.logger.Debug("Sent REQUEST",
		zap.Uint32("requestID", requestID),
		zap.String("action", req.Packet.Action),
		zap.String("targetClientID", req.Packet.TargetClientID))

	// 等待响应或超时
	timer := time.NewTimer(time.Duration(req.Packet.Properties.Timeout) * time.Second)
	defer timer.Stop()

	select {
	case resp := <-respCh:
		return resp, nil
	case <-timer.C:
		return nil, NewRequestTimeoutError(timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, ErrClientClosed
	}
}

// RequestToClient 发送请求到指定客户端
func (c *Client) RequestToClient(ctx context.Context, targetClientID string, req *Request) (*Response, error) {
	if req == nil || req.Packet == nil {
		return nil, ErrRequestNil
	}

	req.Packet.TargetClientID = targetClientID

	return c.Request(ctx, req)
}

// NewRequest 创建新的请求
func NewRequest(action string, payload []byte) *Request {
	return &Request{
		Packet: &packet.RequestPacket{
			Action:     action,
			Payload:    payload,
			Properties: packet.NewRequestProperties(),
		},
	}
}

// WithTimeout 设置请求超时（秒）
func (r *Request) WithTimeout(seconds uint32) *Request {
	if r.Packet.Properties == nil {
		r.Packet.Properties = packet.NewRequestProperties()
	}

	r.Packet.Properties.Timeout = seconds
	return r
}

// WithTraceID 设置追踪 ID
func (r *Request) WithTraceID(traceID string) *Request {
	if r.Packet.Properties == nil {
		r.Packet.Properties = packet.NewRequestProperties()
	}

	r.Packet.Properties.TraceID = traceID
	return r
}

// WithContentType 设置内容类型
func (r *Request) WithContentType(contentType string) *Request {
	if r.Packet.Properties == nil {
		r.Packet.Properties = packet.NewRequestProperties()
	}

	r.Packet.Properties.ContentType = contentType
	return r
}

// WithUserProperties 设置用户属性
func (r *Request) WithUserProperties(props map[string]string) *Request {
	if r.Packet.Properties == nil {
		r.Packet.Properties = packet.NewRequestProperties()
	}

	r.Packet.Properties.UserProperties = props
	return r
}

// WithUserProperty 设置单个用户属性
func (r *Request) WithUserProperty(key, value string) *Request {
	if r.Packet.Properties == nil {
		r.Packet.Properties = packet.NewRequestProperties()
	}

	if r.Packet.Properties.UserProperties == nil {
		r.Packet.Properties.UserProperties = make(map[string]string)
	}
	r.Packet.Properties.UserProperties[key] = value
	return r
}

// NewResponse 创建新的响应（成功）
func NewResponse(payload []byte) *Response {
	return &Response{
		Packet: &packet.ResponsePacket{
			ReasonCode: packet.ReasonSuccess,
			Payload:    payload,
			Properties: packet.NewResponseProperties(),
		},
	}
}

// NewResponseWithCode 创建带有原因码的响应
func NewResponseWithCode(code packet.ReasonCode, payload []byte) *Response {
	return &Response{
		Packet: &packet.ResponsePacket{
			ReasonCode: code,
			Payload:    payload,
			Properties: packet.NewResponseProperties(),
		},
	}
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(code packet.ReasonCode, reason string) *Response {
	properties := packet.NewResponseProperties()
	properties.ReasonString = reason

	return &Response{
		Packet: &packet.ResponsePacket{
			ReasonCode: code,
			Properties: properties,
		},
	}
}

// WithReasonCode 设置原因码
func (r *Response) WithReasonCode(code packet.ReasonCode) *Response {
	r.Packet.ReasonCode = code
	return r
}

// WithReasonString 设置原因字符串
func (r *Response) WithReasonString(reason string) *Response {
	if r.Packet.Properties == nil {
		r.Packet.Properties = packet.NewResponseProperties()
	}

	r.Packet.Properties.ReasonString = reason
	return r
}

// WithTraceID 设置追踪 ID
func (r *Response) WithTraceID(traceID string) *Response {
	if r.Packet.Properties == nil {
		r.Packet.Properties = packet.NewResponseProperties()
	}

	r.Packet.Properties.TraceID = traceID
	return r
}

// WithContentType 设置内容类型
func (r *Response) WithContentType(contentType string) *Response {
	if r.Packet.Properties == nil {
		r.Packet.Properties = packet.NewResponseProperties()
	}

	r.Packet.Properties.ContentType = contentType
	return r
}

// WithUserProperties 设置用户属性
func (r *Response) WithUserProperties(props map[string]string) *Response {
	if r.Packet.Properties == nil {
		r.Packet.Properties = packet.NewResponseProperties()
	}

	r.Packet.Properties.UserProperties = props
	return r
}

// WithUserProperty 设置单个用户属性
func (r *Response) WithUserProperty(key, value string) *Response {
	if r.Packet.Properties == nil {
		r.Packet.Properties = packet.NewResponseProperties()
	}

	if r.Packet.Properties.UserProperties == nil {
		r.Packet.Properties.UserProperties = make(map[string]string)
	}
	r.Packet.Properties.UserProperties[key] = value
	return r
}

// IsSuccess 检查响应是否成功
func (r *Response) IsSuccess() bool {
	return r.Packet.ReasonCode == packet.ReasonSuccess
}

// Error 返回错误信息（如果有）
func (r *Response) Error() string {
	if r.Packet.ReasonCode == packet.ReasonSuccess {
		return ""
	}
	if r.Packet.Properties.ReasonString != "" {
		return r.Packet.Properties.ReasonString
	}
	return r.Packet.ReasonCode.String()
}
