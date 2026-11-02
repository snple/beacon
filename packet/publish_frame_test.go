package packet

import (
	"bytes"
	"testing"
)

// TestPublishFrame_MatchesEncodePacket 验证分段写出的帧与 EncodePacket 逐字节一致。
func TestPublishFrame_MatchesEncodePacket(t *testing.T) {
	props := NewPublishProperties()
	props.ContentType = "text/plain"
	props.TraceID = "trace"
	props.SourceClientID = "src"
	props.UserProperties = map[string]string{"k": "v"}

	payload := []byte("payload-bytes")
	body, err := NewPublishBody("a/b", props, payload)
	if err != nil {
		t.Fatalf("NewPublishBody: %v", err)
	}

	id := NewPacketID()
	frame := body.Frame(id, QoS1, true, true)

	want, err := EncodePacket(&PublishPacket{
		Dup:        true,
		QoS:        QoS1,
		Retain:     true,
		Topic:      "a/b",
		PacketID:   id,
		Properties: props,
		Payload:    payload,
	}, 0)
	if err != nil {
		t.Fatalf("EncodePacket: %v", err)
	}

	var got bytes.Buffer
	if _, err := frame.WriteTo(&got); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	if got.Len() != len(want) {
		t.Fatalf("frame size = %d, want %d", got.Len(), len(want))
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("segmented frame differs from EncodePacket:\n got=%x\nwant=%x", got.Bytes(), want)
	}
	if int(frame.Size()) != len(want) {
		t.Fatalf("Size() = %d, want %d", frame.Size(), len(want))
	}
}

// TestPublishFrame_DoesNotCopyPayload 验证分段直接引用原载荷（零拷贝）。
func TestPublishFrame_DoesNotCopyPayload(t *testing.T) {
	payload := []byte("payload")
	body, err := NewPublishBody("topic", nil, payload)
	if err != nil {
		t.Fatalf("NewPublishBody: %v", err)
	}

	segs := body.Frame(NewPacketID(), QoS0, false, false).Segments()
	last := segs[len(segs)-1]

	if len(last) == 0 || &last[0] != &payload[0] {
		t.Fatal("publish frame should reference the original payload, not a copy")
	}
}

// TestPublishBody_EmptyPayloadAndNilProperties 验证空载荷与 nil 属性也能正确编码。
func TestPublishBody_EmptyPayloadAndNilProperties(t *testing.T) {
	body, err := NewPublishBody("t", nil, nil)
	if err != nil {
		t.Fatalf("NewPublishBody: %v", err)
	}

	id := NewPacketID()

	got := bytes.Buffer{}
	if _, err := body.Frame(id, QoS0, false, false).WriteTo(&got); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	want, err := EncodePacket(&PublishPacket{
		Topic:      "t",
		PacketID:   id,
		Properties: NewPublishProperties(),
	}, 0)
	if err != nil {
		t.Fatalf("EncodePacket: %v", err)
	}

	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("frame = %x, want %x", got.Bytes(), want)
	}
}

var benchSegments [][]byte

// BenchmarkPublishFrame_Segments 度量"按接收者构造帧描述"的成本：
// 内容层预编码一次后，每个接收者只需要 18 字节的头 + 一个分段切片。
func BenchmarkPublishFrame_Segments(b *testing.B) {
	payload := make([]byte, 4096)
	body, err := NewPublishBody("sensors/room1/temperature", NewPublishProperties(), payload)
	if err != nil {
		b.Fatal(err)
	}
	id := NewPacketID()

	b.ReportAllocs()
	for b.Loop() {
		frame := body.Frame(id, QoS1, false, false)
		benchSegments = frame.Segments()
	}
}

// BenchmarkEncodePacketPerDelivery 是对照组：为每个接收者完整编码一次帧
// （旧实现的做法，每次都要分配并复制整个 payload）。
func BenchmarkEncodePacketPerDelivery(b *testing.B) {
	payload := make([]byte, 4096)
	pub := &PublishPacket{
		Topic:      "sensors/room1/temperature",
		Payload:    payload,
		QoS:        QoS1,
		PacketID:   NewPacketID(),
		Properties: NewPublishProperties(),
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := EncodePacket(pub, 0); err != nil {
			b.Fatal(err)
		}
	}
}
