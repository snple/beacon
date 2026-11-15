package client

import (
	"context"
	"crypto/tls"
	"net"
	"strings"
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
	// Address 是服务端 WebSocket 地址，例如 "ws://127.0.0.1:8080/ws"。
	Address string
	// TLSConfig 是 wss:// 连接使用的 TLS 配置；nil 使用系统默认。
	TLSConfig *tls.Config
	// DialTimeout 是 ctx 未带截止时间时使用的超时（默认 10s）。
	DialTimeout time.Duration
	// MaxMessageSize 限制单个 WebSocket 消息大小（默认 16 MiB），
	// 防止恶意服务端用超大帧耗尽客户端内存。
	MaxMessageSize int64
}

// DialContext 实现 Dialer 接口。
func (d *WSDialer) DialContext(ctx context.Context) (net.Conn, error) {
	address := d.Address
	if address == "" {
		address = "ws://127.0.0.1:8080/ws"
	}
	if !hasWSScheme(address) {
		address = "ws://" + address
	}

	if _, ok := ctx.Deadline(); !ok {
		timeout := d.DialTimeout
		if timeout == 0 {
			timeout = defaultConnectTimeout
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	dialer := websocket.Dialer{
		TLSClientConfig: d.TLSConfig,
	}

	ws, _, err := dialer.DialContext(ctx, address, nil)
	if err != nil {
		return nil, NewConnectionError("websocket connect", err)
	}

	limit := d.MaxMessageSize
	if limit <= 0 {
		limit = 16 << 20
	}
	ws.SetReadLimit(limit)

	return wsconn.New(ws), nil
}

// hasWSScheme 检查地址是否带有 ws:// 或 wss:// 前缀
func hasWSScheme(address string) bool {
	return strings.HasPrefix(address, "ws://") || strings.HasPrefix(address, "wss://")
}
