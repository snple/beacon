package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/danclive/nson-go"
	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/device"
	"github.com/snple/beacon/dt"
	"github.com/snple/beacon/edge"
	"go.uber.org/zap"
)

// setupTestEnvironment 创建测试环境：Core 和 Edge 服务
func setupTestEnvironment(t *testing.T) (*core.CoreService, *edge.EdgeService, func()) {
	t.Helper()

	// 创建测试用的 logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// 创建 Core 服务（内存模式，启用 Queen broker）
	coreService, err := core.Core(
		core.WithLogger(logger.Named("core")),
		core.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		core.WithQueenBroker(":13883", nil),                // 测试端口
		core.WithBatchNotifyInterval(100*time.Millisecond), // 加快批量发送
	)
	if err != nil {
		t.Fatalf("Failed to create core service: %v", err)
	}

	// 启动 Core 服务
	coreService.Start()

	// 等待 Core 服务完全启动
	time.Sleep(200 * time.Millisecond)

	// 创建测试节点并设置密钥
	nodeID := "test-node-001"
	secret := "test-secret-123"

	// 在 Core 中预设节点密钥（Edge 连接时需要验证）
	// 必须在 Core 启动后设置
	ctx := context.Background()
	if err := coreService.GetNode().SetSecret(ctx, nodeID, secret); err != nil {
		coreService.Stop()
		t.Fatalf("Failed to set node secret: %v", err)
	}

	t.Logf("Node secret set: %s", nodeID)

	// 创建 Edge 服务，使用简单设备模板
	edgeService, err := edge.Edge(
		edge.WithLogger(logger.Named("edge")),
		edge.WithNodeID(nodeID, secret),
		edge.WithName("Test Edge"),
		edge.WithDeviceManager(device.NewDeviceManager(device.SmartBulb)), // 使用智能灯泡模板
		edge.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		edge.WithNode(edge.NodeOptions{
			Enable:    true,
			QueenAddr: "localhost:13883",
			QueenTLS:  nil,
		}),
		edge.WithBatchNotifyInterval(100*time.Millisecond), // 加快批量发送
	)
	if err != nil {
		coreService.Stop()
		t.Fatalf("Failed to create edge service: %v", err)
	}

	// 启动 Edge 服务
	edgeService.Start()

	// 等待 Edge 连接到 Core
	time.Sleep(500 * time.Millisecond)

	// 验证连接
	if !coreService.IsNodeOnline(nodeID) {
		coreService.Stop()
		edgeService.Stop()
		t.Fatalf("Edge node is not online")
	}

	// 返回清理函数
	cleanup := func() {
		edgeService.Stop()
		coreService.Stop()
	}

	return coreService, edgeService, cleanup
}

// TestCoreEdgeConnection 测试 Core 和 Edge 之间的基本连接
func TestCoreEdgeConnection(t *testing.T) {
	coreService, edgeService, cleanup := setupTestEnvironment(t)
	defer cleanup()

	nodeID := edgeService.GetStorage().GetNodeID()

	// 验证节点在线状态
	if !coreService.IsNodeOnline(nodeID) {
		t.Errorf("Expected node to be online, but it's not")
	}

	t.Logf("✓ Core-Edge connection established successfully")
}

// TestConfigPush 测试配置推送（Edge → Core）
func TestConfigPush(t *testing.T) {
	coreService, edgeService, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	nodeID := edgeService.GetStorage().GetNodeID()

	// Edge 启动时已自动推送配置，这里等待同步完成
	time.Sleep(500 * time.Millisecond)

	// 验证 Core 端已创建并收到了配置
	node, err := coreService.GetNode().View(ctx, nodeID)
	if err != nil {
		t.Fatalf("Failed to get node from core: %v", err)
	}

	if node.ID != nodeID {
		t.Errorf("Expected node ID %s, got %s", nodeID, node.ID)
	}

	if len(node.Wires) == 0 {
		t.Errorf("Expected wires to be pushed, but got none")
	}

	// 验证 Wire 和 Pin 数据
	wires, err := coreService.GetWire().List(ctx, nodeID)
	if err != nil {
		t.Fatalf("Failed to list wires: %v", err)
	}

	if len(wires) == 0 {
		t.Fatalf("Expected at least one wire, got none")
	}

	// 验证第一个 Wire 的 Pins
	firstWire := wires[0]
	pins, err := coreService.GetPin().List(ctx, nodeID, firstWire.ID)
	if err != nil {
		t.Fatalf("Failed to list pins: %v", err)
	}

	if len(pins) == 0 {
		t.Fatalf("Expected at least one pin, got none")
	}

	t.Logf("✓ Config push successful: %d wires, %d pins in first wire", len(wires), len(pins))
}

// TestPinValueSync 测试 PinValue 同步（Edge → Core）
func TestPinValueSync(t *testing.T) {
	coreService, edgeService, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// 等待配置同步完成（setupTestEnvironment 已自动推送）
	time.Sleep(500 * time.Millisecond)

	// 在 Edge 端设置 PinValue
	edgeStorage := edgeService.GetStorage()
	testPinID := "ctrl.on" // SmartBulb 的开关 Pin (本地格式)
	testValue := nson.Bool(true)

	err := edgeStorage.SetPinValue(ctx, dt.PinValue{
		ID:      testPinID,
		Value:   testValue,
		Updated: time.Now(),
	})
	if err != nil {
		t.Fatalf("Failed to set pin value on edge: %v", err)
	}

	// 触发批量通知
	edgeService.NotifyPinValue(testPinID, testValue, false)

	// 等待同步
	time.Sleep(500 * time.Millisecond)

	// 在 Core 端验证 PinValue (Core 端存储的是完整格式: "NodeID.WireName.PinName")
	nodeID := edgeService.GetStorage().GetNodeID()
	fullPinID := nodeID + "." + testPinID
	coreValue, updated, err := coreService.GetPinValue().GetValue(ctx, fullPinID)
	if err != nil {
		t.Fatalf("Failed to get pin value from core: %v", err)
	}

	t.Logf("Core GetValue result: value=%v, updated=%v, pinID=%s", coreValue, updated, fullPinID)

	if coreValue == nil {
		t.Fatalf("Expected pin value to be synced, but got nil")
	}

	coreBool, ok := coreValue.(nson.Bool)
	if !ok {
		t.Fatalf("Expected bool value, got %T", coreValue)
	}

	if bool(coreBool) != true {
		t.Errorf("Expected value true, got %v", coreBool)
	}

	t.Logf("✓ PinValue sync successful: %s = %v", testPinID, coreValue)
}

// TestPinValueBatchSync 测试 PinValue 批量同步
func TestPinValueBatchSync(t *testing.T) {
	coreService, edgeService, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	nodeID := edgeService.GetStorage().GetNodeID()

	// 等待配置同步完成
	time.Sleep(500 * time.Millisecond)

	// 批量设置多个 PinValue
	edgeStorage := edgeService.GetStorage()

	testData := map[string]nson.Value{
		"ctrl.on":  nson.Bool(true),
		"ctrl.dim": nson.U8(80),
		"ctrl.cct": nson.U16(4000),
	}

	for pinID, value := range testData {
		if err := edgeStorage.SetPinValue(ctx, dt.PinValue{
			ID:      pinID,
			Value:   value,
			Updated: time.Now(),
		}); err != nil {
			t.Fatalf("Failed to set pin value %s: %v", pinID, err)
		}
		// 触发通知
		edgeService.NotifyPinValue(pinID, value, false)
	}

	// 等待批量同步
	time.Sleep(500 * time.Millisecond)

	// 验证所有值都已同步到 Core
	successCount := 0
	for pinID, expectedValue := range testData {
		fullPinID := nodeID + "." + pinID
		coreValue, _, err := coreService.GetPinValue().GetValue(ctx, fullPinID)
		if err != nil {
			t.Errorf("Failed to get pin value %s from core: %v", pinID, err)
			continue
		}

		if coreValue == nil {
			t.Errorf("Pin value %s not synced", pinID)
			continue
		}

		// 简单验证类型匹配
		if fmt.Sprintf("%T", coreValue) != fmt.Sprintf("%T", expectedValue) {
			t.Errorf("Pin %s: type mismatch, expected %T, got %T", pinID, expectedValue, coreValue)
			continue
		}

		successCount++
		t.Logf("✓ Pin %s synced: %v", pinID, coreValue)
	}

	if successCount != len(testData) {
		t.Errorf("Expected %d pins to sync, only %d succeeded", len(testData), successCount)
	}
}

// TestPinWriteSync 测试 PinWrite 同步（Core → Edge）
func TestPinWriteSync(t *testing.T) {
	coreService, edgeService, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	nodeID := edgeService.GetStorage().GetNodeID()

	// 等待配置同步完成
	time.Sleep(500 * time.Millisecond)

	// 在 Core 端设置 PinWrite
	testPinID := "ctrl.on"
	fullPinID := nodeID + "." + testPinID
	testValue := nson.Bool(false)

	err := coreService.GetPinWrite().SetWrite(ctx, dt.PinValue{
		ID:      fullPinID,
		Value:   testValue,
		Updated: time.Now(),
	}, false)
	if err != nil {
		t.Fatalf("Failed to set pin write on core: %v", err)
	}

	// 触发实时通知
	coreService.NotifyPinWrite(nodeID, fullPinID, testValue, true)

	// 等待同步
	time.Sleep(500 * time.Millisecond)

	// 在 Edge 端验证 PinWrite 已收到（注意：在实际系统中，设备驱动会读取 PinWrite 并更新 PinValue）
	edgeStorage := edgeService.GetStorage()
	edgeWrite, _, err := edgeStorage.GetPinWrite(testPinID)
	if err != nil {
		t.Fatalf("Failed to get pin write from edge: %v", err)
	}

	if edgeWrite == nil {
		t.Fatalf("Expected pin write to be synced, but got nil value")
	}

	edgeBool, ok := edgeWrite.(nson.Bool)
	if !ok {
		t.Fatalf("Expected bool value, got %T", edgeWrite)
	}

	if bool(edgeBool) != false {
		t.Errorf("Expected value false, got %v", edgeBool)
	}

	t.Logf("✓ PinWrite sync successful: %s = %v", testPinID, edgeWrite)
}

// TestPinWriteBatchSync 测试 PinWrite 批量同步
func TestPinWriteBatchSync(t *testing.T) {
	coreService, edgeService, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	nodeID := edgeService.GetStorage().GetNodeID()

	// 等待配置同步完成
	time.Sleep(500 * time.Millisecond)

	// 批量设置多个 PinWrite
	testData := map[string]nson.Value{
		"ctrl.on":  nson.Bool(true),
		"ctrl.dim": nson.U8(50),
	}

	for pinID, value := range testData {
		fullPinID := nodeID + "." + pinID
		if err := coreService.GetPinWrite().SetWrite(ctx, dt.PinValue{
			ID:      fullPinID,
			Value:   value,
			Updated: time.Now(),
		}, false); err != nil {
			t.Fatalf("Failed to set pin write %s: %v", pinID, err)
		}
		// 触发批量通知
		coreService.NotifyPinWrite(nodeID, fullPinID, value, false)
	}

	// 等待批量同步
	time.Sleep(500 * time.Millisecond)

	// 验证所有写入都已同步到 Edge
	edgeStorage := edgeService.GetStorage()
	successCount := 0

	for pinID, expectedValue := range testData {
		edgeWrite, _, err := edgeStorage.GetPinWrite(pinID)
		if err != nil {
			t.Errorf("Failed to get pin write %s from edge: %v", pinID, err)
			continue
		}

		if edgeWrite == nil {
			t.Errorf("Pin write %s not synced", pinID)
			continue
		}

		if fmt.Sprintf("%T", edgeWrite) != fmt.Sprintf("%T", expectedValue) {
			t.Errorf("Pin %s: type mismatch, expected %T, got %T", pinID, expectedValue, edgeWrite)
			continue
		}

		successCount++
		t.Logf("✓ Pin %s write synced: %v", pinID, edgeWrite)
	}

	if successCount != len(testData) {
		t.Errorf("Expected %d pin writes to apply, only %d succeeded", len(testData), successCount)
	}
}

// TestRealtimeVsBatchNotification 测试实时和批量通知的差异
func TestRealtimeVsBatchNotification(t *testing.T) {
	coreService, edgeService, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	nodeID := edgeService.GetStorage().GetNodeID()

	// 等待配置同步完成
	time.Sleep(500 * time.Millisecond)

	// 测试实时通知（应该立即发送）
	t.Run("Realtime", func(t *testing.T) {
		pinID := "ctrl.on"
		value := nson.Bool(true)

		// 设置并立即发送
		edgeStorage := edgeService.GetStorage()
		if err := edgeStorage.SetPinValue(ctx, dt.PinValue{
			ID:      pinID,
			Value:   value,
			Updated: time.Now(),
		}); err != nil {
			t.Fatalf("Failed to set pin value: %v", err)
		}

		// 实时模式
		if err := edgeService.NotifyPinValue(pinID, value, true); err != nil {
			t.Fatalf("Failed to notify pin value: %v", err)
		}

		// 短暂等待（实时应该很快）
		time.Sleep(100 * time.Millisecond)

		// 验证
		fullPinID := nodeID + "." + pinID
		coreValue, _, err := coreService.GetPinValue().GetValue(ctx, fullPinID)
		if err != nil || coreValue == nil {
			t.Errorf("Realtime notification failed")
		} else {
			t.Logf("✓ Realtime notification delivered in <100ms")
		}
	})

	// 测试批量通知（会等待批量间隔）
	t.Run("Batch", func(t *testing.T) {
		pinID := "ctrl.dim"
		value := nson.U8(75)

		edgeStorage := edgeService.GetStorage()
		if err := edgeStorage.SetPinValue(ctx, dt.PinValue{
			ID:      pinID,
			Value:   value,
			Updated: time.Now(),
		}); err != nil {
			t.Fatalf("Failed to set pin value: %v", err)
		}

		// 批量模式
		if err := edgeService.NotifyPinValue(pinID, value, false); err != nil {
			t.Fatalf("Failed to notify pin value: %v", err)
		}

		// 需要等待批量间隔（100ms）
		time.Sleep(200 * time.Millisecond)

		// 验证
		fullPinID := nodeID + "." + pinID
		coreValue, _, err := coreService.GetPinValue().GetValue(ctx, fullPinID)
		if err != nil || coreValue == nil {
			t.Errorf("Batch notification failed")
		} else {
			t.Logf("✓ Batch notification delivered after interval")
		}
	})
}

// TestBidirectionalSync 测试双向同步
func TestBidirectionalSync(t *testing.T) {
	coreService, edgeService, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	nodeID := edgeService.GetStorage().GetNodeID()

	// 等待配置同步完成
	time.Sleep(500 * time.Millisecond)

	// 1. Edge → Core: 上报 PinValue
	t.Log("Step 1: Edge → Core (PinValue)")
	pinID := "ctrl.dim"
	edgeValue1 := nson.U8(30)

	edgeStorage := edgeService.GetStorage()
	if err := edgeStorage.SetPinValue(ctx, dt.PinValue{
		ID:      pinID,
		Value:   edgeValue1,
		Updated: time.Now(),
	}); err != nil {
		t.Fatalf("Failed to set edge pin value: %v", err)
	}
	edgeService.NotifyPinValue(pinID, edgeValue1, true)
	time.Sleep(200 * time.Millisecond)

	// 验证 Core 收到
	fullPinID := nodeID + "." + pinID
	coreValue1, _, err := coreService.GetPinValue().GetValue(ctx, fullPinID)
	if err != nil || coreValue1 == nil {
		t.Fatalf("Core didn't receive value from edge")
	}
	t.Logf("✓ Core received: %v", coreValue1)

	// 2. Core → Edge: 下发 PinWrite
	t.Log("Step 2: Core → Edge (PinWrite)")
	coreValue2 := nson.U8(70)

	if err := coreService.GetPinWrite().SetWrite(ctx, dt.PinValue{
		ID:      fullPinID,
		Value:   coreValue2,
		Updated: time.Now(),
	}, false); err != nil {
		t.Fatalf("Failed to set core pin write: %v", err)
	}
	coreService.NotifyPinWrite(nodeID, fullPinID, coreValue2, true)
	time.Sleep(200 * time.Millisecond)

	// 验证 Edge 收到
	edgeValue2, _, err := edgeStorage.GetPinValue(pinID)
	if err != nil || edgeValue2 == nil {
		t.Fatalf("Edge didn't receive write from core")
	}
	t.Logf("✓ Edge received: %v", edgeValue2)

	// 3. Edge → Core: 再次上报更新后的值
	t.Log("Step 3: Edge → Core (Updated PinValue)")
	if err := edgeStorage.SetPinValue(ctx, dt.PinValue{
		ID:      pinID,
		Value:   coreValue2,
		Updated: time.Now(),
	}); err != nil {
		t.Fatalf("Failed to update edge pin value: %v", err)
	}
	edgeService.NotifyPinValue(pinID, coreValue2, true)
	time.Sleep(200 * time.Millisecond)

	// 最终验证
	coreValue3, _, err := coreService.GetPinValue().GetValue(ctx, fullPinID)
	if err != nil || coreValue3 == nil {
		t.Fatalf("Core didn't receive updated value")
	}

	if v, ok := coreValue3.(nson.U8); !ok || uint8(v) != 70 {
		t.Errorf("Expected final value 70, got %v", coreValue3)
	}

	t.Logf("✓ Bidirectional sync completed successfully")
}

// TestEdgeReconnection 测试 Edge 重连
func TestEdgeReconnection(t *testing.T) {
	// 创建测试环境
	logger, _ := zap.NewDevelopment()

	// 创建 Core
	coreService, err := core.Core(
		core.WithLogger(logger.Named("core")),
		core.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		core.WithQueenBroker(":13884", nil),
		core.WithBatchNotifyInterval(100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("Failed to create core: %v", err)
	}
	coreService.Start()
	defer coreService.Stop()
	time.Sleep(200 * time.Millisecond)

	// 设置节点密钥
	nodeID := "reconnect-test-001"
	secret := "reconnect-secret"
	ctx := context.Background()

	if err := coreService.GetNode().SetSecret(ctx, nodeID, secret); err != nil {
		t.Fatalf("Failed to set node secret: %v", err)
	}

	// 创建第一次 Edge 连接
	t.Log("Creating first Edge connection...")
	edgeService1, err := edge.Edge(
		edge.WithLogger(logger.Named("edge1")),
		edge.WithNodeID(nodeID, secret),
		edge.WithDeviceManager(device.NewDeviceManager(device.SmartBulb)),
		edge.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		edge.WithNode(edge.NodeOptions{
			Enable:    true,
			QueenAddr: "localhost:13884",
		}),
	)
	if err != nil {
		t.Fatalf("Failed to create edge1: %v", err)
	}

	edgeService1.Start()
	time.Sleep(500 * time.Millisecond)

	// 验证在线
	if !coreService.IsNodeOnline(nodeID) {
		t.Fatalf("Edge1 is not online")
	}
	t.Logf("✓ Edge1 connected")

	// 断开第一个连接
	t.Log("Disconnecting Edge1...")
	edgeService1.Stop()
	time.Sleep(500 * time.Millisecond)

	// 验证离线
	if coreService.IsNodeOnline(nodeID) {
		t.Errorf("Edge1 should be offline")
	}
	t.Logf("✓ Edge1 disconnected")

	// 创建第二次连接（模拟重连）
	t.Log("Creating second Edge connection (reconnect)...")
	edgeService2, err := edge.Edge(
		edge.WithLogger(logger.Named("edge2")),
		edge.WithNodeID(nodeID, secret),
		edge.WithDeviceManager(device.NewDeviceManager(device.SmartBulb)),
		edge.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		edge.WithNode(edge.NodeOptions{
			Enable:    true,
			QueenAddr: "localhost:13884",
		}),
	)
	if err != nil {
		t.Fatalf("Failed to create edge2: %v", err)
	}

	edgeService2.Start()
	defer edgeService2.Stop()
	time.Sleep(500 * time.Millisecond)

	// 验证重连成功
	if !coreService.IsNodeOnline(nodeID) {
		t.Fatalf("Edge2 is not online after reconnect")
	}
	t.Logf("✓ Edge2 reconnected successfully")

	// 验证重连后通讯正常（等待自动同步）
	time.Sleep(800 * time.Millisecond)

	node, err := coreService.GetNode().View(ctx, nodeID)
	if err != nil {
		t.Fatalf("Failed to get node after reconnect: %v", err)
	}

	if len(node.Wires) == 0 {
		t.Errorf("Expected config to be pushed after reconnect")
	}

	t.Logf("✓ Communication works after reconnection")
}

// TestMultipleEdges 测试多个 Edge 节点同时连接
func TestMultipleEdges(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// 创建 Core
	coreService, err := core.Core(
		core.WithLogger(logger.Named("core")),
		core.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		core.WithQueenBroker(":13885", nil),
	)
	if err != nil {
		t.Fatalf("Failed to create core: %v", err)
	}
	coreService.Start()
	defer coreService.Stop()
	time.Sleep(200 * time.Millisecond)

	ctx := context.Background()
	nodeCount := 3
	edges := make([]*edge.EdgeService, nodeCount)

	// 创建多个 Edge 节点
	for i := 0; i < nodeCount; i++ {
		nodeID := fmt.Sprintf("multi-edge-%03d", i)
		secret := fmt.Sprintf("secret-%03d", i)

		// 在 Core 设置节点密钥
		if err := coreService.GetNode().SetSecret(ctx, nodeID, secret); err != nil {
			t.Fatalf("Failed to set node secret %d: %v", i, err)
		}

		// 创建 Edge 服务
		edgeService, err := edge.Edge(
			edge.WithLogger(logger.Named(fmt.Sprintf("edge%d", i))),
			edge.WithNodeID(nodeID, secret),
			edge.WithDeviceManager(device.NewDeviceManager(device.SmartBulb)),
			edge.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
			edge.WithNode(edge.NodeOptions{
				Enable:    true,
				QueenAddr: "localhost:13885",
			}),
		)
		if err != nil {
			t.Fatalf("Failed to create edge %d: %v", i, err)
		}

		edges[i] = edgeService
		edgeService.Start()
	}

	// 清理
	defer func() {
		for _, e := range edges {
			if e != nil {
				e.Stop()
			}
		}
	}()

	// 等待所有连接建立
	time.Sleep(time.Second)

	// 验证所有节点都在线
	onlineCount := 0
	for i := 0; i < nodeCount; i++ {
		nodeID := fmt.Sprintf("multi-edge-%03d", i)
		if coreService.IsNodeOnline(nodeID) {
			onlineCount++
			t.Logf("✓ Edge %d is online", i)
		} else {
			t.Errorf("Edge %d is not online", i)
		}
	}

	if onlineCount != nodeCount {
		t.Errorf("Expected %d nodes online, got %d", nodeCount, onlineCount)
	}

	// 等待所有节点自动推送配置
	time.Sleep(1000 * time.Millisecond)

	// 验证所有配置都收到了
	for i := 0; i < nodeCount; i++ {
		nodeID := fmt.Sprintf("multi-edge-%03d", i)
		node, err := coreService.GetNode().View(ctx, nodeID)
		if err != nil {
			t.Errorf("Failed to get node %d: %v", i, err)
			continue
		}

		if len(node.Wires) == 0 {
			t.Errorf("Node %d has no wires", i)
		} else {
			t.Logf("✓ Edge %d config synced: %d wires", i, len(node.Wires))
		}
	}

	t.Logf("✓ All %d edges connected and communicating", nodeCount)
}

// TestPinWriteFullSync 测试 Edge 重连后全量同步 PinWrite
func TestPinWriteFullSync(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// 创建 Core
	coreService, err := core.Core(
		core.WithLogger(logger.Named("core")),
		core.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		core.WithQueenBroker(":13886", nil),
	)
	if err != nil {
		t.Fatalf("Failed to create core: %v", err)
	}
	coreService.Start()
	defer coreService.Stop()
	time.Sleep(200 * time.Millisecond)

	nodeID := "fullsync-test-001"
	secret := "fullsync-secret"
	ctx := context.Background()

	if err := coreService.GetNode().SetSecret(ctx, nodeID, secret); err != nil {
		t.Fatalf("Failed to set node secret: %v", err)
	}

	// 创建第一次 Edge 连接并推送配置
	t.Log("First connection: pushing config...")
	edgeService1, err := edge.Edge(
		edge.WithLogger(logger.Named("edge1")),
		edge.WithNodeID(nodeID, secret),
		edge.WithDeviceManager(device.NewDeviceManager(device.SmartBulb)),
		edge.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		edge.WithNode(edge.NodeOptions{
			Enable:    true,
			QueenAddr: "localhost:13886",
		}),
	)
	if err != nil {
		t.Fatalf("Failed to create edge1: %v", err)
	}

	edgeService1.Start()
	time.Sleep(800 * time.Millisecond) // 等待自动推送完成

	// 在 Core 端设置多个 PinWrite（在 Edge 离线时）
	t.Log("Setting PinWrites while Edge is offline...")
	edgeService1.Stop()
	time.Sleep(300 * time.Millisecond)

	testWrites := map[string]nson.Value{
		"ctrl.on":  nson.Bool(true),
		"ctrl.dim": nson.U8(60),
		"ctrl.cct": nson.U16(3500),
	}

	for pinID, value := range testWrites {
		fullPinID := nodeID + "." + pinID
		if err := coreService.GetPinWrite().SetWrite(ctx, dt.PinValue{
			ID:      fullPinID,
			Value:   value,
			Updated: time.Now(),
		}, false); err != nil {
			t.Fatalf("Failed to set pin write %s: %v", pinID, err)
		}
		t.Logf("Set PinWrite: %s = %v", pinID, value)
	}

	// 创建第二次连接（应该触发全量同步）
	t.Log("Reconnecting Edge (should trigger full sync)...")
	edgeService2, err := edge.Edge(
		edge.WithLogger(logger.Named("edge2")),
		edge.WithNodeID(nodeID, secret),
		edge.WithDeviceManager(device.NewDeviceManager(device.SmartBulb)),
		edge.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		edge.WithNode(edge.NodeOptions{
			Enable:    true,
			QueenAddr: "localhost:13886",
		}),
	)
	if err != nil {
		t.Fatalf("Failed to create edge2: %v", err)
	}

	edgeService2.Start()
	defer edgeService2.Stop()
	time.Sleep(time.Second) // 等待全量同步完成

	// 验证所有 PinWrite 都已同步到 Edge
	edgeStorage := edgeService2.GetStorage()
	successCount := 0

	for pinID, expectedValue := range testWrites {
		edgeWrite, _, err := edgeStorage.GetPinWrite(pinID)
		if err != nil {
			t.Errorf("Failed to get pin write %s: %v", pinID, err)
			continue
		}

		if edgeWrite == nil {
			t.Errorf("Pin write %s not synced", pinID)
			continue
		}

		if fmt.Sprintf("%T", edgeWrite) != fmt.Sprintf("%T", expectedValue) {
			t.Errorf("Pin %s: type mismatch", pinID)
			continue
		}

		successCount++
		t.Logf("✓ Pin %s synced after reconnect: %v", pinID, edgeWrite)
	}

	if successCount != len(testWrites) {
		t.Errorf("Expected %d pin writes to sync, only %d succeeded", len(testWrites), successCount)
	} else {
		t.Logf("✓ Full PinWrite sync successful: %d/%d pins", successCount, len(testWrites))
	}
}
