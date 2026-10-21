package core

import (
	"fmt"
	"testing"

	"github.com/snple/beacon/packet"
)

// TestSubscriptionTree_Subscribe 测试订阅功能
func TestSubscriptionTree_Subscribe(t *testing.T) {
	tree := newSubTree()

	// 测试单个订阅
	isNew := tree.subscribe("client1", "topic1", packet.SubscribeOptions{QoS: packet.QoS0})
	if !isNew {
		t.Error("Expected new subscription")
	}

	// 测试重复订阅（更新 QoS）
	isNew = tree.subscribe("client1", "topic1", packet.SubscribeOptions{QoS: packet.QoS1})
	if isNew {
		t.Error("Expected existing subscription")
	}

	// 测试批量订阅
	subs := []packet.Subscription{
		{Topic: "topic2", Options: packet.SubscribeOptions{QoS: packet.QoS0}},
		{Topic: "topic3", Options: packet.SubscribeOptions{QoS: packet.QoS1}},
	}
	results := tree.subscribeMultiple("client2", subs)
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
	expectedCodes := map[string]packet.ReasonCode{
		"topic2": packet.ReasonGrantedQoS0,
		"topic3": packet.ReasonGrantedQoS1,
	}
	for topic, code := range results {
		if code != expectedCodes[topic] {
			t.Errorf("Expected %v for %s, got %v", expectedCodes[topic], topic, code)
		}
	}

	// 验证计数
	if count := tree.subscriptionCount(); count != 3 {
		t.Errorf("Expected 3 subscriptions, got %d", count)
	}
}

// TestSubscriptionTree_InvalidTopic 测试无效主题
func TestSubscriptionTree_InvalidTopic(t *testing.T) {
	tree := newSubTree()

	tests := []struct {
		name  string
		topic string
	}{
		{"空字符串", ""},
		{"包含空字符", "topic\x00test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subs := []packet.Subscription{
				{Topic: tt.topic, Options: packet.SubscribeOptions{QoS: packet.QoS0}},
			}
			results := tree.subscribeMultiple("client1", subs)
			if code, ok := results[tt.topic]; !ok || code != packet.ReasonTopicFilterInvalid {
				t.Errorf("Expected ReasonTopicFilterInvalid for %s, got %v", tt.name, code)
			}
		})
	}
}

// TestSubscriptionTree_Unsubscribe 测试取消订阅
func TestSubscriptionTree_Unsubscribe(t *testing.T) {
	tree := newSubTree()

	// 订阅一些主题
	tree.subscribe("client1", "topic1", packet.SubscribeOptions{QoS: packet.QoS0})
	tree.subscribe("client1", "topic2", packet.SubscribeOptions{QoS: packet.QoS0})
	tree.subscribe("client2", "topic3", packet.SubscribeOptions{QoS: packet.QoS0})

	// 测试单个取消订阅
	removed := tree.unsubscribe("client1", "topic1")
	if !removed {
		t.Error("Expected subscription to be removed")
	}

	// 验证已取消订阅
	if count := tree.subscriptionCount(); count != 2 {
		t.Errorf("Expected 2 subscriptions after unsubscribe, got %d", count)
	}

	// 测试取消不存在的订阅
	removed = tree.unsubscribe("client1", "topic1")
	if removed {
		t.Error("Expected no subscription to remove")
	}

	// 测试批量取消订阅
	results := tree.unsubscribeMultiple("client1", []string{"topic2", "topic3"})
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
	if code, ok := results["topic2"]; !ok || code != packet.ReasonSuccess {
		t.Errorf("Expected ReasonSuccess for topic2, got %v", code)
	}
	if code, ok := results["topic3"]; !ok || code != packet.ReasonNoSubscriptionExisted {
		t.Errorf("Expected ReasonNoSubscriptionExisted for topic3, got %v", code)
	}
}

// TestSubscriptionTree_UnsubscribeClient 测试取消客户端的所有订阅
func TestSubscriptionTree_UnsubscribeClient(t *testing.T) {
	tree := newSubTree()

	// 订阅多个客户端的主题
	tree.subscribe("client1", "topic1", packet.SubscribeOptions{QoS: packet.QoS0})
	tree.subscribe("client1", "topic2", packet.SubscribeOptions{QoS: packet.QoS0})
	tree.subscribe("client1", "topic3", packet.SubscribeOptions{QoS: packet.QoS0})
	tree.subscribe("client2", "topic4", packet.SubscribeOptions{QoS: packet.QoS0})
	tree.subscribe("client2", "topic5", packet.SubscribeOptions{QoS: packet.QoS0})

	// 取消 client1 的所有订阅
	count := tree.unsubscribeClient("client1")
	if count != 3 {
		t.Errorf("Expected 3 unsubscribed topics, got %d", count)
	}

	// 验证 client2 的订阅仍然存在
	if remaining := tree.subscriptionCount(); remaining != 2 {
		t.Errorf("Expected 2 subscriptions remaining, got %d", remaining)
	}

	// 验证 client1 的订阅已清空
	topics := tree.getClientTopics("client1")
	if len(topics) != 0 {
		t.Errorf("Expected 0 topics for client1, got %d", len(topics))
	}
}

// TestSubscriptionTree_Match 测试主题匹配
func TestSubscriptionTree_Match(t *testing.T) {
	tree := newSubTree()

	// 订阅各种模式（订阅者使用通配符）
	tree.subscribe("client1", "sensor/temp", packet.SubscribeOptions{QoS: packet.QoS0}) // 精确订阅
	tree.subscribe("client2", "sensor/*", packet.SubscribeOptions{QoS: packet.QoS1})    // 单层通配符
	tree.subscribe("client3", "sensor/**", packet.SubscribeOptions{QoS: packet.QoS0})   // 多层通配符
	tree.subscribe("client4", "+/temp", packet.SubscribeOptions{QoS: packet.QoS1})      // 旧通配符格式（测试不匹配）

	tests := []struct {
		topic           string
		expectedClients map[string]packet.QoS
		description     string
	}{
		{
			topic: "sensor/temp",
			expectedClients: map[string]packet.QoS{
				"client1": packet.QoS0,
				"client2": packet.QoS1,
				"client3": packet.QoS0,
			},
			description: "精确匹配和通配符",
		},
		{
			topic: "sensor/humidity",
			expectedClients: map[string]packet.QoS{
				"client2": packet.QoS1,
				"client3": packet.QoS0,
			},
			description: "单层和多层通配符",
		},
		{
			topic: "sensor/room/temp",
			expectedClients: map[string]packet.QoS{
				"client3": packet.QoS0,
			},
			description: "仅多层通配符",
		},
		{
			topic:           "other/topic",
			expectedClients: map[string]packet.QoS{},
			description:     "无匹配",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			matches := tree.matchTopic(tt.topic)
			if len(matches) != len(tt.expectedClients) {
				t.Errorf("Expected %d matches, got %d for topic %s",
					len(tt.expectedClients), len(matches), tt.topic)
				t.Logf("Got matches: %+v", matches)
			}
			for clientID, expectedQoS := range tt.expectedClients {
				if opts, ok := matches[clientID]; !ok {
					t.Errorf("Expected client %s to match topic %s", clientID, tt.topic)
				} else if opts.QoS != expectedQoS {
					t.Errorf("Expected QoS %d for client %s, got %d",
						expectedQoS, clientID, opts.QoS)
				}
			}
		})
	}
}

// TestSubscriptionTree_MatchQoSPriority 测试 QoS 优先级
func TestSubscriptionTree_MatchQoSPriority(t *testing.T) {
	tree := newSubTree()

	// 同一个客户端订阅多个匹配的主题，使用不同的 QoS
	tree.subscribe("client1", "sensor/+", packet.SubscribeOptions{QoS: packet.QoS0})
	tree.subscribe("client1", "sensor/**", packet.SubscribeOptions{QoS: packet.QoS1})
	tree.subscribe("client1", "sensor/temp", packet.SubscribeOptions{QoS: packet.QoS0})

	// 匹配应该返回最高的 QoS
	matches := tree.matchTopic("sensor/temp")
	if opts, ok := matches["client1"]; !ok {
		t.Error("Expected client1 to match")
	} else if opts.QoS != packet.QoS1 {
		t.Errorf("Expected QoS1 (highest), got QoS%d", opts.QoS)
	}
}

// TestSubscriptionTree_GetClientTopics 测试获取客户端订阅的主题
func TestSubscriptionTree_GetClientTopics(t *testing.T) {
	tree := newSubTree()

	// 订阅多个主题
	tree.subscribe("client1", "topic1", packet.SubscribeOptions{QoS: packet.QoS0})
	tree.subscribe("client1", "topic2", packet.SubscribeOptions{QoS: packet.QoS0})
	tree.subscribe("client1", "topic3/subtopic", packet.SubscribeOptions{QoS: packet.QoS0})
	tree.subscribe("client2", "topic4", packet.SubscribeOptions{QoS: packet.QoS0})

	// 获取 client1 的订阅
	topics := tree.getClientTopics("client1")
	if len(topics) != 3 {
		t.Errorf("Expected 3 topics for client1, got %d", len(topics))
	}

	// 验证包含正确的主题
	topicMap := make(map[string]bool)
	for _, topic := range topics {
		topicMap[topic] = true
	}
	if !topicMap["topic1"] || !topicMap["topic2"] || !topicMap["topic3/subtopic"] {
		t.Error("Missing expected topics for client1")
	}

	// 获取不存在的客户端的订阅
	topics = tree.getClientTopics("nonexistent")
	if len(topics) != 0 {
		t.Errorf("Expected 0 topics for nonexistent client, got %d", len(topics))
	}
}

// TestSubscriptionTree_WildcardEdgeCases 测试通配符边界情况
func TestSubscriptionTree_WildcardEdgeCases(t *testing.T) {
	tree := newSubTree()

	// 订阅各种通配符组合
	tree.subscribe("client1", "*", packet.SubscribeOptions{QoS: packet.QoS0})
	tree.subscribe("client2", "**", packet.SubscribeOptions{QoS: packet.QoS0})
	tree.subscribe("client3", "*/*", packet.SubscribeOptions{QoS: packet.QoS0})
	tree.subscribe("client4", "topic/*/**", packet.SubscribeOptions{QoS: packet.QoS0})

	tests := []struct {
		topic           string
		expectedClients []string
		description     string
	}{
		{
			topic:           "single",
			expectedClients: []string{"client1", "client2"},
			description:     "单层主题",
		},
		{
			topic:           "a/b",
			expectedClients: []string{"client2", "client3"},
			description:     "两层主题",
		},
		{
			topic:           "topic/x/y/z",
			expectedClients: []string{"client2", "client4"},
			description:     "多层主题",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			matches := tree.matchTopic(tt.topic)
			if len(matches) != len(tt.expectedClients) {
				t.Errorf("Expected %d matches for %s, got %d",
					len(tt.expectedClients), tt.topic, len(matches))
				t.Logf("Got matches: %+v", matches)
			}
			for _, clientID := range tt.expectedClients {
				if _, ok := matches[clientID]; !ok {
					t.Errorf("Expected client %s to match topic %s", clientID, tt.topic)
				}
			}
		})
	}
}

// TestSubscriptionTree_Concurrent 测试并发安全
func TestSubscriptionTree_Concurrent(t *testing.T) {
	tree := newSubTree()

	// 并发订阅和取消订阅
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			clientID := "client" + string(rune('0'+id))
			tree.subscribe(clientID, "topic1", packet.SubscribeOptions{QoS: packet.QoS0})
			tree.subscribe(clientID, "topic2", packet.SubscribeOptions{QoS: packet.QoS1})
			tree.matchTopic("topic1")
			tree.getClientTopics(clientID)
			tree.unsubscribeClient(clientID)
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证最终状态一致
	count := tree.subscriptionCount()
	t.Logf("Final subscription count: %d", count)
}

// TestSubscriptionTree_ComplexHierarchy 测试复杂层级结构
func TestSubscriptionTree_ComplexHierarchy(t *testing.T) {
	tree := newSubTree()

	// 构建复杂的订阅层级
	tree.subscribe("client1", "home/living-room/temp", packet.SubscribeOptions{QoS: packet.QoS0})
	tree.subscribe("client2", "home/living-room/*", packet.SubscribeOptions{QoS: packet.QoS1})
	tree.subscribe("client3", "home/*/temp", packet.SubscribeOptions{QoS: packet.QoS0})
	tree.subscribe("client4", "home/**", packet.SubscribeOptions{QoS: packet.QoS1})
	tree.subscribe("client5", "*/*/*", packet.SubscribeOptions{QoS: packet.QoS0})

	// 测试复杂匹配
	matches := tree.matchTopic("home/living-room/temp")
	expectedClients := map[string]bool{
		"client1": true,
		"client2": true,
		"client3": true,
		"client4": true,
		"client5": true,
	}

	if len(matches) != len(expectedClients) {
		t.Errorf("Expected %d matches, got %d", len(expectedClients), len(matches))
		t.Logf("Got matches: %+v", matches)
	}

	for clientID := range expectedClients {
		if _, ok := matches[clientID]; !ok {
			t.Errorf("Expected client %s to match", clientID)
		}
	}
}

// TestSubscriptionTree_MultiWildcardInMiddle 测试 ** 位于模式中间时的匹配
func TestSubscriptionTree_MultiWildcardInMiddle(t *testing.T) {
	// ** 只能作为最后一层；中缀过滤器即使通过内部接口写入，也不应匹配任何主题
	tree := newSubTree()
	tree.subscribe("c1", "a/**/b", packet.SubscribeOptions{QoS: packet.QoS0})
	tree.subscribe("c2", "a/**", packet.SubscribeOptions{QoS: packet.QoS1})
	tree.subscribe("c3", "**/b", packet.SubscribeOptions{QoS: packet.QoS0})

	tests := []struct {
		topic    string
		expected []string
	}{
		{"a/b", []string{"c2"}},
		{"a/x/b", []string{"c2"}},
		{"a/x/y/b", []string{"c2"}},
		{"a/x", []string{"c2"}},
		{"a/x/y", []string{"c2"}},
		{"x/b", nil},
		{"b", nil},
	}

	for _, tt := range tests {
		t.Run(tt.topic, func(t *testing.T) {
			matches := tree.matchTopic(tt.topic)
			if len(matches) != len(tt.expected) {
				t.Errorf("topic %s: got %d matches (want %d): %+v",
					tt.topic, len(matches), len(tt.expected), matches)
				return
			}
			for _, clientID := range tt.expected {
				if _, ok := matches[clientID]; !ok {
					t.Errorf("topic %s: expected client %s to match", tt.topic, clientID)
				}
			}
		})
	}
}

// TestSubscriptionTree_ConsecutiveMultiWildcard 测试连续 ** 折叠
func TestSubscriptionTree_ConsecutiveMultiWildcard(t *testing.T) {
	// a/**/**/b 含非末层 **，属于非法过滤器：可被写入/删除，但永远不匹配
	tree := newSubTree()
	tree.subscribe("c1", "a/**/**/b", packet.SubscribeOptions{QoS: packet.QoS0})

	for _, topic := range []string{"a/x/b", "a/x/y/b", "a/x"} {
		if _, ok := tree.matchTopic(topic)["c1"]; ok {
			t.Errorf("a/**/**/b is invalid and should not match %s", topic)
		}
	}

	if !tree.unsubscribe("c1", "a/**/**/b") {
		t.Errorf("unsubscribe with folded pattern should succeed")
	}
	if _, ok := tree.matchTopic("a/x/b")["c1"]; ok {
		t.Errorf("subscription should be removed after unsubscribe")
	}
}

// TestSubscriptionTree_PrunesEmptyNodes 验证取消订阅后会剪掉空节点，避免主题 churn 造成内存泄漏。
func TestSubscriptionTree_PrunesEmptyNodes(t *testing.T) {
	tree := newSubTree()
	opts := packet.SubscribeOptions{QoS: packet.QoS0}

	for i := 0; i < 100; i++ {
		tree.subscribe("c1", fmt.Sprintf("device/%d/telemetry", i), opts)
	}
	if got := len(tree.root.children); got != 1 {
		t.Fatalf("expected only the shared 'device' node, got %d root children", got)
	}

	if removed := tree.unsubscribeClient("c1"); removed != 100 {
		t.Fatalf("expected 100 removed subscriptions, got %d", removed)
	}
	if got := len(tree.root.children); got != 0 {
		t.Fatalf("expected tree to be pruned, got %d root children", got)
	}
}

// TestSubscriptionTree_PruneKeepsSharedNodes 验证剪枝不会删除仍被其它订阅使用的节点。
func TestSubscriptionTree_PruneKeepsSharedNodes(t *testing.T) {
	tree := newSubTree()
	opts := packet.SubscribeOptions{QoS: packet.QoS0}

	tree.subscribe("c1", "a/b/c", opts)
	tree.subscribe("c2", "a/b/d", opts)

	if ok := tree.unsubscribe("c1", "a/b/c"); !ok {
		t.Fatal("expected unsubscribe to succeed")
	}
	if _, ok := tree.matchTopic("a/b/d")["c2"]; !ok {
		t.Fatal("shared prefix must keep the other subscription alive")
	}

	if ok := tree.unsubscribe("c2", "a/b/d"); !ok {
		t.Fatal("expected unsubscribe to succeed")
	}
	if got := len(tree.root.children); got != 0 {
		t.Fatalf("expected tree to be pruned after last unsubscribe, got %d root children", got)
	}
}

// TestWildcardSemantics_ConsistentAcrossImplementations 验证 packet.MatchTopic、
// 订阅树和保留消息索引使用同一套通配符语义（** 仅允许作为最后一层）。
func TestWildcardSemantics_ConsistentAcrossImplementations(t *testing.T) {
	filters := []string{"a", "a/b", "a/*", "a/**", "*", "**", "*/b", "a/*/c", "a/b/**"}
	topics := []string{"a", "a/b", "a/b/c", "a/b/c/d", "a/x", "b", "x/b", "a//b"}

	for _, filter := range filters {
		if !packet.ValidateTopicFilter(filter) {
			t.Fatalf("test filter %q should be valid", filter)
		}
		for _, topic := range topics {
			want := packet.MatchTopic(filter, topic)

			tree := newSubTree()
			tree.subscribe("c1", filter, packet.SubscribeOptions{QoS: packet.QoS0})
			_, inTree := tree.matchTopic(topic)["c1"]

			store := newRetainStore()
			if err := store.set(topic, 0); err != nil {
				t.Fatalf("retain set failed: %v", err)
			}
			inRetain := len(store.matchTopics(filter)) == 1

			if inTree != want {
				t.Errorf("subTree(%q, %q) = %v, packet.MatchTopic = %v", filter, topic, inTree, want)
			}
			if inRetain != want {
				t.Errorf("retainStore(%q, %q) = %v, packet.MatchTopic = %v", filter, topic, inRetain, want)
			}
		}
	}
}
