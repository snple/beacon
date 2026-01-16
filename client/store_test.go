package client

import (
	"os"
	"testing"
	"time"

	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// TestStoreConfig 测试存储配置
func TestStoreConfig(t *testing.T) {
	cfg := DefaultStoreConfig()

	if !cfg.Enabled {
		t.Error("Default config should be enabled")
	}
	if cfg.SyncWrites {
		t.Error("Default config should have SyncWrites=false")
	}
	if cfg.ValueLogFileSize != 256 {
		t.Errorf("Default ValueLogFileSize = %d, want 256", cfg.ValueLogFileSize)
	}

	// 测试验证
	if err := cfg.Validate(); err != nil {
		t.Errorf("Default config validation failed: %v", err)
	}

	// 测试无效配置
	cfg.ValueLogFileSize = 0
	if err := cfg.Validate(); err == nil {
		t.Error("Validation should fail with ValueLogFileSize=0")
	}
}

// TestMessageStoreInMemory 测试内存模式的消息存储
func TestMessageStoreInMemory(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	cfg := StoreConfig{
		Enabled:    true,
		DataDir:    "", // 内存模式
		SyncWrites: false,
		GCInterval: 0, // 禁用 GC
		Logger:     logger,
	}

	store, err := NewMessageStore(cfg)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 测试保存消息
	msg := &StoredMessage{
		PacketID:       1,
		Topic:          "test/topic",
		Payload:        []byte("hello world"),
		QoS:            packet.QoS1,
		Retain:         false,
		Priority:       packet.PriorityNormal,
		TraceID:        "trace-123",
		ContentType:    "text/plain",
		UserProperties: map[string]string{"key": "value"},
		ExpiryTime:     0,
		EnqueueTime:    time.Now().Unix(),
	}

	if err := store.Save(msg); err != nil {
		t.Fatalf("Failed to save message: %v", err)
	}

	// 测试获取消息
	retrieved, err := store.Get(1)
	if err != nil {
		t.Fatalf("Failed to get message: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Retrieved message is nil")
	}
	if retrieved.PacketID != 1 {
		t.Errorf("PacketID = %d, want 1", retrieved.PacketID)
	}
	if retrieved.Topic != "test/topic" {
		t.Errorf("Topic = %q, want %q", retrieved.Topic, "test/topic")
	}
	if string(retrieved.Payload) != "hello world" {
		t.Errorf("Payload = %q, want %q", string(retrieved.Payload), "hello world")
	}
	if retrieved.QoS != packet.QoS1 {
		t.Errorf("QoS = %v, want %v", retrieved.QoS, packet.QoS1)
	}
	if retrieved.TraceID != "trace-123" {
		t.Errorf("TraceID = %q, want %q", retrieved.TraceID, "trace-123")
	}
	if retrieved.UserProperties["key"] != "value" {
		t.Errorf("UserProperties[key] = %q, want %q", retrieved.UserProperties["key"], "value")
	}

	// 测试统计
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Failed to count messages: %v", err)
	}
	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
	}

	// 测试删除消息
	if err := store.Delete(1); err != nil {
		t.Fatalf("Failed to delete message: %v", err)
	}

	// 验证删除
	retrieved, err = store.Get(1)
	if err != nil {
		t.Fatalf("Error after delete: %v", err)
	}
	if retrieved != nil {
		t.Error("Message should be deleted")
	}

	count, err = store.Count()
	if err != nil {
		t.Fatalf("Failed to count after delete: %v", err)
	}
	if count != 0 {
		t.Errorf("Count after delete = %d, want 0", count)
	}
}

// TestMessageStoreGetAll 测试获取所有消息
func TestMessageStoreGetAll(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	cfg := StoreConfig{
		Enabled:    true,
		DataDir:    "",
		SyncWrites: false,
		GCInterval: 0,
		Logger:     logger,
	}

	store, err := NewMessageStore(cfg)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 保存多条消息
	for i := uint16(1); i <= 5; i++ {
		msg := &StoredMessage{
			PacketID:    i,
			Topic:       "test/topic",
			Payload:     []byte("payload"),
			QoS:         packet.QoS1,
			EnqueueTime: time.Now().Unix(),
		}
		if err := store.Save(msg); err != nil {
			t.Fatalf("Failed to save message %d: %v", i, err)
		}
	}

	// 获取所有消息
	messages, err := store.GetAll()
	if err != nil {
		t.Fatalf("Failed to get all messages: %v", err)
	}
	if len(messages) != 5 {
		t.Errorf("GetAll returned %d messages, want 5", len(messages))
	}
}

func TestMessageStorePacketIDSeed(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	cfg := StoreConfig{
		Enabled:    true,
		DataDir:    "",
		SyncWrites: false,
		GCInterval: 0,
		Logger:     logger,
	}

	store, err := NewMessageStore(cfg)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	seed0, err := store.PacketIDSeed()
	if err != nil {
		t.Fatalf("PacketIDSeed: %v", err)
	}
	if seed0 != uint16(minPacketID) {
		t.Fatalf("seed=%d, want %d", seed0, uint16(minPacketID))
	}

	if err := store.Save(&StoredMessage{PacketID: 10, Topic: "t", Payload: []byte("x"), QoS: packet.QoS1, EnqueueTime: time.Now().Unix()}); err != nil {
		t.Fatalf("save: %v", err)
	}
	seed1, err := store.PacketIDSeed()
	if err != nil {
		t.Fatalf("PacketIDSeed: %v", err)
	}
	if seed1 != 10 {
		t.Fatalf("seed=%d, want 10", seed1)
	}

	// Seed follows the latest stored/allocated PacketID (non-monotonic to support wrap).
	if err := store.Save(&StoredMessage{PacketID: 3, Topic: "t", Payload: []byte("x"), QoS: packet.QoS1, EnqueueTime: time.Now().Unix()}); err != nil {
		t.Fatalf("save: %v", err)
	}
	seed2, _ := store.PacketIDSeed()
	if seed2 != 3 {
		t.Fatalf("seed=%d, want 3", seed2)
	}

	if err := store.SetPacketIDSeed(1); err != nil {
		t.Fatalf("set seed: %v", err)
	}
	seed2b, _ := store.PacketIDSeed()
	if seed2b != 1 {
		t.Fatalf("seed=%d, want 1", seed2b)
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	seed3, err := store.PacketIDSeed()
	if err != nil {
		t.Fatalf("PacketIDSeed: %v", err)
	}
	if seed3 != uint16(minPacketID) {
		t.Fatalf("seed=%d, want %d", seed3, uint16(minPacketID))
	}
}

// TestMessageStoreGetBatch 测试批量获取消息
func TestMessageStoreGetBatch(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	cfg := StoreConfig{
		Enabled:    true,
		DataDir:    "",
		SyncWrites: false,
		GCInterval: 0,
		Logger:     logger,
	}

	store, err := NewMessageStore(cfg)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 保存 10 条消息
	for i := uint16(1); i <= 10; i++ {
		msg := &StoredMessage{
			PacketID:    i,
			Topic:       "test/topic",
			Payload:     []byte("payload"),
			QoS:         packet.QoS1,
			EnqueueTime: time.Now().Unix(),
		}
		if err := store.Save(msg); err != nil {
			t.Fatalf("Failed to save message %d: %v", i, err)
		}
	}

	// 测试批量获取（限制 5 条）
	messages, err := store.GetBatch(5, nil)
	if err != nil {
		t.Fatalf("Failed to get batch: %v", err)
	}
	if len(messages) != 5 {
		t.Errorf("GetBatch returned %d messages, want 5", len(messages))
	}

	// 测试排除某些 PacketID
	exclude := map[uint16]bool{1: true, 2: true, 3: true}
	messages, err = store.GetBatch(10, exclude)
	if err != nil {
		t.Fatalf("Failed to get batch with exclude: %v", err)
	}
	if len(messages) != 7 {
		t.Errorf("GetBatch with exclude returned %d messages, want 7", len(messages))
	}

	// 确保排除的 ID 不在结果中
	for _, msg := range messages {
		if exclude[msg.PacketID] {
			t.Errorf("Message with PacketID %d should be excluded", msg.PacketID)
		}
	}
}

// TestMessageStoreClear 测试清空存储
func TestMessageStoreClear(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	cfg := StoreConfig{
		Enabled:    true,
		DataDir:    "",
		SyncWrites: false,
		GCInterval: 0,
		Logger:     logger,
	}

	store, err := NewMessageStore(cfg)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 保存多条消息
	for i := uint16(1); i <= 5; i++ {
		msg := &StoredMessage{
			PacketID:    i,
			Topic:       "test/topic",
			Payload:     []byte("payload"),
			QoS:         packet.QoS1,
			EnqueueTime: time.Now().Unix(),
		}
		store.Save(msg)
	}

	// 清空
	if err := store.Clear(); err != nil {
		t.Fatalf("Failed to clear: %v", err)
	}

	// 验证
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Failed to count after clear: %v", err)
	}
	if count != 0 {
		t.Errorf("Count after clear = %d, want 0", count)
	}
}

// TestMessageStoreExpiry 测试过期消息
func TestMessageStoreExpiry(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	cfg := StoreConfig{
		Enabled:    true,
		DataDir:    "",
		SyncWrites: false,
		GCInterval: 0,
		Logger:     logger,
	}

	store, err := NewMessageStore(cfg)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 保存一条已过期的消息
	expiredMsg := &StoredMessage{
		PacketID:    1,
		Topic:       "test/expired",
		Payload:     []byte("expired"),
		QoS:         packet.QoS1,
		ExpiryTime:  time.Now().Unix() - 1, // 已过期
		EnqueueTime: time.Now().Unix(),
	}
	store.Save(expiredMsg)

	// 保存一条未过期的消息
	validMsg := &StoredMessage{
		PacketID:    2,
		Topic:       "test/valid",
		Payload:     []byte("valid"),
		QoS:         packet.QoS1,
		ExpiryTime:  time.Now().Unix() + 3600, // 1 小时后过期
		EnqueueTime: time.Now().Unix(),
	}
	store.Save(validMsg)

	// GetAll 应该只返回未过期的消息
	messages, err := store.GetAll()
	if err != nil {
		t.Fatalf("Failed to get all: %v", err)
	}
	if len(messages) != 1 {
		t.Errorf("GetAll returned %d messages, want 1 (should skip expired)", len(messages))
	}
	if len(messages) > 0 && messages[0].PacketID != 2 {
		t.Errorf("Expected valid message with PacketID=2, got %d", messages[0].PacketID)
	}

	// 测试清理过期消息
	cleaned, err := store.CleanupExpired()
	if err != nil {
		t.Fatalf("Failed to cleanup expired: %v", err)
	}
	if cleaned != 1 {
		t.Errorf("CleanupExpired cleaned %d messages, want 1", cleaned)
	}
}

// TestMessageStoreUpdateLastSentTime 测试更新最后发送时间
func TestMessageStoreUpdateLastSentTime(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	cfg := StoreConfig{
		Enabled:    true,
		DataDir:    "",
		SyncWrites: false,
		GCInterval: 0,
		Logger:     logger,
	}

	store, err := NewMessageStore(cfg)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	msg := &StoredMessage{
		PacketID:      1,
		Topic:         "test/topic",
		Payload:       []byte("payload"),
		QoS:           packet.QoS1,
		EnqueueTime:   time.Now().Unix(),
		LastSentTime:  0,
		DeliveryCount: 0,
	}
	store.Save(msg)

	// 更新最后发送时间
	if err := store.UpdateLastSentTime(1); err != nil {
		t.Fatalf("Failed to update last sent time: %v", err)
	}

	// 验证
	retrieved, _ := store.Get(1)
	if retrieved.LastSentTime == 0 {
		t.Error("LastSentTime should be updated")
	}
	if retrieved.DeliveryCount != 1 {
		t.Errorf("DeliveryCount = %d, want 1", retrieved.DeliveryCount)
	}

	// 再次更新
	store.UpdateLastSentTime(1)
	retrieved, _ = store.Get(1)
	if retrieved.DeliveryCount != 2 {
		t.Errorf("DeliveryCount after second update = %d, want 2", retrieved.DeliveryCount)
	}
}

// TestMessageStoreStats 测试存储统计
func TestMessageStoreStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	cfg := StoreConfig{
		Enabled:    true,
		DataDir:    "",
		SyncWrites: false,
		GCInterval: 0,
		Logger:     logger,
	}

	store, err := NewMessageStore(cfg)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 保存消息
	for i := uint16(1); i <= 3; i++ {
		msg := &StoredMessage{
			PacketID:    i,
			Topic:       "test/topic",
			Payload:     []byte("payload"),
			QoS:         packet.QoS1,
			EnqueueTime: time.Now().Unix(),
		}
		store.Save(msg)
	}

	stats, err := store.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	if stats.TotalMessages != 3 {
		t.Errorf("TotalMessages = %d, want 3", stats.TotalMessages)
	}
}

// TestStoredMessageEncodeDecode 测试消息编码解码
func TestStoredMessageEncodeDecode(t *testing.T) {
	original := &StoredMessage{
		PacketID:        12345,
		Topic:           "test/topic/深度/路径",
		Payload:         []byte("hello 世界"),
		QoS:             packet.QoS1,
		Retain:          true,
		Priority:        packet.PriorityHigh,
		TraceID:         "trace-abc-123",
		ContentType:     "application/json",
		UserProperties:  map[string]string{"key1": "value1", "key2": "value2"},
		ExpiryTime:      1234567890,
		EnqueueTime:     1234567800,
		LastSentTime:    1234567850,
		DeliveryCount:   3,
		TargetClientID:  "target-client",
		ResponseTopic:   "response/topic",
		CorrelationData: []byte("correlation-data"),
	}

	// 编码
	data, err := encodeStoredMessage(original)
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	// 解码
	decoded, err := decodeStoredMessage(data)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	// 验证所有字段
	if decoded.PacketID != original.PacketID {
		t.Errorf("PacketID = %d, want %d", decoded.PacketID, original.PacketID)
	}
	if decoded.Topic != original.Topic {
		t.Errorf("Topic = %q, want %q", decoded.Topic, original.Topic)
	}
	if string(decoded.Payload) != string(original.Payload) {
		t.Errorf("Payload = %q, want %q", string(decoded.Payload), string(original.Payload))
	}
	if decoded.QoS != original.QoS {
		t.Errorf("QoS = %v, want %v", decoded.QoS, original.QoS)
	}
	if decoded.Retain != original.Retain {
		t.Errorf("Retain = %v, want %v", decoded.Retain, original.Retain)
	}
	if decoded.Priority != original.Priority {
		t.Errorf("Priority = %v, want %v", decoded.Priority, original.Priority)
	}
	if decoded.TraceID != original.TraceID {
		t.Errorf("TraceID = %q, want %q", decoded.TraceID, original.TraceID)
	}
	if decoded.ContentType != original.ContentType {
		t.Errorf("ContentType = %q, want %q", decoded.ContentType, original.ContentType)
	}
	if decoded.ExpiryTime != original.ExpiryTime {
		t.Errorf("ExpiryTime = %d, want %d", decoded.ExpiryTime, original.ExpiryTime)
	}
	if decoded.EnqueueTime != original.EnqueueTime {
		t.Errorf("EnqueueTime = %d, want %d", decoded.EnqueueTime, original.EnqueueTime)
	}
	if decoded.LastSentTime != original.LastSentTime {
		t.Errorf("LastSentTime = %d, want %d", decoded.LastSentTime, original.LastSentTime)
	}
	if decoded.DeliveryCount != original.DeliveryCount {
		t.Errorf("DeliveryCount = %d, want %d", decoded.DeliveryCount, original.DeliveryCount)
	}
	if decoded.TargetClientID != original.TargetClientID {
		t.Errorf("TargetClientID = %q, want %q", decoded.TargetClientID, original.TargetClientID)
	}
	if decoded.ResponseTopic != original.ResponseTopic {
		t.Errorf("ResponseTopic = %q, want %q", decoded.ResponseTopic, original.ResponseTopic)
	}
	if string(decoded.CorrelationData) != string(original.CorrelationData) {
		t.Errorf("CorrelationData = %q, want %q", string(decoded.CorrelationData), string(original.CorrelationData))
	}
	if len(decoded.UserProperties) != len(original.UserProperties) {
		t.Errorf("UserProperties length = %d, want %d", len(decoded.UserProperties), len(original.UserProperties))
	}
	for k, v := range original.UserProperties {
		if decoded.UserProperties[k] != v {
			t.Errorf("UserProperties[%q] = %q, want %q", k, decoded.UserProperties[k], v)
		}
	}
}

// TestClientWithStore 测试带持久化存储的客户端
func TestClientWithStore(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	cfg := DefaultStoreConfig()
	cfg.DataDir = "" // 内存模式
	cfg.Logger = logger

	c, err := NewWithOptions(
		NewClientOptions().
			WithClientID("test-client").
			WithStore(&cfg).
			WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	if c.store == nil {
		t.Error("Client store should not be nil")
	}

	// 测试 GetMessageStore
	if c.GetMessageStore() == nil {
		t.Error("GetMessageStore() should return store")
	}

	// 关闭客户端
	c.Close()

	// 关闭后 store 应该为 nil
	if c.store != nil {
		t.Error("Store should be nil after Close()")
	}
}

// TestClientWithStoreOptions 测试便捷的存储配置选项
func TestClientWithStoreOptions(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	c, err := NewWithOptions(
		NewClientOptions().
			WithClientID("test-client").
			WithStoreDataDir(""). // 内存模式
			WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	if c.store == nil {
		t.Error("Client store should not be nil with WithStoreDataDir")
	}

	c.Close()
}

// TestClientWithRetransmitInterval 测试重传间隔配置
func TestClientWithRetransmitInterval(t *testing.T) {
	c, err := NewWithOptions(
		NewClientOptions().
			WithClientID("test-client").
			WithRetransmitInterval(10 * time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if c.options.RetransmitInterval != 10*time.Second {
		t.Errorf("RetransmitInterval = %v, want 10s", c.options.RetransmitInterval)
	}
}

// TestMessageStorePersistent 测试持久化模式
func TestMessageStorePersistent(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "queen-client-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := StoreConfig{
		Enabled:          true,
		DataDir:          tempDir,
		SyncWrites:       true,
		ValueLogFileSize: 64,
		GCInterval:       0,
		Logger:           logger,
	}

	// 第一次打开，保存数据
	store1, err := NewMessageStore(cfg)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	msg := &StoredMessage{
		PacketID:    1,
		Topic:       "test/persist",
		Payload:     []byte("persistent data"),
		QoS:         packet.QoS1,
		EnqueueTime: time.Now().Unix(),
	}
	if err := store1.Save(msg); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// 关闭
	store1.Close()

	// 第二次打开，验证数据持久化
	store2, err := NewMessageStore(cfg)
	if err != nil {
		t.Fatalf("Failed to reopen store: %v", err)
	}
	defer store2.Close()

	retrieved, err := store2.Get(1)
	if err != nil {
		t.Fatalf("Failed to get after reopen: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Message should persist after reopen")
	}
	if retrieved.Topic != "test/persist" {
		t.Errorf("Topic = %q, want %q", retrieved.Topic, "test/persist")
	}
	if string(retrieved.Payload) != "persistent data" {
		t.Errorf("Payload = %q, want %q", string(retrieved.Payload), "persistent data")
	}
}

// BenchmarkStoreSave 基准测试消息保存
func BenchmarkStoreSave(b *testing.B) {
	cfg := StoreConfig{
		Enabled:    true,
		DataDir:    "",
		SyncWrites: false,
		GCInterval: 0,
	}

	store, _ := NewMessageStore(cfg)
	defer store.Close()

	msg := &StoredMessage{
		PacketID:    1,
		Topic:       "test/benchmark",
		Payload:     []byte("benchmark payload"),
		QoS:         packet.QoS1,
		EnqueueTime: time.Now().Unix(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg.PacketID = uint16(i%65535 + 1)
		store.Save(msg)
	}
}

// BenchmarkStoreGet 基准测试消息获取
func BenchmarkStoreGet(b *testing.B) {
	cfg := StoreConfig{
		Enabled:    true,
		DataDir:    "",
		SyncWrites: false,
		GCInterval: 0,
	}

	store, _ := NewMessageStore(cfg)
	defer store.Close()

	// 预先保存一条消息
	msg := &StoredMessage{
		PacketID:    1,
		Topic:       "test/benchmark",
		Payload:     []byte("benchmark payload"),
		QoS:         packet.QoS1,
		EnqueueTime: time.Now().Unix(),
	}
	store.Save(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Get(1)
	}
}

// BenchmarkEncodeStoredMessage 基准测试消息编码
func BenchmarkEncodeStoredMessage(b *testing.B) {
	msg := &StoredMessage{
		PacketID:       12345,
		Topic:          "test/benchmark/topic",
		Payload:        []byte("benchmark payload data"),
		QoS:            packet.QoS1,
		Retain:         true,
		Priority:       packet.PriorityHigh,
		TraceID:        "trace-123",
		ContentType:    "application/json",
		UserProperties: map[string]string{"key": "value"},
		ExpiryTime:     1234567890,
		EnqueueTime:    1234567800,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encodeStoredMessage(msg)
	}
}

// BenchmarkDecodeStoredMessage 基准测试消息解码
func BenchmarkDecodeStoredMessage(b *testing.B) {
	msg := &StoredMessage{
		PacketID:       12345,
		Topic:          "test/benchmark/topic",
		Payload:        []byte("benchmark payload data"),
		QoS:            packet.QoS1,
		Retain:         true,
		Priority:       packet.PriorityHigh,
		TraceID:        "trace-123",
		ContentType:    "application/json",
		UserProperties: map[string]string{"key": "value"},
		ExpiryTime:     1234567890,
		EnqueueTime:    1234567800,
	}

	data, _ := encodeStoredMessage(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decodeStoredMessage(data)
	}
}
