// Package client 提供消息持久化存储
package client

import (
	"time"

	"github.com/dgraph-io/badger/v4"
	istore "github.com/snple/beacon/internal/store"
)

// StoreOptions 存储配置（类型别名，保持向后兼容）
type StoreOptions = istore.Options

// DefaultStoreOptions 返回默认存储配置
func DefaultStoreOptions() StoreOptions {
	return StoreOptions{
		DataDir:          "",    // InMemory 模式
		SyncWrites:       false, // 默认异步写入，性能更好
		ValueLogFileSize: 256,   // 256MB
		GCInterval:       5 * time.Minute,
		Logger:           nil,
	}
}

// store 消息持久化存储
// 用于存储 QoS=1 的发送消息，确保客户端重连后消息不丢失
type store struct {
	*istore.Store
}

// newStore 创建消息存储
func newStore(options StoreOptions) (*store, error) {
	s, err := istore.New(options)
	if err != nil {
		return nil, err
	}

	return &store{Store: s}, nil
}

// Close 关闭存储
func (ms *store) Close() error {
	return ms.Store.Close()
}

// db 返回底层数据库（兼容现有代码）
func (ms *store) db() *badger.DB {
	return ms.Store.DB()
}
