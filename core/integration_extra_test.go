package core_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// 会话总数上限必须覆盖离线保留会话。
func TestMaxSessionsRejected(t *testing.T) {
	_, addr := startCore(t, core.NewCoreOptions().WithMaxSessions(1))
	c1 := newTestClient(t, addr, "max-1", "ch/a")
	defer func() { _ = c1.Close() }()

	opts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("max-2").
		WithAutoReconnect(false).
		WithLogger(zap.NewNop())
	c2, err := client.NewWithOptions(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c2.Close() }()

	err = c2.ConnectContext(context.Background())
	if err == nil {
		t.Fatal("second session should be rejected")
	}
	var se *client.ServerError
	if !errors.As(err, &se) || se.Reason != packet.ReasonServerBusy {
		t.Fatalf("expected ServerBusy, got %v", err)
	}
}

// 全局在途字节上限必须阻止无界内存增长。
func TestMaxInflightBytesRejected(t *testing.T) {
	_, addr := startCore(t, core.NewCoreOptions().WithMaxInflightBytes(1))
	sub := newTestClient(t, addr, "inflight-sub", "ch/a")
	defer func() { _ = sub.Close() }()
	pub := newTestClient(t, addr, "inflight-pub")
	defer func() { _ = pub.Close() }()

	ctx := context.Background()
	if err := pub.Push(ctx, "ch/a", []byte("first"), client.PushOptions{}); err != nil {
		t.Fatalf("first push should be accepted: %v", err)
	}
	err := pub.Push(ctx, "ch/a", []byte("second"), client.PushOptions{})
	if err == nil {
		t.Fatal("second push should exceed the in-flight limit")
	}
	var se *client.ServerError
	if !errors.As(err, &se) || se.Reason != packet.ReasonQuotaExceeded {
		t.Fatalf("expected QuotaExceeded, got %v", err)
	}
}

// 重复 Connect 必须显式失败，不能泄漏旧连接或启动多个 supervise。
func TestDuplicateConnectRejected(t *testing.T) {
	_, addr := startCore(t, nil)
	c := newTestClient(t, addr, "dup-connect", "ch/a")
	defer func() { _ = c.Close() }()

	if err := c.ConnectContext(context.Background()); !errors.Is(err, client.ErrAlreadyConnected) {
		t.Fatalf("expected ErrAlreadyConnected, got %v", err)
	}
}

func TestPollMessage(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := pub.Push(ctx, "ch/a", []byte{byte('a' + i)}, client.PushOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 3; i++ {
		m, err := sub.PollMessage(ctx, 2*time.Second)
		if err != nil {
			t.Fatalf("poll %d: %v", i, err)
		}
		if string(m.Payload) != string([]byte{byte('a' + i)}) {
			t.Fatalf("unexpected payload %q", m.Payload)
		}
	}
}

func TestPollMessageTimeout(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")

	start := time.Now()
	_, err := sub.PollMessage(context.Background(), 300*time.Millisecond)
	if !errors.Is(err, client.ErrPollTimeout) {
		t.Fatalf("expected ErrPollTimeout, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("timeout took too long: %v", elapsed)
	}
}

func TestPushSingleRecord(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/**")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	for _, m := range []struct {
		channel string
		payload string
	}{
		{"ch/a", "1"},
		{"ch/b", "2"},
		{"other/c", "3"},
	} {
		if err := pub.Push(ctx, m.channel, []byte(m.payload), client.PushOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	for _, want := range []string{"1", "2"} {
		msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
		if err != nil {
			t.Fatal(err)
		}
		if msg == nil || string(msg.Payload) != want {
			t.Fatalf("expected %q, got %+v", want, msg)
		}
		if err := msg.Ack(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPushWithoutAck(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	if err := pub.Push(ctx, "ch/a", []byte("noack"), client.PushOptions{}.WithoutAck()); err != nil {
		t.Fatal(err)
	}

	// 异步到达，稍等
	deadline := time.Now().Add(2 * time.Second)
	for b.GetStats().MessagesReceived == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "noack" {
		t.Fatalf("expected noack message, got %+v", msg)
	}
}

func TestOfflineQueueDeliveredOnReconnect(t *testing.T) {
	b, addr := startCore(t, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c1 := newTestClient(t, addr, "device-1", "cmd/**")
	if err := c1.Close(); err != nil {
		t.Fatal(err)
	}

	// 等会话解绑
	deadline := time.Now().Add(2 * time.Second)
	for b.GetStats().ClientsConnected != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if _, err := b.PushToClient("device-1", "cmd/x", []byte("queued"), core.PushOptions{}); err != nil {
		t.Fatalf("publish to offline session: %v", err)
	}

	c2, err := client.NewWithOptions(client.NewClientOptions().
		WithCore(addr).
		WithClientID("device-1").
		WithAutoReconnect(false).
		WithLogger(zap.NewNop()))
	if err != nil {
		t.Fatal(err)
	}
	if err := c2.ConnectContext(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c2.Close() }()

	if !c2.SessionPresent() {
		t.Fatal("session should be present after reconnect")
	}

	msg, err := c2.Pull(ctx, client.PullOptions{WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "queued" {
		t.Fatalf("offline queue not delivered: %+v", msg)
	}
}

func TestWebSocketTransport(t *testing.T) {
	b, err := core.NewWithOptions(core.NewCoreOptions().WithLogger(zap.NewNop()))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.Stop() }()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = b.ServeWSListener(l, "/ws") }()
	time.Sleep(100 * time.Millisecond)

	addr := "ws://" + l.Addr().String() + "/ws"
	opts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("ws-1").
		WithAutoReconnect(false).
		WithLogger(zap.NewNop())
	c, err := client.NewWithOptions(opts)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.ConnectContext(ctx); err != nil {
		t.Fatalf("ws connect: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.Declare(ctx, "ws/**"); err != nil {
		t.Fatal(err)
	}

	pub, err := client.NewWithOptions(client.NewClientOptions().
		WithCore(addr).
		WithClientID("ws-2").
		WithAutoReconnect(false).
		WithLogger(zap.NewNop()))
	if err != nil {
		t.Fatal(err)
	}
	if err := pub.ConnectContext(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pub.Close() }()

	if err := pub.Push(ctx, "ws/hello", []byte("over ws"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}

	msg, err := c.Pull(ctx, client.PullOptions{WaitMs: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "over ws" {
		t.Fatalf("ws delivery failed: %+v", msg)
	}
}

func TestSessionExpiry(t *testing.T) {
	b, addr := startCore(t, core.NewCoreOptions().
		WithSessionTimeout(1).
		WithSessionCheckInterval(100*time.Millisecond))

	c, err := client.NewWithOptions(client.NewClientOptions().
		WithCore(addr).
		WithClientID("tmp-1").
		WithAutoReconnect(false).
		WithLogger(zap.NewNop()))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := c.ConnectContext(ctx); err != nil {
		t.Fatal(err)
	}
	_ = c.Close()

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := b.GetClientInfo("tmp-1"); err != nil {
			return // 会话已过期并清理
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("session should have expired")
}

func TestDeclareReplacesChannels(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "a/**")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	if _, err := sub.Declare(ctx, "b/**"); err != nil {
		t.Fatal(err)
	}

	if err := pub.Push(ctx, "a/x", []byte("no"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := pub.Push(ctx, "b/x", []byte("yes"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}

	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || msg.Channel != "b/x" {
		t.Fatalf("declare did not replace: %+v", msg)
	}
}

func TestMessageAck(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	if err := pub.Push(ctx, "ch/a", []byte("v"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}
	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil {
		t.Fatal("expected 1 message")
	}
	if err := msg.Ack(ctx); err != nil {
		t.Fatal(err)
	}

	again, err := sub.Pull(ctx, client.PullOptions{WaitMs: 200})
	if err != nil {
		t.Fatal(err)
	}
	if again != nil {
		t.Fatalf("acked message should not be redelivered, got %+v", again)
	}
}

func TestPriorityHighBeforeMedium(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	medOpt := client.PushOptions{}.WithPriority(packet.PriorityMedium)
	highOpt := client.PushOptions{}.WithPriority(packet.PriorityHigh)

	if err := pub.Push(ctx, "ch/a", []byte("med"), medOpt); err != nil {
		t.Fatal(err)
	}
	if err := pub.Push(ctx, "ch/a", []byte("high"), highOpt); err != nil {
		t.Fatal(err)
	}

	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "high" {
		t.Fatalf("expected high priority first, got %+v", msg)
	}
}

// 高优先级消息必须能绕过未确认的中优先级消息，占用第二个投递位。
func TestHighPriorityBypassesUnackedMedium(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	medOpt := client.PushOptions{}.WithPriority(packet.PriorityMedium)
	highOpt := client.PushOptions{}.WithPriority(packet.PriorityHigh)

	if err := pub.Push(ctx, "ch/a", []byte("med"), medOpt); err != nil {
		t.Fatal(err)
	}
	med, err := sub.Pull(ctx, client.PullOptions{WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if med == nil || string(med.Payload) != "med" {
		t.Fatalf("expected medium message, got %+v", med)
	}
	// 故意不 commit med。

	if err := pub.Push(ctx, "ch/a", []byte("high"), highOpt); err != nil {
		t.Fatal(err)
	}
	high, err := sub.Pull(ctx, client.PullOptions{WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if high == nil || string(high.Payload) != "high" || high.Dup {
		t.Fatalf("high priority should bypass unacked medium, got %+v", high)
	}

	info, err := b.GetClientInfo("sub-1")
	if err != nil {
		t.Fatal(err)
	}
	if info.Delivered != 2 {
		t.Fatalf("delivered=%d, want 2 occupied slots", info.Delivered)
	}

	if n, err := sub.CommitAll(ctx); err != nil || n != 2 {
		t.Fatalf("commit: n=%d err=%v", n, err)
	}
	if next, err := sub.Pull(ctx, client.PullOptions{WaitMs: 100}); err != nil || next != nil {
		t.Fatalf("expected empty after commit, got %+v err=%v", next, err)
	}
}

func TestOnDeliverRejectsMessage(t *testing.T) {
	opts := core.NewCoreOptions().WithHooks(&core.Hooks{
		Message: core.MessageHandlerFunc{
			DeliverFn: func(ctx *core.DeliverContext) bool {
				return ctx.Channel != "ch/blocked"
			},
		},
	})
	b, addr := startCore(t, opts)
	sub := newTestClient(t, addr, "sub-1", "ch/**")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	if err := pub.Push(ctx, "ch/ok", []byte("keep"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := pub.Push(ctx, "ch/blocked", []byte("drop"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}

	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || msg.Channel != "ch/ok" {
		t.Fatalf("OnDeliver filter failed: %+v", msg)
	}
	if _, err := sub.CommitAll(ctx); err != nil {
		t.Fatal(err)
	}

	// 第二次拉取会取出 ch/blocked，OnDeliver 拒绝后直接丢弃。
	again, err := sub.Pull(ctx, client.PullOptions{WaitMs: 100})
	if err != nil {
		t.Fatal(err)
	}
	if again != nil {
		t.Fatalf("rejected message should be gone, got %+v", again)
	}
	if b.GetStats().MessagesDropped != 1 {
		t.Fatalf("dropped=%d", b.GetStats().MessagesDropped)
	}
}

// 回归：队列满丢弃新消息时不能重复释放引用，否则在途计数会变成负数。
func TestQueueFullDoesNotLeakInflight(t *testing.T) {
	b, addr := startCore(t, core.NewCoreOptions().WithQueueCap(1))
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	ctx := context.Background()

	res := b.Push("ch/a", []byte("high"), core.PushOptions{}.WithPriority(packet.PriorityHigh))
	if res.Matched != 1 {
		t.Fatalf("first push should match: %+v", res)
	}
	if got := b.GetStats().InFlightMessages; got != 1 {
		t.Fatalf("inflight after first push = %d, want 1", got)
	}

	// 队列已满且新消息优先级更低 → 新消息被丢弃
	res = b.Push("ch/a", []byte("low"), core.PushOptions{}.WithPriority(packet.PriorityLow))
	if res.Matched != 0 || res.Dropped != 1 {
		t.Fatalf("low-priority push should be dropped: %+v", res)
	}
	if got := b.GetStats().InFlightMessages; got != 1 {
		t.Fatalf("inflight after drop = %d, want 1 (no double release)", got)
	}
	if got := b.GetStats().InFlightBytes; got <= 0 {
		t.Fatalf("inflight bytes = %d", got)
	}

	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "high" {
		t.Fatalf("unexpected message: %+v", msg)
	}
	if _, err := sub.CommitAll(ctx); err != nil {
		t.Fatal(err)
	}

	if got := b.GetStats().InFlightMessages; got != 0 {
		t.Fatalf("inflight after commit = %d, want 0", got)
	}
	if got := b.GetStats().InFlightBytes; got != 0 {
		t.Fatalf("inflight bytes after commit = %d, want 0", got)
	}
}

func TestAdminPushDefaultsToMedium(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	ctx := context.Background()

	b.Push("ch/a", []byte("v"), core.PushOptions{})

	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil {
		t.Fatal("expected 1 message")
	}
	if msg.Priority != packet.PriorityMedium {
		t.Fatalf("default priority = %v, want medium", msg.Priority)
	}
	if !msg.NeedsCommit() {
		t.Fatal("medium priority must require commit")
	}
}

func TestAdminPushExplicitLow(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	ctx := context.Background()

	b.Push("ch/a", []byte("v"), core.PushOptions{}.WithPriority(packet.PriorityLow))

	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || msg.NeedsCommit() {
		t.Fatalf("explicit low priority should not need commit: %+v", msg)
	}
}

func TestAdminPushInvalidChannel(t *testing.T) {
	b, _ := startCore(t, nil)
	if _, err := b.PushToClient("x", "bad/*/channel", nil, core.PushOptions{}); err == nil {
		t.Fatal("expected error for invalid channel")
	}
}

// 回归：payload 在推送后被调用方修改，不应影响已入队消息。
func TestAdminPushCopiesPayload(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	ctx := context.Background()

	payload := []byte("original")
	b.Push("ch/a", payload, core.PushOptions{})
	copy(payload, "MODIFIED")

	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "original" {
		t.Fatalf("payload not copied: %+v", msg)
	}
}

// channel 名/过滤器的长度上限必须按服务端配置生效。
func TestChannelLengthLimitEnforced(t *testing.T) {
	ctx := context.Background()

	t.Run("default", func(t *testing.T) {
		_, addr := startCore(t, nil)
		c := newTestClient(t, addr, "len-default")

		assertPushChannel(t, c, ctx, strings.Repeat("a", packet.MinChannelLength), true)
		assertPushChannel(t, c, ctx, strings.Repeat("a", packet.MinChannelLength+1), false)
		assertDeclareChannel(t, c, ctx, strings.Repeat("a", packet.MinChannelLength), packet.ReasonSuccess)
		assertDeclareChannel(t, c, ctx, strings.Repeat("a", packet.MinChannelLength+1), packet.ReasonChannelFilterInvalid)
	})

	t.Run("configured", func(t *testing.T) {
		b, addr := startCore(t, core.NewCoreOptions().WithMaxChannelLength(512))
		if got := b.Info().MaxChannelLength; got != 512 {
			t.Fatalf("Info().MaxChannelLength=%d, want 512", got)
		}
		c := newTestClient(t, addr, "len-512")

		assertPushChannel(t, c, ctx, strings.Repeat("a", 512), true)
		assertPushChannel(t, c, ctx, strings.Repeat("a", 513), false)
		assertDeclareChannel(t, c, ctx, strings.Repeat("a", 512), packet.ReasonSuccess)
		assertDeclareChannel(t, c, ctx, strings.Repeat("a", 513), packet.ReasonChannelFilterInvalid)
	})
}

func assertPushChannel(t *testing.T, c *client.Client, ctx context.Context, channel string, wantOK bool) {
	t.Helper()
	err := c.Push(ctx, channel, nil, client.PushOptions{})
	if wantOK {
		// 没有订阅者时返回 NoMatch（非错误），说明 channel 本身已通过校验。
		if err != nil {
			t.Fatalf("channel len %d should be accepted: %v", len(channel), err)
		}
		return
	}
	var se *client.ServerError
	if !errors.As(err, &se) || se.Reason != packet.ReasonChannelInvalid {
		t.Fatalf("channel len %d: got %v, want ChannelInvalid", len(channel), err)
	}
}

func assertDeclareChannel(t *testing.T, c *client.Client, ctx context.Context, filter string, want packet.ReasonCode) {
	t.Helper()
	res, err := c.Declare(ctx, filter)
	if err != nil {
		t.Fatalf("declare len %d: %v", len(filter), err)
	}
	// DECLARE_RESP 只回显失败项；接受时结果为空。
	if want == packet.ReasonSuccess {
		if len(res) != 0 {
			t.Fatalf("declare len %d: expected no failures, got %+v", len(filter), res)
		}
		return
	}
	if len(res) != 1 || res[0].Reason != want {
		t.Fatalf("declare len %d: got %+v, want reason %s", len(filter), res, want)
	}
}

// 离线会话只保留 Medium/High；Low 直接丢弃。队列满后沿用既有淘汰策略。
func TestOfflineQueuePolicy(t *testing.T) {
	b, addr := startCore(t, core.NewCoreOptions().WithQueueCap(2))
	sub := newTestClient(t, addr, "offline-sub", "off/**")
	pub := newTestClient(t, addr, "offline-pub")
	ctx := context.Background()

	_ = sub.Close()
	waitClientDetached(t, b, "offline-sub")

	push := func(channel string, prio packet.Priority) {
		t.Helper()
		err := pub.Push(ctx, channel, []byte(channel), client.PushOptions{}.WithPriority(prio))
		if err != nil {
			t.Fatalf("push %s: %v", channel, err)
		}
	}

	push("off/m1", packet.PriorityMedium)
	push("off/h1", packet.PriorityHigh)
	// Low 在离线时被丢弃，不占用队列。
	push("off/l1", packet.PriorityLow)
	// 队列已满（cap=2）：挤掉最旧的 Medium。
	push("off/m2", packet.PriorityMedium)

	info, err := b.GetClientInfo("offline-sub")
	if err != nil {
		t.Fatal(err)
	}
	if info.Pending != [3]uint32{0, 1, 1} {
		t.Fatalf("pending=%v, want [0 1 1] (Low dropped, oldest Medium evicted)", info.Pending)
	}
	if info.MessagesDropped != 2 {
		t.Fatalf("MessagesDropped=%d, want 2 (1 Low skipped + 1 evicted)", info.MessagesDropped)
	}

	// 非 CleanSession 重连：复用会话并取回 High → Medium。
	sub2 := newTestClient(t, addr, "offline-sub")
	if !sub2.SessionPresent() {
		t.Fatal("session should be present on reconnect")
	}
	for _, want := range []string{"off/h1", "off/m2"} {
		msg, err := sub2.Pull(ctx, client.PullOptions{WaitMs: 300})
		if err != nil {
			t.Fatal(err)
		}
		if msg == nil || msg.Channel != want {
			t.Fatalf("offline queue = %+v, want %s", msg, want)
		}
		if err := msg.Ack(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

// CleanSession 必须丢弃既有 pending 并返回 SessionPresent=false。
func TestCleanSessionClearsState(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "clean-1", "c/**")
	pub := newTestClient(t, addr, "clean-pub")
	ctx := context.Background()

	_ = sub.Close()
	waitClientDetached(t, b, "clean-1")

	if err := pub.Push(ctx, "c/x", []byte("stale"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}

	clean := client.NewClientOptions().
		WithCore(addr).
		WithClientID("clean-1").
		WithCleanSession(true).
		WithAutoReconnect(false).
		WithLogger(zap.NewNop())
	c2, err := client.NewWithOptions(clean)
	if err != nil {
		t.Fatal(err)
	}
	if err := c2.ConnectContext(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c2.Close() }()

	if c2.SessionPresent() {
		t.Fatal("CleanSession should report SessionPresent=false")
	}
	msg, err := c2.Pull(ctx, client.PullOptions{Immediate: true})
	if err != nil {
		t.Fatal(err)
	}
	if msg != nil {
		t.Fatalf("CleanSession should clear pending, got %+v", msg)
	}
}

// CleanSession 在旧连接仍存活时同样生效（接管 + 清理）。
func TestCleanSessionTakeover(t *testing.T) {
	b, addr := startCore(t, nil)
	old := newTestClient(t, addr, "clean-2", "c/**")
	pub := newTestClient(t, addr, "clean-pub2")
	ctx := context.Background()

	if err := pub.Push(ctx, "c/1", []byte("stale"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}
	// 先确认旧会话确实收到了积压。
	if info, err := b.GetClientInfo("clean-2"); err != nil || info.Pending[1] != 1 {
		t.Fatalf("pending before clean = %+v, err=%v", info, err)
	}

	clean := client.NewClientOptions().
		WithCore(addr).
		WithClientID("clean-2").
		WithCleanSession(true).
		WithAutoReconnect(false).
		WithLogger(zap.NewNop())
	c2, err := client.NewWithOptions(clean)
	if err != nil {
		t.Fatal(err)
	}
	if err := c2.ConnectContext(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c2.Close() }()

	if c2.SessionPresent() {
		t.Fatal("CleanSession takeover should report SessionPresent=false")
	}
	msg, err := c2.Pull(ctx, client.PullOptions{Immediate: true})
	if err != nil {
		t.Fatal(err)
	}
	if msg != nil {
		t.Fatalf("CleanSession takeover should clear pending, got %+v", msg)
	}
	if old.Connected() {
		t.Fatal("old connection should be taken over")
	}
}

func waitClientDetached(t *testing.T, b *core.Core, clientID string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := b.GetClientInfo(clientID); err == nil && !info.Connected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("client %q did not detach in time", clientID)
}

// MaxClients 必须在握手阶段原子预占，并发建连不能超卖。
func TestMaxClientsNotOversold(t *testing.T) {
	_, addr := startCore(t, core.NewCoreOptions().WithMaxClients(1))

	const dials = 8
	results := make(chan error, dials)
	var wg sync.WaitGroup
	var keepMu sync.Mutex
	var keep []net.Conn
	t.Cleanup(func() {
		keepMu.Lock()
		defer keepMu.Unlock()
		for _, nc := range keep {
			_ = nc.Close()
		}
	})

	for i := 0; i < dials; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			nc, err := net.DialTimeout("tcp", addr, 3*time.Second)
			if err != nil {
				results <- err
				return
			}
			// 成功的连接保持打开：关闭会释放在线名额，让后续握手被接受。
			keepMu.Lock()
			keep = append(keep, nc)
			keepMu.Unlock()
			_ = nc.SetDeadline(time.Now().Add(5 * time.Second))

			connect := packet.NewConnectPacket(fmt.Sprintf("limit-%d", i))
			connect.KeepAlive = 60
			if err := packet.WritePacket(nc, connect, 0); err != nil {
				results <- err
				return
			}
			pkt, err := packet.ReadPacket(nc, 1<<20)
			if err != nil {
				results <- err
				return
			}
			connack := pkt.(*packet.ConnackPacket)
			if connack.ReasonCode == packet.ReasonSuccess {
				results <- nil
				return
			}
			results <- fmt.Errorf("rejected: %s", connack.ReasonCode)
		}(i)
	}
	wg.Wait()
	close(results)

	ok := 0
	for err := range results {
		if err == nil {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("MaxClients=1 but %d connections were accepted", ok)
	}
}

// 定向投递需要独立授权：未授予 AllowTargeted 的身份必须被拒绝。
func TestTargetedPublishRequiresPermission(t *testing.T) {
	var allow atomic.Bool
	hooks := &core.Hooks{
		Auth: core.AuthHandlerFunc(func(ctx *core.AuthContext) error {
			ctx.Identity = &core.Identity{
				ID:            ctx.ClientID,
				ReadChannels:  []string{"**"},
				WriteChannels: []string{"**"},
				AllowTargeted: allow.Load(),
			}
			return nil
		}),
	}
	_, addr := startCore(t, core.NewCoreOptions().WithHooks(hooks))

	sub := newTestClient(t, addr, "acl-sub", "cmd/**")
	pub := newTestClient(t, addr, "acl-pub")
	ctx := context.Background()

	err := pub.PushTo(ctx, "acl-sub", "cmd/x", []byte("v"), client.PushOptions{})
	var se *client.ServerError
	if !errors.As(err, &se) || se.Reason != packet.ReasonNotAuthorized {
		t.Fatalf("expected NotAuthorized for targeted publish, got %v", err)
	}

	allow.Store(true)
	// 身份在建连时确定，授权变化后必须重新建连。
	pubAllowed := newTestClient(t, addr, "acl-pub-2")
	if err := pubAllowed.PushTo(ctx, "acl-sub", "cmd/x", []byte("v"), client.PushOptions{}); err != nil {
		t.Fatalf("targeted publish should be allowed: %v", err)
	}
	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil || msg == nil {
		t.Fatalf("target should receive message, got %+v err=%v", msg, err)
	}
}

// 会话恢复时必须按本次身份重新校验保留的声明。
func TestSessionDeclarationsRevalidatedOnReconnect(t *testing.T) {
	var restricted atomic.Bool
	hooks := &core.Hooks{
		Auth: core.AuthHandlerFunc(func(ctx *core.AuthContext) error {
			read := []string{"a/**"}
			if restricted.Load() {
				read = []string{"b/**"}
			}
			ctx.Identity = &core.Identity{
				ID:            ctx.ClientID,
				ReadChannels:  read,
				WriteChannels: []string{"**"},
			}
			return nil
		}),
	}
	b, addr := startCore(t, core.NewCoreOptions().WithHooks(hooks))

	c := newTestClient(t, addr, "dev-1", "a/**")
	info, err := b.GetClientInfo("dev-1")
	if err != nil || len(info.Channels) != 1 {
		t.Fatalf("channels before reconnect = %+v err=%v", info, err)
	}
	_ = c.Close()
	waitClientDetached(t, b, "dev-1")

	restricted.Store(true)
	_ = newTestClient(t, addr, "dev-1")

	info, err = b.GetClientInfo("dev-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Channels) != 0 {
		t.Fatalf("declarations must be re-filtered by the new identity, got %v", info.Channels)
	}
}

// DECLARE 数量超限时整批拒绝，返回单条紧凑错误。
func TestDeclareOverLimitRejectedCompactly(t *testing.T) {
	_, addr := startCore(t, core.NewCoreOptions().WithMaxChannelsPerClient(2))
	c := newTestClient(t, addr, "dc-1")

	res, err := c.Declare(context.Background(), "a/1", "a/2", "a/3")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Reason != packet.ReasonQuotaExceeded {
		t.Fatalf("expected single QuotaExceeded result, got %+v", res)
	}
}
