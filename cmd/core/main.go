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
	addr := flag.String("addr", ":3883", "Listen address")
	maxClients := flag.Int("max-clients", 10000, "Maximum number of clients")
	authEnabled := flag.Bool("auth", false, "Enable authentication")
	authSecret := flag.String("auth-secret", "", "Authentication secret key")
	storageEnabled := flag.Bool("storage", true, "Enable BadgerDB persistence")
	storageDir := flag.String("storage-dir", "./data/messages", "BadgerDB storage directory")
	showVersion := flag.Bool("version", false, "Show version")
	debug := flag.Bool("debug", false, "Enable debug logging")

	flag.Parse()

	if *showVersion {
		fmt.Printf("Queen 0.1 core v%s\n", version)
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
		WithMaxClients(*maxClients).
		WithRetainEnabled(true).
		WithStorageEnabled(*storageEnabled).
		WithStorageDir(*storageDir).
		WithLogger(logger)

	// 如果启用认证
	if *authEnabled {
		secret := *authSecret
		opts.WithAuthHandler(&core.AuthHandlerFunc{
			// OnConnect: 处理连接时的认证
			ConnectFunc: func(ctx *core.AuthConnectContext) error {
				// 简单的 token 认证示例
				// 从 ConnectPacket.Properties 获取认证数据
				if ctx.Packet.Properties != nil && secret != "" && string(ctx.Packet.Properties.AuthData) != secret {
					return fmt.Errorf("invalid auth data")
				}
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
	fmt.Printf("║  Persistence:      %-40v ║\n", *storageEnabled)
	if *storageEnabled {
		fmt.Printf("║  Storage Dir:      %-40s ║\n", *storageDir)
	}
	fmt.Printf("║  Auth:             %-40v ║\n", *authEnabled)
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
