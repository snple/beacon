package packet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/danclive/nson-go"
)

// rawPublish 构造一个 PUBLISH 包体：flags + topic + packetID + 属性长度字段。
func rawPublish(propLen uint32) []byte {
	var body bytes.Buffer
	body.WriteByte(0)             // flags: QoS0, 无 retain/dup
	body.Write([]byte{0, 1, 't'}) // topic "t"
	body.Write(make([]byte, 12))  // packetID
	_ = binary.Write(&body, binary.BigEndian, propLen)
	return body.Bytes()
}

func readRawPacket(t *testing.T, pktType PacketType, body []byte) (Packet, error) {
	t.Helper()

	var raw bytes.Buffer
	raw.WriteByte(byte(pktType))
	var remaining [4]byte
	binary.BigEndian.PutUint32(remaining[:], uint32(len(body)))
	raw.Write(remaining[:])
	raw.Write(body)

	return ReadPacket(&raw, 1<<20)
}

// TestReadPacket_PropertyLengthDoesNotAllocateBeyondInput 验证属性长度字段
// 不能超过输入中实际剩余的字节，否则小报文可以触发数百 MB 分配。
func TestReadPacket_PropertyLengthDoesNotAllocateBeyondInput(t *testing.T) {
	tests := []struct {
		name    string
		propLen uint32
	}{
		// 小于包级硬上限，但远超实际剩余字节（0）
		{"larger than remaining", 1000},
		// 超过包级硬上限
		{"larger than hard limit", 0x7FFFFFFF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := readRawPacket(t, PUBLISH, rawPublish(tt.propLen))
			if !errors.Is(err, ErrMalformedPacket) {
				t.Fatalf("expected ErrMalformedPacket, got %v", err)
			}
		})
	}
}

// TestReadPacket_TraceDetailsCountDoesNotOverAllocate 验证 TRACE 的 details 数量
// 先按剩余字节校验，避免畸形包触发大 map 预分配。
func TestReadPacket_TraceDetailsCountDoesNotOverAllocate(t *testing.T) {
	var body bytes.Buffer
	body.Write(make([]byte, 8))                               // 4 个空字符串
	body.Write(make([]byte, 8))                               // timestamp
	_ = binary.Write(&body, binary.BigEndian, uint16(0xFFFF)) // detailsCount

	_, err := readRawPacket(t, TRACE, body.Bytes())
	if !errors.Is(err, ErrMalformedPacket) {
		t.Fatalf("expected ErrMalformedPacket, got %v", err)
	}
}

func TestWritePacket_RoundTripAfterEncodeRefactor(t *testing.T) {
	id := testPacketID()
	pkt := NewPublishPacket("a/b", []byte("payload"))
	pkt.QoS = QoS1
	pkt.PacketID = id
	pkt.Properties.ExpiryTime = 123
	pkt.Properties.UserProperties = map[string]string{"k": "v"}

	var buf bytes.Buffer
	if err := WritePacket(&buf, pkt, 1<<20); err != nil {
		t.Fatalf("WritePacket: %v", err)
	}

	got, err := ReadPacket(&buf, 1<<20)
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}
	pub, ok := got.(*PublishPacket)
	if !ok {
		t.Fatalf("got %T, want *PublishPacket", got)
	}
	if pub.Topic != pkt.Topic || string(pub.Payload) != "payload" || pub.QoS != QoS1 {
		t.Fatalf("round trip mismatch: %+v", pub)
	}
}

// TestWritePacket_ErrorsAreDistinguished 验证编码错误与写入错误可区分：
// 编码错误不应被当成连接级故障。
func TestWritePacket_ErrorsAreDistinguished(t *testing.T) {
	oversized := NewPublishPacket(strings.Repeat("a", MaxStringLength+1), nil)
	err := WritePacket(io.Discard, oversized, 0)
	if err == nil {
		t.Fatal("expected encode error")
	}
	var writeErr *WriteError
	if errors.As(err, &writeErr) {
		t.Fatalf("encode error must not be a WriteError: %v", err)
	}

	failing := &failingWriter{}
	err = WritePacket(failing, NewPingPacket(1, 1), 0)
	if !errors.As(err, &writeErr) {
		t.Fatalf("expected WriteError, got %v", err)
	}
}

type failingWriter struct{}

func (f *failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

// testPacketID 返回一个非零 PacketID。
func testPacketID() nson.Id {
	var id nson.Id
	id[11] = 1
	return id
}
