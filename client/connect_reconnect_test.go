package client

import (
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

func startTestCore(t *testing.T, sessionPresent bool) (addr string, stop func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read CONNECT
		pktIn, err := packet.ReadPacket(conn)
		if err != nil {
			return
		}
		_, ok := pktIn.(*packet.ConnectPacket)
		if !ok {
			return
		}

		// Write CONNACK success
		ack := packet.NewConnackPacket(packet.ReasonSuccess)
		ack.SessionPresent = sessionPresent
		recvWindow := uint16(50)
		ack.Properties.ReceiveWindow = &recvWindow
		_ = packet.WritePacket(conn, ack)

		// Best-effort: keep reading until client disconnects.
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		for {
			_, err := packet.ReadPacket(conn)
			if err != nil {
				return
			}
		}
	}()

	return ln.Addr().String(), func() {
		_ = ln.Close()
		<-done
	}
}

func TestClient_Connect_DoesNotResetLocalState(t *testing.T) {
	addr, stop := startTestCore(t, false)
	defer stop()

	opts := NewClientOptions().
		WithCore(addr).
		WithClientID("c1").
		WithKeepAlive(1).
		WithConnectTimeout(2 * time.Second).
		WithLogger(zap.NewNop())

	c, err := NewWithOptions(opts)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer c.Close()

	c.subscribedTopicsMu.Lock()
	c.subscribedTopics["t/1"] = true
	c.subscribedTopicsMu.Unlock()

	c.actionsMu.Lock()
	c.registeredActions["a1"] = true
	c.actionsMu.Unlock()

	if err := c.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	_ = c.Disconnect()

	c.subscribedTopicsMu.RLock()
	_, okTopic := c.subscribedTopics["t/1"]
	c.subscribedTopicsMu.RUnlock()
	if !okTopic {
		t.Fatalf("expected subscribed topic preserved")
	}

	c.actionsMu.RLock()
	_, okAction := c.registeredActions["a1"]
	c.actionsMu.RUnlock()
	if !okAction {
		t.Fatalf("expected registered action preserved")
	}
}

func TestClient_Connect_CleanSessionClearsStore(t *testing.T) {
	addr, stop := startTestCore(t, false)
	defer stop()

	cfg := DefaultStoreConfig()
	cfg.DataDir = "" // in-memory
	cfg.Logger = zap.NewNop()

	opts := NewClientOptions().
		WithCore(addr).
		WithClientID("c1").
		WithKeepAlive(1).
		WithConnectTimeout(2 * time.Second).
		WithCleanSession(true).
		WithStore(&cfg).
		WithLogger(zap.NewNop())

	c, err := NewWithOptions(opts)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer c.Close()

	if c.store == nil {
		t.Fatalf("expected store enabled")
	}

	err = c.store.Save(&StoredMessage{PacketID: 10, Topic: "t", Payload: []byte("x"), QoS: packet.QoS1})
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	countBefore, err := c.store.Count()
	if err != nil {
		t.Fatalf("count before: %v", err)
	}
	if countBefore == 0 {
		t.Fatalf("expected persisted message before connect")
	}

	if err := c.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Disconnect()

	countAfter, err := c.store.Count()
	if err != nil {
		t.Fatalf("count after: %v", err)
	}
	if countAfter != 0 {
		t.Fatalf("expected store cleared for clean session, got %d", countAfter)
	}
}
