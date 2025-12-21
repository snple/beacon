package actuators

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/device"
)

// ============================================================================
// GPIO Actuator - 用于树莓派等开发板的 GPIO 控制
// ============================================================================

func init() {
	device.RegisterActuator("gpio", func() device.Actuator {
		return &GPIOActuator{
			pins: make(map[string]*GPIOPin),
		}
	})
}

// GPIOActuator GPIO 执行器
type GPIOActuator struct {
	mu   sync.RWMutex
	pins map[string]*GPIOPin // pinName -> GPIOPin
}

// GPIOPin GPIO 引脚抽象
type GPIOPin struct {
	Number   int  // GPIO 引脚号
	IsOutput bool // 是否为输出
	State    bool // 当前状态（仅输出）
	PullUp   bool // 是否启用上拉
	PullDown bool // 是否启用下拉
	Inverted bool // 是否反相（高电平=false）
}

func (a *GPIOActuator) Init(ctx context.Context, config device.ActuatorConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 初始化 pins map
	a.pins = make(map[string]*GPIOPin)

	// 从 Pin.Addr 解析 GPIO 配置
	// 格式: "GPIO17" 或 "17:out:pullup" 或 "17:in:pulldown:inverted"
	for _, pin := range config.Pins {
		if pin.Addr == "" {
			continue
		}

		gpioPin, err := parseGPIOAddr(pin.Addr)
		if err != nil {
			return fmt.Errorf("parse GPIO addr for pin %s: %w", pin.Name, err)
		}

		// 根据 Pin.Rw 设置方向
		gpioPin.IsOutput = (pin.Rw == device.RW || pin.Rw == device.WO)

		a.pins[pin.Name] = gpioPin

		// TODO: 实际的 GPIO 初始化
		// 这里需要调用具体的 GPIO 库，如 periph.io 或 sysfs
		// 示例:
		// if err := initGPIO(gpioPin); err != nil {
		//     return err
		// }
	}

	return nil
}

func (a *GPIOActuator) Execute(ctx context.Context, pinName string, value nson.Value) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	gpioPin, ok := a.pins[pinName]
	if !ok {
		return fmt.Errorf("GPIO pin not found: %s", pinName)
	}

	if !gpioPin.IsOutput {
		return fmt.Errorf("GPIO pin %s is not output", pinName)
	}

	// 转换为布尔值
	state := false
	switch value.DataType() {
	case nson.DataTypeBOOL:
		if boolVal, ok := value.(nson.Bool); ok {
			state = bool(boolVal)
		}
	case nson.DataTypeU8:
		if u8Val, ok := value.(nson.U8); ok {
			state = u8Val > 0
		}
	case nson.DataTypeU16:
		if u16Val, ok := value.(nson.U16); ok {
			state = u16Val > 0
		}
	case nson.DataTypeU32:
		if u32Val, ok := value.(nson.U32); ok {
			state = u32Val > 0
		}
	default:
		return fmt.Errorf("unsupported value type for GPIO: %v", value.DataType())
	}

	// 处理反相
	if gpioPin.Inverted {
		state = !state
	}

	// TODO: 实际写入 GPIO
	// setGPIO(gpioPin.Number, state)
	gpioPin.State = state

	return nil
}

func (a *GPIOActuator) Read(ctx context.Context, pinNames []string) (map[string]nson.Value, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string]nson.Value)

	// 确定要读取的 Pin
	toRead := pinNames
	if toRead == nil {
		toRead = make([]string, 0, len(a.pins))
		for name := range a.pins {
			toRead = append(toRead, name)
		}
	}

	for _, name := range toRead {
		gpioPin, ok := a.pins[name]
		if !ok {
			continue
		}

		// TODO: 实际读取 GPIO
		// state := readGPIO(gpioPin.Number)
		state := gpioPin.State

		// 处理反相
		if gpioPin.Inverted {
			state = !state
		}

		result[name] = nson.Bool(state)
	}

	return result, nil
}

func (a *GPIOActuator) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// TODO: 清理 GPIO 资源
	a.pins = make(map[string]*GPIOPin)
	return nil
}

func (a *GPIOActuator) Info() device.ActuatorInfo {
	return device.ActuatorInfo{
		Name:         "GPIO Actuator",
		Type:         "gpio",
		Version:      "1.0.0",
		Capabilities: []string{"read", "write"},
	}
}

// parseGPIOAddr 解析 GPIO 地址配置
// 支持格式:
// - "GPIO17" 或 "17" -> 引脚17，默认配置
// - "17:out" -> 输出模式
// - "17:in:pullup" -> 输入模式，上拉
// - "17:out:inverted" -> 输出模式，反相
func parseGPIOAddr(addr string) (*GPIOPin, error) {
	pin := &GPIOPin{}

	// 简单解析（实际应该更健壮）
	var numStr string
	if len(addr) > 4 && addr[:4] == "GPIO" {
		numStr = addr[4:]
	} else {
		numStr = addr
	}

	// 解析引脚号（第一个冒号之前）
	parts := []byte(numStr)
	numEnd := len(parts)
	for i, c := range parts {
		if c == ':' {
			numEnd = i
			break
		}
	}

	num, err := strconv.Atoi(string(parts[:numEnd]))
	if err != nil {
		return nil, fmt.Errorf("invalid GPIO number: %s", addr)
	}
	pin.Number = num

	// 解析其他选项
	if numEnd < len(parts) {
		options := string(parts[numEnd+1:])
		// 简单的字符串匹配
		// TODO: 更严格的解析
		_ = options
		// if strings.Contains(options, "out") { pin.IsOutput = true }
		// if strings.Contains(options, "pullup") { pin.PullUp = true }
		// ...
	}

	return pin, nil
}
