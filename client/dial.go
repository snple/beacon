package client

import (
	"context"
	"crypto/tls"
	"net"
	"time"
)

// Dialer 连接拨号器接口
// 用户可以实现此接口来自定义连接建立方式（WebSocket、QUIC、Unix Socket 等）
type Dialer interface {
	Dial() (net.Conn, error)
}

// TCPDialer TCP/TLS 拨号器
type TCPDialer struct {
	Address     string        // 服务器地址 (例如: "127.0.0.1:3883" 或 "tls://127.0.0.1:3884")
	TLSConfig   *tls.Config   // TLS 配置（可选）
	DialTimeout time.Duration // 连接超时（默认 10s）
}

// Dial 实现 Dialer 接口
func (d *TCPDialer) Dial() (net.Conn, error) {
	address := d.Address
	useTLS := false
	if len(address) > 6 && address[:6] == "tls://" {
		address = address[6:]
		useTLS = true
	}

	timeout := d.DialTimeout
	if timeout == 0 {
		timeout = defaultConnectTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var conn net.Conn
	var err error

	if useTLS || d.TLSConfig != nil {
		tlsConfig := d.TLSConfig
		if tlsConfig == nil {
			tlsConfig = &tls.Config{}
		}
		conn, err = (&tls.Dialer{Config: tlsConfig}).DialContext(ctx, "tcp", address)
	} else {
		conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", address)
	}

	if err != nil {
		return nil, NewConnectionError("connect", err)
	}

	return conn, nil
}
