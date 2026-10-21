package core

import (
	"bytes"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/client"
	"github.com/snple/beacon/internal/mq"
	"github.com/stretchr/testify/require"
)

// TestSessionRecord_DoesNotPersistAuthData 验证会话记录不会把 CONNECT 中的
// 认证数据（设备密钥）明文写入磁盘。
func TestSessionRecord_DoesNotPersistAuthData(t *testing.T) {
	dir := t.TempDir()
	c := testSetupCore(t, NewCoreOptions().WithStoreDir(dir))
	defer c.Stop()

	addr := testServe(t, c)

	const secret = "super-secret-device-key"
	cl := testSetupClient(t, addr, "auth-client", client.NewClientOptions().
		WithSessionTimeout(60).
		WithAuth("token", []byte(secret)))
	defer cl.Close()

	require.NoError(t, cl.Subscribe("auth/topic"))
	require.NoError(t, cl.Disconnect())

	// 等离线记录落盘（注册时 ExpiresAt=0，断开后变为 >0）。
	var raw []byte
	ok := eventually(t, 3*time.Second, 20*time.Millisecond, "offline session persisted", func() bool {
		raw = nil
		err := c.store.db().View(func(txn *badger.Txn) error {
			item, err := txn.Get(sessionKey("auth-client"))
			if err != nil {
				return err
			}
			return item.Value(func(v []byte) error {
				raw = append([]byte(nil), v...)
				return nil
			})
		})
		if err != nil {
			return false
		}

		var rec sessionRecord
		if err := mq.DecodeMessage(&rec, raw); err != nil {
			return false
		}
		return rec.ExpiresAt > 0
	})
	require.True(t, ok, "offline session record was not persisted")
	require.NotEmpty(t, raw)

	require.False(t, bytes.Contains(raw, []byte(secret)),
		"session record must not contain the authentication secret")
	require.False(t, bytes.Contains(raw, []byte("token")),
		"session record must not contain the authentication method")
}
