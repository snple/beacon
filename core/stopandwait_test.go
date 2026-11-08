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

	// 第一次拉取 m1
	first, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if first == nil || string(first.Payload) != "m1" || first.Dup {
		t.Fatalf("first pull: %+v", first)
	}

	// 未确认时再拉：只重发 m1，不应出现 m2
	second, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if second == nil || !second.Dup || second.ID != first.ID {
		t.Fatalf("expected duplicate of m1, got %+v", second)
	}

	// 确认后才应拿到 m2
	if _, err := sub.CommitAll(ctx); err != nil {
		t.Fatal(err)
	}
	third, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if third == nil || string(third.Payload) != "m2" || third.Dup {
		t.Fatalf("third pull should be m2, got %+v", third)
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

	if _, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300}); err != nil {
		t.Fatal(err)
	}
	// Low 已删除，不需要 commit；第二次应拿到剩下那条而不是重复
	next, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.Dup || string(next.Payload) != "l2" {
		t.Fatalf("low priority should not be gated: %+v", next)
	}
}

// 客户端并发 Pull 会被 SDK 串行化，本地未确认集合不会互相覆盖。
func TestConcurrentPullSerializedByClient(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "cc-2", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	for _, v := range []string{"1", "2"} {
		if err := pub.Push(ctx, "ch/a", []byte(v), client.PushOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	results := make([]*client.Message, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 200})
			if err != nil {
				t.Errorf("pull: %v", err)
				return
			}
			results[i] = msg
		}(i)
	}
	wg.Wait()

	// 串行化后第二次拿到的仍是同一条未确认消息（dup）。
	if results[0] == nil || results[1] == nil {
		t.Fatalf("both pulls should return a message: %+v", results)
	}
	if results[0].ID != results[1].ID {
		t.Fatalf("concurrent pulls should observe the same unacked message")
	}
	if results[0].Dup == results[1].Dup {
		t.Fatalf("exactly one pull should be marked dup: %+v", results)
	}

	if n, err := sub.CommitAll(ctx); err != nil {
		t.Fatal(err)
	} else if n != 1 {
		t.Fatalf("committed %d, want 1 unique message", n)
	}

	next, err := sub.Pull(ctx, client.PullOptions{WaitMs: 200})
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.Dup {
		t.Fatalf("remaining message should be fresh, got %+v", next)
	}
}
