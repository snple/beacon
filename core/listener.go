package core

import (
	"crypto/tls"
	"errors"
	"net"
	"time"

	"go.uber.org/zap"
)

// ServeTCP 启动 TCP 监听器并处理连接
// 这是一个辅助函数，用于快速启动标准的 TCP 服务
func (c *Core) ServeTCP(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()

	c.logger.Info("Started TCP listener",
		zap.String("address", listener.Addr().String()))

	return c.serve(listener)
}

// ServeTLS 启动 TLS 监听器并处理连接
// 这是一个辅助函数，用于快速启动标准的 TLS 服务
func (c *Core) ServeTLS(address string, tlsConfig *tls.Config) error {
	listener, err := tls.Listen("tcp", address, tlsConfig)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()

	c.logger.Info("Started TLS listener",
		zap.String("address", listener.Addr().String()))

	return c.serve(listener)
}

// Serve 从给定的 net.Listener 接受连接并处理
// 用户可以创建任何类型的 listener（TCP、Unix Socket、WebSocket、QUIC 等）
// 此方法会阻塞，直到 listener 关闭或 Core 停止
func (c *Core) Serve(listener net.Listener) error {
	c.logger.Info("Started custom listener",
		zap.String("address", listener.Addr().String()))
	return c.serve(listener)
}

func (c *Core) serve(listener net.Listener) error {
	stopClose := make(chan struct{})
	defer close(stopClose)

	go func() {
		select {
		case <-c.ctx.Done():
			_ = listener.Close()
		case <-stopClose:
		}
	}()

	for c.running.Load() {
		conn, err := listener.Accept()
		if err != nil {
			if !c.running.Load() {
				// Core 已停止，正常退出
				return nil
			}
			// 检查是否是因为 listener 被关闭导致的错误
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			// 记录错误但继续接受连接。短暂退避，避免 EMFILE 等持续性
			// 错误导致忙循环刷日志、占满 CPU。
			c.logger.Error("Accept error", zap.Error(err))
			select {
			case <-c.ctx.Done():
				return nil
			case <-time.After(100 * time.Millisecond):
			}
			continue
		}

		// HandleConn 会自动检查客户端数量限制
		if err := c.HandleConn(conn); err != nil {
			c.logger.Debug("Failed to handle connection", zap.Error(err))
		}
	}
	return nil
}
