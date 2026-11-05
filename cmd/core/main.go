// beacon-core 是 Beacon v4 消息服务端。
//
// v4 采用 pull 模型：客户端 PUSH 发布、PULL 拉取、COMMIT 确认；
// 服务端为每个客户端维护内存队列，除响应外不主动发送数据。
package main

import (
	"crypto/subtle"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

var version = "0.4.0"

// nodesTOML 认证文件格式：client_id → secret 映射。
type nodesTOML struct {
	Nodes []nodeEntry `toml:"nodes"`
}

type nodeEntry struct {
	Serial string `toml:"serial"`
	Secret string `toml:"secret"`
}

// authenticator 基于静态密钥表的认证器。
type authenticator struct {
	mu      sync.RWMutex
	secrets map[string]string
}

func loadNodes(path string) (map[string]string, error) {
	var cfg nodesTOML
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(cfg.Nodes))
	for _, n := range cfg.Nodes {
		out[n.Serial] = n.Secret
	}
	return out, nil
}

func (a *authenticator) Authenticate(ctx *core.AuthContext) error {
	a.mu.RLock()
	secret, ok := a.secrets[ctx.ClientID]
	a.mu.RUnlock()

	if !ok {
		return &core.AuthError{Reason: packet.ReasonNotAuthorized}
	}
	if subtle.ConstantTimeCompare([]byte(secret), ctx.Data) != 1 {
		return &core.AuthError{Reason: packet.ReasonBadAuthMethodOrAuthData}
	}

	ctx.Identity = &core.Identity{
		ID:   ctx.ClientID,
		Name: ctx.ClientID,
		// 已认证设备可以声明全部 channel；需要更细的隔离时按业务改这里。
		ReadChannels:  []string{"**"},
		WriteChannels: []string{"**"},
	}
	return nil
}

func main() {
	var (
		addr        = flag.String("addr", ":5208", "TCP listen address")
		wsAddr      = flag.String("ws-addr", "", "WebSocket listen address (empty to disable)")
		wsPath      = flag.String("ws-path", "/ws", "WebSocket endpoint path")
		metricsAddr = flag.String("metrics-addr", "", "metrics/health listen address (empty to disable)")
		authFile    = flag.String("auth-file", "", "TOML file with device secrets (empty to disable auth)")
		queueCap    = flag.Int("queue-cap", 10000, "per-client pending queue capacity (0 = unlimited)")
		maxRecords  = flag.Int("max-records", 1000, "max records per PULL")
		maxBytes    = flag.Int("max-bytes", 1<<20, "max bytes per PULL/PUSH")
		maxSessions = flag.Int("max-sessions", 10000, "max sessions including offline (-1 = unlimited)")
		maxInflight = flag.Int64("max-inflight-bytes", 256<<20, "global in-flight message bytes (-1 = unlimited)")
		sessTimeout = flag.Int("session-timeout", 86400,
			"session retention seconds after disconnect (0 = delete immediately)")
		maxChanLen = flag.Int("max-channel-length", packet.MinChannelLength,
			"max bytes for channel name/filter (256-1024)")
		debug       = flag.Bool("debug", false, "enable debug logging")
		showVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	logLevel := zap.NewProductionConfig()
	if *debug {
		logLevel.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}
	logger, err := logLevel.Build()
	if err != nil {
		fmt.Fprintln(os.Stderr, "init logger:", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

	if *maxRecords < 1 || *maxRecords > 65535 {
		logger.Fatal("max-records must be between 1 and 65535")
	}
	if *maxBytes < 1 || int64(*maxBytes) > int64(^uint32(0)) {
		logger.Fatal("max-bytes must be between 1 and 4294967295")
	}
	if *maxChanLen < packet.MinChannelLength || *maxChanLen > packet.MaxChannelLength {
		logger.Fatal("max-channel-length must be between 256 and 1024")
	}
	if *sessTimeout < 0 || uint64(*sessTimeout) > uint64(^uint32(0)) {
		logger.Fatal("session-timeout must be between 0 and 4294967295")
	}

	opts := core.NewCoreOptions().
		WithQueueCap(*queueCap).
		WithMaxRecords(uint16(*maxRecords)).
		WithMaxBytes(uint32(*maxBytes)).
		WithMaxSessions(*maxSessions).
		WithMaxInflightBytes(*maxInflight).
		WithSessionTimeout(uint32(*sessTimeout)).
		WithMaxChannelLength(*maxChanLen).
		WithLogger(logger)

	if *authFile != "" {
		secrets, err := loadNodes(*authFile)
		if err != nil {
			logger.Fatal("load auth file", zap.Error(err))
		}
		auth := &authenticator{secrets: secrets}
		opts.WithHooks(&core.Hooks{Auth: auth})
		logger.Info("authentication enabled", zap.Int("devices", len(secrets)))
	}

	b, err := core.NewWithOptions(opts)
	if err != nil {
		logger.Fatal("create core", zap.Error(err))
	}
	if err := b.Start(); err != nil {
		logger.Fatal("start core", zap.Error(err))
	}

	errCh := make(chan error, 3)

	go func() {
		logger.Info("TCP listener", zap.String("addr", *addr))
		errCh <- b.ServeTCP(*addr)
	}()
	if *wsAddr != "" {
		go func() {
			logger.Info("WebSocket listener", zap.String("addr", *wsAddr), zap.String("path", *wsPath))
			errCh <- b.ServeWS(*wsAddr, *wsPath)
		}()
	}
	if *metricsAddr != "" {
		go func() {
			logger.Info("metrics listener", zap.String("addr", *metricsAddr))
			errCh <- b.ServeMetrics(*metricsAddr)
		}()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("shutting down", zap.String("signal", sig.String()))
	case err := <-errCh:
		logger.Error("listener stopped", zap.Error(err))
	}

	if err := b.Stop(); err != nil {
		logger.Error("stop core", zap.Error(err))
	}
}
