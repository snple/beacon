package client

import (
	"net"
	"testing"

	"github.com/snple/beacon/core"
	"go.uber.org/zap"
)

// testLogger 创建测试用的 logger
func testLogger() *zap.Logger {
	logger, _ := zap.NewDevelopment()
	return logger
}

// testSetupCore 创建测试用的 Core 并返回监听地址
func testSetupCore(t *testing.T) (*core.Core, string) {
	t.Helper()

	opts := core.NewCoreOptions().
		WithLogger(testLogger()).
		WithConnectTimeout(5).
		WithRequestQueueSize(100)

	c, err := core.NewWithOptions(opts)
	if err != nil {
		t.Fatalf("Failed to create core: %v", err)
	}

	if err := c.Start(); err != nil {
		t.Fatalf("Failed to start core: %v", err)
	}

	// 启动监听器并获取地址
	addr := testServe(t, c)

	return c, addr
}

// testServe 在随机端口启动 TCP 监听器并返回地址
func testServe(t *testing.T, c *core.Core) string {
	t.Helper()
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	addr := listener.Addr().String()
	go c.Serve(listener)
	return addr
}

// testSetupClient 创建测试用的 Client
func testSetupClient(t *testing.T, coreAddr string, clientID string) *Client {
	t.Helper()

	opts := NewClientOptions().
		WithCore(coreAddr).
		WithClientID(clientID).
		WithLogger(testLogger()).
		WithRequestQueueSize(100)

	c, err := NewWithOptions(opts)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if err := c.Connect(); err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}

	return c
}
