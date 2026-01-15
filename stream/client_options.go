package stream

import (
	"go.uber.org/zap"
)

// ClientOption configures a stream Client.
type ClientOption func(*Client)

func WithCoreAddr(addr string) ClientOption {
	return func(c *Client) {
		c.coreAddr = addr
	}
}

func WithClientID(clientID string) ClientOption {
	return func(c *Client) {
		c.clientID = clientID
	}
}

func WithAuth(method string, data []byte) ClientOption {
	return func(c *Client) {
		c.authMethod = method
		c.authData = data
	}
}

func WithLogger(logger *zap.Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}
