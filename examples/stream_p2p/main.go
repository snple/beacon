// stream_p2p 演示两个客户端之间通过 core 中继建立点对点字节流。
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/core"
	"go.uber.org/zap"
)

const (
	mainAddr   = "127.0.0.1:15208"
	streamAddr = "127.0.0.1:15209"
)

func main() {
	logger, _ := zap.NewDevelopment()
	defer func() { _ = logger.Sync() }()

	b, err := core.NewWithOptions(core.NewCoreOptions().
		WithStreamEnabled(true). // 默认关闭；演示中显式开启
		WithLogger(logger))
	if err != nil {
		log.Fatal(err)
	}
	if err := b.Start(); err != nil {
		log.Fatal(err)
	}
	defer func() { _ = b.Stop() }()

	go func() { _ = b.ServeTCP(mainAddr) }()
	go func() { _ = b.ServeStreamTCP(streamAddr) }()
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()

	newClient := func(id string) *client.Client {
		c, err := client.NewWithOptions(client.NewClientOptions().
			WithCore(mainAddr).
			WithClientID(id).
			WithStream(true). // 在 CONNECT 中声明支持 stream
			WithAutoReconnect(false).
			WithLogger(logger))
		if err != nil {
			log.Fatal(err)
		}
		if err := c.ConnectContext(ctx); err != nil {
			log.Fatal(err)
		}
		return c
	}

	// 接收方：接受后把收到的内容原样回写
	worker := newClient("worker")
	defer func() { _ = worker.Close() }()
	worker.OnStreamRequest(func(req *client.StreamRequest) {
		s, err := req.Accept(ctx, []byte("welcome"), nil)
		if err != nil {
			log.Printf("accept stream: %v", err)
			return
		}
		defer func() { _ = s.Close() }()
		_, _ = io.Copy(s, s)
	})

	// 发起方
	caller := newClient("caller")
	defer func() { _ = caller.Close() }()

	stream, err := caller.OpenStream(ctx, "worker", "chat/1", []byte("hello"), nil)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = stream.Close() }()

	fmt.Printf("stream %d established: %s -> %s, peer metadata=%q\n",
		stream.ID(), stream.LocalClientID(), stream.PeerClientID(), stream.Metadata())

	if _, err := stream.Write([]byte("ping over stream")); err != nil {
		log.Fatal(err)
	}
	buf := make([]byte, len("ping over stream"))
	if _, err := io.ReadFull(stream, buf); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("echo: %q\n", buf)

	stats := b.GetStats()
	fmt.Printf("active streams=%d total=%d a->b=%d b->a=%d\n",
		stats.StreamsActive, stats.StreamsTotal, stats.StreamBytesAToB, stats.StreamBytesBToA)
}
