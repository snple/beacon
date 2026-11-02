package core

import (
	"testing"
	"time"

	"github.com/snple/beacon/packet"
)

// newTestRetainStore 创建一个使用独立消息表的 retainStore。
func newTestRetainStore() *retainStore {
	return newRetainStore(newMessageStore())
}

// testRetainMessage 构造一条已注册的保留消息（引用计数 1，交给 retainStore）。
func testRetainMessage(topic string) *Message {
	msg := createTestMessage(topic, []byte("payload"), packet.QoS0)
	msg.Retain = true
	msg.Packet.Retain = true
	return msg
}

// TestRetainStoreBasicOperations 测试基本操作
func TestRetainStoreBasicOperations(t *testing.T) {
	store := newTestRetainStore()

	// 测试 set 和 exists
	if err := store.set("sensor/temperature", testRetainMessage("sensor/temperature"), 0); err != nil {
		t.Fatalf("Failed to set retain message index: %v", err)
	}

	if store.getEntry("sensor/temperature") == nil {
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
	if err := store.set("sensor/temperature", testRetainMessage("sensor/temperature"), 12345); err != nil {
		t.Fatalf("Failed to update retain message index: %v", err)
	}

	entry = store.getEntry("sensor/temperature")
	if entry.expiryTime != 12345 {
		t.Errorf("ExpiryTime mismatch: got %d, want 12345", entry.expiryTime)
	}

	// 测试 remove
	store.remove("sensor/temperature")
	if store.getEntry("sensor/temperature") != nil {
		t.Error("Topic should be removed")
	}
}

// TestRetainStoreInvalidTopics 测试无效主题
func TestRetainStoreInvalidTopics(t *testing.T) {
	store := newTestRetainStore()

	// 空主题
	if err := store.set("", testRetainMessage(""), 0); err != ErrInvalidRetainMessage {
		t.Errorf("Expected ErrInvalidRetainMessage for empty topic, got %v", err)
	}
}

// TestRetainStoreWildcardMatching 测试通配符匹配
func TestRetainStoreWildcardMatching(t *testing.T) {
	store := newTestRetainStore()

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
		store.set(topic, testRetainMessage(topic), 0)
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

		// ** 只能作为最后一层：位于中间的过滤器非法，不匹配任何主题
		{"home/**/temperature", 0},
		{"home/room1/**/temperature", 0},
		{"*/**/light", 0},
		{"home/**/extra", 0},
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
	store := newTestRetainStore()

	now := time.Now().Unix()

	// 设置一个已过期的索引
	store.set("expired/topic", testRetainMessage("expired/topic"), now-100)

	// 设置一个未过期的索引
	store.set("valid/topic", testRetainMessage("valid/topic"), now+100)

	// 设置一个永不过期的索引
	store.set("permanent/topic", testRetainMessage("permanent/topic"), 0)

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
	if store.getEntry("expired/topic") != nil {
		t.Error("Expired topic should be removed after cleanup")
	}
	if got := len(store.matchTopics("**")); got != 2 {
		t.Errorf("Count should be 2 after cleanup, got %d", got)
	}
}

// TestRetainStoreRemoval 测试删除
func TestRetainStoreRemoval(t *testing.T) {
	store := newTestRetainStore()

	// 设置保留消息索引
	store.set("test/topic", testRetainMessage("test/topic"), 0)

	if got := len(store.matchTopics("**")); got != 1 {
		t.Errorf("Count should be 1, got %d", got)
	}

	// 删除
	store.remove("test/topic")
	if got := len(store.matchTopics("**")); got != 0 {
		t.Errorf("Count should be 0 after removal, got %d", got)
	}

	// 验证确实被删除
	if store.getEntry("test/topic") != nil {
		t.Error("Topic should be removed")
	}
}

// TestRetainStoreClear 测试清空
func TestRetainStoreClear(t *testing.T) {
	store := newTestRetainStore()

	// 添加多个索引
	for i := 0; i < 10; i++ {
		topic := "test/" + string(rune('a'+i))
		store.set(topic, testRetainMessage(topic), 0)
	}

	if got := len(store.matchTopics("**")); got != 10 {
		t.Errorf("Count should be 10, got %d", got)
	}

	// 清空
	store.clear()

	if got := len(store.matchTopics("**")); got != 0 {
		t.Errorf("Count should be 0 after clear, got %d", got)
	}

	// 验证所有索引都被清空
	topics := store.matchTopics("**")
	if len(topics) != 0 {
		t.Errorf("Should have no topics after clear, got %d", len(topics))
	}
}

// TestRetainStoreNestedTopics 测试嵌套主题
func TestRetainStoreNestedTopics(t *testing.T) {
	store := newTestRetainStore()

	topics := []string{
		"a",
		"a/b",
		"a/b/c",
		"a/b/c/d",
	}

	for _, topic := range topics {
		store.set(topic, testRetainMessage(topic), 0)
	}

	// 测试精确匹配
	if store.getEntry("a/b/c") == nil {
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
	store := newTestRetainStore()

	// 并发写入
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				topic := "concurrent/test/" + string(rune('a'+id))
				store.set(topic, testRetainMessage(topic), 0)
			}
			done <- true
		}(i)
	}

	// 并发读取
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				store.matchTopics("concurrent/**")
				_ = store.getEntry("concurrent/test/a")
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

		// ** 只能作为最后一层：位于中间的过滤器非法，不匹配任何主题
		{"a/**/b", "a/b", false},
		{"a/**/b", "a/x/b", false},
		{"a/**/b", "a/x/y/b", false},
		{"a/**/b/**", "a/b/c", false},
		{"**/b", "a/b", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"->"+tt.topic, func(t *testing.T) {
			got := packet.MatchTopic(tt.pattern, tt.topic)
			if got != tt.want {
				t.Errorf("packet.MatchTopic(%s, %s) = %v, want %v", tt.pattern, tt.topic, got, tt.want)
			}
		})
	}
}

// TestRetainStore_MatchMessages 验证保留消息直接引用共享消息表，不复制内容。
func TestRetainStore_MatchMessages(t *testing.T) {
	store := newTestRetainStore()

	const topic = "sensor/temperature"
	msg := testRetainMessage(topic)
	if err := store.set(topic, msg, 0); err != nil {
		t.Fatalf("Failed to set retain message: %v", err)
	}

	if store.getEntry(topic) == nil {
		t.Error("Topic should exist in retainStore")
	}

	msgs := store.matchMessages("sensor/**")
	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgs))
	}
	if msgs[0] != msg {
		t.Error("retainStore should reference the shared message, not a copy")
	}

	store.store.release(msgs[0])
}

// TestRetainStoreScalability 测试大量消息的可扩展性
func TestRetainStoreScalability(t *testing.T) {
	store := newTestRetainStore()

	// 添加大量主题（模拟数万条记录的场景）
	count := 10000
	for i := 0; i < count; i++ {
		// 使用唯一的主题格式
		topic := "device/" + string(rune('a'+i%26)) + "/" + string(rune('a'+(i/26)%26)) + "/" + string(rune('a'+(i/676)%26)) + "/status/" + string(rune('0'+i%10))
		store.set(topic, testRetainMessage(topic), 0)
	}

	if got := len(store.matchTopics("**")); got != count {
		t.Errorf("Count should be %d, got %d", count, got)
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

// TestRetainStore_PrunesEmptyNodes 验证删除保留消息后会剪掉空节点。
func TestRetainStore_PrunesEmptyNodes(t *testing.T) {
	store := newTestRetainStore()
	if err := store.set("a/b/c", testRetainMessage("a/b/c"), 0); err != nil {
		t.Fatal(err)
	}
	if got := len(store.root.children); got != 1 {
		t.Fatalf("expected 1 root child, got %d", got)
	}

	store.remove("a/b/c")
	if got := len(store.root.children); got != 0 {
		t.Fatalf("expected empty tree after remove, got %d root children", got)
	}
	if got := len(store.matchTopics("**")); got != 0 {
		t.Fatalf("expected count 0, got %d", got)
	}
}

// TestRetainStore_ExpiredCleanupPrunesNodes 验证过期清理同样会剪枝。
func TestRetainStore_ExpiredCleanupPrunesNodes(t *testing.T) {
	store := newTestRetainStore()
	if err := store.set("a/b/c", testRetainMessage("a/b/c"), time.Now().Unix()-1); err != nil {
		t.Fatal(err)
	}

	count, topics := store.cleanupExpired()
	if count != 1 || len(topics) != 1 {
		t.Fatalf("expected 1 expired entry, got count=%d topics=%v", count, topics)
	}
	if got := len(store.root.children); got != 0 {
		t.Fatalf("expected empty tree after expiry cleanup, got %d root children", got)
	}
}
