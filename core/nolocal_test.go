package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
)

// TestNoLocal_BasicScenario 测试 NoLocal 基本场景
// 1. 客户端订阅主题时启用 NoLocal
// 2. 客户端发布消息到该主题
// 3. 验证客户端不会收到自己发布的消息
func TestNoLocal_BasicScenario(t *testing.T) {
	// 创建并启动 core
	server := testSetupCore(t, nil)
	defer server.Stop()

	addr := testServe(t, server)

	// 创建客户端
	c1 := testSetupClient(t, addr, "client1", nil)
	defer c1.Close()

	// 订阅主题，启用 NoLocal
	subscribeOpts := client.NewSubscribeOptions()
	subscribeOpts.NoLocal = true
	if err := c1.SubscribeWithOptions([]string{"test/nolocal"}, subscribeOpts); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// 客户端发布消息到该主题
	if err := c1.Publish("test/nolocal", []byte("hello from client1"),
		client.NewPublishOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatal(err)
	}

	// 等待一段时间，确保消息已被处理
	time.Sleep(500 * time.Millisecond)

	// 验证客户端不应该收到消息
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := c1.PollMessage(ctx, 1*time.Second)
	if err == nil {
		t.Fatal("Expected timeout, but received message (NoLocal not working)")
	}
	if !errors.Is(err, client.ErrPollTimeout) {
		t.Fatalf("Expected ErrPollTimeout, got: %v", err)
	}
}

// TestNoLocal_MultipleClients 测试多客户端场景
// 1. client1 订阅主题，启用 NoLocal
// 2. client2 订阅同一主题，不启用 NoLocal
// 3. client1 发布消息
// 4. 验证 client1 不会收到消息，但 client2 会收到
func TestNoLocal_MultipleClients(t *testing.T) {
	server := testSetupCore(t, nil)
	defer server.Stop()

	addr := testServe(t, server)

	// 创建 client1，启用 NoLocal
	c1 := testSetupClient(t, addr, "client1", nil)
	defer c1.Close()

	subscribeOpts1 := client.NewSubscribeOptions()
	subscribeOpts1.NoLocal = true
	if err := c1.SubscribeWithOptions([]string{"test/nolocal"}, subscribeOpts1); err != nil {
		t.Fatal(err)
	}

	// 创建 client2，不启用 NoLocal
	c2 := testSetupClient(t, addr, "client2", nil)
	defer c2.Close()

	if err := c2.Subscribe("test/nolocal"); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// client1 发布消息
	if err := c1.Publish("test/nolocal", []byte("hello from client1"),
		client.NewPublishOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// client1 不应该收到消息
	ctx1, cancel1 := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel1()

	_, err := c1.PollMessage(ctx1, 1*time.Second)
	if err == nil {
		t.Fatal("client1: Expected timeout, but received message (NoLocal not working)")
	}
	if !errors.Is(err, client.ErrPollTimeout) {
		t.Fatalf("client1: Expected ErrPollTimeout, got: %v", err)
	}

	// client2 应该收到消息
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()

	msg, err := c2.PollMessage(ctx2, 1*time.Second)
	if err != nil {
		t.Fatalf("client2: Failed to receive message: %v", err)
	}

	if string(msg.Packet.Payload) != "hello from client1" {
		t.Errorf("client2: Unexpected payload: %s", string(msg.Packet.Payload))
	}

	// 验证 SourceClientID
	if msg.Packet.Properties.SourceClientID != "client1" {
		t.Errorf("Expected SourceClientID=client1, got: %s", msg.Packet.Properties.SourceClientID)
	}
}

// TestNoLocal_Disabled 测试 NoLocal 禁用时的行为
// 1. 客户端订阅主题，不启用 NoLocal
// 2. 客户端发布消息到该主题
// 3. 验证客户端会收到自己发布的消息
func TestNoLocal_Disabled(t *testing.T) {
	server := testSetupCore(t, nil)
	defer server.Stop()

	addr := testServe(t, server)

	// 创建客户端
	c1 := testSetupClient(t, addr, "client1", nil)
	defer c1.Close()

	// 订阅主题，不启用 NoLocal
	if err := c1.Subscribe("test/normal"); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// 客户端发布消息到该主题
	if err := c1.Publish("test/normal", []byte("hello from client1"),
		client.NewPublishOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatal(err)
	}

	// 客户端应该收到消息
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	msg, err := c1.PollMessage(ctx, 1*time.Second)
	if err != nil {
		t.Fatalf("Failed to receive message: %v", err)
	}

	if string(msg.Packet.Payload) != "hello from client1" {
		t.Errorf("Unexpected payload: %s", string(msg.Packet.Payload))
	}

	// 验证 SourceClientID
	if msg.Packet.Properties.SourceClientID != "client1" {
		t.Errorf("Expected SourceClientID=client1, got: %s", msg.Packet.Properties.SourceClientID)
	}
}

// TestNoLocal_WithWildcard 测试 NoLocal 与通配符订阅
// 1. 客户端使用通配符订阅，启用 NoLocal
// 2. 客户端发布消息到匹配的主题
// 3. 验证客户端不会收到自己发布的消息
func TestNoLocal_WithWildcard(t *testing.T) {
	server := testSetupCore(t, nil)
	defer server.Stop()

	addr := testServe(t, server)

	// 创建客户端
	c1 := testSetupClient(t, addr, "client1", nil)
	defer c1.Close()

	// 使用通配符订阅，启用 NoLocal
	subscribeOpts := client.NewSubscribeOptions()
	subscribeOpts.NoLocal = true
	if err := c1.SubscribeWithOptions([]string{"test/**"}, subscribeOpts); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// 客户端发布消息到匹配的主题
	if err := c1.Publish("test/nolocal/wildcard", []byte("wildcard test"),
		client.NewPublishOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatal(err)
	}

	time.Sleep(500 * time.Millisecond)

	// 验证客户端不应该收到消息
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := c1.PollMessage(ctx, 1*time.Second)
	if err == nil {
		t.Fatal("Expected timeout, but received message (NoLocal not working with wildcard)")
	}
	if !errors.Is(err, client.ErrPollTimeout) {
		t.Fatalf("Expected ErrPollTimeout, got: %v", err)
	}
}

// TestNoLocal_QoS0 测试 NoLocal 对 QoS 0 消息的支持
func TestNoLocal_QoS0(t *testing.T) {
	server := testSetupCore(t, nil)
	defer server.Stop()

	addr := testServe(t, server)

	// 创建客户端
	c1 := testSetupClient(t, addr, "client1", nil)
	defer c1.Close()

	// 订阅主题，启用 NoLocal
	subscribeOpts := client.NewSubscribeOptions()
	subscribeOpts.NoLocal = true
	if err := c1.SubscribeWithOptions([]string{"test/qos0"}, subscribeOpts); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// 客户端发布 QoS 0 消息
	if err := c1.Publish("test/qos0", []byte("qos0 message"),
		client.NewPublishOptions().WithQoS(packet.QoS0)); err != nil {
		t.Fatal(err)
	}

	time.Sleep(500 * time.Millisecond)

	// 验证客户端不应该收到消息
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := c1.PollMessage(ctx, 1*time.Second)
	if err == nil {
		t.Fatal("Expected timeout, but received QoS 0 message (NoLocal not working)")
	}
	if !errors.Is(err, client.ErrPollTimeout) {
		t.Fatalf("Expected DeadlineExceeded, got: %v", err)
	}
}
