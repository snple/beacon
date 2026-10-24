package packet

import (
	"bytes"
	"errors"
	"testing"

	"github.com/danclive/nson-go"
)

// minimalPackets 返回各包类型可编码的最小实例。
func minimalPackets() map[PacketType]Packet {
	sub := NewSubscribePacket(nson.NewId())
	sub.AddSubscription("a", QoS0)

	suback := NewSubackPacket(nson.NewId())
	suback.ReasonCodes = append(suback.ReasonCodes, ReasonGrantedQoS0)

	unsub := NewUnsubscribePacket(nson.NewId())
	unsub.Topics = append(unsub.Topics, "a")

	unsuback := NewUnsubackPacket(nson.NewId())
	unsuback.ReasonCodes = append(unsuback.ReasonCodes, ReasonSuccess)

	pub := NewPublishPacket("a", nil)
	pub.PacketID = nson.NewId()

	return map[PacketType]Packet{
		CONNECT:     NewConnectPacket(),
		CONNACK:     NewConnackPacket(ReasonSuccess),
		PUBLISH:     pub,
		PUBACK:      NewPubackPacket(nson.NewId(), ReasonSuccess),
		SUBSCRIBE:   sub,
		SUBACK:      suback,
		UNSUBSCRIBE: unsub,
		UNSUBACK:    unsuback,
		PING:        NewPingPacket(1, 2),
		PONG:        NewPongPacket(1, 2, 3),
	}
}

// TestMinRemainingLengthIsLowerBound 保证早拒绝阈值不会误伤本实现编码出的合法包。
func TestMinRemainingLengthIsLowerBound(t *testing.T) {
	for typ, pkt := range minimalPackets() {
		var buf bytes.Buffer
		if err := WritePacket(&buf, pkt, 0); err != nil {
			t.Fatalf("%v: encode: %v", typ, err)
		}

		remaining := uint32(buf.Len() - 5)
		minLen, ok := minRemainingLength[typ]
		if !ok {
			t.Fatalf("%v: missing minRemainingLength entry", typ)
		}
		if remaining < minLen {
			t.Errorf("%v: encoded remaining=%d < minRemainingLength=%d", typ, remaining, minLen)
		}
	}
}

// TestReadPacketRejectsTruncatedBody 验证低于最小长度的包在分配缓冲前即被拒绝。
func TestReadPacketRejectsTruncatedBody(t *testing.T) {
	for typ, minLen := range minRemainingLength {
		if minLen == 0 {
			continue
		}

		var buf bytes.Buffer
		h := FixedHeader{Type: typ, Remaining: minLen - 1}
		if err := h.Encode(&buf); err != nil {
			t.Fatalf("%v: encode header: %v", typ, err)
		}
		buf.Write(make([]byte, minLen-1))

		if _, err := ReadPacket(&buf, 0); !errors.Is(err, ErrMalformedPacket) {
			t.Errorf("%v: ReadPacket error = %v, want ErrMalformedPacket", typ, err)
		}
	}
}
