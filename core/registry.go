package core

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// ActionHandler 代表一个 action 的处理者
type ActionHandler struct {
	ClientID     string        // 处理者的客户端ID
	Concurrency  uint16        // 并发处理能力
	RegisterAt   time.Time     // 注册时间
	RequestsSent atomic.Uint64 // 已发送的请求数（用于轮询负载均衡）
}

// ActionRegistry action 注册表
type ActionRegistry struct {
	// action -> handlers
	actions map[string][]*ActionHandler
	mu      sync.RWMutex
}

// NewActionRegistry 创建新的 action 注册表
func NewActionRegistry() *ActionRegistry {
	return &ActionRegistry{
		actions: make(map[string][]*ActionHandler),
	}
}

// Register 注册 action
func (r *ActionRegistry) Register(clientID string, actions []string, concurrency uint16) map[string]packet.ReasonCode {
	r.mu.Lock()
	defer r.mu.Unlock()

	results := make(map[string]packet.ReasonCode)

	for _, action := range actions {
		// 验证 action 名称
		if !validateActionName(action) {
			results[action] = packet.ReasonActionInvalid
			continue
		}

		// 检查是否已注册
		handlers := r.actions[action]
		duplicate := false
		for _, h := range handlers {
			if h.ClientID == clientID {
				results[action] = packet.ReasonDuplicateAction
				duplicate = true
				break
			}
		}

		if duplicate {
			continue
		}

		// 添加新的 handler
		handler := &ActionHandler{
			ClientID:    clientID,
			Concurrency: concurrency,
			RegisterAt:  time.Now(),
		}
		r.actions[action] = append(r.actions[action], handler)
		results[action] = packet.ReasonSuccess
	}

	return results
}

// Unregister 注销 action
func (r *ActionRegistry) Unregister(clientID string, actions []string) map[string]packet.ReasonCode {
	r.mu.Lock()
	defer r.mu.Unlock()

	results := make(map[string]packet.ReasonCode)

	for _, action := range actions {
		handlers := r.actions[action]
		found := false

		// 查找并移除
		for i, h := range handlers {
			if h.ClientID == clientID {
				r.actions[action] = append(handlers[:i], handlers[i+1:]...)
				found = true
				break
			}
		}

		// 如果没有 handler 了，删除 action
		if len(r.actions[action]) == 0 {
			delete(r.actions, action)
		}

		if found {
			results[action] = packet.ReasonSuccess
		} else {
			results[action] = packet.ReasonNoSubscriptionExisted
		}
	}

	return results
}

// UnregisterClient 注销客户端的所有 actions
func (r *ActionRegistry) UnregisterClient(clientID string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var unregistered []string

	for action, handlers := range r.actions {
		for i, h := range handlers {
			if h.ClientID == clientID {
				r.actions[action] = append(handlers[:i], handlers[i+1:]...)
				unregistered = append(unregistered, action)
				break
			}
		}

		// 如果没有 handler 了，删除 action
		if len(r.actions[action]) == 0 {
			delete(r.actions, action)
		}
	}

	return unregistered
}

// SelectHandler 选择一个处理者
// targetClientID 为空时，选择接收窗口最大的客户端
func (r *ActionRegistry) SelectHandler(action string, targetClientID string, getClient func(string) *Client) (string, packet.ReasonCode) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handlers := r.actions[action]
	if len(handlers) == 0 {
		return "", packet.ReasonActionNotFound
	}

	// 如果指定了目标客户端
	if targetClientID != "" {
		for _, h := range handlers {
			if h.ClientID == targetClientID {
				// 检查客户端是否在线
				client := getClient(targetClientID)
				if client == nil {
					return "", packet.ReasonClientNotFound
				}

				return targetClientID, packet.ReasonSuccess
			}
		}
		return "", packet.ReasonClientNotFound
	}

	// 未指定目标客户端，选择负载最小的（使用轮询算法）
	var bestHandler *ActionHandler
	var minRequests = ^uint64(0) // 初始化为最大值

	for _, h := range handlers {
		client := getClient(h.ClientID)
		if client == nil {
			continue
		}

		// 选择已分配请求数最少的客户端（轮询负载均衡）
		requests := h.RequestsSent.Load()
		if requests < minRequests {
			minRequests = requests
			bestHandler = h
		}
	}

	if bestHandler == nil {
		return "", packet.ReasonNoAvailableHandler
	}

	// 增加请求计数
	bestHandler.RequestsSent.Add(1)

	return bestHandler.ClientID, packet.ReasonSuccess
}

// GetHandlers 获取 action 的所有处理者
func (r *ActionRegistry) GetHandlers(action string) []*ActionHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handlers := r.actions[action]
	result := make([]*ActionHandler, len(handlers))
	copy(result, handlers)
	return result
}

// GetClientActions 获取客户端注册的所有 actions
func (r *ActionRegistry) GetClientActions(clientID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var actions []string
	for action, handlers := range r.actions {
		for _, h := range handlers {
			if h.ClientID == clientID {
				actions = append(actions, action)
				break
			}
		}
	}
	return actions
}

// GetActionsForClient 获取客户端注册的所有 actions（别名）
func (r *ActionRegistry) GetActionsForClient(clientID string) []string {
	return r.GetClientActions(clientID)
}

// Count 返回注册的 action 总数
func (r *ActionRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.actions)
}

// validateActionName 验证 action 名称
// action 名称规则：
// - 不能为空
// - 长度不超过 65535
// - 不能包含通配符
// - 不能包含空字符
func validateActionName(action string) bool {
	if len(action) == 0 || len(action) > packet.MaxTopicLength {
		return false
	}

	// 不允许通配符
	if strings.Contains(action, packet.TopicWildcardSingle) ||
		strings.Contains(action, packet.TopicWildcardMulti) ||
		strings.ContainsAny(action, "+#") {
		return false
	}

	// 不允许空字符
	if strings.ContainsRune(action, 0) {
		return false
	}

	return true
}

// PendingRequest 等待响应的请求
type PendingRequest struct {
	CoreRequestID   uint64 // core 生成的全局唯一 ID
	ClientRequestID uint32 // 客户端原始 RequestID（用于响应时还原）
	SourceClientID  string // 请求来源客户端
	TargetClientID  string // 处理请求的客户端
	Action          string
	CreateAt        time.Time
	ExpiryTime      int64 // 过期时间戳
	Timer           *time.Timer
}

// requestKey 用于唯一标识一个请求的复合键
type requestKey struct {
	SourceClientID  string
	ClientRequestID uint32
}

// RequestTracker 请求追踪器
// 使用 Core 生成的全局唯一 ID 来追踪请求，解决客户端 RequestID 冲突问题
type RequestTracker struct {
	// coreRequestID -> PendingRequest（主索引，用于超时处理）
	requests map[uint64]*PendingRequest

	// (sourceClientID, clientRequestID) -> coreRequestID（辅助索引，用于响应路由）
	clientIndex map[requestKey]uint64

	mu sync.RWMutex

	// 全局唯一 ID 生成器
	nextCoreRequestID atomic.Uint64

	core *Core

	// 存储从 core 发送的请求的响应等待器
	coreWaiters coreRequestWaiters
}

// NewRequestTracker 创建请求追踪器
func NewRequestTracker(core *Core) *RequestTracker {
	t := &RequestTracker{
		requests:    make(map[uint64]*PendingRequest),
		clientIndex: make(map[requestKey]uint64),
		core:        core,
		coreWaiters: coreRequestWaiters{
			waiters: make(map[uint64]chan *Response),
		},
	}
	// 从 1 开始，0 保留用于表示无效 ID
	t.nextCoreRequestID.Store(1)
	return t
}

// Track 追踪请求（从客户端收到的 REQUEST 包）
// 返回 coreRequestID 用于后续响应路由
func (t *RequestTracker) Track(req *packet.RequestPacket, targetClientID string) uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	// 生成全局唯一的 coreRequestID
	coreRequestID := t.nextCoreRequestID.Add(1)

	// 计算过期时间（将相对时间转换为绝对时间）
	var expiryTime int64
	var timeout time.Duration

	if req.Properties.Timeout > 0 {
		timeout = time.Duration(req.Properties.Timeout) * time.Second
		expiryTime = time.Now().Add(timeout).Unix()
	} else {
		// 默认 30 秒超时
		timeout = t.core.options.DefaultRequestTimeout
		expiryTime = time.Now().Add(timeout).Unix()
	}

	pr := &PendingRequest{
		CoreRequestID:   coreRequestID,
		ClientRequestID: req.RequestID,
		SourceClientID:  req.SourceClientID,
		TargetClientID:  targetClientID,
		Action:          req.Action,
		CreateAt:        time.Now(),
		ExpiryTime:      expiryTime,
	}

	// 设置超时定时器
	pr.Timer = time.AfterFunc(timeout, func() {
		t.sendTimeoutResponse(pr)
	})

	// 存储到主索引
	t.requests[coreRequestID] = pr

	// 存储到辅助索引（用于响应路由）
	key := requestKey{SourceClientID: req.SourceClientID, ClientRequestID: req.RequestID}
	t.clientIndex[key] = coreRequestID
	return coreRequestID
}

// CompleteByCoreID 通过 coreRequestID 完成请求
func (t *RequestTracker) CompleteByCoreID(coreRequestID uint64) *PendingRequest {
	t.mu.Lock()
	defer t.mu.Unlock()

	pr := t.requests[coreRequestID]
	if pr != nil {
		if pr.Timer != nil {
			pr.Timer.Stop()
		}
		delete(t.requests, coreRequestID)

		// 同时删除辅助索引
		key := requestKey{SourceClientID: pr.SourceClientID, ClientRequestID: pr.ClientRequestID}
		delete(t.clientIndex, key)
	}

	return pr
}

// CompleteByClientID 通过客户端 ID 和 RequestID 完成请求
func (t *RequestTracker) CompleteByClientID(sourceClientID string, clientRequestID uint32) *PendingRequest {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := requestKey{SourceClientID: sourceClientID, ClientRequestID: clientRequestID}
	coreRequestID, exists := t.clientIndex[key]
	if !exists {
		return nil
	}

	pr := t.requests[coreRequestID]
	if pr != nil {
		if pr.Timer != nil {
			pr.Timer.Stop()
		}
		delete(t.requests, coreRequestID)
		delete(t.clientIndex, key)
	}

	return pr
}

// sendTimeoutResponse 发送超时响应
func (t *RequestTracker) sendTimeoutResponse(pr *PendingRequest) {
	// 检查并删除请求
	t.mu.Lock()
	existing, exists := t.requests[pr.CoreRequestID]
	if !exists {
		// 请求已经被其他地方处理了（正常响应或取消）
		t.mu.Unlock()
		t.core.logger.Debug("Request already completed, skipping timeout response",
			zap.Uint64("coreRequestID", pr.CoreRequestID))
		return
	}

	// 双重检查：确保是同一个请求实例
	if existing != pr {
		// 这种情况理论上不应该发生，但为了安全起见
		t.mu.Unlock()
		t.core.logger.Warn("Request instance mismatch in timeout handler",
			zap.Uint64("coreRequestID", pr.CoreRequestID))
		return
	}

	delete(t.requests, pr.CoreRequestID)
	// 同时删除辅助索引
	key := requestKey{SourceClientID: pr.SourceClientID, ClientRequestID: pr.ClientRequestID}
	delete(t.clientIndex, key)
	t.mu.Unlock()

	// 发送给源客户端
	t.core.clientsMu.RLock()
	sourceClient := t.core.clients[pr.SourceClientID]
	t.core.clientsMu.RUnlock()

	if sourceClient != nil {
		pkt := packet.NewResponsePacket(
			pr.ClientRequestID, // 使用客户端原始 RequestID
			pr.SourceClientID,
			packet.ReasonRequestTimeout,
			nil,
		)
		pkt.Properties.ReasonString = fmt.Sprintf("Request timeout: action=%s", pr.Action)
		sourceClient.WritePacket(pkt)
		return
	}
}

// Count 返回等待中的请求数量
func (t *RequestTracker) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.requests)
}

// Cleanup 清理指定客户端的所有请求
func (t *RequestTracker) Cleanup(clientID string) {
	// 收集需要清理的请求
	t.mu.Lock()
	toCleanup := make([]*PendingRequest, 0)
	for id, pr := range t.requests {
		if pr.SourceClientID == clientID || pr.TargetClientID == clientID {
			toCleanup = append(toCleanup, pr)
			delete(t.requests, id)
			// 同时删除辅助索引
			key := requestKey{SourceClientID: pr.SourceClientID, ClientRequestID: pr.ClientRequestID}
			delete(t.clientIndex, key)
		}
	}
	t.mu.Unlock()

	// 在锁外处理清理
	for _, pr := range toCleanup {
		// 停止定时器
		if pr.Timer != nil {
			// Stop 返回 true 表示成功停止，false 表示已经触发或停止
			if !pr.Timer.Stop() {
				// 定时器已经触发或已停止
				// 但我们已经从 map 中删除了，所以超时处理会被跳过
			}
		}

		// 根据断开的客户端角色发送通知
		if pr.SourceClientID == clientID {
			// 源客户端断开
			t.core.logger.Debug("Request cancelled due to source client disconnect",
				zap.Uint64("coreRequestID", pr.CoreRequestID),
				zap.String("sourceClientID", clientID),
				zap.String("targetClientID", pr.TargetClientID))

		} else if pr.TargetClientID == clientID {
			// 目标客户端断开，通知源客户端请求失败
			t.core.clientsMu.RLock()
			sourceClient := t.core.clients[pr.SourceClientID]
			t.core.clientsMu.RUnlock()

			if sourceClient != nil {
				pkt := packet.NewResponsePacket(
					pr.ClientRequestID,
					pr.SourceClientID,
					packet.ReasonClientNotFound,
					nil,
				)
				pkt.Properties.ReasonString = fmt.Sprintf("Target client disconnected: %s", clientID)
				sourceClient.WritePacket(pkt)
			}
		}
	}

	if len(toCleanup) > 0 {
		t.core.logger.Info("Cleaned up pending requests",
			zap.String("clientID", clientID),
			zap.Int("count", len(toCleanup)))
	}
}

// ============================================================================
// Core-initiated Request Support
// ============================================================================

// coreRequestWaiters 存储从 core 发送的请求的响应等待器
type coreRequestWaiters struct {
	waiters map[uint64]chan *Response
	mu      sync.RWMutex
}

// GenerateRequestID 生成一个新的请求 ID（用于 core 发送的请求）
func (t *RequestTracker) GenerateRequestID() uint32 {
	// 为了避免与客户端的请求 ID 冲突，core 生成的请求 ID 使用高位
	// 正常的客户端请求 ID 范围是 1-65535 (uint16)
	// 为了简化，我们使用 coreRequestID 作为返回值
	// 在实际应用中，应该使用原子计数器
	id := t.nextCoreRequestID.Add(1)
	return uint32(id)
}

// RegisterResponseWaiter 注册一个响应等待器
func (t *RequestTracker) RegisterResponseWaiter(requestID uint32, respCh chan *Response) {
	t.coreWaiters.mu.Lock()
	defer t.coreWaiters.mu.Unlock()
	t.coreWaiters.waiters[uint64(requestID)] = respCh
}

// UnregisterResponseWaiter 注销响应等待器
func (t *RequestTracker) UnregisterResponseWaiter(requestID uint32) {
	t.coreWaiters.mu.Lock()
	defer t.coreWaiters.mu.Unlock()
	delete(t.coreWaiters.waiters, uint64(requestID))
}

// GetResponseWaiter 获取响应等待器
func (t *RequestTracker) GetResponseWaiter(requestID uint32) chan *Response {
	t.coreWaiters.mu.RLock()
	defer t.coreWaiters.mu.RUnlock()
	return t.coreWaiters.waiters[uint64(requestID)]
}
