package device

import "github.com/snple/beacon/dt"

// ============================================================================
// 标准设备定义（使用 Builder 链式 API）
// ============================================================================

var (
	// SmartBulbOnOff 智能开关灯泡
	SmartBulbOnOff = New("smart_bulb_onoff", "智能开关灯泡").
			Tags("lighting", "smart_home").
			Wire("root").
			Pins(BasicInfoPins).
			Wire("light").
			Pins(OnOffPins).
			Done()

	// SmartBulbDimmable 可调光智能灯泡
	SmartBulbDimmable = New("smart_bulb_dimmable", "可调光智能灯泡").
				Tags("lighting", "smart_home", "dimmable").
				Wire("root").
				Pins(BasicInfoPins).
				Wire("light").
				Pins(OnOffPins).
				Pins(LevelControlPins).
				Done()

	// SmartBulbColor 彩色智能灯泡
	SmartBulbColor = New("smart_bulb_color", "彩色智能灯泡").
			Tags("lighting", "smart_home", "color").
			Wire("root").
			Pins(BasicInfoPins).
			Wire("light").
			Pins(OnOffPins).
			Pins(LevelControlPins).
			Pins(ColorControlPins).
			Done()

	// TempHumiSensor 温湿度传感器
	TempHumiSensor = New("temp_humi_sensor", "温湿度传感器").
			Tags("sensor", "environment").
			Wire("root").
			Pins(BasicInfoPins).
			Wire("sensor").
			Pins(TemperaturePins).
			Pins(HumidityPins).
			Done()

	// Switch2Gang 双路开关
	Switch2Gang = New("switch_2gang", "双路智能开关").
			Tags("switch", "smart_home").
			Wire("root").
			Pins(BasicInfoPins).
			Wire("switch1").
			Pin("onoff", dt.TypeBool, RW).
			Wire("switch2").
			Pin("onoff", dt.TypeBool, RW).
			Done()

	// SmartSocket 智能插座
	SmartSocket = New("smart_socket", "智能插座").
			Tags("socket", "smart_home").
			Wire("root").
			Pins(BasicInfoPins).
			Wire("socket").
			Pins(OnOffPins).
			Done()
)
