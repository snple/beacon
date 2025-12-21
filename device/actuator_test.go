package device

import (
	"context"
	"testing"

	"github.com/danclive/nson-go"
)

func TestNoOpActuator(t *testing.T) {
	ctx := context.Background()
	actuator := &NoOpActuator{
		state: make(map[string]nson.Value),
	}

	config := ActuatorConfig{
		Wire: Wire{
			Name: "test",
			Type: "noop",
		},
		Pins: []Pin{
			{Name: "on", Type: nson.DataTypeBOOL, Default: nson.Bool(false)},
			{Name: "dim", Type: nson.DataTypeU8, Default: nson.U8(0)},
		},
	}

	// 初始化
	if err := actuator.Init(ctx, config); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// 检查默认值
	state := actuator.GetState()
	if len(state) != 2 {
		t.Errorf("Expected 2 default values, got %d", len(state))
	}

	// 执行写入
	if err := actuator.Execute(ctx, "on", nson.Bool(true)); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// 读取值
	values, err := actuator.Read(ctx, []string{"on"})
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if v, ok := values["on"]; !ok {
		t.Error("on value not found")
	} else if boolVal, ok := v.(nson.Bool); !ok || !bool(boolVal) {
		t.Errorf("Expected on=true, got %v", v)
	}

	// 读取所有
	allValues, err := actuator.Read(ctx, nil)
	if err != nil {
		t.Fatalf("Read all failed: %v", err)
	}

	if len(allValues) != 2 {
		t.Errorf("Expected 2 values, got %d", len(allValues))
	}
}

func TestActuatorRegistry(t *testing.T) {
	// 测试注册
	called := false
	RegisterActuator("test_type", func() Actuator {
		called = true
		return &NoOpActuator{state: make(map[string]nson.Value)}
	})

	// 获取执行器
	act, err := GetActuator("test_type")
	if err != nil {
		t.Fatalf("GetActuator failed: %v", err)
	}

	if !called {
		t.Error("Factory function not called")
	}

	if act == nil {
		t.Error("Actuator is nil")
	}

	// 测试不存在的类型
	_, err = GetActuator("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent type")
	}

	// 测试列表
	types := ListActuators()
	found := false
	for _, typ := range types {
		if typ == "test_type" {
			found = true
			break
		}
	}
	if !found {
		t.Error("test_type not found in list")
	}
}

func TestActuatorChain(t *testing.T) {
	ctx := context.Background()

	// 创建两个 NoOp 执行器
	act1 := &NoOpActuator{state: make(map[string]nson.Value)}
	act2 := &NoOpActuator{state: make(map[string]nson.Value)}

	chain := NewActuatorChain(act1, act2)

	config := ActuatorConfig{
		Wire: Wire{
			Name: "test_chain",
			Type: "noop",
		},
		Pins: []Pin{
			{Name: "test", Type: nson.DataTypeBOOL},
		},
	}

	// 初始化
	if err := chain.Init(ctx, config); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// 执行写入（应该同时写入两个执行器）
	if err := chain.Execute(ctx, "test", nson.Bool(true)); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// 验证两个执行器都被写入
	state1 := act1.GetState()
	state2 := act2.GetState()

	if v, ok := state1["test"]; !ok {
		t.Error("act1 not updated")
	} else if boolVal, ok := v.(nson.Bool); !ok || !bool(boolVal) {
		t.Error("act1 value incorrect")
	}

	if v, ok := state2["test"]; !ok {
		t.Error("act2 not updated")
	} else if boolVal, ok := v.(nson.Bool); !ok || !bool(boolVal) {
		t.Error("act2 value incorrect")
	}

	// 读取（应该合并结果）
	values, err := chain.Read(ctx, nil)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if v, ok := values["test"]; !ok {
		t.Error("Chain read failed: value not found")
	} else if boolVal, ok := v.(nson.Bool); !ok || !bool(boolVal) {
		t.Errorf("Chain read failed: expected true, got %v", v)
	}
}
