package client

import (
	"bufio"
	"context"
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// mockConn 是一个模拟的 net.Conn
type mockConn struct {
	net.Conn
}

func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (m *mockConn) Write(b []byte) (n int, err error)  { return len(b), nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }

func TestConnection_PongSeqUpdatesRTT(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := &Client{
		ctx:    context.Background(),
		logger: logger,
	}

	conn := &Conn{
		client: client,
		conn:   &mockConn{},
		reader: bufio.NewReader(&mockConn{}),
		writer: bufio.NewWriter(&mockConn{}),
		logger: logger,
	}

	seq := uint32(123)
	now := time.Now().UnixNano()
	conn.lastPingSeq.Store(seq)
	conn.lastPingSent.Store(now - int64(2*time.Millisecond))

	conn.handlePong(&packet.PongPacket{Seq: seq})

	rtt := conn.LastRTT()
	if rtt <= 0 {
		t.Fatalf("expected rtt > 0, got %v", rtt)
	}
	if rtt > time.Second {
		t.Fatalf("expected rtt <= 1s, got %v", rtt)
	}
}
