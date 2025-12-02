package device

// ============================================================================
// 标准家庭物联网设备模板
// ============================================================================

func init() {
	// 注册所有标准设备模板
	registerLightingDevices()
	registerSensorDevices()
	registerSwitchDevices()
	registerSecurityDevices()
	registerClimateDevices()
	registerApplianceDevices()
}

// ============================================================================
// 照明设备
// ============================================================================

func registerLightingDevices() {
	// 1. 智能开关灯泡
	RegisterDevice(&DeviceTemplate{
		ID:           "smart_bulb_onoff",
		Name:         "智能开关灯泡",
		Category:     CategoryLighting,
		Description:  "支持开关控制的智能灯泡",
		Manufacturer: "Generic",
		Model:        "SB-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "light", Clusters: []string{"OnOff"}, Required: true},
		},
	})

	// 2. 可调光灯泡
	RegisterDevice(&DeviceTemplate{
		ID:           "smart_bulb_dimmable",
		Name:         "可调光智能灯泡",
		Category:     CategoryLighting,
		Description:  "支持开关和亮度调节的智能灯泡",
		Manufacturer: "Generic",
		Model:        "SB-002",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "light", Clusters: []string{"OnOff", "LevelControl"}, Required: true},
		},
	})

	// 3. 彩色智能灯泡
	RegisterDevice(&DeviceTemplate{
		ID:           "smart_bulb_color",
		Name:         "彩色智能灯泡",
		Category:     CategoryLighting,
		Description:  "支持开关、亮度和颜色调节的智能灯泡",
		Manufacturer: "Generic",
		Model:        "SB-003",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "light", Clusters: []string{"OnOff", "LevelControl", "ColorControl"}, Required: true},
		},
	})

	// 4. LED 灯带
	RegisterDevice(&DeviceTemplate{
		ID:           "led_strip",
		Name:         "LED 智能灯带",
		Category:     CategoryLighting,
		Description:  "支持颜色和效果控制的 LED 灯带",
		Manufacturer: "Generic",
		Model:        "LS-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "strip", Clusters: []string{"OnOff", "LevelControl", "ColorControl"}, Required: true},
		},
	})
}

// ============================================================================
// 传感器设备
// ============================================================================

func registerSensorDevices() {
	// 1. 温湿度传感器
	RegisterDevice(&DeviceTemplate{
		ID:           "temp_humi_sensor",
		Name:         "温湿度传感器",
		Category:     CategorySensor,
		Description:  "测量环境温度和湿度",
		Manufacturer: "Generic",
		Model:        "TH-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "sensor", Clusters: []string{"TemperatureMeasurement", "HumidityMeasurement"}, Required: true},
		},
	})

	// 2. 温度传感器
	RegisterDevice(&DeviceTemplate{
		ID:           "temperature_sensor",
		Name:         "温度传感器",
		Category:     CategorySensor,
		Description:  "测量环境温度",
		Manufacturer: "Generic",
		Model:        "TS-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "sensor", Clusters: []string{"TemperatureMeasurement"}, Required: true},
		},
	})

	// 3. 湿度传感器
	RegisterDevice(&DeviceTemplate{
		ID:           "humidity_sensor",
		Name:         "湿度传感器",
		Category:     CategorySensor,
		Description:  "测量环境湿度",
		Manufacturer: "Generic",
		Model:        "HS-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "sensor", Clusters: []string{"HumidityMeasurement"}, Required: true},
		},
	})

	// 4. 空气质量传感器
	RegisterDevice(&DeviceTemplate{
		ID:           "air_quality_sensor",
		Name:         "空气质量传感器",
		Category:     CategorySensor,
		Description:  "测量 PM2.5、CO2 等空气质量指标",
		Manufacturer: "Generic",
		Model:        "AQ-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "sensor", Clusters: []string{"TemperatureMeasurement", "HumidityMeasurement"}, Required: true},
		},
	})
}

// ============================================================================
// 开关设备
// ============================================================================

func registerSwitchDevices() {
	// 1. 单路开关
	RegisterDevice(&DeviceTemplate{
		ID:           "switch_1gang",
		Name:         "单路智能开关",
		Category:     CategorySwitch,
		Description:  "单路墙壁开关",
		Manufacturer: "Generic",
		Model:        "SW-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "switch1", Clusters: []string{"OnOff"}, Required: true},
		},
	})

	// 2. 双路开关
	RegisterDevice(&DeviceTemplate{
		ID:           "switch_2gang",
		Name:         "双路智能开关",
		Category:     CategorySwitch,
		Description:  "双路墙壁开关",
		Manufacturer: "Generic",
		Model:        "SW-002",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "switch1", Clusters: []string{"OnOff"}, Required: true},
			{Name: "switch2", Clusters: []string{"OnOff"}, Required: true},
		},
	})

	// 3. 三路开关
	RegisterDevice(&DeviceTemplate{
		ID:           "switch_3gang",
		Name:         "三路智能开关",
		Category:     CategorySwitch,
		Description:  "三路墙壁开关",
		Manufacturer: "Generic",
		Model:        "SW-003",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "switch1", Clusters: []string{"OnOff"}, Required: true},
			{Name: "switch2", Clusters: []string{"OnOff"}, Required: true},
			{Name: "switch3", Clusters: []string{"OnOff"}, Required: true},
		},
	})

	// 4. 智能插座
	RegisterDevice(&DeviceTemplate{
		ID:           "smart_socket",
		Name:         "智能插座",
		Category:     CategorySocket,
		Description:  "支持远程控制的智能插座",
		Manufacturer: "Generic",
		Model:        "SK-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "outlet", Clusters: []string{"OnOff"}, Required: true},
		},
	})
}

// ============================================================================
// 安防设备
// ============================================================================

func registerSecurityDevices() {
	// 1. 门窗传感器
	RegisterDevice(&DeviceTemplate{
		ID:           "door_window_sensor",
		Name:         "门窗传感器",
		Category:     CategorySecuritySensor,
		Description:  "检测门窗开关状态",
		Manufacturer: "Generic",
		Model:        "DW-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "sensor", Clusters: []string{"OnOff"}, Required: true}, // OnOff 代表开关状态
		},
	})

	// 2. 人体传感器
	RegisterDevice(&DeviceTemplate{
		ID:           "motion_sensor",
		Name:         "人体传感器",
		Category:     CategorySecuritySensor,
		Description:  "检测人体移动",
		Manufacturer: "Generic",
		Model:        "MS-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "sensor", Clusters: []string{"OnOff"}, Required: true}, // OnOff 代表是否检测到移动
		},
	})

	// 3. 烟雾传感器
	RegisterDevice(&DeviceTemplate{
		ID:           "smoke_sensor",
		Name:         "烟雾传感器",
		Category:     CategorySecuritySensor,
		Description:  "检测烟雾和火灾",
		Manufacturer: "Generic",
		Model:        "SM-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "sensor", Clusters: []string{"OnOff"}, Required: true}, // OnOff 代表是否检测到烟雾
		},
	})

	// 4. 水浸传感器
	RegisterDevice(&DeviceTemplate{
		ID:           "water_leak_sensor",
		Name:         "水浸传感器",
		Category:     CategorySecuritySensor,
		Description:  "检测漏水情况",
		Manufacturer: "Generic",
		Model:        "WL-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "sensor", Clusters: []string{"OnOff"}, Required: true}, // OnOff 代表是否检测到漏水
		},
	})

	// 5. 智能门锁
	RegisterDevice(&DeviceTemplate{
		ID:           "smart_lock",
		Name:         "智能门锁",
		Category:     CategoryLock,
		Description:  "支持多种开锁方式的智能门锁",
		Manufacturer: "Generic",
		Model:        "SL-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "lock", Clusters: []string{"OnOff"}, Required: true}, // OnOff 代表锁定/解锁状态
		},
	})
}

// ============================================================================
// 环境控制设备
// ============================================================================

func registerClimateDevices() {
	// 1. 电动窗帘
	RegisterDevice(&DeviceTemplate{
		ID:           "smart_curtain",
		Name:         "智能窗帘",
		Category:     CategoryCurtain,
		Description:  "电动控制的智能窗帘",
		Manufacturer: "Generic",
		Model:        "CT-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "curtain", Clusters: []string{"OnOff", "LevelControl"}, Required: true}, // LevelControl 控制开合程度
		},
	})

	// 2. 空调控制器
	RegisterDevice(&DeviceTemplate{
		ID:           "ac_controller",
		Name:         "空调控制器",
		Category:     CategoryAirConditioner,
		Description:  "红外或智能协议控制空调",
		Manufacturer: "Generic",
		Model:        "AC-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "ac", Clusters: []string{"OnOff", "TemperatureMeasurement"}, Required: true},
		},
	})

	// 3. 智能风扇
	RegisterDevice(&DeviceTemplate{
		ID:           "smart_fan",
		Name:         "智能风扇",
		Category:     CategoryFan,
		Description:  "支持速度调节的智能风扇",
		Manufacturer: "Generic",
		Model:        "FN-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "fan", Clusters: []string{"OnOff", "LevelControl"}, Required: true}, // LevelControl 控制风速
		},
	})

	// 4. 加湿器
	RegisterDevice(&DeviceTemplate{
		ID:           "humidifier",
		Name:         "智能加湿器",
		Category:     CategoryHumidifier,
		Description:  "支持湿度控制的智能加湿器",
		Manufacturer: "Generic",
		Model:        "HF-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "humidifier", Clusters: []string{"OnOff", "HumidityMeasurement"}, Required: true},
		},
	})

	// 5. 空气净化器
	RegisterDevice(&DeviceTemplate{
		ID:           "air_purifier",
		Name:         "空气净化器",
		Category:     CategoryAirPurifier,
		Description:  "智能空气净化器",
		Manufacturer: "Generic",
		Model:        "AP-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "purifier", Clusters: []string{"OnOff", "LevelControl"}, Required: true}, // LevelControl 控制风速档位
		},
	})
}

// ============================================================================
// 家电设备
// ============================================================================

func registerApplianceDevices() {
	// 1. 智能网关
	RegisterDevice(&DeviceTemplate{
		ID:           "smart_gateway",
		Name:         "智能网关",
		Category:     CategoryGateway,
		Description:  "连接各种子设备的中心网关",
		Manufacturer: "Generic",
		Model:        "GW-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
		},
	})

	// 2. 智能门铃
	RegisterDevice(&DeviceTemplate{
		ID:           "smart_doorbell",
		Name:         "智能门铃",
		Category:     CategoryDoorbell,
		Description:  "支持视频通话的智能门铃",
		Manufacturer: "Generic",
		Model:        "DB-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "doorbell", Clusters: []string{"OnOff"}, Required: true}, // OnOff 代表按下状态
		},
	})

	// 3. 智能摄像头
	RegisterDevice(&DeviceTemplate{
		ID:           "smart_camera",
		Name:         "智能摄像头",
		Category:     CategoryCamera,
		Description:  "支持远程查看的智能摄像头",
		Manufacturer: "Generic",
		Model:        "CM-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "camera", Clusters: []string{"OnOff"}, Required: true}, // OnOff 代表开关状态
		},
	})

	// 4. 智能报警器
	RegisterDevice(&DeviceTemplate{
		ID:           "smart_alarm",
		Name:         "智能报警器",
		Category:     CategoryAlarm,
		Description:  "智能报警系统",
		Manufacturer: "Generic",
		Model:        "AL-001",
		Version:      "1.0",
		Wires: []WireTemplate{
			{Name: "root", Clusters: []string{"BasicInformation"}, Required: true},
			{Name: "alarm", Clusters: []string{"OnOff"}, Required: true}, // OnOff 代表报警状态
		},
	})
}
