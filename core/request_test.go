package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// testLogger 创建测试用的 logger
func testLogger() *zap.Logger {
	logger, _ := zap.NewDevelopment()
	return logger
}

// setupTestCore 创建测试用的 Core
func setupTestCore(t *testing.T, opts *CoreOptions) *Core {
	if opts == nil {
		opts = NewCoreOptions()
	}
	opts.WithAddress("127.0.0.1:0"). // 使用随机端口
						WithLogger(testLogger()).
						WithConnectTimeout(5).
						WithRequestQueueSize(100)

	core, err := NewWithOptions(opts)
	if err != nil {
		t.Fatalf("Failed to create core: %v", err)
	}

	if err := core.Start(); err != nil {
		t.Fatalf("Failed to start core: %v", err)
	}

	return core
}

// setupTestClient 创建测试用的 Client
func setupTestClient(t *testing.T, coreAddr string, clientID string, opts *client.ClientOptions) *client.Client {
	if opts == nil {
		opts = client.NewClientOptions()
	}
	opts.WithCore(coreAddr).
		WithClientID(clientID).
		WithLogger(testLogger()).
		WithCleanSession(true).
		WithRequestQueueSize(100)

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
// Request/Response 结构体测试
// ============================================================================

func TestNewRequest(t *testing.T) {
	req := NewRequest("test.action", []byte("payload"))

	if req.Action != "test.action" {
		t.Errorf("Expected action 'test.action', got '%s'", req.Action)
	}
	if string(req.Payload) != "payload" {
		t.Errorf("Expected payload 'payload', got '%s'", string(req.Payload))
	}
}

func TestRequestBuilder(t *testing.T) {
	req := NewRequest("test.action", []byte("data")).
		WithTimeout(30).
		WithTraceID("trace-123").
		WithContentType("application/json").
		WithUserProperty("key1", "value1").
		WithUserProperty("key2", "value2")

	if req.Timeout != 30 {
		t.Errorf("Expected timeout 30, got %d", req.Timeout)
	}
	if req.TraceID != "trace-123" {
		t.Errorf("Expected traceID 'trace-123', got '%s'", req.TraceID)
	}
	if req.ContentType != "application/json" {
		t.Errorf("Expected contentType 'application/json', got '%s'", req.ContentType)
	}
	if req.UserProperties["key1"] != "value1" {
		t.Errorf("Expected UserProperties[key1]='value1', got '%s'", req.UserProperties["key1"])
	}
	if req.UserProperties["key2"] != "value2" {
		t.Errorf("Expected UserProperties[key2]='value2', got '%s'", req.UserProperties["key2"])
	}
}

func TestNewResponse(t *testing.T) {
	resp := NewResponse([]byte("result"))

	if resp.ReasonCode != packet.ReasonSuccess {
		t.Errorf("Expected ReasonSuccess, got %v", resp.ReasonCode)
	}
	if string(resp.Payload) != "result" {
		t.Errorf("Expected payload 'result', got '%s'", string(resp.Payload))
	}
	if !resp.IsSuccess() {
		t.Error("Expected IsSuccess() to return true")
	}
}

func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse(packet.ReasonServerBusy, "服务繁忙")

	if resp.ReasonCode != packet.ReasonServerBusy {
		t.Errorf("Expected ReasonServerBusy, got %v", resp.ReasonCode)
	}
	if resp.ReasonString != "服务繁忙" {
		t.Errorf("Expected reason '服务繁忙', got '%s'", resp.ReasonString)
	}
	if resp.IsSuccess() {
		t.Error("Expected IsSuccess() to return false")
	}
	if resp.Error() != "服务繁忙" {
		t.Errorf("Expected Error() to return '服务繁忙', got '%s'", resp.Error())
	}
}

func TestResponseBuilder(t *testing.T) {
	resp := NewResponse([]byte("data")).
		WithTraceID("trace-456").
		WithContentType("application/json").
		WithUserProperty("resp-key", "resp-value")

	if resp.TraceID != "trace-456" {
		t.Errorf("Expected traceID 'trace-456', got '%s'", resp.TraceID)
	}
	if resp.ContentType != "application/json" {
		t.Errorf("Expected contentType 'application/json', got '%s'", resp.ContentType)
	}
	if resp.UserProperties["resp-key"] != "resp-value" {
		t.Errorf("Expected UserProperties[resp-key]='resp-value', got '%s'", resp.UserProperties["resp-key"])
	}
}

// ============================================================================
// core REQUEST/RESPONSE 集成测试
// ============================================================================

// TestCorePollRequest 测试 core 轮询请求
func TestCorePollRequest(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建客户端
	c := setupTestClient(t, addr, "client1", nil)
	defer c.Close()

	// 客户端向 core 发送请求
	go func() {
		time.Sleep(100 * time.Millisecond) // 等待 core 开始轮询
		_, err := c.RequestToCore(context.Background(), "test.echo", []byte("hello"), nil)
		if err != nil {
			t.Logf("Request error (expected if core responds): %v", err)
		}
	}()

	// core 轮询接收请求
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := core.PollRequest(ctx, 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to poll request: %v", err)
	}

	if req == nil {
		t.Fatal("Expected request, got nil")
	}

	if req.Action != "test.echo" {
		t.Errorf("Expected action 'test.echo', got '%s'", req.Action)
	}
	if string(req.Payload) != "hello" {
		t.Errorf("Expected payload 'hello', got '%s'", string(req.Payload))
	}
	if req.SourceClientID != "client1" {
		t.Errorf("Expected SourceClientID 'client1', got '%s'", req.SourceClientID)
	}

	// core 响应请求
	err = req.RespondSuccess([]byte("world"))
	if err != nil {
		t.Fatalf("Failed to respond: %v", err)
	}
}

// TestCoreRequestToClient 测试 core 向客户端发送请求
func TestCoreRequestToClient(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建客户端
	c := setupTestClient(t, addr, "worker1", nil)
	defer c.Close()

	// 客户端注册 action
	err := c.Register("calculate")
	if err != nil {
		t.Fatalf("Failed to register action: %v", err)
	}

	// 等待注册生效
	time.Sleep(50 * time.Millisecond)

	// 客户端在后台处理请求
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := c.PollRequest(ctx, 3*time.Second)
		if err != nil {
			t.Errorf("Client failed to poll request: %v", err)
			return
		}

		if req == nil {
			t.Error("Client expected request, got nil")
			return
		}

		if req.Action != "calculate" {
			t.Errorf("Expected action 'calculate', got '%s'", req.Action)
		}

		// 处理并响应
		result := []byte("42")
		err = req.RespondSuccess(result)
		if err != nil {
			t.Errorf("Failed to respond: %v", err)
		}
	}()

	// 等待客户端准备好接收
	time.Sleep(100 * time.Millisecond)

	// core 发送请求
	resp, err := core.RequestToClient(context.Background(), "worker1", "calculate", []byte("6*7"), nil)
	if err != nil {
		t.Fatalf("core request failed: %v", err)
	}

	if !resp.IsSuccess() {
		t.Errorf("Expected success response, got: %v - %s", resp.ReasonCode, resp.Error())
	}
	if string(resp.Payload) != "42" {
		t.Errorf("Expected response '42', got '%s'", string(resp.Payload))
	}

	wg.Wait()
}

// TestClientToClientRequest 测试客户端之间的请求
func TestClientToClientRequest(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建服务端客户端
	server := setupTestClient(t, addr, "server", nil)
	defer server.Close()

	// 创建请求端客户端
	requester := setupTestClient(t, addr, "requester", nil)
	defer requester.Close()

	// 服务端注册 action
	err := server.Register("greet")
	if err != nil {
		t.Fatalf("Failed to register action: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// 服务端在后台处理请求
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := server.PollRequest(ctx, 3*time.Second)
		if err != nil {
			t.Errorf("Server failed to poll request: %v", err)
			return
		}

		if req == nil {
			t.Error("Server expected request, got nil")
			return
		}

		// 验证来源
		if req.SourceClientID != "requester" {
			t.Errorf("Expected SourceClientID 'requester', got '%s'", req.SourceClientID)
		}

		// 响应
		greeting := "Hello, " + string(req.Payload) + "!"
		err = req.RespondSuccess([]byte(greeting))
		if err != nil {
			t.Errorf("Failed to respond: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// 请求端发送请求（通过 action 路由，不指定目标）
	resp, err := requester.Request(context.Background(), "greet", []byte("World"), nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if !resp.IsSuccess() {
		t.Errorf("Expected success, got: %v", resp.Error())
	}
	if string(resp.Payload) != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got '%s'", string(resp.Payload))
	}

	wg.Wait()
}

// TestClientToClientDirectRequest 测试客户端之间的直接请求（指定目标）
func TestClientToClientDirectRequest(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建服务端客户端
	server := setupTestClient(t, addr, "direct-server", nil)
	defer server.Close()

	// 创建请求端客户端
	requester := setupTestClient(t, addr, "direct-requester", nil)
	defer requester.Close()

	// 服务端在后台处理请求
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := server.PollRequest(ctx, 3*time.Second)
		if err != nil {
			t.Errorf("Server failed to poll request: %v", err)
			return
		}

		if req == nil {
			t.Error("Server expected request, got nil")
			return
		}

		err = req.RespondSuccess([]byte("direct response"))
		if err != nil {
			t.Errorf("Failed to respond: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// 请求端直接向指定客户端发送请求
	resp, err := requester.RequestToClient(
		context.Background(),
		"direct-server",
		"any.action",
		[]byte("direct request"),
		nil,
	)
	if err != nil {
		t.Fatalf("Direct request failed: %v", err)
	}

	if !resp.IsSuccess() {
		t.Errorf("Expected success, got: %v", resp.Error())
	}
	if string(resp.Payload) != "direct response" {
		t.Errorf("Expected 'direct response', got '%s'", string(resp.Payload))
	}

	wg.Wait()
}

// TestRequestTimeout 测试请求超时
func TestRequestTimeout(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建服务端客户端（但不响应）
	server := setupTestClient(t, addr, "slow-server", nil)
	defer server.Close()

	// 创建请求端客户端
	requester := setupTestClient(t, addr, "timeout-requester", nil)
	defer requester.Close()

	// 服务端注册 action 但不响应
	err := server.Register("slow.action")
	if err != nil {
		t.Fatalf("Failed to register action: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// 服务端轮询但不响应（模拟慢处理）
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.PollRequest(ctx, 5*time.Second)
		// 不响应
	}()

	time.Sleep(50 * time.Millisecond)

	// 请求端发送请求，设置短超时
	start := time.Now()
	_, err = requester.Request(context.Background(), "slow.action", []byte("data"), client.NewRequestOptions().WithTimeout(500*time.Millisecond))
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	// 检查超时时间大致正确
	if elapsed < 400*time.Millisecond || elapsed > 1*time.Second {
		t.Errorf("Expected timeout around 500ms, got %v", elapsed)
	}
}

// TestRequestToNonExistentClient 测试请求不存在的客户端
func TestRequestToNonExistentClient(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	requester := setupTestClient(t, addr, "requester-noexist", nil)
	defer requester.Close()

	// 请求不存在的客户端
	resp, err := requester.RequestToClient(
		context.Background(),
		"non-existent-client",
		"test.action",
		[]byte("data"),
		nil,
	)

	// 应该收到错误响应
	if err != nil {
		// 超时或其他错误也可以接受
		t.Logf("Got error: %v", err)
		return
	}

	if resp.IsSuccess() {
		t.Error("Expected failure response for non-existent client")
	}
}

// TestRequestToUnregisteredAction 测试请求未注册的 action
func TestRequestToUnregisteredAction(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	requester := setupTestClient(t, addr, "requester-unregistered", nil)
	defer requester.Close()

	// 请求未注册的 action
	resp, err := requester.Request(context.Background(), "unregistered.action", []byte("data"), client.NewRequestOptions().WithTimeout(1*time.Second))

	if err != nil {
		t.Logf("Got error: %v", err)
		return
	}

	if resp.IsSuccess() {
		t.Error("Expected failure response for unregistered action")
	}

	if resp.ReasonCode != packet.ReasonActionNotFound {
		t.Errorf("Expected ReasonActionNotFound, got %v", resp.ReasonCode)
	}
}

// TestMultipleActionsRegistration 测试多个 action 注册
func TestMultipleActionsRegistration(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	server := setupTestClient(t, addr, "multi-action-server", nil)
	defer server.Close()

	// 注册多个 actions
	err := server.RegisterMultiple([]string{"action1", "action2", "action3"})
	if err != nil {
		t.Fatalf("Failed to register actions: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// 验证注册
	actions := server.GetRegisteredActions()
	if len(actions) != 3 {
		t.Errorf("Expected 3 registered actions, got %d", len(actions))
	}

	// 验证 core 端
	handlers := core.GetActionsForClient("multi-action-server")
	if len(handlers) != 3 {
		t.Errorf("Expected core to have 3 actions for client, got %d", len(handlers))
	}
}

// TestActionUnregistration 测试 action 注销
func TestActionUnregistration(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	server := setupTestClient(t, addr, "unregister-server", nil)
	defer server.Close()

	// 注册 actions
	err := server.RegisterMultiple([]string{"keep", "remove"})
	if err != nil {
		t.Fatalf("Failed to register actions: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// 注销一个 action
	err = server.Unregister("remove")
	if err != nil {
		t.Fatalf("Failed to unregister action: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// 验证
	if server.HasAction("remove") {
		t.Error("Action 'remove' should not exist after unregistration")
	}
	if !server.HasAction("keep") {
		t.Error("Action 'keep' should still exist")
	}
}

// TestConcurrentRequests 测试并发请求
func TestConcurrentRequests(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	server := setupTestClient(t, addr, "concurrent-server", nil)
	defer server.Close()

	requester := setupTestClient(t, addr, "concurrent-requester", nil)
	defer requester.Close()

	server.Register("concurrent.echo")
	time.Sleep(50 * time.Millisecond)

	// 服务端处理请求
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				req, err := server.PollRequest(ctx, 100*time.Millisecond)
				if err != nil || req == nil {
					continue
				}
				req.RespondSuccess(req.Payload)
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// 并发发送多个请求
	const numRequests = 10
	var wg sync.WaitGroup
	errors := make(chan error, numRequests)
	responses := make(chan *client.Response, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			payload := []byte(fmt.Sprintf("request-%d", idx))
			resp, err := requester.Request(context.Background(), "concurrent.echo", payload, client.NewRequestOptions().WithTimeout(5*time.Second))
			if err != nil {
				errors <- err
				return
			}
			responses <- resp
		}(i)
	}

	wg.Wait()
	close(errors)
	close(responses)

	// 检查结果
	errCount := 0
	for err := range errors {
		t.Errorf("Concurrent request error: %v", err)
		errCount++
	}

	successCount := 0
	for resp := range responses {
		if resp.IsSuccess() {
			successCount++
		}
	}

	if errCount > 0 {
		t.Errorf("Got %d errors in concurrent requests", errCount)
	}
	if successCount != numRequests {
		t.Errorf("Expected %d successful responses, got %d", numRequests, successCount)
	}
}

// TestCoreRequestWithOptions 测试 core 使用选项发送请求
func TestCoreRequestWithOptions(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	server := setupTestClient(t, addr, "options-server", nil)
	defer server.Close()

	server.Register("options.test")
	time.Sleep(50 * time.Millisecond)

	// 服务端处理请求并检查属性
	var receivedReq *client.Request
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := server.PollRequest(ctx, 3*time.Second)
		if err != nil || req == nil {
			return
		}
		receivedReq = req
		req.RespondSuccess([]byte("ok"))
	}()

	time.Sleep(100 * time.Millisecond)

	// core 使用选项发送请求
	opts := &RequestOptions{
		TargetClientID: "options-server",
		Timeout:        5 * time.Second,
		TraceID:        "trace-opts-123",
		ContentType:    "application/json",
		UserProperties: map[string]string{"custom": "value"},
	}

	resp, err := core.Request(context.Background(), "options.test", []byte("test"), opts)
	if err != nil {
		t.Fatalf("Request with options failed: %v", err)
	}

	wg.Wait()

	if !resp.IsSuccess() {
		t.Errorf("Expected success, got: %v", resp.Error())
	}

	// 验证服务端收到的属性
	if receivedReq != nil {
		if receivedReq.TraceID != "trace-opts-123" {
			t.Errorf("Expected TraceID 'trace-opts-123', got '%s'", receivedReq.TraceID)
		}
		if receivedReq.ContentType != "application/json" {
			t.Errorf("Expected ContentType 'application/json', got '%s'", receivedReq.ContentType)
		}
	}
}

// TestResponseWithProperties 测试响应携带属性
func TestResponseWithProperties(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	server := setupTestClient(t, addr, "props-server", nil)
	defer server.Close()

	requester := setupTestClient(t, addr, "props-requester", nil)
	defer requester.Close()

	server.Register("props.test")
	time.Sleep(50 * time.Millisecond)

	// 服务端响应时携带属性
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := server.PollRequest(ctx, 3*time.Second)
		if err != nil || req == nil {
			return
		}

		// 使用 Builder 模式构建响应
		resp := client.NewResponse([]byte("result")).
			WithTraceID("response-trace").
			WithContentType("text/plain").
			WithUserProperty("server-prop", "server-value")

		req.Response(resp)
	}()

	time.Sleep(100 * time.Millisecond)

	resp, err := requester.Request(context.Background(), "props.test", []byte("data"), nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.TraceID != "response-trace" {
		t.Errorf("Expected TraceID 'response-trace', got '%s'", resp.TraceID)
	}
	if resp.ContentType != "text/plain" {
		t.Errorf("Expected ContentType 'text/plain', got '%s'", resp.ContentType)
	}
	if resp.UserProperties["server-prop"] != "server-value" {
		t.Errorf("Expected UserProperties[server-prop]='server-value', got '%s'", resp.UserProperties["server-prop"])
	}
}

// TestPollRequestTimeout 测试轮询超时
func TestPollRequestTimeout(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	// core 轮询但没有请求
	start := time.Now()
	req, err := core.PollRequest(context.Background(), 200*time.Millisecond)
	elapsed := time.Since(start)

	if !errors.Is(err, ErrPollTimeout) {
		t.Errorf("Expected ErrPollTimeout, got: %v", err)
	}
	if req != nil {
		t.Error("Expected nil request on timeout")
	}
	if elapsed < 150*time.Millisecond || elapsed > 500*time.Millisecond {
		t.Errorf("Expected timeout around 200ms, got %v", elapsed)
	}
}

// TestClientPollRequestTimeout 测试客户端轮询超时
func TestClientPollRequestTimeout(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	c := setupTestClient(t, addr, "poll-timeout-client", nil)
	defer c.Close()

	// 客户端轮询但没有请求
	start := time.Now()
	ctx := context.Background()
	req, err := c.PollRequest(ctx, 200*time.Millisecond)
	elapsed := time.Since(start)

	if !errors.Is(err, client.ErrPollTimeout) {
		t.Errorf("Expected client.ErrPollTimeout, got: %v", err)
	}
	if req != nil {
		t.Error("Expected nil request on timeout")
	}
	if elapsed < 150*time.Millisecond || elapsed > 500*time.Millisecond {
		t.Errorf("Expected timeout around 200ms, got %v", elapsed)
	}
}

// TestLoadBalancing 测试负载均衡
func TestLoadBalancing(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建多个服务端
	server1 := setupTestClient(t, addr, "lb-server-1", nil)
	defer server1.Close()
	server2 := setupTestClient(t, addr, "lb-server-2", nil)
	defer server2.Close()

	// 都注册同一个 action
	server1.Register("lb.action")
	server2.Register("lb.action")
	time.Sleep(100 * time.Millisecond)

	// 验证有两个处理者
	handlers := core.GetActionHandlers("lb.action")
	if len(handlers) != 2 {
		t.Errorf("Expected 2 handlers, got %d", len(handlers))
	}

	// 创建请求端
	requester := setupTestClient(t, addr, "lb-requester", nil)
	defer requester.Close()

	// 两个服务端都处理请求
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server1Count := 0
	server2Count := 0
	var mu sync.Mutex

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				req, err := server1.PollRequest(ctx, 100*time.Millisecond)
				if err != nil || req == nil {
					continue
				}
				mu.Lock()
				server1Count++
				mu.Unlock()
				req.RespondSuccess([]byte("server1"))
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				req, err := server2.PollRequest(ctx, 100*time.Millisecond)
				if err != nil || req == nil {
					continue
				}
				mu.Lock()
				server2Count++
				mu.Unlock()
				req.RespondSuccess([]byte("server2"))
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// 发送多个请求
	const numRequests = 10
	for i := 0; i < numRequests; i++ {
		resp, err := requester.Request(context.Background(), "lb.action", []byte("data"), client.NewRequestOptions().WithTimeout(2*time.Second))
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		} else if !resp.IsSuccess() {
			t.Errorf("Request %d got error: %v", i, resp.Error())
		}
	}

	cancel()
	time.Sleep(100 * time.Millisecond)

	// 验证负载分布（轮询应该大致均匀）
	mu.Lock()
	total := server1Count + server2Count
	mu.Unlock()

	if total != numRequests {
		t.Errorf("Expected %d total requests handled, got %d", numRequests, total)
	}

	t.Logf("Load distribution: server1=%d, server2=%d", server1Count, server2Count)
}

// ============================================================================
// 错误处理测试
// ============================================================================

func TestRespondErrorWithCode(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	server := setupTestClient(t, addr, "error-server", nil)
	defer server.Close()

	requester := setupTestClient(t, addr, "error-requester", nil)
	defer requester.Close()

	server.Register("error.test")
	time.Sleep(50 * time.Millisecond)

	// 服务端响应错误
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := server.PollRequest(ctx, 3*time.Second)
		if err != nil || req == nil {
			return
		}

		req.RespondError(packet.ReasonImplementationError, "Something went wrong")
	}()

	time.Sleep(100 * time.Millisecond)

	resp, err := requester.Request(context.Background(), "error.test", []byte("data"), nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.IsSuccess() {
		t.Error("Expected error response")
	}
	if resp.ReasonCode != packet.ReasonImplementationError {
		t.Errorf("Expected ReasonImplementationError, got %v", resp.ReasonCode)
	}
	if resp.ReasonString != "Something went wrong" {
		t.Errorf("Expected reason 'Something went wrong', got '%s'", resp.ReasonString)
	}
}

// ============================================================================
// 边界情况测试
// ============================================================================

func TestEmptyPayload(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	server := setupTestClient(t, addr, "empty-server", nil)
	defer server.Close()

	requester := setupTestClient(t, addr, "empty-requester", nil)
	defer requester.Close()

	server.Register("empty.test")
	time.Sleep(50 * time.Millisecond)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := server.PollRequest(ctx, 3*time.Second)
		if err != nil || req == nil {
			return
		}

		// 验证空 payload
		if len(req.Payload) != 0 {
			t.Errorf("Expected empty payload, got %d bytes", len(req.Payload))
		}

		// 响应空 payload
		req.RespondSuccess(nil)
	}()

	time.Sleep(100 * time.Millisecond)

	resp, err := requester.Request(context.Background(), "empty.test", nil, nil)
	if err != nil {
		t.Fatalf("Request with empty payload failed: %v", err)
	}

	if !resp.IsSuccess() {
		t.Errorf("Expected success, got: %v", resp.Error())
	}
	if len(resp.Payload) != 0 {
		t.Errorf("Expected empty response payload, got %d bytes", len(resp.Payload))
	}
}

func TestLargePayload(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	server := setupTestClient(t, addr, "large-server", nil)
	defer server.Close()

	requester := setupTestClient(t, addr, "large-requester", nil)
	defer requester.Close()

	server.Register("large.test")
	time.Sleep(50 * time.Millisecond)

	// 创建大 payload (1MB)
	largePayload := make([]byte, 1024*1024)
	for i := range largePayload {
		largePayload[i] = byte(i % 256)
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		req, err := server.PollRequest(ctx, 5*time.Second)
		if err != nil || req == nil {
			return
		}

		// 验证收到完整 payload
		if len(req.Payload) != len(largePayload) {
			t.Errorf("Expected %d bytes, got %d", len(largePayload), len(req.Payload))
		}

		// 响应同样大的 payload
		req.RespondSuccess(req.Payload)
	}()

	time.Sleep(100 * time.Millisecond)

	resp, err := requester.Request(context.Background(), "large.test", largePayload, client.NewRequestOptions().WithTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("Large payload request failed: %v", err)
	}

	if !resp.IsSuccess() {
		t.Errorf("Expected success, got: %v", resp.Error())
	}
	if len(resp.Payload) != len(largePayload) {
		t.Errorf("Expected %d bytes in response, got %d", len(largePayload), len(resp.Payload))
	}
}

func TestSpecialCharactersInAction(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	server := setupTestClient(t, addr, "special-server", nil)
	defer server.Close()

	requester := setupTestClient(t, addr, "special-requester", nil)
	defer requester.Close()

	// 使用包含特殊字符的 action 名称
	actionName := "com.example/service:method-v1.0"
	server.Register(actionName)
	time.Sleep(50 * time.Millisecond)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := server.PollRequest(ctx, 3*time.Second)
		if err != nil || req == nil {
			return
		}

		if req.Action != actionName {
			t.Errorf("Expected action '%s', got '%s'", actionName, req.Action)
		}

		req.RespondSuccess([]byte("ok"))
	}()

	time.Sleep(100 * time.Millisecond)

	resp, err := requester.Request(context.Background(), actionName, []byte("data"), nil)
	if err != nil {
		t.Fatalf("Request with special action name failed: %v", err)
	}

	if !resp.IsSuccess() {
		t.Errorf("Expected success, got: %v", resp.Error())
	}
}

// ============================================================================
// RequestOptions Builder 测试
// ============================================================================

// TestRequestOptionsBuilder 测试 RequestOptions 的链式构建方法
func TestRequestOptionsBuilder(t *testing.T) {
	opts := DefaultRequestOptions().
		WithTargetClientID("target-client").
		WithTimeout(60*time.Second).
		WithTraceID("trace-abc").
		WithContentType("application/json").
		WithUserProperty("key1", "value1").
		WithUserProperty("key2", "value2")

	if opts.TargetClientID != "target-client" {
		t.Errorf("Expected TargetClientID 'target-client', got '%s'", opts.TargetClientID)
	}
	if opts.Timeout != 60*time.Second {
		t.Errorf("Expected Timeout 60s, got %v", opts.Timeout)
	}
	if opts.TraceID != "trace-abc" {
		t.Errorf("Expected TraceID 'trace-abc', got '%s'", opts.TraceID)
	}
	if opts.ContentType != "application/json" {
		t.Errorf("Expected ContentType 'application/json', got '%s'", opts.ContentType)
	}
	if opts.UserProperties["key1"] != "value1" {
		t.Errorf("Expected UserProperties[key1]='value1', got '%s'", opts.UserProperties["key1"])
	}
	if opts.UserProperties["key2"] != "value2" {
		t.Errorf("Expected UserProperties[key2]='value2', got '%s'", opts.UserProperties["key2"])
	}
}

// TestRequestOptionsWithUserProperties 测试批量设置用户属性
func TestRequestOptionsWithUserProperties(t *testing.T) {
	props := map[string]string{
		"prop1": "val1",
		"prop2": "val2",
		"prop3": "val3",
	}

	opts := DefaultRequestOptions().WithUserProperties(props)

	if len(opts.UserProperties) != 3 {
		t.Errorf("Expected 3 user properties, got %d", len(opts.UserProperties))
	}
	for k, v := range props {
		if opts.UserProperties[k] != v {
			t.Errorf("Expected UserProperties[%s]='%s', got '%s'", k, v, opts.UserProperties[k])
		}
	}
}

// ============================================================================
// core Request 路由测试
// ============================================================================

// TestCoreRequestByAction 测试 core 通过 action 路由发送请求
func TestCoreRequestByAction(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建服务端客户端并注册 action
	server := setupTestClient(t, addr, "action-server", nil)
	defer server.Close()

	server.Register("core.echo")
	time.Sleep(50 * time.Millisecond)

	// 服务端处理请求
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := server.PollRequest(ctx, 3*time.Second)
		if err != nil || req == nil {
			return
		}

		// 响应：原样返回数据
		req.RespondSuccess(append([]byte("echo:"), req.Payload...))
	}()

	time.Sleep(100 * time.Millisecond)

	// core 通过 action 路由发送请求（不指定 targetClientID）
	resp, err := core.Request(context.Background(), "core.echo", []byte("hello"), nil)
	if err != nil {
		t.Fatalf("RequestByAction failed: %v", err)
	}

	if !resp.IsSuccess() {
		t.Fatalf("Expected success, got: %v", resp.Error())
	}

	expected := "echo:hello"
	if string(resp.Payload) != expected {
		t.Errorf("Expected payload '%s', got '%s'", expected, string(resp.Payload))
	}
}

// TestCoreRequestToClient 测试 core 直接向指定客户端发送请求
func TestCoreRequestToClientDirect(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建服务端客户端（不需要注册 action，直接通过 clientID 路由）
	server := setupTestClient(t, addr, "direct-server", nil)
	defer server.Close()

	// 服务端处理请求
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := server.PollRequest(ctx, 3*time.Second)
		if err != nil || req == nil {
			return
		}

		// 响应：返回确认消息
		req.RespondSuccess([]byte("received:" + string(req.Payload)))
	}()

	time.Sleep(100 * time.Millisecond)

	// core 直接向指定客户端发送请求
	resp, err := core.RequestToClient(context.Background(), "direct-server", "any.action", []byte("test-data"), nil)
	if err != nil {
		t.Fatalf("RequestToClient failed: %v", err)
	}

	if !resp.IsSuccess() {
		t.Fatalf("Expected success, got: %v", resp.Error())
	}

	expected := "received:test-data"
	if string(resp.Payload) != expected {
		t.Errorf("Expected payload '%s', got '%s'", expected, string(resp.Payload))
	}
}

// TestCoreRequestActionNotFound 测试请求未注册的 action
func TestCoreRequestActionNotFound(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	// 使用 RequestByAction 请求一个未注册的 action
	_, err := core.Request(context.Background(), "nonexistent.action", []byte("data"), nil)
	if err == nil {
		t.Fatal("Expected error for unregistered action, got nil")
	}

	// 使用 errors.Is() 检查错误类型
	if !errors.Is(err, ErrActionNotFound) {
		t.Errorf("Expected ErrActionNotFound, got: %v", err)
	}

	// 也可以使用类型断言获取详细信息
	var actionErr *ActionNotFoundError
	if errors.As(err, &actionErr) {
		if actionErr.Action != "nonexistent.action" {
			t.Errorf("Expected action 'nonexistent.action', got '%s'", actionErr.Action)
		}
	}
}

// TestCoreRequestClientNotFound 测试请求不存在的客户端
func TestCoreRequestClientNotFound(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	// 使用 RequestToClient 请求一个不存在的客户端
	_, err := core.RequestToClient(context.Background(), "nonexistent-client", "test.action", []byte("data"), nil)
	if err == nil {
		t.Fatal("Expected error for nonexistent client, got nil")
	}

	// 使用 errors.Is() 检查错误类型
	if !errors.Is(err, ErrClientNotFound) {
		t.Errorf("Expected ErrClientNotFound, got: %v", err)
	}

	// 也可以使用类型断言获取详细信息
	var clientErr *ClientNotFoundError
	if errors.As(err, &clientErr) {
		if clientErr.ClientID != "nonexistent-client" {
			t.Errorf("Expected clientID 'nonexistent-client', got '%s'", clientErr.ClientID)
		}
	}
}

// TestCoreRequestWithFullOptions 测试使用完整选项发送请求
func TestCoreRequestWithFullOptions(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	server := setupTestClient(t, addr, "full-opts-server", nil)
	defer server.Close()

	server.Register("full.test")
	time.Sleep(50 * time.Millisecond)

	// 服务端验证请求属性
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := server.PollRequest(ctx, 3*time.Second)
		if err != nil || req == nil {
			return
		}

		// 验证属性被正确传递
		if req.TraceID != "test-trace-id" {
			t.Errorf("Expected TraceID 'test-trace-id', got '%s'", req.TraceID)
		}
		if req.ContentType != "application/xml" {
			t.Errorf("Expected ContentType 'application/xml', got '%s'", req.ContentType)
		}

		// 返回带属性的响应
		resp := client.NewResponse([]byte("ok")).
			WithTraceID(req.TraceID).
			WithContentType("application/xml").
			WithUserProperty("server-key", "server-value")

		req.Response(resp)
	}()

	time.Sleep(100 * time.Millisecond)

	// 使用完整选项发送请求
	opts := DefaultRequestOptions().
		WithTimeout(10*time.Second).
		WithTraceID("test-trace-id").
		WithContentType("application/xml").
		WithUserProperty("client-key", "client-value")

	resp, err := core.Request(context.Background(), "full.test", []byte("data"), opts)
	if err != nil {
		t.Fatalf("Request with full options failed: %v", err)
	}

	if !resp.IsSuccess() {
		t.Fatalf("Expected success, got: %v", resp.Error())
	}

	// 验证响应属性
	if resp.TraceID != "test-trace-id" {
		t.Errorf("Expected TraceID 'test-trace-id', got '%s'", resp.TraceID)
	}
	if resp.ContentType != "application/xml" {
		t.Errorf("Expected ContentType 'application/xml', got '%s'", resp.ContentType)
	}
	if resp.UserProperties["server-key"] != "server-value" {
		t.Errorf("Expected UserProperties[server-key]='server-value', got '%s'", resp.UserProperties["server-key"])
	}
}

// TestCoreRequestLoadBalancingByAction 测试通过 action 路由的负载均衡
func TestCoreRequestLoadBalancingByAction(t *testing.T) {
	core := setupTestCore(t, nil)
	defer core.Stop()

	addr := core.GetAddress()

	// 创建多个服务端客户端，注册同一个 action
	servers := make([]*client.Client, 3)
	serverCounts := make([]int, 3)
	var countMu sync.Mutex

	for i := 0; i < 3; i++ {
		servers[i] = setupTestClient(t, addr, fmt.Sprintf("lb-server-%d", i), nil)
		defer servers[i].Close()
		servers[i].Register("lb.action")

		// 启动每个服务端的请求处理
		go func(idx int, server *client.Client) {
			for {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				req, err := server.PollRequest(ctx, 2*time.Second)
				cancel()

				if err != nil {
					return
				}
				if req == nil {
					continue
				}

				countMu.Lock()
				serverCounts[idx]++
				countMu.Unlock()

				req.RespondSuccess([]byte(fmt.Sprintf("server-%d", idx)))
			}
		}(i, servers[i])
	}

	time.Sleep(100 * time.Millisecond)

	// 发送多个请求
	requestCount := 12
	for i := 0; i < requestCount; i++ {
		resp, err := core.Request(context.Background(), "lb.action", []byte("test"), nil)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		if !resp.IsSuccess() {
			t.Errorf("Request %d: expected success, got: %v", i, resp.Error())
		}
	}

	time.Sleep(100 * time.Millisecond)

	// 验证负载均衡（每个服务端至少处理一些请求）
	countMu.Lock()
	defer countMu.Unlock()

	totalHandled := 0
	for i, count := range serverCounts {
		totalHandled += count
		t.Logf("Server %d handled %d requests", i, count)
	}

	if totalHandled != requestCount {
		t.Errorf("Expected %d total requests handled, got %d", requestCount, totalHandled)
	}

	// 检查负载是否分布（至少有2个服务端处理了请求）
	activeServers := 0
	for _, count := range serverCounts {
		if count > 0 {
			activeServers++
		}
	}
	if activeServers < 2 {
		t.Errorf("Expected load to be distributed across at least 2 servers, only %d server(s) handled requests", activeServers)
	}
}

// ============================================================================
// 错误类型测试
// ============================================================================

// TestErrorTypes 测试自定义错误类型
func TestErrorTypes(t *testing.T) {
	t.Run("ClientNotFoundError", func(t *testing.T) {
		err := NewClientNotFoundError("test-client")

		// 测试 errors.Is
		if !errors.Is(err, ErrClientNotFound) {
			t.Error("Expected errors.Is(err, ErrClientNotFound) to be true")
		}

		// 测试 errors.As
		var clientErr *ClientNotFoundError
		if !errors.As(err, &clientErr) {
			t.Error("Expected errors.As to succeed")
		}
		if clientErr.ClientID != "test-client" {
			t.Errorf("Expected ClientID 'test-client', got '%s'", clientErr.ClientID)
		}

		// 测试 Error() 消息
		expectedMsg := "client not found: test-client"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
		}
	})

	t.Run("ActionNotFoundError", func(t *testing.T) {
		err := NewActionNotFoundError("test.action")

		if !errors.Is(err, ErrActionNotFound) {
			t.Error("Expected errors.Is(err, ErrActionNotFound) to be true")
		}

		var actionErr *ActionNotFoundError
		if !errors.As(err, &actionErr) {
			t.Error("Expected errors.As to succeed")
		}
		if actionErr.Action != "test.action" {
			t.Errorf("Expected Action 'test.action', got '%s'", actionErr.Action)
		}
	})

	t.Run("NoAvailableHandlerError", func(t *testing.T) {
		err := NewNoAvailableHandlerError("busy.action")

		if !errors.Is(err, ErrNoAvailableHandler) {
			t.Error("Expected errors.Is(err, ErrNoAvailableHandler) to be true")
		}

		var handlerErr *NoAvailableHandlerError
		if !errors.As(err, &handlerErr) {
			t.Error("Expected errors.As to succeed")
		}
		if handlerErr.Action != "busy.action" {
			t.Errorf("Expected Action 'busy.action', got '%s'", handlerErr.Action)
		}
	})

	t.Run("RequestTimeoutError", func(t *testing.T) {
		err := NewRequestTimeoutError(5 * time.Second)

		var timeoutErr *RequestTimeoutError
		if !errors.As(err, &timeoutErr) {
			t.Error("Expected errors.As to succeed")
		}

		expectedMsg := "request timeout after 5s"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
		}
	})

	t.Run("ClientClosedError", func(t *testing.T) {
		err := NewClientClosedError("closed-client")

		if !errors.Is(err, ErrClientClosed) {
			t.Error("Expected errors.Is(err, ErrClientClosed) to be true")
		}

		var closedErr *ClientClosedError
		if !errors.As(err, &closedErr) {
			t.Error("Expected errors.As to succeed")
		}
		if closedErr.ClientID != "closed-client" {
			t.Errorf("Expected ClientID 'closed-client', got '%s'", closedErr.ClientID)
		}
	})

	t.Run("ClientNotAvailableError", func(t *testing.T) {
		err := NewClientNotAvailableError("unavailable-client")

		if !errors.Is(err, ErrClientNotAvailable) {
			t.Error("Expected errors.Is(err, ErrClientNotAvailable) to be true")
		}

		var unavailableErr *ClientNotAvailableError
		if !errors.As(err, &unavailableErr) {
			t.Error("Expected errors.As to succeed")
		}
		if unavailableErr.ClientID != "unavailable-client" {
			t.Errorf("Expected ClientID 'unavailable-client', got '%s'", unavailableErr.ClientID)
		}
	})
}

// TestCoreNotRunningError 测试 core 未运行错误
func TestCoreNotRunningError(t *testing.T) {
	core, err := NewWithOptions(NewCoreOptions().WithAddress("127.0.0.1:0"))
	if err != nil {
		t.Fatalf("Failed to create core: %v", err)
	}
	// 注意：不启动 core

	// 测试 PollRequest
	_, err = core.PollRequest(context.Background(), 100*time.Millisecond)
	if !errors.Is(err, ErrCoreNotRunning) {
		t.Errorf("Expected ErrCoreNotRunning, got: %v", err)
	}

	// 测试 PollMessage
	_, err = core.PollMessage(context.Background(), 100*time.Millisecond)
	if !errors.Is(err, ErrCoreNotRunning) {
		t.Errorf("Expected ErrCoreNotRunning, got: %v", err)
	}
}

// containsString 辅助函数：检查字符串是否包含子串
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
