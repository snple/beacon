package core_test

import (
	"context"
	"sync"
	"testing"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
)

// 验证「delivered 非空时不返回新消息」这个硬门。
func TestStopAndWaitGate(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	for _, v := range []string{"m1", "m2", "m3"} {
		if err := pub.Push(ctx, "ch/a", []byte(v), client.PushOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	// 第一次拉 2 条（队列里还有 1 条未投递）
	first, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 2, WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 {
		t.Fatalf("first pull: %d", len(first))
	}
	if first[0].Dup || first[1].Dup {
		t.Fatal("first pull should not be marked dup")
	}

	// 第二次拉：delivered 有 2 条未确认 → 只重发这 2 条，不应出现 m3
	second, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 2, WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 2 {
		t.Fatalf("second pull: %d", len(second))
	}
	for _, m := range second {
		if !m.Dup {
			t.Fatalf("second pull should be all dup, got %+v", m)
		}
		if string(m.Payload) == "m3" {
			t.Fatal("stop-and-wait violated: new message leaked while delivered non-empty")
		}
	}

	// 确认后第三次拉，才应拿到 m3
	if _, err := sub.CommitAll(ctx); err != nil {
		t.Fatal(err)
	}
	third, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 2, WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if len(third) != 1 || string(third[0].Payload) != "m3" || third[0].Dup {
		t.Fatalf("third pull should be [m3], got %+v", third)
	}
}

// 部分确认：commit 掉 delivered 的一部分后，拉取应继续重发剩下的，而不是取新消息。
func TestStopAndWaitPartialCommit(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-2", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	for _, v := range []string{"a", "b", "c"} {
		if err := pub.Push(ctx, "ch/a", []byte(v), client.PushOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	first, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 3, WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 3 {
		t.Fatalf("expected 3, got %d", len(first))
	}

	// 只确认第一条
	if err := first[0].Ack(ctx); err != nil {
		t.Fatal(err)
	}

	next, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 3, WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 2 {
		t.Fatalf("expected remaining 2 unacked, got %d: %+v", len(next), next)
	}
	for _, m := range next {
		if !m.Dup {
			t.Fatalf("partial commit should yield dups only: %+v", m)
		}
	}
}

// 低优先级消息不进入 delivered，因此不会阻塞新消息。
func TestLowPriorityDoesNotBlock(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-3", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	low := client.PushOptions{}.WithPriority(packet.PriorityLow)
	if err := pub.Push(ctx, "ch/a", []byte("l1"), low); err != nil {
		t.Fatal(err)
	}
	if err := pub.Push(ctx, "ch/a", []byte("l2"), low); err != nil {
		t.Fatal(err)
	}

	if _, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 1, WaitMs: 300}); err != nil {
		t.Fatal(err)
	}
	// Low 已删除，不需要 commit；第二次应拿到剩下那条而不是重复
	next, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 1, WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 1 || next[0].Dup || string(next[0].Payload) != "l2" {
		t.Fatalf("low priority should not be gated: %+v", next)
	}
}

// 客户端并发 Pull 会被 SDK 串行化，本地未确认集合不会互相覆盖。
func TestConcurrentPullSerializedByClient(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "cc-2", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	for _, v := range []string{"1", "2", "3", "4"} {
		if err := pub.Push(ctx, "ch/a", []byte(v), client.PushOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	var got int
	var mu sync.Mutex
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			msgs, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 2, WaitMs: 200})
			if err != nil {
				t.Errorf("pull: %v", err)
				return
			}
			mu.Lock()
			got += len(msgs)
			mu.Unlock()
		}()
	}
	wg.Wait()

	// 串行化后第一次拿到 2 条，第二次仍是这 2 条的重复（因为没 commit）
	if got != 4 {
		t.Fatalf("expected 4 records total (2 + 2 dups), got %d", got)
	}

	// 关键：CommitAll 必须把 4 条都确认掉（而不是只确认最后一批）
	n, err := sub.CommitAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("committed %d, want 2 unique messages", n)
	}

	msgs, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 4, WaitMs: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("remaining should be 2, got %d", len(msgs))
	}
	for _, m := range msgs {
		if m.Dup {
			t.Fatalf("remaining messages should be fresh, got dup: %+v", m)
		}
	}
}
