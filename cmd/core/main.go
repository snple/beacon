package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/snple/beacon/core"
	"go.uber.org/zap"
)

var (
	version = "0.1.0"
)

func main() {
	// 命令行参数
	addr := flag.String("addr", ":5208", "Listen address")
	maxClients := flag.Uint("max-clients", 10000, "Maximum number of clients")
	authEnabled := flag.Bool("auth", false, "Enable authentication")
	authSecret := flag.String("auth-secret", "", "Authentication secret key")
	storeDir := flag.String("store-dir", "./data", "BadgerDB storage directory")
	showVersion := flag.Bool("version", false, "Show version")
	debug := flag.Bool("debug", false, "Enable debug logging")

	flag.Parse()

	if *showVersion {
		fmt.Printf("Beacon 0.1 core v%s\n", version)
		os.Exit(0)
	}

	// 设置日志器
	var logger *zap.Logger
	var err error
	if *debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	// 构建 core 选项
	opts := core.NewCoreOptions().
		WithAddress(*addr).
		WithMaxClients(uint32(*maxClients)).
		WithRetainEnabled(true).
		WithStoreDir(*storeDir).
		WithLogger(logger)

	// 添加连接生命周期钩子
	opts.WithConnectionHandler(&core.ConnectionHandlerFunc{
		ConnectFunc: func(ctx *core.ConnectContext) {
			logger.Info("Client connected",
				zap.String("client_id", ctx.ClientID),
				zap.String("remote_addr", ctx.RemoteAddr),
				zap.Bool("session_present", ctx.SessionPresent),
				zap.Uint16("keep_alive", ctx.Packet.KeepAlive),
			)
		},
		DisconnectFunc: func(ctx *core.DisconnectContext) {
			logger.Info("Client disconnected",
				zap.String("client_id", ctx.ClientID),
				zap.Int64("duration_seconds", ctx.Duration),
			)
		},
	})

	// 添加消息处理钩子
	opts.WithMessageHandler(&core.MessageHandlerFunc{
		PublishFunc: func(ctx *core.PublishContext) error {
			logger.Debug("Message published",
				zap.String("client_id", ctx.ClientID),
				zap.String("topic", ctx.Packet.Topic),
				zap.Int("payload_size", len(ctx.Packet.Payload)),
				zap.Uint8("qos", uint8(ctx.Packet.QoS)),
			)
			return nil
		},
		DeliverFunc: func(ctx *core.DeliverContext) bool {
			logger.Debug("Message delivered",
				zap.String("client_id", ctx.ClientID),
				zap.String("topic", ctx.Packet.Topic),
				zap.Int("payload_size", len(ctx.Packet.Payload)),
			)
			return true
		},
	})

	// 添加订阅管理钩子
	opts.WithSubscriptionHandler(&core.SubscriptionHandlerFunc{
		SubscribeFunc: func(ctx *core.SubscribeContext) error {
			logger.Info("Client subscribed",
				zap.String("client_id", ctx.ClientID),
				zap.String("topic", ctx.Subscription.Topic),
			)
			return nil
		},
		UnsubscribeFunc: func(ctx *core.UnsubscribeContext) {
			logger.Info("Client unsubscribed",
				zap.String("client_id", ctx.ClientID),
				zap.String("topic", ctx.Topic),
			)
		},
	})

	// 如果启用认证
	if *authEnabled {
		secret := *authSecret
		opts.WithAuthHandler(&core.AuthHandlerFunc{
			// OnConnect: 处理连接时的认证
			ConnectFunc: func(ctx *core.AuthConnectContext) error {
				// 简单的 token 认证示例
				// 从 ConnectPacket.Properties 获取认证数据
				if ctx.Packet.Properties != nil && secret != "" && string(ctx.Packet.Properties.AuthData) != secret {
					logger.Warn("Authentication failed",
						zap.String("client_id", ctx.ClientID),
						zap.String("reason", "invalid auth data"),
					)
					return fmt.Errorf("invalid auth data")
				}
				logger.Info("Authentication successful",
					zap.String("client_id", ctx.ClientID),
				)
				return nil
			},
		})
	}

	// 创建并启动 core
	core, err := core.NewWithOptions(opts)
	if err != nil {
		logger.Error("Failed to create core", zap.Error(err))
		os.Exit(1)
	}

	if err := core.Start(); err != nil {
		logger.Error("Failed to start core", zap.Error(err))
		os.Exit(1)
	}

	// 打印启动信息
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║              Queen core                             ║")
	fmt.Println("╠════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Version:          %-40s ║\n", version)
	fmt.Printf("║  Address:          %-40s ║\n", *addr)
	fmt.Printf("║  Max Clients:      %-40d ║\n", *maxClients)
	fmt.Printf("║  Persistence:      %-40v ║\n", *storeDir != "")
	fmt.Printf("║  Storage Dir:      %-40s ║\n", *storeDir)
	fmt.Printf("║  Auth:             %-40v ║\n", *authEnabled)
	fmt.Printf("║  Debug:            %-40v ║\n", *debug)
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop the core...")

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down...")
	core.Stop()
	fmt.Println("core stopped.")
}
