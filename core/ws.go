package core

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/snple/beacon/internal/wsconn"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

// defaultWSCheckOrigin 只允许同源浏览器连接（无 Origin 的非浏览器客户端放行）。
// 需要跨域时显式配置 WithWSCheckOrigin。
func defaultWSCheckOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

// ServeWS 启动 HTTP 服务并在指定路径上提供 WebSocket 接入。
//
//	path 示例："/ws"（为空则默认 "/ws"）
//
// 升级成功后，每个 WebSocket 连接会被包装为 net.Conn 交给
// HandleConn 处理，与 TCP/TLS 连接走完全相同的协议流程。
// 此方法会阻塞，直到 HTTP 服务关闭或 Core 停止。
func (c *Core) ServeWS(address, path string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	return c.ServeWSListener(listener, path)
}

// ServeWSListener 在已有 listener 上提供 WebSocket 接入（阻塞，直到 Core 停止或服务关闭）。
func (c *Core) ServeWSListener(listener net.Listener, path string) error {
	if path == "" {
		path = "/ws"
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, c.handleWebSocket)

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Core 停止时关闭 HTTP 服务，让 ServeWSListener 返回。
	// stopClose 保证服务因其它原因退出时该协程也会结束，不留下孤儿 goroutine。
	stopClose := make(chan struct{})
	defer close(stopClose)

	go func() {
		select {
		case <-c.ctx.Done():
			_ = srv.Close()
		case <-stopClose:
		}
	}()

	id := c.registerListener(listener.Addr().String(), "ws")
	defer c.unregisterListener(id)

	c.logger.Info("Started WebSocket listener",
		zap.String("address", listener.Addr().String()),
		zap.String("path", path))

	err := srv.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

// handleWebSocket 处理一次 WebSocket 升级请求
func (c *Core) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	u := upgrader
	u.CheckOrigin = c.options.WSCheckOrigin
	if u.CheckOrigin == nil {
		u.CheckOrigin = defaultWSCheckOrigin
	}
	ws, err := u.Upgrade(w, r, nil)
	if err != nil {
		c.logger.Debug("WebSocket upgrade failed",
			zap.String("remote", r.RemoteAddr),
			zap.Error(err))
		return
	}

	// 在升级之前限制单个 WebSocket 消息的大小。
	// gorilla 默认不限制帧大小，而 wsconn 会先把整条消息读入内存，
	// 之后才轮到 beacon 的 MaxPacketSize 校验——不设限等于绕过包大小限制。
	if limit := c.options.MaxPacketSize; limit > 0 {
		ws.SetReadLimit(int64(limit))
	}

	// 复用 core 的限流与连接处理（HandleConn 内部自动检查
	// 运行状态、客户端数量与连接速率限制）
	if err := c.HandleConn(wsconn.New(ws)); err != nil {
		c.logger.Debug("WebSocket connection rejected",
			zap.String("remote", ws.RemoteAddr().String()),
			zap.Error(err))
		_ = ws.Close()
		return
	}
}
