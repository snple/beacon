package client

import (
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestConn_WriteTimeoutClosesConnection 验证对端不读数据时客户端写操作
// 会在超时后返回写错误并关闭底层连接。
func TestConn_WriteTimeoutClosesConnection(t *testing.T) {
	c, err := NewWithOptions(NewClientOptions().WithWriteTimeout(100 * time.Millisecond))
	require.NoError(t, err)
	defer c.Close()

	a, b := net.Pipe()
	defer b.Close()

	conn := newConn(c, a, packet.NewConnackPacket(packet.ReasonSuccess))

	start := time.Now()
	err = conn.writePacket(packet.NewPingPacket(1, 1))
	elapsed := time.Since(start)

	require.Error(t, err)
	var writeErr *packet.WriteError
	require.ErrorAs(t, err, &writeErr)
	require.Less(t, elapsed, 2*time.Second, "write should time out quickly")

	_ = b.SetReadDeadline(time.Now().Add(time.Second))
	_, readErr := b.Read(make([]byte, 1))
	require.Error(t, readErr, "peer should observe connection close")
}

// TestConn_RemoteEOFTriggersDisconnect 验证对端直接关闭 TCP（没有 DISCONNECT）
// 时客户端也会进入未连接状态，从而触发自动重连。
func TestConn_RemoteEOFTriggersDisconnect(t *testing.T) {
	c, err := NewWithOptions(NewClientOptions())
	require.NoError(t, err)
	defer c.Close()

	a, b := net.Pipe()
	conn := newConn(c, a, packet.NewConnackPacket(packet.ReasonSuccess))

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()
	c.connected.Store(true)

	conn.start()

	// 对端关闭 → 客户端读到 EOF
	require.NoError(t, b.Close())

	require.True(t, eventually(t, 3*time.Second, 20*time.Millisecond, "client marked disconnected", func() bool {
		return !c.IsConnected()
	}), "client should mark itself disconnected after remote EOF")
}
