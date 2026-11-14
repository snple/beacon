// core_api 演示进程内管理 API：发布、拉取、定向投递、客户端查询。
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewDevelopment()
	defer func() { _ = logger.Sync() }()

	b, err := core.NewWithOptions(core.NewCoreOptions().WithLogger(logger))
	if err != nil {
		log.Fatal(err)
	}
	if err := b.Start(); err != nil {
		log.Fatal(err)
	}
	defer func() { _ = b.Stop() }()

	go func() { _ = b.ServeTCP("127.0.0.1:15208") }()
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()

	worker, err := client.NewWithOptions(client.NewClientOptions().
		WithCore("127.0.0.1:15208").
		WithClientID("worker-1").
		WithChannels("jobs/**").
		WithLogger(logger))
	if err != nil {
		log.Fatal(err)
	}
	if err := worker.ConnectContext(ctx); err != nil {
		log.Fatal(err)
	}
	defer func() { _ = worker.Close() }()

	// 服务端推送（系统身份，不做写权限校验）
	res := b.Push("jobs/image", []byte("resize"),
		core.PushOptions{}.WithPriority(packet.PriorityHigh).WithReliable(true))
	fmt.Printf("published id=%s matched=%d dropped=%d\n", res.ID.Hex()[:8], res.Matched, res.Dropped)

	// 定向投递：跳过 channel 匹配，直接进入目标客户端的队列
	// （目标必须已声明匹配 jobs/direct 的过滤器）
	if _, err := b.PushToClient("worker-1", "jobs/direct", []byte("just for you"),
		core.PushOptions{}.WithReliable(true)); err != nil {
		log.Fatal(err)
	}

	// 客户端逐条拉取 + 确认
	for {
		m, err := worker.Pull(ctx, client.PullOptions{WaitMs: 1000})
		if err != nil {
			log.Fatal(err)
		}
		if m == nil {
			break
		}
		fmt.Printf("got %-12s %-16s priority=%s\n", m.Channel, string(m.Payload), m.Priority)
		if err := m.Ack(ctx); err != nil {
			log.Fatal(err)
		}
	}

	for _, s := range b.ListClients() {
		fmt.Printf("client %s online=%v pending=%v delivered=%d\n", s.ClientID, s.Connected, s.Pending, s.Delivered)
	}
	if info, err := b.GetClientInfo("worker-1"); err == nil {
		fmt.Printf("worker-1 channels=%v sent=%d\n", info.Channels, info.MessagesSent)
	}
}
