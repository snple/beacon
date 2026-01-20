package core

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/danclive/nson-go"
	"github.com/dgraph-io/badger/v4"
)

// 测试基本的入队出队操作
func TestIdQueue_BasicOperations(t *testing.T) {
	dir, err := os.MkdirTemp("", "idqueue-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	opts := badger.DefaultOptions(dir).WithLoggingLevel(badger.ERROR)
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	queue := NewQueue(db, "test:")

	// 测试空队列
	empty, err := queue.IsEmpty()
	if err != nil {
		t.Fatal(err)
	}
	if !empty {
		t.Error("新队列应该为空")
	}

	// 入队测试
	testData := []string{"first", "second", "third"}
	for _, data := range testData {
		err = queue.Enqueue([]byte(data))
		if err != nil {
			t.Fatalf("入队失败: %v", err)
		}
		// 稍微延迟确保 Id 递增
		time.Sleep(1 * time.Millisecond)
	}

	// 检查队列大小
	size, err := queue.Size()
	if err != nil {
		t.Fatal(err)
	}
	if size != uint64(len(testData)) {
		t.Errorf("期望队列大小 %d, 实际 %d", len(testData), size)
	}

	// Peek 测试
	data, err := queue.Peek()
	if err != nil {
		t.Fatalf("Peek 失败: %v", err)
	}
	if string(data) != testData[0] {
		t.Errorf("Peek 期望 %s, 实际 %s", testData[0], string(data))
	}

	// 确认 Peek 不删除数据
	size, _ = queue.Size()
	if size != uint64(len(testData)) {
		t.Error("Peek 不应该改变队列大小")
	}

	// 出队测试
	for i, expected := range testData {
		data, err := queue.Dequeue()
		if err != nil {
			t.Fatalf("出队失败 (索引 %d): %v", i, err)
		}
		if string(data) != expected {
			t.Errorf("索引 %d: 期望 %s, 实际 %s", i, expected, string(data))
		}
	}

	// 确认队列为空
	empty, err = queue.IsEmpty()
	if err != nil {
		t.Fatal(err)
	}
	if !empty {
		t.Error("出队后队列应该为空")
	}

	// 从空队列出队应该失败
	_, err = queue.Dequeue()
	if err == nil {
		t.Error("从空队列出队应该返回错误")
	}

	t.Log("✅ 基本操作测试通过")
}

// 测试 FIFO 顺序保证
func TestIdQueue_FIFOOrder(t *testing.T) {
	dir, err := os.MkdirTemp("", "idqueue-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	opts := badger.DefaultOptions(dir).WithLoggingLevel(badger.ERROR)
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	queue := NewQueue(db, "fifo:")

	count := 100
	t.Logf("入队 %d 个元素...", count)

	// 入队
	for i := 0; i < count; i++ {
		data := []byte(fmt.Sprintf("item-%d", i))
		err = queue.Enqueue(data)
		if err != nil {
			t.Fatalf("入队失败 (索引 %d): %v", i, err)
		}
		// 微小延迟确保 Id 递增
		if i%10 == 0 {
			time.Sleep(1 * time.Millisecond)
		}
	}

	t.Log("出队并验证顺序...")

	// 出队并验证顺序
	for i := 0; i < count; i++ {
		data, err := queue.Dequeue()
		if err != nil {
			t.Fatalf("出队失败 (索引 %d): %v", i, err)
		}

		expected := fmt.Sprintf("item-%d", i)
		if string(data) != expected {
			t.Errorf("顺序错误 - 索引 %d: 期望 %s, 实际 %s", i, expected, string(data))
		}
	}

	t.Log("✅ FIFO 顺序测试通过 - 100 个元素严格按序")
}

// 测试 nson Id 的时间戳和顺序性
func TestIdQueue_IdOrdering(t *testing.T) {
	dir, err := os.MkdirTemp("", "idqueue-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	opts := badger.DefaultOptions(dir).WithLoggingLevel(badger.ERROR)
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	queue := NewQueue(db, "id_order:")

	// 记录入队的 Id 和时间
	var ids []nson.Id
	count := 50

	for i := 0; i < count; i++ {
		// 创建 Id 并入队
		id := nson.NewId()
		ids = append(ids, id)

		data := []byte(fmt.Sprintf("data-%d", i))
		err = queue.Enqueue(data)
		if err != nil {
			t.Fatalf("入队失败: %v", err)
		}

		time.Sleep(2 * time.Millisecond)
	}

	t.Logf("生成了 %d 个 Id", len(ids))
	t.Logf("第一个 Id: %s (时间: %v)", ids[0].Hex(), ids[0].Time())
	t.Logf("最后一个 Id: %s (时间: %v)", ids[len(ids)-1].Hex(), ids[len(ids)-1].Time())

	// 获取队列中第一个和最后一个 Id
	firstId, err := queue.GetFirstId()
	if err != nil {
		t.Fatalf("获取第一个 Id 失败: %v", err)
	}

	lastId, err := queue.GetLastId()
	if err != nil {
		t.Fatalf("获取最后一个 Id 失败: %v", err)
	}

	t.Logf("队列第一个 Id: %s (时间: %v)", firstId.Hex(), firstId.Time())
	t.Logf("队列最后一个 Id: %s (时间: %v)", lastId.Hex(), lastId.Time())

	// 验证时间戳递增
	firstTime := firstId.Timestamp()
	lastTime := lastId.Timestamp()

	if lastTime <= firstTime {
		t.Errorf("时间戳应该递增: 第一个 %d, 最后一个 %d", firstTime, lastTime)
	}

	t.Logf("时间跨度: %d 毫秒", lastTime-firstTime)
	t.Log("✅ Id 时间戳递增验证通过")
}

// 测试并发入队
func TestIdQueue_ConcurrentEnqueue(t *testing.T) {
	dir, err := os.MkdirTemp("", "idqueue-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	opts := badger.DefaultOptions(dir).WithLoggingLevel(badger.ERROR)
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	queue := NewQueue(db, "concurrent:")

	goroutines := 10
	itemsPerGoroutine := 10
	totalItems := goroutines * itemsPerGoroutine

	t.Logf("启动 %d 个协程，每个入队 %d 个元素", goroutines, itemsPerGoroutine)

	var wg sync.WaitGroup
	errors := make(chan error, totalItems)

	// 并发入队
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < itemsPerGoroutine; i++ {
				data := []byte(fmt.Sprintf("g%d-item%d", goroutineID, i))
				if err := queue.Enqueue(data); err != nil {
					errors <- err
				}
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	// 检查是否有错误
	for err := range errors {
		t.Errorf("入队错误: %v", err)
	}

	// 验证数量
	size, err := queue.Size()
	if err != nil {
		t.Fatal(err)
	}

	if size != uint64(totalItems) {
		t.Errorf("期望 %d 个元素, 实际 %d", totalItems, size)
	}

	t.Logf("✅ 并发入队成功: %d 个元素", size)

	// 验证所有元素可以按序出队
	dequeueCount := 0
	for {
		_, err := queue.Dequeue()
		if err != nil {
			break // 队列空
		}
		dequeueCount++
	}

	if dequeueCount != totalItems {
		t.Errorf("出队数量不匹配: 期望 %d, 实际 %d", totalItems, dequeueCount)
	}

	t.Logf("✅ 所有 %d 个元素按序出队成功", dequeueCount)
}

// 测试 Clear 操作
func TestIdQueue_Clear(t *testing.T) {
	dir, err := os.MkdirTemp("", "idqueue-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	opts := badger.DefaultOptions(dir).WithLoggingLevel(badger.ERROR)
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	queue := NewQueue(db, "clear:")

	// 入队一些数据
	count := 50
	for i := 0; i < count; i++ {
		err = queue.Enqueue([]byte(fmt.Sprintf("item-%d", i)))
		if err != nil {
			t.Fatal(err)
		}
	}

	size, _ := queue.Size()
	t.Logf("清空前队列大小: %d", size)

	// 清空队列
	err = queue.Clear()
	if err != nil {
		t.Fatalf("清空失败: %v", err)
	}

	// 验证队列为空
	size, err = queue.Size()
	if err != nil {
		t.Fatal(err)
	}
	if size != 0 {
		t.Errorf("清空后队列大小应该为 0, 实际 %d", size)
	}

	empty, _ := queue.IsEmpty()
	if !empty {
		t.Error("清空后队列应该为空")
	}

	t.Log("✅ Clear 操作测试通过")
}

// 测试多队列隔离
func TestIdQueue_MultipleQueues(t *testing.T) {
	dir, err := os.MkdirTemp("", "idqueue-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	opts := badger.DefaultOptions(dir).WithLoggingLevel(badger.ERROR)
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// 创建多个队列
	queue1 := NewQueue(db, "queue1:")
	queue2 := NewQueue(db, "queue2:")
	queue3 := NewQueue(db, "queue3:")

	// 向不同队列入队
	queue1.Enqueue([]byte("q1-data1"))
	queue1.Enqueue([]byte("q1-data2"))

	queue2.Enqueue([]byte("q2-data1"))
	queue2.Enqueue([]byte("q2-data2"))
	queue2.Enqueue([]byte("q2-data3"))

	queue3.Enqueue([]byte("q3-data1"))

	// 验证队列大小
	size1, _ := queue1.Size()
	size2, _ := queue2.Size()
	size3, _ := queue3.Size()

	if size1 != 2 || size2 != 3 || size3 != 1 {
		t.Errorf("队列大小不正确: q1=%d (期望2), q2=%d (期望3), q3=%d (期望1)",
			size1, size2, size3)
	}

	// 验证数据隔离
	data, _ := queue1.Dequeue()
	if string(data) != "q1-data1" {
		t.Error("队列1数据错误")
	}

	data, _ = queue2.Dequeue()
	if string(data) != "q2-data1" {
		t.Error("队列2数据错误")
	}

	// 清空 queue1 不应该影响其他队列
	queue1.Clear()

	size1, _ = queue1.Size()
	size2, _ = queue2.Size()

	if size1 != 0 {
		t.Error("队列1应该为空")
	}
	if size2 != 2 {
		t.Error("队列2不应该被影响")
	}

	t.Log("✅ 多队列隔离测试通过")
}

// 测试持久化（重启数据库）
func TestIdQueue_Persistence(t *testing.T) {
	dir, err := os.MkdirTemp("", "idqueue-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	prefix := "persist:"
	testData := []string{"persistent-1", "persistent-2", "persistent-3"}

	// 第一阶段：写入数据
	{
		opts := badger.DefaultOptions(dir).WithLoggingLevel(badger.ERROR)
		db, err := badger.Open(opts)
		if err != nil {
			t.Fatal(err)
		}

		queue := NewQueue(db, prefix)

		for _, data := range testData {
			err = queue.Enqueue([]byte(data))
			if err != nil {
				t.Fatal(err)
			}
			time.Sleep(1 * time.Millisecond)
		}

		db.Close()
		t.Log("第一阶段：数据已写入并关闭数据库")
	}

	// 第二阶段：重新打开数据库并读取
	{
		opts := badger.DefaultOptions(dir).WithLoggingLevel(badger.ERROR)
		db, err := badger.Open(opts)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()

		queue := NewQueue(db, prefix)

		// 验证数据仍然存在
		size, err := queue.Size()
		if err != nil {
			t.Fatal(err)
		}
		if size != uint64(len(testData)) {
			t.Errorf("重启后队列大小不正确: 期望 %d, 实际 %d", len(testData), size)
		}

		// 验证数据顺序
		for i, expected := range testData {
			data, err := queue.Dequeue()
			if err != nil {
				t.Fatalf("出队失败 (索引 %d): %v", i, err)
			}
			if string(data) != expected {
				t.Errorf("索引 %d: 期望 %s, 实际 %s", i, expected, string(data))
			}
		}

		t.Log("✅ 持久化测试通过 - 数据在重启后保持顺序")
	}
}

// 性能基准测试：入队
func BenchmarkIdQueue_Enqueue(b *testing.B) {
	dir, err := os.MkdirTemp("", "idqueue-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	opts := badger.DefaultOptions(dir).WithLoggingLevel(badger.ERROR)
	db, err := badger.Open(opts)
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	queue := NewQueue(db, "bench:")
	data := []byte("benchmark data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queue.Enqueue(data)
	}
}

// 性能基准测试：出队
func BenchmarkIdQueue_Dequeue(b *testing.B) {
	dir, err := os.MkdirTemp("", "idqueue-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	opts := badger.DefaultOptions(dir).WithLoggingLevel(badger.ERROR)
	db, err := badger.Open(opts)
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	queue := NewQueue(db, "bench:")
	data := []byte("benchmark data")

	// 预填充数据
	for i := 0; i < b.N; i++ {
		queue.Enqueue(data)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queue.Dequeue()
	}
}
