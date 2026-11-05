package core_test

import (
	"context"
	"errors"
	"net"
	"strings"
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

func TestPushBatch(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/**")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	records := []client.PushRecord{
		{Channel: "ch/a", Payload: []byte("1")},
		{Channel: "ch/b", Payload: []byte("2")},
		{Channel: "other/c", Payload: []byte("3")},
	}
	if err := pub.PushBatch(ctx, records, client.PushOptions{}); err != nil {
		t.Fatal(err)
	}

	msgs, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 matching messages, got %d", len(msgs))
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
	msgs, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
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

	msgs, err := c2.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || string(msgs[0].Payload) != "queued" {
		t.Fatalf("offline queue not delivered: %+v", msgs)
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

	msgs, err := c.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || string(msgs[0].Payload) != "over ws" {
		t.Fatalf("ws delivery failed: %+v", msgs)
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

	msgs, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].Channel != "b/x" {
		t.Fatalf("declare did not replace: %+v", msgs)
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
	msgs, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1")
	}
	if err := msgs[0].Ack(ctx); err != nil {
		t.Fatal(err)
	}

	again, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("acked message should not be redelivered, got %d", len(again))
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

	msgs, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 1, WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || string(msgs[0].Payload) != "high" {
		t.Fatalf("expected high priority first, got %+v", msgs)
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

	msgs, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].Channel != "ch/ok" {
		t.Fatalf("OnDeliver filter failed: %+v", msgs)
	}
	if b.GetStats().MessagesDropped != 1 {
		t.Fatalf("dropped=%d", b.GetStats().MessagesDropped)
	}
	if _, err := sub.CommitAll(ctx); err != nil {
		t.Fatal(err)
	}

	// 被丢弃的消息不能留在队列里
	again, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("rejected message should be gone, got %d", len(again))
	}
}

// 回归：队列满丢弃新消息时不能重复释放引用，否则在途计数会变成负数。
func TestQueueFullDoesNotLeakInflight(t *testing.T) {
	b, addr := startCore(t, core.NewCoreOptions().WithQueueCap(1))
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	ctx := context.Background()

	res := b.Push("ch/a", []byte("high"), core.PushOptions{Priority: packet.PriorityHigh})
	if res.Matched != 1 {
		t.Fatalf("first push should match: %+v", res)
	}
	if got := b.GetStats().InFlightMessages; got != 1 {
		t.Fatalf("inflight after first push = %d, want 1", got)
	}

	// 队列已满且新消息优先级更低 → 新消息被丢弃
	res = b.Push("ch/a", []byte("low"), core.PushOptions{Priority: packet.PriorityLow})
	if res.Matched != 0 || res.Dropped != 1 {
		t.Fatalf("low-priority push should be dropped: %+v", res)
	}
	if got := b.GetStats().InFlightMessages; got != 1 {
		t.Fatalf("inflight after drop = %d, want 1 (no double release)", got)
	}
	if got := b.GetStats().InFlightBytes; got <= 0 {
		t.Fatalf("inflight bytes = %d", got)
	}

	msgs, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || string(msgs[0].Payload) != "high" {
		t.Fatalf("unexpected batch: %+v", msgs)
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

	msgs, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message")
	}
	if msgs[0].Priority != packet.PriorityMedium {
		t.Fatalf("default priority = %v, want medium", msgs[0].Priority)
	}
	if !msgs[0].NeedsCommit() {
		t.Fatal("medium priority must require commit")
	}
}

func TestAdminPushExplicitLow(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	ctx := context.Background()

	b.Push("ch/a", []byte("v"), core.PushOptions{}.WithPriority(packet.PriorityLow))

	msgs, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].NeedsCommit() {
		t.Fatalf("explicit low priority should not need commit: %+v", msgs)
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

	msgs, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || string(msgs[0].Payload) != "original" {
		t.Fatalf("payload not copied: %q", msgs[0].Payload)
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
	msgs, err := sub2.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].Channel != "off/h1" || msgs[1].Channel != "off/m2" {
		t.Fatalf("offline queue = %+v, want [off/h1 off/m2]", msgs)
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
	msgs, err := c2.Pull(ctx, client.PullOptions{MaxRecords: 10, Immediate: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("CleanSession should clear pending, got %+v", msgs)
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
	msgs, err := c2.Pull(ctx, client.PullOptions{MaxRecords: 10, Immediate: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("CleanSession takeover should clear pending, got %+v", msgs)
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
