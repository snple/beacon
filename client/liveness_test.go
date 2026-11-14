package client_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
)

// 服务端不再响应任何包（但 TCP 仍保持连接）时，客户端必须主动判定连接失效。
func TestClientDetectsUnresponsiveServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	go func() {
		nc, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = nc.Close() }()

		if _, err := packet.ReadPacket(nc, 1<<20); err != nil {
			return
		}
		connack := packet.NewConnackPacket(packet.ReasonSuccess)
		connack.KeepAlive = 1
		connack.Limits.MaxPacketSize = 1 << 20
		connack.AssignedClientID = "live-1"
		connack.SessionID = 1
		if err := packet.WritePacket(nc, connack, 0); err != nil {
			return
		}

		// 只读取 PING/PULL，不发送任何响应，也不关闭连接。
		buf := make([]byte, 4096)
		for {
			if _, err := nc.Read(buf); err != nil {
				return
			}
		}
	}()

	opts := client.NewClientOptions().
		WithCore(ln.Addr().String()).
		WithClientID("live-1").
		WithKeepAlive(1).
		WithAutoReconnect(false)
	c, err := client.NewWithOptions(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.ConnectContext(ctx); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for c.Connected() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if c.Connected() {
		t.Fatal("client did not detect an unresponsive (half-open) connection")
	}
}
