package stream

import (
	"time"

	"go.uber.org/zap"
)

// CoreOption configures a stream Core.
type CoreOption func(*Core)

func WithAddress(address string) CoreOption {
	return func(b *Core) {
		b.address = address
	}
}

func WithAuthHandler(authHandler AuthHandler) CoreOption {
	return func(b *Core) {
		b.authHandler = authHandler
	}
}

func WithCoreLogger(logger *zap.Logger) CoreOption {
	return func(b *Core) {
		b.logger = logger
	}
}

// WithMaxWaitingConns 设置等待中的接收连接总数上限（默认 10000，0 表示不限制）。
// 超限的新等待连接会被拒绝并关闭。
func WithMaxWaitingConns(n int) CoreOption {
	return func(b *Core) {
		b.maxWaitingConns = n
	}
}

// WithWriteTimeout 设置控制包（CONNACK/PONG/STREAM_ACK 等）的单次写超时，
// 默认 10 秒。数据转发路径不受此限制（需要背压）。
func WithWriteTimeout(d time.Duration) CoreOption {
	return func(b *Core) {
		b.writeTimeout = d
	}
}
