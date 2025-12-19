package storage

import (
	"context"
	"testing"
	"time"

	"github.com/snple/beacon/dt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew 测试 Storage 创建
func TestNew(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)

	assert.NotNil(t, s)
	assert.NotNil(t, s.nodes)
	assert.NotNil(t, s.secrets)
	assert.NotNil(t, s.index)
	assert.NotNil(t, s.db)
}

// TestStorage_Push_GetNode 测试推送和获取节点
func TestStorage_Push_GetNode(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	// 创建测试节点
	node := &dt.Node{
		ID:      "node-001",
		Name:    "TestNode",
		Updated: time.Now(),
		Wires: []dt.Wire{
			{
				ID:   "wire-001",
				Name: "TestWire",
				Type: "modbus",
				Pins: []dt.Pin{
					{
						ID:   "pin-001",
						Name: "TestPin",
						Addr: "0x01",
						Type: 1,
						Rw:   3,
					},
				},
			},
		},
	}

	// 序列化并推送节点
	data, err := encodeNode(node)
	require.NoError(t, err)

	err = s.Push(ctx, data)
	require.NoError(t, err)

	// 获取节点
	retrieved, err := s.GetNode("node-001")
	require.NoError(t, err)
	assert.Equal(t, node.ID, retrieved.ID)
	assert.Equal(t, node.Name, retrieved.Name)
	assert.Equal(t, 1, len(retrieved.Wires))
	assert.Equal(t, "wire-001", retrieved.Wires[0].ID)
}

// TestStorage_GetNodeByName 测试按名称获取节点
func TestStorage_GetNodeByName(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	node := &dt.Node{
		ID:      "node-001",
		Name:    "TestNode",
		Updated: time.Now(),
		Wires:   []dt.Wire{},
	}

	data, err := encodeNode(node)
	require.NoError(t, err)
	err = s.Push(ctx, data)
	require.NoError(t, err)

	// 按名称获取
	retrieved, err := s.GetNodeByName("TestNode")
	require.NoError(t, err)
	assert.Equal(t, "node-001", retrieved.ID)
	assert.Equal(t, "TestNode", retrieved.Name)

	// 测试不存在的节点
	_, err = s.GetNodeByName("NonExistent")
	assert.Error(t, err)
}

// TestStorage_ListNodes 测试列出所有节点
func TestStorage_ListNodes(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	// 添加多个节点
	for i := 1; i <= 3; i++ {
		node := &dt.Node{
			ID:      "node-00" + string(rune('0'+i)),
			Name:    "Node" + string(rune('0'+i)),
			Updated: time.Now(),
			Wires:   []dt.Wire{},
		}
		data, err := encodeNode(node)
		require.NoError(t, err)
		err = s.Push(ctx, data)
		require.NoError(t, err)
	}

	nodes := s.ListNodes()
	assert.Equal(t, 3, len(nodes))
}

// TestStorage_GetWireByID 测试按 ID 获取 Wire
func TestStorage_GetWireByID(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	node := &dt.Node{
		ID:      "node-001",
		Name:    "TestNode",
		Updated: time.Now(),
		Wires: []dt.Wire{
			{
				ID:   "wire-001",
				Name: "TestWire",
				Type: "modbus",
				Pins: []dt.Pin{},
			},
		},
	}

	data, err := encodeNode(node)
	require.NoError(t, err)
	err = s.Push(ctx, data)
	require.NoError(t, err)

	// 获取 Wire
	wire, err := s.GetWireByID("wire-001")
	require.NoError(t, err)
	assert.Equal(t, "wire-001", wire.ID)
	assert.Equal(t, "TestWire", wire.Name)

	// 测试不存在的 Wire
	_, err = s.GetWireByID("non-existent")
	assert.Error(t, err)
}

// TestStorage_GetWireByName 测试按名称获取 Wire
func TestStorage_GetWireByName(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	node := &dt.Node{
		ID:      "node-001",
		Name:    "TestNode",
		Updated: time.Now(),
		Wires: []dt.Wire{
			{
				ID:   "wire-001",
				Name: "TestWire",
				Type: "modbus",
				Pins: []dt.Pin{},
			},
		},
	}

	data, err := encodeNode(node)
	require.NoError(t, err)
	err = s.Push(ctx, data)
	require.NoError(t, err)

	// 获取 Wire
	wire, err := s.GetWireByName("node-001", "TestWire")
	require.NoError(t, err)
	assert.Equal(t, "wire-001", wire.ID)
	assert.Equal(t, "TestWire", wire.Name)
}

// TestStorage_GetWireByFullName 测试按全名获取 Wire
func TestStorage_GetWireByFullName(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	node := &dt.Node{
		ID:      "node-001",
		Name:    "TestNode",
		Updated: time.Now(),
		Wires: []dt.Wire{
			{
				ID:   "wire-001",
				Name: "TestWire",
				Type: "modbus",
				Pins: []dt.Pin{},
			},
		},
	}

	data, err := encodeNode(node)
	require.NoError(t, err)
	err = s.Push(ctx, data)
	require.NoError(t, err)

	// 按全名获取 (格式: nodeName.wireName)
	wire, err := s.GetWireByFullName("TestNode.TestWire")
	require.NoError(t, err)
	assert.Equal(t, "wire-001", wire.ID)
	assert.Equal(t, "TestWire", wire.Name)

	// 测试无效格式
	_, err = s.GetWireByFullName("InvalidFormat")
	assert.Error(t, err)
}

// TestStorage_ListWires 测试列出节点的所有 Wire
func TestStorage_ListWires(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	node := &dt.Node{
		ID:      "node-001",
		Name:    "TestNode",
		Updated: time.Now(),
		Wires: []dt.Wire{
			{ID: "wire-001", Name: "Wire1", Type: "modbus", Pins: []dt.Pin{}},
			{ID: "wire-002", Name: "Wire2", Type: "mqtt", Pins: []dt.Pin{}},
		},
	}

	data, err := encodeNode(node)
	require.NoError(t, err)
	err = s.Push(ctx, data)
	require.NoError(t, err)

	wires, err := s.ListWires("node-001")
	require.NoError(t, err)
	assert.Equal(t, 2, len(wires))
}

// TestStorage_GetPinByID 测试按 ID 获取 Pin
func TestStorage_GetPinByID(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	node := &dt.Node{
		ID:      "node-001",
		Name:    "TestNode",
		Updated: time.Now(),
		Wires: []dt.Wire{
			{
				ID:   "wire-001",
				Name: "TestWire",
				Type: "modbus",
				Pins: []dt.Pin{
					{
						ID:   "pin-001",
						Name: "TestPin",
						Addr: "0x01",
						Type: 1,
						Rw:   3,
					},
				},
			},
		},
	}

	data, err := encodeNode(node)
	require.NoError(t, err)
	err = s.Push(ctx, data)
	require.NoError(t, err)

	// 获取 Pin
	pin, err := s.GetPinByID("pin-001")
	require.NoError(t, err)
	assert.Equal(t, "pin-001", pin.ID)
	assert.Equal(t, "TestPin", pin.Name)

	// 测试不存在的 Pin
	_, err = s.GetPinByID("non-existent")
	assert.Error(t, err)
}

// TestStorage_GetPinByName 测试按名称获取 Pin
func TestStorage_GetPinByName(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	node := &dt.Node{
		ID:      "node-001",
		Name:    "TestNode",
		Updated: time.Now(),
		Wires: []dt.Wire{
			{
				ID:   "wire-001",
				Name: "TestWire",
				Type: "modbus",
				Pins: []dt.Pin{
					{ID: "pin-001", Name: "TestPin", Addr: "0x01", Type: 1, Rw: 3},
				},
			},
		},
	}

	data, err := encodeNode(node)
	require.NoError(t, err)
	err = s.Push(ctx, data)
	require.NoError(t, err)

	// 获取 Pin (格式是 WireName.PinName)
	pin, err := s.GetPinByName("node-001", "TestWire.TestPin")
	require.NoError(t, err)
	assert.Equal(t, "pin-001", pin.ID)
	assert.Equal(t, "TestPin", pin.Name)
}

// TestStorage_ListPins 测试列出 Wire 的所有 Pin
func TestStorage_ListPins(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	node := &dt.Node{
		ID:      "node-001",
		Name:    "TestNode",
		Updated: time.Now(),
		Wires: []dt.Wire{
			{
				ID:   "wire-001",
				Name: "TestWire",
				Type: "modbus",
				Pins: []dt.Pin{
					{ID: "pin-001", Name: "Pin1", Addr: "0x01", Type: 1, Rw: 3},
					{ID: "pin-002", Name: "Pin2", Addr: "0x02", Type: 1, Rw: 3},
					{ID: "pin-003", Name: "Pin3", Addr: "0x03", Type: 1, Rw: 3},
				},
			},
		},
	}

	data, err := encodeNode(node)
	require.NoError(t, err)
	err = s.Push(ctx, data)
	require.NoError(t, err)

	pins, err := s.ListPins("wire-001")
	require.NoError(t, err)
	assert.Equal(t, 3, len(pins))
}

// TestStorage_ListPinsByNode 测试列出节点的所有 Pin
func TestStorage_ListPinsByNode(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	node := &dt.Node{
		ID:      "node-001",
		Name:    "TestNode",
		Updated: time.Now(),
		Wires: []dt.Wire{
			{
				ID:   "wire-001",
				Name: "Wire1",
				Type: "modbus",
				Pins: []dt.Pin{
					{ID: "pin-001", Name: "Pin1", Addr: "0x01", Type: 1, Rw: 3},
					{ID: "pin-002", Name: "Pin2", Addr: "0x02", Type: 1, Rw: 3},
				},
			},
			{
				ID:   "wire-002",
				Name: "Wire2",
				Type: "",
				Pins: []dt.Pin{
					{ID: "pin-003", Name: "Pin3", Addr: "topic/test", Type: 1, Rw: 3},
				},
			},
		},
	}

	data, err := encodeNode(node)
	require.NoError(t, err)
	err = s.Push(ctx, data)
	require.NoError(t, err)

	pins, err := s.ListPinsByNode("node-001")
	require.NoError(t, err)
	assert.Equal(t, 3, len(pins))
}

// TestStorage_DeleteNode 测试删除节点
func TestStorage_DeleteNode(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	node := &dt.Node{
		ID:      "node-001",
		Name:    "TestNode",
		Updated: time.Now(),
		Wires:   []dt.Wire{},
	}

	data, err := encodeNode(node)
	require.NoError(t, err)
	err = s.Push(ctx, data)
	require.NoError(t, err)

	// 验证节点存在
	_, err = s.GetNode("node-001")
	require.NoError(t, err)

	// 删除节点
	err = s.DeleteNode(ctx, "node-001")
	require.NoError(t, err)

	// 验证节点已删除
	_, err = s.GetNode("node-001")
	assert.Error(t, err)
}

// TestStorage_SecretOperations 测试 Secret 操作
func TestStorage_SecretOperations(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	nodeID := "node-001"
	secret := "test-secret-123"

	// 设置 Secret
	err := s.SetSecret(ctx, nodeID, secret)
	require.NoError(t, err)

	// 获取 Secret
	retrieved, err := s.GetSecret(nodeID)
	require.NoError(t, err)
	assert.Equal(t, secret, retrieved)

	// 更新 Secret
	newSecret := "new-secret-456"
	err = s.SetSecret(ctx, nodeID, newSecret)
	require.NoError(t, err)

	retrieved, err = s.GetSecret(nodeID)
	require.NoError(t, err)
	assert.Equal(t, newSecret, retrieved)
}

// TestStorage_Load 测试从数据库加载
func TestStorage_Load(t *testing.T) {
	db := setupTestDB(t)

	// 第一个 storage 实例：写入数据
	s1 := New(db)
	ctx := context.Background()

	node := &dt.Node{
		ID:      "node-001",
		Name:    "TestNode",
		Updated: time.Now(),
		Wires: []dt.Wire{
			{
				ID:   "wire-001",
				Name: "TestWire",
				Type: "modbus",
				Pins: []dt.Pin{
					{ID: "pin-001", Name: "TestPin", Addr: "0x01", Type: 1, Rw: 3},
				},
			},
		},
	}

	data, err := encodeNode(node)
	require.NoError(t, err)
	err = s1.Push(ctx, data)
	require.NoError(t, err)

	err = s1.SetSecret(ctx, "node-001", "secret-123")
	require.NoError(t, err)

	// 第二个 storage 实例：从数据库加载
	s2 := New(db)
	err = s2.Load(ctx)
	require.NoError(t, err)

	// 验证数据已加载
	loadedNode, err := s2.GetNode("node-001")
	require.NoError(t, err)
	assert.Equal(t, "node-001", loadedNode.ID)
	assert.Equal(t, "TestNode", loadedNode.Name)

	loadedSecret, err := s2.GetSecret("node-001")
	require.NoError(t, err)
	assert.Equal(t, "secret-123", loadedSecret)

	// 验证索引已构建
	wire, err := s2.GetWireByID("wire-001")
	require.NoError(t, err)
	assert.Equal(t, "wire-001", wire.ID)

	pin, err := s2.GetPinByID("pin-001")
	require.NoError(t, err)
	assert.Equal(t, "pin-001", pin.ID)
}

// TestStorage_UpdateNode 测试更新节点
func TestStorage_UpdateNode(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	// 初始节点
	node := &dt.Node{
		ID:      "node-001",
		Name:    "TestNode",
		Updated: time.Now(),
		Wires: []dt.Wire{
			{ID: "wire-001", Name: "Wire1", Type: "modbus", Pins: []dt.Pin{}},
		},
	}

	data, err := encodeNode(node)
	require.NoError(t, err)
	err = s.Push(ctx, data)
	require.NoError(t, err)

	// 更新节点 (添加新 Wire)
	updatedNode := &dt.Node{
		ID:      "node-001",
		Name:    "TestNode",
		Updated: time.Now(),
		Wires: []dt.Wire{
			{ID: "wire-001", Name: "Wire1", Type: "modbus", Pins: []dt.Pin{}},
			{ID: "wire-002", Name: "Wire2", Type: "mqtt", Pins: []dt.Pin{}},
		},
	}

	data, err = encodeNode(updatedNode)
	require.NoError(t, err)
	err = s.Push(ctx, data)
	require.NoError(t, err)

	// 验证更新
	retrieved, err := s.GetNode("node-001")
	require.NoError(t, err)
	assert.Equal(t, 2, len(retrieved.Wires))

	// 验证新 Wire 的索引已更新
	wire, err := s.GetWireByID("wire-002")
	require.NoError(t, err)
	assert.Equal(t, "wire-002", wire.ID)
}

// TestStorage_ErrorCases 测试各种错误情况
func TestStorage_ErrorCases(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)

	// 获取不存在的节点
	_, err := s.GetNode("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// 获取不存在的 Wire
	_, err = s.GetWireByID("non-existent")
	assert.Error(t, err)

	// 获取不存在的 Pin
	_, err = s.GetPinByID("non-existent")
	assert.Error(t, err)

	// 获取不存在的 Secret
	_, err = s.GetSecret("non-existent")
	assert.Error(t, err)

	// 删除不存在的节点
	err = s.DeleteNode(context.Background(), "non-existent")
	assert.Error(t, err)
}
