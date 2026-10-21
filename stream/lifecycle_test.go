package stream

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/stream/packet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestStreamCore(t *testing.T, opts ...CoreOption) *Core {
	t.Helper()

	logger, _ := zap.NewDevelopment()
	all := append([]CoreOption{WithAddress("127.0.0.1:0"), WithCoreLogger(logger)}, opts...)

	c, err := NewCore(all...)
	require.NoError(t, err)
	require.NoError(t, c.Start())
	return c
}

// TestCoreStop_ClosesActiveStreams 验证存在活跃流时 Stop 不会阻塞。
func TestCoreStop_ClosesActiveStreams(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	core := newTestStreamCore(t)

	clientA, err := NewClient(WithCoreAddr(core.GetAddress()), WithClientID("stop-source"), WithLogger(logger))
	require.NoError(t, err)
	defer clientA.Close()

	clientB, err := NewClient(WithCoreAddr(core.GetAddress()), WithClientID("stop-target"), WithLogger(logger))
	require.NoError(t, err)
	defer clientB.Close()

	accepted := make(chan *Stream, 1)
	go func() {
		s, err := clientB.AcceptStream()
		if err == nil {
			accepted <- s
		}
	}()

	time.Sleep(300 * time.Millisecond)

	stream, err := clientA.OpenStream("stop-target", "stop/topic", nil, nil)
	require.NoError(t, err)
	defer stream.Close()

	select {
	case <-accepted:
	case <-time.After(3 * time.Second):
		t.Fatal("target did not accept stream")
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = core.Stop()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Core.Stop blocked with an active stream")
	}
}

// TestCoreMaxWaitingConns 验证等待连接总数受 MaxWaitingConns 限制。
func TestCoreMaxWaitingConns(t *testing.T) {
	core := newTestStreamCore(t, WithMaxWaitingConns(1))
	defer core.Stop()

	addr := core.GetAddress()

	wait1 := dialAndWait(t, addr, "wait-1")
	defer wait1.Close()

	// 第二个等待连接应被拒绝并关闭。
	wait2 := dialAndWait(t, addr, "wait-2")
	defer wait2.Close()

	_ = wait2.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err := packet.ReadPacket(wait2)
	require.Error(t, err, "second waiting connection should be closed")

	// 第一个连接仍然保持等待（读取超时而不是 EOF）。
	_ = wait1.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, err = packet.ReadPacket(wait1)
	var netErr net.Error
	require.True(t, errors.As(err, &netErr) && netErr.Timeout(), "first waiter should still be open, got %v", err)
}

func dialAndWait(t *testing.T, addr, clientID string) net.Conn {
	t.Helper()

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)

	connect := &packet.ConnectPacket{ClientID: clientID, Properties: map[string]string{}}
	require.NoError(t, packet.WritePacket(conn, connect))

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	pkt, err := packet.ReadPacket(conn)
	require.NoError(t, err)
	ack, ok := pkt.(*packet.ConnackPacket)
	require.True(t, ok)
	require.Equal(t, packet.Success, ack.ReasonCode)
	require.NoError(t, conn.SetReadDeadline(time.Time{}))

	require.NoError(t, packet.WritePacket(conn, &packet.StreamReadyPacket{Properties: map[string]string{}}))
	return conn
}

// TestClientAcceptStream_StopsOnAuthRejection 验证认证被拒绝时 AcceptStream 立即返回，
// 而不是每 5 秒无限重试。
func TestClientAcceptStream_StopsOnAuthRejection(t *testing.T) {
	core := newTestStreamCore(t, WithAuthHandler(func(_, _ string, _ []byte) error {
		return errors.New("denied")
	}))
	defer core.Stop()

	cl, err := NewClient(WithCoreAddr(core.GetAddress()), WithClientID("denied"))
	require.NoError(t, err)
	defer cl.Close()

	result := make(chan error, 1)
	go func() {
		_, err := cl.AcceptStream()
		result <- err
	}()

	select {
	case err := <-result:
		require.ErrorIs(t, err, ErrAuthRejected)
	case <-time.After(3 * time.Second):
		t.Fatal("AcceptStream should return immediately on auth rejection")
	}
}
