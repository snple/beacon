package client

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/snple/beacon/packet"
)

// TestClientOptionsBuilder 测试 ClientOptions Builder 模式
func TestClientOptionsBuilder(t *testing.T) {
	opts := NewClientOptions().
		WithCore("localhost:3883").
		WithClientID("test-client").
		WithKeepAlive(30).
		WithCleanSession(true).
		WithSessionExpiry(3600).
		WithConnectTimeout(5*time.Second).
		WithPublishTimeout(10*time.Second).
		WithTraceID("trace-123").
		WithUserProperty("key1", "value1").
		WithUserProperty("key2", "value2").
		WithRequestQueueSize(200).
		WithMessageQueueSize(500)

	if opts.Core != "localhost:3883" {
		t.Errorf("expected core localhost:3883, got %s", opts.Core)
	}
	if opts.ClientID != "test-client" {
		t.Errorf("expected clientID test-client, got %s", opts.ClientID)
	}
	if opts.KeepAlive != 30 {
		t.Errorf("expected keepAlive 30, got %d", opts.KeepAlive)
	}
	if !opts.CleanSession {
		t.Error("expected cleanSession true")
	}
	if opts.SessionExpiry != 3600 {
		t.Errorf("expected sessionExpiry 3600, got %d", opts.SessionExpiry)
	}
	if opts.ConnectTimeout != 5*time.Second {
		t.Errorf("expected connectTimeout 5s, got %v", opts.ConnectTimeout)
	}
	if opts.PublishTimeout != 10*time.Second {
		t.Errorf("expected publishTimeout 10s, got %v", opts.PublishTimeout)
	}
	if opts.TraceID != "trace-123" {
		t.Errorf("expected traceID trace-123, got %s", opts.TraceID)
	}
	if opts.UserProperties["key1"] != "value1" {
		t.Errorf("expected UserProperties[key1]=value1, got %s", opts.UserProperties["key1"])
	}
	if opts.UserProperties["key2"] != "value2" {
		t.Errorf("expected UserProperties[key2]=value2, got %s", opts.UserProperties["key2"])
	}
	if opts.RequestQueueSize != 200 {
		t.Errorf("expected RequestQueueSize 200, got %d", opts.RequestQueueSize)
	}
	if opts.MessageQueueSize != 500 {
		t.Errorf("expected MessageQueueSize 500, got %d", opts.MessageQueueSize)
	}
}

// TestPublishOptionsBuilder 测试 PublishOptions Builder 模式
func TestPublishOptionsBuilder(t *testing.T) {
	opts := NewPublishOptions().
		WithQoS(packet.QoS1).
		WithRetain(true).
		WithPriority(packet.PriorityHigh).
		WithTraceID("pub-trace-123").
		WithContentType("application/json").
		WithExpiry(3600).
		WithTimeout(5*time.Second).
		WithTargetClientID("target-client").
		WithResponseTopic("response/topic").
		WithUserProperty("pub-key", "pub-value")

	if opts.QoS != packet.QoS1 {
		t.Errorf("expected QoS1, got %v", opts.QoS)
	}
	if !opts.Retain {
		t.Error("expected Retain true")
	}
	if opts.Priority != packet.PriorityHigh {
		t.Errorf("expected PriorityHigh, got %v", opts.Priority)
	}
	if opts.TraceID != "pub-trace-123" {
		t.Errorf("expected traceID pub-trace-123, got %s", opts.TraceID)
	}
	if opts.ContentType != "application/json" {
		t.Errorf("expected contentType application/json, got %s", opts.ContentType)
	}
	if opts.Expiry != 3600 {
		t.Errorf("expected expiry 3600, got %d", opts.Expiry)
	}
	if opts.Timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", opts.Timeout)
	}
	if opts.TargetClientID != "target-client" {
		t.Errorf("expected targetClientID target-client, got %s", opts.TargetClientID)
	}
	if opts.ResponseTopic != "response/topic" {
		t.Errorf("expected responseTopic response/topic, got %s", opts.ResponseTopic)
	}
	if opts.UserProperties["pub-key"] != "pub-value" {
		t.Errorf("expected UserProperties[pub-key]=pub-value, got %s", opts.UserProperties["pub-key"])
	}
}

// TestSubscribeOptionsBuilder 测试 SubscribeOptions Builder 模式
func TestSubscribeOptionsBuilder(t *testing.T) {
	opts := NewSubscribeOptions().
		WithQoS(packet.QoS1).
		WithNoLocal(true).
		WithRetainAsPublished(true)

	if opts.QoS != packet.QoS1 {
		t.Errorf("expected QoS1, got %v", opts.QoS)
	}
	if !opts.NoLocal {
		t.Error("expected NoLocal true")
	}
	if !opts.RetainAsPublished {
		t.Error("expected RetainAsPublished true")
	}
}

// TestClientOptionsDefaults 测试默认值
func TestClientOptionsDefaults(t *testing.T) {
	opts := NewClientOptions()

	if opts.Core != "localhost:3883" {
		t.Errorf("expected default core localhost:3883, got %s", opts.Core)
	}
	if opts.KeepAlive != 60 {
		t.Errorf("expected default keepAlive 60, got %d", opts.KeepAlive)
	}
	if !opts.CleanSession {
		t.Error("expected default cleanSession true")
	}
	if opts.ConnectTimeout != 10*time.Second {
		t.Errorf("expected default connectTimeout 10s, got %v", opts.ConnectTimeout)
	}
	if opts.PublishTimeout != 30*time.Second {
		t.Errorf("expected default publishTimeout 30s, got %v", opts.PublishTimeout)
	}
	if opts.RequestQueueSize != 100 {
		t.Errorf("expected default requestQueueSize 100, got %d", opts.RequestQueueSize)
	}
	if opts.MessageQueueSize != 100 {
		t.Errorf("expected default messageQueueSize 100, got %d", opts.MessageQueueSize)
	}
}

// TestPublishOptionsDefaults 测试 PublishOptions 默认值
func TestPublishOptionsDefaults(t *testing.T) {
	opts := NewPublishOptions()

	if opts.QoS != packet.QoS0 {
		t.Errorf("expected default QoS0, got %v", opts.QoS)
	}
	if opts.Priority != packet.PriorityNormal {
		t.Errorf("expected default PriorityNormal, got %v", opts.Priority)
	}
	if opts.Retain {
		t.Error("expected default Retain false")
	}
}

// TestClientOptionsWithHooks 测试 ClientOptions 带钩子
func TestClientOptionsWithHooks(t *testing.T) {
	connectCalled := false
	opts := NewClientOptions().
		WithCore("test:1234").
		WithClientID("test-client").
		WithConnectionHandler(&ConnectionHandlerFunc{
			ConnectFunc: func(ctx *ConnectContext) {
				connectCalled = true
			},
		})

	if opts.Core != "test:1234" {
		t.Errorf("expected core test:1234, got %s", opts.Core)
	}
	if opts.ClientID != "test-client" {
		t.Errorf("expected clientID test-client, got %s", opts.ClientID)
	}
	if opts.Hooks.ConnectionHandler == nil {
		t.Error("expected ConnectionHandler to be set")
	}

	// 测试钩子是否正确设置
	opts.Hooks.callOnConnect(&ConnectContext{SessionPresent: true})
	if !connectCalled {
		t.Error("expected ConnectionHandler.OnConnect to be called")
	}
}

// TestRequestBuilderPattern 测试 Request 的 Builder 模式
func TestRequestBuilderPattern(t *testing.T) {
	req := NewRequest("test-action", []byte("payload")).
		WithTimeout(60).
		WithTraceID("req-trace").
		WithContentType("text/plain").
		WithUserProperty("key1", "value1")

	if req.Packet.Action != "test-action" {
		t.Errorf("expected action test-action, got %s", req.Packet.Action)
	}
	if string(req.Packet.Payload) != "payload" {
		t.Errorf("expected payload 'payload', got %s", string(req.Packet.Payload))
	}
	if req.Packet.Properties.Timeout != 60 {
		t.Errorf("expected timeout 60, got %d", req.Packet.Properties.Timeout)
	}
	if req.Packet.Properties.TraceID != "req-trace" {
		t.Errorf("expected traceID req-trace, got %s", req.Packet.Properties.TraceID)
	}
	if req.Packet.Properties.ContentType != "text/plain" {
		t.Errorf("expected contentType text/plain, got %s", req.Packet.Properties.ContentType)
	}
	if req.Packet.Properties.UserProperties["key1"] != "value1" {
		t.Errorf("expected UserProperties[key1]=value1, got %s", req.Packet.Properties.UserProperties["key1"])
	}
}

// TestResponseBuilderPattern 测试 Response 的 Builder 模式
func TestResponseBuilderPattern(t *testing.T) {
	resp := NewResponse([]byte("response data")).
		WithTraceID("resp-trace").
		WithContentType("application/json").
		WithUserProperty("resp-key", "resp-value")

	if !resp.IsSuccess() {
		t.Error("expected response to be success")
	}
	if string(resp.Packet.Payload) != "response data" {
		t.Errorf("expected payload 'response data', got %s", string(resp.Packet.Payload))
	}
	if resp.Packet.Properties.TraceID != "resp-trace" {
		t.Errorf("expected traceID resp-trace, got %s", resp.Packet.Properties.TraceID)
	}
	if resp.Packet.Properties.ContentType != "application/json" {
		t.Errorf("expected contentType application/json, got %s", resp.Packet.Properties.ContentType)
	}
	if resp.Packet.Properties.UserProperties["resp-key"] != "resp-value" {
		t.Errorf("expected UserProperties[resp-key]=resp-value, got %s", resp.Packet.Properties.UserProperties["resp-key"])
	}
}

// TestErrorResponse 测试错误响应
func TestErrorResponse(t *testing.T) {
	resp := NewErrorResponse(packet.ReasonNotAuthorized, "permission denied").
		WithTraceID("error-trace")

	if resp.IsSuccess() {
		t.Error("expected response to be failure")
	}
	if resp.Packet.ReasonCode != packet.ReasonNotAuthorized {
		t.Errorf("expected ReasonNotAuthorized, got %v", resp.Packet.ReasonCode)
	}
	if resp.Packet.Properties.ReasonString != "permission denied" {
		t.Errorf("expected reasonString 'permission denied', got %s", resp.Packet.Properties.ReasonString)
	}
	if resp.Packet.Properties.TraceID != "error-trace" {
		t.Errorf("expected traceID error-trace, got %s", resp.Packet.Properties.TraceID)
	}
	if resp.Error() != "permission denied" {
		t.Errorf("expected Error() 'permission denied', got %s", resp.Error())
	}
}

// TestPollMessageNotConnected 测试未连接时 PollMessage
func TestPollMessageNotConnected(t *testing.T) {
	client := &Client{}
	client.connected.Store(false)

	_, err := client.PollMessage(context.Background(), time.Second)
	if err == nil {
		t.Error("expected error when not connected")
	}
	if !errors.Is(err, ErrNotConnected) {
		t.Errorf("expected ErrNotConnected, got %v", err)
	}
}

// TestSubscribeNotConnected 测试未连接时 Subscribe
func TestSubscribeNotConnected(t *testing.T) {
	client := &Client{}
	client.connected.Store(false)

	err := client.Subscribe("topic1", "topic2")
	if err == nil {
		t.Error("expected error when not connected")
	}
	if !errors.Is(err, ErrNotConnected) {
		t.Errorf("expected ErrNotConnected, got %v", err)
	}
}

// TestSubscribeWithOptionsNotConnected 测试未连接时 SubscribeWithOptions
func TestSubscribeWithOptionsNotConnected(t *testing.T) {
	client := &Client{}
	client.connected.Store(false)

	opts := NewSubscribeOptions().WithQoS(packet.QoS1)
	err := client.SubscribeWithOptions([]string{"topic1"}, opts)
	if err == nil {
		t.Error("expected error when not connected")
	}
}

// TestSubscribeWithEmptyTopics 测试空主题列表
func TestSubscribeWithEmptyTopics(t *testing.T) {
	client := &Client{}
	client.connected.Store(true)

	err := client.SubscribeWithOptions([]string{}, nil)
	if err == nil {
		t.Error("expected error with empty topics")
	}
	if !errors.Is(err, ErrTopicsEmpty) {
		t.Errorf("expected ErrTopicsEmpty, got %v", err)
	}
}

// TestPublishWithOptionsNotConnected 测试未连接时 PublishWithOptions
func TestPublishWithOptionsNotConnected(t *testing.T) {
	client := &Client{}
	client.connected.Store(false)

	opts := NewPublishOptions().WithQoS(packet.QoS1)
	err := client.Publish("topic", []byte("payload"), opts)
	if err == nil {
		t.Error("expected error when not connected")
	}
}

// TestUserPropertiesChaining 测试 UserProperties 链式调用
func TestUserPropertiesChaining(t *testing.T) {
	opts := NewClientOptions().
		WithUserProperty("key1", "value1").
		WithUserProperty("key2", "value2").
		WithUserProperty("key3", "value3")

	if len(opts.UserProperties) != 3 {
		t.Errorf("expected 3 user properties, got %d", len(opts.UserProperties))
	}
	if opts.UserProperties["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %s", opts.UserProperties["key1"])
	}
	if opts.UserProperties["key2"] != "value2" {
		t.Errorf("expected key2=value2, got %s", opts.UserProperties["key2"])
	}
	if opts.UserProperties["key3"] != "value3" {
		t.Errorf("expected key3=value3, got %s", opts.UserProperties["key3"])
	}
}

// TestPublishOptionsUserPropertiesChaining 测试 PublishOptions UserProperties 链式调用
func TestPublishOptionsUserPropertiesChaining(t *testing.T) {
	opts := NewPublishOptions().
		WithUserProperty("pub1", "val1").
		WithUserProperty("pub2", "val2")

	if len(opts.UserProperties) != 2 {
		t.Errorf("expected 2 user properties, got %d", len(opts.UserProperties))
	}
}
