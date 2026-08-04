package core

import (
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/snple/beacon/internal/wsconn"
	"go.uber.org/zap"
)

// upgrader 是 WebSocket 升级器。
// CheckOrigin 返回 true：beacon 协议自带认证（CONNECT/AUTH），
// 浏览器跨域限制由应用层自行决定是否启用。
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// ServeWS 启动 HTTP 服务并在指定路径上提供 WebSocket 接入。
//
//	path 示例："/ws"（为空则默认 "/ws"）
//
// 升级成功后，每个 WebSocket 连接会被包装为 net.Conn 交给
// HandleConn 处理，与 TCP/TLS 连接走完全相同的协议流程。
// 此方法会阻塞，直到 HTTP 服务关闭或 Core 停止。
func (c *Core) ServeWS(address, path string) error {
	if path == "" {
		path = "/ws"
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, c.handleWebSocket)

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Core 停止时关闭 HTTP 服务，让 ServeWS 返回
	go func() {
		<-c.ctx.Done()
		_ = srv.Close()
	}()

	c.logger.Info("Started WebSocket listener",
		zap.String("address", address),
		zap.String("path", path))

	err = srv.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// handleWebSocket 处理一次 WebSocket 升级请求
func (c *Core) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		c.logger.Debug("WebSocket upgrade failed",
			zap.String("remote", r.RemoteAddr),
			zap.Error(err))
		return
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
