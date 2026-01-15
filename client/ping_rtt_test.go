package client

import (
	"testing"
	"time"

	"github.com/snple/beacon/packet"
)

func TestClient_PongSeqUpdatesRTT(t *testing.T) {
	c := &Client{}

	seq := uint32(123)
	now := time.Now().UnixNano()
	c.lastPingSeq.Store(seq)
	c.lastPingSent.Store(now - int64(2*time.Millisecond))

	c.handlePacket(&packet.PongPacket{Seq: seq})

	rtt := c.LastRTT()
	if rtt <= 0 {
		t.Fatalf("expected rtt > 0, got %v", rtt)
	}
	if rtt > time.Second {
		t.Fatalf("expected rtt <= 1s, got %v", rtt)
	}
}
