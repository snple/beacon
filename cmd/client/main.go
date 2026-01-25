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

var (
	version = "0.1.0"
)

func main() {
	// 命令行参数
	core := flag.String("core", "localhost:5208", "Core address")
	clientID := flag.String("client-id", "", "Client ID (default: auto-generated)")
	mode := flag.String("mode", "pub", "Mode: pub (publish), sub (subscribe), req (request), poll (polling)")

	// 消息相关参数
	topic := flag.String("topic", "test/topic", "Topic for pub/sub mode")
	message := flag.String("message", "Hello, Beacon!", "Message payload for pub mode")
	action := flag.String("action", "echo", "Action name for req/poll mode")

	// 连接参数
	keepAlive := flag.Uint("keepalive", 60, "Keep alive interval (seconds)")
	sessionTimeout := flag.Uint("session-timeout", 0, "Session timeout (seconds)")

	// 其他参数
	count := flag.Int("count", 1, "Number of messages to publish/request")
	interval := flag.Duration("interval", 1*time.Second, "Interval between messages")
	showVersion := flag.Bool("version", false, "Show version")
	debug := flag.Bool("debug", false, "Enable debug logging")

	flag.Parse()

	if *showVersion {
		fmt.Printf("Beacon 0.1 client v%s\n", version)
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
	defer logger.Sync()

	// 打印启动信息
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    Beacon Client                               ║")
	fmt.Println("╠════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Version:          %-40s ║\n", version)
	fmt.Printf("║  Core:             %-40s ║\n", *core)
	fmt.Printf("║  Client ID:        %-40s ║\n", getClientID(*clientID))
	fmt.Printf("║  Mode:             %-40s ║\n", *mode)
	fmt.Printf("║  Keep Alive:       %-40d ║\n", *keepAlive)
	fmt.Printf("║  Session Timeout:  %-40d ║\n", *sessionTimeout)
	fmt.Printf("║  Debug:            %-40v ║\n", *debug)
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 创建客户端
	c, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(*core).
			WithClientID(*clientID).
			WithKeepAlive(uint16(*keepAlive)).
			WithSessionTimeout(uint32(*sessionTimeout)).
			WithLogger(logger).
			WithConnectionHandler(&client.ConnectionHandlerFunc{
				ConnectFunc: func(ctx *client.ConnectContext) {
					fmt.Printf("✓ Connected to core (ClientID: %s, SessionPresent: %v)\n",
						ctx.ClientID, ctx.SessionPresent)
				},
				DisconnectFunc: func(ctx *client.DisconnectContext) {
					if ctx.Err != nil {
						fmt.Printf("✗ Disconnected: %v\n", ctx.Err)
					} else {
						fmt.Println("✓ Disconnected")
					}
				},
			}),
	)
	if err != nil {
		logger.Fatal("Failed to create client", zap.Error(err))
	}
	defer c.Close()

	// 连接到 core
	if err := c.Connect(); err != nil {
		logger.Fatal("Failed to connect", zap.Error(err))
	}

	// 信号处理
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nReceived interrupt signal, shutting down...")
		cancel()
	}()

	// 根据模式执行不同操作
	switch strings.ToLower(*mode) {
	case "pub", "publish":
		runPublishMode(ctx, c, *topic, *message, *count, *interval)
	case "sub", "subscribe":
		runSubscribeMode(ctx, c, *topic)
	case "req", "request":
		runRequestMode(ctx, c, *action, *message, *count, *interval)
	case "poll", "polling":
		runPollingMode(ctx, c, *action)
	default:
		logger.Fatal("Unknown mode", zap.String("mode", *mode))
	}

	fmt.Println("\nShutting down...")
}

func getClientID(id string) string {
	if id == "" {
		return "auto-generated"
	}
	return id
}

func runPublishMode(ctx context.Context, c *client.Client, topic, message string, count int, interval time.Duration) {
	fmt.Printf("Publishing %d message(s) to topic '%s'...\n\n", count, topic)

	for i := 0; i < count; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg := fmt.Sprintf("%s #%d", message, i+1)
		if err := c.Publish(topic, []byte(msg), nil); err != nil {
			fmt.Printf("✗ Failed to publish message #%d: %v\n", i+1, err)
			continue
		}

		fmt.Printf("✓ Published message #%d: %s\n", i+1, msg)

		if i < count-1 && interval > 0 {
			time.Sleep(interval)
		}
	}

	fmt.Printf("\n✓ Successfully published %d message(s)\n", count)
}

func runSubscribeMode(ctx context.Context, c *client.Client, topic string) {
	fmt.Printf("Subscribing to topic '%s'...\n\n", topic)

	if err := c.Subscribe(topic); err != nil {
		fmt.Printf("✗ Failed to subscribe: %v\n", err)
		return
	}

	fmt.Println("✓ Subscribed, waiting for messages (Ctrl+C to exit)...")
	fmt.Println()

	msgCount := 0
	for {
		msg, err := c.PollMessage(ctx, 5*time.Second)
		if err != nil {
			if errors.Is(err, client.ErrPollTimeout) {
				continue
			}
			if ctx.Err() != nil {
				break
			}
			fmt.Printf("✗ Error polling message: %v\n", err)
			break
		}

		msgCount++
		fmt.Printf("✓ [%d] Received message on '%s': %s\n", msgCount, msg.Topic(), string(msg.Payload()))
	}

	fmt.Printf("\n✓ Received %d message(s)\n", msgCount)
}

func runRequestMode(ctx context.Context, c *client.Client, action, message string, count int, interval time.Duration) {
	fmt.Printf("Sending %d request(s) to action '%s'...\n\n", count, action)

	successCount := 0
	for i := 0; i < count; i++ {
		select {
		case <-ctx.Done():
			goto done
		default:
		}

		msg := fmt.Sprintf("%s #%d", message, i+1)
		reqPkt := &client.Request{
			Packet: &packet.RequestPacket{
				Action:  action,
				Payload: []byte(msg),
			},
		}

		reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		resp, err := c.Request(reqCtx, reqPkt)
		cancel()

		if err != nil {
			fmt.Printf("✗ Request #%d failed: %v\n", i+1, err)
			continue
		}

		fmt.Printf("✓ Request #%d succeeded: %s\n", i+1, string(resp.Payload()))
		successCount++

		if i < count-1 && interval > 0 {
			time.Sleep(interval)
		}
	}

done:
	fmt.Printf("\n✓ Completed %d/%d request(s)\n", successCount, count)
}

func runPollingMode(ctx context.Context, c *client.Client, action string) {
	fmt.Printf("Registering action '%s' and waiting for requests...\n\n", action)

	if err := c.Register(action); err != nil {
		fmt.Printf("✗ Failed to register action: %v\n", err)
		return
	}

	fmt.Println("✓ Registered, waiting for requests (Ctrl+C to exit)...")
	fmt.Println()

	reqCount := 0
	for {
		req, err := c.PollRequest(ctx, 5*time.Second)
		if err != nil {
			if err == client.ErrPollTimeout {
				continue
			}
			if ctx.Err() != nil {
				break
			}
			fmt.Printf("✗ Error polling request: %v\n", err)
			break
		}

		reqCount++
		fmt.Printf("✓ [%d] Received request on '%s': %s\n", reqCount, req.Packet.Action, string(req.Packet.Payload))

		// Echo 响应
		respPayload := fmt.Sprintf("Echo: %s", string(req.Packet.Payload))
		resp := &client.Response{
			Packet: &packet.ResponsePacket{
				RequestID:      req.Packet.RequestID,
				TargetClientID: req.Packet.SourceClientID,
				ReasonCode:     packet.ReasonSuccess,
				Payload:        []byte(respPayload),
			},
		}
		if err := req.Response(resp); err != nil {
			fmt.Printf("✗ Failed to send response: %v\n", err)
		} else {
			fmt.Printf("  → Sent response: %s\n", respPayload)
		}
	}

	fmt.Printf("\n✓ Processed %d request(s)\n", reqCount)
}
