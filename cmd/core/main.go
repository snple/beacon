package main

import (
	"crypto/subtle"
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

// wsEnabledText 返回 WebSocket 监听信息（未启用时显示 "disabled"）
func wsEnabledText(wsAddr, wsPath string) string {
	if wsAddr == "" {
		return "disabled"
	}
	return wsAddr + wsPath
}

func main() {
	// 命令行参数
	addr := flag.String("addr", ":5208", "Listen address")
	maxClients := flag.Uint("max-clients", 10000, "Maximum number of clients")
	authFile := flag.String("auth-file", "", "Path to auth file (TOML with [[nodes]] serial + secret)")
	storeDir := flag.String("store-dir", "./data", "BadgerDB storage directory")
	wsAddr := flag.String("ws-addr", "", "WebSocket listen address (empty = disabled, e.g. \":8080\")")
	wsPath := flag.String("ws-path", "/ws", "WebSocket endpoint path")
	metricsAddr := flag.String("metrics-addr", "", "Metrics/health HTTP listen address (empty = disabled, e.g. \":9101\")")
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
	defer func() { _ = logger.Sync() }()

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
				// 使用恒定时间比较，避免时序侧信道泄露 secret
				if !ok || subtle.ConstantTimeCompare([]byte(secret), ctx.Packet.Properties.AuthData) != 1 {
					am.logger.Warn("Auth failed", zap.String("client_id", ctx.ClientID))
					return fmt.Errorf("auth failed: %s", ctx.ClientID)
				}
				return nil
			},
		})
	}

	// 创建并启动 broker（变量名避免遮蔽 core 包）
	broker, err := core.NewWithOptions(opts)
	if err != nil {
		logger.Error("Failed to create core", zap.Error(err))
		os.Exit(1)
	}

	if err := broker.Start(); err != nil {
		logger.Error("Failed to start core", zap.Error(err))
		os.Exit(1)
	}

	// 监听器错误统一送入 errCh，由主协程优雅停机，避免 goroutine 内 os.Exit 跳过清理。
	errCh := make(chan error, 3)

	go func() {
		logger.Info("Starting TCP server", zap.String("address", *addr))
		errCh <- broker.ServeTCP(*addr)
	}()

	if *wsAddr != "" {
		go func() {
			logger.Info("Starting WebSocket server", zap.String("address", *wsAddr), zap.String("path", *wsPath))
			errCh <- broker.ServeWS(*wsAddr, *wsPath)
		}()
	}

	if *metricsAddr != "" {
		go func() {
			logger.Info("Starting metrics server", zap.String("address", *metricsAddr))
			errCh <- broker.ServeMetrics(*metricsAddr)
		}()
	}

	printBanner(version, *addr, wsEnabledText(*wsAddr, *wsPath),
		wsEnabledText(*metricsAddr, "/metrics"), wsEnabledText(*metricsAddr, "/healthz"),
		*maxClients, *storeDir, *authFile, *debug)

	// 等待中断信号或监听器退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		fmt.Println("\nShutting down...")
	case err := <-errCh:
		logger.Error("Server stopped, shutting down", zap.Error(err))
	}

	_ = broker.Stop()
	fmt.Println("core stopped.")
}

// printBanner 输出对齐的启动信息（内部宽度 64）。
func printBanner(version, addr, ws, metrics, health string, maxClients uint, storeDir, authFile string, debug bool) {
	row := func(label, value string) {
		fmt.Printf("║  %-22s%-40s║\n", label, value)
	}

	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	row("Beacon core", "")
	fmt.Println("╠════════════════════════════════════════════════════════════════╣")
	row("Version:", version)
	row("Address:", addr)
	row("WebSocket:", ws)
	row("Metrics:", metrics)
	row("Health:", health)
	row("Max Clients:", fmt.Sprintf("%d", maxClients))
	row("Persistence:", fmt.Sprintf("%v", storeDir != ""))
	row("Storage Dir:", storeDir)
	row("Auth File:", authFile)
	row("Debug:", fmt.Sprintf("%v", debug))
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop the core...")
}
