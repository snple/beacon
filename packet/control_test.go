package packet

import (
	"bytes"
	"errors"
	"testing"
)

// TestPingPong_RoundTrip 验证 PING/PONG 编解码往返
func TestPingPong_RoundTrip(t *testing.T) {
	// PING
	ping := NewPingPacket(42, 1234567890)
	var buf bytes.Buffer
	if err := WritePacket(&buf, ping, 0); err != nil {
		t.Fatalf("encode ping: %v", err)
	}
	pkt, err := ReadPacket(bytes.NewReader(buf.Bytes()), 0)
	if err != nil {
		t.Fatalf("decode ping: %v", err)
	}
	gotPing, ok := pkt.(*PingPacket)
	if !ok {
		t.Fatalf("expected PingPacket, got %T", pkt)
	}
	if gotPing.Seq != 42 || gotPing.Timestamp != 1234567890 {
		t.Fatalf("ping roundtrip mismatch: seq=%d ts=%d", gotPing.Seq, gotPing.Timestamp)
	}

	// PONG
	pong := NewPongPacket(42, 1234567890, 50)
	buf.Reset()
	if err := WritePacket(&buf, pong, 0); err != nil {
		t.Fatalf("encode pong: %v", err)
	}
	pkt, err = ReadPacket(bytes.NewReader(buf.Bytes()), 0)
	if err != nil {
		t.Fatalf("decode pong: %v", err)
	}
	gotPong, ok := pkt.(*PongPacket)
	if !ok {
		t.Fatalf("expected PongPacket, got %T", pkt)
	}
	if gotPong.Seq != 42 || gotPong.Echo != 1234567890 || gotPong.Load != 50 {
		t.Fatalf("pong roundtrip mismatch: seq=%d echo=%d load=%d", gotPong.Seq, gotPong.Echo, gotPong.Load)
	}
}

// TestPingPong_RejectsWrongRemainingLength 验证 PING/PONG 的 Remaining 长度校验错误路径
func TestPingPong_RejectsWrongRemainingLength(t *testing.T) {
	cases := []struct {
		name      string
		pktType   byte
		remaining uint32
	}{
		{"ping too short", byte(PING), 11},
		{"ping too long", byte(PING), 13},
		{"pong too short", byte(PONG), 12},
		{"pong too long", byte(PONG), 14},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 构造 5 字节固定头 + remaining 字节的包体（包体内容无关紧要，
			// 因为 decode 会先校验 Remaining 长度）
			data := make([]byte, 5+int(tc.remaining))
			data[0] = tc.pktType
			data[1] = byte(tc.remaining >> 24)
			data[2] = byte(tc.remaining >> 16)
			data[3] = byte(tc.remaining >> 8)
			data[4] = byte(tc.remaining)

			_, err := ReadPacket(bytes.NewReader(data), 0)
			if err == nil {
				t.Fatal("expected error for wrong Remaining length")
			}
			if !errors.Is(err, ErrMalformedPacket) {
				t.Fatalf("expected ErrMalformedPacket, got %v", err)
			}
		})
	}
}

// TestDisconnectEmptyBodyEncode 验证「正常断开且无属性」编码为空包体。
//
// 回归：旧实现把整个固定头写进了包体，实际产出 10 字节
// （外层头 Remaining=5 + 内层头），对端会读到畸形数据；
// 同时解码→重编码后字节发生变化。
func TestDisconnectEmptyBodyEncode(t *testing.T) {
	disc := NewDisconnectPacket(ReasonNormalDisconnect)
	disc.Properties = nil

	var buf bytes.Buffer
	if err := WritePacket(&buf, disc, 0); err != nil {
		t.Fatalf("编码失败: %v", err)
	}
	if buf.Len() != 5 {
		t.Fatalf("期望 5 字节（仅固定头），实际 %d: %x", buf.Len(), buf.Bytes())
	}

	pkt, err := ReadPacket(bytes.NewReader(buf.Bytes()), 0)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	decoded, ok := pkt.(*DisconnectPacket)
	if !ok {
		t.Fatalf("类型不符: %T", pkt)
	}
	if decoded.ReasonCode != ReasonNormalDisconnect {
		t.Fatalf("ReasonCode = %d, 期望 %d", decoded.ReasonCode, ReasonNormalDisconnect)
	}

	// 解码后重新编码必须完全一致
	var again bytes.Buffer
	if err := WritePacket(&again, decoded, 0); err != nil {
		t.Fatalf("二次编码失败: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), again.Bytes()) {
		t.Fatalf("编码不稳定:\n%x\n%x", buf.Bytes(), again.Bytes())
	}
}
