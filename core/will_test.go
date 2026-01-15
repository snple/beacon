package core

import (
	"context"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
)

// ============================================================================
// 遗嘱消息（Will）测试
// ============================================================================

// TestWillMessage_BasicDelivery 测试基本的遗嘱消息发送
func TestWillMessage_BasicDelivery(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建订阅者客户端
	subscriber := setupTestClient(t, addr, "subscriber", nil)
	defer subscriber.Close()

	// 订阅遗嘱主题
	err := subscriber.Subscribe("will/topic")
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

	// 创建带遗嘱消息的客户端
	willOpts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("will-client").
		WithCleanSession(true).
		WithWill(&client.WillMessage{
			Topic:   "will/topic",
			Payload: []byte("client disconnected unexpectedly"),
			QoS:     packet.QoS0,
			Retain:  false,
		})

	willClient, err := client.NewWithOptions(willOpts)
	if err != nil {
		t.Fatalf("Failed to create will client: %v", err)
	}

	err = willClient.Connect()
	if err != nil {
		t.Fatalf("Failed to connect will client: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 模拟异常断开：强制关闭连接（不发送 DISCONNECT 包）
	// 这会触发遗嘱消息的发布
	willClient.ForceClose()

	// 等待并验证遗嘱消息
	select {
	case msg := <-received:
		if msg.Topic != "will/topic" {
			t.Errorf("Expected topic 'will/topic', got '%s'", msg.Topic)
		}
		if string(msg.Payload) != "client disconnected unexpectedly" {
			t.Errorf("Expected payload 'client disconnected unexpectedly', got '%s'", string(msg.Payload))
		}
		if msg.SourceClientID != "will-client" {
			t.Errorf("Expected SourceClientID 'will-client', got '%s'", msg.SourceClientID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for will message")
	}
}

// TestWillMessage_NormalDisconnect 测试正常断开时不发送遗嘱消息
func TestWillMessage_NormalDisconnect(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建订阅者客户端
	subscriber := setupTestClient(t, addr, "subscriber", nil)
	defer subscriber.Close()

	// 订阅遗嘱主题
	err := subscriber.Subscribe("will/normal")
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

	// 创建带遗嘱消息的客户端
	willOpts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("normal-client").
		WithCleanSession(true).
		WithWill(&client.WillMessage{
			Topic:   "will/normal",
			Payload: []byte("should not be sent"),
			QoS:     packet.QoS0,
			Retain:  false,
		})

	willClient, err := client.NewWithOptions(willOpts)
	if err != nil {
		t.Fatalf("Failed to create will client: %v", err)
	}

	err = willClient.Connect()
	if err != nil {
		t.Fatalf("Failed to connect will client: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 正常断开：发送 DISCONNECT 包
	err = willClient.Disconnect()
	if err != nil {
		t.Fatalf("Failed to disconnect: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// 验证没有收到遗嘱消息
	select {
	case msg := <-received:
		t.Errorf("Should not receive will message on normal disconnect, but got: %s", string(msg.Payload))
	case <-time.After(500 * time.Millisecond):
		// 正确：没有收到遗嘱消息
	}
}

// TestWillMessage_WithQoS1 测试 QoS1 遗嘱消息
func TestWillMessage_WithQoS1(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建订阅者客户端
	subscriber := setupTestClient(t, addr, "qos1-subscriber", nil)
	defer subscriber.Close()

	// 订阅遗嘱主题（使用 QoS1）
	err := subscriber.SubscribeWithOptions([]string{"will/qos1"},
		client.NewSubscribeOptions().WithQoS(packet.QoS1))
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

	// 创建带 QoS1 遗嘱消息的客户端
	willOpts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("qos1-will-client").
		WithCleanSession(true).
		WithWill(&client.WillMessage{
			Topic:   "will/qos1",
			Payload: []byte("qos1 will message"),
			QoS:     packet.QoS1,
			Retain:  false,
		})

	willClient, err := client.NewWithOptions(willOpts)
	if err != nil {
		t.Fatalf("Failed to create will client: %v", err)
	}

	err = willClient.Connect()
	if err != nil {
		t.Fatalf("Failed to connect will client: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 异常断开
	willClient.ForceClose()

	// 验证遗嘱消息（QoS1 应该确保投递）
	select {
	case msg := <-received:
		if msg.Topic != "will/qos1" {
			t.Errorf("Expected topic 'will/qos1', got '%s'", msg.Topic)
		}
		if string(msg.Payload) != "qos1 will message" {
			t.Errorf("Expected payload 'qos1 will message', got '%s'", string(msg.Payload))
		}
		if msg.QoS != packet.QoS1 {
			t.Errorf("Expected QoS1, got %v", msg.QoS)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for QoS1 will message")
	}
}

// TestWillMessage_WithRetain 测试保留（Retain）遗嘱消息
func TestWillMessage_WithRetain(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建带保留遗嘱消息的客户端
	willOpts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("retain-will-client").
		WithCleanSession(true).
		WithWill(&client.WillMessage{
			Topic:   "will/retain",
			Payload: []byte("retained will message"),
			QoS:     packet.QoS0,
			Retain:  true, // 保留消息
		})

	willClient, err := client.NewWithOptions(willOpts)
	if err != nil {
		t.Fatalf("Failed to create will client: %v", err)
	}

	err = willClient.Connect()
	if err != nil {
		t.Fatalf("Failed to connect will client: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 异常断开，触发遗嘱消息
	willClient.ForceClose()

	time.Sleep(200 * time.Millisecond)

	// 创建新的订阅者，订阅保留主题
	subscriber := setupTestClient(t, addr, "retain-subscriber", nil)
	defer subscriber.Close()

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

	// 订阅主题，应该立即收到保留消息
	err = subscriber.Subscribe("will/retain")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// 验证保留的遗嘱消息
	select {
	case msg := <-received:
		if msg.Topic != "will/retain" {
			t.Errorf("Expected topic 'will/retain', got '%s'", msg.Topic)
		}
		if string(msg.Payload) != "retained will message" {
			t.Errorf("Expected payload 'retained will message', got '%s'", string(msg.Payload))
		}
		// 注意：默认 RetainAsPublished=false，所以接收到的消息 Retain 标志被清除
		// 这是符合协议规范的行为
		if msg.Retain {
			t.Error("Expected Retain flag to be false (default RetainAsPublished=false)")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for retained will message")
	}
}

// TestWillMessage_WithProperties 测试带属性的遗嘱消息
func TestWillMessage_WithProperties(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建订阅者客户端
	subscriber := setupTestClient(t, addr, "props-subscriber", nil)
	defer subscriber.Close()

	// 订阅遗嘱主题
	err := subscriber.Subscribe("will/props")
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

	// 创建带属性的遗嘱消息
	priority := packet.PriorityHigh
	willOpts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("props-will-client").
		WithCleanSession(true).
		WithWill(&client.WillMessage{
			Topic:       "will/props",
			Payload:     []byte("will with properties"),
			QoS:         packet.QoS0,
			Retain:      false,
			Priority:    &priority,
			TraceID:     "will-trace-123",
			ContentType: "text/plain",
			UserProperties: map[string]string{
				"source":   "will-test",
				"priority": "high",
			},
		})

	willClient, err := client.NewWithOptions(willOpts)
	if err != nil {
		t.Fatalf("Failed to create will client: %v", err)
	}

	err = willClient.Connect()
	if err != nil {
		t.Fatalf("Failed to connect will client: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 异常断开
	willClient.ForceClose()

	// 验证遗嘱消息及其属性
	select {
	case msg := <-received:
		if msg.Topic != "will/props" {
			t.Errorf("Expected topic 'will/props', got '%s'", msg.Topic)
		}
		if string(msg.Payload) != "will with properties" {
			t.Errorf("Expected payload 'will with properties', got '%s'", string(msg.Payload))
		}
		if msg.Priority != packet.PriorityHigh {
			t.Errorf("Expected Priority High, got %v", msg.Priority)
		}
		if msg.TraceID != "will-trace-123" {
			t.Errorf("Expected TraceID 'will-trace-123', got '%s'", msg.TraceID)
		}
		if msg.ContentType != "text/plain" {
			t.Errorf("Expected ContentType 'text/plain', got '%s'", msg.ContentType)
		}
		if msg.UserProperties["source"] != "will-test" {
			t.Errorf("Expected UserProperty source='will-test', got '%s'", msg.UserProperties["source"])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for will message with properties")
	}
}

// TestWillMessage_MultipleSubscribers 测试遗嘱消息发送给多个订阅者
func TestWillMessage_MultipleSubscribers(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建多个订阅者
	subscribers := make([]*client.Client, 3)
	receivedChans := make([]chan *client.Message, 3)

	for i := 0; i < 3; i++ {
		clientID := "multi-subscriber-" + string(rune('1'+i))
		subscribers[i] = setupTestClient(t, addr, clientID, nil)
		defer subscribers[i].Close()

		err := subscribers[i].Subscribe("will/multi")
		if err != nil {
			t.Fatalf("Failed to subscribe client %d: %v", i, err)
		}

		receivedChans[i] = make(chan *client.Message, 10)
		go func(idx int) {
			for {
				msg, err := subscribers[idx].PollMessage(context.Background(), 5*time.Second)
				if err != nil {
					return
				}
				receivedChans[idx] <- msg
			}
		}(i)
	}

	time.Sleep(100 * time.Millisecond)

	// 创建带遗嘱消息的客户端
	willOpts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("multi-will-client").
		WithCleanSession(true).
		WithWill(&client.WillMessage{
			Topic:   "will/multi",
			Payload: []byte("will to multiple subscribers"),
			QoS:     packet.QoS0,
			Retain:  false,
		})

	willClient, err := client.NewWithOptions(willOpts)
	if err != nil {
		t.Fatalf("Failed to create will client: %v", err)
	}

	err = willClient.Connect()
	if err != nil {
		t.Fatalf("Failed to connect will client: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 异常断开
	willClient.ForceClose()

	// 验证所有订阅者都收到遗嘱消息
	for i := 0; i < 3; i++ {
		select {
		case msg := <-receivedChans[i]:
			if msg.Topic != "will/multi" {
				t.Errorf("Subscriber %d: Expected topic 'will/multi', got '%s'", i, msg.Topic)
			}
			if string(msg.Payload) != "will to multiple subscribers" {
				t.Errorf("Subscriber %d: Expected payload 'will to multiple subscribers', got '%s'", i, string(msg.Payload))
			}
		case <-time.After(1 * time.Second):
			t.Errorf("Subscriber %d: Timeout waiting for will message", i)
		}
	}
}

// TestWillMessage_WithExpiry 测试带过期时间的遗嘱消息
func TestWillMessage_WithExpiry(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建订阅者客户端
	subscriber := setupTestClient(t, addr, "expiry-subscriber", nil)
	defer subscriber.Close()

	// 订阅遗嘱主题
	err := subscriber.Subscribe("will/expiry")
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

	// 创建带过期时间的遗嘱消息
	willOpts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("expiry-will-client").
		WithCleanSession(true).
		WithWill(&client.WillMessage{
			Topic:   "will/expiry",
			Payload: []byte("will with expiry"),
			QoS:     packet.QoS0,
			Retain:  false,
			Expiry:  30 * time.Second, // 30秒后过期
		})

	willClient, err := client.NewWithOptions(willOpts)
	if err != nil {
		t.Fatalf("Failed to create will client: %v", err)
	}

	err = willClient.Connect()
	if err != nil {
		t.Fatalf("Failed to connect will client: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 异常断开
	willClient.ForceClose()

	// 验证遗嘱消息及过期时间
	select {
	case msg := <-received:
		if msg.Topic != "will/expiry" {
			t.Errorf("Expected topic 'will/expiry', got '%s'", msg.Topic)
		}
		if string(msg.Payload) != "will with expiry" {
			t.Errorf("Expected payload 'will with expiry', got '%s'", string(msg.Payload))
		}
		if msg.ExpiryTime == 0 {
			t.Error("Expected ExpiryTime to be set")
		}
		// 验证过期时间在合理范围内（约30秒后）
		expectedExpiry := time.Now().Unix() + 30
		if msg.ExpiryTime < expectedExpiry-5 || msg.ExpiryTime > expectedExpiry+5 {
			t.Errorf("ExpiryTime out of expected range: got %d, expected around %d", msg.ExpiryTime, expectedExpiry)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for will message with expiry")
	}
}

// TestWillMessage_TargetClientID 测试发送给指定客户端的遗嘱消息
func TestWillMessage_TargetClientID(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建目标客户端
	targetClient := setupTestClient(t, addr, "target-client", nil)
	defer targetClient.Close()

	// 创建另一个非目标客户端
	otherClient := setupTestClient(t, addr, "other-client", nil)
	defer otherClient.Close()

	// 两个客户端都订阅相同主题
	err := targetClient.Subscribe("will/targeted")
	if err != nil {
		t.Fatalf("Failed to subscribe target client: %v", err)
	}

	err = otherClient.Subscribe("will/targeted")
	if err != nil {
		t.Fatalf("Failed to subscribe other client: %v", err)
	}

	// 启动两个客户端的消息接收
	targetReceived := make(chan *client.Message, 10)
	go func() {
		for {
			msg, err := targetClient.PollMessage(context.Background(), 5*time.Second)
			if err != nil {
				return
			}
			targetReceived <- msg
		}
	}()

	otherReceived := make(chan *client.Message, 10)
	go func() {
		for {
			msg, err := otherClient.PollMessage(context.Background(), 5*time.Second)
			if err != nil {
				return
			}
			otherReceived <- msg
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// 创建带目标客户端的遗嘱消息
	willOpts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("targeted-will-client").
		WithCleanSession(true).
		WithWill(&client.WillMessage{
			Topic:          "will/targeted",
			Payload:        []byte("targeted will message"),
			QoS:            packet.QoS0,
			Retain:         false,
			TargetClientID: "target-client", // 只发送给 target-client
		})

	willClient, err := client.NewWithOptions(willOpts)
	if err != nil {
		t.Fatalf("Failed to create will client: %v", err)
	}

	err = willClient.Connect()
	if err != nil {
		t.Fatalf("Failed to connect will client: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 异常断开
	willClient.ForceClose()

	// 验证只有目标客户端收到消息
	select {
	case msg := <-targetReceived:
		if msg.Topic != "will/targeted" {
			t.Errorf("Expected topic 'will/targeted', got '%s'", msg.Topic)
		}
		if string(msg.Payload) != "targeted will message" {
			t.Errorf("Expected payload 'targeted will message', got '%s'", string(msg.Payload))
		}
		if msg.TargetClientID != "target-client" {
			t.Errorf("Expected TargetClientID 'target-client', got '%s'", msg.TargetClientID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Target client: Timeout waiting for will message")
	}

	// 验证其他客户端没有收到消息
	select {
	case msg := <-otherReceived:
		t.Errorf("Other client should not receive targeted message, but got: %s", string(msg.Payload))
	case <-time.After(500 * time.Millisecond):
		// 正确：其他客户端没有收到消息
	}
}
