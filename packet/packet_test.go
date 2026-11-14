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
	in.AuthMethod = "plain"
	in.AuthData = []byte("secret")
	in.Properties = map[string]string{"model": "x1", "fw": "1.2"}

	out := roundTrip(t, in).(*ConnectPacket)
	if out.ClientID != in.ClientID || out.KeepAlive != in.KeepAlive ||
		out.CleanSession() != in.CleanSession() || out.MaxPacketSize != in.MaxPacketSize {
		t.Fatalf("header mismatch: %+v", out)
	}
	if out.AuthMethod != "plain" || string(out.AuthData) != "secret" {
		t.Fatalf("auth mismatch")
	}
	if out.Properties["model"] != "x1" || out.Properties["fw"] != "1.2" {
		t.Fatalf("properties mismatch: %+v", out.Properties)
	}
}

func TestConnackRoundTrip(t *testing.T) {
	in := NewConnackPacket(ReasonSuccess)
	in.SetSessionPresent(true)
	in.AssignedClientID = "c1"
	in.KeepAlive = 25
	in.SessionID = 42
	in.Pending = [3]uint32{1, 2, 3}
	in.Limits = Limits{MaxPacketSize: 1024, MaxPullBatch: 8}
	in.Properties = map[string]string{"region": "cn", "limit": "32"}

	out := roundTrip(t, in).(*ConnackPacket)
	if !out.SessionPresent() || out.AssignedClientID != "c1" || out.SessionID != 42 || out.KeepAlive != 25 {
		t.Fatalf("mismatch: %+v", out)
	}
	if out.Pending != in.Pending {
		t.Fatalf("depth mismatch: %+v", out)
	}
	if out.Limits != in.Limits {
		t.Fatalf("limits mismatch: %+v", out.Limits)
	}
	if out.Properties["region"] != "cn" || out.Properties["limit"] != "32" {
		t.Fatalf("properties mismatch: %+v", out.Properties)
	}
}

func TestPushRoundTrip(t *testing.T) {
	rec := Record{Channel: "a/b", Payload: []byte("hello"), Headers: map[string]string{"x": "y"}}
	rec.SetPriority(PriorityHigh)

	in := NewPushPacket(7, rec)
	in.Flags |= PushFlagWantAck

	out := roundTrip(t, in).(*PushPacket)
	if out.RequestID != 7 || out.Flags&PushFlagWantAck == 0 {
		t.Fatalf("header mismatch: %+v", out)
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
	if out.RequestID != 9 || !out.HasRecord() || out.Pending != in.Pending || out.RecordCount() != 1 {
		t.Fatalf("mismatch: %+v", out)
	}
	if !out.Records[0].Dup() || out.Records[0].Priority() != PriorityLow {
		t.Fatalf("record mismatch: %+v", out.Records[0])
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
	if c.RequestID != 3 || len(c.Entries) != 1 || c.Entries[0].ID != id {
		t.Fatalf("commit mismatch")
	}

	d := NewDeclarePacket(4, []ChannelDecl{{Channel: "x/**"}, {Channel: "y"}})
	d.Channels[1].SetTargetOnly(true)
	dd := roundTrip(t, d).(*DeclarePacket)
	if len(dd.Channels) != 2 || dd.Channels[0].Channel != "x/**" {
		t.Fatalf("declare mismatch")
	}
	if dd.Channels[0].TargetOnly() || !dd.Channels[1].TargetOnly() {
		t.Fatalf("declare flags mismatch: %+v", dd.Channels)
	}

	resp := NewDeclareRespPacket(5)
	resp.Results = []ChannelResult{{Channel: "x/**", Reason: ReasonSuccess}}
	resp.Limits = Limits{MaxPacketSize: 4096, MaxWaitMs: 1000, MaxChannelsPerClient: 8}
	rr := roundTrip(t, resp).(*DeclareRespPacket)
	if len(rr.Results) != 1 || rr.Limits.MaxWaitMs != 1000 || rr.Limits.MaxChannelsPerClient != 8 {
		t.Fatalf("declare resp mismatch: %+v", rr)
	}
}

func TestOptionalPropertiesRoundTrip(t *testing.T) {
	props := map[string]string{"trace": "abc"}

	push := NewPushPacket(40, Record{Channel: "a", Payload: []byte("x")})
	push.Properties = props
	if out := roundTrip(t, push).(*PushPacket); out.Properties["trace"] != "abc" {
		t.Fatalf("push properties mismatch: %+v", out.Properties)
	}

	ok := NewPushOkPacket(41)
	ok.Reason = ReasonNotAuthorized
	ok.Properties = props
	if out := roundTrip(t, ok).(*PushOkPacket); out.Properties["trace"] != "abc" {
		t.Fatalf("push ok properties mismatch: %+v", out.Properties)
	}

	pull := NewPullPacket(42, 10)
	pull.Properties = props
	if out := roundTrip(t, pull).(*PullPacket); out.Properties["trace"] != "abc" {
		t.Fatalf("pull properties mismatch: %+v", out.Properties)
	}

	pullOK := NewPullOkPacket(43)
	pullOK.Properties = props
	if out := roundTrip(t, pullOK).(*PullOkPacket); out.Properties["trace"] != "abc" {
		t.Fatalf("pull ok properties mismatch: %+v", out.Properties)
	}

	declare := NewDeclarePacket(44, nil)
	declare.Properties = props
	if out := roundTrip(t, declare).(*DeclarePacket); out.Properties["trace"] != "abc" {
		t.Fatalf("declare properties mismatch: %+v", out.Properties)
	}

	declareResp := NewDeclareRespPacket(45)
	declareResp.Properties = props
	if out := roundTrip(t, declareResp).(*DeclareRespPacket); out.Properties["trace"] != "abc" {
		t.Fatalf("declare resp properties mismatch: %+v", out.Properties)
	}
}

func TestControlRoundTrip(t *testing.T) {
	p := roundTrip(t, NewPingPacket(11, 12345)).(*PingPacket)
	if p.Seq != 11 || p.Timestamp != 12345 {
		t.Fatalf("ping mismatch")
	}

	d := NewDisconnectPacket(ReasonSessionTakenOver)
	dd := roundTrip(t, d).(*DisconnectPacket)
	if dd.ReasonCode != ReasonSessionTakenOver {
		t.Fatalf("disconnect mismatch")
	}
}

func TestPriorityDefault(t *testing.T) {
	var r Record
	if r.Priority() != PriorityMedium {
		t.Fatalf("default priority should be medium, got %v", r.Priority())
	}
	if bits := (r.Flags & recordFlagPriorityMask) >> 1; bits != 0 {
		t.Fatalf("medium must encode as 00, got %02b", bits)
	}

	for _, tc := range []struct {
		p    Priority
		wire uint8
	}{
		{PriorityLow, 1},
		{PriorityMedium, 0},
		{PriorityHigh, 2},
	} {
		r.SetPriority(tc.p)
		if r.Priority() != tc.p {
			t.Fatalf("priority %v not preserved, got %v", tc.p, r.Priority())
		}
		if bits := (r.Flags & recordFlagPriorityMask) >> 1; bits != tc.wire {
			t.Fatalf("priority %v encoded as %02b, want %02b", tc.p, bits, tc.wire)
		}

		rec := Record{Channel: "c", Payload: []byte("x")}
		rec.SetPriority(tc.p)
		body, err := EncodeRecordBody(&rec)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeRecord(body)
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Priority() != tc.p {
			t.Fatalf("priority %v not preserved on the wire, got %v", tc.p, decoded.Priority())
		}
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
	frame[recordFlagsOffset] = 0x03 << 1 // Priority=11：非法

	if _, _, err := ReadPacketWithSize(bytes.NewReader(frame), 1<<20); err == nil {
		t.Fatal("expected error for invalid priority bits")
	}
}

// 回归：Record 的保留位必须被拒绝，避免不同实现产生不同解释。
func TestReservedRecordFlagRejected(t *testing.T) {
	frame, err := EncodePacket(NewPushPacket(1, Record{Channel: "a", Payload: []byte("x")}), 0)
	if err != nil {
		t.Fatal(err)
	}
	const recordFlagsOffset = 5 + 4 + 1
	frame[recordFlagsOffset] |= 1 << 4

	if _, _, err := ReadPacketWithSize(bytes.NewReader(frame), 1<<20); err == nil {
		t.Fatal("expected error for reserved record flag")
	}
}

func TestUnknownPacketFlagsRejected(t *testing.T) {
	cases := []struct {
		name string
		pkt  Packet
		pos  int
	}{
		{"connect", NewConnectPacket("c"), 6},
		{"connack", NewConnackPacket(ReasonSuccess), 5},
		{"push", NewPushPacket(1, Record{Channel: "a"}), 9},
		{"push_ok", NewPushOkPacket(1), 9},
		{"pull", NewPullPacket(1, 0), 9},
		{"pull_ok", NewPullOkPacket(1), 9},
		{"declare", NewDeclarePacket(1, nil), 9},
		{"declare_resp", NewDeclareRespPacket(1), 9},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frame, err := EncodePacket(tc.pkt, 0)
			if err != nil {
				t.Fatal(err)
			}
			frame[tc.pos] |= 0x80
			if _, _, err := ReadPacketWithSize(bytes.NewReader(frame), 1<<20); err == nil {
				t.Fatal("expected error for unknown packet flags")
			}
		})
	}
}

func TestUnknownCommitActionRejected(t *testing.T) {
	frame, err := EncodePacket(NewCommitPacket(1, NewPacketID()), 0)
	if err != nil {
		t.Fatal(err)
	}
	const actionOffset = 5 + 4 + 2 + PacketIDSize
	frame[actionOffset] = 2
	if _, _, err := ReadPacketWithSize(bytes.NewReader(frame), 1<<20); err == nil {
		t.Fatal("expected error for unknown commit action")
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

func TestRecordReliability(t *testing.T) {
	var r Record
	if r.Reliable() {
		t.Fatal("reliability must default to false")
	}
	r.SetPriority(PriorityHigh)
	if r.Reliable() {
		t.Fatal("priority must not imply reliability")
	}
	r.SetReliable(true)
	if !r.Reliable() {
		t.Fatal("explicit reliability not preserved")
	}
	r.SetPriority(PriorityLow)
	if !r.Reliable() {
		t.Fatal("reliability must not change with priority")
	}
	r.SetReliable(false)
	if r.Reliable() {
		t.Fatal("explicit unreliable not preserved")
	}

	rec := Record{Channel: "c", Payload: []byte("x")}
	rec.SetReliable(true)
	body, err := EncodeRecordBody(&rec)
	if err != nil {
		t.Fatal(err)
	}
	if body[0]&0x40 != 0 {
		t.Fatalf("reserved reliability bit must stay clear: %#x", body[0])
	}
	decoded, err := DecodeRecord(body)
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.Reliable() {
		t.Fatal("reliable bit must survive round trip")
	}
}

func TestPushOkRoundTrip(t *testing.T) {
	in := NewPushOkPacket(30)
	in.Reason = ReasonSuccess
	in.ID = NewPacketID()
	in.Matched = 2
	in.Dropped = 1

	out := roundTrip(t, in).(*PushOkPacket)
	if out.RequestID != 30 || out.Reason != ReasonSuccess || out.ID != in.ID ||
		out.Matched != 2 || out.Dropped != 1 {
		t.Fatalf("push ok mismatch: %+v", out)
	}
}

func TestPushOkSizeMatchesEncoding(t *testing.T) {
	p := NewPushOkPacket(1)
	p.Properties = map[string]string{"error": "not authorized"}
	frame, err := EncodePacket(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) != p.Size() {
		t.Fatalf("PushOk.Size() = %d, encoded size = %d", p.Size(), len(frame))
	}
}

func TestBatchRoundTrip(t *testing.T) {
	pull := NewPullPacket(20, 100)
	pull.MaxCount = 8
	pull.MaxBytes = 4096
	pp := roundTrip(t, pull).(*PullPacket)
	if pp.MaxCount != 8 || pp.MaxBytes != 4096 || pp.WaitMs != 100 {
		t.Fatalf("pull mismatch: %+v", pp)
	}

	resp := NewPullOkPacket(21)
	for i := 0; i < 3; i++ {
		rec := Record{Channel: "a", Payload: []byte{byte(i)}}
		rec.SetPriority(PriorityMedium)
		resp.AddRecord(rec)
	}
	rr := roundTrip(t, resp).(*PullOkPacket)
	if rr.RecordCount() != 3 || rr.Records[2].Payload[0] != 2 {
		t.Fatalf("pull ok batch mismatch: %+v", rr)
	}

	ids := []PacketID{NewPacketID(), NewPacketID()}
	commit := NewCommitBatchPacket(22, []CommitEntry{
		{ID: ids[0], Action: CommitActionCommit},
		{ID: ids[1], Action: CommitActionNack, Reason: ReasonConsumerBacklog},
	})
	cc := roundTrip(t, commit).(*CommitPacket)
	if len(cc.Entries) != 2 || cc.Entries[1].Action != CommitActionNack ||
		cc.Entries[1].Reason != ReasonConsumerBacklog {
		t.Fatalf("commit batch mismatch: %+v", cc)
	}

	ok := NewCommitOkPacket(23)
	ok.Committed, ok.Nacked, ok.Unknown = 2, 1, 3
	ook := roundTrip(t, ok).(*CommitOkPacket)
	if ook.Committed != 2 || ook.Nacked != 1 || ook.Unknown != 3 {
		t.Fatalf("commit ok mismatch: %+v", ook)
	}
}

func TestRecordFlagLayout(t *testing.T) {
	rec := Record{
		Channel: "c",
		Headers: map[string]string{"k": "v"},
		Payload: []byte("x"),
	}
	rec.SetReliable(true)
	rec.SetExpiresAt(123)
	rec.SetPublishedAt(456)

	body, err := EncodeRecordBody(&rec)
	if err != nil {
		t.Fatal(err)
	}
	flags := body[0]
	if flags&(1<<4) != 0 {
		t.Fatalf("reserved bit4 must stay clear: %08b", flags)
	}
	for _, bit := range []struct {
		name string
		bit  uint8
	}{
		{"HasHeaders", 1 << 3},
		{"Reliable", 1 << 5},
		{"HasExpiry", 1 << 6},
		{"HasPublishedAt", 1 << 7},
	} {
		if flags&bit.bit == 0 {
			t.Fatalf("%s flag not set: %08b", bit.name, flags)
		}
	}
}

func TestRecordExpiryRoundTrip(t *testing.T) {
	rec := Record{Channel: "c", Payload: []byte("x")}
	rec.SetExpiresAt(123456789)
	rec.SetPublishedAt(123456000)

	body, err := EncodeRecordBody(&rec)
	if err != nil {
		t.Fatal(err)
	}
	out, err := DecodeRecord(body)
	if err != nil {
		t.Fatal(err)
	}
	if !out.HasExpiry() || out.ExpiresAt != 123456789 {
		t.Fatalf("expiry mismatch: %+v", out)
	}
	if !out.HasPublishedAt() || out.PublishedAt != 123456000 {
		t.Fatalf("publishedAt mismatch: %+v", out)
	}
	if out.Expired(123456788) || !out.Expired(123456789) {
		t.Fatal("expiry boundary mismatch")
	}
}
