package core_test

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/core"
	"github.com/snple/beacon/packet"
)

// rawConn 是绕过 client SDK 的最小协议客户端，用于测试协议层行为。
type rawConn struct {
	nc net.Conn
	t  *testing.T
}

func dialRaw(t *testing.T, addr, clientID string, channels ...string) *rawConn {
	t.Helper()

	nc, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = nc.SetDeadline(time.Now().Add(10 * time.Second))

	r := &rawConn{nc: nc, t: t}
	t.Cleanup(func() { _ = nc.Close() })

	connect := packet.NewConnectPacket(clientID)
	connect.KeepAlive = 60
	connect.MaxPacketSize = 1 << 20
	r.send(connect)

	resp, err := r.recv()
	if err != nil {
		t.Fatalf("read connack: %v", err)
	}
	connack, ok := resp.(*packet.ConnackPacket)
	if !ok {
		t.Fatalf("expected CONNACK, got %s", resp.Type())
	}
	if connack.ReasonCode != packet.ReasonSuccess {
		t.Fatalf("connack reason: %s", connack.ReasonCode)
	}

	if len(channels) > 0 {
		r.declare(channels...)
	}
	return r
}

// declare 发送 DECLARE 并等待 DECLARE_RESP（订阅声明不再随 CONNECT 发送）。
func (r *rawConn) declare(channels ...string) *packet.DeclareRespPacket {
	r.t.Helper()

	decls := make([]packet.ChannelDecl, len(channels))
	for i, ch := range channels {
		decls[i] = packet.ChannelDecl{Channel: ch}
	}
	req := packet.NewDeclarePacket(uint32(time.Now().UnixNano()), decls)
	r.send(req)

	resp, err := r.recv()
	if err != nil {
		r.t.Fatalf("read declare resp: %v", err)
	}
	ok, valid := resp.(*packet.DeclareRespPacket)
	if !valid {
		r.t.Fatalf("expected DECLARE_RESP, got %s", resp.Type())
	}
	if len(ok.Results) > 0 {
		r.t.Fatalf("declare rejected: %+v", ok.Results)
	}
	return ok
}

func (r *rawConn) send(pkt packet.Packet) {
	r.t.Helper()
	if err := packet.WritePacket(r.nc, pkt, 0); err != nil {
		r.t.Fatalf("send %s: %v", pkt.Type(), err)
	}
}

func (r *rawConn) recv() (packet.Packet, error) {
	return packet.ReadPacket(r.nc, 1<<20)
}

func (r *rawConn) expect(t *testing.T, want packet.PacketType) packet.Packet {
	t.Helper()
	pkt, err := r.recv()
	if err != nil {
		t.Fatalf("recv %s: %v", want, err)
	}
	if pkt.Type() != want {
		t.Fatalf("expected %s, got %s", want, pkt.Type())
	}
	return pkt
}

// 服务端必须拒绝同一连接上的第二个在途 PULL。
func TestServerRejectsSecondInFlightPull(t *testing.T) {
	_, addr := startCore(t, nil)
	rc := dialRaw(t, addr, "raw-1", "ch/a")

	// 第一个 PULL 进入长轮询（队列为空）
	rc.send(packet.NewPullPacket(1, 3000))

	time.Sleep(150 * time.Millisecond)

	// 第二个 PULL 应立刻被拒绝
	start := time.Now()
	rc.send(packet.NewPullPacket(2, 100))

	pkt := rc.expect(t, packet.PULL_OK)
	first := pkt.(*packet.PullOkPacket)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("second pull was not rejected immediately: %v", elapsed)
	}
	if first.RequestID != 2 {
		t.Fatalf("expected the rejected request first, got requestID=%d", first.RequestID)
	}
	if first.Reason != packet.ReasonRateLimited {
		t.Fatalf("expected RateLimited, got %s", first.Reason)
	}

	// 第一个 PULL 仍在等待，随后超时返回空响应
	pkt = rc.expect(t, packet.PULL_OK)
	second := pkt.(*packet.PullOkPacket)
	if second.RequestID != 1 {
		t.Fatalf("expected requestID=1, got %d", second.RequestID)
	}
	if second.Reason != packet.ReasonSuccess {
		t.Fatalf("first pull should succeed with empty result, got %s", second.Reason)
	}
	if second.HasRecord() {
		t.Fatal("expected empty result")
	}
}

// 长轮询期间连接仍能处理 PING（读循环不被阻塞）。
func TestLongPollDoesNotBlockPing(t *testing.T) {
	_, addr := startCore(t, nil)
	rc := dialRaw(t, addr, "raw-2", "ch/a")

	rc.send(packet.NewPullPacket(1, 3000))
	time.Sleep(100 * time.Millisecond)

	start := time.Now()
	rc.send(packet.NewPingPacket(7, 12345))
	pkt := rc.expect(t, packet.PONG)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("ping blocked by long poll: %v", elapsed)
	}
	if pong := pkt.(*packet.PongPacket); pong.Seq != 7 || pong.Echo != 12345 {
		t.Fatalf("unexpected pong: %+v", pong)
	}

	pkt = rc.expect(t, packet.PULL_OK)
	if got := pkt.(*packet.PullOkPacket); got.RequestID != 1 {
		t.Fatalf("unexpected pull response id %d", got.RequestID)
	}
}

// 回归：非法优先级位（3）不能使 broker panic；连接应被拒绝且服务继续可用。
func TestInvalidPriorityIsRejected(t *testing.T) {
	_, addr := startCore(t, nil)
	rc := dialRaw(t, addr, "raw-prio", "ch/a")

	body := []byte{}
	body = append(body, 0, 0, 0, 1)               // RequestID
	body = append(body, packet.PushFlagWantAck)   // Flags
	body = append(body, 0x16)                     // Record flags: HasPriority | priority=3
	body = append(body, make([]byte, 12)...)      // ID
	body = append(body, 0, 4, 'c', 'h', '/', 'a') // Channel
	body = append(body, 0, 0)                     // Target
	body = append(body, 0, 0)                     // Source
	body = append(body, 0, 0)                     // Headers
	body = append(body, 0, 0, 0, 0)               // Payload

	frame := make([]byte, 5+len(body))
	frame[0] = byte(packet.PUSH)
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(body)))
	copy(frame[5:], body)

	if _, err := rc.nc.Write(frame); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = rc.nc.SetReadDeadline(time.Now().Add(2 * time.Second))
	// 服务端必须显式告知断开原因，而不是静默关闭。
	pkt, err := rc.recv()
	if err != nil {
		t.Fatalf("expected DISCONNECT before close: %v", err)
	}
	dp, ok := pkt.(*packet.DisconnectPacket)
	if !ok || dp.ReasonCode != packet.ReasonMalformedPacket {
		t.Fatalf("expected DISCONNECT(MalformedPacket), got %s %+v", pkt.Type(), pkt)
	}
	if _, err := rc.recv(); err == nil {
		t.Fatal("expected connection to be closed after DISCONNECT")
	}

	// 服务端仍然存活，可接受新连接。
	_ = dialRaw(t, addr, "raw-prio-2", "ch/a")
}

// 回归：PULL 结果必须适配客户端声明的 MaxPacketSize，不能静默断连。
func TestPullHonorsClientMaxPacketSize(t *testing.T) {
	b, addr := startCore(t, nil)

	nc, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	_ = nc.SetDeadline(time.Now().Add(10 * time.Second))

	connect := packet.NewConnectPacket("small-buf")
	connect.KeepAlive = 60
	connect.MaxPacketSize = 512
	if err := packet.WritePacket(nc, connect, 0); err != nil {
		t.Fatal(err)
	}
	pkt, err := packet.ReadPacket(nc, 1<<20)
	if err != nil {
		t.Fatalf("read connack: %v", err)
	}
	if connack := pkt.(*packet.ConnackPacket); connack.ReasonCode != packet.ReasonSuccess {
		t.Fatalf("connack: %s", connack.ReasonCode)
	}

	// 定向投递要求目标声明过该 channel，这里显式声明。
	rc := &rawConn{nc: nc, t: t}
	rc.declare("big/ch")

	b.Push("big/ch", make([]byte, 2048), core.PushOptions{}.WithPriority(packet.PriorityMedium))

	if err := packet.WritePacket(nc, packet.NewPullPacket(1, 100), 0); err != nil {
		t.Fatal(err)
	}
	pkt, err = packet.ReadPacket(nc, 1<<20)
	if err != nil {
		t.Fatalf("server disconnected instead of returning a PULL_OK: %v", err)
	}
	ok := pkt.(*packet.PullOkPacket)
	if ok.Reason != packet.ReasonPacketTooLarge {
		t.Fatalf("expected PacketTooLarge, got %s", ok.Reason)
	}
	if ok.HasRecord() {
		t.Fatal("expected no record")
	}

	// 消息没有被丢弃，仍在队列中等待客户端提高上限后重试。
	info, err := b.GetClientInfo("small-buf")
	if err != nil {
		t.Fatal(err)
	}
	if info.Pending[1]+info.Pending[2] != 1 {
		t.Fatalf("message should remain queued, pending=%v", info.Pending)
	}
}

// CONNACK 必须告知服务端实际要求的 KeepAlive。
func TestConnackCarriesServerKeepAlive(t *testing.T) {
	_, addr := startCore(t, core.NewCoreOptions().WithKeepAlive(15))

	nc, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	_ = nc.SetDeadline(time.Now().Add(5 * time.Second))

	connect := packet.NewConnectPacket("ka-2")
	connect.KeepAlive = 60
	if err := packet.WritePacket(nc, connect, 0); err != nil {
		t.Fatal(err)
	}
	pkt, err := packet.ReadPacket(nc, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	connack := pkt.(*packet.ConnackPacket)
	if connack.KeepAlive != 15 {
		t.Fatalf("CONNACK KeepAlive = %d, want 15", connack.KeepAlive)
	}
}

// 长轮询期间有新消息时立即唤醒。
func TestLongPollWakesWithData(t *testing.T) {
	b, addr := startCore(t, nil)
	rc := dialRaw(t, addr, "raw-3", "ch/a")

	rc.send(packet.NewPullPacket(1, 5000))
	time.Sleep(150 * time.Millisecond)

	b.Push("ch/a", []byte("hello"), core.PushOptions{}.WithPriority(packet.PriorityHigh))

	pkt := rc.expect(t, packet.PULL_OK)
	ok := pkt.(*packet.PullOkPacket)
	if !ok.HasRecord() || string(ok.Record.Payload) != "hello" {
		t.Fatalf("unexpected record: %+v", ok.Record)
	}
	if ok.Record.Dup() {
		t.Fatal("first delivery should not be dup")
	}
}

// 不向后兼容：版本不是 4 时静默关闭，不返回 CONNACK。
func TestUnsupportedVersionClosesSilently(t *testing.T) {
	_, addr := startCore(t, nil)

	nc, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	_ = nc.SetReadDeadline(time.Now().Add(3 * time.Second))

	connect := packet.NewConnectPacket("old-proto")
	connect.Version = 3
	if err := packet.WritePacket(nc, connect, 0); err != nil {
		t.Fatal(err)
	}

	if pkt, err := packet.ReadPacket(nc, 1<<20); err == nil {
		t.Fatalf("expected silent close, got %s", pkt.Type())
	}
}

// 客户端发送服务端方向的包属于协议错误，必须收到 DISCONNECT(ProtocolError)。
func TestUnexpectedPacketSendsProtocolError(t *testing.T) {
	_, addr := startCore(t, nil)
	rc := dialRaw(t, addr, "raw-proto", "ch/a")

	rc.send(packet.NewConnackPacket(packet.ReasonSuccess))

	_ = rc.nc.SetReadDeadline(time.Now().Add(2 * time.Second))
	pkt, err := rc.recv()
	if err != nil {
		t.Fatalf("expected DISCONNECT: %v", err)
	}
	dp, ok := pkt.(*packet.DisconnectPacket)
	if !ok || dp.ReasonCode != packet.ReasonProtocolError {
		t.Fatalf("expected DISCONNECT(ProtocolError), got %s %+v", pkt.Type(), pkt)
	}
}

// PUSH_OK 始终携带单条记录的结果。
func TestPushOkReportsResult(t *testing.T) {
	_, addr := startCore(t, nil)
	rc := dialRaw(t, addr, "raw-ok", "ch/a")

	rec := packet.Record{Channel: "ch/a", Payload: []byte("x")}
	rec.SetPriority(packet.PriorityMedium)
	pkt := packet.NewPushPacket(9, rec)
	pkt.Flags |= packet.PushFlagWantAck
	rc.send(pkt)

	resp := rc.expect(t, packet.PUSH_OK).(*packet.PushOkPacket)
	if resp.Reason != packet.ReasonSuccess || resp.Matched != 1 || resp.Dropped != 0 || resp.ID.IsZero() {
		t.Fatalf("unexpected result: %+v", resp)
	}
}

// 无匹配 channel 时返回 NoMatch 与服务端生成的 ID。
func TestPushNoMatchReportsResult(t *testing.T) {
	_, addr := startCore(t, nil)
	rc := dialRaw(t, addr, "raw-nomatch", "ch/a")

	rec := packet.Record{Channel: "ch/b", Payload: []byte("x")}
	rec.SetPriority(packet.PriorityMedium)
	pkt := packet.NewPushPacket(10, rec)
	pkt.Flags |= packet.PushFlagWantAck
	rc.send(pkt)

	resp := rc.expect(t, packet.PUSH_OK).(*packet.PushOkPacket)
	if resp.Reason != packet.ReasonNoMatch || resp.ID.IsZero() {
		t.Fatalf("unexpected result: %+v", resp)
	}
}

// WantAck=0 时服务端不回包，但消息仍然入队。
func TestPushWithoutAckReturnsNoResponse(t *testing.T) {
	_, addr := startCore(t, nil)
	rc := dialRaw(t, addr, "raw-noack", "ch/a")

	rec := packet.Record{Channel: "ch/a", Payload: []byte("x")}
	rec.SetPriority(packet.PriorityMedium)
	rc.send(packet.NewPushPacket(11, rec))

	// 立即 PULL 应该能取到刚推送的消息；若服务端错误地回了 PUSH_OK，
	// 这里会先读到 PUSH_OK 而失败。
	rc.send(packet.NewPullPacket(12, 1000))
	pkt := rc.expect(t, packet.PULL_OK).(*packet.PullOkPacket)
	if !pkt.HasRecord() || string(pkt.Record.Payload) != "x" {
		t.Fatalf("unexpected pull result: %+v", pkt.Record)
	}
}

// 服务端必须拒绝无法理解的声明标志位，而不是静默忽略。
func TestDeclareUnknownFlagsRejected(t *testing.T) {
	_, addr := startCore(t, nil)
	rc := dialRaw(t, addr, "raw-decl")

	req := packet.NewDeclarePacket(20, []packet.ChannelDecl{{
		Channel: "a/b",
		Flags:   1 << 7,
	}})
	rc.send(req)

	resp := rc.expect(t, packet.DECLARE_RESP).(*packet.DeclareRespPacket)
	if resp.Accepted != 0 {
		t.Fatalf("unexpected accepted count: %+v", resp)
	}
	if len(resp.Results) != 1 || resp.Results[0].Channel != "a/b" ||
		resp.Results[0].Reason != packet.ReasonChannelFilterInvalid {
		t.Fatalf("unexpected declare results: %+v", resp.Results)
	}
}
