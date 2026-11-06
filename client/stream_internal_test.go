package client

import (
	"context"
	"net"
	"testing"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestOpenStreamRejectsOldServer(t *testing.T) {
	c, err := NewWithOptions(NewClientOptions().
		WithStream(true).
		WithLogger(zap.NewNop()))
	require.NoError(t, err)

	local, remote := net.Pipe()
	defer local.Close()
	defer remote.Close()

	cn := newConn(c, local)
	// 旧服务端不会置 StreamSupported。
	cn.connack = packet.NewConnackPacket(packet.ReasonSuccess)

	c.mu.Lock()
	c.conn = cn
	c.mu.Unlock()

	_, err = c.OpenStream(context.Background(), "b", "x", nil, nil)
	require.ErrorIs(t, err, ErrStreamNotSupported)
}

func TestHandleStreamAbortClosesWait(t *testing.T) {
	c, err := NewWithOptions(NewClientOptions().
		WithStream(true).
		WithLogger(zap.NewNop()))
	require.NoError(t, err)

	local, remote := net.Pipe()
	defer local.Close()
	defer remote.Close()

	wait := c.registerStreamWait(7)
	wait.setConn(local)

	c.handleStreamAbort(&packet.StreamAbortPacket{
		StreamID:   7,
		ReasonCode: packet.ReasonStreamTimeout,
	})
	require.True(t, wait.isAborted())
}

func TestStreamRequestOnceSemantics(t *testing.T) {
	c, err := NewWithOptions(NewClientOptions().
		WithStream(true).
		WithLogger(zap.NewNop()))
	require.NoError(t, err)

	rejected := &StreamRequest{client: c}
	require.ErrorIs(t, rejected.Reject(packet.ReasonStreamRejected, "no"), ErrNotConnected)
	_, err = rejected.Accept(context.Background(), nil, nil)
	require.ErrorIs(t, err, ErrStreamAlreadyHandled)

	accepted := &StreamRequest{client: c}
	_, err = accepted.Accept(context.Background(), nil, nil)
	require.ErrorIs(t, err, ErrNotConnected)
	require.ErrorIs(t, accepted.Reject(packet.ReasonStreamRejected, "no"), ErrStreamAlreadyHandled)
}
