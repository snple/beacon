package core

import (
	"context"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

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

// TestSession_SurvivesReconnect 验证会话（订阅 + 离线积压）在重连后恢复。
// 会话只保存在内存中：进程重启即失效。
func TestSession_SurvivesReconnect(t *testing.T) {
	core := testSetupCore(t, NewCoreOptions().WithRetransmitInterval(time.Second))
	defer core.Stop()
	addr := testServe(t, core)

	first := testSetupClient(t, addr, "resume-sub", client.NewClientOptions().WithSessionTimeout(60))
	if err := first.SubscribeWithOptions([]string{"resume/topic"}, client.NewSubscribeOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := first.Disconnect(); err != nil {
		t.Fatalf("Failed to disconnect: %v", err)
	}
	_ = first.Close()

	publisher := testSetupClient(t, addr, "resume-pub", nil)
	defer publisher.Close()
	if err := publisher.Publish("resume/topic", []byte("before-reconnect"),
		client.NewPublishOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	require.True(t, eventually(t, 3*time.Second, 20*time.Millisecond, "message queued for offline session", func() bool {
		info, err := core.GetClientInfo("resume-sub")
		return err == nil && info.QueueSize == 1
	}))

	// 同一 ClientID、未设置 CleanStart → 恢复会话
	second := testSetupClient(t, addr, "resume-sub", client.NewClientOptions().WithSessionTimeout(60))
	defer second.Close()

	if !second.SessionPresent() {
		t.Fatal("session should be resumed on reconnect")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msg, err := second.PollMessage(ctx, 4*time.Second)
	require.NoError(t, err, "queued message should be delivered after reconnect")
	require.Equal(t, []byte("before-reconnect"), msg.Packet.Payload)

	// 订阅也应恢复：无需重新订阅即可收到新消息
	require.NoError(t, publisher.Publish("resume/topic", []byte("after-reconnect"),
		client.NewPublishOptions().WithQoS(packet.QoS1)))

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	msg2, err := second.PollMessage(ctx2, 4*time.Second)
	require.NoError(t, err, "subscription should be restored")
	require.Equal(t, []byte("after-reconnect"), msg2.Packet.Payload)
}

// TestSession_ExpiredOfflineIsDiscarded 验证离线会话过期后（订阅 + 队列）被清理。
func TestSession_ExpiredOfflineIsDiscarded(t *testing.T) {
	core := testSetupCore(t, NewCoreOptions())
	defer core.Stop()

	addr := testServe(t, core)

	first := testSetupClient(t, addr, "expired-sub", client.NewClientOptions().WithSessionTimeout(1))
	if err := first.SubscribeWithOptions([]string{"expired/topic"}, client.NewSubscribeOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := first.Disconnect(); err != nil {
		t.Fatalf("Failed to disconnect: %v", err)
	}
	_ = first.Close()

	// 等待离线会话过期：ExpiredCheckInterval 的最小值是 10s，这里直接驱动清理。
	require.True(t, eventually(t, 5*time.Second, 50*time.Millisecond, "offline session expired", func() bool {
		core.cleanupExpiredSessions()
		_, err := core.GetClientInfo("expired-sub")
		return err != nil
	}), "expired session must be removed")

	second := testSetupClient(t, addr, "expired-sub", client.NewClientOptions().WithSessionTimeout(1))
	defer second.Close()

	if second.SessionPresent() {
		t.Fatal("expired session must not be resumed")
	}

	subs, err := getClientSubscriptionsForTest(core, "expired-sub")
	require.NoError(t, err)
	require.Empty(t, subs, "expired session's subscriptions must be gone")
}
