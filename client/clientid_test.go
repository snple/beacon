package client

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestClient_RejectsInvalidClientID 验证客户端在握手前就拒绝非法 ClientID。
func TestClient_RejectsInvalidClientID(t *testing.T) {
	c, err := NewWithOptions(NewClientOptions().WithClientID("bad:id"))
	require.NoError(t, err)
	defer c.Close()

	a, b := net.Pipe()
	defer b.Close()

	err = c.ConnectWithConn(a)
	require.ErrorIs(t, err, ErrClientIDInvalid)
}
