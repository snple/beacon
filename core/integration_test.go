package core

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// ============================================================================
// 测试辅助函数
// ============================================================================

// testLogger 创建测试用的 logger
func testIntegrationLogger() *zap.Logger {
	logger, _ := zap.NewDevelopment()
	return logger
}

// testSetupCore 创建测试用的 Core
func testSetupCore(t *testing.T, opts *CoreOptions) *Core {
	t.Helper()
	if opts == nil {
		opts = NewCoreOptions()
	}
	opts.WithAddress("127.0.0.1:0"). // 使用随机端口
						WithLogger(testIntegrationLogger()).
						WithConnectTimeout(5).
						WithRequestQueueSize(100).
						WithMessageQueueSize(100)

	core, err := NewWithOptions(opts)
	if err != nil {
		t.Fatalf("Failed to create core: %v", err)
	}

	if err := core.Start(); err != nil {
		t.Fatalf("Failed to start core: %v", err)
	}

	return core
}

// testSetupClient 创建测试用的 Client
func testSetupClient(t *testing.T, coreAddr string, clientID string, opts *client.ClientOptions) *client.Client {
	t.Helper()
	if opts == nil {
		opts = client.NewClientOptions()
	}
	opts.WithCore(coreAddr).
		WithClientID(clientID).
		WithLogger(testIntegrationLogger()).
		WithRequestQueueSize(100).
		WithMessageQueueSize(100)

	c, err := client.NewWithOptions(opts)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if err := c.Connect(); err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}

	return c
}

// ============================================================================
// 发布订阅测试
// ============================================================================

// TestPubSub_BasicQoS0 测试基本的 QoS0 发布订阅
func TestPubSub_BasicQoS0(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建发布者和订阅者
	publisher := testSetupClient(t, addr, "publisher1", nil)
	defer publisher.Close()

	subscriber := testSetupClient(t, addr, "subscriber1", nil)
	defer subscriber.Close()

	// 订阅主题
	err := subscriber.Subscribe("test/topic")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// 等待订阅生效
	time.Sleep(100 * time.Millisecond)

	// 发布消息
	payload := []byte("hello world")
	err = publisher.Publish("test/topic", payload, nil)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// 订阅者接收消息
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	msg, err := subscriber.PollMessage(ctx, 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to poll message: %v", err)
	}

	if msg.Packet.Topic != "test/topic" {
		t.Errorf("Expected topic 'test/topic', got '%s'", msg.Packet.Topic)
	}
	if string(msg.Packet.Payload) != "hello world" {
		t.Errorf("Expected payload 'hello world', got '%s'", string(msg.Packet.Payload))
	}
}

// TestPubSub_BasicQoS1 测试基本的 QoS1 发布订阅
func TestPubSub_BasicQoS1(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	publisher := testSetupClient(t, addr, "pub-qos1", nil)
	defer publisher.Close()

	subscriber := testSetupClient(t, addr, "sub-qos1", nil)
	defer subscriber.Close()

	// 订阅主题（QoS1）
	err := subscriber.SubscribeWithOptions([]string{"test/qos1"}, client.NewSubscribeOptions().WithQoS(packet.QoS1))
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 发布 QoS1 消息
	opts := client.NewPublishOptions().WithQoS(packet.QoS1)
	err = publisher.Publish("test/qos1", []byte("qos1 message"), opts)
	if err != nil {
		t.Fatalf("Failed to publish QoS1: %v", err)
	}

	// 接收消息
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	msg, err := subscriber.PollMessage(ctx, 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to poll message: %v", err)
	}

	if string(msg.Packet.Payload) != "qos1 message" {
		t.Errorf("Expected 'qos1 message', got '%s'", string(msg.Packet.Payload))
	}
}

// TestPubSub_WildcardSingleLevel 测试单层通配符 (*)
func TestPubSub_WildcardSingleLevel(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	publisher := testSetupClient(t, addr, "pub-wild", nil)
	defer publisher.Close()

	subscriber := testSetupClient(t, addr, "sub-wild", nil)
	defer subscriber.Close()

	// 订阅通配符主题（使用 * 代替 MQTT 的 +）
	err := subscriber.Subscribe("sensor/*/temperature")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 发布匹配的消息
	topics := []string{
		"sensor/room1/temperature",
		"sensor/room2/temperature",
		"sensor/kitchen/temperature",
	}

	for _, topic := range topics {
		err = publisher.Publish(topic, []byte(topic), nil)
		if err != nil {
			t.Fatalf("Failed to publish to %s: %v", topic, err)
		}
	}

	// 接收所有消息
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	received := make(map[string]bool)
	for i := 0; i < 3; i++ {
		msg, err := subscriber.PollMessage(ctx, 2*time.Second)
		if err != nil {
			t.Fatalf("Failed to poll message %d: %v", i, err)
		}
		received[msg.Packet.Topic] = true
	}

	for _, topic := range topics {
		if !received[topic] {
			t.Errorf("Did not receive message for topic: %s", topic)
		}
	}
}

// TestPubSub_WildcardMultiLevel 测试多层通配符 (**)
func TestPubSub_WildcardMultiLevel(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	publisher := testSetupClient(t, addr, "pub-multi", nil)
	defer publisher.Close()

	subscriber := testSetupClient(t, addr, "sub-multi", nil)
	defer subscriber.Close()

	// 订阅多层通配符主题（使用 ** 代替 MQTT 的 #）
	err := subscriber.Subscribe("home/**")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 发布匹配的消息
	topics := []string{
		"home/room1",
		"home/room1/light",
		"home/room2/sensor/temperature",
	}

	for _, topic := range topics {
		err = publisher.Publish(topic, []byte(topic), nil)
		if err != nil {
			t.Fatalf("Failed to publish to %s: %v", topic, err)
		}
	}

	// 接收所有消息
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	received := make(map[string]bool)
	for i := 0; i < 3; i++ {
		msg, err := subscriber.PollMessage(ctx, 2*time.Second)
		if err != nil {
			t.Fatalf("Failed to poll message %d: %v", i, err)
		}
		received[msg.Packet.Topic] = true
	}

	for _, topic := range topics {
		if !received[topic] {
			t.Errorf("Did not receive message for topic: %s", topic)
		}
	}
}

// TestPubSub_MultipleSubscribers 测试多个订阅者
func TestPubSub_MultipleSubscribers(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	publisher := testSetupClient(t, addr, "pub-multi-sub", nil)
	defer publisher.Close()

	// 创建多个订阅者
	const numSubscribers = 3
	subscribers := make([]*client.Client, numSubscribers)
	msgChannels := make([]chan *client.Message, numSubscribers)
	errChannels := make([]chan error, numSubscribers)

	for i := 0; i < numSubscribers; i++ {
		subscribers[i] = testSetupClient(t, addr, "sub"+string(rune('A'+i)), nil)
		defer subscribers[i].Close()

		err := subscribers[i].Subscribe("broadcast")
		if err != nil {
			t.Fatalf("Subscriber %d failed to subscribe: %v", i, err)
		}

		// 为每个订阅者启动轮询协程（这会初始化消息队列）
		msgChannels[i] = make(chan *client.Message, 1)
		errChannels[i] = make(chan error, 1)
		go func(sub *client.Client, msgCh chan *client.Message, errCh chan error) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			msg, err := sub.PollMessage(ctx, 4*time.Second)
			if err != nil {
				errCh <- err
			} else {
				msgCh <- msg
			}
		}(subscribers[i], msgChannels[i], errChannels[i])
	}

	// 等待所有订阅者的轮询协程启动
	time.Sleep(200 * time.Millisecond)

	// 发布消息
	err := publisher.Publish("broadcast", []byte("hello all"), nil)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// 所有订阅者都应收到消息
	for i := 0; i < numSubscribers; i++ {
		select {
		case msg := <-msgChannels[i]:
			if string(msg.Packet.Payload) != "hello all" {
				t.Errorf("Subscriber %d got wrong payload: %s", i, string(msg.Packet.Payload))
			}
		case err := <-errChannels[i]:
			t.Fatalf("Subscriber %d failed to receive: %v", i, err)
		case <-time.After(5 * time.Second):
			t.Fatalf("Subscriber %d timed out", i)
		}
	}
}

// TestPubSub_Unsubscribe 测试取消订阅
func TestPubSub_Unsubscribe(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	publisher := testSetupClient(t, addr, "pub-unsub", nil)
	defer publisher.Close()

	subscriber := testSetupClient(t, addr, "sub-unsub", nil)
	defer subscriber.Close()

	// 订阅
	err := subscriber.Subscribe("test/unsub")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// 取消订阅
	err = subscriber.Unsubscribe("test/unsub")
	if err != nil {
		t.Fatalf("Failed to unsubscribe: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// 发布消息
	err = publisher.Publish("test/unsub", []byte("should not receive"), nil)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// 订阅者不应收到消息
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err = subscriber.PollMessage(ctx, 300*time.Millisecond)
	if err == nil {
		t.Error("Should not receive message after unsubscribe")
	}
}

// TestPubSub_TargetClientID 测试定向发布
func TestPubSub_TargetClientID(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	publisher := testSetupClient(t, addr, "pub-target", nil)
	defer publisher.Close()

	target := testSetupClient(t, addr, "target-client", nil)
	defer target.Close()

	other := testSetupClient(t, addr, "other-client", nil)
	defer other.Close()

	// 两个客户端都订阅同一主题
	err := target.Subscribe("direct/topic")
	if err != nil {
		t.Fatalf("Target failed to subscribe: %v", err)
	}
	err = other.Subscribe("direct/topic")
	if err != nil {
		t.Fatalf("Other failed to subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 定向发布给 target-client
	err = publisher.PublishToClient("target-client", "direct/topic", []byte("only for you"), nil)
	if err != nil {
		t.Fatalf("Failed to publish to client: %v", err)
	}

	// target 应该收到消息
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	msg, err := target.PollMessage(ctx, 2*time.Second)
	if err != nil {
		t.Fatalf("Target failed to receive: %v", err)
	}
	if string(msg.Packet.Payload) != "only for you" {
		t.Errorf("Target got wrong payload: %s", string(msg.Packet.Payload))
	}

	// other 不应该收到消息
	ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel2()

	_, err = other.PollMessage(ctx2, 300*time.Millisecond)
	if err == nil {
		t.Error("Other should not receive directed message")
	}
}

// ============================================================================
// 保留消息测试
// ============================================================================

// TestRetain_Basic 测试基本保留消息
func TestRetain_Basic(t *testing.T) {
	opts := NewCoreOptions().WithRetainEnabled(true)
	core := testSetupCore(t, opts)
	defer core.Stop()

	addr := core.GetAddress()

	// 发布保留消息
	publisher := testSetupClient(t, addr, "pub-retain", nil)
	defer publisher.Close()

	retainOpts := client.NewPublishOptions().WithRetain(true)
	err := publisher.Publish("retain/topic", []byte("retained message"), retainOpts)
	if err != nil {
		t.Fatalf("Failed to publish retain message: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 新订阅者应立即收到保留消息
	subscriber := testSetupClient(t, addr, "sub-retain", nil)
	defer subscriber.Close()

	// 启动轮询协程（先初始化消息队列）
	msgCh := make(chan *client.Message, 1)
	errCh := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		msg, err := subscriber.PollMessage(ctx, 4*time.Second)
		if err != nil {
			errCh <- err
		} else {
			msgCh <- msg
		}
	}()

	// 等待轮询协程启动
	time.Sleep(100 * time.Millisecond)

	// 现在订阅，保留消息应该被投递
	err = subscriber.Subscribe("retain/topic")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// 等待接收保留消息
	select {
	case msg := <-msgCh:
		if string(msg.Packet.Payload) != "retained message" {
			t.Errorf("Expected 'retained message', got '%s'", string(msg.Packet.Payload))
		}
	case err := <-errCh:
		t.Fatalf("Failed to receive retain message: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for retain message")
	}
}

// TestRetain_Update 测试更新保留消息
func TestRetain_Update(t *testing.T) {
	opts := NewCoreOptions().WithRetainEnabled(true)
	core := testSetupCore(t, opts)
	defer core.Stop()

	addr := core.GetAddress()

	publisher := testSetupClient(t, addr, "pub-retain-update", nil)
	defer publisher.Close()

	retainOpts := client.NewPublishOptions().WithRetain(true)

	// 发布初始保留消息
	err := publisher.Publish("retain/update", []byte("version1"), retainOpts)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// 更新保留消息
	err = publisher.Publish("retain/update", []byte("version2"), retainOpts)
	if err != nil {
		t.Fatalf("Failed to publish update: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 新订阅者应收到最新版本
	subscriber := testSetupClient(t, addr, "sub-retain-update", nil)
	defer subscriber.Close()

	// 启动轮询协程
	msgCh := make(chan *client.Message, 1)
	errCh := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		msg, err := subscriber.PollMessage(ctx, 4*time.Second)
		if err != nil {
			errCh <- err
		} else {
			msgCh <- msg
		}
	}()

	time.Sleep(100 * time.Millisecond)

	err = subscriber.Subscribe("retain/update")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	select {
	case msg := <-msgCh:
		if string(msg.Packet.Payload) != "version2" {
			t.Errorf("Expected 'version2', got '%s'", string(msg.Packet.Payload))
		}
	case err := <-errCh:
		t.Fatalf("Failed to receive: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for retain message")
	}
}

// TestRetain_WildcardMatch 测试通配符匹配保留消息
func TestRetain_WildcardMatch(t *testing.T) {
	opts := NewCoreOptions().WithRetainEnabled(true)
	core := testSetupCore(t, opts)
	defer core.Stop()

	addr := core.GetAddress()

	publisher := testSetupClient(t, addr, "pub-retain-wild", nil)
	defer publisher.Close()

	retainOpts := client.NewPublishOptions().WithRetain(true)

	// 发布多个保留消息
	topics := []string{"device/sensor1/temp", "device/sensor2/temp", "device/sensor3/temp"}
	for i, topic := range topics {
		err := publisher.Publish(topic, []byte("temp"+string(rune('1'+i))), retainOpts)
		if err != nil {
			t.Fatalf("Failed to publish to %s: %v", topic, err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	// 使用通配符订阅（使用 * 代替 MQTT 的 +）
	subscriber := testSetupClient(t, addr, "sub-retain-wild", nil)
	defer subscriber.Close()

	// 启动轮询协程
	msgCh := make(chan *client.Message, 5)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 3; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			msg, err := subscriber.PollMessage(ctx, 4*time.Second)
			cancel()
			if err != nil {
				return
			}
			msgCh <- msg
		}
	}()

	time.Sleep(100 * time.Millisecond)

	err := subscriber.Subscribe("device/*/temp")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// 应收到所有匹配的保留消息
	received := make(map[string]string)
	timeout := time.After(5 * time.Second)
	for len(received) < 3 {
		select {
		case msg := <-msgCh:
			received[msg.Packet.Topic] = string(msg.Packet.Payload)
		case <-timeout:
			t.Fatalf("Timeout waiting for messages, received %d of 3", len(received))
		}
	}

	for _, topic := range topics {
		if _, ok := received[topic]; !ok {
			t.Errorf("Did not receive retain message for: %s", topic)
		}
	}
}

// ============================================================================
// 遗嘱消息测试
// ============================================================================

// TestWill_Basic 测试基本遗嘱消息
func TestWill_Basic(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建监听遗嘱消息的订阅者
	watcher := testSetupClient(t, addr, "will-watcher", nil)
	defer watcher.Close()

	err := watcher.Subscribe("client/*/status")
	if err != nil {
		t.Fatalf("Watcher failed to subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 创建带遗嘱消息的客户端
	willClient, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("will-client").
			WithLogger(testIntegrationLogger()).
			WithWillSimple("client/will-client/status", []byte("offline"), packet.QoS0, false),
	)
	if err != nil {
		t.Fatalf("Failed to create will client: %v", err)
	}

	err = willClient.Connect()
	if err != nil {
		t.Fatalf("Failed to connect will client: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 强制断开连接（模拟异常断开）
	willClient.ForceClose()

	// 监听者应收到遗嘱消息
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msg, err := watcher.PollMessage(ctx, 4*time.Second)
	if err != nil {
		t.Fatalf("Failed to receive will message: %v", err)
	}

	if msg.Packet.Topic != "client/will-client/status" {
		t.Errorf("Expected topic 'client/will-client/status', got '%s'", msg.Packet.Topic)
	}
	if string(msg.Packet.Payload) != "offline" {
		t.Errorf("Expected payload 'offline', got '%s'", string(msg.Packet.Payload))
	}
}

// TestWill_NormalDisconnect 测试正常断开不发送遗嘱
func TestWill_NormalDisconnect(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建监听者
	watcher := testSetupClient(t, addr, "will-watcher2", nil)
	defer watcher.Close()

	err := watcher.Subscribe("client/*/status")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 创建带遗嘱消息的客户端
	willClient, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("will-client2").
			WithLogger(testIntegrationLogger()).
			WithWillSimple("client/will-client2/status", []byte("offline"), packet.QoS0, false),
	)
	if err != nil {
		t.Fatalf("Failed to create will client: %v", err)
	}

	err = willClient.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 正常断开连接
	willClient.Disconnect()
	willClient.Close()

	// 监听者不应收到遗嘱消息
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err = watcher.PollMessage(ctx, 300*time.Millisecond)
	if err == nil {
		t.Error("Should not receive will message on normal disconnect")
	}
}

// TestWill_WithQoS1 测试 QoS1 遗嘱消息
func TestWill_WithQoS1(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建监听者（使用 QoS1 订阅）
	watcher := testSetupClient(t, addr, "will-watcher-qos1", nil)
	defer watcher.Close()

	err := watcher.SubscribeWithOptions([]string{"client/*/qos1status"}, client.NewSubscribeOptions().WithQoS(packet.QoS1))
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 创建带 QoS1 遗嘱的客户端
	willClient, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("will-qos1-client").
			WithLogger(testIntegrationLogger()).
			WithWillSimple("client/will-qos1-client/qos1status", []byte("qos1-offline"), packet.QoS1, false),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	err = willClient.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 强制断开
	willClient.ForceClose()

	// 接收遗嘱消息
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msg, err := watcher.PollMessage(ctx, 4*time.Second)
	if err != nil {
		t.Fatalf("Failed to receive will message: %v", err)
	}

	if string(msg.Packet.Payload) != "qos1-offline" {
		t.Errorf("Expected 'qos1-offline', got '%s'", string(msg.Packet.Payload))
	}
}

// ============================================================================
// 消息过期测试
// ============================================================================

// TestExpiry_MessageExpires 测试消息过期
func TestExpiry_MessageExpires(t *testing.T) {
	// 设置短的默认过期时间（但过期检查间隔最小 10s）
	opts := NewCoreOptions().
		WithDefaultMessageExpiry(1 * time.Second).
		WithExpiredCheckInterval(10 * time.Second)
	core := testSetupCore(t, opts)
	defer core.Stop()

	addr := core.GetAddress()

	publisher := testSetupClient(t, addr, "pub-expiry", nil)
	defer publisher.Close()

	// 发布带短过期时间的消息
	expiryOpts := client.NewPublishOptions().WithExpiry(1) // 1秒过期
	err := publisher.Publish("expiry/test", []byte("will expire"), expiryOpts)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// 等待消息过期
	time.Sleep(2 * time.Second)

	// 新订阅者不应收到过期消息
	subscriber := testSetupClient(t, addr, "sub-expiry", nil)
	defer subscriber.Close()

	err = subscriber.Subscribe("expiry/test")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err = subscriber.PollMessage(ctx, 300*time.Millisecond)
	if err == nil {
		t.Error("Should not receive expired message")
	}
}

// TestExpiry_CustomExpiry 测试自定义过期时间
func TestExpiry_CustomExpiry(t *testing.T) {
	opts := NewCoreOptions().
		WithDefaultMessageExpiry(10 * time.Second) // 默认较长
	core := testSetupCore(t, opts)
	defer core.Stop()

	addr := core.GetAddress()

	publisher := testSetupClient(t, addr, "pub-custom-expiry", nil)
	defer publisher.Close()

	subscriber := testSetupClient(t, addr, "sub-custom-expiry", nil)
	defer subscriber.Close()

	err := subscriber.Subscribe("custom/expiry")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 发布带自定义长过期时间的消息
	longExpiry := client.NewPublishOptions().WithExpiry(60) // 60秒
	err = publisher.Publish("custom/expiry", []byte("long lived"), longExpiry)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// 应能收到消息
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	msg, err := subscriber.PollMessage(ctx, 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to receive: %v", err)
	}

	if string(msg.Packet.Payload) != "long lived" {
		t.Errorf("Expected 'long lived', got '%s'", string(msg.Packet.Payload))
	}
}

// ============================================================================
// Hooks 测试
// ============================================================================

// TestHooks_OnConnect 测试连接钩子
func TestHooks_OnConnect(t *testing.T) {
	var connectedClients []string
	var disconnectedClients []string
	var mu sync.Mutex

	coreOpts := NewCoreOptions().
		WithConnectionHandler(&ConnectionHandlerFunc{
			ConnectFunc: func(ctx *ConnectContext) {
				mu.Lock()
				connectedClients = append(connectedClients, ctx.ClientID)
				mu.Unlock()
			},
			DisconnectFunc: func(ctx *DisconnectContext) {
				mu.Lock()
				disconnectedClients = append(disconnectedClients, ctx.ClientID)
				mu.Unlock()
			},
		})

	core := testSetupCore(t, coreOpts)
	defer core.Stop()

	addr := core.GetAddress()

	// 连接多个客户端
	clients := make([]*client.Client, 3)
	for i := 0; i < 3; i++ {
		clients[i] = testSetupClient(t, addr, "hook-client"+string(rune('A'+i)), nil)
	}

	time.Sleep(100 * time.Millisecond)

	// 检查连接钩子被调用
	mu.Lock()
	if len(connectedClients) != 3 {
		t.Errorf("Expected 3 connected clients, got %d", len(connectedClients))
	}
	mu.Unlock()

	// 断开连接
	for _, c := range clients {
		c.Close()
	}

	time.Sleep(200 * time.Millisecond)

	// 检查断开钩子被调用
	mu.Lock()
	if len(disconnectedClients) != 3 {
		t.Errorf("Expected 3 disconnected clients, got %d", len(disconnectedClients))
	}
	mu.Unlock()
}

// TestHooks_OnPublish 测试发布钩子
func TestHooks_OnPublish(t *testing.T) {
	var publishedCount atomic.Int32

	coreOpts := NewCoreOptions().
		WithMessageHandler(&MessageHandlerFunc{
			PublishFunc: func(ctx *PublishContext) error {
				publishedCount.Add(1)
				// 拒绝发布到 "forbidden" 主题
				if ctx.Packet.Topic == "forbidden" {
					return errors.New("topic forbidden")
				}
				return nil
			},
		})

	core := testSetupCore(t, coreOpts)
	defer core.Stop()

	addr := core.GetAddress()

	publisher := testSetupClient(t, addr, "hook-pub", nil)
	defer publisher.Close()

	subscriber := testSetupClient(t, addr, "hook-sub", nil)
	defer subscriber.Close()

	err := subscriber.Subscribe("allowed")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	err = subscriber.Subscribe("forbidden")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 发布到允许的主题
	err = publisher.Publish("allowed", []byte("ok"), nil)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// 发布到禁止的主题
	err = publisher.Publish("forbidden", []byte("no"), nil)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// 应该只收到 allowed 主题的消息
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	msg, err := subscriber.PollMessage(ctx, 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to receive: %v", err)
	}

	if msg.Packet.Topic != "allowed" {
		t.Errorf("Expected topic 'allowed', got '%s'", msg.Packet.Topic)
	}

	// 不应再收到消息
	ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel2()

	_, err = subscriber.PollMessage(ctx2, 300*time.Millisecond)
	if err == nil {
		t.Error("Should not receive message from forbidden topic")
	}

	// 钩子应被调用两次
	if publishedCount.Load() != 2 {
		t.Errorf("Expected 2 publish hook calls, got %d", publishedCount.Load())
	}
}

// TestHooks_OnDeliver 测试投递钩子
func TestHooks_OnDeliver(t *testing.T) {
	coreOpts := NewCoreOptions().
		WithMessageHandler(&MessageHandlerFunc{
			DeliverFunc: func(ctx *DeliverContext) bool {
				// 只投递给 "vip-" 前缀的客户端
				return len(ctx.ClientID) >= 4 && ctx.ClientID[:4] == "vip-"
			},
		})

	core := testSetupCore(t, coreOpts)
	defer core.Stop()

	addr := core.GetAddress()

	publisher := testSetupClient(t, addr, "deliver-pub", nil)
	defer publisher.Close()

	vipSub := testSetupClient(t, addr, "vip-sub", nil)
	defer vipSub.Close()

	normalSub := testSetupClient(t, addr, "normal-sub", nil)
	defer normalSub.Close()

	err := vipSub.Subscribe("news")
	if err != nil {
		t.Fatalf("VIP failed to subscribe: %v", err)
	}
	err = normalSub.Subscribe("news")
	if err != nil {
		t.Fatalf("Normal failed to subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 发布消息
	err = publisher.Publish("news", []byte("breaking news"), nil)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// VIP 应收到消息
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	msg, err := vipSub.PollMessage(ctx, 2*time.Second)
	if err != nil {
		t.Fatalf("VIP failed to receive: %v", err)
	}
	if string(msg.Packet.Payload) != "breaking news" {
		t.Errorf("VIP got wrong payload: %s", string(msg.Packet.Payload))
	}

	// Normal 不应收到消息
	ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel2()

	_, err = normalSub.PollMessage(ctx2, 300*time.Millisecond)
	if err == nil {
		t.Error("Normal should not receive message")
	}
}

// TestHooks_OnSubscribe 测试订阅钩子
func TestHooks_OnSubscribe(t *testing.T) {
	coreOpts := NewCoreOptions().
		WithSubscriptionHandler(&SubscriptionHandlerFunc{
			SubscribeFunc: func(ctx *SubscribeContext) error {
				// 禁止订阅 "admin/#" 主题
				if len(ctx.Subscription.Topic) >= 6 && ctx.Subscription.Topic[:6] == "admin/" {
					return errors.New("admin topics forbidden")
				}
				return nil
			},
		})

	core := testSetupCore(t, coreOpts)
	defer core.Stop()

	addr := core.GetAddress()

	c := testSetupClient(t, addr, "sub-hook-client", nil)
	defer c.Close()

	// 订阅普通主题应成功
	err := c.Subscribe("user/data")
	if err != nil {
		t.Errorf("Should allow subscribing to user/data: %v", err)
	}

	// 订阅管理员主题应失败
	err = c.Subscribe("admin/settings")
	if err == nil {
		t.Error("Should reject subscribing to admin topics")
	}
}

// TestHooks_ClientOnMessage 测试客户端消息钩子
func TestHooks_ClientOnMessage(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	publisher := testSetupClient(t, addr, "client-hook-pub", nil)
	defer publisher.Close()

	var receivedCount atomic.Int32
	var filteredOut atomic.Int32

	// 创建带消息钩子的客户端
	subOpts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("client-hook-sub").
		WithLogger(testIntegrationLogger()).
		WithMessageHandler(client.MessageHandlerFunc(func(ctx *client.PublishContext) bool {
			receivedCount.Add(1)
			// 过滤掉 payload 为 "filter-me" 的消息
			if string(ctx.Packet.Payload) == "filter-me" {
				filteredOut.Add(1)
				return false
			}
			return true
		}))

	subscriber, err := client.NewWithOptions(subOpts)
	if err != nil {
		t.Fatalf("Failed to create subscriber: %v", err)
	}
	err = subscriber.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer subscriber.Close()

	err = subscriber.Subscribe("hook/test")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 发布多条消息
	publisher.Publish("hook/test", []byte("keep-me"), nil)
	publisher.Publish("hook/test", []byte("filter-me"), nil)
	publisher.Publish("hook/test", []byte("also-keep"), nil)

	// 接收消息
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	received := make([]string, 0)
	for i := 0; i < 2; i++ {
		msg, err := subscriber.PollMessage(ctx, 2*time.Second)
		if err != nil {
			break
		}
		received = append(received, string(msg.Packet.Payload))
	}

	// 验证
	if len(received) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(received))
	}

	if receivedCount.Load() != 3 {
		t.Errorf("Expected 3 hook calls, got %d", receivedCount.Load())
	}

	if filteredOut.Load() != 1 {
		t.Errorf("Expected 1 filtered message, got %d", filteredOut.Load())
	}
}

// ============================================================================
// 连接管理测试
// ============================================================================

// TestConnection_Reconnect 测试重连
func TestConnection_Reconnect(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建客户端（autoReconnect 在 Connect 成功后自动启用）
	opts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("reconnect-client").
		WithLogger(testIntegrationLogger())

	c, err := client.NewWithOptions(opts)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	err = c.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// 订阅
	err = c.Subscribe("reconnect/test")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// 强制断开
	c.ForceClose()

	// 等待重连
	time.Sleep(2 * time.Second)

	// 应该已重连
	if !c.IsConnected() {
		t.Error("Client should be reconnected")
	}
}

// TestConnection_MaxClients 测试最大客户端限制
func TestConnection_MaxClients(t *testing.T) {
	opts := NewCoreOptions().WithMaxClients(2)
	core := testSetupCore(t, opts)
	defer core.Stop()

	addr := core.GetAddress()

	// 连接两个客户端（应该成功）
	c1 := testSetupClient(t, addr, "max-client1", nil)
	defer c1.Close()

	c2 := testSetupClient(t, addr, "max-client2", nil)
	defer c2.Close()

	// 第三个客户端应该失败
	c3Opts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("max-client3").
		WithLogger(testIntegrationLogger())

	c3, err := client.NewWithOptions(c3Opts)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer c3.Close()

	err = c3.Connect()
	if err == nil {
		t.Error("Third client should fail to connect")
	}
}

// TestConnection_ClientTakeover 测试客户端接管
func TestConnection_ClientTakeover(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 第一个客户端连接
	c1, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("takeover-client").
			WithLogger(testIntegrationLogger()),
	)
	if err != nil {
		t.Fatalf("Failed to create c1: %v", err)
	}

	err = c1.Connect()
	if err != nil {
		t.Fatalf("Failed to connect c1: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 第二个客户端使用相同 ID 连接
	c2, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("takeover-client").
			WithLogger(testIntegrationLogger()),
	)
	if err != nil {
		t.Fatalf("Failed to create c2: %v", err)
	}

	err = c2.Connect()
	if err != nil {
		t.Fatalf("Failed to connect c2: %v", err)
	}
	defer c2.Close()

	time.Sleep(100 * time.Millisecond)

	// 第一个客户端应被断开
	if c1.IsConnected() {
		t.Error("First client should be disconnected")
	}

	// 第二个客户端应保持连接
	if !c2.IsConnected() {
		t.Error("Second client should stay connected")
	}

	c1.Close()
}

// ============================================================================
// 统计信息测试
// ============================================================================

// TestStats_Basic 测试基本统计信息
func TestStats_Basic(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 初始状态
	stats := core.GetStats()
	if stats.ClientsConnected != 0 {
		t.Errorf("Initial ClientsConnected should be 0, got %d", stats.ClientsConnected)
	}

	// 连接客户端
	c1 := testSetupClient(t, addr, "stats-client1", nil)
	c2 := testSetupClient(t, addr, "stats-client2", nil)

	time.Sleep(100 * time.Millisecond)

	stats = core.GetStats()
	if stats.ClientsConnected != 2 {
		t.Errorf("Expected 2 connected clients, got %d", stats.ClientsConnected)
	}

	// 订阅
	c1.Subscribe("stats/topic")
	c2.Subscribe("stats/topic")

	time.Sleep(50 * time.Millisecond)

	stats = core.GetStats()
	if stats.SubscriptionsCount < 2 {
		t.Errorf("Expected at least 2 subscriptions, got %d", stats.SubscriptionsCount)
	}

	// 断开一个客户端
	c1.Close()

	time.Sleep(100 * time.Millisecond)

	stats = core.GetStats()
	if stats.ClientsConnected != 1 {
		t.Errorf("Expected 1 connected client after disconnect, got %d", stats.ClientsConnected)
	}

	c2.Close()
}

// ============================================================================
// 管理 API 测试
// ============================================================================

// TestManagement_ListClients 测试列出客户端
func TestManagement_ListClients(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	c1 := testSetupClient(t, addr, "list-client1", nil)
	defer c1.Close()
	c2 := testSetupClient(t, addr, "list-client2", nil)
	defer c2.Close()

	time.Sleep(100 * time.Millisecond)

	clients := core.ListClients()
	if len(clients) != 2 {
		t.Errorf("Expected 2 clients, got %d", len(clients))
	}

	clientIDs := make(map[string]bool)
	for _, info := range clients {
		clientIDs[info.ClientID] = true
	}

	if !clientIDs["list-client1"] {
		t.Error("Missing list-client1")
	}
	if !clientIDs["list-client2"] {
		t.Error("Missing list-client2")
	}
}

// TestManagement_GetClientInfo 测试获取客户端信息
func TestManagement_GetClientInfo(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	c := testSetupClient(t, addr, "info-client", nil)
	defer c.Close()

	err := c.Subscribe("info/topic1", "info/topic2")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	info, err := core.GetClientInfo("info-client")
	if err != nil {
		t.Fatalf("Failed to get client info: %v", err)
	}

	if info.ClientID != "info-client" {
		t.Errorf("Expected ClientID 'info-client', got '%s'", info.ClientID)
	}

	if !info.Connected {
		t.Error("Client should be connected")
	}

	if len(info.Subscriptions) != 2 {
		t.Errorf("Expected 2 subscriptions, got %d", len(info.Subscriptions))
	}
}

// TestManagement_DisconnectClient 测试断开客户端
func TestManagement_DisconnectClient(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	c := testSetupClient(t, addr, "disconnect-client", nil)
	defer c.Close()

	time.Sleep(100 * time.Millisecond)

	// 管理员断开客户端
	err := core.DisconnectClient("disconnect-client", packet.ReasonServerBusy)
	if err != nil {
		t.Fatalf("Failed to disconnect: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if c.IsConnected() {
		t.Error("Client should be disconnected")
	}
}

// TestManagement_GetTopicSubscribers 测试获取主题订阅者
func TestManagement_GetTopicSubscribers(t *testing.T) {
	core := testSetupCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	c1 := testSetupClient(t, addr, "topic-sub1", nil)
	defer c1.Close()
	c2 := testSetupClient(t, addr, "topic-sub2", nil)
	defer c2.Close()
	c3 := testSetupClient(t, addr, "topic-sub3", nil)
	defer c3.Close()

	c1.Subscribe("shared/topic")
	c2.Subscribe("shared/topic")
	c3.Subscribe("other/topic")

	time.Sleep(100 * time.Millisecond)

	subscribers := core.GetTopicSubscribers("shared/topic")
	if len(subscribers) != 2 {
		t.Errorf("Expected 2 subscribers, got %d", len(subscribers))
	}
}

// ============================================================================
// 保留消息高级测试
// ============================================================================

// TestRetain_EmptyPayloadClear 测试空载荷清除保留消息
func TestRetain_EmptyPayloadClear(t *testing.T) {
	opts := NewCoreOptions().WithRetainEnabled(true)
	core := testSetupCore(t, opts)
	defer core.Stop()

	addr := core.GetAddress()

	publisher := testSetupClient(t, addr, "pub-retain-clear", nil)
	defer publisher.Close()

	retainOpts := client.NewPublishOptions().WithRetain(true)

	// 发布保留消息
	err := publisher.Publish("retain/clear", []byte("data"), retainOpts)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 发布空载荷保留消息（应该清除）
	err = publisher.Publish("retain/clear", []byte{}, retainOpts)
	if err != nil {
		t.Fatalf("Failed to publish empty: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 新订阅者不应收到任何保留消息
	subscriber := testSetupClient(t, addr, "sub-retain-clear", nil)
	defer subscriber.Close()

	msgCh := make(chan *client.Message, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		msg, err := subscriber.PollMessage(ctx, 1*time.Second)
		if err == nil {
			msgCh <- msg
		}
	}()

	time.Sleep(100 * time.Millisecond)

	err = subscriber.Subscribe("retain/clear")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// 不应收到任何消息
	select {
	case msg := <-msgCh:
		t.Errorf("Should not receive message, got: %s", string(msg.Packet.Payload))
	case <-time.After(2 * time.Second):
		// 正确：超时表示没有收到消息
	}
}

// TestRetain_MultiLevelWildcard 测试多层通配符
func TestRetain_MultiLevelWildcard(t *testing.T) {
	opts := NewCoreOptions().WithRetainEnabled(true)
	core := testSetupCore(t, opts)
	defer core.Stop()

	addr := core.GetAddress()

	publisher := testSetupClient(t, addr, "pub-retain-multi", nil)
	defer publisher.Close()

	retainOpts := client.NewPublishOptions().WithRetain(true)

	// 发布多个层级的保留消息
	topics := []string{
		"home/room1/temperature",
		"home/room1/humidity",
		"home/room2/temperature",
		"home/room2/humidity",
		"home/room3/light/brightness",
		"home/room3/light/color",
	}

	for i, topic := range topics {
		err := publisher.Publish(topic, []byte{byte('a' + i)}, retainOpts)
		if err != nil {
			t.Fatalf("Failed to publish to %s: %v", topic, err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	// 使用 ** 订阅所有
	subscriber := testSetupClient(t, addr, "sub-retain-multi", nil)
	defer subscriber.Close()

	msgCh := make(chan *client.Message, 10)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 6; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			msg, err := subscriber.PollMessage(ctx, 4*time.Second)
			cancel()
			if err != nil {
				return
			}
			msgCh <- msg
		}
	}()

	time.Sleep(100 * time.Millisecond)

	err := subscriber.Subscribe("home/**")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// 应收到所有6个保留消息
	received := 0
	timeout := time.After(5 * time.Second)
	for received < 6 {
		select {
		case <-msgCh:
			received++
		case <-timeout:
			t.Fatalf("Timeout waiting for messages, received %d of 6", received)
		}
	}

	if received != 6 {
		t.Errorf("Expected 6 messages, got %d", received)
	}
}

// TestRetain_QoSDowngrade 测试保留消息的 QoS 降级
func TestRetain_QoSDowngrade(t *testing.T) {
	opts := NewCoreOptions().WithRetainEnabled(true)
	core := testSetupCore(t, opts)
	defer core.Stop()

	addr := core.GetAddress()

	publisher := testSetupClient(t, addr, "pub-retain-qos", nil)
	defer publisher.Close()

	// 发布 QoS1 保留消息
	retainOpts := client.NewPublishOptions().WithRetain(true).WithQoS(packet.QoS1)
	err := publisher.Publish("retain/qos", []byte("qos1 message"), retainOpts)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 订阅者使用 QoS0 订阅
	subscriber := testSetupClient(t, addr, "sub-retain-qos", nil)
	defer subscriber.Close()

	msgCh := make(chan *client.Message, 1)
	errCh := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		msg, err := subscriber.PollMessage(ctx, 4*time.Second)
		if err != nil {
			errCh <- err
		} else {
			msgCh <- msg
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// QoS0 订阅
	subOpts := client.NewSubscribeOptions().WithQoS(packet.QoS0)
	err = subscriber.SubscribeWithOptions([]string{"retain/qos"}, subOpts)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// 应收到 QoS0 的消息（降级）
	select {
	case msg := <-msgCh:
		if string(msg.Packet.Payload) != "qos1 message" {
			t.Errorf("Payload mismatch: got %s", string(msg.Packet.Payload))
		}
		// QoS 应该被降级为 0
		if msg.Packet.QoS != packet.QoS0 {
			t.Errorf("Expected QoS0, got QoS%d", msg.Packet.QoS)
		}
	case err := <-errCh:
		t.Fatalf("Failed to receive: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

// TestRetain_Persistence 测试保留消息持久化
func TestRetain_Persistence(t *testing.T) {
	// 创建临时目录用于持久化
	tmpDir := t.TempDir()

	opts := NewCoreOptions().
		WithRetainEnabled(true).
		WithStoreDir(tmpDir)

	core := testSetupCore(t, opts)
	addr := core.GetAddress()

	publisher := testSetupClient(t, addr, "pub-persist", nil)

	retainOpts := client.NewPublishOptions().WithRetain(true)
	err := publisher.Publish("persist/topic", []byte("persistent data"), retainOpts)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	publisher.Close()
	core.Stop()

	// 重启 core
	core2 := testSetupCore(t, opts)
	defer core2.Stop()

	addr2 := core2.GetAddress()

	// 新订阅者应收到持久化的保留消息
	subscriber := testSetupClient(t, addr2, "sub-persist", nil)
	defer subscriber.Close()

	msgCh := make(chan *client.Message, 1)
	errCh := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		msg, err := subscriber.PollMessage(ctx, 4*time.Second)
		if err != nil {
			errCh <- err
		} else {
			msgCh <- msg
		}
	}()

	time.Sleep(100 * time.Millisecond)

	err = subscriber.Subscribe("persist/topic")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	select {
	case msg := <-msgCh:
		if string(msg.Packet.Payload) != "persistent data" {
			t.Errorf("Payload mismatch: got %s", string(msg.Packet.Payload))
		}
	case err := <-errCh:
		t.Fatalf("Failed to receive: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for persisted message")
	}
}

// TestRetain_ConcurrentPublish 测试并发发布保留消息
func TestRetain_ConcurrentPublish(t *testing.T) {
	opts := NewCoreOptions().WithRetainEnabled(true)
	core := testSetupCore(t, opts)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建多个发布者
	numPublishers := 10
	var wg sync.WaitGroup
	wg.Add(numPublishers)

	retainOpts := client.NewPublishOptions().WithRetain(true)

	for i := 0; i < numPublishers; i++ {
		go func(id int) {
			defer wg.Done()
			publisher := testSetupClient(t, addr, "pub-concurrent-"+string(rune('a'+id)), nil)
			defer publisher.Close()

			topic := "concurrent/test/" + string(rune('a'+id))
			err := publisher.Publish(topic, []byte{byte(id)}, retainOpts)
			if err != nil {
				t.Errorf("Publisher %d failed: %v", id, err)
			}

			time.Sleep(100 * time.Millisecond)
		}(i)
	}

	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	// 订阅所有并验证
	subscriber := testSetupClient(t, addr, "sub-concurrent", nil)
	defer subscriber.Close()

	msgCh := make(chan *client.Message, 20)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < numPublishers; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			msg, err := subscriber.PollMessage(ctx, 4*time.Second)
			cancel()
			if err != nil {
				return
			}
			msgCh <- msg
		}
	}()

	time.Sleep(100 * time.Millisecond)

	err := subscriber.Subscribe("concurrent/**")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	received := 0
	timeout := time.After(10 * time.Second)
	for received < numPublishers {
		select {
		case <-msgCh:
			received++
		case <-timeout:
			t.Fatalf("Timeout, received %d of %d messages", received, numPublishers)
		}
	}

	if received != numPublishers {
		t.Errorf("Expected %d messages, got %d", numPublishers, received)
	}
}
