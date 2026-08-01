package core

import (
	"testing"
	"time"
)

// TestConnRateLimiter_Burst 验证初始突发容量：初始令牌数 = max(burst, rate)
func TestConnRateLimiter_Burst(t *testing.T) {
	limiter := newConnRateLimiter(1, 5) // rate=1/s, burst=5
	for i := 0; i < 5; i++ {
		if !limiter.Allow() {
			t.Fatalf("expected allow at burst slot %d", i)
		}
	}
	if limiter.Allow() {
		t.Fatal("expected deny after burst consumed")
	}
}

// TestConnRateLimiter_MaxTokensAtLeastRate 验证令牌上限至少为 rate：
// rate=10 > burst=2 时初始应有 10 个令牌
func TestConnRateLimiter_MaxTokensAtLeastRate(t *testing.T) {
	limiter := newConnRateLimiter(10, 2)
	for i := 0; i < 10; i++ {
		if !limiter.Allow() {
			t.Fatalf("expected allow %d (initial tokens = max(burst, rate))", i)
		}
	}
	if limiter.Allow() {
		t.Fatal("expected deny after initial tokens consumed")
	}
}

// TestConnRateLimiter_Refill 验证令牌随时间补充（rate=5/s，等待 ~250ms 应至少补充 1 个）
func TestConnRateLimiter_Refill(t *testing.T) {
	limiter := newConnRateLimiter(5, 5) // 5/s → 200ms/令牌
	for i := 0; i < 5; i++ {
		if !limiter.Allow() {
			t.Fatal("expected initial burst allows")
		}
	}
	if limiter.Allow() {
		t.Fatal("expected deny after tokens exhausted")
	}

	// 等待约 250ms → 至少补充 1 个令牌
	time.Sleep(250 * time.Millisecond)

	allowed := 0
	for i := 0; i < 6; i++ {
		if limiter.Allow() {
			allowed++
		} else {
			break
		}
	}
	if allowed == 0 {
		t.Fatal("expected at least one refilled token after waiting")
	}
}

// TestConnRateLimiter_ZeroRate 验证 rate=0 时只允许突发，之后永久拒绝
func TestConnRateLimiter_ZeroRate(t *testing.T) {
	limiter := newConnRateLimiter(0, 3) // rate=0, burst=3
	for i := 0; i < 3; i++ {
		if !limiter.Allow() {
			t.Fatalf("expected allow at burst slot %d", i)
		}
	}
	if limiter.Allow() {
		t.Fatal("expected deny after burst consumed with zero rate")
	}
}
