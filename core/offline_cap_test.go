package core

import (
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/stretchr/testify/require"
)

// TestOfflineSessionCap_EvictsEarliest 验证离线会话超过上限时驱逐最早过期的会话。
func TestOfflineSessionCap_EvictsEarliest(t *testing.T) {
	c := testSetupCore(t, NewCoreOptions().WithMaxOfflineSessions(1))
	defer c.Stop()

	addr := testServe(t, c)

	first := testSetupClient(t, addr, "cap-first", client.NewClientOptions().WithSessionTimeout(60))
	defer first.Close()
	require.NoError(t, first.Subscribe("cap/topic"))
	require.NoError(t, first.Disconnect())

	require.True(t, eventually(t, 3*time.Second, 20*time.Millisecond, "first offline session registered", func() bool {
		return c.GetStats().OfflineSessions == 1
	}), "first offline session should be registered")

	second := testSetupClient(t, addr, "cap-second", client.NewClientOptions().WithSessionTimeout(60))
	defer second.Close()
	require.NoError(t, second.Subscribe("cap/topic"))
	require.NoError(t, second.Disconnect())

	require.True(t, eventually(t, 3*time.Second, 20*time.Millisecond, "earliest offline session evicted", func() bool {
		if c.GetStats().OfflineSessions != 1 {
			return false
		}
		_, errFirst := c.GetClientInfo("cap-first")
		_, errSecond := c.GetClientInfo("cap-second")
		return errFirst != nil && errSecond == nil
	}), "earliest offline session should be evicted, newest preserved")

	if _, err := c.GetClientSubscriptions("cap-first"); err == nil {
		t.Fatal("evicted session's subscriptions should be removed")
	}
}
