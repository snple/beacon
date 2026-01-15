package stream

import (
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
