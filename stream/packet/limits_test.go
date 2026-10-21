package packet

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func rawPacket(t *testing.T, typ PacketType, body []byte) []byte {
	t.Helper()

	raw := make([]byte, 5+len(body))
	raw[0] = byte(typ)
	binary.BigEndian.PutUint32(raw[1:5], uint32(len(body)))
	copy(raw[5:], body)
	return raw
}

// TestReadPacket_RejectsOversizedMetadata 验证声明长度超过实际剩余字节的
// Metadata 不会触发大分配。
func TestReadPacket_RejectsOversizedMetadata(t *testing.T) {
	var body bytes.Buffer
	_ = EncodeUint32(&body, 1)             // StreamID
	_ = EncodeString(&body, "t")           // Topic
	_ = EncodeString(&body, "src")         // SourceClientID
	_ = EncodeString(&body, "dst")         // TargetClientID
	_ = EncodeUint32(&body, MaxPacketSize) // Metadata 声明 16MiB，但后面没有数据

	_, err := ReadPacket(bytes.NewReader(rawPacket(t, STREAM_OPEN, body.Bytes())))
	if err == nil {
		t.Fatal("expected error for oversized metadata")
	}
}

// TestReadPacket_RejectsOversizedString 验证声明长度超过剩余字节的字符串被拒绝。
func TestReadPacket_RejectsOversizedString(t *testing.T) {
	body := []byte{0x00, 0x00, 0x03, 0xe8} // Topic 长度 = 1000，但没有内容

	_, err := ReadPacket(bytes.NewReader(rawPacket(t, STREAM_OPEN, body)))
	if err == nil {
		t.Fatal("expected error for oversized string")
	}
}

// TestReadPacket_RejectsPingWithBody 验证 PING/PONG 包体必须为空。
func TestReadPacket_RejectsPingWithBody(t *testing.T) {
	for _, typ := range []PacketType{PING, PONG} {
		_, err := ReadPacket(bytes.NewReader(rawPacket(t, typ, []byte{0x00})))
		if err == nil {
			t.Fatalf("%v with body must be rejected", typ)
		}
	}
}
