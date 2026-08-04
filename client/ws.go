package client

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/gorilla/websocket"
	"github.com/snple/beacon/internal/wsconn"
)

// WSDialer WebSocket 拨号器，实现 Dialer 接口。
//
// 用法：
//
//	wsDialer := &WSDialer{Address: "ws://127.0.0.1:8080/ws"}
//	c.ConnectWithDialer(wsDialer)
//
// Address 支持 ws:// 与 wss:// 两种 scheme，路径默认为 "/ws"。
type WSDialer struct {
	Address     string        // 服务器 WebSocket 地址 (例如 "ws://127.0.0.1:8080/ws")
	TLSConfig   *tls.Config   // wss:// 时的 TLS 配置（可选）
	DialTimeout time.Duration // 连接超时（默认 10s）
}

// Dial 建立 WebSocket 连接并包装为 net.Conn
func (d *WSDialer) Dial() (net.Conn, error) {
	address := d.Address
	if address == "" {
		address = "ws://127.0.0.1:8080/ws"
	}
	if !hasWSScheme(address) {
		address = "ws://" + address
	}

	timeout := d.DialTimeout
	if timeout == 0 {
		timeout = defaultConnectTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	dialer := websocket.Dialer{
		HandshakeTimeout: timeout,
		TLSClientConfig:  d.TLSConfig,
	}

	ws, _, err := dialer.DialContext(ctx, address, nil)
	if err != nil {
		return nil, NewConnectionError("websocket connect", err)
	}

	return wsconn.New(ws), nil
}

// hasWSScheme 检查地址是否带有 ws:// 或 wss:// 前缀
func hasWSScheme(address string) bool {
	return len(address) >= 3 && (address[:3] == "ws:" || address[:4] == "wss:")
}
