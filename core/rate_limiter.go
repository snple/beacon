package core

import (
	"sync"
	"time"
)

// tokenBucket 通用令牌桶限流器（连接速率 / 发布速率共用）
type tokenBucket struct {
	mu        sync.Mutex
	tokens    float64
	maxTokens float64
	rate      float64 // 每秒补充的令牌数
	lastCheck time.Time
}

// newTokenBucket 创建令牌桶限流器
// rate: 每秒允许的事件数，burst: 突发容量（小于 rate 时取 rate）
func newTokenBucket(rate float64, burst int) *tokenBucket {
	maxTokens := float64(burst)
	if maxTokens < rate {
		maxTokens = rate
	}
	return &tokenBucket{
		tokens:    maxTokens,
		maxTokens: maxTokens,
		rate:      rate,
		lastCheck: time.Now(),
	}
}

// Allow 尝试消耗一个令牌，返回是否允许
func (r *tokenBucket) Allow() bool {
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
