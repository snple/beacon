package core

import (
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/packet"
)

// createMockConn 创建测试用的网络连接
func createMockConn(addr string, t *testing.T) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

// TestSessionRestoration_KeepSession 测试会话恢复功能（KeepSession=true）
func TestSessionRestoration_KeepSession(t *testing.T) {
	server, err := NewWithOptions(
		NewCoreOptions().
			WithAddress(":0").
			WithStorageEnabled(true).
			WithStorageDir(t.TempDir()).
			WithMaxSessionTimeout(3600), // 1小时
	)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()

	if err := server.Start(); err != nil {
		t.Fatal(err)
	}

	addr := server.listener.Addr().String()

	// 第一次连接：KeepSession=true
	conn1 := createMockConn(addr, t)
	defer conn1.Close()

	connect1 := packet.NewConnectPacket()
	connect1.ClientID = "test-client"
	connect1.KeepSession = true
	connect1.KeepAlive = 60

	if err := packet.WritePacket(conn1, connect1); err != nil {
		t.Fatal(err)
	}

	// 读取 CONNACK
	pkt, err := packet.ReadPacket(conn1)
	if err != nil {
		t.Fatal(err)
	}
	connack1, ok := pkt.(*packet.ConnackPacket)
	if !ok {
		t.Fatalf("Expected CONNACK, got %T", pkt)
	}
	if connack1.ReasonCode != packet.ReasonSuccess {
		t.Fatalf("Expected success, got %v", connack1.ReasonCode)
	}
	if connack1.SessionPresent {
		t.Fatal("First connection should not have session present")
	}

	// 订阅主题
	sub := packet.NewSubscribePacket(1)
	sub.Subscriptions = []packet.Subscription{
		{Topic: "test/topic", Options: packet.SubscribeOptions{QoS: packet.QoS1}},
	}
	if err := packet.WritePacket(conn1, sub); err != nil {
		t.Fatal(err)
	}

	// 读取 SUBACK
	pkt, err = packet.ReadPacket(conn1)
	if err != nil {
		t.Fatal(err)
	}
	suback, ok := pkt.(*packet.SubackPacket)
	if !ok {
		t.Fatalf("Expected SUBACK, got %T", pkt)
	}
	if len(suback.ReasonCodes) != 1 || suback.ReasonCodes[0] != packet.ReasonCode(packet.QoS1) {
		t.Fatalf("Unexpected SUBACK reason codes: %v", suback.ReasonCodes)
	}

	// 关闭第一个连接（模拟异常断开 - 不发送 DISCONNECT）
	conn1.Close()
	time.Sleep(100 * time.Millisecond) // 等待服务器处理断开

	// 检查会话是否保留
	server.clientsMu.RLock()
	client, exists := server.clients["test-client"]
	server.clientsMu.RUnlock()

	if !exists {
		t.Fatal("Client session should be preserved")
	}

	// 检查订阅是否保留
	client.session.subsMu.RLock()
	_, subExists := client.session.subscriptions["test/topic"]
	client.session.subsMu.RUnlock()

	if !subExists {
		t.Fatal("Subscription should be preserved")
	}

	// 第二次连接：恢复会话
	conn2 := createMockConn(addr, t)
	defer conn2.Close()

	connect2 := packet.NewConnectPacket()
	connect2.ClientID = "test-client"
	connect2.KeepSession = true
	connect2.KeepAlive = 60

	if err := packet.WritePacket(conn2, connect2); err != nil {
		t.Fatal(err)
	}

	// 读取 CONNACK
	pkt, err = packet.ReadPacket(conn2)
	if err != nil {
		t.Fatal(err)
	}
	connack2, ok := pkt.(*packet.ConnackPacket)
	if !ok {
		t.Fatalf("Expected CONNACK, got %T", pkt)
	}
	if connack2.ReasonCode != packet.ReasonSuccess {
		t.Fatalf("Expected success, got %v", connack2.ReasonCode)
	}
	if !connack2.SessionPresent {
		t.Fatal("Second connection should have session present")
	}

	// 验证是否复用了同一个 Client 对象
	server.clientsMu.RLock()
	client2, exists := server.clients["test-client"]
	server.clientsMu.RUnlock()

	if !exists {
		t.Fatal("Client should exist after reconnection")
	}

	// 检查是否是同一个 Client 对象（会话复用）
	if client != client2 {
		t.Fatal("Should reuse the same Client object for session restoration")
	}

	// 检查订阅是否仍然存在
	client2.session.subsMu.RLock()
	_, subExists = client2.session.subscriptions["test/topic"]
	client2.session.subsMu.RUnlock()

	if !subExists {
		t.Fatal("Subscription should still exist after session restoration")
	}
}

// TestSessionRestoration_CleanSession 测试清除会话功能（KeepSession=false）
func TestSessionRestoration_CleanSession(t *testing.T) {
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
	conn1 := createMockConn(addr, t)
	defer conn1.Close()

	connect1 := packet.NewConnectPacket()
	connect1.ClientID = "test-client"
	connect1.KeepSession = false
	connect1.KeepAlive = 60

	if err := packet.WritePacket(conn1, connect1); err != nil {
		t.Fatal(err)
	}

	// 读取 CONNACK
	pkt, err := packet.ReadPacket(conn1)
	if err != nil {
		t.Fatal(err)
	}
	connack1, ok := pkt.(*packet.ConnackPacket)
	if !ok {
		t.Fatalf("Expected CONNACK, got %T", pkt)
	}
	if connack1.ReasonCode != packet.ReasonSuccess {
		t.Fatalf("Expected success, got %v", connack1.ReasonCode)
	}

	// 订阅主题
	sub := packet.NewSubscribePacket(1)
	sub.Subscriptions = []packet.Subscription{
		{Topic: "test/topic", Options: packet.SubscribeOptions{QoS: packet.QoS1}},
	}
	if err := packet.WritePacket(conn1, sub); err != nil {
		t.Fatal(err)
	}

	// 读取 SUBACK
	pkt, err = packet.ReadPacket(conn1)
	if err != nil {
		t.Fatal(err)
	}

	// 关闭连接
	conn1.Close()
	time.Sleep(100 * time.Millisecond)

	// 检查会话是否被清除
	server.clientsMu.RLock()
	_, exists := server.clients["test-client"]
	server.clientsMu.RUnlock()

	if exists {
		t.Fatal("Client session should be cleared when KeepSession=false")
	}

	// 第二次连接：应该是新会话
	conn2 := createMockConn(addr, t)
	defer conn2.Close()

	connect2 := packet.NewConnectPacket()
	connect2.ClientID = "test-client"
	connect2.KeepSession = false
	connect2.KeepAlive = 60

	if err := packet.WritePacket(conn2, connect2); err != nil {
		t.Fatal(err)
	}

	// 读取 CONNACK
	pkt, err = packet.ReadPacket(conn2)
	if err != nil {
		t.Fatal(err)
	}
	connack2, ok := pkt.(*packet.ConnackPacket)
	if !ok {
		t.Fatalf("Expected CONNACK, got %T", pkt)
	}
	if connack2.SessionPresent {
		t.Fatal("Second connection should not have session present")
	}
}

// TestSessionTakeover 测试会话接管（同一客户端ID的新连接）
func TestSessionTakeover(t *testing.T) {
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

	// 第一次连接
	conn1 := createMockConn(addr, t)
	defer conn1.Close()

	connect1 := packet.NewConnectPacket()
	connect1.ClientID = "test-client"
	connect1.KeepSession = true
	connect1.KeepAlive = 60

	if err := packet.WritePacket(conn1, connect1); err != nil {
		t.Fatal(err)
	}

	// 读取 CONNACK
	pkt, err := packet.ReadPacket(conn1)
	if err != nil {
		t.Fatal(err)
	}
	connack1, ok := pkt.(*packet.ConnackPacket)
	if !ok {
		t.Fatalf("Expected CONNACK, got %T", pkt)
	}
	if connack1.ReasonCode != packet.ReasonSuccess {
		t.Fatalf("Expected success, got %v", connack1.ReasonCode)
	}

	// 获取第一个客户端对象
	server.clientsMu.RLock()
	client1, exists1 := server.clients["test-client"]
	server.clientsMu.RUnlock()

	if !exists1 {
		t.Fatal("Client should exist")
	}

	// 保存旧连接的引用（用于后续检查）
	client1.connMu.RLock()
	oldConn := client1.conn
	client1.connMu.RUnlock()

	// 第二次连接（同一个 ClientID，会话接管）
	conn2 := createMockConn(addr, t)
	defer conn2.Close()

	connect2 := packet.NewConnectPacket()
	connect2.ClientID = "test-client"
	connect2.KeepSession = true
	connect2.KeepAlive = 60

	if err := packet.WritePacket(conn2, connect2); err != nil {
		t.Fatal(err)
	}

	// 读取 CONNACK
	pkt, err = packet.ReadPacket(conn2)
	if err != nil {
		t.Fatal(err)
	}
	connack2, ok := pkt.(*packet.ConnackPacket)
	if !ok {
		t.Fatalf("Expected CONNACK, got %T", pkt)
	}
	if connack2.ReasonCode != packet.ReasonSuccess {
		t.Fatalf("Expected success, got %v", connack2.ReasonCode)
	}
	if !connack2.SessionPresent {
		t.Fatal("Session should be present on takeover")
	}

	// 第一个连接应该收到 DISCONNECT
	go func() {
		disconnectPkt, err := packet.ReadPacket(conn1)
		if err == nil {
			if disconnect, ok := disconnectPkt.(*packet.DisconnectPacket); ok {
				if disconnect.ReasonCode != packet.ReasonSessionTakenOver {
					t.Errorf("Expected SessionTakenOver, got %v", disconnect.ReasonCode)
				}
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// 验证第一个客户端的旧连接已关闭
	if oldConn != nil && !oldConn.closed.Load() {
		t.Fatal("First client's old connection should be closed after takeover")
	}

	// 验证仍然是同一个 Client 对象（会话复用）
	server.clientsMu.RLock()
	client2, exists2 := server.clients["test-client"]
	server.clientsMu.RUnlock()

	if !exists2 {
		t.Fatal("Client should still exist")
	}

	if client1 != client2 {
		t.Fatal("Should reuse the same Client object for session takeover")
	}

	// 验证新连接正常工作
	if client2.Closed() {
		t.Fatal("Second client connection should be open")
	}
}

// TestSessionExpiry 测试会话过期
func TestSessionExpiry(t *testing.T) {
	server, err := NewWithOptions(
		NewCoreOptions().
			WithAddress(":0").
			WithStorageEnabled(true).
			WithStorageDir(t.TempDir()).
			WithMaxSessionTimeout(2).                   // 2秒最大过期时间
			WithExpiredCheckInterval(10 * time.Second), // 10秒检查间隔（最小值）
	)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()

	if err := server.Start(); err != nil {
		t.Fatal(err)
	}

	addr := server.listener.Addr().String()

	// 连接并订阅
	conn1 := createMockConn(addr, t)
	defer conn1.Close()

	connect1 := packet.NewConnectPacket()
	connect1.ClientID = "test-client"
	connect1.KeepSession = true
	connect1.KeepAlive = 60
	connect1.Properties = packet.NewConnectProperties()
	connect1.Properties.SessionTimeout = 2 // 2秒后过期

	if err := packet.WritePacket(conn1, connect1); err != nil {
		t.Fatal(err)
	}

	// 读取 CONNACK
	pkt, err := packet.ReadPacket(conn1)
	if err != nil {
		t.Fatal(err)
	}
	connack1, ok := pkt.(*packet.ConnackPacket)
	if !ok {
		t.Fatalf("Expected CONNACK, got %T", pkt)
	}
	if connack1.ReasonCode != packet.ReasonSuccess {
		t.Fatalf("Expected success, got %v", connack1.ReasonCode)
	}

	// 订阅主题
	sub := packet.NewSubscribePacket(1)
	sub.Subscriptions = []packet.Subscription{
		{Topic: "test/topic", Options: packet.SubscribeOptions{QoS: packet.QoS1}},
	}
	if err := packet.WritePacket(conn1, sub); err != nil {
		t.Fatal(err)
	}

	// 读取 SUBACK
	pkt, err = packet.ReadPacket(conn1)
	if err != nil {
		t.Fatal(err)
	}

	// 关闭连接
	conn1.Close()
	time.Sleep(100 * time.Millisecond)

	// 验证会话仍然存在
	server.clientsMu.RLock()
	_, exists := server.clients["test-client"]
	server.clientsMu.RUnlock()

	if !exists {
		t.Fatal("Session should exist immediately after disconnect")
	}

	// 等待会话过期（2秒）
	time.Sleep(3 * time.Second)

	// 手动触发过期会话清理（因为自动检查间隔是 10 秒）
	server.cleanupExpiredSessions()

	// 验证会话已被清除
	server.clientsMu.RLock()
	_, exists = server.clients["test-client"]
	server.clientsMu.RUnlock()

	if exists {
		t.Fatal("Session should be expired and cleaned up")
	}
}

// TestOfflineMessageQueue 测试离线消息队列（会话保留时）
func TestOfflineMessageQueue(t *testing.T) {
	server, err := NewWithOptions(
		NewCoreOptions().
			WithAddress(":0").
			WithStorageEnabled(true).
			WithStorageDir(t.TempDir()).
			WithMaxSessionTimeout(3600).
			WithRetransmitInterval(1 * time.Second), // 使用1秒重传间隔以加速测试
	)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()

	if err := server.Start(); err != nil {
		t.Fatal(err)
	}

	addr := server.listener.Addr().String()

	// 第一次连接并订阅
	conn1 := createMockConn(addr, t)
	defer conn1.Close()

	connect1 := packet.NewConnectPacket()
	connect1.ClientID = "test-client"
	connect1.KeepSession = true
	connect1.KeepAlive = 60

	if err := packet.WritePacket(conn1, connect1); err != nil {
		t.Fatal(err)
	}

	// 读取 CONNACK
	pkt, err := packet.ReadPacket(conn1)
	if err != nil {
		t.Fatal(err)
	}

	// 订阅主题
	sub := packet.NewSubscribePacket(1)
	sub.Subscriptions = []packet.Subscription{
		{Topic: "test/topic", Options: packet.SubscribeOptions{QoS: packet.QoS1}},
	}
	if err := packet.WritePacket(conn1, sub); err != nil {
		t.Fatal(err)
	}

	// 读取 SUBACK
	pkt, err = packet.ReadPacket(conn1)
	if err != nil {
		t.Fatal(err)
	}

	// 关闭连接（客户端离线）
	conn1.Close()
	time.Sleep(100 * time.Millisecond)

	// 在客户端离线时发布消息
	testMsg := Message{
		QoS:       packet.QoS1,
		Retain:    false,
		Timestamp: time.Now().Unix(),
		Packet: &packet.PublishPacket{
			Topic:   "test/topic",
			Payload: []byte("offline message"),
			QoS:     packet.QoS1,
		},
	}
	server.publish(testMsg)

	time.Sleep(100 * time.Millisecond)

	// 验证消息已持久化
	server.clientsMu.RLock()
	client, exists := server.clients["test-client"]
	server.clientsMu.RUnlock()

	if !exists {
		t.Fatal("Client session should exist")
	}

	// 验证消息已持久化到 messageStore（客户端离线时不会进入 pendingAck）
	if server.messageStore == nil {
		t.Fatal("Message store should be enabled")
	}

	// 检查持久化的消息数量
	count, err := server.messageStore.countMessages(client.ID)
	if err != nil {
		t.Fatal("Failed to get messages:", err)
	}
	if count != 1 {
		t.Fatalf("Expected 1 persisted message, got %d", count)
	}

	// 重新连接
	conn2 := createMockConn(addr, t)
	defer conn2.Close()

	connect2 := packet.NewConnectPacket()
	connect2.ClientID = "test-client"
	connect2.KeepSession = true
	connect2.KeepAlive = 60

	if err := packet.WritePacket(conn2, connect2); err != nil {
		t.Fatal(err)
	}

	// 读取 CONNACK
	pkt, err = packet.ReadPacket(conn2)
	if err != nil {
		t.Fatal(err)
	}
	connack2, ok := pkt.(*packet.ConnackPacket)
	if !ok {
		t.Fatalf("Expected CONNACK, got %T", pkt)
	}
	if !connack2.SessionPresent {
		t.Fatal("Session should be present")
	}

	// 应该收到离线消息
	pkt, err = packet.ReadPacket(conn2)
	if err != nil {
		t.Fatal("Should receive offline message:", err)
	}

	pub, ok := pkt.(*packet.PublishPacket)
	if !ok {
		t.Fatalf("Expected PUBLISH, got %T", pkt)
	}

	if pub.Topic != "test/topic" {
		t.Fatalf("Expected topic 'test/topic', got '%s'", pub.Topic)
	}

	if string(pub.Payload) != "offline message" {
		t.Fatalf("Expected payload 'offline message', got '%s'", string(pub.Payload))
	}

	// 发送 PUBACK
	puback := packet.NewPubackPacket(pub.PacketID, packet.ReasonSuccess)
	if err := packet.WritePacket(conn2, puback); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// 验证消息已从 pendingAck 中移除
	client.session.pendingAckMu.Lock()
	pendingCount := len(client.session.pendingAck)
	client.session.pendingAckMu.Unlock()

	if pendingCount != 0 {
		t.Fatalf("Expected 0 pending messages after ACK, got %d", pendingCount)
	}
}

// TestWillMessage 测试遗嘱消息
func TestWillMessage(t *testing.T) {
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

	// 第一个客户端：订阅遗嘱主题
	conn1 := createMockConn(addr, t)
	defer conn1.Close()

	connect1 := packet.NewConnectPacket()
	connect1.ClientID = "subscriber"
	connect1.KeepSession = false
	connect1.KeepAlive = 60

	if err := packet.WritePacket(conn1, connect1); err != nil {
		t.Fatal(err)
	}

	// 读取 CONNACK
	pkt, err := packet.ReadPacket(conn1)
	if err != nil {
		t.Fatal(err)
	}

	// 订阅遗嘱主题
	sub := packet.NewSubscribePacket(1)
	sub.Subscriptions = []packet.Subscription{
		{Topic: "will/topic", Options: packet.SubscribeOptions{QoS: packet.QoS0}},
	}
	if err := packet.WritePacket(conn1, sub); err != nil {
		t.Fatal(err)
	}

	// 读取 SUBACK
	pkt, err = packet.ReadPacket(conn1)
	if err != nil {
		t.Fatal(err)
	}

	// 第二个客户端：设置遗嘱消息
	conn2 := createMockConn(addr, t)

	connect2 := packet.NewConnectPacket()
	connect2.ClientID = "will-client"
	connect2.KeepSession = false
	connect2.KeepAlive = 60
	connect2.Will = true
	connect2.WillPacket = &packet.PublishPacket{
		Topic:   "will/topic",
		Payload: []byte("client disconnected unexpectedly"),
		QoS:     packet.QoS0,
	}

	if err := packet.WritePacket(conn2, connect2); err != nil {
		t.Fatal(err)
	}

	// 读取 CONNACK
	pkt, err = packet.ReadPacket(conn2)
	if err != nil {
		t.Fatal(err)
	}

	// 异常断开连接（不发送 DISCONNECT）
	conn2.Close()
	time.Sleep(200 * time.Millisecond)

	// 订阅者应该收到遗嘱消息
	pkt, err = packet.ReadPacket(conn1)
	if err != nil {
		t.Fatal("Should receive will message:", err)
	}

	pub, ok := pkt.(*packet.PublishPacket)
	if !ok {
		t.Fatalf("Expected PUBLISH, got %T", pkt)
	}

	if pub.Topic != "will/topic" {
		t.Fatalf("Expected topic 'will/topic', got '%s'", pub.Topic)
	}

	if string(pub.Payload) != "client disconnected unexpectedly" {
		t.Fatalf("Expected will payload, got '%s'", string(pub.Payload))
	}
}

// TestWillMessage_NormalDisconnect 测试正常断开时不发送遗嘱消息
func TestWillMessage_NormalDisconnect(t *testing.T) {
	server, err := NewWithOptions(
		NewCoreOptions().
			WithAddress(":0"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()

	if err := server.Start(); err != nil {
		t.Fatal(err)
	}

	addr := server.listener.Addr().String()

	// 订阅者
	conn1 := createMockConn(addr, t)
	defer conn1.Close()

	connect1 := packet.NewConnectPacket()
	connect1.ClientID = "subscriber"
	connect1.KeepSession = false
	connect1.KeepAlive = 60

	if err := packet.WritePacket(conn1, connect1); err != nil {
		t.Fatal(err)
	}

	_, _ = packet.ReadPacket(conn1)

	sub := packet.NewSubscribePacket(1)
	sub.Subscriptions = []packet.Subscription{
		{Topic: "will/topic", Options: packet.SubscribeOptions{QoS: packet.QoS0}},
	}
	if err := packet.WritePacket(conn1, sub); err != nil {
		t.Fatal(err)
	}

	_, _ = packet.ReadPacket(conn1)

	// 带遗嘱的客户端
	conn2 := createMockConn(addr, t)
	defer conn2.Close()

	connect2 := packet.NewConnectPacket()
	connect2.ClientID = "will-client"
	connect2.KeepSession = false
	connect2.KeepAlive = 60
	connect2.Will = true
	connect2.WillPacket = &packet.PublishPacket{
		Topic:   "will/topic",
		Payload: []byte("should not be sent"),
		QoS:     packet.QoS0,
	}

	if err := packet.WritePacket(conn2, connect2); err != nil {
		t.Fatal(err)
	}

	_, _ = packet.ReadPacket(conn2)

	// 正常断开（发送 DISCONNECT）
	disconnect := packet.NewDisconnectPacket(packet.ReasonNormalDisconnect)
	if err := packet.WritePacket(conn2, disconnect); err != nil {
		t.Fatal(err)
	}

	conn2.Close()
	time.Sleep(200 * time.Millisecond)

	// 订阅者不应该收到遗嘱消息
	// 设置读取超时
	conn1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, err = packet.ReadPacket(conn1)
	if err == nil {
		t.Fatal("Should not receive will message on normal disconnect")
	}
}
