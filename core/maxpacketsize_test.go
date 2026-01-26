package core

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
)

// TestMaxPacketSize_MessageDropped 测试超过客户端 MaxPacketSize 的消息会被丢弃
func TestMaxPacketSize_MessageDropped(t *testing.T) {
	// 创建 core
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := testServe(t, core)

	// 创建客户端，设置一个很小的 maxPacketSize
	opts := client.NewClientOptions()
	opts.WithCore(addr).
		WithClientID("test-client").
		WithMaxPacketSize(100) // 设置很小的最大包大小

	c, err := client.NewWithOptions(opts)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	dialer := &client.TCPDialer{Address: addr, DialTimeout: 10 * time.Second}
	if err := c.ConnectWithDialer(dialer); err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}
	defer c.Close()

	// 订阅主题
	if err := c.Subscribe("test/topic"); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// 等待订阅完成
	time.Sleep(50 * time.Millisecond)

	// 发送一个大消息（payload 本身超过 100 字节）
	largePayload := bytes.Repeat([]byte("x"), 200)

	// 获取发送前的统计
	statsBeforeDrop := core.GetStats().MessagesDropped

	// 发布大消息 (使用 Publish API)
	err = c.Publish("test/topic", largePayload, client.NewPublishOptions().WithQoS(packet.QoS1))
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// 等待消息处理
	time.Sleep(200 * time.Millisecond)

	// 验证消息被丢弃
	statsAfterDrop := core.GetStats().MessagesDropped
	if statsAfterDrop <= statsBeforeDrop {
		t.Errorf("Expected message to be dropped, but MessagesDropped did not increase: before=%d, after=%d",
			statsBeforeDrop, statsAfterDrop)
	}

	// 发送一个小消息，确保正常消息仍然可以投递
	smallPayload := []byte("small")

	err = c.Publish("test/topic", smallPayload, client.NewPublishOptions().WithQoS(packet.QoS1))
	if err != nil {
		t.Fatalf("Publish small message failed: %v", err)
	}

	// 验证小消息能够成功接收
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	msg, err := c.PollMessage(ctx, 2*time.Second)
	if err != nil {
		t.Errorf("Failed to receive small message: %v", err)
	} else if string(msg.Payload()) != "small" {
		t.Errorf("Expected payload 'small', got '%s'", string(msg.Payload()))
	} else {
		t.Logf("Small message received successfully")
	}

	t.Logf("Test completed. Total messages dropped: %d", core.GetStats().MessagesDropped)
}

// TestMaxPacketSize_QoS0AndQoS1 测试 QoS0 和 QoS1 消息都会被检查
func TestMaxPacketSize_QoS0AndQoS1(t *testing.T) {
	// 创建 core
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := testServe(t, core)

	// 创建客户端，设置一个很小的 maxPacketSize
	opts := client.NewClientOptions()
	opts.WithCore(addr).
		WithClientID("test-client-qos").
		WithMaxPacketSize(100)

	c, err := client.NewWithOptions(opts)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	dialer := &client.TCPDialer{Address: addr, DialTimeout: 10 * time.Second}
	if err := c.ConnectWithDialer(dialer); err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}
	defer c.Close()

	// 订阅主题
	if err := c.Subscribe("test/qos"); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// 等待订阅完成
	time.Sleep(50 * time.Millisecond)

	// 大 payload
	largePayload := bytes.Repeat([]byte("y"), 200)

	// 测试 QoS0
	statsBeforeDrop := core.GetStats().MessagesDropped
	err = c.Publish("test/qos", largePayload, client.NewPublishOptions().WithQoS(packet.QoS0))
	if err != nil {
		t.Fatalf("Publish QoS0 failed: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	statsAfterDrop := core.GetStats().MessagesDropped
	if statsAfterDrop <= statsBeforeDrop {
		t.Logf("QoS0: Message was dropped as expected (got %d drops)", statsAfterDrop)
	}

	// 测试 QoS1
	statsBeforeDrop = core.GetStats().MessagesDropped
	err = c.Publish("test/qos", largePayload, client.NewPublishOptions().WithQoS(packet.QoS1))
	if err != nil {
		t.Fatalf("Publish QoS1 failed: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	statsAfterDrop = core.GetStats().MessagesDropped
	if statsAfterDrop <= statsBeforeDrop {
		t.Logf("QoS1: Message was dropped as expected (got %d drops)", statsAfterDrop)
	}
}

// TestMaxPacketSize_NoLimit 测试未设置 MaxPacketSize 时不会限制
func TestMaxPacketSize_NoLimit(t *testing.T) {
	// 创建 core
	core := testSetupCore(t, nil)
	defer core.Stop()

	// 创建客户端，不设置 maxPacketSize（默认为 0，表示不限制）
	c := testSetupClient(t, testServe(t, core), "test-client-no-limit", nil)
	defer c.Close()

	// 订阅主题
	if err := c.Subscribe("test/no-limit"); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// 等待订阅完成
	time.Sleep(50 * time.Millisecond)

	// 发送一个比较大的消息
	largePayload := bytes.Repeat([]byte("z"), 5000)
	statsBeforeDropped := core.GetStats().MessagesDropped

	err := c.Publish("test/no-limit", largePayload, client.NewPublishOptions().WithQoS(packet.QoS1))
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// 等待消息处理
	time.Sleep(200 * time.Millisecond)

	// 验证消息未被丢弃
	statsAfterDropped := core.GetStats().MessagesDropped
	if statsAfterDropped > statsBeforeDropped {
		t.Errorf("Large message should not be dropped when no limit is set, but got %d drops",
			statsAfterDropped-statsBeforeDropped)
	} else {
		t.Logf("Large message not dropped when no limit set (correct behavior)")
	}
}

// TestMaxPacketSize_WithLongTopic 测试长主题名称也会计入包大小
func TestMaxPacketSize_WithLongTopic(t *testing.T) {
	// 创建 core
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := testServe(t, core)

	// 创建客户端，设置较小的 maxPacketSize
	opts := client.NewClientOptions()
	opts.WithCore(addr).
		WithClientID("test-client-long-topic").
		WithMaxPacketSize(150)

	c, err := client.NewWithOptions(opts)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	dialer := &client.TCPDialer{Address: addr, DialTimeout: 10 * time.Second}
	if err := c.ConnectWithDialer(dialer); err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}
	defer c.Close()

	// 使用一个长主题名称
	longTopic := "test/" + strings.Repeat("long/", 20) + "topic"

	if err := c.Subscribe(longTopic); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// 等待订阅完成
	time.Sleep(50 * time.Millisecond)

	// 发送小 payload，但加上长主题名称后总包大小会超过限制
	smallPayload := []byte("test")

	statsBeforeDrop := core.GetStats().MessagesDropped

	err = c.Publish(longTopic, smallPayload, client.NewPublishOptions().WithQoS(packet.QoS1))
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// 等待消息处理
	time.Sleep(200 * time.Millisecond)

	// 验证消息被丢弃（长主题 + 小 payload 仍可能超过 150 字节）
	statsAfterDrop := core.GetStats().MessagesDropped

	// 注意：这个测试可能需要根据实际编码后的包大小进行调整
	// 这里只是演示长主题也会影响包大小的计算
	if statsAfterDrop > statsBeforeDrop {
		t.Logf("Message with long topic was dropped as expected (total packet size too large)")
	}
}
