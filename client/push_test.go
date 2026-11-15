package client

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/snple/beacon/packet"
)

// 明显的参数错误必须在本地拒绝，不产生网络往返。
func TestPushValidation(t *testing.T) {
	c, err := NewWithOptions(NewClientOptions().WithClientID("t").WithServer("127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if _, err := c.Push(ctx, "", nil, PushOptions{}); !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("empty channel: %v", err)
	}
	if _, err := c.Push(ctx, "a/b", nil, PushOptions{TTL: -time.Second}); !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("negative TTL: %v", err)
	}
	if _, err := c.Push(ctx, "a/b", nil, PushOptions{Priority: Priority(9)}); !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("invalid priority: %v", err)
	}
	if _, err := c.PushTo(ctx, "", "a/b", nil, PushOptions{}); !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("empty target: %v", err)
	}
	if _, err := c.Declare(ctx, Channel{Filter: "a/#"}); !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("invalid filter: %v", err)
	}
}

// 零值 Priority 按 Medium 编码，显式 Low 必须保留。
func TestNewRecordPriorityAndExpiry(t *testing.T) {
	rec := newRecord("a/b", "", []byte("x"), PushOptions{})
	if rec.Priority() != packet.PriorityMedium {
		t.Fatalf("default priority = %v", rec.Priority())
	}

	rec = newRecord("a/b", "", nil, PushOptions{Priority: PriorityLow})
	if rec.Priority() != packet.PriorityLow {
		t.Fatalf("explicit low priority = %v", rec.Priority())
	}

	now := time.Now().UnixMilli()
	rec = newRecord("a/b", "", nil, PushOptions{TTL: time.Hour})
	if !rec.HasExpiry() || rec.ExpiresAt <= now {
		t.Fatalf("TTL not applied: %+v", rec)
	}
	if rec.PublishedAt == 0 {
		t.Fatal("TTL should also record the publisher timestamp")
	}

	rec = newRecord("a/b", "", nil, PushOptions{ExpiresAt: 12345})
	if rec.ExpiresAt != 12345 {
		t.Fatalf("explicit expiry = %d", rec.ExpiresAt)
	}
}

// 空提交不产生请求，也不报错。
func TestCommitEmptyIsNoop(t *testing.T) {
	c, err := NewWithOptions(NewClientOptions().WithClientID("t").WithServer("127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Commit(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res != (CommitResult{}) {
		t.Fatalf("unexpected result: %+v", res)
	}
}
