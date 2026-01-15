package core

import (
	"testing"

	"github.com/snple/beacon/packet"
)

func TestRetainStoreMatchForSubscription(t *testing.T) {
	store := NewRetainStore()
	msg := &Message{
		Topic:   "test/topic",
		Payload: []byte("retained message"),
		Retain:  true,
		QoS:     packet.QoS1,
	}
	store.Set("test/topic", msg)

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
msgs := store.MatchForSubscription("test/topic", tt.retainAsPublished, tt.isNewSubscription, tt.retainHandling)
if len(msgs) != tt.expectMsgCount {
t.Errorf("Count mismatch: got %d, want %d", len(msgs), tt.expectMsgCount)
}
if tt.expectMsgCount > 0 && msgs[0].Retain != tt.expectRetainFlag {
				t.Errorf("Retain flag mismatch: got %v, want %v", msgs[0].Retain, tt.expectRetainFlag)
			}
		})
	}
}
