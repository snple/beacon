package storage

import (
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

// setupManagedDB 创建一个 managed 模式的内存数据库用于测试
func setupManagedDB(t *testing.T) *badger.DB {
	opts := badger.DefaultOptions("").WithInMemory(true)
	db, err := badger.OpenManaged(opts)
	if err != nil {
		t.Fatalf("failed to open managed database: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	return db
}

// TestManagedMode_WriteOrder 测试 managed 模式下写入数据的顺序
// 验证：使用不同的 commitTs 写入数据后，迭代器返回的数据顺序
func TestManagedMode_WriteOrder(t *testing.T) {
	db := setupManagedDB(t)

	// 使用不同的 commitTs 写入数据（乱序写入）
	// 模拟真实场景：数据可能以任意顺序写入，但我们想按时间戳读取
	testData := []struct {
		key      string
		value    string
		commitTs uint64
	}{
		{"pw:node1:pin1", "value1", 100},
		{"pw:node1:pin2", "value2", 300}, // 时间戳较大，但先写入
		{"pw:node1:pin3", "value3", 200},
		{"pw:node1:pin4", "value4", 500},
		{"pw:node1:pin5", "value5", 400},
	}

	// 使用 NewTransactionAt + CommitAt 写入数据
	for _, td := range testData {
		txn := db.NewTransactionAt(td.commitTs, true)
		err := txn.Set([]byte(td.key), []byte(td.value))
		require.NoError(t, err)
		err = txn.CommitAt(td.commitTs, nil)
		require.NoError(t, err)
	}

	t.Log("=== 数据写入完成 ===")
	t.Log("写入顺序: pin1(ts=100), pin2(ts=300), pin3(ts=200), pin4(ts=500), pin5(ts=400)")

	// 测试1：使用最大时间戳读取所有数据，检查迭代顺序
	t.Log("\n=== 测试1: 读取所有数据（readTs=1000），检查迭代顺序 ===")
	readTxn := db.NewTransactionAt(1000, false)
	defer readTxn.Discard()

	it := readTxn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	prefix := []byte("pw:node1:")
	t.Log("迭代器返回顺序:")
	var keys []string
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		key := string(item.Key())
		version := item.Version()
		val, _ := item.ValueCopy(nil)
		t.Logf("  Key: %s, Version(commitTs): %d, Value: %s", key, version, string(val))
		keys = append(keys, key)
	}

	// 结论：迭代器是按 key 的字典序返回的，不是按 commitTs
	t.Log("\n结论: 迭代器按 KEY 字典序返回，不是按 commitTs 排序")
}

// TestManagedMode_ReadAtTimestamp 测试使用特定时间戳读取数据
// 验证：NewTransactionAt(readTs) 是否只能看到 commitTs <= readTs 的数据
func TestManagedMode_ReadAtTimestamp(t *testing.T) {
	db := setupManagedDB(t)

	// 写入相同 key 的多个版本
	key := []byte("pw:node1:pin1")

	// 版本1: ts=100
	txn1 := db.NewTransactionAt(100, true)
	require.NoError(t, txn1.Set(key, []byte("value_v1_ts100")))
	require.NoError(t, txn1.CommitAt(100, nil))

	// 版本2: ts=200
	txn2 := db.NewTransactionAt(200, true)
	require.NoError(t, txn2.Set(key, []byte("value_v2_ts200")))
	require.NoError(t, txn2.CommitAt(200, nil))

	// 版本3: ts=300
	txn3 := db.NewTransactionAt(300, true)
	require.NoError(t, txn3.Set(key, []byte("value_v3_ts300")))
	require.NoError(t, txn3.CommitAt(300, nil))

	t.Log("=== 同一个 key 写入了 3 个版本 ===")
	t.Log("版本1: ts=100, value=value_v1_ts100")
	t.Log("版本2: ts=200, value=value_v2_ts200")
	t.Log("版本3: ts=300, value=value_v3_ts300")

	// 测试不同 readTs 读取的结果
	testCases := []struct {
		readTs        uint64
		expectedValue string
		shouldExist   bool
	}{
		{50, "", false},                // ts=50 < 所有版本，看不到数据
		{100, "value_v1_ts100", true},  // ts=100 能看到版本1
		{150, "value_v1_ts100", true},  // ts=150 能看到版本1（最新的 <= 150）
		{200, "value_v2_ts200", true},  // ts=200 能看到版本2
		{250, "value_v2_ts200", true},  // ts=250 能看到版本2
		{300, "value_v3_ts300", true},  // ts=300 能看到版本3
		{1000, "value_v3_ts300", true}, // ts=1000 能看到版本3（最新）
	}

	t.Log("\n=== 测试不同 readTs 读取的结果 ===")
	for _, tc := range testCases {
		txn := db.NewTransactionAt(tc.readTs, false)
		item, err := txn.Get(key)

		if tc.shouldExist {
			require.NoError(t, err, "readTs=%d should see data", tc.readTs)
			val, _ := item.ValueCopy(nil)
			version := item.Version()
			t.Logf("readTs=%d: Value=%s, Version=%d", tc.readTs, string(val), version)
			require.Equal(t, tc.expectedValue, string(val))
		} else {
			require.Error(t, err, "readTs=%d should NOT see data", tc.readTs)
			t.Logf("readTs=%d: 数据不可见 (key not found)", tc.readTs)
		}
		txn.Discard()
	}

	t.Log("\n结论: NewTransactionAt(readTs) 只能看到 commitTs <= readTs 的最新版本")
}

// TestManagedMode_FilterByTimestamp 测试是否能通过时间戳过滤数据
// 这是核心测试：验证 managed 模式是否能优化 ListPinWrites 的 after 过滤
func TestManagedMode_FilterByTimestamp(t *testing.T) {
	db := setupManagedDB(t)

	// 模拟 ListPinWrites 场景：多个 pin 有不同的更新时间
	pins := []struct {
		pinID    string
		commitTs uint64
	}{
		{"pin1", 100},
		{"pin2", 200},
		{"pin3", 300},
		{"pin4", 400},
		{"pin5", 500},
	}

	for _, p := range pins {
		key := []byte(fmt.Sprintf("pw:node1:%s", p.pinID))
		txn := db.NewTransactionAt(p.commitTs, true)
		require.NoError(t, txn.Set(key, []byte(fmt.Sprintf("value_%s", p.pinID))))
		require.NoError(t, txn.CommitAt(p.commitTs, nil))
	}

	t.Log("=== 写入 5 个 pin 数据，不同 commitTs ===")
	t.Log("pin1(ts=100), pin2(ts=200), pin3(ts=300), pin4(ts=400), pin5(ts=500)")

	// 问题：我想找 commitTs > 250 的数据（即 pin3, pin4, pin5）
	t.Log("\n=== 测试: 能否只读取 ts > 250 的数据？===")

	// 方法1：使用 SinceTs 选项过滤
	t.Log("\n方法1: 使用 SinceTs=250 迭代器（只读取 version > 250 的数据）")
	txn := db.NewTransactionAt(1000, false)
	opts := badger.DefaultIteratorOptions
	opts.SinceTs = 250 // 只读取 version > 250 的数据
	it := txn.NewIterator(opts)

	prefix := []byte("pw:node1:")
	var filteredCount int
	t.Log("SinceTs=250 模式下的迭代结果:")
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		version := item.Version()
		val, _ := item.ValueCopy(nil)
		t.Logf("  Key=%s, Version=%d, Value=%s", string(item.Key()), version, string(val))
		filteredCount++
	}
	it.Close()
	txn.Discard()
	t.Logf("方法1结果: 找到 %d 条符合条件的数据 (预期 3 条: pin3, pin4, pin5)", filteredCount)

	// 方法2：使用 AllVersions + SinceTs
	t.Log("\n方法2: 使用 AllVersions=true + SinceTs=250 迭代器")
	txn2 := db.NewTransactionAt(1000, false)
	opts2 := badger.DefaultIteratorOptions
	opts2.AllVersions = true
	opts2.SinceTs = 250
	it2 := txn2.NewIterator(opts2)

	t.Log("AllVersions + SinceTs=250 模式下的迭代结果:")
	for it2.Seek(prefix); it2.ValidForPrefix(prefix); it2.Next() {
		item := it2.Item()
		val, _ := item.ValueCopy(nil)
		t.Logf("  Key=%s, Version=%d, Value=%s", string(item.Key()), item.Version(), string(val))
	}
	it2.Close()
	txn2.Discard()

	t.Log("\n=== 结论 ===")
	t.Log("1. 使用 SinceTs 可以只返回 version > SinceTs 的数据")
	t.Log("2. 迭代器仍然按 KEY 字典序返回，不是按时间戳排序")
	t.Log("3. 如果需要按时间排序，仍需手动排序")
}

// TestManagedMode_VersionOrder 测试迭代器返回数据的 Version 顺序
func TestManagedMode_VersionOrder(t *testing.T) {
	db := setupManagedDB(t)

	// 写入多个 key，commitTs 顺序与 key 顺序不同
	testData := []struct {
		key      string
		commitTs uint64
	}{
		{"pw:node1:a", 500}, // key 字典序最小，但 ts 最大
		{"pw:node1:b", 100}, // key 字典序中间，ts 最小
		{"pw:node1:c", 300}, // key 字典序最大，ts 中间
	}

	for _, td := range testData {
		txn := db.NewTransactionAt(td.commitTs, true)
		require.NoError(t, txn.Set([]byte(td.key), []byte(fmt.Sprintf("ts=%d", td.commitTs))))
		require.NoError(t, txn.CommitAt(td.commitTs, nil))
	}

	t.Log("=== 写入数据 ===")
	t.Log("key=a (ts=500), key=b (ts=100), key=c (ts=300)")

	t.Log("\n=== 迭代器返回顺序 ===")
	txn := db.NewTransactionAt(1000, false)
	defer txn.Discard()

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	var versions []uint64
	prefix := []byte("pw:node1:")
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		val, _ := item.ValueCopy(nil)
		t.Logf("  Key=%s, Version=%d, Value=%s", string(item.Key()), item.Version(), string(val))
		versions = append(versions, item.Version())
	}

	t.Log("\n=== Version 顺序分析 ===")
	t.Logf("返回的 Version 顺序: %v", versions)

	// 检查是否需要排序
	needSort := false
	for i := 1; i < len(versions); i++ {
		if versions[i] < versions[i-1] {
			needSort = true
			break
		}
	}

	if needSort {
		t.Log("结论: Version 不是有序的，如果需要按时间排序，必须手动排序！")
	} else {
		t.Log("结论: Version 恰好是有序的（但这是巧合，不是保证）")
	}
}

// TestManagedMode_OptimizationStrategy 测试优化策略
// 演示如何利用 managed 模式 + SinceTs 优化 ListPinWrites
func TestManagedMode_OptimizationStrategy(t *testing.T) {
	db := setupManagedDB(t)

	// 模拟场景：有 20 个 pin，我们只想获取最近更新的
	numPins := 20
	for i := 1; i <= numPins; i++ {
		key := []byte(fmt.Sprintf("pw:node1:pin%03d", i))
		commitTs := uint64(i * 100) // ts: 100, 200, 300, ...
		txn := db.NewTransactionAt(commitTs, true)
		require.NoError(t, txn.Set(key, []byte(fmt.Sprintf("value_%d", i))))
		require.NoError(t, txn.CommitAt(commitTs, nil))
	}

	t.Logf("=== 写入 %d 个 pin，ts 范围 100-%d ===", numPins, numPins*100)

	// 模拟 ListPinWrites(nodeID, after=time.Unix(0,1500), limit=5)
	// 只想获取 ts > 1500 的数据
	afterTs := uint64(1500)
	limit := 5

	t.Logf("\n=== 查询: ts > %d, limit=%d ===", afterTs, limit)

	// 使用 SinceTs 优化查询
	txn := db.NewTransactionAt(uint64(numPins*100+100), false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.SinceTs = afterTs // 只读取 version > afterTs 的数据
	it := txn.NewIterator(opts)
	defer it.Close()

	prefix := []byte("pw:node1:")
	var results []struct {
		key     string
		version uint64
	}

	scannedCount := 0
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		scannedCount++
		item := it.Item()
		version := item.Version()

		results = append(results, struct {
			key     string
			version uint64
		}{string(item.Key()), version})

		if len(results) >= limit {
			break
		}
	}

	t.Logf("扫描了 %d 条记录（使用 SinceTs=%d 过滤后）", scannedCount, afterTs)
	t.Logf("符合条件的结果 (%d 条):", len(results))
	for _, r := range results {
		t.Logf("  Key=%s, Version=%d", r.key, r.version)
	}

	t.Log("\n=== 优化分析 ===")
	t.Log("优点:")
	t.Log("  1. SinceTs 可以跳过旧数据，避免扫描所有记录")
	t.Log("  2. 可用 item.Version() 直接获取 commitTs，无需解码 value")
	t.Log("  3. 将 updated 时间戳作为 commitTs，可省去 Updated 字段的解码")
	t.Log("")
	t.Log("缺点:")
	t.Log("  1. 迭代器按 key 排序，不是按 ts 排序")
	t.Log("  2. 如果需要按时间排序，仍需手动排序")
	t.Log("")
	t.Log("结论:")
	t.Log("  managed 模式 + SinceTs 可以有效优化 ListPinWrites")
}
