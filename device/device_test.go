package device

import (
	"testing"

	"github.com/snple/beacon/dt"
)

func TestDeviceBuilder(t *testing.T) {
	// 测试基本构建
	dev := New("test_device", "测试设备").
		Tags("test", "demo").
		Wire("control").
		Pin("switch", dt.TypeBool, RW).
		Pin("value", dt.TypeI32, RO).
		Done()

	if dev.ID != "test_device" {
		t.Errorf("expected ID=test_device, got %s", dev.ID)
	}

	if len(dev.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(dev.Tags))
	}

	if len(dev.Wires) != 1 {
		t.Errorf("expected 1 wire, got %d", len(dev.Wires))
	}

	if len(dev.Wires[0].Pins) != 2 {
		t.Errorf("expected 2 pins, got %d", len(dev.Wires[0].Pins))
	}
}

func TestMultipleWires(t *testing.T) {
	dev := New("multi_wire", "多Wire设备").
		Wire("wire1").
		Pin("pin1", dt.TypeBool, RW).
		Wire("wire2").
		Pin("pin2", dt.TypeI32, RW).
		Done()

	if len(dev.Wires) != 2 {
		t.Errorf("expected 2 wires, got %d", len(dev.Wires))
	}
}

func TestPinSets(t *testing.T) {
	dev := New("test_pins", "测试PinSet").
		Wire("control").
		Pins(OnOffPins).
		Pins(LevelControlPins).
		Done()

	if len(dev.Wires) != 1 {
		t.Errorf("expected 1 wire, got %d", len(dev.Wires))
	}

	// OnOff (1 pin) + LevelControl (3 pins) = 4 pins
	if len(dev.Wires[0].Pins) != 4 {
		t.Errorf("expected 4 pins, got %d", len(dev.Wires[0].Pins))
	}
}

func TestRegistry(t *testing.T) {
	// 测试获取标准设备
	dev, ok := Get("smart_bulb_onoff")
	if !ok {
		t.Error("smart_bulb_onoff not found")
	}

	if dev.Name != "智能开关灯泡" {
		t.Errorf("expected name=智能开关灯泡, got %s", dev.Name)
	}

	// 测试标签查询
	lighting := ByTag("lighting")
	if len(lighting) < 3 {
		t.Errorf("expected at least 3 lighting devices, got %d", len(lighting))
	}

	// 测试列出所有设备
	all := List()
	if len(all) < 6 {
		t.Errorf("expected at least 6 devices, got %d", len(all))
	}

	// 测试所有标签
	tags := AllTags()
	if len(tags) == 0 {
		t.Error("AllTags returned empty")
	}
}

func TestCustomDevice(t *testing.T) {
	// 注册自定义设备
	custom := New("custom_device", "自定义设备").
		Tags("custom", "test").
		Wire("control").
		Pin("custom_pin", dt.TypeString, RW).
		Done()

	Register(custom)

	dev, ok := Get("custom_device")
	if !ok {
		t.Error("custom device not found")
	}

	if len(dev.Tags) < 1 || dev.Tags[0] != "custom" {
		t.Errorf("expected tags to contain 'custom', got %v", dev.Tags)
	}
}
