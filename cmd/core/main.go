package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/snple/beacon/core"
	"go.uber.org/zap"
)

var (
	version = "0.1.0"
)

// authMap 线程安全的认证表。
type authMap struct {
	nodes  map[string]string
	logger *zap.Logger
}

// nodesTOML 认证文件格式：client_id → secret 映射。
type nodesTOML struct {
	Nodes []nodeEntry `toml:"nodes"`
}

type nodeEntry struct {
	Serial string `toml:"serial"`
	Secret string `toml:"secret"`
}

// loadNodes 加载认证文件，返回 serial → secret 映射。
func loadNodes(path string) (map[string]string, error) {
	var cfg nodesTOML
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	nodes := make(map[string]string, len(cfg.Nodes))
	for _, n := range cfg.Nodes {
		if n.Serial == "" || n.Secret == "" {
			continue
		}
		nodes[n.Serial] = n.Secret
	}
	return nodes, nil
}

func main() {
	// 命令行参数
	addr := flag.String("addr", ":5208", "Listen address")
	maxClients := flag.Uint("max-clients", 10000, "Maximum number of clients")
	authFile := flag.String("auth-file", "", "Path to auth file (TOML with [[nodes]] serial + secret)")
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

	// 认证（通过 auth-file 指定的 TOML 文件，支持 SIGHUP 热重载）
	var authNodes atomic.Pointer[authMap]
	if *authFile != "" {
		nodes, err := loadNodes(*authFile)
		if err != nil {
			logger.Fatal("Failed to load auth file", zap.Error(err))
		}
		authNodes.Store(&authMap{nodes: nodes, logger: logger})
		logger.Info("Authentication enabled", zap.String("file", *authFile), zap.Int("nodes", len(nodes)))

		// SIGHUP 热重载
		go func() {
			ch := make(chan os.Signal, 1)
			signal.Notify(ch, syscall.SIGHUP)
			for range ch {
				nodes, err := loadNodes(*authFile)
				if err != nil {
					logger.Error("Failed to reload auth file", zap.Error(err))
					continue
				}
				authNodes.Store(&authMap{nodes: nodes, logger: logger})
				logger.Info("Auth file reloaded", zap.Int("nodes", len(nodes)))
			}
		}()

		opts.WithAuthHandler(&core.AuthHandlerFunc{
			ConnectFunc: func(ctx *core.AuthConnectContext) error {
				if ctx.Packet.Properties == nil || len(ctx.Packet.Properties.AuthData) == 0 {
					return fmt.Errorf("auth required")
				}
				am := authNodes.Load()
				secret, ok := am.nodes[ctx.ClientID]
				if !ok || secret != string(ctx.Packet.Properties.AuthData) {
					am.logger.Warn("Auth failed", zap.String("client_id", ctx.ClientID))
					return fmt.Errorf("auth failed: %s", ctx.ClientID)
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

	// 在后台启动TCP服务
	go func() {
		logger.Info("Starting TCP server", zap.String("address", *addr))
		if err := core.ServeTCP(*addr); err != nil {
			logger.Error("TCP server error", zap.Error(err))
			os.Exit(1)
		}
	}()

	// 打印启动信息
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║              Beacon core                             ║")
	fmt.Println("╠════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Version:          %-40s ║\n", version)
	fmt.Printf("║  Address:          %-40s ║\n", *addr)
	fmt.Printf("║  Max Clients:      %-40d ║\n", *maxClients)
	fmt.Printf("║  Persistence:      %-40v ║\n", *storeDir != "")
	fmt.Printf("║  Storage Dir:      %-40s ║\n", *storeDir)
	fmt.Printf("║  Auth File:       %-40s ║\n", *authFile)
	fmt.Printf("║  Debug:           %-40v ║\n", *debug)
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
