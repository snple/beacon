package device

import (
	"fmt"
	"sync"

	"github.com/danclive/nson-go"
)

// ============================================================================
// Actuator 接口 - Wire 级别的执行器
// ============================================================================

// Actuator 是 Wire 级别的硬件执行器接口
// 每个 Wire 对应一个 Actuator，负责将 Pin 写入转换为实际的硬件操作
type Actuator interface {
	// Init 初始化执行器（如打开串口、连接设备等）
	// config: Wire 配置信息，包含地址映射等
	Init(config ActuatorConfig) error

	// Execute 执行 Pin 写入操作
	// pinName: Pin 名称（不含 Wire 前缀，如 "on", "dim"）
	// value: 要写入的值
	Execute(pinName string, value nson.Value) error

	// Read 读取 Pin 的实际值（用于传感器等只读设备）
	// pinNames: 要读取的 Pin 名称列表，nil 表示读取所有
	// 返回: map[pinName]value
	Read(pinNames []string) (map[string]nson.Value, error)

	// Close 关闭执行器，释放资源
	Close() error

	// Info 获取执行器信息（用于诊断）
	Info() ActuatorInfo
}

// ActuatorConfig 执行器配置
type ActuatorConfig struct {
	Wire Wire  // Wire
	Pins []Pin // Pin 列表（包含地址映射等）
}

// ActuatorInfo 执行器信息
type ActuatorInfo struct {
	Name         string   // 执行器名称
	Type         string   // 执行器类型
	Version      string   // 版本
	Capabilities []string // 支持的功能（如 "read", "write", "batch"）
}

// ============================================================================
// 执行器注册表
// ============================================================================

var (
	actuatorRegistry   = make(map[string]ActuatorFactory)
	actuatorRegistryMu sync.RWMutex
)

// ActuatorFactory 执行器工厂函数
type ActuatorFactory func() Actuator

// RegisterActuator 注册执行器工厂
// wireType: Wire.Type，如 "modbus_rtu", "gpio", "mqtt"
func RegisterActuator(wireType string, factory ActuatorFactory) {
	actuatorRegistryMu.Lock()
	defer actuatorRegistryMu.Unlock()
	actuatorRegistry[wireType] = factory
}

// GetActuator 根据 Wire.Type 创建执行器实例
func GetActuator(wireType string) (Actuator, error) {
	actuatorRegistryMu.RLock()
	factory, ok := actuatorRegistry[wireType]
	actuatorRegistryMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no actuator registered for wire type: %s", wireType)
	}

	return factory(), nil
}

// ListActuators 列出所有已注册的执行器类型
func ListActuators() []string {
	actuatorRegistryMu.RLock()
	defer actuatorRegistryMu.RUnlock()

	types := make([]string, 0, len(actuatorRegistry))
	for t := range actuatorRegistry {
		types = append(types, t)
	}
	return types
}

// ============================================================================
// NoOpActuator - 空操作执行器（用于纯软件模拟）
// ============================================================================

func init() {
	RegisterActuator("noop", func() Actuator {
		return &NoOpActuator{
			state: make(map[string]nson.Value),
		}
	})
	RegisterActuator("", func() Actuator { // 默认执行器
		return &NoOpActuator{
			state: make(map[string]nson.Value),
		}
	})
}

// NoOpActuator 不执行任何实际操作，只记录状态（用于测试和虚拟设备）
type NoOpActuator struct {
	mu    sync.RWMutex
	state map[string]nson.Value
	info  ActuatorInfo
}

func (a *NoOpActuator) Init(config ActuatorConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.info = ActuatorInfo{
		Name:         "NoOp Actuator",
		Type:         "noop",
		Version:      "1.0.0",
		Capabilities: []string{"read", "write"},
	}

	// 初始化默认值
	for _, pin := range config.Pins {
		if pin.Default != nil {
			a.state[pin.Name] = pin.Default
		}
	}

	return nil
}

func (a *NoOpActuator) Execute(pinName string, value nson.Value) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.state[pinName] = value
	return nil
}

func (a *NoOpActuator) Read(pinNames []string) (map[string]nson.Value, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string]nson.Value)

	if pinNames == nil {
		// 读取所有
		for k, v := range a.state {
			result[k] = v
		}
	} else {
		// 读取指定的
		for _, name := range pinNames {
			if v, ok := a.state[name]; ok {
				result[name] = v
			}
		}
	}

	return result, nil
}

func (a *NoOpActuator) Close() error {
	return nil
}

func (a *NoOpActuator) Info() ActuatorInfo {
	return a.info
}

// GetState 获取内部状态（用于测试）
func (a *NoOpActuator) GetState() map[string]nson.Value {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string]nson.Value, len(a.state))
	for k, v := range a.state {
		result[k] = v
	}
	return result
}

// ============================================================================
// ActuatorChain - 执行器链（组合多个执行器）
// ============================================================================

// ActuatorChain 支持组合多个执行器
// 写入时按顺序执行所有执行器
// 读取时合并所有执行器的结果
type ActuatorChain struct {
	actuators []Actuator
	mu        sync.RWMutex
}

func NewActuatorChain(actuators ...Actuator) *ActuatorChain {
	return &ActuatorChain{
		actuators: actuators,
	}
}

func (c *ActuatorChain) Init(config ActuatorConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, act := range c.actuators {
		if err := act.Init(config); err != nil {
			return fmt.Errorf("initialize actuator: %w", err)
		}
	}
	return nil
}

func (c *ActuatorChain) Execute(pinName string, value nson.Value) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, act := range c.actuators {
		if err := act.Execute(pinName, value); err != nil {
			return err
		}
	}
	return nil
}

func (c *ActuatorChain) Read(pinNames []string) (map[string]nson.Value, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]nson.Value)

	for _, act := range c.actuators {
		values, err := act.Read(pinNames)
		if err != nil {
			return nil, err
		}
		// 合并结果（后面的覆盖前面的）
		for k, v := range values {
			result[k] = v
		}
	}

	return result, nil
}

func (c *ActuatorChain) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var lastErr error
	for _, act := range c.actuators {
		if err := act.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (c *ActuatorChain) Info() ActuatorInfo {
	return ActuatorInfo{
		Name:    "Actuator Chain",
		Type:    "chain",
		Version: "1.0.0",
	}
}

// Add 添加执行器到链中
func (c *ActuatorChain) Add(act Actuator) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.actuators = append(c.actuators, act)
}
