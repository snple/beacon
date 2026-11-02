package core_test

import (
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
	connect.SessionTimeout = 60
	connect.MaxPacketSize = 1 << 20
	for _, ch := range channels {
		connect.Channels = append(connect.Channels, packet.ChannelDecl{Channel: ch})
	}
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
	return r
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
	rc.send(packet.NewPullPacket(1, 10, 0, 3000))

	time.Sleep(150 * time.Millisecond)

	// 第二个 PULL 应立刻被拒绝
	start := time.Now()
	rc.send(packet.NewPullPacket(2, 10, 0, 100))

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
	if first.RetryAfterMs != 1000 {
		t.Fatalf("RetryAfterMs = %d, want 1000", first.RetryAfterMs)
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
	if len(second.Records) != 0 {
		t.Fatalf("expected empty records, got %d", len(second.Records))
	}
}

// 长轮询期间连接仍能处理 PING（读循环不被阻塞）。
func TestLongPollDoesNotBlockPing(t *testing.T) {
	_, addr := startCore(t, nil)
	rc := dialRaw(t, addr, "raw-2", "ch/a")

	rc.send(packet.NewPullPacket(1, 10, 0, 3000))
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

// 长轮询期间有新消息时立即唤醒。
func TestLongPollWakesWithData(t *testing.T) {
	b, addr := startCore(t, nil)
	rc := dialRaw(t, addr, "raw-3", "ch/a")

	rc.send(packet.NewPullPacket(1, 10, 0, 5000))
	time.Sleep(150 * time.Millisecond)

	b.Push("ch/a", []byte("hello"), core.PushOptions{Priority: packet.PriorityHigh})

	pkt := rc.expect(t, packet.PULL_OK)
	ok := pkt.(*packet.PullOkPacket)
	if len(ok.Records) != 1 || string(ok.Records[0].Payload) != "hello" {
		t.Fatalf("unexpected records: %+v", ok.Records)
	}
	if ok.Records[0].Dup() {
		t.Fatal("first delivery should not be dup")
	}
}
