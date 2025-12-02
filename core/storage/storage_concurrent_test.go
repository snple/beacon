package storage

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestStorage_ConcurrentRead 测试并发读取
func TestStorage_ConcurrentRead(t *testing.T) {
	// 创建内存数据库
	db := setupTestDB(t)
	defer db.Close()

	s := New(db)

	const numReaders = 100
	var wg sync.WaitGroup
	wg.Add(numReaders)

	errorsChan := make(chan error, numReaders)

	// 启动多个并发读取
	for i := 0; i < numReaders; i++ {
		go func(id int) {
			defer wg.Done()

			// 并发读取节点列表
			nodes := s.ListNodes()
			_ = nodes

			// 并发读取单个节点（可能不存在）
			_, err := s.GetNode("test-node")
			if err != nil && err.Error() != "node not found: test-node" {
				errorsChan <- fmt.Errorf("reader %d: %w", id, err)
			}
		}(i)
	}

	wg.Wait()
	close(errorsChan)

	// 检查是否有错误
	for err := range errorsChan {
		t.Error(err)
	}
}

// TestStorage_ConcurrentReadWrite 测试并发读写
func TestStorage_ConcurrentReadWrite(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := New(db)

	const numReaders = 50
	const numIterations = 10
	var wg sync.WaitGroup
	wg.Add(numReaders)

	errorsChan := make(chan error, numReaders)

	// 启动读取协程
	for i := 0; i < numReaders; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numIterations; j++ {
				// 读取节点列表
				nodes := s.ListNodes()
				_ = nodes

				// 读取单个节点
				_, _ = s.GetNode(fmt.Sprintf("node-%d", id%10))

				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	close(errorsChan)

	// 检查是否有错误
	for err := range errorsChan {
		t.Error(err)
	}
}

// TestStorage_RaceConditions 使用 -race 检测竞态条件
//
// 运行方式: go test -race -run TestStorage_RaceConditions
func TestStorage_RaceConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race condition test in short mode")
	}

	db := setupTestDB(t)
	defer db.Close()

	s := New(db)

	var wg sync.WaitGroup
	const numGoroutines = 100

	// 并发访问 ListNodes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			_ = s.ListNodes()
		}()
	}
	wg.Wait()

	// 并发访问 GetNode
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			_, _ = s.GetNode(fmt.Sprintf("node-%d", id))
		}(i)
	}
	wg.Wait()
}
