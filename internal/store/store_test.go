package store

import (
	"testing"

	"github.com/dgraph-io/badger/v4"
)

// TestStoreClose_Idempotent 验证 Store.Close 可安全多次调用（回归 M4：
// 之前 close(closeCh) 在第二次调用时会 panic "close of closed channel"）。
func TestStoreClose_Idempotent(t *testing.T) {
	s, err := New(Options{})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}

	// 第二次 Close 不应 panic，且应返回 nil
	if err := s.Close(); err != nil {
		t.Fatalf("second close should return nil, got %v", err)
	}
}

// TestStoreClose_AfterDBUsed 验证 store 关闭后再关闭仍然安全（幂等）。
func TestStoreClose_AfterDBUsed(t *testing.T) {
	s, err := New(Options{})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// 使用 DB 执行一次简单写入，确保 DB 已初始化
	if err := s.DB().Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("k"), []byte("v"))
	}); err != nil {
		t.Fatalf("db write failed: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second close should return nil, got %v", err)
	}
}
