// Package store 提供基于 BadgerDB 的消息持久化存储
// 被 core 和 client 包共享使用
package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"go.uber.org/zap"
)

// Options 存储配置
type Options struct {
	DataDir          string        // 存储路径（空字符串表示使用内存模式）
	SyncWrites       bool          // 同步写入 (更安全但更慢)
	ValueLogFileSize int64         // 值日志文件大小 (MB)
	GCInterval       time.Duration // GC 间隔
	Logger           *zap.Logger   // 日志
}

// Validate 验证存储配置的有效性
func (o *Options) Validate() error {
	if o.ValueLogFileSize <= 0 {
		return fmt.Errorf("valueLogFileSize must be > 0, got %d", o.ValueLogFileSize)
	}
	if o.GCInterval < 0 {
		return fmt.Errorf("gcInterval must be >= 0, got %v", o.GCInterval)
	}
	return nil
}

// Store 消息持久化存储
type Store struct {
	options Options
	db      *badger.DB
	logger  *zap.Logger
	closeCh chan struct{}
	wg      sync.WaitGroup
}

// New 创建存储实例
func New(options Options) (*Store, error) {
	if options.Logger == nil {
		options.Logger, _ = zap.NewDevelopment()
	}

	var opts badger.Options
	if options.DataDir == "" {
		opts = badger.DefaultOptions("").WithInMemory(true)
		options.Logger.Info("Message store using InMemory mode")
	} else {
		opts = badger.DefaultOptions(options.DataDir)
	}

	opts.Logger = nil
	opts.SyncWrites = options.SyncWrites
	if options.ValueLogFileSize > 0 {
		opts.ValueLogFileSize = options.ValueLogFileSize << 20
	}

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}

	s := &Store{
		options: options,
		db:      db,
		logger:  options.Logger,
		closeCh: make(chan struct{}),
	}

	if options.GCInterval > 0 {
		s.wg.Add(1)
		go s.gcLoop()
	}

	if options.DataDir == "" {
		options.Logger.Info("Message store initialized (InMemory mode)",
			zap.Bool("syncWrites", options.SyncWrites))
	} else {
		options.Logger.Info("Message store initialized",
			zap.String("dataDir", options.DataDir),
			zap.Bool("syncWrites", options.SyncWrites))
	}

	return s, nil
}

// DB 返回底层 BadgerDB 实例
func (s *Store) DB() *badger.DB {
	return s.db
}

// Close 关闭存储
func (s *Store) Close() error {
	close(s.closeCh)
	s.wg.Wait()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// gcLoop 定期运行 GC
func (s *Store) gcLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.options.GCInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.closeCh:
			return
		case <-ticker.C:
			s.runGC()
		}
	}
}

// runGC 执行垃圾回收
func (s *Store) runGC() {
	for {
		err := s.db.RunValueLogGC(0.5)
		if err != nil {
			break
		}
	}
}
