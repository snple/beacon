package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
)

// ============================================================================
// PublishToCore 测试 - TargetClientID == "core"
// ============================================================================

// TestPublishToCore_BasicDelivery 测试基本的 PublishToCore 功能
func TestPublishToCore_BasicDelivery(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建客户端
	c := setupTestClient(t, addr, "test-client", nil)
	defer c.Close()

	// 启动 PollMessage goroutine
	pollCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	msgReceived := make(chan *Message, 1)
	go func() {
		msg, err := core.PollMessage(pollCtx, 5*time.Second)
		if err == nil {
			msgReceived <- msg
		}
	}()

	// 等待 PollMessage 初始化队列
	time.Sleep(100 * time.Millisecond)

	// 发布消息到 core (TargetClientID="core")
	err := c.PublishToCore("test/topic", []byte("test message to core"), nil)
	if err != nil {
		t.Fatalf("Failed to publish to core: %v", err)
	}

	// 验证消息被 PollMessage 接收
	select {
	case msg := <-msgReceived:
		if msg.Topic != "test/topic" {
			t.Errorf("Expected topic 'test/topic', got '%s'", msg.Topic)
		}
		if string(msg.Payload) != "test message to core" {
			t.Errorf("Expected payload 'test message to core', got '%s'", string(msg.Payload))
		}
		if msg.TargetClientID != "core" {
			t.Errorf("Expected TargetClientID 'core', got '%s'", msg.TargetClientID)
		}
		if msg.SourceClientID != "test-client" {
			t.Errorf("Expected SourceClientID 'test-client', got '%s'", msg.SourceClientID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

// TestPublishToCore_NotForwardedToOtherClients 测试发送到 core 的消息不会转发给其他客户端
func TestPublishToCore_NotForwardedToOtherClients(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建发布者客户端
	publisher := setupTestClient(t, addr, "publisher", nil)
	defer publisher.Close()

	// 创建订阅者客户端
	subscriber := setupTestClient(t, addr, "subscriber", nil)
	defer subscriber.Close()

	// 订阅者订阅主题
	err := subscriber.Subscribe("test/topic")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// 启动订阅者的消息接收
	received := make(chan *client.Message, 10)
	go func() {
		for {
			msg, err := subscriber.PollMessage(context.Background(), 5*time.Second)
			if err != nil {
				return
			}
			received <- msg
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// 启动 PollMessage
	pollCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pollReceived := make(chan *Message, 1)
	go func() {
		msg, err := core.PollMessage(pollCtx, 5*time.Second)
		if err == nil {
			pollReceived <- msg
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// 发布消息到 core
	err = publisher.PublishToCore("test/topic", []byte("message to core only"), nil)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// 验证 PollMessage 收到消息
	select {
	case msg := <-pollReceived:
		if string(msg.Payload) != "message to core only" {
			t.Errorf("PollMessage: Expected payload 'message to core only', got '%s'", string(msg.Payload))
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("PollMessage: Timeout waiting for message")
	}

	// 验证订阅者没有收到消息
	select {
	case msg := <-received:
		t.Errorf("Subscriber should not receive message, but got: %s", string(msg.Payload))
	case <-time.After(500 * time.Millisecond):
		// 正确：订阅者没有收到消息
	}
}

// TestPublishToCore_QoS1 测试 QoS1 消息发送到 core
func TestPublishToCore_QoS1(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	c := setupTestClient(t, addr, "qos1-client", nil)
	defer c.Close()

	// 启动 PollMessage
	pollCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	msgReceived := make(chan *Message, 1)
	go func() {
		msg, err := core.PollMessage(pollCtx, 5*time.Second)
		if err == nil {
			msgReceived <- msg
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// 发布 QoS1 消息到 core
	err := c.PublishToCore("test/qos1", []byte("qos1 message"),
		client.NewPublishOptions().WithQoS(packet.QoS1))
	if err != nil {
		t.Fatalf("Failed to publish QoS1 message: %v", err)
	}

	// 验证消息被接收
	select {
	case msg := <-msgReceived:
		if msg.QoS != packet.QoS1 {
			t.Errorf("Expected QoS1, got QoS%d", msg.QoS)
		}
		if string(msg.Payload) != "qos1 message" {
			t.Errorf("Expected payload 'qos1 message', got '%s'", string(msg.Payload))
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for QoS1 message")
	}
}

// TestPublishToCore_WithProperties 测试带属性的消息发送到 core
func TestPublishToCore_WithProperties(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	c := setupTestClient(t, addr, "props-client", nil)
	defer c.Close()

	// 启动 PollMessage
	pollCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	msgReceived := make(chan *Message, 1)
	go func() {
		msg, err := core.PollMessage(pollCtx, 5*time.Second)
		if err == nil {
			msgReceived <- msg
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// 发布带属性的消息
	err := c.PublishToCore("test/props", []byte("message with properties"),
		client.NewPublishOptions().
			WithTraceID("trace-123").
			WithContentType("application/json").
			WithUserProperty("key1", "value1").
			WithUserProperty("key2", "value2"))
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// 验证消息和属性
	select {
	case msg := <-msgReceived:
		if msg.TraceID != "trace-123" {
			t.Errorf("Expected TraceID 'trace-123', got '%s'", msg.TraceID)
		}
		if msg.ContentType != "application/json" {
			t.Errorf("Expected ContentType 'application/json', got '%s'", msg.ContentType)
		}
		if msg.UserProperties["key1"] != "value1" {
			t.Errorf("Expected UserProperties[key1]='value1', got '%s'", msg.UserProperties["key1"])
		}
		if msg.UserProperties["key2"] != "value2" {
			t.Errorf("Expected UserProperties[key2]='value2', got '%s'", msg.UserProperties["key2"])
		}
		if msg.SourceClientID != "props-client" {
			t.Errorf("Expected SourceClientID 'props-client', got '%s'", msg.SourceClientID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

// TestPublishToCore_MultipleMessages 测试连续发送多个消息到 core
func TestPublishToCore_MultipleMessages(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	c := setupTestClient(t, addr, "multi-client", nil)
	defer c.Close()

	const messageCount = 5
	receivedMessages := make([]*Message, 0, messageCount)
	var mu sync.Mutex

	// 启动多个 PollMessage goroutine
	pollCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < messageCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			msg, err := core.PollMessage(pollCtx, 10*time.Second)
			if err == nil {
				mu.Lock()
				receivedMessages = append(receivedMessages, msg)
				mu.Unlock()
			}
		}()
	}

	// 等待 PollMessage 初始化
	time.Sleep(200 * time.Millisecond)

	// 发布多个消息
	for i := 0; i < messageCount; i++ {
		payload := fmt.Sprintf("message-%d", i)
		err := c.PublishToCore("test/multi", []byte(payload), nil)
		if err != nil {
			t.Fatalf("Failed to publish message %d: %v", i, err)
		}
		time.Sleep(50 * time.Millisecond) // 避免消息太快
	}

	// 等待所有消息被接收
	wg.Wait()

	// 验证收到的消息数量
	mu.Lock()
	count := len(receivedMessages)
	mu.Unlock()

	if count != messageCount {
		t.Errorf("Expected %d messages, got %d", messageCount, count)
	}

	// 验证每个消息的内容
	mu.Lock()
	payloads := make(map[string]bool)
	for _, msg := range receivedMessages {
		if msg.Topic != "test/multi" {
			t.Errorf("Expected topic 'test/multi', got '%s'", msg.Topic)
		}
		if msg.TargetClientID != "core" {
			t.Errorf("Expected TargetClientID 'core', got '%s'", msg.TargetClientID)
		}
		payloads[string(msg.Payload)] = true
	}
	mu.Unlock()

	// 验证所有预期的消息都被接收
	for i := 0; i < messageCount; i++ {
		expected := fmt.Sprintf("message-%d", i)
		if !payloads[expected] {
			t.Errorf("Missing message with payload '%s'", expected)
		}
	}
}

// TestPublishToCore_QueueNotInitialized 测试当 PollMessage 未调用时消息被丢弃
func TestPublishToCore_QueueNotInitialized(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	c := setupTestClient(t, addr, "no-poll-client", nil)
	defer c.Close()

	// 不调用 PollMessage，直接发布消息
	err := c.PublishToCore("test/no-poll", []byte("should be dropped"), nil)

	// 发布应该成功（但消息会被丢弃，因为队列未初始化）
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// 等待一小段时间确保消息已经被处理（或丢弃）
	time.Sleep(100 * time.Millisecond)

	// 现在启动 PollMessage，应该收不到之前的消息（因为队列之前未初始化，消息被丢弃了）
	// 这个测试验证的是：在 PollMessage 调用前发送的消息会被丢弃
	pollCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	msg, err := core.PollMessage(pollCtx, 200*time.Millisecond)
	// 应该超时，因为之前的消息已被丢弃
	if err == nil && msg != nil {
		t.Errorf("Should not receive dropped message, but got: %s", string(msg.Payload))
	}
}
