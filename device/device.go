package device

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/danclive/nson-go"
	"go.uber.org/zap"
)

// ============================================================================
// 核心数据结构（设计为与 storage.Node/Wire/Pin 兼容）
// ============================================================================

// Device 设备定义（纯配置，可以值传递）
type Device struct {
	ID    string   // 设备类型标识（如 "smart_bulb_color"）
	Name  string   // 设备显示名称（如 "彩色智能灯泡"）
	Tags  []string // 标签列表（用于分类和查询，如 "lighting", "smart_home" 等）
	Desc  string   // 设备描述
	Wires []Wire   // Wire 模板列表
}

// Wire 端点模板定义（对应 storage.Wire）
type Wire struct {
	Name string   // Wire 名称
	Tags []string // 标签列表（用于分类和查询）
	Desc string   // Wire 描述
	Type string   // Wire 类型（可选，用于标识功能类型）
	Pins []Pin    // Pin 模板列表

	// 执行器配置（可选，与 Device 定义绑定）
	Actuator       Actuator          // 关联的执行器（nil 表示使用 NoOpActuator）
	ActuatorConfig map[string]string // 执行器配置选项
}

// Pin 属性模板定义（对应 storage.Pin）
type Pin struct {
	Name      string     // Pin 名称
	Tags      []string   // 标签列表（用于分类和查询）
	Desc      string     // Pin 描述
	Addr      string     // 地址映射（可选，用于绑定实际硬件地址）
	Type      uint32     // 数据类型（nson.DataType）
	Rw        int32      // 读写权限：0=只读，1=只写，2=读写，3=内部可写
	Default   nson.Value // 默认值（可选）
	Min       nson.Value // 最小值（可选，用于数值类型）
	Max       nson.Value // 最大值（可选，用于数值类型）
	Step      nson.Value // 步进值（可选，用于数值类型，如 0.5, 1, 10）
	Precision int        // 精度/小数位数（可选，用于浮点类型）
	Unit      string     // 单位（可选，如 "°C", "%", "W", "lux"）
	Enum      []EnumItem // 枚举值列表（可选，用于限定取值范围）
}

// EnumItem 枚举项
type EnumItem struct {
	Value nson.Value // 值
	Label string     // 显示名称
}

// Enum 创建枚举项
func Enum(value nson.Value, label string) EnumItem {
	return EnumItem{Value: value, Label: label}
}

// DeepCopyWire 深拷贝 Wire（包括 Pins）
func DeepCopyWire(wire *Wire) Wire {
	if wire == nil {
		return Wire{}
	}

	copied := Wire{
		Name: wire.Name,
		Desc: wire.Desc,
		Type: wire.Type,
	}

	// 深拷贝 Tags
	if wire.Tags != nil {
		copied.Tags = make([]string, len(wire.Tags))
		copy(copied.Tags, wire.Tags)
	}

	// 深拷贝 Pins
	if wire.Pins != nil {
		copied.Pins = make([]Pin, len(wire.Pins))
		for i, pin := range wire.Pins {
			copied.Pins[i] = DeepCopyPin(pin)
		}
	}

	// 深拷贝 ActuatorConfig
	if wire.ActuatorConfig != nil {
		copied.ActuatorConfig = make(map[string]string, len(wire.ActuatorConfig))
		maps.Copy(copied.ActuatorConfig, wire.ActuatorConfig)
	}

	// Actuator 是接口，只复制引用（执行器通常是单例或共享的）
	copied.Actuator = wire.Actuator

	return copied
}

// DeepCopyPin 深拷贝 Pin
func DeepCopyPin(pin Pin) Pin {
	copied := Pin{
		Name:      pin.Name,
		Desc:      pin.Desc,
		Addr:      pin.Addr,
		Type:      pin.Type,
		Rw:        pin.Rw,
		Precision: pin.Precision,
		Unit:      pin.Unit,
		Default:   pin.Default,
		Min:       pin.Min,
		Max:       pin.Max,
		Step:      pin.Step,
	}

	// 深拷贝 Tags
	if pin.Tags != nil {
		copied.Tags = make([]string, len(pin.Tags))
		copy(copied.Tags, pin.Tags)
	}

	// 深拷贝 Enum
	if pin.Enum != nil {
		copied.Enum = make([]EnumItem, len(pin.Enum))
		for i, item := range pin.Enum {
			copied.Enum[i] = EnumItem{
				Value: item.Value,
				Label: item.Label,
			}
		}
	}

	return copied
}

// ============================================================================
// Pin Builder
// ============================================================================

// PinBuilderT Pin 构建器
type PinBuilderT struct {
	pin Pin
}

// PinBuilder 创建 Pin 构建器
func PinBuilder(name string, typ uint32, rw int32) *PinBuilderT {
	return &PinBuilderT{
		pin: Pin{
			Name: name,
			Type: typ,
			Rw:   rw,
			Tags: []string{},
		},
	}
}

// Desc 设置描述
func (p *PinBuilderT) Desc(desc string) *PinBuilderT {
	p.pin.Desc = desc
	return p
}

// Tags 设置标签
func (p *PinBuilderT) Tags(tags ...string) *PinBuilderT {
	p.pin.Tags = tags
	return p
}

// Addr 设置地址映射
func (p *PinBuilderT) Addr(addr string) *PinBuilderT {
	p.pin.Addr = addr
	return p
}

// Default 设置默认值
func (p *PinBuilderT) Default(def nson.Value) *PinBuilderT {
	p.pin.Default = def
	return p
}

// Min 设置最小值
func (p *PinBuilderT) Min(min nson.Value) *PinBuilderT {
	p.pin.Min = min
	return p
}

// Max 设置最大值
func (p *PinBuilderT) Max(max nson.Value) *PinBuilderT {
	p.pin.Max = max
	return p
}

// Range 同时设置最小值和最大值
func (p *PinBuilderT) Range(min, max nson.Value) *PinBuilderT {
	p.pin.Min = min
	p.pin.Max = max
	return p
}

// Step 设置步进值
func (p *PinBuilderT) Step(step nson.Value) *PinBuilderT {
	p.pin.Step = step
	return p
}

// Precision 设置精度（小数位数）
func (p *PinBuilderT) Precision(precision int) *PinBuilderT {
	p.pin.Precision = precision
	return p
}

// Unit 设置单位
func (p *PinBuilderT) Unit(unit string) *PinBuilderT {
	p.pin.Unit = unit
	return p
}

// Enum 设置枚举值（带标签）
func (p *PinBuilderT) Enum(items ...EnumItem) *PinBuilderT {
	p.pin.Enum = items
	return p
}

// EnumValues 设置枚举值（简化版，无标签）
func (p *PinBuilderT) EnumValues(values ...nson.Value) *PinBuilderT {
	p.pin.Enum = make([]EnumItem, len(values))
	for i, v := range values {
		p.pin.Enum[i] = EnumItem{Value: v}
	}
	return p
}

// Build 构建 Pin
func (p *PinBuilderT) Build() Pin {
	return p.pin
}

// ============================================================================
// Wire Builder
// ============================================================================

// WireBuilderT Wire 构建器
type WireBuilderT struct {
	wire Wire
}

// WireBuilder 创建 Wire 构建器
func WireBuilder(name string) *WireBuilderT {
	return &WireBuilderT{
		wire: Wire{
			Name:           name,
			Pins:           []Pin{},
			Tags:           []string{},
			ActuatorConfig: make(map[string]string),
		},
	}
}

// Desc 设置描述
func (w *WireBuilderT) Desc(desc string) *WireBuilderT {
	w.wire.Desc = desc
	return w
}

// Tags 设置标签
func (w *WireBuilderT) Tags(tags ...string) *WireBuilderT {
	w.wire.Tags = tags
	return w
}

// Type 设置类型
func (w *WireBuilderT) Type(typ string) *WireBuilderT {
	w.wire.Type = typ
	return w
}

// Pin 添加 Pin
func (w *WireBuilderT) Pin(p Pin) *WireBuilderT {
	w.wire.Pins = append(w.wire.Pins, p)
	return w
}

// WithActuator 设置执行器（设备定义时绑定）
func (w *WireBuilderT) WithActuator(act Actuator) *WireBuilderT {
	w.wire.Actuator = act
	return w
}

// ActuatorOption 设置执行器配置选项
func (w *WireBuilderT) ActuatorOption(key, value string) *WireBuilderT {
	w.wire.ActuatorConfig[key] = value
	return w
}

// ActuatorOptions 批量设置执行器配置选项
func (w *WireBuilderT) ActuatorOptions(opts map[string]string) *WireBuilderT {
	maps.Copy(w.wire.ActuatorConfig, opts)
	return w
}

// Build 构建 Wire（可选，用于显式构建）
func (w *WireBuilderT) Build() Wire {
	return w.wire
}

// ============================================================================
// Device Builder
// ============================================================================

// DeviceBuilderT 设备构建器
type DeviceBuilderT struct {
	device Device
}

// DeviceBuilder 创建设备构建器
func DeviceBuilder(id, name string) *DeviceBuilderT {
	return &DeviceBuilderT{
		device: Device{
			ID:    id,
			Name:  name,
			Wires: []Wire{},
			Tags:  []string{},
		},
	}
}

// Desc 设置描述
func (d *DeviceBuilderT) Desc(desc string) *DeviceBuilderT {
	d.device.Desc = desc
	return d
}

// Tags 设置标签
func (d *DeviceBuilderT) Tags(tags ...string) *DeviceBuilderT {
	d.device.Tags = tags
	return d
}

// Wire 添加 Wire
func (d *DeviceBuilderT) Wire(w *WireBuilderT) *DeviceBuilderT {
	d.device.Wires = append(d.device.Wires, w.wire)
	return d
}

// Done 完成构建
func (d *DeviceBuilderT) Done() Device {
	return d.device
}

// ============================================================================
// 兼容旧 API（可选，逐步迁移后删除）
// ============================================================================

// New 创建设备构建器（兼容旧 API）
func New(id, name string) *DeviceBuilderT {
	return DeviceBuilder(id, name)
}

// ============================================================================
// 常量定义
// ============================================================================

// 读写权限
const (
	RO = 0 // 只读（Read Only）
	WO = 1 // 只写（Write Only）
	RW = 2 // 读写（Read Write）
	IW = 3 // 内部可写（Internal Write）
)

// 开关状态
const (
	ON  = 1
	OFF = 0
)

// ============================================================================
// DeviceManager - 设备运行时管理器（管理执行器生命周期）
// ============================================================================

// DeviceManager 设备运行时管理器
type DeviceManager struct {
	device Device // 设备配置（只读）

	// 运行时状态
	actuators    map[string]Actuator // wireID -> Actuator 实例
	pollInterval time.Duration       // 轮询间隔
	pollEnabled  map[string]bool     // wireID -> 是否启用轮询
	pollCancel   context.CancelFunc  // 停止轮询
	pollWG       sync.WaitGroup      // 等待轮询结束
	mu           sync.RWMutex        // 保护 actuators
	logger       *zap.Logger         // 日志
	onPinRead    PinReadCallback     // Pin 读取回调（用于上报传感器数据）
}

// PinReadCallback Pin 读取回调（用于将传感器数据上报到上层）
type PinReadCallback func(wireID, pinName string, value nson.Value) error

// NewDeviceManager 创建设备管理器
func NewDeviceManager(dev Device) *DeviceManager {
	return &DeviceManager{
		device:       dev,
		pollInterval: 5 * time.Second, // 默认 5 秒轮询
	}
}

// GetDevice 获取设备配置（只读）
func (dm *DeviceManager) GetDevice() Device {
	return dm.device
}

// Initialize 初始化设备的所有执行器
func (dm *DeviceManager) Init(ctx context.Context, logger *zap.Logger, onPinRead PinReadCallback) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.actuators != nil {
		return fmt.Errorf("device already initialized")
	}

	dm.actuators = make(map[string]Actuator)
	dm.pollEnabled = make(map[string]bool)
	dm.logger = logger
	dm.onPinRead = onPinRead

	// 为每个 Wire 初始化执行器
	for i := range dm.device.Wires {
		wire := &dm.device.Wires[i]

		// 选择执行器
		var actuator Actuator
		if wire.Actuator != nil {
			// 使用 Wire 中预配置的执行器
			actuator = wire.Actuator
		} else if wire.Type != "" {
			// 根据 Wire.Type 从注册表获取
			act, err := GetActuator(wire.Type)
			if err != nil {
				logger.Sugar().Warnf("No actuator for wire %s (type=%s), using noop: %v",
					wire.Name, wire.Type, err)
				actuator, _ = GetActuator("") // NoOpActuator
			} else {
				actuator = act
			}
		} else {
			// 默认使用 NoOpActuator
			actuator, _ = GetActuator("")
		}

		// 初始化执行器
		config := ActuatorConfig{
			Wire: DeepCopyWire(wire),
			Pins: wire.Pins,
		}

		if err := actuator.Init(ctx, config); err != nil {
			return fmt.Errorf("initialize actuator for wire %s: %w", wire.Name, err)
		}

		dm.actuators[wire.Name] = actuator

		// 检查是否需要轮询（有只读或可读写的 Pin）
		hasReadable := false
		for _, pin := range wire.Pins {
			if pin.Rw == RO || pin.Rw == RW {
				hasReadable = true
				break
			}
		}
		dm.pollEnabled[wire.Name] = hasReadable

		info := actuator.Info()
		logger.Sugar().Infof("Actuator initialized: wire=%s, type=%s, name=%s, version=%s",
			wire.Name, wire.Type, info.Name, info.Version)
	}

	return nil
}

// Start 启动设备（开始轮询传感器等）
func (dm *DeviceManager) Start(ctx context.Context) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.actuators == nil {
		return fmt.Errorf("device not initialized, call Initialize first")
	}

	if dm.pollCancel != nil {
		return fmt.Errorf("device already started")
	}

	// 创建轮询上下文
	pollCtx, cancel := context.WithCancel(ctx)
	dm.pollCancel = cancel

	// 为每个可读的 Wire 启动轮询
	for wireID, enabled := range dm.pollEnabled {
		if !enabled {
			continue
		}

		wireID := wireID // 捕获变量
		dm.pollWG.Add(1)
		go dm.pollActuator(pollCtx, wireID)
	}

	dm.logger.Info("Device started")
	return nil
}

// Stop 停止设备（停止轮询，关闭执行器）
func (dm *DeviceManager) Stop() error {
	dm.mu.Lock()
	if dm.pollCancel != nil {
		dm.pollCancel()
		dm.pollCancel = nil
	}
	dm.mu.Unlock()

	// 等待所有轮询停止
	dm.pollWG.Wait()

	// 关闭所有执行器
	dm.mu.Lock()
	defer dm.mu.Unlock()

	var errs []error
	for wireID, actuator := range dm.actuators {
		if err := actuator.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close actuator %s: %w", wireID, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("stop device: %v", errs)
	}

	dm.logger.Info("Device stopped")
	return nil
}

// Execute 执行 Pin 写入
func (dm *DeviceManager) Execute(ctx context.Context, wireID, pinName string, value nson.Value) error {
	dm.mu.RLock()
	actuator, ok := dm.actuators[wireID]
	dm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no actuator for wire: %s", wireID)
	}

	return actuator.Execute(ctx, pinName, value)
}

// Read 读取 Wire 的所有可读 Pin
func (dm *DeviceManager) Read(ctx context.Context, wireID string) (map[string]nson.Value, error) {
	dm.mu.RLock()
	actuator, ok := dm.actuators[wireID]
	dm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no actuator for wire: %s", wireID)
	}

	return actuator.Read(ctx, nil)
}

// GetActuatorInfo 获取 Wire 的执行器信息
func (dm *DeviceManager) GetActuatorInfo(wireID string) (ActuatorInfo, error) {
	dm.mu.RLock()
	actuator, ok := dm.actuators[wireID]
	dm.mu.RUnlock()

	if !ok {
		return ActuatorInfo{}, fmt.Errorf("no actuator for wire: %s", wireID)
	}

	return actuator.Info(), nil
}

// ListActuatorInfos 列出所有执行器信息
func (dm *DeviceManager) ListActuatorInfos() map[string]ActuatorInfo {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	infos := make(map[string]ActuatorInfo, len(dm.actuators))
	for wireID, actuator := range dm.actuators {
		infos[wireID] = actuator.Info()
	}
	return infos
}

// SetPollInterval 设置轮询间隔
func (dm *DeviceManager) SetPollInterval(interval time.Duration) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.pollInterval = interval
}

// pollActuator 轮询某个 Wire 的传感器数据
func (dm *DeviceManager) pollActuator(ctx context.Context, wireID string) {
	defer dm.pollWG.Done()

	ticker := time.NewTicker(dm.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dm.mu.RLock()
			actuator, ok := dm.actuators[wireID]
			dm.mu.RUnlock()

			if !ok {
				continue
			}

			// 读取所有 Pin
			values, err := actuator.Read(ctx, nil)
			if err != nil {
				dm.logger.Sugar().Warnf("Poll actuator %s: %v", wireID, err)
				continue
			}

			// 回调上报
			if dm.onPinRead != nil {
				for pinName, value := range values {
					if err := dm.onPinRead(wireID, pinName, value); err != nil {
						dm.logger.Sugar().Warnf("OnPinRead callback for %s.%s: %v",
							wireID, pinName, err)
					}
				}
			}
		}
	}
}
