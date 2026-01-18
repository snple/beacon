package core

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snple/beacon/packet"
)

// ActionHandler 代表一个 action 的处理者
type ActionHandler struct {
	ClientID     string        // 处理者的客户端ID
	Concurrency  uint16        // 并发处理能力
	RegisterAt   time.Time     // 注册时间
	RequestsSent atomic.Uint64 // 已发送的请求数（用于轮询负载均衡）
}

// actionRegistry action 注册表
type actionRegistry struct {
	// action -> handlers
	actions map[string][]*ActionHandler
	mu      sync.RWMutex
}

// newActionRegistry 创建新的 action 注册表
func newActionRegistry() *actionRegistry {
	return &actionRegistry{
		actions: make(map[string][]*ActionHandler),
	}
}

// Register 注册 action
func (r *actionRegistry) Register(clientID string, actions []string, concurrency uint16) map[string]packet.ReasonCode {
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
func (r *actionRegistry) Unregister(clientID string, actions []string) map[string]packet.ReasonCode {
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
func (r *actionRegistry) UnregisterClient(clientID string) []string {
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
func (r *actionRegistry) SelectHandler(action string, targetClientID string, getClient func(string) *Client) (string, packet.ReasonCode) {
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
func (r *actionRegistry) GetHandlers(action string) []*ActionHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handlers := r.actions[action]
	result := make([]*ActionHandler, len(handlers))
	copy(result, handlers)
	return result
}

// GetClientActions 获取客户端注册的所有 actions
func (r *actionRegistry) GetClientActions(clientID string) []string {
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
func (r *actionRegistry) GetActionsForClient(clientID string) []string {
	return r.GetClientActions(clientID)
}

// Count 返回注册的 action 总数
func (r *actionRegistry) Count() int {
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
		strings.Contains(action, packet.TopicWildcardMulti) {
		return false
	}

	// 不允许空字符
	if strings.ContainsRune(action, 0) {
		return false
	}

	return true
}
