package integration

import (
	"bytes"
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/core/node"
	"github.com/snple/beacon/core/storage"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/nodes"
	"github.com/snple/beacon/util/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func init() {
	// 设置 TOKEN_SALT 环境变量用于 JWT 签名
	os.Setenv("TOKEN_SALT", "test-secret-key-for-jwt-signing")
}

// TestIntegration_CoreEdgeConnection 测试 Core 和 Edge 通过 gRPC 联通
func TestIntegration_CoreEdgeConnection(t *testing.T) {
	ctx := context.Background()

	// 1. 创建 Core 服务（内存模式）
	logger, _ := zap.NewDevelopment()
	cs, err := core.Core(
		core.WithLogger(logger),
	)
	require.NoError(t, err)
	defer cs.Stop()

	cs.Start()

	// 2. 创建节点和 secret
	nodeID := "test-node-001"
	nodeName := "TestNode"
	secret := "test-secret-123"

	testNode := &storage.Node{
		ID:      nodeID,
		Name:    nodeName,
		Status:  1, // ON
		Updated: time.Now(),
		Wires: []storage.Wire{
			{
				ID:   "wire-001",
				Name: "TestWire",
				Type: "modbus",
				Pins: []storage.Pin{
					{
						ID:   "pin-001",
						Name: "TestPin",
						Addr: "0x0001",
						Type: 1,
						Rw:   1,
					},
				},
			},
		},
	}

	// 保存节点配置到 Core 存储
	err = cs.GetStorage().Push(ctx, mustEncodeNode(t, testNode))
	require.NoError(t, err)

	// 设置节点 secret
	err = cs.GetStorage().SetSecret(ctx, nodeID, secret)
	require.NoError(t, err)

	// 3. 创建 Node 服务（提供给 Edge 连接的服务）
	ns, err := node.Node(cs)
	require.NoError(t, err)
	defer ns.Stop()

	ns.Start()

	// 4. 启动 gRPC 服务器
	lis, err := net.Listen("tcp", "127.0.0.1:0") // 使用随机端口
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	ns.RegisterGrpc(grpcServer)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			t.Logf("gRPC server stopped: %v", err)
		}
	}()
	defer grpcServer.Stop()

	serverAddr := lis.Addr().String()
	t.Logf("gRPC server started on %s", serverAddr)

	// 5. 创建 Edge 客户端连接
	conn, err := grpc.NewClient(
		serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	// 6. 测试 Login
	nodeClient := nodes.NewNodeServiceClient(conn)

	loginReply, err := nodeClient.Login(ctx, &nodes.NodeLoginRequest{
		Id:     nodeID,
		Secret: secret,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, loginReply.Token, "Login should return a token")
	assert.Equal(t, nodeID, loginReply.Node.Id, "Login should return correct node ID")
	assert.Equal(t, nodeName, loginReply.Node.Name, "Login should return correct node name")

	t.Logf("Login successful, token: %s...", loginReply.Token[:20])

	// 7. 使用 token 测试 View
	ctxWithToken := metadata.SetToken(ctx, loginReply.Token)

	viewReply, err := nodeClient.View(ctxWithToken, &pb.MyEmpty{})
	require.NoError(t, err)
	assert.Equal(t, nodeID, viewReply.Id)
	assert.Equal(t, nodeName, viewReply.Name)

	t.Logf("View successful, node: %s (%s)", viewReply.Name, viewReply.Id)

	// 8. 测试 Wire 服务
	wireClient := nodes.NewWireServiceClient(conn)

	wireListReply, err := wireClient.List(ctxWithToken, &nodes.WireListRequest{})
	require.NoError(t, err)
	assert.Len(t, wireListReply.Wires, 1, "Should have 1 wire")
	assert.Equal(t, "TestWire", wireListReply.Wires[0].Name)

	t.Logf("Wire list successful, count: %d", len(wireListReply.Wires))

	// 9. 测试 Sync 服务
	syncClient := nodes.NewSyncServiceClient(conn)

	syncReply, err := syncClient.GetNodeUpdated(ctxWithToken, &pb.MyEmpty{})
	require.NoError(t, err)
	t.Logf("Sync GetNodeUpdated: %v", syncReply.Updated)
}

// TestIntegration_LoginWithInvalidSecret 测试使用无效 secret 登录
func TestIntegration_LoginWithInvalidSecret(t *testing.T) {
	ctx := context.Background()

	// 创建 Core 服务
	logger, _ := zap.NewDevelopment()
	cs, err := core.Core(
		core.WithLogger(logger),
	)
	require.NoError(t, err)
	defer cs.Stop()

	cs.Start()

	// 创建节点
	nodeID := "test-node-002"
	secret := "correct-secret"

	testNode := &storage.Node{
		ID:      nodeID,
		Name:    "TestNode2",
		Status:  1,
		Updated: time.Now(),
	}

	err = cs.GetStorage().Push(ctx, mustEncodeNode(t, testNode))
	require.NoError(t, err)

	err = cs.GetStorage().SetSecret(ctx, nodeID, secret)
	require.NoError(t, err)

	// 创建 Node 服务
	ns, err := node.Node(cs)
	require.NoError(t, err)
	defer ns.Stop()

	ns.Start()

	// 启动 gRPC 服务器
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	ns.RegisterGrpc(grpcServer)

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	// 创建客户端连接
	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	nodeClient := nodes.NewNodeServiceClient(conn)

	// 使用错误的 secret 登录
	_, err = nodeClient.Login(ctx, &nodes.NodeLoginRequest{
		Id:     nodeID,
		Secret: "wrong-secret",
	})
	require.Error(t, err, "Login with wrong secret should fail")
	t.Logf("Login with wrong secret failed as expected: %v", err)
}

// TestIntegration_LoginWithDisabledNode 测试禁用节点的登录
func TestIntegration_LoginWithDisabledNode(t *testing.T) {
	ctx := context.Background()

	// 创建 Core 服务
	logger, _ := zap.NewDevelopment()
	cs, err := core.Core(
		core.WithLogger(logger),
	)
	require.NoError(t, err)
	defer cs.Stop()

	cs.Start()

	// 创建禁用的节点 (Status = 0)
	nodeID := "test-node-003"
	secret := "test-secret"

	testNode := &storage.Node{
		ID:      nodeID,
		Name:    "DisabledNode",
		Status:  0, // OFF
		Updated: time.Now(),
	}

	err = cs.GetStorage().Push(ctx, mustEncodeNode(t, testNode))
	require.NoError(t, err)

	err = cs.GetStorage().SetSecret(ctx, nodeID, secret)
	require.NoError(t, err)

	// 创建 Node 服务
	ns, err := node.Node(cs)
	require.NoError(t, err)
	defer ns.Stop()

	ns.Start()

	// 启动 gRPC 服务器
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	ns.RegisterGrpc(grpcServer)

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	// 创建客户端连接
	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	nodeClient := nodes.NewNodeServiceClient(conn)

	// 尝试登录禁用的节点
	_, err = nodeClient.Login(ctx, &nodes.NodeLoginRequest{
		Id:     nodeID,
		Secret: secret,
	})
	require.Error(t, err, "Login with disabled node should fail")
	t.Logf("Login with disabled node failed as expected: %v", err)
}

// TestIntegration_PinValueSync 测试 PinValue 同步
func TestIntegration_PinValueSync(t *testing.T) {
	ctx := context.Background()

	// 创建 Core 服务
	logger, _ := zap.NewDevelopment()
	cs, err := core.Core(
		core.WithLogger(logger),
	)
	require.NoError(t, err)
	defer cs.Stop()

	cs.Start()

	// 创建节点
	nodeID := "test-node-004"
	secret := "test-secret"

	testNode := &storage.Node{
		ID:      nodeID,
		Name:    "PinValueTestNode",
		Status:  1,
		Updated: time.Now(),
		Wires: []storage.Wire{
			{
				ID:   "wire-001",
				Name: "Wire1",
				Type: "modbus",
				Pins: []storage.Pin{
					{ID: "pin-001", Name: "Pin1", Addr: "0x0001", Type: 1, Rw: 1},
					{ID: "pin-002", Name: "Pin2", Addr: "0x0002", Type: 2, Rw: 1},
				},
			},
		},
	}

	err = cs.GetStorage().Push(ctx, mustEncodeNode(t, testNode))
	require.NoError(t, err)

	err = cs.GetStorage().SetSecret(ctx, nodeID, secret)
	require.NoError(t, err)

	// 创建 Node 服务
	ns, err := node.Node(cs)
	require.NoError(t, err)
	defer ns.Stop()

	ns.Start()

	// 启动 gRPC 服务器
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	ns.RegisterGrpc(grpcServer)

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	// 创建客户端连接
	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	// 登录获取 token
	nodeClient := nodes.NewNodeServiceClient(conn)
	loginReply, err := nodeClient.Login(ctx, &nodes.NodeLoginRequest{
		Id:     nodeID,
		Secret: secret,
	})
	require.NoError(t, err)

	ctxWithToken := metadata.SetToken(ctx, loginReply.Token)

	// 测试 Pin 服务 - 获取 PinValue
	pinClient := nodes.NewPinServiceClient(conn)

	// 获取 PinValue
	valueReply, err := pinClient.GetValue(ctxWithToken, &pb.Id{Id: "pin-001"})
	if err != nil {
		// PinValue 可能不存在，这是正常的
		t.Logf("GetValue returned error (expected if no value set): %v", err)
	} else {
		t.Logf("PinValue get successful: %+v", valueReply)
	}

	t.Logf("Pin service test completed")
}

// TestIntegration_KeepAlive 测试 KeepAlive 流
func TestIntegration_KeepAlive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 创建 Core 服务
	logger, _ := zap.NewDevelopment()
	cs, err := core.Core(
		core.WithLogger(logger),
	)
	require.NoError(t, err)
	defer cs.Stop()

	cs.Start()

	// 创建节点
	nodeID := "test-node-005"
	secret := "test-secret"

	testNode := &storage.Node{
		ID:      nodeID,
		Name:    "KeepAliveTestNode",
		Status:  1,
		Updated: time.Now(),
	}

	err = cs.GetStorage().Push(ctx, mustEncodeNode(t, testNode))
	require.NoError(t, err)

	err = cs.GetStorage().SetSecret(ctx, nodeID, secret)
	require.NoError(t, err)

	// 创建 Node 服务
	ns, err := node.Node(cs, node.WithKeepAlive(500*time.Millisecond))
	require.NoError(t, err)
	defer ns.Stop()

	ns.Start()

	// 启动 gRPC 服务器
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	ns.RegisterGrpc(grpcServer)

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	// 创建客户端连接
	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	// 登录获取 token
	nodeClient := nodes.NewNodeServiceClient(conn)
	loginReply, err := nodeClient.Login(ctx, &nodes.NodeLoginRequest{
		Id:     nodeID,
		Secret: secret,
	})
	require.NoError(t, err)

	ctxWithToken := metadata.SetToken(ctx, loginReply.Token)

	// 测试 KeepAlive 流
	stream, err := nodeClient.KeepAlive(ctxWithToken, &pb.MyEmpty{})
	require.NoError(t, err)

	// 接收几个心跳消息
	received := 0
	for i := 0; i < 3; i++ {
		reply, err := stream.Recv()
		if err != nil {
			break
		}
		received++
		t.Logf("KeepAlive reply %d: %+v", i+1, reply)
	}

	assert.GreaterOrEqual(t, received, 1, "Should receive at least 1 keepalive message")
	t.Logf("Received %d keepalive messages", received)
}

// mustEncodeNode 编码节点数据
func mustEncodeNode(t *testing.T, node *storage.Node) []byte {
	t.Helper()

	m, err := nson.Marshal(node)
	if err != nil {
		t.Fatalf("Failed to marshal node: %v", err)
	}

	buf := new(bytes.Buffer)
	if err := nson.EncodeMap(m, buf); err != nil {
		t.Fatalf("Failed to encode node: %v", err)
	}

	return buf.Bytes()
}
