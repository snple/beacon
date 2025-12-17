package storage

import (
	"testing"

	"github.com/dgraph-io/badger/v4"
)

// setupTestDB 创建一个临时的内存数据库用于测试
func setupTestDB(t *testing.T) *badger.DB {
	opts := badger.DefaultOptions("").WithInMemory(true)
	// 使用 ManagedDB 模式以支持 NewTransactionAt 和 CommitAt
	db, err := badger.OpenManaged(opts)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	return db
}
