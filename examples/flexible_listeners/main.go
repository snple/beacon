package main

import (
	"crypto/tls"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/snple/beacon/core"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewDevelopment()

	// 创建 Core（不再需要配置监听地址）
	c, err := core.NewWithOptions(
		core.NewCoreOptions().
			WithMaxClients(1000).
			WithLogger(logger),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 启动 Core
	if err := c.Start(); err != nil {
		log.Fatal(err)
	}

	// 方式 1: 使用内置的 ServeTCP（最简单）
	go func() {
		if err := c.ServeTCP(":3883"); err != nil {
			logger.Error("TCP server error", zap.Error(err))
		}
	}()

	// 方式 2: 使用内置的 ServeTLS
	go func() {
		cert, _ := tls.LoadX509KeyPair("cert.pem", "key.pem")
		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
		if err := c.ServeTLS(":8883", tlsConfig); err != nil {
			logger.Error("TLS server error", zap.Error(err))
		}
	}()

	// 方式 3: 自定义 listener，完全控制
	go func() {
		listener, err := net.Listen("tcp", ":9883")
		if err != nil {
			logger.Error("Failed to create listener", zap.Error(err))
			return
		}
		defer listener.Close()

		logger.Info("Custom listener started on :9883")

		// 使用 Serve 方法
		if err := c.Serve(listener); err != nil {
			logger.Error("Serve error", zap.Error(err))
		}
	}()

	// 方式 4: 完全手动控制 accept 循环
	go func() {
		listener, err := net.Listen("tcp", ":10883")
		if err != nil {
			logger.Error("Failed to create listener", zap.Error(err))
			return
		}
		defer listener.Close()

		logger.Info("Manual listener started on :10883")

		for {
			conn, err := listener.Accept()
			if err != nil {
				logger.Error("Accept error", zap.Error(err))
				break
			}

			// 直接调用 HandleConn
			if err := c.HandleConn(conn); err != nil {
				logger.Warn("HandleConn error", zap.Error(err))
			}
		}
	}()

	logger.Info("All servers started")

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down...")
	c.Stop()
}
