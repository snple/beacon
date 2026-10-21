package client

import (
	"context"
	"crypto/tls"
	"net"
	"strings"
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
	if rest, ok := strings.CutPrefix(address, "tls://"); ok {
		address = rest
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
		tlsConfig := newTLSConfig(d.TLSConfig, address)
		conn, err = (&tls.Dialer{Config: tlsConfig}).DialContext(ctx, "tcp", address)
	} else {
		conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", address)
	}

	if err != nil {
		return nil, NewConnectionError("connect", err)
	}

	return conn, nil
}

// newTLSConfig 返回用于本次连接的 TLS 配置。
//
// base 为 nil 时使用安全默认值（最小 TLS 1.2），并在未显式配置 ServerName 时
// 从地址推导，避免因证书校验失败而被迫关闭校验。base 非 nil 时先克隆，
// 保证不会修改调用方共享的配置对象。
func newTLSConfig(base *tls.Config, address string) *tls.Config {
	var cfg *tls.Config
	if base == nil {
		cfg = &tls.Config{MinVersion: tls.VersionTLS12}
	} else {
		cfg = base.Clone()
	}

	if cfg.ServerName == "" && !cfg.InsecureSkipVerify {
		cfg.ServerName = serverNameFromAddress(address)
	}
	return cfg
}

// serverNameFromAddress 从 "host:port" 中提取用于 SNI/证书校验的主机名。
// IP 字面量（含 IPv6）与无法解析的地址返回空串，交由证书 IP SAN 校验。
func serverNameFromAddress(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	host = strings.Trim(host, "[]")
	if host == "" || net.ParseIP(host) != nil {
		return ""
	}
	return host
}
