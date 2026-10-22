package core

import (
	"context"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

func TestClientIDFromQueueKey(t *testing.T) {
	id := make([]byte, 12)
	key := append([]byte("client:a:b:c:"), id...)

	clientID, ok := clientIDFromQueueKey(key)
	require.True(t, ok)
	require.Equal(t, "a:b:c", clientID, "legacy queue keys may encode IDs containing ':'")

	_, ok = clientIDFromQueueKey([]byte("client:short"))
	require.False(t, ok)

	_, ok = clientIDFromQueueKey([]byte("session:foo"))
	require.False(t, ok)
}

// TestSession_CleanStartDiscardsExistingSession 验证 CleanStart 会丢弃旧会话的订阅与待投递消息。
func TestSession_CleanStartDiscardsExistingSession(t *testing.T) {
	core := testSetupCore(t, NewCoreOptions().WithRetransmitInterval(time.Second))
	defer core.Stop()

	addr := testServe(t, core)

	first := testSetupClient(t, addr, "cleanstart-sub", client.NewClientOptions().WithSessionTimeout(60))
	if err := first.SubscribeWithOptions([]string{"cleanstart/topic"}, client.NewSubscribeOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := first.Disconnect(); err != nil {
		t.Fatalf("Failed to disconnect: %v", err)
	}
	defer first.Close()

	publisher := testSetupClient(t, addr, "cleanstart-pub", nil)
	defer publisher.Close()

	if err := publisher.Publish("cleanstart/topic", []byte("queued"),
		client.NewPublishOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	require.True(t, eventually(t, 3*time.Second, 20*time.Millisecond, "message queued for offline session", func() bool {
		info, err := core.GetClientInfo("cleanstart-sub")
		return err == nil && info.QueueSize == 1
	}))

	// 带 CleanStart 重连：服务端应丢弃旧会话
	second := testSetupClient(t, addr, "cleanstart-sub", client.NewClientOptions().
		WithSessionTimeout(60).
		WithCleanStart(true))
	defer second.Close()

	if second.SessionPresent() {
		t.Fatal("CleanStart must not resume the old session")
	}

	subs, err := getClientSubscriptionsForTest(core, "cleanstart-sub")
	require.NoError(t, err)
	require.Empty(t, subs, "old subscriptions must be discarded")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = second.PollMessage(ctx, 300*time.Millisecond)
	require.ErrorIs(t, err, client.ErrPollTimeout, "queued messages must be discarded by CleanStart")

	info, err := core.GetClientInfo("cleanstart-sub")
	require.NoError(t, err)
	require.Equal(t, int64(0), info.QueueSize)
}

// TestSession_PersistsAcrossRestart 验证会话元数据（订阅 + 离线积压）在 core 重启后恢复。
func TestSession_PersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	newOpts := func() *CoreOptions {
		return NewCoreOptions().WithStoreDir(dir).WithRetransmitInterval(time.Second)
	}

	core1 := testSetupCore(t, newOpts())
	addr1 := testServe(t, core1)

	first := testSetupClient(t, addr1, "persist-sub", client.NewClientOptions().WithSessionTimeout(60))
	if err := first.SubscribeWithOptions([]string{"persist/topic"}, client.NewSubscribeOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := first.Disconnect(); err != nil {
		t.Fatalf("Failed to disconnect: %v", err)
	}
	_ = first.Close()

	publisher := testSetupClient(t, addr1, "persist-pub", nil)
	if err := publisher.Publish("persist/topic", []byte("before-restart"),
		client.NewPublishOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}
	_ = publisher.Close()

	require.True(t, eventually(t, 3*time.Second, 20*time.Millisecond, "message queued before restart", func() bool {
		info, err := core1.GetClientInfo("persist-sub")
		return err == nil && info.QueueSize == 1
	}))

	require.NoError(t, core1.Stop())

	core2 := testSetupCore(t, newOpts())
	defer core2.Stop()
	addr2 := testServe(t, core2)

	// 同一 ClientID、未设置 CleanStart → 恢复会话
	second := testSetupClient(t, addr2, "persist-sub", client.NewClientOptions().WithSessionTimeout(60))
	defer second.Close()

	if !second.SessionPresent() {
		t.Fatal("session should be restored after restart")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msg, err := second.PollMessage(ctx, 4*time.Second)
	require.NoError(t, err, "queued message should survive restart")
	require.Equal(t, []byte("before-restart"), msg.Packet.Payload)

	// 订阅也应恢复：无需重新订阅即可收到新消息
	publisher2 := testSetupClient(t, addr2, "persist-pub2", nil)
	defer publisher2.Close()
	require.NoError(t, publisher2.Publish("persist/topic", []byte("after-restart"),
		client.NewPublishOptions().WithQoS(packet.QoS1)))

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	msg2, err := second.PollMessage(ctx2, 4*time.Second)
	require.NoError(t, err, "subscription should be restored after restart")
	require.Equal(t, []byte("after-restart"), msg2.Packet.Payload)
}

// TestSession_ExpiredWhileStoppedIsDiscarded 验证停机期间过期的会话（订阅 + 队列）在启动时被清理。
func TestSession_ExpiredWhileStoppedIsDiscarded(t *testing.T) {
	dir := t.TempDir()

	core1 := testSetupCore(t, NewCoreOptions().WithStoreDir(dir))
	addr1 := testServe(t, core1)

	first := testSetupClient(t, addr1, "expired-sub", client.NewClientOptions().WithSessionTimeout(1))
	if err := first.SubscribeWithOptions([]string{"expired/topic"}, client.NewSubscribeOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := first.Disconnect(); err != nil {
		t.Fatalf("Failed to disconnect: %v", err)
	}
	_ = first.Close()

	// 等待会话过期后再重启
	time.Sleep(1200 * time.Millisecond)
	require.NoError(t, core1.Stop())

	core2 := testSetupCore(t, NewCoreOptions().WithStoreDir(dir))
	defer core2.Stop()
	addr2 := testServe(t, core2)

	second := testSetupClient(t, addr2, "expired-sub", client.NewClientOptions().WithSessionTimeout(1))
	defer second.Close()

	if second.SessionPresent() {
		t.Fatal("expired session must not be resumed")
	}
	subs, err := getClientSubscriptionsForTest(core2, "expired-sub")
	require.NoError(t, err)
	require.Empty(t, subs, "expired session's subscriptions must be gone")

	queue := NewQueue(core2.store.db(), "client:expired-sub:")
	require.Equal(t, int64(0), queue.Count(), "expired session's queue must be cleaned up")
}

// TestSession_OrphanClientQueueCleanedOnStart 验证没有会话记录的客户端队列在启动时被清理。
func TestSession_OrphanClientQueueCleanedOnStart(t *testing.T) {
	dir := t.TempDir()

	core1 := testSetupCore(t, NewCoreOptions().WithStoreDir(dir))

	queue := NewQueue(core1.store.db(), "client:orphan-client:")
	require.NoError(t, queue.Enqueue(createTestMessage("orphan/topic", []byte("x"), packet.QoS0)))
	require.Equal(t, int64(1), queue.Count())
	require.NoError(t, core1.Stop())

	core2 := testSetupCore(t, NewCoreOptions().WithStoreDir(dir))
	defer core2.Stop()

	restored := NewQueue(core2.store.db(), "client:orphan-client:")
	require.Equal(t, int64(0), restored.Count(), "orphan queue should be cleaned up on startup")
}
