package core

import (
	"testing"

	"github.com/snple/beacon/packet"
)

// TestActionRegistry_Register 测试注册功能
func TestActionRegistry_Register(t *testing.T) {
	registry := newActionRegistry()

	// 测试单个注册
	results := registry.register("client1", []string{"action1"}, 1)
	if code, ok := results["action1"]; !ok || code != packet.ReasonSuccess {
		t.Errorf("Expected ReasonSuccess, got %v", code)
	}

	// 测试批量注册
	results = registry.register("client2", []string{"action2", "action3"}, 2)
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
	for action, code := range results {
		if code != packet.ReasonSuccess {
			t.Errorf("Expected ReasonSuccess for %s, got %v", action, code)
		}
	}

	// 测试重复注册
	results = registry.register("client1", []string{"action1"}, 1)
	if code, ok := results["action1"]; !ok || code != packet.ReasonDuplicateAction {
		t.Errorf("Expected ReasonDuplicateAction, got %v", code)
	}

	// 验证计数
	if count := registry.actionCount(); count != 3 {
		t.Errorf("Expected 3 actions, got %d", count)
	}
}

// TestActionRegistry_InvalidAction 测试无效 action
func TestActionRegistry_InvalidAction(t *testing.T) {
	registry := newActionRegistry()

	tests := []struct {
		name   string
		action string
	}{
		{"空字符串", ""},
		{"包含单层通配符", "action/*/test"},
		{"包含多层通配符", "action/**"},
		{"包含空字符", "action\x00test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := registry.register("client1", []string{tt.action}, 1)
			if code, ok := results[tt.action]; !ok || code != packet.ReasonActionInvalid {
				t.Errorf("Expected ReasonActionInvalid for %s, got %v", tt.name, code)
			}
		})
	}
}

// TestActionRegistry_Unregister 测试注销功能
func TestActionRegistry_Unregister(t *testing.T) {
	registry := newActionRegistry()

	// 注册一些 actions
	registry.register("client1", []string{"action1", "action2"}, 1)
	registry.register("client2", []string{"action3"}, 1)

	// 测试单个注销
	results := registry.unregister("client1", []string{"action1"})
	if code, ok := results["action1"]; !ok || code != packet.ReasonSuccess {
		t.Errorf("Expected ReasonSuccess, got %v", code)
	}

	// 验证已注销
	if count := registry.actionCount(); count != 2 {
		t.Errorf("Expected 2 actions after unregister, got %d", count)
	}

	// 测试注销不存在的 action
	results = registry.unregister("client1", []string{"action1"})
	if code, ok := results["action1"]; !ok || code != packet.ReasonNoSubscriptionExisted {
		t.Errorf("Expected ReasonNoSubscriptionExisted, got %v", code)
	}

	// 测试批量注销
	results = registry.unregister("client1", []string{"action2", "action3"})
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
	if code, ok := results["action2"]; !ok || code != packet.ReasonSuccess {
		t.Errorf("Expected ReasonSuccess for action2, got %v", code)
	}
	if code, ok := results["action3"]; !ok || code != packet.ReasonNoSubscriptionExisted {
		t.Errorf("Expected ReasonNoSubscriptionExisted for action3, got %v", code)
	}
}

// TestActionRegistry_UnregisterClient 测试注销客户端
func TestActionRegistry_UnregisterClient(t *testing.T) {
	registry := newActionRegistry()

	// 注册多个客户端的 actions
	registry.register("client1", []string{"action1", "action2", "action3"}, 1)
	registry.register("client2", []string{"action4", "action5"}, 1)

	// 注销 client1 的所有 actions
	unregistered := registry.unregisterClient("client1")
	if len(unregistered) != 3 {
		t.Errorf("Expected 3 unregistered actions, got %d", len(unregistered))
	}

	// 验证 client2 的 actions 仍然存在
	if count := registry.actionCount(); count != 2 {
		t.Errorf("Expected 2 actions remaining, got %d", count)
	}

	// 验证 client1 的 actions 已清空
	actions := registry.getClientActions("client1")
	if len(actions) != 0 {
		t.Errorf("Expected 0 actions for client1, got %d", len(actions))
	}
}

// TestActionRegistry_GetHandlers 测试获取 handlers
func TestActionRegistry_GetHandlers(t *testing.T) {
	registry := newActionRegistry()

	// 注册多个客户端处理同一个 action
	registry.register("client1", []string{"action1"}, 1)
	registry.register("client2", []string{"action1"}, 2)

	handlers := registry.getHandlers("action1")
	if len(handlers) != 2 {
		t.Errorf("Expected 2 handlers, got %d", len(handlers))
	}

	// 验证 handler 信息
	clientIDs := make(map[string]bool)
	for _, h := range handlers {
		clientIDs[h.ClientID] = true
		if h.ClientID == "client1" && h.Concurrency != 1 {
			t.Errorf("Expected concurrency 1 for client1, got %d", h.Concurrency)
		}
		if h.ClientID == "client2" && h.Concurrency != 2 {
			t.Errorf("Expected concurrency 2 for client2, got %d", h.Concurrency)
		}
	}

	if !clientIDs["client1"] || !clientIDs["client2"] {
		t.Error("Missing expected client IDs in handlers")
	}
}

// TestActionRegistry_SelectHandler 测试 handler 选择
func TestActionRegistry_SelectHandler(t *testing.T) {
	registry := newActionRegistry()

	// 创建模拟的 getClient 函数
	clients := map[string]*Client{
		"client1": {ID: "client1"},
		"client2": {ID: "client2"},
	}
	getClient := func(id string) *Client {
		return clients[id]
	}

	// 注册两个 handlers
	registry.register("client1", []string{"action1"}, 1)
	registry.register("client2", []string{"action1"}, 1)

	// 测试选择特定客户端
	clientID, code := registry.selectHandler("action1", "client1", getClient)
	if code != packet.ReasonSuccess {
		t.Errorf("Expected ReasonSuccess, got %v", code)
	}
	if clientID != "client1" {
		t.Errorf("Expected client1, got %s", clientID)
	}

	// 测试选择不存在的目标客户端
	_, code = registry.selectHandler("action1", "client3", getClient)
	if code != packet.ReasonClientNotFound {
		t.Errorf("Expected ReasonClientNotFound, got %v", code)
	}

	// 测试负载均衡（轮询）
	clientID, code = registry.selectHandler("action1", "", getClient)
	if code != packet.ReasonSuccess {
		t.Errorf("Expected ReasonSuccess, got %v", code)
	}
	if clientID != "client1" && clientID != "client2" {
		t.Errorf("Expected client1 or client2, got %s", clientID)
	}

	// 测试不存在的 action
	_, code = registry.selectHandler("nonexistent", "", getClient)
	if code != packet.ReasonActionNotFound {
		t.Errorf("Expected ReasonActionNotFound, got %v", code)
	}
}

// TestActionRegistry_GetClientActions 测试获取客户端的 actions
func TestActionRegistry_GetClientActions(t *testing.T) {
	registry := newActionRegistry()

	// 注册 actions
	registry.register("client1", []string{"action1", "action2", "action3"}, 1)
	registry.register("client2", []string{"action4"}, 1)

	// 获取 client1 的 actions
	actions := registry.getClientActions("client1")
	if len(actions) != 3 {
		t.Errorf("Expected 3 actions for client1, got %d", len(actions))
	}

	// 验证包含正确的 actions
	actionMap := make(map[string]bool)
	for _, action := range actions {
		actionMap[action] = true
	}
	if !actionMap["action1"] || !actionMap["action2"] || !actionMap["action3"] {
		t.Error("Missing expected actions for client1")
	}

	// 获取不存在的客户端的 actions
	actions = registry.getClientActions("nonexistent")
	if len(actions) != 0 {
		t.Errorf("Expected 0 actions for nonexistent client, got %d", len(actions))
	}
}

// TestActionRegistry_Concurrent 测试并发安全
func TestActionRegistry_Concurrent(t *testing.T) {
	registry := newActionRegistry()

	// 并发注册
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			clientID := "client" + string(rune('0'+id))
			registry.register(clientID, []string{"action1", "action2"}, 1)
			registry.getClientActions(clientID)
			registry.unregisterClient(clientID)
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证最终状态一致
	count := registry.actionCount()
	t.Logf("Final action count: %d", count)
}
