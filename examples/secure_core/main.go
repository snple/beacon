// Package main 演示生产环境常见的加固配置：认证、TLS、配额与运维端点。
//
// 运行前准备证书（自签名即可用于本地验证）：
//
//	go run ./examples/secure_core -cert server.crt -key server.key
//
// 客户端连接时使用 tls:// 前缀；未显式配置 ServerName 时会从地址推导：
//
//	dialer := &client.TCPDialer{
//	    Address:   "tls://127.0.0.1:8883",
//	    TLSConfig: &tls.Config{RootCAs: pool},
//	}
package main

import (
	"crypto/subtle"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/snple/beacon/core"
	"go.uber.org/zap"
)

func main() {
	addr := flag.String("addr", ":8883", "TLS listen address")
	metricsAddr := flag.String("metrics-addr", ":9101", "Metrics/health listen address")
	certFile := flag.String("cert", "", "TLS certificate file (PEM)")
	keyFile := flag.String("key", "", "TLS private key file (PEM)")
	clientID := flag.String("client-id", "device-1", "Accepted client ID")
	secret := flag.String("secret", "s3cr3t", "Expected auth data (secret)")
	flag.Parse()

	if *certFile == "" || *keyFile == "" {
		log.Fatal("-cert and -key are required (generate a self-signed pair for testing)")
	}

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal(err)
	}

	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatalf("load key pair: %v", err)
	}

	opts := core.NewCoreOptions().
		WithLogger(logger).
		// 配额与背压：避免慢消费者/异常客户端把内存和磁盘写满
		WithMaxClientQueueSize(5000, core.QueueOverflowDropNewest).
		WithMaxPendingMessages(50000).
		WithMaxPublishRate(2000, 4000).
		WithMaxStoreBytes(4 << 30). // 4 GiB 磁盘水位
		WithRetainEnabled(true)

	// 认证：CONNECT 携带的 AuthData 必须与注册的 secret 一致
	opts.WithAuthHandler(&core.AuthHandlerFunc{
		ConnectFunc: func(ctx *core.AuthConnectContext) error {
			if ctx.ClientID != *clientID {
				return fmt.Errorf("unknown client: %s", ctx.ClientID)
			}
			if ctx.Packet.Properties == nil || len(ctx.Packet.Properties.AuthData) == 0 {
				return fmt.Errorf("auth data required")
			}
			if subtle.ConstantTimeCompare(ctx.Packet.Properties.AuthData, []byte(*secret)) != 1 {
				return fmt.Errorf("auth failed")
			}
			return nil
		},
	})

	c, err := core.NewWithOptions(opts)
	if err != nil {
		log.Fatalf("create core: %v", err)
	}
	if err := c.Start(); err != nil {
		log.Fatalf("start core: %v", err)
	}
	defer c.Stop()

	// 运维端点：/healthz 存活探针、/metrics Prometheus 指标、/stats JSON 快照
	go func() {
		if err := c.ServeMetrics(*metricsAddr); err != nil {
			logger.Error("metrics server", zap.Error(err))
		}
	}()

	go func() {
		if err := c.ServeTLS(*addr, &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}); err != nil {
			logger.Error("tls server", zap.Error(err))
			os.Exit(1)
		}
	}()

	logger.Info("secure core started",
		zap.String("addr", *addr),
		zap.String("metrics", *metricsAddr))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
