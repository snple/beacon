package core

import (
	"os"
	"testing"
	"time"

	"github.com/snple/beacon/packet"
)

// TestStorageInMemoryMode 测试 InMemory 模式
func TestStorageInMemoryMode(t *testing.T) {
	config := StorageConfig{
		DataDir:          "", // 空字符串启用 InMemory 模式
		Enabled:          true,
		SyncWrites:       false,
		ValueLogFileSize: 1024,
		GCInterval:       1 * time.Minute,
	}

	// 验证配置
	err := config.Validate()
	if err != nil {
		t.Fatalf("InMemory config should be valid: %v", err)
	}

	// 创建存储
	store, err := NewMessageStore(config)
	if err != nil {
		t.Fatalf("Failed to create InMemory store: %v", err)
	}
	defer store.Close()

	// 测试保存和获取消息
	msg := &Message{
		Topic:      "test/topic",
		Payload:    []byte("hello inmemory"),
		QoS:        packet.QoS1,
		PacketID:   1,
		Timestamp:  time.Now().Unix(),
		ExpiryTime: time.Now().Add(1 * time.Hour).Unix(),
	}

	err = store.Save("test-client", msg)
	if err != nil {
		t.Fatalf("Failed to save message: %v", err)
	}

	// 获取消息
	messages, err := store.GetPendingMessages("test-client")
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}

	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	if string(messages[0].Payload) != "hello inmemory" {
		t.Errorf("Expected payload 'hello inmemory', got '%s'", string(messages[0].Payload))
	}

	// 删除消息
	err = store.Delete("test-client", 1)
	if err != nil {
		t.Fatalf("Failed to delete message: %v", err)
	}

	// 验证删除
	messages, err = store.GetPendingMessages("test-client")
	if err != nil {
		t.Fatalf("Failed to get messages after delete: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("Expected 0 messages after delete, got %d", len(messages))
	}
}

// TestStorageWithTempDir 测试使用临时目录
func TestStorageWithTempDir(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "queen-storage-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := StorageConfig{
		DataDir:          tmpDir,
		Enabled:          true,
		SyncWrites:       false,
		ValueLogFileSize: 1024,
		GCInterval:       0, // 禁用 GC 以加快测试
	}

	// 创建存储
	store, err := NewMessageStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// 保存消息
	msg := &Message{
		Topic:     "test/persistent",
		Payload:   []byte("persistent data"),
		QoS:       packet.QoS1,
		PacketID:  1,
		Timestamp: time.Now().Unix(),
	}

	err = store.Save("test-client", msg)
	if err != nil {
		t.Fatalf("Failed to save message: %v", err)
	}

	// 关闭存储
	store.Close()

	// 重新打开存储
	store2, err := NewMessageStore(config)
	if err != nil {
		t.Fatalf("Failed to reopen store: %v", err)
	}
	defer store2.Close()

	// 验证数据持久化
	messages, err := store2.GetPendingMessages("test-client")
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}

	if len(messages) != 1 {
		t.Errorf("Expected 1 message after reopen, got %d", len(messages))
	}

	if string(messages[0].Payload) != "persistent data" {
		t.Errorf("Expected payload 'persistent data', got '%s'", string(messages[0].Payload))
	}
}

// TestStorageRetainMessages 测试保留消息
func TestStorageRetainMessages(t *testing.T) {
	config := StorageConfig{
		DataDir:          "", // InMemory 模式
		Enabled:          true,
		ValueLogFileSize: 1024,
		GCInterval:       0,
	}

	store, err := NewMessageStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 保存保留消息
	msg := &Message{
		Topic:     "sensor/temperature",
		Payload:   []byte("25.5"),
		QoS:       packet.QoS0,
		Retain:    true,
		Timestamp: time.Now().Unix(),
	}

	err = store.SaveRetainMessage(msg.Topic, msg)
	if err != nil {
		t.Fatalf("Failed to save retain message: %v", err)
	}

	// 获取保留消息
	retainMsgs, err := store.GetAllRetainMessages()
	if err != nil {
		t.Fatalf("Failed to get retain messages: %v", err)
	}

	if len(retainMsgs) != 1 {
		t.Errorf("Expected 1 retain message, got %d", len(retainMsgs))
	}

	if string(retainMsgs[0].Payload) != "25.5" {
		t.Errorf("Expected payload '25.5', got '%s'", string(retainMsgs[0].Payload))
	}

	// 删除保留消息（通过发送空 payload）
	err = store.SaveRetainMessage(msg.Topic, &Message{Topic: msg.Topic, Payload: []byte{}})
	if err != nil {
		t.Fatalf("Failed to delete retain message: %v", err)
	}

	// 验证删除
	retainMsgs, err = store.GetAllRetainMessages()
	if err != nil {
		t.Fatalf("Failed to get retain messages after delete: %v", err)
	}

	if len(retainMsgs) != 0 {
		t.Errorf("Expected 0 retain messages after delete, got %d", len(retainMsgs))
	}
}

// TestStorageExpiredMessages 测试过期消息清理
func TestStorageExpiredMessages(t *testing.T) {
	config := StorageConfig{
		DataDir:          "", // InMemory 模式
		Enabled:          true,
		ValueLogFileSize: 1024,
		GCInterval:       0,
	}

	store, err := NewMessageStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 保存已过期的消息
	expiredMsg := &Message{
		Topic:      "test/expired",
		Payload:    []byte("expired"),
		QoS:        packet.QoS1,
		PacketID:   1,
		Timestamp:  time.Now().Unix(),
		ExpiryTime: time.Now().Add(-1 * time.Hour).Unix(), // 已过期
	}

	err = store.Save("test-client", expiredMsg)
	if err != nil {
		t.Fatalf("Failed to save expired message: %v", err)
	}

	// 保存未过期的消息
	validMsg := &Message{
		Topic:      "test/valid",
		Payload:    []byte("valid"),
		QoS:        packet.QoS1,
		PacketID:   2,
		Timestamp:  time.Now().Unix(),
		ExpiryTime: time.Now().Add(1 * time.Hour).Unix(),
	}

	err = store.Save("test-client", validMsg)
	if err != nil {
		t.Fatalf("Failed to save valid message: %v", err)
	}

	// GetPendingMessages 应该过滤过期消息
	messages, err := store.GetPendingMessages("test-client")
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}

	if len(messages) != 1 {
		t.Errorf("Expected 1 valid message (expired filtered), got %d", len(messages))
	}

	if string(messages[0].Payload) != "valid" {
		t.Errorf("Expected payload 'valid', got '%s'", string(messages[0].Payload))
	}

	// 手动清理过期消息
	count, err := store.CleanupExpired()
	if err != nil {
		t.Fatalf("Failed to cleanup expired messages: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected to cleanup 1 expired message, got %d", count)
	}
}

// TestStorageStats 测试存储统计
func TestStorageStats(t *testing.T) {
	config := StorageConfig{
		DataDir:          "", // InMemory 模式
		Enabled:          true,
		ValueLogFileSize: 1024,
		GCInterval:       0,
	}

	store, err := NewMessageStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 保存一些消息
	for i := range 5 {
		msg := &Message{
			Topic:     "test/stats",
			Payload:   []byte("data"),
			QoS:       packet.QoS1,
			PacketID:  uint16(i + 1),
			Timestamp: time.Now().Unix(),
		}
		store.Save("test-client", msg)
	}

	// 获取统计信息
	stats, err := store.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.TotalMessages != 5 {
		t.Errorf("Expected 5 total messages, got %d", stats.TotalMessages)
	}
}

// TestStorageDeleteAllForClient 测试删除客户端所有消息
func TestStorageDeleteAllForClient(t *testing.T) {
	config := StorageConfig{
		DataDir:          "", // InMemory 模式
		Enabled:          true,
		ValueLogFileSize: 1024,
		GCInterval:       0,
	}

	store, err := NewMessageStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 为两个客户端保存消息
	for i := 0; i < 3; i++ {
		msg := &Message{
			Topic:     "test/client1",
			Payload:   []byte("client1-data"),
			QoS:       packet.QoS1,
			PacketID:  uint16(i + 1),
			Timestamp: time.Now().Unix(),
		}
		store.Save("client1", msg)
	}

	for i := 0; i < 2; i++ {
		msg := &Message{
			Topic:     "test/client2",
			Payload:   []byte("client2-data"),
			QoS:       packet.QoS1,
			PacketID:  uint16(i + 1),
			Timestamp: time.Now().Unix(),
		}
		store.Save("client2", msg)
	}

	// 删除 client1 的所有消息
	err = store.DeleteAllForClient("client1")
	if err != nil {
		t.Fatalf("Failed to delete all for client1: %v", err)
	}

	// 验证 client1 没有消息
	msgs1, err := store.GetPendingMessages("client1")
	if err != nil {
		t.Fatalf("Failed to get messages for client1: %v", err)
	}
	if len(msgs1) != 0 {
		t.Errorf("Expected 0 messages for client1, got %d", len(msgs1))
	}

	// 验证 client2 仍有消息
	msgs2, err := store.GetPendingMessages("client2")
	if err != nil {
		t.Fatalf("Failed to get messages for client2: %v", err)
	}
	if len(msgs2) != 2 {
		t.Errorf("Expected 2 messages for client2, got %d", len(msgs2))
	}
}
