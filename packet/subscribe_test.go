package packet

import (
	"bytes"
	"testing"

	"github.com/danclive/nson-go"
)

func TestSubscribeOptionsEncoding(t *testing.T) {
	tests := []struct {
		name              string
		qos               QoS
		noLocal           bool
		retainAsPublished bool
		retainHandling    uint8
	}{
		{"Basic QoS0", QoS0, false, false, 0},
		{"Basic QoS1", QoS1, false, false, 0},
		{"NoLocal", QoS1, true, false, 0},
		{"RetainAsPublished", QoS1, false, true, 0},
		{"RetainHandling 1", QoS1, false, false, 1},
		{"RetainHandling 2", QoS1, false, false, 2},
		{"All options", QoS1, true, true, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := NewSubscribePacket(nson.NewId())
			sub.Subscriptions = append(sub.Subscriptions, Subscription{
				Topic: "test/topic",
				Options: SubscribeOptions{
					QoS:               tt.qos,
					NoLocal:           tt.noLocal,
					RetainAsPublished: tt.retainAsPublished,
					RetainHandling:    tt.retainHandling,
				},
			})

			var buf bytes.Buffer
			if err := WritePacket(&buf, sub, 0); err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			r := bytes.NewReader(buf.Bytes())
			var header FixedHeader
			if err := header.Decode(r); err != nil {
				t.Fatalf("Decode header failed: %v", err)
			}

			decoded := &SubscribePacket{}
			if err := decoded.decode(r, header); err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			if len(decoded.Subscriptions) != 1 {
				t.Fatalf("Wrong number of subscriptions: got %d, want 1", len(decoded.Subscriptions))
			}

			opts := decoded.Subscriptions[0].Options
			if opts.QoS != tt.qos {
				t.Errorf("QoS mismatch: got %v, want %v", opts.QoS, tt.qos)
			}
			if opts.NoLocal != tt.noLocal {
				t.Errorf("NoLocal mismatch: got %v, want %v", opts.NoLocal, tt.noLocal)
			}
			if opts.RetainAsPublished != tt.retainAsPublished {
				t.Errorf("RetainAsPublished mismatch: got %v, want %v", opts.RetainAsPublished, tt.retainAsPublished)
			}
			if opts.RetainHandling != tt.retainHandling {
				t.Errorf("RetainHandling mismatch: got %v, want %v", opts.RetainHandling, tt.retainHandling)
			}
		})
	}
}
