package core

import (
	"bytes"
	"net"
	"testing"
	"time"

	streampacket "github.com/snple/beacon/stream/packet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestPending(c *Core) (*streamPending, func()) {
	srcPipe, srcPeer := net.Pipe()
	tgtPipe, tgtPeer := net.Pipe()

	src := newConn(c, srcPipe)
	tgt := newConn(c, tgtPipe)
	srcClient := newClient(c, "a")
	tgtClient := newClient(c, "b")
	srcClient.swapConn(src)
	tgtClient.swapConn(tgt)
	src.client = srcClient
	tgt.client = tgtClient

	p := &streamPending{
		id:          1,
		sourceID:    "a",
		targetID:    "b",
		source:      src,
		target:      tgt,
		ticket:      bytes.Repeat([]byte{1}, streamTicketSize),
		challenge:   bytes.Repeat([]byte{2}, streamMinChallengeSize),
		accepted:    true,
		deadline:    time.Now().Add(time.Minute),
		bindTimeout: time.Second,
		acceptCh:    make(chan struct{}),
	}

	c.streamMu.Lock()
	c.streamPending[p.id] = p
	c.streamMu.Unlock()

	cleanup := func() {
		_ = srcPipe.Close()
		_ = tgtPipe.Close()
		_ = srcPeer.Close()
		_ = tgtPeer.Close()
	}
	return p, cleanup
}

func TestBindStreamDataRejectsWrongTicket(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions().
		WithStreamEnabled(true).
		WithStreamAllowAnySourceIP(true).
		WithLogger(zap.NewNop()))
	require.NoError(t, err)

	p, cleanup := newTestPending(c)
	defer cleanup()

	data, peer := net.Pipe()
	defer data.Close()
	defer peer.Close()

	_, _, reason, _ := c.bindStreamData(&streampacket.BindPacket{
		Version:   streampacket.Version,
		StreamID:  p.id,
		Role:      streampacket.RoleInitiator,
		Ticket:    []byte("wrong"),
		Challenge: p.challenge,
	}, &streamDataConn{Conn: data})
	require.Equal(t, streampacket.NotAuthorized, reason)
}

func TestBindStreamDataRejectsExpired(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions().
		WithStreamEnabled(true).
		WithStreamAllowAnySourceIP(true).
		WithLogger(zap.NewNop()))
	require.NoError(t, err)

	p, cleanup := newTestPending(c)
	defer cleanup()
	p.deadline = time.Now().Add(-time.Second)

	data, peer := net.Pipe()
	defer data.Close()
	defer peer.Close()

	_, _, reason, _ := c.bindStreamData(&streampacket.BindPacket{
		Version:   streampacket.Version,
		StreamID:  p.id,
		Role:      streampacket.RoleInitiator,
		Ticket:    p.ticket,
		Challenge: p.challenge,
	}, &streamDataConn{Conn: data})
	require.Equal(t, streampacket.StreamNotFound, reason)
}

func TestBindStreamDataRejectsInitiatorBeforeAccept(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions().
		WithStreamEnabled(true).
		WithStreamAllowAnySourceIP(true).
		WithLogger(zap.NewNop()))
	require.NoError(t, err)

	p, cleanup := newTestPending(c)
	defer cleanup()
	p.accepted = false
	p.challenge = nil

	data, peer := net.Pipe()
	defer data.Close()
	defer peer.Close()

	_, _, reason, _ := c.bindStreamData(&streampacket.BindPacket{
		Version:  streampacket.Version,
		StreamID: p.id,
		Role:     streampacket.RoleInitiator,
		Ticket:   p.ticket,
	}, &streamDataConn{Conn: data})
	require.Equal(t, streampacket.NotAuthorized, reason)
}

func TestStreamBindSourceAllowed(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions().WithStreamEnabled(true))
	require.NoError(t, err)

	p := &streamPending{sourceIP: "127.0.0.1", targetIP: "::1"}
	require.True(t, c.streamBindSourceAllowed(p, streampacket.RoleInitiator, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 1}))
	require.False(t, c.streamBindSourceAllowed(p, streampacket.RoleInitiator, &net.TCPAddr{IP: net.ParseIP("127.0.0.2"), Port: 1}))
	require.True(t, c.streamBindSourceAllowed(p, streampacket.RoleAcceptor, &net.TCPAddr{IP: net.ParseIP("0:0:0:0:0:0:0:1"), Port: 1}))
}
