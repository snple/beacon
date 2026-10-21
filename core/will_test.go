package core

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestWill_DisconnectWithWillPublishes 验证 DISCONNECT(DisconnectWithWill) 会发布遗嘱。
func TestWill_DisconnectWithWillPublishes(t *testing.T) {
	core := testSetupCore(t, NewCoreOptions())
	defer core.Stop()

	addr := testServe(t, core)

	subscriber := testSetupClient(t, addr, "will-explicit-sub", nil)
	defer subscriber.Close()
	if err := subscriber.Subscribe("will/explicit"); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	willPkt := packet.NewPublishPacket("will/explicit", []byte("explicit-bye"))
	publisher := testSetupClient(t, addr, "will-explicit-pub",
		client.NewClientOptions().WithWill(&client.WillMessage{Packet: willPkt}))
	defer publisher.Close()

	time.Sleep(100 * time.Millisecond)

	if err := publisher.DisconnectWithReason(packet.ReasonDisconnectWithWill); err != nil {
		t.Fatalf("Failed to disconnect with will: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	msg, err := subscriber.PollMessage(ctx, 2*time.Second)
	require.NoError(t, err, "will should be published on DisconnectWithWill")
	require.Equal(t, []byte("explicit-bye"), msg.Packet.Payload)
}

// TestWill_NormalDisconnectDoesNotPublish 验证普通 DISCONNECT 不会发布遗嘱。
func TestWill_NormalDisconnectDoesNotPublish(t *testing.T) {
	core := testSetupCore(t, NewCoreOptions())
	defer core.Stop()

	addr := testServe(t, core)

	subscriber := testSetupClient(t, addr, "will-normal-sub", nil)
	defer subscriber.Close()
	if err := subscriber.Subscribe("will/normal"); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	willPkt := packet.NewPublishPacket("will/normal", []byte("should-not-arrive"))
	publisher := testSetupClient(t, addr, "will-normal-pub",
		client.NewClientOptions().WithWill(&client.WillMessage{Packet: willPkt}))
	defer publisher.Close()

	time.Sleep(100 * time.Millisecond)

	if err := publisher.Disconnect(); err != nil {
		t.Fatalf("Failed to disconnect: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := subscriber.PollMessage(ctx, 300*time.Millisecond)
	require.ErrorIs(t, err, client.ErrPollTimeout, "normal DISCONNECT must clear the will")
}

// TestWill_ServerShutdownDoesNotPublish 验证服务端停机关闭连接时不发布遗嘱。
func TestWill_ServerShutdownDoesNotPublish(t *testing.T) {
	core, err := NewWithOptions(NewCoreOptions())
	require.NoError(t, err)
	defer core.store.close()

	client := newQuotaTestClient(t, core, "will-shutdown")
	client.session.willPacket = packet.NewPublishPacket("will/shutdown", []byte("bye"))

	conn := &conn{client: client, conn: &failingWriteConn{}}
	conn.closeReason.Store(uint32(packet.ReasonServerShuttingDown))
	client.conn = conn

	before := core.queue.Count()
	core.handleClientDisconnect(client)
	require.Equal(t, before, core.queue.Count(),
		"server shutdown must not publish the will")
}

// TestWill_AbnormalDisconnectPublishes 验证异常断开（心跳超时/网络错误）仍会发布遗嘱。
func TestWill_AbnormalDisconnectPublishes(t *testing.T) {
	core, err := NewWithOptions(NewCoreOptions())
	require.NoError(t, err)
	defer core.store.close()

	client := newQuotaTestClient(t, core, "will-abnormal")
	client.session.willPacket = packet.NewPublishPacket("will/abnormal", []byte("bye"))

	conn := &conn{client: client, conn: &failingWriteConn{}}
	conn.closeReason.Store(uint32(packet.ReasonKeepAliveTimeout))
	client.conn = conn

	before := core.queue.Count()
	core.handleClientDisconnect(client)
	require.Equal(t, before+1, core.queue.Count(),
		"abnormal disconnect must publish the will")
}

// TestWill_ExpiryIsRelativeToPublish 验证遗嘱 TTL 从实际发布时间起算，
// 且不会沿用 CONNECT 时携带的绝对时间。
func TestWill_ExpiryIsRelativeToPublish(t *testing.T) {
	core, err := NewWithOptions(NewCoreOptions().WithDefaultMessageExpiry(24 * time.Hour))
	require.NoError(t, err)
	defer core.store.close()

	client := newQuotaTestClient(t, core, "will-ttl")
	will := packet.NewPublishPacket("will/ttl", []byte("bye"))
	will.Properties.ExpiryTime = 30 // 相对 TTL：30 秒
	client.session.willPacket = will

	client.publishWill()

	msg, err := core.queue.Peek()
	require.NoError(t, err)
	require.NotNil(t, msg.Packet.Properties)

	got := time.Unix(msg.Packet.Properties.ExpiryTime, 0)
	require.WithinDuration(t, time.Now().Add(30*time.Second), got, 3*time.Second,
		"will TTL should start at publish time, not at CONNECT time")
}

// TestWill_ZeroExpiryStaysUnlimited 验证未设置 TTL 且未配置 DefaultMessageExpiry 时遗嘱永不过期。
func TestWill_ZeroExpiryStaysUnlimited(t *testing.T) {
	core, err := NewWithOptions(NewCoreOptions().WithDefaultMessageExpiry(0))
	require.NoError(t, err)
	defer core.store.close()

	client := newQuotaTestClient(t, core, "will-no-ttl")
	client.session.willPacket = packet.NewPublishPacket("will/no-ttl", []byte("bye"))

	client.publishWill()

	msg, err := core.queue.Peek()
	require.NoError(t, err)
	require.Zero(t, msg.Packet.Properties.ExpiryTime)
}

// TestWill_ConcurrentAccessNoRace 验证公开 API 与遗嘱处理的并发访问无数据竞争。
//
// 回归：WillPacket/SetWillPacket 直接读写 session.willPacket，与
// publishWill/clearWill 并发时是 data race（公开 API 可触发）。
func TestWill_ConcurrentAccessNoRace(t *testing.T) {
	core := testSetupCore(t, NewCoreOptions())
	defer core.Stop()

	c := &Client{ID: "will-race", core: core, session: &session{core: core}}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(4)
		go func(n int) {
			defer wg.Done()
			c.SetWillPacket(packet.NewPublishPacket("will/race", []byte("x")))
		}(i)
		go func() {
			defer wg.Done()
			_ = c.WillPacket()
		}()
		go func() {
			defer wg.Done()
			c.clearWill()
		}()
		go func() {
			defer wg.Done()
			c.extractWillMessage(packet.NewConnectPacket())
		}()
	}
	wg.Wait()
}

// TestWill_PublishAtMostOnce 验证并发调用 publishWill 时遗嘱最多发布一次。
func TestWill_PublishAtMostOnce(t *testing.T) {
	core := testSetupCore(t, NewCoreOptions())
	defer core.Stop()

	c := &Client{ID: "will-once", core: core, session: &session{core: core}}
	c.SetWillPacket(packet.NewPublishPacket("will/once", []byte("bye")))

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.publishWill()
		}()
	}
	wg.Wait()

	require.Equal(t, int64(1), core.queue.Count(), "遗嘱只应发布一次")
	require.Nil(t, c.WillPacket(), "发布后应清除遗嘱")
}
