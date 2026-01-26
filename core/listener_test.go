package core

import (
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/client"
)

// TestHandleConn 测试 HandleConn API
func TestHandleConn(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions())
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	// 创建一个 listener
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	addr := listener.Addr().String()

	// 手动 accept 并调用 HandleConn
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Logf("Accept error: %v", err)
			return
		}
		if err := c.HandleConn(conn); err != nil {
			t.Logf("HandleConn error: %v", err)
		}
	}()

	// 创建客户端连接
	cli, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("test-handleconn"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	if err := cli.Connect(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// 验证客户端已连接
	if c.stats.ClientsConnected.Load() != 1 {
		t.Errorf("expected 1 client connected, got %d", c.stats.ClientsConnected.Load())
	}
}

// TestServeTCP 测试 ServeTCP 方法
func TestServeTCP(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions())
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	// 在后台启动 TCP 服务
	go func() {
		if err := c.ServeTCP(":0"); err != nil {
			t.Logf("ServeTCP error: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
}

// TestServe 测试 Serve 方法
func TestServe(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions())
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	addr := listener.Addr().String()

	// 在后台启动服务
	go func() {
		if err := c.Serve(listener); err != nil {
			t.Logf("Serve error: %v", err)
		}
	}()

	time.Sleep(50 * time.Millisecond)

	// 创建客户端连接
	cli, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("test-serve"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	if err := cli.Connect(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// 验证客户端已连接
	if c.stats.ClientsConnected.Load() != 1 {
		t.Errorf("expected 1 client connected, got %d", c.stats.ClientsConnected.Load())
	}
}

// TestMultipleListeners 测试同时使用多个 listener
func TestMultipleListeners(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions().WithMaxClients(100))
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	// 创建多个 listener
	listener1, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener1.Close()

	listener2, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener2.Close()

	addr1 := listener1.Addr().String()
	addr2 := listener2.Addr().String()

	// 启动两个 listener
	go c.Serve(listener1)
	go c.Serve(listener2)

	time.Sleep(50 * time.Millisecond)

	// 连接到第一个 listener
	cli1, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr1).
			WithClientID("client1"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cli1.Close()

	if err := cli1.Connect(); err != nil {
		t.Fatal(err)
	}

	// 连接到第二个 listener
	cli2, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr2).
			WithClientID("client2"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cli2.Close()

	if err := cli2.Connect(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// 验证两个客户端都已连接
	connected := c.stats.ClientsConnected.Load()
	if connected != 2 {
		t.Errorf("expected 2 clients connected, got %d", connected)
	}
}

// TestHandleConnMaxClients 测试 HandleConn 的客户端数量限制
func TestHandleConnMaxClients(t *testing.T) {
	c, err := NewWithOptions(
		NewCoreOptions().WithMaxClients(1),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	addr := listener.Addr().String()

	// 启动服务
	go c.Serve(listener)

	time.Sleep(50 * time.Millisecond)

	// 连接第一个客户端（应该成功）
	cli1, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("client1"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cli1.Close()

	if err := cli1.Connect(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// 连接第二个客户端（应该被拒绝）
	cli2, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID("client2").
			WithConnectTimeout(1),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cli2.Close()

	// 第二个连接应该失败或超时
	err = cli2.Connect()
	if err == nil {
		t.Error("expected second client to be rejected, but it connected successfully")
	}

	// 验证只有一个客户端连接
	if c.stats.ClientsConnected.Load() != 1 {
		t.Errorf("expected 1 client connected, got %d", c.stats.ClientsConnected.Load())
	}
}
