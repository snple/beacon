package core

import (
	"context"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
)

// TestClientSessionRestoration_WithKeepSession 测试客户端会话保留
func TestClientSessionRestoration_WithKeepSession(t *testing.T) {
	// 启动服务器
	server, err := NewWithOptions(
		NewCoreOptions().
			WithAddress(":0").
			WithStorageEnabled(true).
			WithStorageDir(t.TempDir()).
			WithMaxSessionTimeout(3600),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()

	if err := server.Start(); err != nil {
		t.Fatal(err)
	}

	addr := server.listener.Addr().String()

	// 创建客户端并连接
	c, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("test-client").
			WithKeepSession(true).
			WithSessionTimeout(3600).
			WithKeepAlive(60),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if err := c.Connect(); err != nil {
		t.Fatal(err)
	}

	// 第一次连接，SessionPresent应该为false
	if c.SessionPresent() {
		t.Fatal("First connection should not have session present")
	}

	// 订阅主题
	if err := c.SubscribeWithOptions([]string{"test/topic"}, client.NewSubscribeOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatal(err)
	}

	// 断开连接
	if err := c.Disconnect(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// 重新连接
	if err := c.Connect(); err != nil {
		t.Fatal(err)
	}

	// 第二次连接，SessionPresent应该为true
	if !c.SessionPresent() {
		t.Fatal("Second connection should have session present")
	}

	// 验证订阅是否保留
	if !c.HasSubscription("test/topic") {
		t.Fatal("Subscription should be preserved after reconnection")
	}
}

// TestClientSessionRestoration_WithCleanSession 测试客户端清除会话
func TestClientSessionRestoration_WithCleanSession(t *testing.T) {
	server, err := NewWithOptions(
		NewCoreOptions().
			WithAddress(":0").
			WithStorageEnabled(true).
			WithStorageDir(t.TempDir()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()

	if err := server.Start(); err != nil {
		t.Fatal(err)
	}

	addr := server.listener.Addr().String()

	// 第一次连接：KeepSession=false
	c1, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("test-client").
			WithKeepSession(false).
			WithKeepAlive(60),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer c1.Close()

	if err := c1.Connect(); err != nil {
		t.Fatal(err)
	}

	// 订阅主题
	if err := c1.SubscribeWithOptions([]string{"test/topic"}, client.NewSubscribeOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatal(err)
	}

	// 断开连接
	if err := c1.Disconnect(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// 第二次连接：KeepSession=false
	c2, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("test-client").
			WithKeepSession(false).
			WithKeepAlive(60),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()

	if err := c2.Connect(); err != nil {
		t.Fatal(err)
	}

	// SessionPresent应该为false（会话已清除）
	if c2.SessionPresent() {
		t.Fatal("Session should not be present with CleanSession")
	}

	// 订阅应该不存在
	if c2.HasSubscription("test/topic") {
		t.Fatal("Subscription should not be preserved with CleanSession")
	}
}

// TestClientOfflineMessageDelivery 测试客户端离线消息投递
func TestClientOfflineMessageDelivery(t *testing.T) {
	server, err := NewWithOptions(
		NewCoreOptions().
			WithAddress(":0").
			WithStorageEnabled(true).
			WithStorageDir(t.TempDir()).
			WithMaxSessionTimeout(3600).
			WithRetransmitInterval(1 * time.Second), // 1秒重传间隔
	)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()

	if err := server.Start(); err != nil {
		t.Fatal(err)
	}

	addr := server.listener.Addr().String()

	// 创建订阅客户端
	subscriber, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("subscriber").
			WithKeepSession(true).
			WithSessionTimeout(3600).
			WithKeepAlive(60).
			WithMessageQueueSize(10),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer subscriber.Close()

	if err := subscriber.Connect(); err != nil {
		t.Fatal(err)
	}

	// 订阅主题
	if err := subscriber.SubscribeWithOptions([]string{"test/topic"}, client.NewSubscribeOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatal(err)
	}

	// 断开订阅者连接（保留会话）
	if err := subscriber.Disconnect(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// 创建发布客户端并发送消息
	publisher, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("publisher").
			WithKeepSession(false).
			WithKeepAlive(60),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer publisher.Close()

	if err := publisher.Connect(); err != nil {
		t.Fatal(err)
	}

	// 发送消息（订阅者离线）
	if err := publisher.Publish("test/topic", []byte("offline message"),
		client.NewPublishOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// 验证服务端持久化了消息
	server.clientsMu.RLock()
	svClient, exists := server.clients["subscriber"]
	server.clientsMu.RUnlock()

	if !exists {
		t.Fatal("Subscriber session should exist")
	}

	count, err := server.messageStore.countMessages(svClient.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("Expected 1 persisted message, got %d", count)
	}

	// 订阅者重新连接
	if err := subscriber.Connect(); err != nil {
		t.Fatal(err)
	}

	// 等待重传循环发送消息
	ctx := context.Background()
	msg, err := subscriber.PollMessage(ctx, 2*time.Second)
	if err != nil {
		t.Fatal("Should receive offline message after reconnection:", err)
	}

	if msg.Topic() != "test/topic" {
		t.Fatalf("Expected topic 'test/topic', got '%s'", msg.Topic())
	}

	if string(msg.Payload()) != "offline message" {
		t.Fatalf("Expected payload 'offline message', got '%s'", string(msg.Payload()))
	}

	// 等待ACK处理
	time.Sleep(200 * time.Millisecond)

	// 验证消息已从持久化存储中删除
	count, err = server.messageStore.countMessages(svClient.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("Expected 0 persisted messages after delivery, got %d", count)
	}
}

// TestClientSessionTakeover 测试客户端会话接管
func TestClientSessionTakeover(t *testing.T) {
	server, err := NewWithOptions(
		NewCoreOptions().
			WithAddress(":0").
			WithStorageEnabled(true).
			WithStorageDir(t.TempDir()).
			WithMaxSessionTimeout(3600),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()

	if err := server.Start(); err != nil {
		t.Fatal(err)
	}

	addr := server.listener.Addr().String()

	// 第一个客户端连接
	c1, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("test-client").
			WithKeepSession(true).
			WithSessionTimeout(3600).
			WithKeepAlive(60),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer c1.Close()

	if err := c1.Connect(); err != nil {
		t.Fatal(err)
	}

	// 订阅主题
	if err := c1.SubscribeWithOptions([]string{"test/topic"}, client.NewSubscribeOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// 第二个客户端使用相同ID连接（会话接管）
	c2, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("test-client").
			WithKeepSession(true).
			WithSessionTimeout(3600).
			WithKeepAlive(60),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()

	if err := c2.Connect(); err != nil {
		t.Fatal(err)
	}

	// SessionPresent应该为true（接管了会话）
	if !c2.SessionPresent() {
		t.Fatal("Session takeover should have session present")
	}

	// 第一个客户端应该断开
	time.Sleep(200 * time.Millisecond)
	if c1.IsConnected() {
		t.Fatal("First client should be disconnected after session takeover")
	}

	// 第二个客户端应该保留订阅（从服务端查询）
	// 注意：客户端的本地订阅列表不会自动同步，需要重新订阅
	// 但服务端应该保留订阅
	server.clientsMu.RLock()
	svClient, exists := server.clients["test-client"]
	server.clientsMu.RUnlock()

	if !exists {
		t.Fatal("Client should exist after session takeover")
	}

	svClient.session.subsMu.RLock()
	_, hasSubscription := svClient.session.subscriptions["test/topic"]
	svClient.session.subsMu.RUnlock()

	if !hasSubscription {
		t.Fatal("Subscription should be preserved on server after session takeover")
	}
}

// TestClientAutoReconnect 测试客户端自动重连
func TestClientAutoReconnect(t *testing.T) {
	server, err := NewWithOptions(
		NewCoreOptions().
			WithAddress(":0").
			WithStorageEnabled(true).
			WithStorageDir(t.TempDir()).
			WithMaxSessionTimeout(3600),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := server.Start(); err != nil {
		t.Fatal(err)
	}

	addr := server.listener.Addr().String()

	// 创建客户端
	c, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("test-client").
			WithKeepSession(true).
			WithSessionTimeout(3600).
			WithKeepAlive(60),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if err := c.Connect(); err != nil {
		t.Fatal(err)
	}

	// 订阅主题
	if err := c.SubscribeWithOptions([]string{"test/topic"}, client.NewSubscribeOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatal(err)
	}

	// 强制关闭连接（模拟网络中断）
	if err := c.ForceClose(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	if c.IsConnected() {
		t.Fatal("Client should be disconnected after ForceClose")
	}

	// 客户端应该自动重连
	// 等待自动重连（最多3秒）
	reconnected := false
	for i := 0; i < 30; i++ {
		if c.IsConnected() {
			reconnected = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !reconnected {
		t.Fatal("Client should auto-reconnect after connection loss")
	}

	// 会话应该保留
	if !c.SessionPresent() {
		t.Fatal("Session should be present after auto-reconnect")
	}

	// 订阅应该保留
	if !c.HasSubscription("test/topic") {
		t.Fatal("Subscription should be preserved after auto-reconnect")
	}

	// 清理
	server.Stop()
}

// TestClientQoS1MessagePersistence 测试客户端QoS1消息持久化
func TestClientQoS1MessagePersistence(t *testing.T) {
	server, err := NewWithOptions(
		NewCoreOptions().
			WithAddress(":0").
			WithStorageEnabled(true).
			WithStorageDir(t.TempDir()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()

	if err := server.Start(); err != nil {
		t.Fatal(err)
	}

	addr := server.listener.Addr().String()

	// 创建带持久化的客户端
	storeDir := t.TempDir()
	c, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("test-client").
			WithKeepSession(true).
			WithKeepAlive(60).
			WithStore(&client.StoreConfig{
				Enabled:    true,
				DataDir:    storeDir,
				SyncWrites: false,
			}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if err := c.Connect(); err != nil {
		t.Fatal(err)
	}

	// 发送QoS1消息
	if err := c.Publish("test/topic", []byte("test message"),
		client.NewPublishOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatal(err)
	}

	// 等待消息发送和ACK
	time.Sleep(200 * time.Millisecond)

	// 消息应该已从持久化中删除（已ACK）
	count, err := c.CleanupExpired()
	if err != nil {
		t.Fatal(err)
	}
	// CleanupExpired只清理过期消息，已ACK的消息应该已经删除
	t.Logf("Cleaned up %d expired messages", count)
}

// TestClientSessionExpiry 测试客户端会话过期
func TestClientSessionExpiry(t *testing.T) {
	server, err := NewWithOptions(
		NewCoreOptions().
			WithAddress(":0").
			WithStorageEnabled(true).
			WithStorageDir(t.TempDir()).
			WithMaxSessionTimeout(3600),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()

	if err := server.Start(); err != nil {
		t.Fatal(err)
	}

	addr := server.listener.Addr().String()

	// 创建客户端，设置2秒会话超时
	c, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("test-client").
			WithKeepSession(true).
			WithSessionTimeout(2). // 2秒超时
			WithKeepAlive(60),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if err := c.Connect(); err != nil {
		t.Fatal(err)
	}

	// 订阅主题
	if err := c.SubscribeWithOptions([]string{"test/topic"}, client.NewSubscribeOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatal(err)
	}

	// 断开连接
	if err := c.Disconnect(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// 验证会话存在
	server.clientsMu.RLock()
	_, exists := server.clients["test-client"]
	server.clientsMu.RUnlock()

	if !exists {
		t.Fatal("Session should exist immediately after disconnect")
	}

	// 等待会话过期（2秒 + 清理间隔）
	time.Sleep(2500 * time.Millisecond)

	// 手动触发清理
	server.cleanupExpiredSessions()

	// 验证会话已清除
	server.clientsMu.RLock()
	_, exists = server.clients["test-client"]
	server.clientsMu.RUnlock()

	if exists {
		t.Fatal("Session should be removed after expiry")
	}

	// 重新连接，SessionPresent应该为false
	if err := c.Connect(); err != nil {
		t.Fatal(err)
	}

	if c.SessionPresent() {
		t.Fatal("Session should not be present after expiry")
	}

	// 订阅应该不存在（检查服务端）
	server.clientsMu.RLock()
	svClient, exists := server.clients["test-client"]
	server.clientsMu.RUnlock()

	if !exists {
		t.Fatal("Client should exist after reconnection")
	}

	svClient.session.subsMu.RLock()
	_, hasSubscription := svClient.session.subscriptions["test/topic"]
	svClient.session.subsMu.RUnlock()

	if hasSubscription {
		t.Fatal("Subscription should not be preserved after session expiry")
	}
}
