package core

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestConnect_RejectsInvalidClientID 验证服务端用 CONNACK(ReasonClientIDNotValid)
// 拒绝含 ':'、控制字符或超长的 ClientID。
func TestConnect_RejectsInvalidClientID(t *testing.T) {
	c := testSetupCore(t, NewCoreOptions())
	defer c.Stop()

	addr := testServe(t, c)

	ids := []string{
		"bad:id",
		"bad\nid",
		strings.Repeat("a", packet.MaxClientIDLength+1),
	}

	for _, id := range ids {
		t.Run(id[:min(len(id), 8)], func(t *testing.T) {
			conn, err := net.Dial("tcp", addr)
			require.NoError(t, err)
			defer conn.Close()

			connect := packet.NewConnectPacket()
			connect.ClientID = id
			require.NoError(t, packet.WritePacket(conn, connect, 0))

			require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
			pkt, err := packet.ReadPacket(conn, 0)
			require.NoError(t, err)

			connack, ok := pkt.(*packet.ConnackPacket)
			require.True(t, ok, "expected CONNACK, got %T", pkt)
			require.Equal(t, packet.ReasonClientIDNotValid, connack.ReasonCode)
		})
	}
}

// TestRecoverSessions_DropsInvalidClientID 验证旧版本遗留的非法 ID 会话
// 在启动时被丢弃，其队列按孤儿清理。
func TestRecoverSessions_DropsInvalidClientID(t *testing.T) {
	dir := t.TempDir()

	c1 := testSetupCore(t, NewCoreOptions().WithStoreDir(dir))
	c1.saveRecord(&sessionRecord{
		ClientID:  "legacy:id",
		Timeout:   60,
		ExpiresAt: time.Now().Add(time.Minute).Unix(),
	})

	queue := NewQueue(c1.store.db(), "client:legacy:id:")
	require.NoError(t, queue.Enqueue(createTestMessage("legacy/topic", []byte("x"), packet.QoS0)))
	require.Equal(t, int64(1), queue.Count())
	require.NoError(t, c1.Stop())

	c2 := testSetupCore(t, NewCoreOptions().WithStoreDir(dir))
	defer c2.Stop()

	var recordFound bool
	require.NoError(t, c2.store.db().View(func(txn *badger.Txn) error {
		_, err := txn.Get(sessionKey("legacy:id"))
		recordFound = err == nil
		return nil
	}))
	require.False(t, recordFound, "invalid client ID session record must be dropped")

	restored := NewQueue(c2.store.db(), "client:legacy:id:")
	require.Equal(t, int64(0), restored.Count(), "invalid client ID queue must be cleaned up")
}
