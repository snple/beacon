package client

import (
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

func TestHandshake_WithWillSimpleAndExpiry_DoesNotMutateCallerConfig(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	opts := NewClientOptions().
		WithClientID("will-client").
		WithWillSimple("will/topic", []byte("will payload"), packet.QoS0, false)
	opts.Will.Expiry = 2 * time.Second

	client := &Client{options: opts}

	connectCh := make(chan *packet.ConnectPacket, 1)
	errCh := make(chan error, 1)
	go func() {
		pkt, err := packet.ReadPacket(serverConn, 0)
		if err != nil {
			errCh <- err
			return
		}

		connect, ok := pkt.(*packet.ConnectPacket)
		if !ok {
			errCh <- err
			return
		}

		connectCh <- connect
		errCh <- packet.WritePacket(serverConn, packet.NewConnackPacket(packet.ReasonSuccess), 0)
	}()

	require.NotPanics(t, func() {
		_, err := client.handshake(clientConn)
		require.NoError(t, err)
	})

	connect := <-connectCh
	require.True(t, connect.Will)
	require.NotNil(t, connect.WillPacket)
	require.NotNil(t, connect.WillPacket.Properties)
	require.Greater(t, connect.WillPacket.Properties.ExpiryTime, time.Now().Unix())

	if opts.Will.Packet.Properties != nil {
		require.Zero(t, opts.Will.Packet.Properties.ExpiryTime, "handshake should not mutate caller-owned will packet properties")
	}

	require.NoError(t, <-errCh)
}

func TestHandshake_WithWillSimple_ClonesPayloadInput(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	payload := []byte("will payload")
	opts := NewClientOptions().
		WithClientID("will-client").
		WithWillSimple("will/topic", payload, packet.QoS0, false)

	copy(payload, []byte("mutated payl"))

	client := &Client{options: opts}

	connectCh := make(chan *packet.ConnectPacket, 1)
	errCh := make(chan error, 1)
	go func() {
		pkt, err := packet.ReadPacket(serverConn, 0)
		if err != nil {
			errCh <- err
			return
		}

		connect, ok := pkt.(*packet.ConnectPacket)
		if !ok {
			errCh <- err
			return
		}

		connectCh <- connect
		errCh <- packet.WritePacket(serverConn, packet.NewConnackPacket(packet.ReasonSuccess), 0)
	}()

	_, err := client.handshake(clientConn)
	require.NoError(t, err)

	connect := <-connectCh
	require.True(t, connect.Will)
	require.NotNil(t, connect.WillPacket)
	require.Equal(t, []byte("will payload"), connect.WillPacket.Payload, "WithWillSimple should not retain caller-owned payload slice")
	require.NoError(t, <-errCh)
}

func TestHandshake_ClonesConnectMutableInputs(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	authData := []byte("secret-token")
	userProperties := map[string]string{"x-origin": "client"}
	opts := NewClientOptions().
		WithClientID("connect-client").
		WithAuth("token", authData).
		WithUserProperties(userProperties)

	copy(authData, []byte("mutate-token"))
	userProperties["x-origin"] = "mutated"

	client := &Client{options: opts}

	connectCh := make(chan *packet.ConnectPacket, 1)
	errCh := make(chan error, 1)
	go func() {
		pkt, err := packet.ReadPacket(serverConn, 0)
		if err != nil {
			errCh <- err
			return
		}

		connect, ok := pkt.(*packet.ConnectPacket)
		if !ok {
			errCh <- err
			return
		}

		connectCh <- connect
		errCh <- packet.WritePacket(serverConn, packet.NewConnackPacket(packet.ReasonSuccess), 0)
	}()

	_, err := client.handshake(clientConn)
	require.NoError(t, err)

	connect := <-connectCh
	require.NotNil(t, connect.Properties)
	require.Equal(t, []byte("secret-token"), connect.Properties.AuthData, "handshake should not retain caller-owned auth data slice")
	require.Equal(t, "client", connect.Properties.UserProperties["x-origin"], "handshake should not retain caller-owned user properties map")
	require.NoError(t, <-errCh)
}
