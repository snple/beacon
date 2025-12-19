package integration

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/device"
	"github.com/snple/beacon/dt"
	"github.com/snple/beacon/edge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestQueenCommunication_Basic 测试基本的 Queen 通信
func TestQueenCommunication_Basic(t *testing.T) {
	ctx := context.Background()

	// 1. 创建 Core 服务，启用 Queen Broker
	logger, _ := zap.NewDevelopment()
	cs, err := core.Core(
		core.WithLogger(logger),
		core.WithQueenBroker("127.0.0.1:13883", nil),
	)
	require.NoError(t, err)
	defer cs.Stop()

	cs.Start()

	time.Sleep(500 * time.Millisecond)

	// 2. 创建测试节点
	nodeID := "test-queen-node-001"
	nodeName := "TestQueenNode"
	secret := "test-queen-secret-123"

	testNode := &dt.Node{
		ID:      nodeID,
		Name:    nodeName,
		Updated: time.Now(),
	}

	err = cs.GetStorage().Push(ctx, mustEncodeNode(t, testNode))
	require.NoError(t, err)

	err = cs.GetStorage().SetSecret(ctx, nodeID, secret)
	require.NoError(t, err)

	// 3. 创建 Edge 服务，使用 Queen 通信和设备模板
	testDevice := device.Device{
		ID:   "test_device_queen",
		Name: "Test Device",
		Wires: []device.Wire{
			{
				Name: "w1",
				Pins: []device.Pin{
					{Name: "p1", Type: uint32(nson.DataTypeBOOL), Rw: device.RW},
				},
			},
		},
	}

	edgeLogger, _ := zap.NewDevelopment()
	es, err := edge.Edge(
		edge.WithLogger(edgeLogger),
		edge.WithNodeID(nodeID, secret),
		edge.WithDeviceTemplate(testDevice),
		edge.WithNode(edge.NodeOptions{
			Enable:    true,
			QueenAddr: "127.0.0.1:13883",
			QueenTLS:  nil,
		}),
	)
	require.NoError(t, err)
	defer es.Stop()

	es.Start()

	// 4. 等待连接建立
	time.Sleep(2 * time.Second)

	// 5. 验证节点是否在线
	isOnline := cs.IsNodeOnline(nodeID)
	assert.True(t, isOnline, "Node should be online via Queen")

	t.Log("✅ Queen basic communication test passed")
}

// mustEncodeNode 编码节点数据
func mustEncodeNode(t *testing.T, node *dt.Node) []byte {
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
