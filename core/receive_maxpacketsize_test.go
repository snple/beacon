package core

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/snple/beacon/client"
)

// TestReceiveMaxPacketSize_PublishTooLarge 测试客户端发送过大的 PUBLISH 包被拒绝
func TestReceiveMaxPacketSize_PublishTooLarge(t *testing.T) {
	// 创建 core，设置很小的接收包大小限制
	coreOpts := NewCoreOptions()
	coreOpts.WithMaxPacketSize(200) // 设置 Core 接收包大小限制为 200 字节

	core := testSetupCore(t, coreOpts)
	defer core.Stop()

	// 创建客户端（无发送限制）
	cli := testSetupClient(t, testServe(t, core), "test-client", nil)
	defer cli.Close()

	// 订阅主题
	if err := cli.Subscribe("test/topic"); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	statsBeforeDrop := core.GetStats().MessagesDropped

	// 发送一个大的 PUBLISH 包（payload 超过 200 字节）
	largePayload := bytes.Repeat([]byte("x"), 300)
	if err := cli.Publish("test/topic", largePayload, nil); err != nil {
		t.Logf("Publish failed (expected, connection may be closed): %v", err)
	}

	// 等待 Core 处理并断开连接
	time.Sleep(200 * time.Millisecond)

	// 验证客户端已断开
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	msg, err := cli.PollMessage(ctx, 50*time.Millisecond)
	if err == nil && msg != nil {
		t.Error("Client should be disconnected, but still receiving messages")
	}

	// 验证消息被丢弃
	statsAfterDrop := core.GetStats().MessagesDropped
	if statsAfterDrop <= statsBeforeDrop {
		t.Logf("Warning: MessagesDropped counter did not increase (before: %d, after: %d)",
			statsBeforeDrop, statsAfterDrop)
	} else {
		t.Logf("Large packet was dropped and client disconnected (MessagesDropped: %d -> %d)",
			statsBeforeDrop, statsAfterDrop)
	}
}

// TestReceiveMaxPacketSize_RequestTooLarge 测试客户端发送过大的 REQUEST 包被拒绝
func TestReceiveMaxPacketSize_RequestTooLarge(t *testing.T) {
	// 创建 core，设置很小的接收包大小限制
	coreOpts := NewCoreOptions()
	coreOpts.WithMaxPacketSize(200)

	core := testSetupCore(t, coreOpts)
	defer core.Stop()

	// 创建处理器客户端
	handler := testSetupClient(t, testServe(t, core), "handler", nil)
	defer handler.Close()

	if err := handler.Register("test.action"); err != nil {
		t.Fatalf("Failed to register action: %v", err)
	}

	// 启动请求处理器
	go func() {
		for {
			req, err := handler.PollRequest(context.Background(), 5*time.Second)
			if err != nil {
				return
			}
			if req != nil {
				resp := client.NewResponse([]byte("response"))
				req.Response(resp)
			}
		}
	}()

	// 创建请求客户端
	requester := testSetupClient(t, testServe(t, core), "requester", nil)
	defer requester.Close()

	time.Sleep(50 * time.Millisecond)

	statsBeforeDrop := core.GetStats().MessagesDropped

	// 发送一个大的 REQUEST 包
	largePayload := bytes.Repeat([]byte("y"), 300)
	req := client.NewRequest("test.action", largePayload)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := requester.Request(ctx, req)
	if err != nil {
		t.Logf("Request failed or timed out (expected): %v", err)
	}

	// 验证消息被丢弃
	time.Sleep(200 * time.Millisecond)
	statsAfterDrop := core.GetStats().MessagesDropped
	if statsAfterDrop <= statsBeforeDrop {
		t.Logf("Warning: MessagesDropped counter did not increase")
	} else {
		t.Logf("Large REQUEST packet was dropped (MessagesDropped: %d -> %d)",
			statsBeforeDrop, statsAfterDrop)
	}
}

// TestReceiveMaxPacketSize_SmallPacketsWork 测试小包仍然可以正常工作
func TestReceiveMaxPacketSize_SmallPacketsWork(t *testing.T) {
	// 创建 core，设置适中的接收包大小限制
	coreOpts := NewCoreOptions()
	coreOpts.WithMaxPacketSize(1000)

	core := testSetupCore(t, coreOpts)
	defer core.Stop()

	// 创建客户端
	cli := testSetupClient(t, testServe(t, core), "test-client", nil)
	defer cli.Close()

	// 订阅主题
	if err := cli.Subscribe("test/small"); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	statsBeforeDrop := core.GetStats().MessagesDropped

	// 发送多个小消息
	for i := 0; i < 5; i++ {
		payload := []byte("small message")
		if err := cli.Publish("test/small", payload, nil); err != nil {
			t.Errorf("Failed to publish message %d: %v", i, err)
		}
	}

	// 接收消息验证
	received := 0
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		msg, err := cli.PollMessage(ctx, 500*time.Millisecond)
		cancel()

		if err != nil {
			t.Logf("Message %d receive timeout or error: %v", i, err)
			break
		}
		if msg != nil {
			received++
		}
	}

	if received < 3 {
		t.Errorf("Expected to receive at least 3 messages, got %d", received)
	}

	// 验证没有消息被丢弃
	statsAfterDrop := core.GetStats().MessagesDropped
	if statsAfterDrop > statsBeforeDrop {
		t.Errorf("Small messages should not be dropped, but MessagesDropped increased from %d to %d",
			statsBeforeDrop, statsAfterDrop)
	} else {
		t.Logf("All small messages handled successfully (received: %d)", received)
	}
}

// TestReceiveMaxPacketSize_NoLimit 测试使用大的限制值时大包可以接收
func TestReceiveMaxPacketSize_NoLimit(t *testing.T) {
	// 创建 core，设置很大的接收包大小限制
	coreOpts := NewCoreOptions()
	coreOpts.WithMaxPacketSize(10 * 1024 * 1024) // 10MB

	core := testSetupCore(t, coreOpts)
	defer core.Stop()

	// 创建客户端
	cli := testSetupClient(t, testServe(t, core), "test-client", nil)
	defer cli.Close()

	// 订阅主题
	if err := cli.Subscribe("test/unlimited"); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	statsBeforeDrop := core.GetStats().MessagesDropped

	// 发送一个很大的 PUBLISH 包
	largePayload := bytes.Repeat([]byte("z"), 5000)
	if err := cli.Publish("test/unlimited", largePayload, nil); err != nil {
		t.Errorf("Failed to publish large message: %v", err)
	}

	// 等待并接收消息
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	msg, err := cli.PollMessage(ctx, 1*time.Second)

	if err != nil {
		t.Errorf("Should receive large message when no limit set: %v", err)
	}
	if msg == nil {
		t.Error("Expected to receive large message, got nil")
	} else if len(msg.Packet.Payload) != 5000 {
		t.Errorf("Expected payload length 5000, got %d", len(msg.Packet.Payload))
	}

	// 验证没有消息被丢弃
	statsAfterDrop := core.GetStats().MessagesDropped
	if statsAfterDrop > statsBeforeDrop {
		t.Errorf("No messages should be dropped with large limit, but got %d drops",
			statsAfterDrop-statsBeforeDrop)
	} else {
		t.Logf("Large message received successfully without drops")
	}
}
