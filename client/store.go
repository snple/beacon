// Package client 提供消息持久化存储
package client

import (
	"fmt"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"go.uber.org/zap"
)

// StoreConfig 存储配置
type StoreOptions struct {
	// 存储路径（空字符串表示使用内存模式）
	DataDir string

	// 同步写入 (更安全但更慢)
	SyncWrites bool

	// 值日志文件大小 (MB)
	ValueLogFileSize int64

	// GC 间隔
	GCInterval time.Duration

	// 日志
	Logger *zap.Logger
}

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

// Validate 验证存储配置的有效性
func (c *StoreOptions) Validate() error {
	// DataDir 为空时使用 InMemory 模式，不需要验证
	if c.ValueLogFileSize <= 0 {
		return fmt.Errorf("valueLogFileSize must be > 0, got %d", c.ValueLogFileSize)
	}

	if c.GCInterval < 0 {
		return fmt.Errorf("gcInterval must be >= 0, got %v", c.GCInterval)
	}

	return nil
}

// store 消息持久化存储
// 用于存储 QoS=1 的发送消息，确保客户端重连后消息不丢失
type store struct {
	options StoreOptions

	db     *badger.DB
	logger *zap.Logger

	// 生命周期
	closeCh chan struct{}
	wg      sync.WaitGroup
}

// newStore 创建消息存储
func newStore(options StoreOptions) (*store, error) {
	if options.Logger == nil {
		options.Logger, _ = zap.NewDevelopment()
	}

	// 配置 badger options
	var opts badger.Options
	if options.DataDir == "" {
		// InMemory 模式
		opts = badger.DefaultOptions("").WithInMemory(true)
		options.Logger.Info("Client message store using InMemory mode")
	} else {
		// 持久化模式
		opts = badger.DefaultOptions(options.DataDir)
	}

	opts.Logger = nil // 禁用 badger 内置日志
	opts.SyncWrites = options.SyncWrites
	if options.ValueLogFileSize > 0 {
		opts.ValueLogFileSize = options.ValueLogFileSize << 20 // 转换为字节
	}

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}

	ms := &store{
		db:      db,
		options: options,
		logger:  options.Logger,
		closeCh: make(chan struct{}),
	}

	// 启动 GC 协程
	if options.GCInterval > 0 {
		ms.wg.Add(1)
		go ms.gcLoop()
	}

	if options.DataDir == "" {
		options.Logger.Info("Client message store initialized (InMemory mode)",
			zap.Bool("syncWrites", options.SyncWrites))
	} else {
		options.Logger.Info("Client message store initialized",
			zap.String("dataDir", options.DataDir),
			zap.Bool("syncWrites", options.SyncWrites))
	}

	return ms, nil
}

// Close 关闭存储
func (ms *store) Close() error {
	close(ms.closeCh)
	ms.wg.Wait()

	if ms.db != nil {
		return ms.db.Close()
	}
	return nil
}

// gcLoop 定期运行 GC
func (ms *store) gcLoop() {
	defer ms.wg.Done()

	ticker := time.NewTicker(ms.options.GCInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ms.closeCh:
			return
		case <-ticker.C:
			ms.runGC()
		}
	}
}

// runGC 执行垃圾回收
func (ms *store) runGC() {
	for {
		err := ms.db.RunValueLogGC(0.5)
		if err != nil {
			break
		}
	}
}
