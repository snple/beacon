package client_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
)

// DECLARE_RESP 回传的 Limits 必须被采用，PULL_OK / DECLARE_RESP 的积压快照
// 必须出现在结果里。
func TestDeclareAndPullExposeServerSnapshot(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		nc, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = nc.Close() }()

		if _, err := packet.ReadPacket(nc, 1<<20); err != nil {
			return
		}
		connack := packet.NewConnackPacket(packet.ReasonSuccess)
		connack.KeepAlive = 30
		connack.AssignedClientID = "snapshot-1"
		connack.SessionID = 1
		connack.Limits.MaxPacketSize = 1 << 10
		if err := packet.WritePacket(nc, connack, 0); err != nil {
			return
		}

		for {
			pkt, err := packet.ReadPacket(nc, 1<<20)
			if err != nil {
				return
			}
			switch p := pkt.(type) {
			case *packet.DeclarePacket:
				resp := packet.NewDeclareRespPacket(p.RequestID)
				resp.Accepted = uint16(len(p.Channels))
				resp.Pending = [3]uint32{1, 2, 3}
				resp.Limits.MaxPacketSize = 1 << 11
				if err := packet.WritePacket(nc, resp, 0); err != nil {
					return
				}
			case *packet.PullPacket:
				resp := packet.NewPullOkPacket(p.RequestID)
				resp.Pending = [3]uint32{4, 5, 6}
				if err := packet.WritePacket(nc, resp, 0); err != nil {
					return
				}
			case *packet.DisconnectPacket:
				return
			}
		}
	}()

	opts := client.NewClientOptions().
		WithServer(ln.Addr().String()).
		WithClientID("snapshot-1").
		WithKeepAlive(30).
		WithAutoReconnect(false)
	c, err := client.NewWithOptions(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.ConnectContext(ctx); err != nil {
		t.Fatal(err)
	}
	if got := c.ServerLimits().MaxPacketSize; got != 1<<10 {
		t.Fatalf("CONNACK limits not applied: %d", got)
	}

	declared, err := c.Declare(ctx, client.Filter("jobs/**"))
	if err != nil {
		t.Fatal(err)
	}
	if declared.Accepted != 1 || len(declared.Rejected) != 0 {
		t.Fatalf("unexpected declare result: %+v", declared)
	}
	if declared.Pending != [3]uint32{1, 2, 3} {
		t.Fatalf("declare pending snapshot lost: %v", declared.Pending)
	}
	if declared.Limits.MaxPacketSize != 1<<11 {
		t.Fatalf("declare limits lost: %d", declared.Limits.MaxPacketSize)
	}
	// DECLARE_RESP 返回的 Limits 必须成为客户端的最新视图。
	if got := c.ServerLimits().MaxPacketSize; got != 1<<11 {
		t.Fatalf("declare limits not applied to client: %d", got)
	}

	pulled, err := c.PullBatch(ctx, client.PullOptions{Immediate: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(pulled.Messages) != 0 {
		t.Fatalf("expected no messages, got %d", len(pulled.Messages))
	}
	if pulled.Pending != [3]uint32{4, 5, 6} {
		t.Fatalf("pull pending snapshot lost: %v", pulled.Pending)
	}

	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-serveDone:
	case <-time.After(2 * time.Second):
		t.Fatal("broker did not observe DISCONNECT")
	}
}
