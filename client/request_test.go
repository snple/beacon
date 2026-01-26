package client

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/snple/beacon/packet"
)

// ============================================================================
// 基本请求-响应测试
// ============================================================================

// TestRequest_BasicClientToClient 测试客户端之间的基本请求-响应
func TestRequest_BasicClientToClient(t *testing.T) {
	broker, addr := testSetupCore(t)
	defer broker.Stop()

	// 创建请求发起者
	requester := testSetupClient(t, addr, "requester")
	defer requester.Close()

	// 创建请求处理者
	handler := testSetupClient(t, addr, "handler")
	defer handler.Close()

	// 注册 action
	err := handler.Register("echo")
	if err != nil {
		t.Fatalf("Failed to register action: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 启动处理者轮询协程
	go func() {
		for {
			req, err := handler.PollRequest(context.Background(), 5*time.Second)
			if err != nil {
				return
			}

			// 回显请求
			resp := NewResponse(req.Packet.Payload)
			req.Response(resp)
		}
	}()

	// 发送请求
	req := NewRequest("echo", []byte("hello"))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := requester.Request(ctx, req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if !resp.IsSuccess() {
		t.Fatalf("Request failed with reason: %s", resp.Error())
	}

	if string(resp.Packet.Payload) != "hello" {
		t.Errorf("Expected payload 'hello', got '%s'", string(resp.Packet.Payload))
	}
}

// TestRequest_MultipleActions 测试多个 action
func TestRequest_MultipleActions(t *testing.T) {
	broker, addr := testSetupCore(t)
	defer broker.Stop()


	requester := testSetupClient(t, addr, "requester")
	defer requester.Close()

	handler := testSetupClient(t, addr, "handler")
	defer handler.Close()

	// 注册多个 actions
	err := handler.RegisterMultiple([]string{"add", "multiply", "concat"})
	if err != nil {
		t.Fatalf("Failed to register actions: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 启动处理者轮询协程
	go func() {
		for {
			req, err := handler.PollRequest(context.Background(), 5*time.Second)
			if err != nil {
				return
			}

			switch req.Packet.Action {
			case "add":
				resp := NewResponse([]byte("10"))
				req.Response(resp)
			case "multiply":
				resp := NewResponse([]byte("20"))
				req.Response(resp)
			case "concat":
				resp := NewResponse([]byte("hello world"))
				req.Response(resp)
			}
		}
	}()

	// 测试不同的 actions
	tests := []struct {
		action   string
		expected string
	}{
		{"add", "10"},
		{"multiply", "20"},
		{"concat", "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			req := NewRequest(tt.action, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			resp, err := requester.Request(ctx, req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}

			if string(resp.Packet.Payload) != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, string(resp.Packet.Payload))
			}
		})
	}
}

// TestRequest_WithTargetClientID 测试指定目标客户端
func TestRequest_WithTargetClientID(t *testing.T) {
	broker, addr := testSetupCore(t)
	defer broker.Stop()


	requester := testSetupClient(t, addr, "requester")
	defer requester.Close()

	handler1 := testSetupClient(t, addr, "handler1")
	defer handler1.Close()

	handler2 := testSetupClient(t, addr, "handler2")
	defer handler2.Close()

	// 两个处理者都注册相同的 action
	handler1.Register("service")
	handler2.Register("service")

	time.Sleep(100 * time.Millisecond)

	// 启动处理者轮询协程
	startHandler := func(h *Client, name string) {
		go func() {
			for {
				req, err := h.PollRequest(context.Background(), 5*time.Second)
				if err != nil {
					return
				}

				resp := NewResponse([]byte(name))
				req.Response(resp)
			}
		}()
	}

	startHandler(handler1, "handler1")
	startHandler(handler2, "handler2")

	// 指定发送给 handler1
	req := NewRequest("service", nil)
	req.Packet.TargetClientID = "handler1"

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := requester.Request(ctx, req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if string(resp.Packet.Payload) != "handler1" {
		t.Errorf("Expected 'handler1', got '%s'", string(resp.Packet.Payload))
	}

	// 指定发送给 handler2
	req2 := NewRequest("service", nil)
	req2.Packet.TargetClientID = "handler2"

	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()

	resp2, err := requester.Request(ctx2, req2)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if string(resp2.Packet.Payload) != "handler2" {
		t.Errorf("Expected 'handler2', got '%s'", string(resp2.Packet.Payload))
	}
}

// ============================================================================
// 超时测试
// ============================================================================

// TestRequest_Timeout 测试请求超时
func TestRequest_Timeout(t *testing.T) {
	broker, addr := testSetupCore(t)
	defer broker.Stop()


	requester := testSetupClient(t, addr, "requester")
	defer requester.Close()

	handler := testSetupClient(t, addr, "slow-handler")
	defer handler.Close()

	handler.Register("slow")
	time.Sleep(100 * time.Millisecond)

	// 启动处理者，但故意延迟响应
	go func() {
		for {
			req, err := handler.PollRequest(context.Background(), 5*time.Second)
			if err != nil {
				return
			}

			// 延迟 3 秒再响应
			time.Sleep(3 * time.Second)
			resp := NewResponse([]byte("too late"))
			req.Response(resp)
		}
	}()

	// 发送超时时间为 1 秒的请求
	req := NewRequest("slow", []byte("test")).WithTimeout(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	_, err := requester.Request(ctx, req)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	// 验证确实是在 1 秒左右超时（允许一些误差）
	if duration < 900*time.Millisecond || duration > 1500*time.Millisecond {
		t.Errorf("Expected timeout around 1s, got %v", duration)
	}
}

// TestRequest_ContextTimeout 测试 Context 超时
func TestRequest_ContextTimeout(t *testing.T) {
	broker, addr := testSetupCore(t)
	defer broker.Stop()


	requester := testSetupClient(t, addr, "requester")
	defer requester.Close()

	handler := testSetupClient(t, addr, "handler")
	defer handler.Close()

	handler.Register("action")
	time.Sleep(100 * time.Millisecond)

	// 处理者延迟响应
	go func() {
		for {
			req, err := handler.PollRequest(context.Background(), 5*time.Second)
			if err != nil {
				return
			}

			time.Sleep(2 * time.Second)
			resp := NewResponse([]byte("late"))
			req.Response(resp)
		}
	}()

	// 使用 Context 超时
	req := NewRequest("action", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := requester.Request(ctx, req)
	if err == nil {
		t.Fatal("Expected context timeout error, got nil")
	}
}

// ============================================================================
// 错误处理测试
// ============================================================================

// TestRequest_ActionNotFound 测试 action 不存在
func TestRequest_ActionNotFound(t *testing.T) {
	broker, addr := testSetupCore(t)
	defer broker.Stop()


	requester := testSetupClient(t, addr, "requester")
	defer requester.Close()

	// 没有客户端注册 "nonexistent" action
	req := NewRequest("nonexistent", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := requester.Request(ctx, req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.Packet.ReasonCode != packet.ReasonActionNotFound {
		t.Errorf("Expected ReasonActionNotFound, got %v", resp.Packet.ReasonCode)
	}
}

// TestRequest_ClientNotFound 测试目标客户端不存在
func TestRequest_ClientNotFound(t *testing.T) {
	broker, addr := testSetupCore(t)
	defer broker.Stop()


	requester := testSetupClient(t, addr, "requester")
	defer requester.Close()

	// 指定不存在的客户端
	req := NewRequest("action", nil)
	req.Packet.TargetClientID = "nonexistent-client"

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := requester.Request(ctx, req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.Packet.ReasonCode != packet.ReasonClientNotFound {
		t.Errorf("Expected ReasonClientNotFound, got %v", resp.Packet.ReasonCode)
	}
}

// TestRequest_ErrorResponse 测试错误响应
func TestRequest_ErrorResponse(t *testing.T) {
	broker, addr := testSetupCore(t)
	defer broker.Stop()


	requester := testSetupClient(t, addr, "requester")
	defer requester.Close()

	handler := testSetupClient(t, addr, "handler")
	defer handler.Close()

	handler.Register("validate")
	time.Sleep(100 * time.Millisecond)

	// 处理者返回错误
	go func() {
		for {
			req, err := handler.PollRequest(context.Background(), 5*time.Second)
			if err != nil {
				return
			}

			// 验证失败，返回错误
			resp := NewErrorResponse(packet.ReasonBadRequest, "Invalid input")
			req.Response(resp)
		}
	}()

	req := NewRequest("validate", []byte("bad data"))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := requester.Request(ctx, req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.IsSuccess() {
		t.Fatal("Expected error response, got success")
	}

	if resp.Packet.ReasonCode != packet.ReasonBadRequest {
		t.Errorf("Expected ReasonBadRequest, got %v", resp.Packet.ReasonCode)
	}

	if resp.Packet.Properties.ReasonString != "Invalid input" {
		t.Errorf("Expected 'Invalid input', got '%s'", resp.Packet.Properties.ReasonString)
	}
}

// ============================================================================
// 并发测试
// ============================================================================

// TestRequest_Concurrent 测试并发请求
func TestRequest_Concurrent(t *testing.T) {
	broker, addr := testSetupCore(t)
	defer broker.Stop()


	requester := testSetupClient(t, addr, "requester")
	defer requester.Close()

	handler := testSetupClient(t, addr, "handler")
	defer handler.Close()

	handler.Register("concurrent")
	time.Sleep(100 * time.Millisecond)

	// 处理者响应请求
	var handledCount atomic.Int32
	go func() {
		for {
			req, err := handler.PollRequest(context.Background(), 10*time.Second)
			if err != nil {
				return
			}

			handledCount.Add(1)
			resp := NewResponse(req.Packet.Payload)
			req.Response(resp)
		}
	}()

	// 并发发送多个请求
	const numRequests = 50
	var wg sync.WaitGroup
	var successCount atomic.Int32

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			payload := []byte(fmt.Sprintf("request-%d", id))
			req := NewRequest("concurrent", payload)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resp, err := requester.Request(ctx, req)
			if err != nil {
				t.Errorf("Request %d failed: %v", id, err)
				return
			}

			if !resp.IsSuccess() {
				t.Errorf("Request %d failed with reason: %s", id, resp.Error())
				return
			}

			if string(resp.Packet.Payload) != string(payload) {
				t.Errorf("Request %d: expected '%s', got '%s'", id, string(payload), string(resp.Packet.Payload))
				return
			}

			successCount.Add(1)
		}(i)
	}

	wg.Wait()

	if successCount.Load() != numRequests {
		t.Errorf("Expected %d successful requests, got %d", numRequests, successCount.Load())
	}

	if handledCount.Load() != numRequests {
		t.Errorf("Expected %d handled requests, got %d", numRequests, handledCount.Load())
	}
}

// TestRequest_ConcurrentHandlers 测试多个处理者负载均衡
func TestRequest_ConcurrentHandlers(t *testing.T) {
	broker, addr := testSetupCore(t)
	defer broker.Stop()


	requester := testSetupClient(t, addr, "requester")
	defer requester.Close()

	// 创建多个处理者
	const numHandlers = 3
	handlers := make([]*Client, numHandlers)
	handlerCounts := make([]atomic.Int32, numHandlers)

	for i := 0; i < numHandlers; i++ {
		handlers[i] = testSetupClient(t, addr, fmt.Sprintf("handler-%d", i))
		defer handlers[i].Close()

		handlers[i].Register("loadbalance")

		// 启动处理协程
		idx := i
		go func() {
			for {
				req, err := handlers[idx].PollRequest(context.Background(), 10*time.Second)
				if err != nil {
					return
				}

				handlerCounts[idx].Add(1)
				resp := NewResponse([]byte(fmt.Sprintf("handler-%d", idx)))
				req.Response(resp)
			}
		}()
	}

	time.Sleep(200 * time.Millisecond)

	// 发送多个请求
	const numRequests = 30
	for i := 0; i < numRequests; i++ {
		req := NewRequest("loadbalance", nil)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

		_, err := requester.Request(ctx, req)
		cancel()

		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
	}

	// 验证负载均衡（每个处理者应该处理一些请求）
	totalHandled := int32(0)
	for i := 0; i < numHandlers; i++ {
		count := handlerCounts[i].Load()
		t.Logf("Handler %d handled %d requests", i, count)
		totalHandled += count

		// 每个处理者至少应该处理一些请求
		if count == 0 {
			t.Errorf("Handler %d didn't handle any requests", i)
		}
	}

	if totalHandled != numRequests {
		t.Errorf("Expected %d total handled, got %d", numRequests, totalHandled)
	}
}

// ============================================================================
// 注册/注销测试
// ============================================================================

// TestRequest_RegisterUnregister 测试注册和注销 action
func TestRequest_RegisterUnregister(t *testing.T) {
	broker, addr := testSetupCore(t)
	defer broker.Stop()


	requester := testSetupClient(t, addr, "requester")
	defer requester.Close()

	handler := testSetupClient(t, addr, "handler")
	defer handler.Close()

	// 注册 action
	err := handler.Register("dynamic")
	if err != nil {
		t.Fatalf("Failed to register: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 启动处理协程
	stopCh := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopCh:
				return
			default:
				req, err := handler.PollRequest(context.Background(), 1*time.Second)
				if err != nil {
					continue
				}

				resp := NewResponse([]byte("ok"))
				req.Response(resp)
			}
		}
	}()

	// 请求应该成功
	req := NewRequest("dynamic", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	resp, err := requester.Request(ctx, req)
	cancel()

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if !resp.IsSuccess() {
		t.Fatal("Expected success")
	}

	// 注销 action
	err = handler.Unregister("dynamic")
	if err != nil {
		t.Fatalf("Failed to unregister: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 请求应该失败（action 不存在）
	req2 := NewRequest("dynamic", nil)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	resp2, err := requester.Request(ctx2, req2)
	cancel2()

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp2.Packet.ReasonCode != packet.ReasonActionNotFound {
		t.Errorf("Expected ReasonActionNotFound, got %v", resp2.Packet.ReasonCode)
	}

	close(stopCh)
}

// TestRequest_GetRegisteredActions 测试获取已注册的 actions
func TestRequest_GetRegisteredActions(t *testing.T) {
	broker, addr := testSetupCore(t)
	defer broker.Stop()


	handler := testSetupClient(t, addr, "handler")
	defer handler.Close()

	// 注册多个 actions
	actions := []string{"action1", "action2", "action3"}
	err := handler.RegisterMultiple(actions)
	if err != nil {
		t.Fatalf("Failed to register: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 获取已注册的 actions
	registered := handler.GetRegisteredActions()

	if len(registered) != len(actions) {
		t.Errorf("Expected %d actions, got %d", len(actions), len(registered))
	}

	// 验证所有 actions 都存在
	actionMap := make(map[string]bool)
	for _, a := range registered {
		actionMap[a] = true
	}

	for _, a := range actions {
		if !actionMap[a] {
			t.Errorf("Action '%s' not found in registered actions", a)
		}
	}
}

// ============================================================================
// 属性测试
// ============================================================================

// TestRequest_WithProperties 测试请求属性
func TestRequest_WithProperties(t *testing.T) {
	broker, addr := testSetupCore(t)
	defer broker.Stop()


	requester := testSetupClient(t, addr, "requester")
	defer requester.Close()

	handler := testSetupClient(t, addr, "handler")
	defer handler.Close()

	handler.Register("props")
	time.Sleep(100 * time.Millisecond)

	// 验证接收到的属性
	receivedProps := make(chan map[string]string, 1)
	receivedTraceID := make(chan string, 1)
	receivedContentType := make(chan string, 1)

	go func() {
		req, err := handler.PollRequest(context.Background(), 5*time.Second)
		if err != nil {
			return
		}

		receivedProps <- req.Packet.Properties.UserProperties
		receivedTraceID <- req.Packet.Properties.TraceID
		receivedContentType <- req.Packet.Properties.ContentType

		resp := NewResponse([]byte("ok"))
		req.Response(resp)
	}()

	// 发送带属性的请求
	req := NewRequest("props", []byte("test")).
		WithTraceID("trace-123").
		WithContentType("application/json").
		WithUserProperty("key1", "value1").
		WithUserProperty("key2", "value2")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := requester.Request(ctx, req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	// 验证属性
	props := <-receivedProps
	if props["key1"] != "value1" || props["key2"] != "value2" {
		t.Errorf("User properties mismatch: %v", props)
	}

	traceID := <-receivedTraceID
	if traceID != "trace-123" {
		t.Errorf("Expected trace ID 'trace-123', got '%s'", traceID)
	}

	contentType := <-receivedContentType
	if contentType != "application/json" {
		t.Errorf("Expected content type 'application/json', got '%s'", contentType)
	}
}

// TestRequest_ResponseWithProperties 测试响应属性
func TestRequest_ResponseWithProperties(t *testing.T) {
	broker, addr := testSetupCore(t)
	defer broker.Stop()


	requester := testSetupClient(t, addr, "requester")
	defer requester.Close()

	handler := testSetupClient(t, addr, "handler")
	defer handler.Close()

	handler.Register("response-props")
	time.Sleep(100 * time.Millisecond)

	go func() {
		req, err := handler.PollRequest(context.Background(), 5*time.Second)
		if err != nil {
			return
		}

		// 响应带属性
		resp := NewResponse([]byte("data")).
			WithTraceID("response-trace").
			WithContentType("text/plain").
			WithUserProperty("server", "handler-1")

		req.Response(resp)
	}()

	req := NewRequest("response-props", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := requester.Request(ctx, req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	// 验证响应属性
	if resp.Packet.Properties.TraceID != "response-trace" {
		t.Errorf("Expected trace ID 'response-trace', got '%s'", resp.Packet.Properties.TraceID)
	}

	if resp.Packet.Properties.ContentType != "text/plain" {
		t.Errorf("Expected content type 'text/plain', got '%s'", resp.Packet.Properties.ContentType)
	}

	if resp.Packet.Properties.UserProperties["server"] != "handler-1" {
		t.Errorf("Expected server 'handler-1', got '%s'", resp.Packet.Properties.UserProperties["server"])
	}
}

// ============================================================================
// 请求队列测试
// ============================================================================

// TestRequest_QueueFull 测试请求队列满
func TestRequest_QueueFull(t *testing.T) {
	broker, addr := testSetupCore(t)
	defer broker.Stop()


	requester := testSetupClient(t, addr, "requester")
	defer requester.Close()

	// 创建队列容量很小的处理者
	opts := NewClientOptions().
		WithCore(addr).
		WithClientID("small-queue-handler").
		WithLogger(testLogger()).
		WithRequestQueueSize(2) // 很小的队列

	handler, err := NewWithOptions(opts)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}
	defer handler.Close()

	if err := handler.Connect(); err != nil {
		t.Fatalf("Failed to connect handler: %v", err)
	}

	handler.Register("queue-test")
	time.Sleep(100 * time.Millisecond)

	// 启动一个 goroutine 来调用 PollRequest，这样会初始化队列
	// 但不处理请求，让请求在队列中堆积
	go func() {
		// 先调用一次 PollRequest 来初始化队列
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		handler.PollRequest(ctx, 1*time.Second)
		cancel()
	}()

	// 等待队列初始化
	time.Sleep(150 * time.Millisecond)

	// 发送多个请求（超过队列容量）
	const numRequests = 10
	var mu sync.Mutex
	serverBusyCount := 0

	for i := 0; i < numRequests; i++ {
		req := NewRequest("queue-test", []byte(fmt.Sprintf("req-%d", i)))
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)

		resp, err := requester.Request(ctx, req)
		cancel()

		if err == nil && resp.Packet.ReasonCode == packet.ReasonServerBusy {
			mu.Lock()
			serverBusyCount++
			mu.Unlock()
		}

		time.Sleep(10 * time.Millisecond)
	}

	// 应该有一些请求被拒绝（队列满）
	if serverBusyCount == 0 {
		t.Error("Expected some requests to be rejected due to queue full")
	}
}

// ============================================================================
// 断线重连测试
// ============================================================================

// TestRequest_HandlerDisconnect 测试处理者断线
func TestRequest_HandlerDisconnect(t *testing.T) {
	broker, addr := testSetupCore(t)
	defer broker.Stop()


	requester := testSetupClient(t, addr, "requester")
	defer requester.Close()

	handler := testSetupClient(t, addr, "handler")

	handler.Register("disconnect-test")
	time.Sleep(100 * time.Millisecond)

	// 处理者立即断线
	handler.Close()

	time.Sleep(100 * time.Millisecond)

	// 请求应该失败（没有可用的处理者）
	req := NewRequest("disconnect-test", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := requester.Request(ctx, req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.Packet.ReasonCode != packet.ReasonActionNotFound {
		t.Errorf("Expected ReasonActionNotFound, got %v", resp.Packet.ReasonCode)
	}
}
