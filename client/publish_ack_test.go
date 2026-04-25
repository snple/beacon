package client

import (
	"context"
	"testing"
	"time"

	"github.com/danclive/nson-go"
	"github.com/stretchr/testify/require"
)

func TestWaitForPublishAck_AckArrivesBeforeWaiting(t *testing.T) {
	client := &Client{ctx: context.Background()}
	connCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn := &Conn{
		client:     client,
		pendingAck: make(map[nson.Id]chan error),
		ctx:        connCtx,
	}

	packetID := nson.NewId()
	waitCh := conn.registerPendingAck(packetID)

	conn.handleAck(packetID, nil)

	err := client.waitForPublishAck(conn, packetID, waitCh, 50*time.Millisecond)
	require.NoError(t, err)

	conn.pendingAckMu.Lock()
	_, exists := conn.pendingAck[packetID]
	conn.pendingAckMu.Unlock()
	require.False(t, exists)
}
