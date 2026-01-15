package core

import (
	"context"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
)

// TestSubscribeWithRetainHandling 测试 RetainHandling 和 RetainAsPublished 的集成
// 这个测试直接使用 core 的内部方法来创建带有订阅选项的订阅
func TestSubscribeWithRetainHandling(t *testing.T) {
	// 启动 core
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 1. 发布保留消息
	publisher := setupTestClient(t, addr, "publisher", nil)
	defer publisher.Close()

	err := publisher.Publish("test/topic", []byte("retained message"),
		client.NewPublishOptions().WithQoS(packet.QoS1).WithRetain(true))
	if err != nil {
		t.Fatalf("Failed to publish retained message: %v", err)
	}

	// 等待消息保存
	time.Sleep(100 * time.Millisecond)

	// 验证保留消息已保存
	msgs := core.GetRetainedMessages("test/topic")
	if len(msgs) == 0 {
		t.Fatal("Retained message was not saved")
	}

	// 2. 测试 RetainHandling = 0 (总是发送, RetainAsPublished = true)
	t.Run("RetainHandling=0 RetainAsPublished=true", func(t *testing.T) {
		retained := core.retainStore.MatchForSubscription(
			"test/topic",
			true,  // RetainAsPublished = true
			true,  // isNewSubscription = true
			0,     // RetainHandling = 0 (总是发送)
		)

		if len(retained) != 1 {
			t.Fatalf("Expected 1 retained message, got %d", len(retained))
		}
		if string(retained[0].Payload) != "retained message" {
			t.Errorf("Expected 'retained message', got '%s'", string(retained[0].Payload))
		}
		if !retained[0].Retain {
			t.Error("Expected Retain=true (RetainAsPublished=true)")
		}
	})

	// 3. 测试 RetainHandling = 0 (总是发送, RetainAsPublished = false)
	t.Run("RetainHandling=0 RetainAsPublished=false", func(t *testing.T) {
		retained := core.retainStore.MatchForSubscription(
			"test/topic",
			false, // RetainAsPublished = false
			true,  // isNewSubscription = true
			0,     // RetainHandling = 0 (总是发送)
		)

		if len(retained) != 1 {
			t.Fatalf("Expected 1 retained message, got %d", len(retained))
		}
		if string(retained[0].Payload) != "retained message" {
			t.Errorf("Expected 'retained message', got '%s'", string(retained[0].Payload))
		}
		if retained[0].Retain {
			t.Error("Expected Retain=false (RetainAsPublished=false)")
		}
	})

	// 4. 测试 RetainHandling = 1 (仅新订阅发送)
	t.Run("RetainHandling=1 new subscription", func(t *testing.T) {
		// 新订阅，应该收到保留消息
		retained := core.retainStore.MatchForSubscription(
			"test/topic",
			true, // RetainAsPublished = true
			true, // isNewSubscription = true
			1,    // RetainHandling = 1 (仅新订阅发送)
		)

		if len(retained) != 1 {
			t.Fatalf("Expected 1 retained message for new subscription, got %d", len(retained))
		}
	})

	t.Run("RetainHandling=1 existing subscription", func(t *testing.T) {
		// 已存在的订阅，不应该收到保留消息
		retained := core.retainStore.MatchForSubscription(
			"test/topic",
			true,  // RetainAsPublished = true
			false, // isNewSubscription = false
			1,     // RetainHandling = 1 (仅新订阅发送)
		)

		if len(retained) != 0 {
			t.Fatalf("Expected 0 retained messages for existing subscription, got %d", len(retained))
		}
	})

	// 5. 测试 RetainHandling = 2 (不发送)
	t.Run("RetainHandling=2 never send", func(t *testing.T) {
		retained := core.retainStore.MatchForSubscription(
			"test/topic",
			true, // RetainAsPublished = true
			true, // isNewSubscription = true
			2,    // RetainHandling = 2 (不发送)
		)

		if len(retained) != 0 {
			t.Fatalf("Expected 0 retained messages with RH=2, got %d", len(retained))
		}
	})
}

// TestSubscribeActualFlow 测试实际订阅流程中的保留消息发送
func TestSubscribeActualFlow(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 1. 发布保留消息
	publisher := setupTestClient(t, addr, "publisher", nil)
	defer publisher.Close()

	err := publisher.Publish("test/retain", []byte("retained message"),
		client.NewPublishOptions().WithQoS(packet.QoS1).WithRetain(true))
	if err != nil {
		t.Fatalf("Failed to publish retained message: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 2. 订阅并接收保留消息
	subscriber := setupTestClient(t, addr, "subscriber", nil)
	defer subscriber.Close()

	// 启动消息接收
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

	// 订阅主题 - 默认应该收到保留消息
	err = subscriber.Subscribe("test/retain")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// 验证收到保留消息
	select {
	case msg := <-received:
		if string(msg.Payload) != "retained message" {
			t.Errorf("Expected 'retained message', got '%s'", string(msg.Payload))
		}
		// 注意：当前客户端默认 RetainAsPublished 为 false，所以 Retain 应该是 false
		// 如果未来客户端支持设置 RetainAsPublished，这里需要相应调整
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for retained message")
	}
}
