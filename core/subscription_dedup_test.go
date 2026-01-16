package core

import (
	"testing"

	"github.com/snple/beacon/packet"
)

// TestSubscriptionDeduplication 测试订阅去重功能
func TestSubscriptionDeduplication(t *testing.T) {
	tree := newSubscriptionTree()

	// 客户端订阅多个重叠的模式
	tree.Add("client1", "sensor/**", packet.QoS0)
	tree.Add("client1", "sensor/*/temp", packet.QoS1)

	// 测试 match（自动去重并保留最高 QoS）
	t.Run("Match_AutoDedup", func(t *testing.T) {
		matches := tree.match("sensor/room1/temp")

		// 应该返回 1 条记录（自动去重）
		if len(matches) != 1 {
			t.Errorf("Expected 1 unique match, got %d", len(matches))
		}

		// 验证是 client1
		qos, ok := matches["client1"]
		if !ok {
			t.Error("Expected client1 in results")
		}

		// 应该保留最高的 QoS (QoS1)
		if qos != packet.QoS1 {
			t.Errorf("Expected QoS1, got %v", qos)
		}
	})
}

// TestQoSMaxSelection 测试 QoS 最大值选择逻辑
func TestQoSMaxSelection(t *testing.T) {
	tests := []struct {
		name          string
		subscriptions []struct {
			topic string
			qos   packet.QoS
		}
		testTopic   string
		expectedQoS packet.QoS
	}{
		{
			name: "KeepHigherQoS_QoS1",
			subscriptions: []struct {
				topic string
				qos   packet.QoS
			}{
				{"sensor/**", packet.QoS0},
				{"sensor/*/temp", packet.QoS1},
			},
			testTopic:   "sensor/room1/temp",
			expectedQoS: packet.QoS1,
		},
		{
			name: "MultipleQoS0",
			subscriptions: []struct {
				topic string
				qos   packet.QoS
			}{
				{"sensor/**", packet.QoS0},
				{"sensor/temp/**", packet.QoS0},
			},
			testTopic:   "sensor/temp/location1",
			expectedQoS: packet.QoS0,
		},
		{
			name: "MultipleQoS1",
			subscriptions: []struct {
				topic string
				qos   packet.QoS
			}{
				{"sensor/**", packet.QoS1},
				{"sensor/*/temp", packet.QoS1},
			},
			testTopic:   "sensor/room1/temp",
			expectedQoS: packet.QoS1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := newSubscriptionTree()

			// 添加所有订阅
			for _, sub := range tt.subscriptions {
				tree.Add("client1", sub.topic, sub.qos)
			}

			// 匹配主题
			matches := tree.match(tt.testTopic)

			// 验证结果
			if len(matches) != 1 {
				t.Errorf("Expected 1 unique match, got %d", len(matches))
			}

			qos, ok := matches["client1"]
			if !ok {
				t.Error("Expected client1 in results")
				return
			}

			if qos != tt.expectedQoS {
				t.Errorf("Expected QoS %v, got %v", tt.expectedQoS, qos)
			}
		})
	}
}

// TestMultipleSubscriptionsWithWildcards 测试复杂的通配符订阅去重
func TestMultipleSubscriptionsWithWildcards(t *testing.T) {
	tree := newSubscriptionTree()

	// 设置多个重叠的订阅
	tree.Add("client1", "sensor/**", packet.QoS0)          // 匹配所有 sensor 下的主题
	tree.Add("client1", "sensor/temp/**", packet.QoS0)     // 匹配 sensor/temp 下的所有主题
	tree.Add("client1", "sensor/*/location1", packet.QoS1) // 匹配特定位置

	// 测试发布到 sensor/temp/location1
	topic := "sensor/temp/location1"

	// match 应该自动去重，返回 1 条记录
	matches := tree.match(topic)
	if len(matches) != 1 {
		t.Errorf("match() expected 1 unique client, got %d", len(matches))
	}

	// 验证 client1 的 QoS 是最高的 (QoS1)
	qos, ok := matches["client1"]
	if !ok {
		t.Error("Expected client1 in results")
		return
	}

	if qos != packet.QoS1 {
		t.Errorf("Expected QoS1 (highest), got %v", qos)
	}
}

// TestMixedClientsWithOverlappingSubscriptions 测试多个客户端的重叠订阅
func TestMixedClientsWithOverlappingSubscriptions(t *testing.T) {
	tree := newSubscriptionTree()

	// client1 有重叠订阅
	tree.Add("client1", "sensor/**", packet.QoS0)
	tree.Add("client1", "sensor/temp/*", packet.QoS1)

	// client2 有单个订阅
	tree.Add("client2", "sensor/temp/room1", packet.QoS1)

	// client3 有重叠订阅
	tree.Add("client3", "sensor/**", packet.QoS1)
	tree.Add("client3", "**", packet.QoS0)

	topic := "sensor/temp/room1"

	// match 应该返回 3 条记录（每个客户端一条，自动去重）
	matches := tree.match(topic)
	if len(matches) != 3 {
		t.Errorf("match() expected 3 unique clients, got %d", len(matches))
	}

	// 验证每个客户端的 QoS
	expected := map[string]packet.QoS{
		"client1": packet.QoS1, // max(QoS0, QoS1) = QoS1
		"client2": packet.QoS1, // QoS1
		"client3": packet.QoS1, // max(QoS1, QoS0) = QoS1
	}

	for clientID, expectedQoS := range expected {
		actualQoS, exists := matches[clientID]
		if !exists {
			t.Errorf("Client %s not found in matches", clientID)
			continue
		}
		if actualQoS != expectedQoS {
			t.Errorf("Client %s: expected QoS %v, got %v", clientID, expectedQoS, actualQoS)
		}
	}
}
