package core

import (
	"testing"

	"github.com/snple/beacon/packet"
)

func TestRetainStoreMatchForSubscription(t *testing.T) {
	store := NewRetainStore()
	pub := packet.NewPublishPacket("test/topic", []byte("retained message"))
	pub.Retain = true
	pub.QoS = packet.QoS1
	msg := &Message{Packet: pub, Retain: true, Timestamp: 0}
	store.set("test/topic", msg)

	tests := []struct {
		name              string
		retainAsPublished bool
		isNewSubscription bool
		retainHandling    uint8
		expectMsgCount    int
		expectRetainFlag  bool
	}{
		{"RH 0, always send", true, false, 0, 1, true},
		{"RH 0, clear retain", false, false, 0, 1, false},
		{"RH 1, new sub", true, true, 1, 1, true},
		{"RH 1, existing sub", true, false, 1, 0, false},
		{"RH 2, never send", true, true, 2, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgs := store.matchForSubscription("test/topic", tt.isNewSubscription, tt.retainAsPublished, tt.retainHandling)
			if len(msgs) != tt.expectMsgCount {
				t.Errorf("Count mismatch: got %d, want %d", len(msgs), tt.expectMsgCount)
			}
			if tt.expectMsgCount > 0 && msgs[0].Retain != tt.expectRetainFlag {
				t.Errorf("Retain flag mismatch: got %v, want %v", msgs[0].Retain, tt.expectRetainFlag)
			}
		})
	}
}
