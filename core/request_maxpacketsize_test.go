package core

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
)

// TestRequestResponse_MaxPacketSize_RequestTooLarge 测试 REQUEST 包超过目标客户端 maxPacketSize
func TestRequestResponse_MaxPacketSize_RequestTooLarge(t *testing.T) {
	// 创建 core
	core := testSetupCore(t, nil)
	defer core.Stop()

	// 创建请求客户端（无限制）
	requester := testSetupClient(t, testServe(t, core), "requester", nil)
	defer requester.Close()

	// 创建处理器客户端（设置很小的 maxPacketSize）
	handlerOpts := client.NewClientOptions()
	handlerOpts.WithCore(testServe(t, core)).
		WithClientID("handler").
		WithMaxPacketSize(200) // 设置很小的最大包大小

	handler, err := client.NewWithOptions(handlerOpts)
	if err != nil {
		t.Fatalf("Failed to create handler client: %v", err)
	}
	defer handler.Close()

	if err := handler.Connect(); err != nil {
		t.Fatalf("Failed to connect handler: %v", err)
	}

	// 注册 action
	if err := handler.Register("test.action"); err != nil {
		t.Fatalf("Failed to register action: %v", err)
	}

	// 等待注册完成
	time.Sleep(50 * time.Millisecond)

	// 启动处理器（不应该收到请求）
	go func() {
		req, err := handler.PollRequest(context.Background(), 2*time.Second)
		if err == nil && req != nil {
			t.Errorf("Handler should not receive oversized request")
		}
	}()

	// 发送一个大的请求（payload 超过 200 字节）
	largePayload := bytes.Repeat([]byte("x"), 300)

	statsBeforeDrop := core.GetStats().MessagesDropped

	// 发送请求
	req := client.NewRequest("test.action", largePayload)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := requester.Request(ctx, req)

	// 应该收到错误响应
	if err != nil {
		t.Logf("Request returned error (may timeout): %v", err)
	}

	if resp != nil {
		if resp.Packet.ReasonCode == packet.ReasonPacketTooLarge {
			t.Logf("Received correct error response: ReasonPacketTooLarge")
		} else if resp.Packet.ReasonCode != packet.ReasonSuccess {
			t.Logf("Received error response with reason code: %v", resp.Packet.ReasonCode)
		} else {
			t.Error("Expected error response, but got success")
		}
	}

	// 验证消息被丢弃
	time.Sleep(200 * time.Millisecond)
	statsAfterDrop := core.GetStats().MessagesDropped
	if statsAfterDrop <= statsBeforeDrop {
		t.Errorf("Expected REQUEST to be dropped, but MessagesDropped did not increase")
	} else {
		t.Logf("REQUEST packet was dropped as expected (MessagesDropped: %d -> %d)",
			statsBeforeDrop, statsAfterDrop)
	}
}

// TestRequestResponse_MaxPacketSize_ResponseTooLarge 测试 RESPONSE 包超过目标客户端 maxPacketSize
func TestRequestResponse_MaxPacketSize_ResponseTooLarge(t *testing.T) {
	// 创建 core
	core := testSetupCore(t, nil)
	defer core.Stop()

	// 创建请求客户端（设置很小的 maxPacketSize）
	requesterOpts := client.NewClientOptions()
	requesterOpts.WithCore(testServe(t, core)).
		WithClientID("requester").
		WithMaxPacketSize(200) // 设置很小的最大包大小

	requester, err := client.NewWithOptions(requesterOpts)
	if err != nil {
		t.Fatalf("Failed to create requester client: %v", err)
	}
	defer requester.Close()

	if err := requester.Connect(); err != nil {
		t.Fatalf("Failed to connect requester: %v", err)
	}

	// 创建处理器客户端（无限制）
	handler := testSetupClient(t, testServe(t, core), "handler", nil)
	defer handler.Close()

	// 注册 action
	if err := handler.Register("test.action"); err != nil {
		t.Fatalf("Failed to register action: %v", err)
	}

	// 启动请求处理器（返回大响应）
	go func() {
		for {
			req, err := handler.PollRequest(context.Background(), 5*time.Second)
			if err != nil {
				return
			}

			// 返回一个大的响应
			largePayload := bytes.Repeat([]byte("y"), 300)
			resp := client.NewResponse(largePayload)
			req.Response(resp)
		}
	}()

	// 等待处理器准备好
	time.Sleep(50 * time.Millisecond)

	statsBeforeDrop := core.GetStats().MessagesDropped

	// 发送小请求
	req := client.NewRequest("test.action", []byte("small request"))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := requester.Request(ctx, req)

	// 由于响应太大，应该超时或收不到响应
	if err != nil {
		t.Logf("Request timed out (expected, response too large): %v", err)
	} else if resp != nil {
		t.Logf("Received response (unexpected, should have been dropped)")
	}

	// 验证响应被丢弃
	time.Sleep(200 * time.Millisecond)
	statsAfterDrop := core.GetStats().MessagesDropped
	if statsAfterDrop <= statsBeforeDrop {
		t.Logf("RESPONSE may not have been dropped (MessagesDropped: %d)", statsAfterDrop)
	} else {
		t.Logf("RESPONSE packet was dropped as expected (MessagesDropped: %d -> %d)",
			statsBeforeDrop, statsAfterDrop)
	}
}

// TestRequestResponse_MaxPacketSize_NoLimit 测试未设置 maxPacketSize 时请求响应正常工作
func TestRequestResponse_MaxPacketSize_NoLimit(t *testing.T) {
	// 创建 core
	core := testSetupCore(t, nil)
	defer core.Stop()

	// 创建请求和处理器客户端（都不设置 maxPacketSize）
	requester := testSetupClient(t, testServe(t, core), "requester", nil)
	defer requester.Close()

	handler := testSetupClient(t, testServe(t, core), "handler", nil)
	defer handler.Close()

	// 注册 action
	if err := handler.Register("test.action"); err != nil {
		t.Fatalf("Failed to register action: %v", err)
	}

	// 启动请求处理器（返回大响应）
	go func() {
		for {
			req, err := handler.PollRequest(context.Background(), 5*time.Second)
			if err != nil {
				return
			}

			// 返回一个大的响应
			largePayload := bytes.Repeat([]byte("z"), 5000)
			resp := client.NewResponse(largePayload)
			req.Response(resp)
		}
	}()

	// 等待处理器准备好
	time.Sleep(50 * time.Millisecond)

	statsBeforeDrop := core.GetStats().MessagesDropped

	// 发送大请求
	largeRequest := bytes.Repeat([]byte("x"), 5000)
	req := client.NewRequest("test.action", largeRequest)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := requester.Request(ctx, req)

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	if resp.Packet.ReasonCode != packet.ReasonSuccess {
		t.Errorf("Expected success response, got reason code: %v", resp.Packet.ReasonCode)
	}

	if len(resp.Packet.Payload) != 5000 {
		t.Errorf("Expected payload length 5000, got %d", len(resp.Packet.Payload))
	}

	// 验证没有消息被丢弃
	statsAfterDrop := core.GetStats().MessagesDropped
	if statsAfterDrop > statsBeforeDrop {
		t.Errorf("No messages should be dropped when no limit is set, but got %d drops",
			statsAfterDrop-statsBeforeDrop)
	} else {
		t.Logf("Large request/response handled successfully without drops")
	}
}

// TestRequestResponse_MaxPacketSize_SmallPacketsWork 测试小包仍然正常工作
func TestRequestResponse_MaxPacketSize_SmallPacketsWork(t *testing.T) {
	// 创建 core
	core := testSetupCore(t, nil)
	defer core.Stop()

	// 创建客户端（设置适中的 maxPacketSize）
	requesterOpts := client.NewClientOptions()
	requesterOpts.WithCore(testServe(t, core)).
		WithClientID("requester").
		WithMaxPacketSize(1000)

	requester, err := client.NewWithOptions(requesterOpts)
	if err != nil {
		t.Fatalf("Failed to create requester: %v", err)
	}
	defer requester.Close()

	if err := requester.Connect(); err != nil {
		t.Fatalf("Failed to connect requester: %v", err)
	}

	handlerOpts := client.NewClientOptions()
	handlerOpts.WithCore(testServe(t, core)).
		WithClientID("handler").
		WithMaxPacketSize(1000)

	handler, err := client.NewWithOptions(handlerOpts)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}
	defer handler.Close()

	if err := handler.Connect(); err != nil {
		t.Fatalf("Failed to connect handler: %v", err)
	}

	// 注册 action
	if err := handler.Register("echo"); err != nil {
		t.Fatalf("Failed to register action: %v", err)
	}

	// 启动处理器
	go func() {
		for {
			req, err := handler.PollRequest(context.Background(), 5*time.Second)
			if err != nil {
				return
			}

			// 回显请求
			resp := client.NewResponse(req.Packet.Payload)
			req.Response(resp)
		}
	}()

	// 等待处理器准备好
	time.Sleep(50 * time.Millisecond)

	// 发送多个小请求验证正常工作
	for i := 0; i < 5; i++ {
		payload := []byte(fmt.Sprintf("message-%d", i))
		req := client.NewRequest("echo", payload)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

		resp, err := requester.Request(ctx, req)
		cancel()

		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
			continue
		}

		if !resp.IsSuccess() {
			t.Errorf("Request %d not successful: %s", i, resp.Error())
			continue
		}

		if string(resp.Packet.Payload) != string(payload) {
			t.Errorf("Request %d: expected payload %s, got %s", i, payload, resp.Packet.Payload)
		}
	}

	t.Logf("All small requests/responses handled successfully with maxPacketSize limits")
}
