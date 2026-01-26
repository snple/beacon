package core

import (
	"net"
	"testing"
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
