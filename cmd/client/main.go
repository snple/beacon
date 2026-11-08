// beacon-client 是 Beacon v4 的命令行客户端，用于调试 push/pull。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

var version = "0.4.0"

func main() {
	var (
		address   = flag.String("core", "127.0.0.1:5208", "Core address (host:port, ws:// or wss://)")
		clientID  = flag.String("client-id", "", "Client ID (default: auto)")
		mode      = flag.String("mode", "pull", "Mode: push | pull")
		channel   = flag.String("channel", "test/channel", "Channel name")
		message   = flag.String("message", "Hello, Beacon!", "Payload for push mode")
		count     = flag.Int("count", 1, "Number of messages to push")
		priority  = flag.String("priority", "medium", "Priority: low | medium | high")
		target    = flag.String("target", "", "Target client ID for directed push")
		channels  = flag.String("channels", "", "Comma separated channel filters to declare")
		interval  = flag.Duration("interval", 0, "Push interval")
		waitMs    = flag.Int("wait-ms", 30000, "Pull long-poll wait in ms (0 = immediate)")
		stdout    = flag.Bool("stdout", true, "Print pulled payloads")
		cleanSess = flag.Bool("clean-session", false, "Discard existing session state on connect")
		debug     = flag.Bool("debug", false, "Enable debug logging")
		showVer   = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	if *showVer {
		fmt.Println(version)
		return
	}

	prio, err := parsePriority(*priority)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	cfg := zap.NewProductionConfig()
	if *debug {
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}
	logger, _ := cfg.Build()
	defer func() { _ = logger.Sync() }()

	opts := client.NewClientOptions().
		WithCore(*address).
		WithClientID(*clientID).
		WithCleanSession(*cleanSess).
		WithLogger(logger)

	if *channels != "" {
		opts.WithChannels(splitList(*channels)...)
	} else if *mode == "pull" {
		opts.WithChannels(*channel)
	}

	c, err := client.NewWithOptions(opts)
	if err != nil {
		logger.Fatal("create client", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := c.ConnectContext(ctx); err != nil {
		logger.Fatal("connect", zap.Error(err))
	}
	logger.Info("connected",
		zap.String("clientID", c.ClientID()),
		zap.Bool("sessionPresent", c.SessionPresent()))
	defer func() { _ = c.Close() }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	switch *mode {
	case "push":
		runPush(ctx, c, *channel, *message, *count, prio, *target, *interval)
	case "pull":
		runPull(ctx, c, *waitMs, *stdout)
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", *mode)
		os.Exit(2)
	}
}

func runPush(ctx context.Context, c *client.Client, channel, message string, count int, prio packet.Priority, target string, interval time.Duration) {
	opts := client.PushOptions{Target: target}.WithPriority(prio)

	for i := 0; i < count; i++ {
		if err := c.Push(ctx, channel, []byte(message), opts); err != nil {
			fmt.Fprintf(os.Stderr, "push failed: %v\n", err)
			return
		}
		fmt.Printf("pushed to %s (priority=%s)\n", channel, prio)
		if interval > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
		}
	}
}

func runPull(ctx context.Context, c *client.Client, waitMs int, print bool) {
	for {
		m, err := c.Pull(ctx, client.PullOptions{
			WaitMs:    uint32(waitMs),
			Immediate: waitMs == 0,
		})
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, client.ErrClientClosed) {
				return
			}
			fmt.Fprintf(os.Stderr, "pull failed: %v\n", err)
			return
		}

		if m != nil {
			if print {
				fmt.Printf("[%s] %s: %s (dup=%v priority=%s)\n",
					m.ID.Hex()[:8], m.Channel, string(m.Payload), m.Dup, m.Priority)
			}
			if err := m.Ack(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "commit failed: %v\n", err)
				return
			}
			continue
		}

		if waitMs == 0 {
			// 非阻塞拉取：避免空转
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
}

func parsePriority(s string) (packet.Priority, error) {
	switch strings.ToLower(s) {
	case "low":
		return packet.PriorityLow, nil
	case "medium", "med", "":
		return packet.PriorityMedium, nil
	case "high":
		return packet.PriorityHigh, nil
	default:
		return 0, fmt.Errorf("invalid priority %q", s)
	}
}

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
