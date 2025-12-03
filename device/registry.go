package device

import "sync"

// ============================================================================
// 设备注册表（线程安全）
// ============================================================================

var (
	registry = make(map[string]Device)
	mu       sync.RWMutex
)

func init() {
	// 自动注册标准设备 - 照明
	Register(SmartBulb)
	Register(ColorBulb)
	Register(LightStrip)

	// 开关插座
	Register(SmartSwitch)
	Register(SmartSwitch2)
	Register(SmartSwitch3)
	Register(SmartPlug)
	Register(PowerStrip)

	// 传感器
	Register(TempHumiSensor)
	Register(MotionSensor)
	Register(DoorSensor)
	Register(WaterLeakSensor)
	Register(SmokeSensor)
	Register(GasSensor)
	Register(AirQualitySensor)

	// 窗帘
	Register(Curtain)
	Register(Blind)

	// 空调暖通
	Register(AC)
	Register(Thermostat)
	Register(FloorHeating)
	Register(Fan)

	// 安防
	Register(DoorLock)
	Register(Camera)
	Register(Alarm)

	// 家电
	Register(WaterHeater)
	Register(Humidifier)
	Register(Dehumidifier)
	Register(AirPurifier)
	Register(RobotVacuum)
	Register(WashingMachine)
	Register(Dishwasher)

	// 厨房
	Register(SmartOven)
	Register(Refrigerator)
	Register(CoffeeMaker)

	// 网关控制器
	Register(Gateway)
	Register(IRRemote)
	Register(SceneController)
}

// Register 注册设备
func Register(device Device) {
	mu.Lock()
	defer mu.Unlock()
	registry[device.ID] = device
}

// Get 获取设备定义
func Get(id string) (Device, bool) {
	mu.RLock()
	defer mu.RUnlock()
	dev, ok := registry[id]
	return dev, ok
}

// List 列出所有设备
func List() []Device {
	mu.RLock()
	defer mu.RUnlock()
	devices := make([]Device, 0, len(registry))
	for _, dev := range registry {
		devices = append(devices, dev)
	}
	return devices
}

// ByTag 按标签获取设备
func ByTag(tag string) []Device {
	mu.RLock()
	defer mu.RUnlock()
	devices := make([]Device, 0)
	for _, dev := range registry {
		for _, t := range dev.Tags {
			if t == tag {
				devices = append(devices, dev)
				break
			}
		}
	}
	return devices
}

// AllTags 获取所有标签
func AllTags() []string {
	mu.RLock()
	defer mu.RUnlock()
	tagMap := make(map[string]bool)
	for _, dev := range registry {
		for _, tag := range dev.Tags {
			tagMap[tag] = true
		}
	}
	tags := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		tags = append(tags, tag)
	}
	return tags
}
