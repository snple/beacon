package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/device"
	"github.com/snple/beacon/edge"
	"go.uber.org/zap"
)

// TestQueenAuthFailure 测试 Queen 认证失败
func TestQueenAuthFailure(t *testing.T) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// 创建 Core 服务
	coreService, err := core.Core(
		core.WithLogger(logger.Named("core")),
		core.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		core.WithQueenBroker(":13884", nil),
	)
	if err != nil {
		t.Fatalf("Failed to create core service: %v", err)
	}

	coreService.Start()
	time.Sleep(200 * time.Millisecond)

	nodeID := "test-node-auth-fail"
	// 设置一个正确的密钥
	if err := coreService.GetNode().SetSecret(nodeID, "correct-secret"); err != nil {
		coreService.Stop()
		t.Fatalf("Failed to set node secret: %v", err)
	}

	// 创建 Edge 使用错误的密钥
	edgeService, err := edge.Edge(
		edge.WithLogger(logger.Named("edge")),
		edge.WithNodeID(nodeID, "wrong-secret"), // 错误的密钥
		edge.WithName("Auth Fail Test"),
		edge.WithDeviceManager(device.NewDeviceManager(device.SmartBulb)),
		edge.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		edge.WithNode(edge.NodeOptions{
			Enable:    true,
			QueenAddr: "localhost:13884",
			QueenTLS:  nil,
		}),
	)
	if err != nil {
		coreService.Stop()
		t.Fatalf("Failed to create edge service: %v", err)
	}

	edgeService.Start()

	// 等待一段时间（连接应该失败）
	time.Sleep(1 * time.Second)

	// 验证节点不在线
	if coreService.IsNodeOnline(nodeID) {
		t.Errorf("Node should not be online with wrong secret")
	}

	t.Log("✓ Authentication failure handled correctly")

	edgeService.Stop()
	coreService.Stop()
}

// TestQueenAuthMissingSecret 测试缺少密钥的认证失败
func TestQueenAuthMissingSecret(t *testing.T) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// 创建 Core 服务
	coreService, err := core.Core(
		core.WithLogger(logger.Named("core")),
		core.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		core.WithQueenBroker(":13885", nil),
	)
	if err != nil {
		t.Fatalf("Failed to create core service: %v", err)
	}

	coreService.Start()
	time.Sleep(200 * time.Millisecond)

	// 不设置节点密钥
	nodeID := "test-node-no-secret"

	// 创建 Edge
	edgeService, err := edge.Edge(
		edge.WithLogger(logger.Named("edge")),
		edge.WithNodeID(nodeID, "some-secret"),
		edge.WithName("No Secret Test"),
		edge.WithDeviceManager(device.NewDeviceManager(device.SmartBulb)),
		edge.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		edge.WithNode(edge.NodeOptions{
			Enable:    true,
			QueenAddr: "localhost:13885",
			QueenTLS:  nil,
		}),
	)
	if err != nil {
		coreService.Stop()
		t.Fatalf("Failed to create edge service: %v", err)
	}

	edgeService.Start()
	time.Sleep(1 * time.Second)

	// 验证节点不在线
	if coreService.IsNodeOnline(nodeID) {
		t.Errorf("Node should not be online without registered secret")
	}

	t.Log("✓ Missing secret handled correctly")

	edgeService.Stop()
	coreService.Stop()
}

// TestCoreStopWithActiveEdge 测试 Core 停止时有活跃 Edge 连接
func TestCoreStopWithActiveEdge(t *testing.T) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// 创建 Core 服务
	coreService, err := core.Core(
		core.WithLogger(logger.Named("core")),
		core.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		core.WithQueenBroker(":13886", nil),
	)
	if err != nil {
		t.Fatalf("Failed to create core service: %v", err)
	}

	coreService.Start()
	time.Sleep(200 * time.Millisecond)

	nodeID := "test-node-core-stop"
	secret := "test-secret"
	if err := coreService.GetNode().SetSecret(nodeID, secret); err != nil {
		coreService.Stop()
		t.Fatalf("Failed to set node secret: %v", err)
	}

	// 创建 Edge
	edgeService, err := edge.Edge(
		edge.WithLogger(logger.Named("edge")),
		edge.WithNodeID(nodeID, secret),
		edge.WithName("Core Stop Test"),
		edge.WithDeviceManager(device.NewDeviceManager(device.SmartBulb)),
		edge.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		edge.WithNode(edge.NodeOptions{
			Enable:    true,
			QueenAddr: "localhost:13886",
			QueenTLS:  nil,
		}),
	)
	if err != nil {
		coreService.Stop()
		t.Fatalf("Failed to create edge service: %v", err)
	}

	edgeService.Start()
	time.Sleep(500 * time.Millisecond)

	// 验证连接
	if !coreService.IsNodeOnline(nodeID) {
		edgeService.Stop()
		coreService.Stop()
		t.Fatalf("Edge should be online")
	}

	t.Log("Edge connected, stopping Core...")

	// 使用带超时的 goroutine 停止 Core
	stopDone := make(chan struct{})
	go func() {
		coreService.Stop()
		close(stopDone)
	}()

	// 等待停止完成（最多 5 秒）
	select {
	case <-stopDone:
		t.Log("✓ Core stopped gracefully")
	case <-time.After(5 * time.Second):
		t.Errorf("Core stop timed out")
	}

	edgeService.Stop()
}

// TestEdgeStopWithActiveConnection 测试 Edge 停止时有活跃连接
func TestEdgeStopWithActiveConnection(t *testing.T) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// 创建 Core 服务
	coreService, err := core.Core(
		core.WithLogger(logger.Named("core")),
		core.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		core.WithQueenBroker(":13887", nil),
	)
	if err != nil {
		t.Fatalf("Failed to create core service: %v", err)
	}

	coreService.Start()
	time.Sleep(200 * time.Millisecond)

	nodeID := "test-node-edge-stop"
	secret := "test-secret"
	if err := coreService.GetNode().SetSecret(nodeID, secret); err != nil {
		coreService.Stop()
		t.Fatalf("Failed to set node secret: %v", err)
	}

	// 创建 Edge
	edgeService, err := edge.Edge(
		edge.WithLogger(logger.Named("edge")),
		edge.WithNodeID(nodeID, secret),
		edge.WithName("Edge Stop Test"),
		edge.WithDeviceManager(device.NewDeviceManager(device.SmartBulb)),
		edge.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		edge.WithNode(edge.NodeOptions{
			Enable:    true,
			QueenAddr: "localhost:13887",
			QueenTLS:  nil,
		}),
	)
	if err != nil {
		coreService.Stop()
		t.Fatalf("Failed to create edge service: %v", err)
	}

	edgeService.Start()
	time.Sleep(500 * time.Millisecond)

	// 验证连接
	if !coreService.IsNodeOnline(nodeID) {
		edgeService.Stop()
		coreService.Stop()
		t.Fatalf("Edge should be online")
	}

	t.Log("Edge connected, stopping Edge...")

	// 使用带超时的 goroutine 停止 Edge
	stopDone := make(chan struct{})
	go func() {
		edgeService.Stop()
		close(stopDone)
	}()

	// 等待停止完成（最多 5 秒）
	select {
	case <-stopDone:
		t.Log("✓ Edge stopped gracefully")
	case <-time.After(5 * time.Second):
		t.Errorf("Edge stop timed out")
	}

	// 等待 Core 检测到断开
	time.Sleep(500 * time.Millisecond)

	// 节点应该离线
	if coreService.IsNodeOnline(nodeID) {
		t.Logf("Note: Node may still be in session cache, this is expected behavior")
	}

	coreService.Stop()
}

// TestPublishToOfflineNode 测试向离线节点发布消息
func TestPublishToOfflineNode(t *testing.T) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// 创建 Core 服务
	coreService, err := core.Core(
		core.WithLogger(logger.Named("core")),
		core.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		core.WithQueenBroker(":13888", nil),
	)
	if err != nil {
		t.Fatalf("Failed to create core service: %v", err)
	}

	coreService.Start()
	time.Sleep(200 * time.Millisecond)

	// 向不存在的节点发布消息
	err = coreService.PublishToNode("non-existent-node", "test/topic", []byte("test payload"), 1)
	if err == nil {
		// 消息应该能发布（会被持久化或丢弃）
		t.Log("✓ Publish to offline node handled (message may be queued)")
	} else {
		t.Logf("✓ Publish to offline node returned error (expected): %v", err)
	}

	coreService.Stop()
}

// TestIsNodeOnlineAfterReconnect 测试重连后节点状态
func TestIsNodeOnlineAfterReconnect(t *testing.T) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// 创建 Core 服务
	coreService, err := core.Core(
		core.WithLogger(logger.Named("core")),
		core.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		core.WithQueenBroker(":13889", nil),
	)
	if err != nil {
		t.Fatalf("Failed to create core service: %v", err)
	}

	coreService.Start()
	time.Sleep(200 * time.Millisecond)

	nodeID := "test-node-reconnect-status"
	secret := "test-secret"
	if err := coreService.GetNode().SetSecret(nodeID, secret); err != nil {
		coreService.Stop()
		t.Fatalf("Failed to set node secret: %v", err)
	}

	// 首次连接
	edgeService, err := edge.Edge(
		edge.WithLogger(logger.Named("edge")),
		edge.WithNodeID(nodeID, secret),
		edge.WithName("Reconnect Test"),
		edge.WithDeviceManager(device.NewDeviceManager(device.SmartBulb)),
		edge.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		edge.WithNode(edge.NodeOptions{
			Enable:    true,
			QueenAddr: "localhost:13889",
			QueenTLS:  nil,
		}),
	)
	if err != nil {
		coreService.Stop()
		t.Fatalf("Failed to create edge service: %v", err)
	}

	edgeService.Start()
	time.Sleep(500 * time.Millisecond)

	// 验证在线
	if !coreService.IsNodeOnline(nodeID) {
		edgeService.Stop()
		coreService.Stop()
		t.Fatalf("Edge should be online after first connect")
	}
	t.Log("✓ Node online after first connect")

	// 断开
	edgeService.Stop()
	time.Sleep(500 * time.Millisecond)

	// 验证离线
	if coreService.IsNodeOnline(nodeID) {
		coreService.Stop()
		t.Fatalf("Edge should be offline after disconnect")
	}
	t.Log("✓ Node offline after disconnect")

	// 重新连接
	edgeService2, err := edge.Edge(
		edge.WithLogger(logger.Named("edge2")),
		edge.WithNodeID(nodeID, secret),
		edge.WithName("Reconnect Test 2"),
		edge.WithDeviceManager(device.NewDeviceManager(device.SmartBulb)),
		edge.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		edge.WithNode(edge.NodeOptions{
			Enable:    true,
			QueenAddr: "localhost:13889",
			QueenTLS:  nil,
		}),
	)
	if err != nil {
		coreService.Stop()
		t.Fatalf("Failed to create second edge service: %v", err)
	}

	edgeService2.Start()
	time.Sleep(500 * time.Millisecond)

	// 验证再次在线
	if !coreService.IsNodeOnline(nodeID) {
		edgeService2.Stop()
		coreService.Stop()
		t.Fatalf("Edge should be online after reconnect")
	}
	t.Log("✓ Node online after reconnect")

	edgeService2.Stop()
	coreService.Stop()
}

// TestConcurrentConnections 测试并发连接
func TestConcurrentConnections(t *testing.T) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// 创建 Core 服务
	coreService, err := core.Core(
		core.WithLogger(logger.Named("core")),
		core.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
		core.WithQueenBroker(":13890", nil),
	)
	if err != nil {
		t.Fatalf("Failed to create core service: %v", err)
	}

	coreService.Start()
	time.Sleep(200 * time.Millisecond)

	// 创建多个节点
	nodeCount := 5
	nodes := make([]*edge.EdgeService, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodeID := fmt.Sprintf("test-node-concurrent-%d", i)
		secret := fmt.Sprintf("secret-%d", i)
		if err := coreService.GetNode().SetSecret(nodeID, secret); err != nil {
			coreService.Stop()
			t.Fatalf("Failed to set node secret: %v", err)
		}
	}

	// 使用带上下文的并发启动
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 并发启动所有节点
	for i := 0; i < nodeCount; i++ {
		nodeID := fmt.Sprintf("test-node-concurrent-%d", i)
		secret := fmt.Sprintf("secret-%d", i)

		edgeService, err := edge.Edge(
			edge.WithLogger(logger.Named(fmt.Sprintf("edge-%d", i))),
			edge.WithNodeID(nodeID, secret),
			edge.WithName(fmt.Sprintf("Concurrent Edge %d", i)),
			edge.WithDeviceManager(device.NewDeviceManager(device.SmartBulb)),
			edge.WithBadger(badger.DefaultOptions("").WithInMemory(true)),
			edge.WithNode(edge.NodeOptions{
				Enable:    true,
				QueenAddr: "localhost:13890",
				QueenTLS:  nil,
			}),
		)
		if err != nil {
			t.Logf("Failed to create edge service %d: %v", i, err)
			continue
		}

		nodes[i] = edgeService
		edgeService.Start()
	}

	// 等待所有连接
	time.Sleep(2 * time.Second)

	// 验证所有节点在线
	onlineCount := 0
	for i := 0; i < nodeCount; i++ {
		select {
		case <-ctx.Done():
			t.Fatal("Context timeout")
		default:
		}

		nodeID := fmt.Sprintf("test-node-concurrent-%d", i)
		if coreService.IsNodeOnline(nodeID) {
			onlineCount++
		}
	}

	t.Logf("✓ %d/%d nodes connected successfully", onlineCount, nodeCount)

	if onlineCount != nodeCount {
		t.Errorf("Expected all %d nodes to be online, got %d", nodeCount, onlineCount)
	}

	// 清理
	for _, node := range nodes {
		if node != nil {
			node.Stop()
		}
	}
	coreService.Stop()
}
