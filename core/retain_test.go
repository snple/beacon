package core

import (
	"testing"
	"time"

	"github.com/snple/beacon/packet"
)

// TestRetainStoreBasicOperations 测试基本操作
func TestRetainStoreBasicOperations(t *testing.T) {
	store := newRetainStore()

	// 测试 set 和 exists
	if err := store.set("sensor/temperature", 0); err != nil {
		t.Fatalf("Failed to set retain message index: %v", err)
	}

	if !store.exists("sensor/temperature") {
		t.Fatal("Topic should exist")
	}

	// 测试 getEntry
	entry := store.getEntry("sensor/temperature")
	if entry == nil {
		t.Fatal("Failed to get entry")
	}
	if entry.topic != "sensor/temperature" {
		t.Errorf("Topic mismatch: got %s, want sensor/temperature", entry.topic)
	}

	// 测试覆盖
	if err := store.set("sensor/temperature", 12345); err != nil {
		t.Fatalf("Failed to update retain message index: %v", err)
	}

	entry = store.getEntry("sensor/temperature")
	if entry.expiryTime != 12345 {
		t.Errorf("ExpiryTime mismatch: got %d, want 12345", entry.expiryTime)
	}

	// 测试 remove
	store.remove("sensor/temperature")
	if store.exists("sensor/temperature") {
		t.Error("Topic should be removed")
	}
}

// TestRetainStoreInvalidTopics 测试无效主题
func TestRetainStoreInvalidTopics(t *testing.T) {
	store := newRetainStore()

	// 空主题
	if err := store.set("", 0); err != ErrInvalidRetainMessage {
		t.Errorf("Expected ErrInvalidRetainMessage for empty topic, got %v", err)
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

	for _, topic := range topics {
		store.set(topic, 0)
	}

	tests := []struct {
		pattern string
		want    int
	}{
		{"home/room1/temperature", 1}, // 精确匹配
		{"home/room1/*", 2},           // 单层通配符
		{"home/*/temperature", 2},     // 单层通配符
		{"home/**", 4},                // 多层通配符
		{"office/**", 2},              // 多层通配符
		{"**", 6},                     // 匹配所有
		{"home/room3/*", 0},           // 无匹配
		{"*/room1/temperature", 1},    // 单层通配符在开头
		{"home/room1/*/extra", 0},     // 路径不匹配
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			topics := store.matchTopics(tt.pattern)
			if len(topics) != tt.want {
				t.Errorf("Pattern %s: got %d topics, want %d", tt.pattern, len(topics), tt.want)
			}
		})
	}
}

// TestRetainStoreExpiry 测试过期功能
func TestRetainStoreExpiry(t *testing.T) {
	store := newRetainStore()

	now := time.Now().Unix()

	// 设置一个已过期的索引
	store.set("expired/topic", now-100)

	// 设置一个未过期的索引
	store.set("valid/topic", now+100)

	// 设置一个永不过期的索引
	store.set("permanent/topic", 0)

	// 验证过期检查
	entry := store.getEntry("expired/topic")
	if !store.isExpired(entry, now) {
		t.Error("Expired topic should be marked as expired")
	}

	entry = store.getEntry("valid/topic")
	if store.isExpired(entry, now) {
		t.Error("Valid topic should not be marked as expired")
	}

	entry = store.getEntry("permanent/topic")
	if store.isExpired(entry, now) {
		t.Error("Permanent topic should not be marked as expired")
	}

	// matchTopics 应该过滤掉已过期的
	topics := store.matchTopics("**")
	if len(topics) != 2 {
		t.Errorf("Expected 2 non-expired topics, got %d", len(topics))
	}

	// 测试清理过期条目
	count, expiredTopics := store.cleanupExpired()
	if count != 1 {
		t.Errorf("Expected 1 expired topic cleaned, got %d", count)
	}
	if len(expiredTopics) != 1 || expiredTopics[0] != "expired/topic" {
		t.Errorf("Expected expired/topic to be cleaned, got %v", expiredTopics)
	}

	// 验证清理后的状态
	if store.exists("expired/topic") {
		t.Error("Expired topic should be removed after cleanup")
	}
	if store.count != 2 {
		t.Errorf("Count should be 2 after cleanup, got %d", store.count)
	}
}

// TestRetainStoreRemoval 测试删除
func TestRetainStoreRemoval(t *testing.T) {
	store := newRetainStore()

	// 设置保留消息索引
	store.set("test/topic", 0)

	if store.count != 1 {
		t.Errorf("Count should be 1, got %d", store.count)
	}

	// 删除
	store.remove("test/topic")
	if store.count != 0 {
		t.Errorf("Count should be 0 after removal, got %d", store.count)
	}

	// 验证确实被删除
	if store.exists("test/topic") {
		t.Error("Topic should be removed")
	}
}

// TestRetainStoreClear 测试清空
func TestRetainStoreClear(t *testing.T) {
	store := newRetainStore()

	// 添加多个索引
	for i := 0; i < 10; i++ {
		topic := "test/" + string(rune('a'+i))
		store.set(topic, 0)
	}

	if store.count != 10 {
		t.Errorf("Count should be 10, got %d", store.count)
	}

	// 清空
	store.clear()

	if store.count != 0 {
		t.Errorf("Count should be 0 after clear, got %d", store.count)
	}

	// 验证所有索引都被清空
	topics := store.matchTopics("**")
	if len(topics) != 0 {
		t.Errorf("Should have no topics after clear, got %d", len(topics))
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
		store.set(topic, 0)
	}

	// 测试精确匹配
	if !store.exists("a/b/c") {
		t.Error("Failed to find nested topic")
	}

	// 测试通配符
	matched := store.matchTopics("a/**")
	if len(matched) != 4 {
		t.Errorf("Expected 4 topics for a/**, got %d", len(matched))
	}

	matched = store.matchTopics("a/b/**")
	if len(matched) != 3 {
		t.Errorf("Expected 3 topics for a/b/**, got %d", len(matched))
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
				store.set(topic, 0)
			}
			done <- true
		}(i)
	}

	// 并发读取
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				store.matchTopics("concurrent/**")
				store.exists("concurrent/test/a")
			}
			done <- true
		}()
	}

	// 等待完成
	for i := 0; i < 20; i++ {
		<-done
	}

	// 验证最终状态
	topics := store.matchTopics("concurrent/**")
	if len(topics) != 10 {
		t.Errorf("Expected 10 topics after concurrent operations, got %d", len(topics))
	}
}

// TestMatchTopic 测试主题匹配函数
func TestMatchTopic(t *testing.T) {
	tests := []struct {
		pattern string
		topic   string
		want    bool
	}{
		// 精确匹配
		{"a/b/c", "a/b/c", true},
		{"a/b/c", "a/b/d", false},
		{"a/b/c", "a/b", false},

		// 单层通配符
		{"a/*/c", "a/b/c", true},
		{"a/*/c", "a/x/c", true},
		{"a/*/c", "a/b/d", false},
		{"*/b/c", "a/b/c", true},
		{"a/b/*", "a/b/c", true},

		// 多层通配符
		{"a/**", "a/b/c", true},
		{"a/**", "a/b/c/d", true},
		{"a/**", "a", true},
		{"**", "a/b/c", true},
		{"a/b/**", "a/b", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"->"+tt.topic, func(t *testing.T) {
			got := matchTopic(tt.pattern, tt.topic)
			if got != tt.want {
				t.Errorf("matchTopic(%s, %s) = %v, want %v", tt.pattern, tt.topic, got, tt.want)
			}
		})
	}
}

// TestRetainStoreWithMessageStore 测试与 messageStore 集成
func TestRetainStoreWithMessageStore(t *testing.T) {
	// 创建内存模式的 messageStore
	config := StoreOptions{
		DataDir:          "", // InMemory 模式
		ValueLogFileSize: 64,
	}

	ms, err := newStore(config)
	if err != nil {
		t.Fatalf("Failed to create message store: %v", err)
	}
	defer ms.close()

	store := newRetainStore()

	// 模拟保存保留消息
	topic := "sensor/temperature"
	pub := packet.NewPublishPacket(topic, []byte("25.5"))
	pub.Retain = true
	msg := &Message{Packet: pub, Retain: true, QoS: packet.QoS1, Timestamp: time.Now().Unix()}

	// 存储索引到 retainStore
	if err := store.set(topic, 0); err != nil {
		t.Fatalf("Failed to set retain index: %v", err)
	}

	// 存储消息到 messageStore
	if err := ms.setRetain(topic, msg); err != nil {
		t.Fatalf("Failed to save retain message: %v", err)
	}

	// 验证索引存在
	if !store.exists(topic) {
		t.Error("Topic should exist in retainStore")
	}

	// 从 messageStore 获取消息
	retrieved, err := ms.getRetain(topic)
	if err != nil {
		t.Fatalf("Failed to get retain message: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Retrieved message should not be nil")
	}
	if string(retrieved.Packet.Payload) != "25.5" {
		t.Errorf("Payload mismatch: got %s, want 25.5", retrieved.Packet.Payload)
	}

	// 测试匹配并获取
	topics := store.matchTopics("sensor/**")
	if len(topics) != 1 {
		t.Errorf("Expected 1 topic, got %d", len(topics))
	}

	for _, tp := range topics {
		msg, err := ms.getRetain(tp)
		if err != nil {
			t.Errorf("Failed to get message for topic %s: %v", tp, err)
		}
		if msg == nil {
			t.Errorf("Message for topic %s should not be nil", tp)
		}
	}
}

// TestRetainStoreScalability 测试大量消息的可扩展性
func TestRetainStoreScalability(t *testing.T) {
	store := newRetainStore()

	// 添加大量主题（模拟数万条记录的场景）
	count := 10000
	for i := 0; i < count; i++ {
		// 使用唯一的主题格式
		topic := "device/" + string(rune('a'+i%26)) + "/" + string(rune('a'+(i/26)%26)) + "/" + string(rune('a'+(i/676)%26)) + "/status/" + string(rune('0'+i%10))
		store.set(topic, 0)
	}

	if store.count != count {
		t.Errorf("Count should be %d, got %d", count, store.count)
	}

	// 测试匹配性能
	start := time.Now()
	topics := store.matchTopics("device/**")
	elapsed := time.Since(start)

	if len(topics) != count {
		t.Errorf("Expected %d topics, got %d", count, len(topics))
	}

	// 匹配 10000 条记录应该在合理时间内完成
	if elapsed > time.Second {
		t.Errorf("Matching took too long: %v", elapsed)
	}

	t.Logf("Matched %d topics in %v", len(topics), elapsed)
}
