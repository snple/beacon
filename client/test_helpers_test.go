package client

import (
	"net"
	"testing"
	"time"

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
		WithConnectTimeout(5)

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

// eventually 轮询等待条件成立（用于替代固定 time.Sleep，降低 flaky 风险）。
// 每隔 pollInterval 检查一次 condition，直到 timeout 超时。
func eventually(t *testing.T, timeout, pollInterval time.Duration, desc string, condition func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(pollInterval)
	}
	t.Logf("eventually timeout: %s", desc)
	return false
}

// testSetupClient 创建测试用的 Client
func testSetupClient(t *testing.T, coreAddr string, clientID string) *Client {
	t.Helper()

	opts := NewClientOptions().
		WithCore(coreAddr).
		WithClientID(clientID).
		WithLogger(testLogger())

	c, err := NewWithOptions(opts)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	dialer := &TCPDialer{
		Address:     coreAddr,
		DialTimeout: 10 * time.Second,
	}
	if err := c.ConnectWithDialer(dialer); err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}

	return c
}
