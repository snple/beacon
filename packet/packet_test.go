package packet

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func roundTrip(t *testing.T, pkt Packet) Packet {
	t.Helper()
	frame, err := EncodePacket(pkt, 0)
	if err != nil {
		t.Fatalf("encode %s: %v", pkt.Type(), err)
	}
	got, n, err := ReadPacketWithSize(bytes.NewReader(frame), 0)
	if err != nil {
		t.Fatalf("decode %s: %v", pkt.Type(), err)
	}
	if int(n) != len(frame) {
		t.Fatalf("size mismatch: got %d want %d", n, len(frame))
	}
	return got
}

func TestConnectRoundTrip(t *testing.T) {
	in := NewConnectPacket("dev-1")
	in.KeepAlive = 30
	in.SetCleanSession(true)
	in.MaxPacketSize = 4096
	in.HasWill = true
	in.WillChannel = "dev/1/offline"
	in.WillPayload = []byte("bye")
	in.WillHeaders = map[string]string{"k": "v"}
	in.AuthMethod = "plain"
	in.AuthData = []byte("secret")

	out := roundTrip(t, in).(*ConnectPacket)
	if out.ClientID != in.ClientID || out.KeepAlive != in.KeepAlive ||
		out.CleanSession() != in.CleanSession() || out.MaxPacketSize != in.MaxPacketSize {
		t.Fatalf("header mismatch: %+v", out)
	}
	if !out.HasWill || out.WillChannel != in.WillChannel ||
		string(out.WillPayload) != string(in.WillPayload) || out.WillHeaders["k"] != "v" {
		t.Fatalf("will mismatch: %+v", out)
	}
	if out.AuthMethod != "plain" || string(out.AuthData) != "secret" {
		t.Fatalf("auth mismatch")
	}
}

func TestConnackRoundTrip(t *testing.T) {
	in := NewConnackPacket(ReasonSuccess)
	in.SetSessionPresent(true)
	in.AssignedClientID = "c1"
	in.ServerMaxPacketSize = 1024
	in.KeepAlive = 25
	in.SessionID = 42
	in.Pending = [3]uint32{1, 2, 3}

	out := roundTrip(t, in).(*ConnackPacket)
	if !out.SessionPresent() || out.AssignedClientID != "c1" || out.SessionID != 42 || out.KeepAlive != 25 {
		t.Fatalf("mismatch: %+v", out)
	}
	if out.Pending != in.Pending {
		t.Fatalf("depth mismatch: %+v", out)
	}
}

func TestPushRoundTrip(t *testing.T) {
	rec := Record{Channel: "a/b", Payload: []byte("hello"), Headers: map[string]string{"x": "y"}}
	rec.SetPriority(PriorityHigh)

	in := NewPushPacket(7, rec)
	in.Flags |= PushFlagWantAck

	out := roundTrip(t, in).(*PushPacket)
	if out.RequestID != 7 || out.Flags&PushFlagWantAck == 0 {
		t.Fatalf("header mismatch")
	}
	got := out.Record
	if got.Channel != "a/b" || string(got.Payload) != "hello" || got.Priority() != PriorityHigh {
		t.Fatalf("record mismatch: %+v", got)
	}
	if got.Headers["x"] != "y" {
		t.Fatalf("headers mismatch")
	}
}

func TestPullRoundTrip(t *testing.T) {
	rec := Record{Channel: "a", Payload: []byte("p")}
	rec.SetPriority(PriorityLow)
	rec.SetDup(true)

	in := NewPullOkPacket(9)
	in.SetRecord(rec)
	in.Pending = [3]uint32{5, 0, 0}

	out := roundTrip(t, in).(*PullOkPacket)
	if out.RequestID != 9 || !out.HasRecord() || out.Pending != in.Pending {
		t.Fatalf("mismatch: %+v", out)
	}
	if !out.Record.Dup() || out.Record.Priority() != PriorityLow {
		t.Fatalf("record mismatch: %+v", out.Record)
	}
}

func TestPullOkEmptyRoundTrip(t *testing.T) {
	in := NewPullOkPacket(10)
	in.Pending = [3]uint32{1, 2, 3}

	out := roundTrip(t, in).(*PullOkPacket)
	if out.HasRecord() || out.Pending != in.Pending {
		t.Fatalf("mismatch: %+v", out)
	}
}

func TestCommitAndDeclareRoundTrip(t *testing.T) {
	id := NewPacketID()
	c := roundTrip(t, NewCommitPacket(3, id)).(*CommitPacket)
	if c.RequestID != 3 || c.ID != id {
		t.Fatalf("commit mismatch")
	}

	d := NewDeclarePacket(4, []ChannelDecl{{Channel: "x/**"}, {Channel: "y"}})
	dd := roundTrip(t, d).(*DeclarePacket)
	if len(dd.Channels) != 2 || dd.Channels[0].Channel != "x/**" {
		t.Fatalf("declare mismatch")
	}

	resp := NewDeclareRespPacket(5)
	resp.Results = []ChannelResult{{Channel: "x/**", Reason: ReasonSuccess}}
	resp.Limits = Limits{MaxPacketSize: 4096, MaxWaitMs: 1000, MaxChannelsPerClient: 8}
	rr := roundTrip(t, resp).(*DeclareRespPacket)
	if len(rr.Results) != 1 || rr.Limits.MaxWaitMs != 1000 || rr.Limits.MaxChannelsPerClient != 8 {
		t.Fatalf("declare resp mismatch: %+v", rr)
	}
}

func TestControlRoundTrip(t *testing.T) {
	p := roundTrip(t, NewPingPacket(11, 12345)).(*PingPacket)
	if p.Seq != 11 || p.Timestamp != 12345 {
		t.Fatalf("ping mismatch")
	}

	d := NewDisconnectPacket(ReasonSessionTakenOver)
	d.Flags |= DisconnectFlagSuppressWill
	dd := roundTrip(t, d).(*DisconnectPacket)
	if dd.ReasonCode != ReasonSessionTakenOver || dd.Flags&DisconnectFlagSuppressWill == 0 {
		t.Fatalf("disconnect mismatch")
	}
}

func TestPriorityDefault(t *testing.T) {
	var r Record
	if r.Priority() != PriorityMedium {
		t.Fatalf("default priority should be medium, got %v", r.Priority())
	}
	if r.HasPriority() {
		t.Fatal("HasPriority should be false by default")
	}

	r.SetPriority(PriorityLow)
	if r.Priority() != PriorityLow || !r.HasPriority() {
		t.Fatal("explicit low priority not preserved")
	}
}

func TestMalformedRejected(t *testing.T) {
	frame, err := EncodePacket(NewPullPacket(1, 0), 0)
	if err != nil {
		t.Fatal(err)
	}
	// 截断包体
	if _, _, err := ReadPacketWithSize(bytes.NewReader(frame[:len(frame)-3]), 0); err == nil {
		t.Fatal("expected error on truncated packet")
	}
	// 超长声明
	frame[1], frame[2], frame[3], frame[4] = 0xFF, 0xFF, 0xFF, 0xFF
	if _, _, err := ReadPacketWithSize(bytes.NewReader(frame), 1<<20); err == nil {
		t.Fatal("expected error on oversized packet")
	}
}

func FuzzReadPacket(f *testing.F) {
	f.Add([]byte{})
	frame, _ := EncodePacket(NewPushPacket(1, Record{Channel: "a", Payload: []byte("x")}), 0)
	f.Add(frame)

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _, _ = ReadPacketWithSize(bytes.NewReader(data), 1<<20)
	})
}

// 回归：非法优先级位（3）必须在解码阶段被拒绝，不能流入 mailbox 的数组下标。
func TestInvalidPriorityRejected(t *testing.T) {
	frame, err := EncodePacket(NewPushPacket(1, Record{Channel: "a", Payload: []byte("x")}), 0)
	if err != nil {
		t.Fatal(err)
	}
	const recordFlagsOffset = 5 + 4 + 1
	frame[recordFlagsOffset] = recordFlagHasPriority | 0x03<<1

	if _, _, err := ReadPacketWithSize(bytes.NewReader(frame), 1<<20); err == nil {
		t.Fatal("expected error for invalid priority bits")
	}
}

// 回归：声明超长字符串的小包不能触发大分配。
func TestDeclaredStringLengthRejected(t *testing.T) {
	body := []byte{}
	body = append(body, 0, 0, 0, 1)                    // RequestID
	body = append(body, 0)                             // Flags
	body = append(body, 0)                             // Record flags
	body = append(body, make([]byte, PacketIDSize)...) // ID
	body = append(body, 0xFF, 0xFF)                    // Channel length = 65535
	body = append(body, make([]byte, 10)...)           // padding: pass the minimum length check

	frame := make([]byte, 5+len(body))
	frame[0] = byte(PUSH)
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(body)))
	copy(frame[5:], body)

	if _, _, err := ReadPacketWithSize(bytes.NewReader(frame), 1<<20); err == nil {
		t.Fatal("expected error for impossible string length")
	}
}

// 回归：声明超大 payload 长度的小包不能触发巨型内存分配。
func TestHugePayloadLengthRejected(t *testing.T) {
	// 手工构造 PUSH：RequestID(4) + Flags(1) + 一条只有头部、payload 长度声明为 0xFFFFFFFF
	body := []byte{}
	body = append(body, 0, 0, 0, 1)                    // RequestID
	body = append(body, 0)                             // Flags
	body = append(body, 0)                             // Record flags
	body = append(body, make([]byte, PacketIDSize)...) // ID
	body = append(body, 0, 1, 'a')                     // Channel
	body = append(body, 0, 0)                          // Target
	body = append(body, 0, 0)                          // Source
	body = append(body, 0, 0)                          // Headers
	body = append(body, 0xFF, 0xFF, 0xFF, 0xFF)        // Payload length

	frame := make([]byte, 5+len(body))
	frame[0] = byte(PUSH)
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(body)))
	copy(frame[5:], body)

	if _, _, err := ReadPacketWithSize(bytes.NewReader(frame), 1<<20); err == nil {
		t.Fatal("expected error for oversized declared payload length")
	}
}
