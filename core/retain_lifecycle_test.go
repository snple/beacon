package core

import (
	"context"
	"testing"
	"time"

	"github.com/snple/beacon/client"
)

// TestRetainMessage_LifecycleIndependence 测试保留消息的生命周期独立性
// 保留消息应该独立于发布者客户端存在，客户端断开后保留消息应该仍然存在
func TestRetainMessage_LifecycleIndependence(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 场景 1：客户端发布保留消息后正常断开
	t.Run("RetainMessageAfterNormalDisconnect", func(t *testing.T) {
		// 创建发布者客户端
		publisher := testSetupClient(t, addr, "publisher-1", nil)

		// 发布保留消息
		err := publisher.Publish("sensor/temp/room1", []byte("25.5"),
			client.NewPublishOptions().WithRetain(true))
		if err != nil {
			t.Fatalf("Failed to publish retain message: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		// 正常断开发布者
		publisher.Close()
		time.Sleep(50 * time.Millisecond)

		// 验证保留消息仍然存在
		retainMsgs := core.GetRetainedMessages("sensor/temp/room1")
		if len(retainMsgs) != 1 {
			t.Errorf("Expected 1 retain message after publisher disconnect, got %d", len(retainMsgs))
		}
		if len(retainMsgs) > 0 && string(retainMsgs[0].Packet.Payload) != "25.5" {
			t.Errorf("Retain message payload mismatch: got %s, want 25.5", retainMsgs[0].Packet.Payload)
		}

		// 新订阅者应该能收到保留消息
		subscriber := testSetupClient(t, addr, "subscriber-1", nil)
		defer subscriber.Close()

		received := make(chan *client.Message, 1)
		go func() {
			for {
				msg, err := subscriber.PollMessage(context.Background(), 5*time.Second)
				if err != nil {
					return
				}
				received <- msg
			}
		}()

		time.Sleep(50 * time.Millisecond)

		// 订阅主题
		err = subscriber.Subscribe("sensor/temp/room1")
		if err != nil {
			t.Fatalf("Failed to subscribe: %v", err)
		}

		// 应该立即收到保留消息
		select {
		case msg := <-received:
			if string(msg.Packet.Payload) != "25.5" {
				t.Errorf("Received wrong message: %s", msg.Packet.Payload)
			}
		case <-time.After(1 * time.Second):
			t.Error("Did not receive retain message")
		}
	})

	// 场景 2：客户端发布保留消息后异常断开
	t.Run("RetainMessageAfterAbnormalDisconnect", func(t *testing.T) {
		// 创建发布者客户端
		publisher := testSetupClient(t, addr, "publisher-2", nil)

		// 发布保留消息
		err := publisher.Publish("sensor/temp/room2", []byte("26.5"),
			client.NewPublishOptions().WithRetain(true))
		if err != nil {
			t.Fatalf("Failed to publish retain message: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		// 异常断开发布者（模拟网络故障）
		publisher.ForceClose()
		time.Sleep(50 * time.Millisecond)

		// 验证保留消息仍然存在
		retainMsgs := core.GetRetainedMessages("sensor/temp/room2")
		if len(retainMsgs) != 1 {
			t.Errorf("Expected 1 retain message after abnormal disconnect, got %d", len(retainMsgs))
		}
		if len(retainMsgs) > 0 && string(retainMsgs[0].Packet.Payload) != "26.5" {
			t.Errorf("Retain message payload mismatch: got %s, want 26.5", retainMsgs[0].Packet.Payload)
		}
	})

	// 场景 3：多个客户端发布到同一主题，保留消息应该被覆盖
	t.Run("RetainMessageOverwrite", func(t *testing.T) {
		// 第一个发布者
		publisher1 := testSetupClient(t, addr, "publisher-3", nil)
		err := publisher1.Publish("sensor/temp/room3", []byte("20.0"),
			client.NewPublishOptions().WithRetain(true))
		if err != nil {
			t.Fatalf("Failed to publish retain message: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
		publisher1.Close()
		time.Sleep(50 * time.Millisecond)

		// 验证第一条保留消息
		retainMsgs := core.GetRetainedMessages("sensor/temp/room3")
		if len(retainMsgs) != 1 || string(retainMsgs[0].Packet.Payload) != "20.0" {
			t.Error("First retain message not found")
		}

		// 第二个发布者覆盖保留消息
		publisher2 := testSetupClient(t, addr, "publisher-4", nil)
		err = publisher2.Publish("sensor/temp/room3", []byte("30.0"),
			client.NewPublishOptions().WithRetain(true))
		if err != nil {
			t.Fatalf("Failed to publish retain message: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
		publisher2.Close()
		time.Sleep(50 * time.Millisecond)

		// 验证保留消息已被覆盖
		retainMsgs = core.GetRetainedMessages("sensor/temp/room3")
		if len(retainMsgs) != 1 {
			t.Errorf("Expected 1 retain message, got %d", len(retainMsgs))
		}
		if len(retainMsgs) > 0 && string(retainMsgs[0].Packet.Payload) != "30.0" {
			t.Errorf("Retain message should be updated to 30.0, got %s", retainMsgs[0].Packet.Payload)
		}
	})

	// 场景 4：清空保留消息（发送空载荷）
	t.Run("ClearRetainMessage", func(t *testing.T) {
		// 先发布保留消息
		publisher := testSetupClient(t, addr, "publisher-5", nil)
		err := publisher.Publish("sensor/temp/room4", []byte("22.0"),
			client.NewPublishOptions().WithRetain(true))
		if err != nil {
			t.Fatalf("Failed to publish retain message: %v", err)
		}
		time.Sleep(50 * time.Millisecond)

		// 验证保留消息存在
		retainMsgs := core.GetRetainedMessages("sensor/temp/room4")
		if len(retainMsgs) != 1 {
			t.Error("Retain message should exist")
		}

		// 发送空载荷清除保留消息
		err = publisher.Publish("sensor/temp/room4", []byte{},
			client.NewPublishOptions().WithRetain(true))
		if err != nil {
			t.Fatalf("Failed to clear retain message: %v", err)
		}
		time.Sleep(50 * time.Millisecond)

		publisher.Close()
		time.Sleep(50 * time.Millisecond)

		// 验证保留消息已被清除
		retainMsgs = core.GetRetainedMessages("sensor/temp/room4")
		if len(retainMsgs) != 0 {
			t.Errorf("Retain message should be cleared, got %d messages", len(retainMsgs))
		}
	})
}

// TestRetainMessage_ServerRestart 测试服务器重启后保留消息的持久化
func TestRetainMessage_ServerRestart(t *testing.T) {
	t.Skip("需要持久化存储支持才能测试服务器重启")
	// 这个测试需要：
	// 1. 启用持久化存储
	// 2. 发布保留消息
	// 3. 停止 core
	// 4. 重新启动 core
	// 5. 验证保留消息仍然存在
}

// TestRetainMessage_WithWildcard 测试通配符订阅获取保留消息
func TestRetainMessage_WithWildcard(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 发布多个保留消息
	publisher := testSetupClient(t, addr, "publisher", nil)
	defer publisher.Close()

	topics := []struct {
		topic   string
		payload string
	}{
		{"sensor/temp/room1", "20.0"},
		{"sensor/temp/room2", "21.0"},
		{"sensor/humidity/room1", "60.0"},
		{"sensor/humidity/room2", "65.0"},
	}

	for _, tt := range topics {
		err := publisher.Publish(tt.topic, []byte(tt.payload),
			client.NewPublishOptions().WithRetain(true))
		if err != nil {
			t.Fatalf("Failed to publish retain message for %s: %v", tt.topic, err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	// 使用通配符订阅获取保留消息
	subscriber := testSetupClient(t, addr, "subscriber", nil)
	defer subscriber.Close()

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

	time.Sleep(50 * time.Millisecond)

	// 订阅 sensor/temp/*
	err := subscriber.Subscribe("sensor/temp/*")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// 应该收到 2 条保留消息
	receivedCount := 0
	timeout := time.After(1 * time.Second)
	for receivedCount < 2 {
		select {
		case msg := <-received:
			t.Logf("Received: %s = %s", msg.Packet.Topic, msg.Packet.Payload)
			receivedCount++
		case <-timeout:
			t.Fatalf("Expected 2 retain messages, got %d", receivedCount)
		}
	}

	if receivedCount != 2 {
		t.Errorf("Expected 2 retain messages for wildcard subscription, got %d", receivedCount)
	}
}
