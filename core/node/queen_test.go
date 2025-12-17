package node

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/danclive/nson-go"
	queen "snple.com/queen/core"
	"snple.com/queen/pkg/protocol"
)

// TestInternalClientCreation 测试内部客户端创建
func TestInternalClientCreation(t *testing.T) {
	broker, err := queen.New(
		queen.WithAddress(":0"),
	)
	if err != nil {
		t.Fatalf("Failed to create broker: %v", err)
	}
	defer broker.Stop()

	if err := broker.Start(); err != nil {
		t.Fatalf("Failed to start broker: %v", err)
	}

	config := queen.InternalClientConfig{
		ClientID:   "test-client",
		BufferSize: 100,
	}
	ic, err := broker.NewInternalClient(config)
	if err != nil {
		t.Fatalf("Failed to create internal client: %v", err)
	}
	defer ic.Close()

	if ic.ID != "test-client" {
		t.Errorf("Expected client ID 'test-client', got '%s'", ic.ID)
	}
}

// TestInternalClientSubscribeAndReceive 测试内部客户端订阅和接收消息
func TestInternalClientSubscribeAndReceive(t *testing.T) {
	broker, err := queen.New(
		queen.WithAddress(":0"),
	)
	if err != nil {
		t.Fatalf("Failed to create broker: %v", err)
	}
	defer broker.Stop()

	if err := broker.Start(); err != nil {
		t.Fatalf("Failed to start broker: %v", err)
	}

	subscriber, err := broker.NewInternalClient(queen.InternalClientConfig{
		ClientID:   "subscriber",
		BufferSize: 10,
	})
	if err != nil {
		t.Fatalf("Failed to create subscriber: %v", err)
	}
	defer subscriber.Close()

	if err := subscriber.Subscribe("test/topic", protocol.QoS1); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	publisher, err := broker.NewInternalClient(queen.InternalClientConfig{
		ClientID:   "publisher",
		BufferSize: 10,
	})
	if err != nil {
		t.Fatalf("Failed to create publisher: %v", err)
	}
	defer publisher.Close()

	testPayload := []byte("hello world")
	if err := publisher.Publish("test/topic", testPayload, queen.PublishOptions{
		QoS: protocol.QoS1,
	}); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	msg, err := subscriber.ReceiveWithTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to receive message: %v", err)
	}

	if msg.Topic != "test/topic" {
		t.Errorf("Expected topic 'test/topic', got '%s'", msg.Topic)
	}

	if !bytes.Equal(msg.Payload, testPayload) {
		t.Errorf("Expected payload %v, got %v", testPayload, msg.Payload)
	}

	if msg.SourceClientID != "publisher" {
		t.Errorf("Expected source client ID 'publisher', got '%s'", msg.SourceClientID)
	}
}

// TestBeaconTopicSubscription 测试 beacon 主题订阅
func TestBeaconTopicSubscription(t *testing.T) {
	broker, err := queen.New(
		queen.WithAddress(":0"),
	)
	if err != nil {
		t.Fatalf("Failed to create broker: %v", err)
	}
	defer broker.Stop()

	if err := broker.Start(); err != nil {
		t.Fatalf("Failed to start broker: %v", err)
	}

	ic, err := broker.NewInternalClient(queen.InternalClientConfig{
		ClientID:   "beacon-core",
		BufferSize: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create internal client: %v", err)
	}
	defer ic.Close()

	topics := []string{
		TopicPush,
		TopicPinValue,
		TopicPinValueSet,
		TopicPinWritePull,
	}

	for _, topic := range topics {
		if err := ic.Subscribe(topic, protocol.QoS1); err != nil {
			t.Errorf("Failed to subscribe to %s: %v", topic, err)
		}
	}

	publisher, err := broker.NewInternalClient(queen.InternalClientConfig{
		ClientID:   "test-publisher",
		BufferSize: 10,
	})
	if err != nil {
		t.Fatalf("Failed to create publisher: %v", err)
	}
	defer publisher.Close()

	testPayload := []byte("test data")
	if err := publisher.Publish(TopicPush, testPayload, queen.PublishOptions{
		QoS: protocol.QoS1,
	}); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	msg, err := ic.ReceiveWithTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to receive message: %v", err)
	}

	if msg.Topic != TopicPush {
		t.Errorf("Expected topic '%s', got '%s'", TopicPush, msg.Topic)
	}
}

// TestRequestResponsePattern 测试 Request/Response 模式
func TestRequestResponsePattern(t *testing.T) {
	broker, err := queen.New(
		queen.WithAddress(":0"),
	)
	if err != nil {
		t.Fatalf("Failed to create broker: %v", err)
	}
	defer broker.Stop()

	if err := broker.Start(); err != nil {
		t.Fatalf("Failed to start broker: %v", err)
	}

	server, err := broker.NewInternalClient(queen.InternalClientConfig{
		ClientID:   "server",
		BufferSize: 10,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	if err := server.RegisterHandler("test.echo", func(req *queen.InternalRequest) ([]byte, protocol.ReasonCode, error) {
		return req.Payload, protocol.ReasonSuccess, nil
	}, 5); err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	client, err := broker.NewInternalClient(queen.InternalClientConfig{
		ClientID:   "client",
		BufferSize: 10,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	testPayload := []byte("hello")
	resp, err := client.Request(ctx, "test.echo", testPayload, &queen.InternalRequestOptions{
		Timeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	if resp.ReasonCode != protocol.ReasonSuccess {
		t.Errorf("Expected reason code Success, got %v", resp.ReasonCode)
	}

	if !bytes.Equal(resp.Payload, testPayload) {
		t.Errorf("Expected payload %v, got %v", testPayload, resp.Payload)
	}
}

// TestPinWritePayloadBuilding 测试构建 Pin 写入 payload
func TestPinWritePayloadBuilding(t *testing.T) {
	t.Run("EmptyWrites", func(t *testing.T) {
		writes := []PinWriteMessage{}
		payload := buildTestPinWritesPayload(t, writes)
		if payload != nil {
			t.Error("Expected nil payload for empty writes")
		}
	})

	t.Run("SingleWrite", func(t *testing.T) {
		writes := []PinWriteMessage{
			{
				Id:    "pin-1",
				Name:  "test.pin1",
				Value: nson.I32(42),
			},
		}
		payload := buildTestPinWritesPayload(t, writes)
		if payload == nil {
			t.Fatal("Expected non-nil payload")
		}

		buf := bytes.NewBuffer(payload)
		arr, err := nson.DecodeArray(buf)
		if err != nil {
			t.Fatalf("Failed to decode array: %v", err)
		}

		if len(arr) != 1 {
			t.Errorf("Expected 1 item, got %d", len(arr))
		}
	})

	t.Run("MultipleWrites", func(t *testing.T) {
		writes := []PinWriteMessage{
			{
				Id:    "pin-1",
				Name:  "test.pin1",
				Value: nson.I32(42),
			},
			{
				Id:    "pin-2",
				Name:  "test.pin2",
				Value: nson.F64(3.14),
			},
		}
		payload := buildTestPinWritesPayload(t, writes)
		if payload == nil {
			t.Fatal("Expected non-nil payload")
		}

		buf := bytes.NewBuffer(payload)
		arr, err := nson.DecodeArray(buf)
		if err != nil {
			t.Fatalf("Failed to decode array: %v", err)
		}

		if len(arr) != 2 {
			t.Errorf("Expected 2 items, got %d", len(arr))
		}
	})
}

func buildTestPinWritesPayload(t *testing.T, writes []PinWriteMessage) []byte {
	if len(writes) == 0 {
		return nil
	}

	arr := make(nson.Array, 0, len(writes))
	for _, w := range writes {
		m, err := nson.Marshal(w)
		if err != nil {
			t.Fatalf("Failed to marshal write: %v", err)
		}
		arr = append(arr, m)
	}

	buf := new(bytes.Buffer)
	if err := nson.EncodeArray(arr, buf); err != nil {
		t.Fatalf("Failed to encode array: %v", err)
	}

	return buf.Bytes()
}

// TestPinValueMessageSerialization 测试 PinValueMessage 序列化
func TestPinValueMessageSerialization(t *testing.T) {
	msg := PinValueMessage{
		Id:    "pin-123",
		Name:  "device.temperature",
		Value: nson.F64(25.5),
	}

	m, err := nson.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	buf := new(bytes.Buffer)
	if err := nson.EncodeMap(m, buf); err != nil {
		t.Fatalf("Failed to encode map: %v", err)
	}

	buf2 := bytes.NewBuffer(buf.Bytes())
	m2, err := nson.DecodeMap(buf2)
	if err != nil {
		t.Fatalf("Failed to decode map: %v", err)
	}

	var msg2 PinValueMessage
	if err := nson.Unmarshal(m2, &msg2); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if msg2.Id != msg.Id {
		t.Errorf("Expected Id '%s', got '%s'", msg.Id, msg2.Id)
	}

	if msg2.Name != msg.Name {
		t.Errorf("Expected Name '%s', got '%s'", msg.Name, msg2.Name)
	}
}

// TestMultipleInternalClients 测试多个内部客户端通信
func TestMultipleInternalClients(t *testing.T) {
	broker, err := queen.New(
		queen.WithAddress(":0"),
	)
	if err != nil {
		t.Fatalf("Failed to create broker: %v", err)
	}
	defer broker.Stop()

	if err := broker.Start(); err != nil {
		t.Fatalf("Failed to start broker: %v", err)
	}

	numSubscribers := 3
	subscribers := make([]*queen.InternalClient, numSubscribers)
	for i := 0; i < numSubscribers; i++ {
		ic, err := broker.NewInternalClient(queen.InternalClientConfig{
			ClientID:   "subscriber-" + string(rune('A'+i)),
			BufferSize: 10,
		})
		if err != nil {
			t.Fatalf("Failed to create subscriber %d: %v", i, err)
		}
		defer ic.Close()

		if err := ic.Subscribe("test/broadcast", protocol.QoS1); err != nil {
			t.Fatalf("Failed to subscribe: %v", err)
		}

		subscribers[i] = ic
	}

	publisher, err := broker.NewInternalClient(queen.InternalClientConfig{
		ClientID:   "broadcaster",
		BufferSize: 10,
	})
	if err != nil {
		t.Fatalf("Failed to create publisher: %v", err)
	}
	defer publisher.Close()

	testPayload := []byte("broadcast message")
	if err := publisher.Publish("test/broadcast", testPayload, queen.PublishOptions{
		QoS: protocol.QoS1,
	}); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	for i, sub := range subscribers {
		msg, err := sub.ReceiveWithTimeout(2 * time.Second)
		if err != nil {
			t.Errorf("Subscriber %d failed to receive: %v", i, err)
			continue
		}

		if !bytes.Equal(msg.Payload, testPayload) {
			t.Errorf("Subscriber %d: expected payload %v, got %v", i, testPayload, msg.Payload)
		}
	}
}

// TestRequestTimeout 测试请求超时
func TestRequestTimeout(t *testing.T) {
	broker, err := queen.New(
		queen.WithAddress(":0"),
	)
	if err != nil {
		t.Fatalf("Failed to create broker: %v", err)
	}
	defer broker.Stop()

	if err := broker.Start(); err != nil {
		t.Fatalf("Failed to start broker: %v", err)
	}

	server, err := broker.NewInternalClient(queen.InternalClientConfig{
		ClientID:   "slow-server",
		BufferSize: 10,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	if err := server.RegisterHandler("slow.action", func(req *queen.InternalRequest) ([]byte, protocol.ReasonCode, error) {
		time.Sleep(3 * time.Second)
		return []byte("too late"), protocol.ReasonSuccess, nil
	}, 1); err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	client, err := broker.NewInternalClient(queen.InternalClientConfig{
		ClientID:   "client",
		BufferSize: 10,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	_, err = client.Request(ctx, "slow.action", []byte("test"), &queen.InternalRequestOptions{
		Timeout: 1 * time.Second,
	})

	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

// BenchmarkMessagePublishSubscribe 基准测试：消息发布订阅
func BenchmarkMessagePublishSubscribe(b *testing.B) {
	broker, err := queen.New(
		queen.WithAddress(":0"),
	)
	if err != nil {
		b.Fatalf("Failed to create broker: %v", err)
	}
	defer broker.Stop()

	if err := broker.Start(); err != nil {
		b.Fatalf("Failed to start broker: %v", err)
	}

	subscriber, err := broker.NewInternalClient(queen.InternalClientConfig{
		ClientID:   "subscriber",
		BufferSize: 1000,
	})
	if err != nil {
		b.Fatalf("Failed to create subscriber: %v", err)
	}
	defer subscriber.Close()

	if err := subscriber.Subscribe("bench/topic", protocol.QoS1); err != nil {
		b.Fatalf("Failed to subscribe: %v", err)
	}

	publisher, err := broker.NewInternalClient(queen.InternalClientConfig{
		ClientID:   "publisher",
		BufferSize: 1000,
	})
	if err != nil {
		b.Fatalf("Failed to create publisher: %v", err)
	}
	defer publisher.Close()

	testPayload := []byte("benchmark payload")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := publisher.Publish("bench/topic", testPayload, queen.PublishOptions{
			QoS: protocol.QoS1,
		}); err != nil {
			b.Fatalf("Failed to publish: %v", err)
		}

		_, err := subscriber.ReceiveWithTimeout(1 * time.Second)
		if err != nil {
			b.Fatalf("Failed to receive: %v", err)
		}
	}
}

// BenchmarkRequestResponse 基准测试：Request/Response
func BenchmarkRequestResponse(b *testing.B) {
	broker, err := queen.New(
		queen.WithAddress(":0"),
	)
	if err != nil {
		b.Fatalf("Failed to create broker: %v", err)
	}
	defer broker.Stop()

	if err := broker.Start(); err != nil {
		b.Fatalf("Failed to start broker: %v", err)
	}

	server, err := broker.NewInternalClient(queen.InternalClientConfig{
		ClientID:   "server",
		BufferSize: 100,
	})
	if err != nil {
		b.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	if err := server.RegisterHandler("bench.echo", func(req *queen.InternalRequest) ([]byte, protocol.ReasonCode, error) {
		return req.Payload, protocol.ReasonSuccess, nil
	}, 10); err != nil {
		b.Fatalf("Failed to register handler: %v", err)
	}

	client, err := broker.NewInternalClient(queen.InternalClientConfig{
		ClientID:   "client",
		BufferSize: 100,
	})
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	testPayload := []byte("benchmark")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Request(ctx, "bench.echo", testPayload, &queen.InternalRequestOptions{
			Timeout: 5 * time.Second,
		})
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}
	}
}
