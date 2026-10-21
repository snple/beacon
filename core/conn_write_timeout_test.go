package core

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/packet"
)

// TestConn_WriteTimeoutClosesConnection 验证对端不读数据时写操作会在超时后
// 返回写错误，并关闭底层连接（而不是永久阻塞发送协程）。
func TestConn_WriteTimeoutClosesConnection(t *testing.T) {
	c := testSetupCore(t, NewCoreOptions().WithWriteTimeout(100*time.Millisecond))
	defer c.Stop()

	a, b := net.Pipe()
	defer b.Close()

	connect := packet.NewConnectPacket()
	connect.ClientID = "write-timeout"
	client := newClient(a, connect, c)
	conn := client.getConn()

	start := time.Now()
	err := conn.writePacket(packet.NewPingPacket(1, 1))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected write error, got nil")
	}
	var writeErr *packet.WriteError
	if !errors.As(err, &writeErr) {
		t.Fatalf("expected *packet.WriteError, got %T: %v", err, err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("write took %v, expected to time out quickly", elapsed)
	}

	// writePacket 应关闭底层连接，让读协程退出。
	_ = b.SetReadDeadline(time.Now().Add(time.Second))
	if _, readErr := b.Read(make([]byte, 1)); readErr == nil {
		t.Fatal("expected peer to observe connection close")
	}
}

// TestCore_StopUnblocksStuckWriter 验证停机不会被阻塞在写操作上的发送协程卡住。
func TestCore_StopUnblocksStuckWriter(t *testing.T) {
	c := testSetupCore(t, NewCoreOptions().WithWriteTimeout(time.Minute))

	a, b := net.Pipe()
	defer b.Close()

	connect := packet.NewConnectPacket()
	connect.ClientID = "stuck-writer"
	client := newClient(a, connect, c)

	c.clientsMu.Lock()
	c.clients[client.ID] = client
	c.clientsMu.Unlock()

	msg := &Message{
		Packet: packet.NewPublishPacket("stuck/topic", make([]byte, 1024)),
		QoS:    packet.QoS0,
	}
	if err := client.deliver(msg); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	// 等待发送协程真正进入阻塞写。
	time.Sleep(100 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c.Stop()
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop blocked on a stuck writer")
	}
}
