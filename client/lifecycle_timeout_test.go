package client

import (
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// pipeDialer 返回预先建好的连接（net.Pipe），用于构造不响应的服务端。
type pipeDialer struct {
	conn net.Conn
}

func (d *pipeDialer) Dial() (net.Conn, error) { return d.conn, nil }

// TestDisconnectAfterReconnect 验证 Disconnect 在重连之后仍然有效。
// 回归：Client.close 曾用 sync.Once 保护，导致第二次 Disconnect 静默失效
// （连接不关闭、c.conn 不复位）。
func TestDisconnectAfterReconnect(t *testing.T) {
	core, addr := testSetupCore(t)
	defer core.Stop()

	c, err := NewWithOptions(NewClientOptions().
		WithClientID("redial-client").
		WithLogger(zap.NewNop()))
	require.NoError(t, err)
	defer c.Close()

	dialer := &TCPDialer{Address: addr, DialTimeout: 5 * time.Second}

	require.NoError(t, c.ConnectWithDialer(dialer))
	require.NoError(t, c.Disconnect())
	require.False(t, c.IsConnected())

	require.NoError(t, c.ConnectWithDialer(dialer))
	require.True(t, c.IsConnected())

	require.NoError(t, c.Disconnect())
	require.False(t, c.IsConnected())

	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()
	require.Nil(t, conn, "second Disconnect must clear the active connection")
}

// TestSubscribeAndUnsubscribeTimeoutErrors 验证订阅/取消订阅超时返回各自的错误，
// 而不是复用 ErrPublishTimeout。
func TestSubscribeAndUnsubscribeTimeoutErrors(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	// 服务端只完成握手，之后吞掉所有包但不回复。
	go func() {
		if _, err := packet.ReadPacket(serverConn, 0); err != nil {
			return
		}
		if err := packet.WritePacket(serverConn, packet.NewConnackPacket(packet.ReasonSuccess), 0); err != nil {
			return
		}
		for {
			if _, err := packet.ReadPacket(serverConn, 0); err != nil {
				return
			}
		}
	}()

	c, err := NewWithOptions(NewClientOptions().
		WithClientID("timeout-client").
		WithLogger(zap.NewNop()).
		WithPublishTimeout(50 * time.Millisecond))
	require.NoError(t, err)
	defer c.Close()

	require.NoError(t, c.ConnectWithDialer(&pipeDialer{conn: clientConn}))

	require.ErrorIs(t, c.Subscribe("timeout/topic"), ErrSubscribeTimeout)
	require.ErrorIs(t, c.Unsubscribe("timeout/topic"), ErrUnsubscribeTimeout)
}
