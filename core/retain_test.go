package core

import (
	"testing"

	"github.com/snple/beacon/packet"
)

func TestRetainStoreMatchForSubscription(t *testing.T) {
	store := newRetainStore()
	pub := packet.NewPublishPacket("test/topic", []byte("retained message"))
	pub.Retain = true
	pub.QoS = packet.QoS1
	msg := &Message{Packet: pub, Retain: true, Timestamp: 0}
	store.set("test/topic", msg)

	tests := []struct {
		name              string
		retainAsPublished bool
		isNewSubscription bool
		retainHandling    uint8
		expectMsgCount    int
		expectRetainFlag  bool
	}{
		{"RH 0, always send", true, false, 0, 1, true},
		{"RH 0, clear retain", false, false, 0, 1, false},
		{"RH 1, new sub", true, true, 1, 1, true},
		{"RH 1, existing sub", true, false, 1, 0, false},
		{"RH 2, never send", true, true, 2, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgs := store.matchForSubscription("test/topic", tt.isNewSubscription, tt.retainAsPublished, tt.retainHandling)
			if len(msgs) != tt.expectMsgCount {
				t.Errorf("Count mismatch: got %d, want %d", len(msgs), tt.expectMsgCount)
			}
			if tt.expectMsgCount > 0 && msgs[0].Retain != tt.expectRetainFlag {
				t.Errorf("Retain flag mismatch: got %v, want %v", msgs[0].Retain, tt.expectRetainFlag)
			}
		})
	}
}

// TestRetainStoreBasicOperations 测试基本操作
func TestRetainStoreBasicOperations(t *testing.T) {
	store := newRetainStore()

	// 测试 set 和 get
	pub1 := packet.NewPublishPacket("sensor/temperature", []byte("25.5"))
	pub1.Retain = true
	msg1 := &Message{Packet: pub1, Retain: true, Timestamp: 123}

	if err := store.set("sensor/temperature", msg1); err != nil {
		t.Fatalf("Failed to set retain message: %v", err)
	}

	retrieved := store.get("sensor/temperature")
	if retrieved == nil {
		t.Fatal("Failed to get retain message")
	}
	if string(retrieved.Packet.Payload) != "25.5" {
		t.Errorf("Payload mismatch: got %s, want 25.5", retrieved.Packet.Payload)
	}

	// 测试覆盖
	pub2 := packet.NewPublishPacket("sensor/temperature", []byte("26.0"))
	pub2.Retain = true
	msg2 := &Message{Packet: pub2, Retain: true, Timestamp: 124}

	if err := store.set("sensor/temperature", msg2); err != nil {
		t.Fatalf("Failed to update retain message: %v", err)
	}

	retrieved = store.get("sensor/temperature")
	if string(retrieved.Packet.Payload) != "26.0" {
		t.Errorf("Updated payload mismatch: got %s, want 26.0", retrieved.Packet.Payload)
	}

	// 测试 remove
	store.remove("sensor/temperature")
	retrieved = store.get("sensor/temperature")
	if retrieved != nil {
		t.Error("Message should be removed")
	}
}

// TestRetainStoreInvalidMessages 测试无效消息
func TestRetainStoreInvalidMessages(t *testing.T) {
	store := newRetainStore()

	// nil 消息
	if err := store.set("test", nil); err != ErrInvalidRetainMessage {
		t.Errorf("Expected ErrInvalidRetainMessage for nil message, got %v", err)
	}

	// Retain 标志为 false
	pub := packet.NewPublishPacket("test", []byte("data"))
	pub.Retain = false
	msg := &Message{Packet: pub, Retain: false}
	if err := store.set("test", msg); err != ErrInvalidRetainMessage {
		t.Errorf("Expected ErrInvalidRetainMessage for non-retain message, got %v", err)
	}
}

// TestRetainStoreWildcardMatching 测试通配符匹配
func TestRetainStoreWildcardMatching(t *testing.T) {
	store := newRetainStore()

	// 准备测试数据
	topics := []string{
		"home/room1/temperature",
		"home/room1/humidity",
		"home/room2/temperature",
		"home/room2/humidity",
		"office/desk1/light",
		"office/desk2/light",
	}

	for i, topic := range topics {
		pub := packet.NewPublishPacket(topic, []byte{byte(i)})
		pub.Retain = true
		msg := &Message{Packet: pub, Retain: true}
		store.set(topic, msg)
	}

	tests := []struct {
		pattern string
		want    int
	}{
		{"home/room1/temperature", 1},    // 精确匹配
		{"home/room1/*", 2},               // 单层通配符
		{"home/*/temperature", 2},         // 单层通配符
		{"home/**", 4},                    // 多层通配符
		{"office/**", 2},                  // 多层通配符
		{"**", 6},                         // 匹配所有
		{"home/room3/*", 0},               // 无匹配
		{"*/room1/temperature", 1},        // 单层通配符在开头
		{"home/room1/*/extra", 0},         // 路径不匹配
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			msgs := store.match(tt.pattern)
			if len(msgs) != tt.want {
				t.Errorf("Pattern %s: got %d messages, want %d", tt.pattern, len(msgs), tt.want)
			}
		})
	}
}

// TestRetainStoreEmptyPayloadRemoval 测试空载荷删除
func TestRetainStoreEmptyPayloadRemoval(t *testing.T) {
	store := newRetainStore()

	// 设置保留消息
	pub := packet.NewPublishPacket("test/topic", []byte("data"))
	pub.Retain = true
	msg := &Message{Packet: pub, Retain: true}
	store.set("test/topic", msg)

	if store.count != 1 {
		t.Errorf("Count should be 1, got %d", store.count)
	}

	// 删除
	store.remove("test/topic")
	if store.count != 0 {
		t.Errorf("Count should be 0 after removal, got %d", store.count)
	}

	// 验证确实被删除
	retrieved := store.get("test/topic")
	if retrieved != nil {
		t.Error("Message should be removed")
	}
}

// TestRetainStoreClear 测试清空
func TestRetainStoreClear(t *testing.T) {
	store := newRetainStore()

	// 添加多个消息
	for i := 0; i < 10; i++ {
		topic := "test/" + string(rune('a'+i))
		pub := packet.NewPublishPacket(topic, []byte{byte(i)})
		pub.Retain = true
		msg := &Message{Packet: pub, Retain: true}
		store.set(topic, msg)
	}

	if store.count != 10 {
		t.Errorf("Count should be 10, got %d", store.count)
	}

	// 清空
	store.clear()

	if store.count != 0 {
		t.Errorf("Count should be 0 after clear, got %d", store.count)
	}

	// 验证所有消息都被清空
	msgs := store.match("**")
	if len(msgs) != 0 {
		t.Errorf("Should have no messages after clear, got %d", len(msgs))
	}
}

// TestRetainStoreNestedTopics 测试嵌套主题
func TestRetainStoreNestedTopics(t *testing.T) {
	store := newRetainStore()

	topics := []string{
		"a",
		"a/b",
		"a/b/c",
		"a/b/c/d",
	}

	for _, topic := range topics {
		pub := packet.NewPublishPacket(topic, []byte(topic))
		pub.Retain = true
		msg := &Message{Packet: pub, Retain: true}
		store.set(topic, msg)
	}

	// 测试精确匹配
	msg := store.get("a/b/c")
	if msg == nil || string(msg.Packet.Payload) != "a/b/c" {
		t.Error("Failed to get nested topic")
	}

	// 测试通配符
	msgs := store.match("a/**")
	if len(msgs) != 4 {
		t.Errorf("Expected 4 messages for a/**, got %d", len(msgs))
	}

	msgs = store.match("a/b/**")
	if len(msgs) != 3 {
		t.Errorf("Expected 3 messages for a/b/**, got %d", len(msgs))
	}
}

// TestRetainStoreConcurrency 测试并发安全
func TestRetainStoreConcurrency(t *testing.T) {
	store := newRetainStore()

	// 并发写入
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				topic := "concurrent/test/" + string(rune('a'+id))
				pub := packet.NewPublishPacket(topic, []byte{byte(j)})
				pub.Retain = true
				msg := &Message{Packet: pub, Retain: true}
				store.set(topic, msg)
			}
			done <- true
		}(i)
	}

	// 并发读取
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				store.match("concurrent/**")
				store.get("concurrent/test/a")
			}
			done <- true
		}()
	}

	// 等待完成
	for i := 0; i < 20; i++ {
		<-done
	}

	// 验证最终状态
	msgs := store.match("concurrent/**")
	if len(msgs) != 10 {
		t.Errorf("Expected 10 messages after concurrent operations, got %d", len(msgs))
	}
}
