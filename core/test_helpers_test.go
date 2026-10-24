package core

import (
	"net"
	"testing"
	"time"
)

// testServe 测试辅助函数：启动一个监听器并返回地址
// 用于替代旧的 GetAddress() 方法
func testServe(t *testing.T, c *Core) string {
	t.Helper()

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}

	addr := listener.Addr().String()

	// 在后台启动服务
	go func() {
		if err := c.Serve(listener); err != nil {
			t.Logf("Serve error: %v", err)
		}
	}()

	return addr
}

// eventually 轮询等待条件成立（用于替代固定 time.Sleep，降低 flaky 风险）。
// 每隔 pollInterval 检查一次 condition，直到 timeout 超时。
// 返回 false 表示超时；描述信息用于日志定位。
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
