package packet

import (
	"bytes"
	"testing"

	"github.com/danclive/nson-go"
)

// TestSubscribePacket_AllowsInvalidFilterOnWire 验证非法过滤器不会导致整包解码失败，
// 以便服务端通过 SUBACK 的逐条原因码反馈。
func TestSubscribePacket_AllowsInvalidFilterOnWire(t *testing.T) {
	sub := NewSubscribePacket(nson.NewId())
	sub.Subscriptions = []Subscription{
		{Topic: "ok/topic", Options: SubscribeOptions{QoS: QoS0}},
		{Topic: "bad/#", Options: SubscribeOptions{QoS: QoS0}},
		{Topic: "a/**/b", Options: SubscribeOptions{QoS: QoS0}},
	}

	var buf bytes.Buffer
	if err := WritePacket(&buf, sub, 0); err != nil {
		t.Fatalf("WritePacket: %v", err)
	}

	pkt, err := ReadPacket(&buf, 0)
	if err != nil {
		t.Fatalf("ReadPacket must accept invalid filters for per-entry reporting: %v", err)
	}

	got, ok := pkt.(*SubscribePacket)
	if !ok {
		t.Fatalf("got %T, want *SubscribePacket", pkt)
	}
	if len(got.Subscriptions) != len(sub.Subscriptions) {
		t.Fatalf("got %d subscriptions, want %d", len(got.Subscriptions), len(sub.Subscriptions))
	}
	for i := range sub.Subscriptions {
		if got.Subscriptions[i].Topic != sub.Subscriptions[i].Topic {
			t.Fatalf("subscription %d: got topic %q, want %q", i, got.Subscriptions[i].Topic, sub.Subscriptions[i].Topic)
		}
	}
}
