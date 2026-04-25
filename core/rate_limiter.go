package core

import (
	"sync"
	"time"
)

// connRateLimiter 连接速率限制器（令牌桶算法）
type connRateLimiter struct {
	mu        sync.Mutex
	tokens    float64
	maxTokens float64
	rate      float64   // 每秒补充的令牌数
	lastCheck time.Time
}

// newConnRateLimiter 创建连接速率限制器
// rate: 每秒允许的连接数，burst: 突发容量
func newConnRateLimiter(rate float64, burst int) *connRateLimiter {
	maxTokens := float64(burst)
	if maxTokens < rate {
		maxTokens = rate
	}
	return &connRateLimiter{
		tokens:    maxTokens,
		maxTokens: maxTokens,
		rate:      rate,
		lastCheck: time.Now(),
	}
}

// Allow 检查是否允许新连接
func (r *connRateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastCheck).Seconds()
	r.lastCheck = now

	// 补充令牌
	r.tokens += elapsed * r.rate
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}

	if r.tokens < 1 {
		return false
	}

	r.tokens--
	return true
}
