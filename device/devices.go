package device

import (
	"github.com/danclive/nson-go"
	"github.com/snple/beacon/dt"
)

// ============================================================================
// 家庭物联网常见设备定义
// ============================================================================

// -----------------------------------------------------------------------------
// 照明设备
// -----------------------------------------------------------------------------

// SmartBulb 智能灯泡
var SmartBulb = DeviceBuilder("bulb", "智能灯泡").
	Tags("lighting", "smart_home").
	Desc("可调光智能灯泡").
	Wire(WireBuilder("ctrl").
		Desc("灯泡控制").
		Pin(OnOffPin).
		Pin(DimPin).
		Pin(CCTPin),
	).
	Done()

// ColorBulb 彩色智能灯泡
var ColorBulb = DeviceBuilder("bulb_rgb", "彩色智能灯泡").
	Tags("lighting", "smart_home", "color").
	Desc("可调光调色智能灯泡").
	Wire(WireBuilder("ctrl").
		Desc("灯泡控制").
		Pin(OnOffPin).
		Pin(DimPin).
		Pin(CCTPin).
		Pin(RGBPin).
		Pin(HuePin).
		Pin(SatPin).
		Pin(ModePin),
	).
	Done()

// LightStrip LED灯带
var LightStrip = DeviceBuilder("strip", "LED灯带").
	Tags("lighting", "smart_home", "color").
	Desc("可调光调色LED灯带").
	Wire(WireBuilder("ctrl").
		Pin(OnOffPin).
		Pin(DimPin).
		Pin(RGBPin).
		Pin(PinBuilder("fx", dt.TypeU8, RW).
			Desc("灯效").
			Default(nson.U8(0)).
			Enum(
				Enum(nson.U8(0), "静态"),
				Enum(nson.U8(1), "呼吸"),
				Enum(nson.U8(2), "流水"),
				Enum(nson.U8(3), "彩虹"),
			).
			Build(),
		).
		Pin(PinBuilder("spd", dt.TypeU8, RW).
			Desc("灯效速度").
			Default(nson.U8(50)).
			Range(nson.U8(0), nson.U8(100)).
			Step(nson.U8(1)).
			Unit("%").
			Build(),
		),
	).
	Done()

// -----------------------------------------------------------------------------
// 开关插座
// -----------------------------------------------------------------------------

// SmartSwitch 智能开关
var SmartSwitch = DeviceBuilder("sw", "智能开关").
	Tags("switch", "smart_home").
	Desc("单路智能开关").
	Wire(WireBuilder("ctrl").
		Pin(OnOffPin),
	).
	Done()

// SmartSwitch2 双路智能开关
var SmartSwitch2 = DeviceBuilder("sw2", "双路智能开关").
	Tags("switch", "smart_home").
	Wire(WireBuilder("ctrl").
		Pin(PinBuilder("on1", dt.TypeBool, RW).
			Default(nson.Bool(false)).
			Build()).
		Pin(PinBuilder("on2", dt.TypeBool, RW).
			Default(nson.Bool(false)).
			Build()),
	).
	Done()

// SmartSwitch3 三路智能开关
var SmartSwitch3 = DeviceBuilder("sw3", "三路智能开关").
	Tags("switch", "smart_home").
	Wire(WireBuilder("ctrl").
		Pin(PinBuilder("on1", dt.TypeBool, RW).
			Default(nson.Bool(false)).
			Build()).
		Pin(PinBuilder("on2", dt.TypeBool, RW).
			Default(nson.Bool(false)).
			Build()).
		Pin(PinBuilder("on3", dt.TypeBool, RW).
			Default(nson.Bool(false)).
			Build()),
	).
	Done()

// SmartPlug 智能插座
var SmartPlug = DeviceBuilder("plug", "智能插座").
	Tags("plug", "smart_home", "power").
	Desc("带电量统计的智能插座").
	Wire(WireBuilder("ctrl").
		Pin(OnOffPin),
	).
	Wire(WireBuilder("meter").
		Desc("电量统计").
		Pin(PwrPin).
		Pin(VoltPin).
		Pin(CurrPin).
		Pin(KwhPin),
	).
	Done()

// PowerStrip 智能排插
var PowerStrip = DeviceBuilder("pstrip", "智能排插").
	Tags("plug", "smart_home", "power").
	Desc("多路智能排插").
	Wire(WireBuilder("ctrl").
		Pin(OnOffPin).
		Pin(PinBuilder("ch", dt.TypeArray, RW).
			Desc("各路开关状态").
			Build()),
	).
	Wire(WireBuilder("meter").
		Pin(PinBuilder("pwr", dt.TypeF32, RO).
			Desc("总功率").
			Precision(1).
			Unit("W").
			Build()),
	).
	Done()

// -----------------------------------------------------------------------------
// 传感器
// -----------------------------------------------------------------------------

// TempHumiSensor 温湿度传感器
var TempHumiSensor = DeviceBuilder("th", "温湿度传感器").
	Tags("sensor", "smart_home", "env").
	Desc("温湿度传感器").
	Wire(WireBuilder("data").
		Pin(TempPin).
		Pin(HumiPin),
	).
	Done()

// MotionSensor 人体传感器
var MotionSensor = DeviceBuilder("pir", "人体传感器").
	Tags("sensor", "smart_home", "security").
	Desc("红外人体移动传感器").
	Wire(WireBuilder("data").
		Pin(PinBuilder("occ", dt.TypeBool, RO).
			Desc("是否有人").
			Build()).
		Pin(LuxPin).
		Pin(LastPin),
	).
	Done()

// DoorSensor 门窗传感器
var DoorSensor = DeviceBuilder("door", "门窗传感器").
	Tags("sensor", "smart_home", "security").
	Desc("门窗开合传感器").
	Wire(WireBuilder("data").
		Pin(OpenPin).
		Pin(LastPin),
	).
	Done()

// WaterLeakSensor 水浸传感器
var WaterLeakSensor = DeviceBuilder("leak", "水浸传感器").
	Tags("sensor", "smart_home", "security").
	Wire(WireBuilder("data").
		Pin(PinBuilder("leak", dt.TypeBool, RO).
			Desc("是否漏水").
			Build()),
	).
	Done()

// SmokeSensor 烟雾传感器
var SmokeSensor = DeviceBuilder("smoke", "烟雾传感器").
	Tags("sensor", "smart_home", "security").
	Wire(WireBuilder("data").
		Pin(AlarmPin).
		Pin(PinBuilder("ppm", dt.TypeU16, RO).
			Desc("烟雾浓度").
			Unit("ppm").
			Build()),
	).
	Done()

// GasSensor 燃气传感器
var GasSensor = DeviceBuilder("gas", "燃气传感器").
	Tags("sensor", "smart_home", "security").
	Wire(WireBuilder("data").
		Pin(AlarmPin).
		Pin(PinBuilder("ppm", dt.TypeU16, RO).
			Desc("燃气浓度").
			Unit("ppm").
			Build()),
	).
	Done()

// AirQualitySensor 空气质量传感器
var AirQualitySensor = DeviceBuilder("aqs", "空气质量传感器").
	Tags("sensor", "smart_home", "env").
	Desc("多合一空气质量传感器").
	Wire(WireBuilder("data").
		Pin(PM25Pin).
		Pin(PM10Pin).
		Pin(CO2Pin).
		Pin(TVOCPin).
		Pin(AQIPin).
		Pin(HCHOPin).
		Pin(TempPin).
		Pin(HumiPin),
	).
	Done()

// -----------------------------------------------------------------------------
// 窗帘/遮阳
// -----------------------------------------------------------------------------

// Curtain 智能窗帘
var Curtain = DeviceBuilder("curt", "智能窗帘").
	Tags("curtain", "smart_home").
	Desc("电动窗帘").
	Wire(WireBuilder("ctrl").
		Pin(PosPin).
		Pin(CmdPin).
		Pin(RunningPin),
	).
	Done()

// Blind 百叶窗
var Blind = DeviceBuilder("blind", "百叶窗").
	Tags("curtain", "smart_home").
	Wire(WireBuilder("ctrl").
		Pin(PosPin).
		Pin(PinBuilder("tilt", dt.TypeI8, RW).
			Desc("叶片角度").
			Range(nson.I8(-90), nson.I8(90)).
			Step(nson.I8(1)).
			Unit("°").
			Build()).
		Pin(CmdPin),
	).
	Done()

// -----------------------------------------------------------------------------
// 空调/暖通
// -----------------------------------------------------------------------------

// AC 空调
var AC = DeviceBuilder("ac", "空调").
	Tags("hvac", "smart_home").
	Desc("智能空调").
	Wire(WireBuilder("ctrl").
		Pin(OnOffPin).
		Pin(PinBuilder("mode", dt.TypeU8, RW).
			Desc("工作模式").
			Default(nson.U8(0)).
			Enum(
				Enum(nson.U8(0), "自动"),
				Enum(nson.U8(1), "制冷"),
				Enum(nson.U8(2), "制热"),
				Enum(nson.U8(3), "除湿"),
				Enum(nson.U8(4), "送风"),
			).
			Build(),
		).
		Pin(TgtTempPin).
		Pin(PinBuilder("fan", dt.TypeU8, RW).
			Desc("风速").
			Default(nson.U8(0)).
			Enum(
				Enum(nson.U8(0), "自动"),
				Enum(nson.U8(1), "低"),
				Enum(nson.U8(2), "中"),
				Enum(nson.U8(3), "高"),
			).
			Build(),
		).
		Pin(SwingPin),
	).
	Wire(WireBuilder("sensor").
		Pin(TempPin).
		Pin(HumiPin),
	).
	Done()

// Thermostat 温控器
var Thermostat = DeviceBuilder("tstat", "温控器").
	Tags("hvac", "smart_home").
	Desc("智能温控器").
	Wire(WireBuilder("ctrl").
		Pin(OnOffPin).
		Pin(ModePin).
		Pin(TgtTempPin).
		Pin(TempPin).
		Pin(RunningPin),
	).
	Done()

// FloorHeating 地暖
var FloorHeating = DeviceBuilder("fheat", "地暖").
	Tags("hvac", "smart_home").
	Wire(WireBuilder("ctrl").
		Pin(OnOffPin).
		Pin(TgtTempPin).
		Pin(TempPin),
	).
	Done()

// Fan 风扇
var Fan = DeviceBuilder("fan", "风扇").
	Tags("hvac", "smart_home").
	Wire(WireBuilder("ctrl").
		Pin(OnOffPin).
		Pin(PinBuilder("spd", dt.TypeU8, RW).
			Desc("风速档位").
			Default(nson.U8(0)).
			Range(nson.U8(0), nson.U8(6)).
			Step(nson.U8(1)).
			Build()).
		Pin(PinBuilder("swing", dt.TypeBool, RW).
			Desc("摇头").
			Default(nson.Bool(false)).
			Build()).
		Pin(ModePin),
	).
	Done()

// -----------------------------------------------------------------------------
// 安防设备
// -----------------------------------------------------------------------------

// DoorLock 智能门锁
var DoorLock = DeviceBuilder("lock", "智能门锁").
	Tags("security", "smart_home").
	Desc("智能门锁").
	Wire(WireBuilder("ctrl").
		Pin(LockPin).
		Pin(PinBuilder("open", dt.TypeBool, RO).
			Desc("门开合状态").
			Build()),
	).
	Wire(WireBuilder("info").
		Pin(BattPin).
		Pin(PinBuilder("method", dt.TypeU8, RO).
			Desc("最后开锁方式").
			Enum(
				Enum(nson.U8(0), "密码"),
				Enum(nson.U8(1), "指纹"),
				Enum(nson.U8(2), "卡"),
				Enum(nson.U8(3), "钥匙"),
				Enum(nson.U8(4), "远程"),
			).
			Build()).
		Pin(PinBuilder("uid", dt.TypeString, RO).
			Desc("最后开锁用户ID").
			Build()).
		Pin(LastPin),
	).
	Wire(WireBuilder("event").
		Desc("事件记录").
		Pin(PinBuilder("log", dt.TypeArray, RO).
			Desc("开锁记录").
			Build()),
	).
	Done()

// Camera 摄像头
var Camera = DeviceBuilder("cam", "摄像头").
	Tags("security", "smart_home").
	Wire(WireBuilder("ctrl").
		Pin(OnOffPin).
		Pin(PinBuilder("rec", dt.TypeBool, RW).
			Desc("录制").
			Build()).
		Pin(PinBuilder("ptz", dt.TypeMap, WO).
			Desc("云台控制 {pan, tilt, zoom}").
			Build()),
	).
	Wire(WireBuilder("info").
		Pin(OnlinePin).
		Pin(PinBuilder("url", dt.TypeString, RO).
			Desc("视频流地址").
			Build()),
	).
	Done()

// Alarm 报警器
var Alarm = DeviceBuilder("alarm", "报警器").
	Tags("security", "smart_home").
	Wire(WireBuilder("ctrl").
		Pin(PinBuilder("arm", dt.TypeBool, RW).
			Desc("布防").
			Build()).
		Pin(ModePin).
		Pin(AlarmPin).
		Pin(PinBuilder("siren", dt.TypeBool, RW).
			Desc("警笛开关").
			Build()),
	).
	Done()

// -----------------------------------------------------------------------------
// 家电设备
// -----------------------------------------------------------------------------

// WaterHeater 热水器
var WaterHeater = DeviceBuilder("wh", "热水器").
	Tags("appliance", "smart_home").
	Wire(WireBuilder("ctrl").
		Pin(OnOffPin).
		Pin(PinBuilder("tgt", dt.TypeU8, RW).
			Desc("目标温度").
			Range(nson.U8(35), nson.U8(75)).
			Step(nson.U8(1)).
			Unit("°C").
			Build()).
		Pin(PinBuilder("temp", dt.TypeU8, RO).
			Desc("当前温度").
			Unit("°C").
			Build()).
		Pin(ModePin),
	).
	Done()

// Humidifier 加湿器
var Humidifier = DeviceBuilder("humf", "加湿器").
	Tags("appliance", "smart_home", "env").
	Wire(WireBuilder("ctrl").
		Pin(OnOffPin).
		Pin(PinBuilder("tgt", dt.TypeU8, RW).
			Desc("目标湿度").
			Range(nson.U8(30), nson.U8(80)).
			Step(nson.U8(5)).
			Unit("%").
			Build()).
		Pin(HumiPin).
		Pin(PinBuilder("level", dt.TypeU8, RW).
			Desc("档位").
			Range(nson.U8(0), nson.U8(3)).
			Step(nson.U8(1)).
			Build()).
		Pin(PinBuilder("water", dt.TypeBool, RO).
			Desc("缺水").
			Build()),
	).
	Done()

// Dehumidifier 除湿机
var Dehumidifier = DeviceBuilder("dehum", "除湿机").
	Tags("appliance", "smart_home", "env").
	Wire(WireBuilder("ctrl").
		Pin(OnOffPin).
		Pin(PinBuilder("tgt", dt.TypeU8, RW).
			Desc("目标湿度").
			Range(nson.U8(30), nson.U8(80)).
			Step(nson.U8(5)).
			Unit("%").
			Build()).
		Pin(HumiPin).
		Pin(ModePin).
		Pin(PinBuilder("tank", dt.TypeBool, RO).
			Desc("水箱满").
			Build()),
	).
	Done()

// AirPurifier 空气净化器
var AirPurifier = DeviceBuilder("ap", "空气净化器").
	Tags("appliance", "smart_home", "env").
	Wire(WireBuilder("ctrl").
		Pin(OnOffPin).
		Pin(ModePin).
		Pin(FanPin),
	).
	Wire(WireBuilder("sensor").
		Pin(PM25Pin).
		Pin(AQIPin).
		Pin(FilterPin),
	).
	Done()

// RobotVacuum 扫地机器人
var RobotVacuum = DeviceBuilder("vacuum", "扫地机器人").
	Tags("appliance", "smart_home").
	Wire(WireBuilder("ctrl").
		Pin(CmdPin).
		Pin(ModePin).
		Pin(PinBuilder("zone", dt.TypeArray, WO).
			Desc("指定清扫区域").
			Build()),
	).
	Wire(WireBuilder("stat").
		Pin(StatePin).
		Pin(BattPin).
		Pin(PinBuilder("area", dt.TypeU32, RO).
			Desc("已清扫面积").
			Unit("m²").
			Build()).
		Pin(PinBuilder("time", dt.TypeU32, RO).
			Desc("清扫时长").
			Unit("s").
			Build()).
		Pin(ErrPin),
	).
	Done()

// WashingMachine 洗衣机
var WashingMachine = DeviceBuilder("washer", "洗衣机").
	Tags("appliance", "smart_home").
	Wire(WireBuilder("ctrl").
		Pin(CmdPin).
		Pin(PinBuilder("prog", dt.TypeU8, RW).
			Desc("程序").
			Build()).
		Pin(PinBuilder("temp", dt.TypeU8, RW).
			Desc("水温").
			Unit("°C").
			Build()).
		Pin(PinBuilder("spin", dt.TypeU16, RW).
			Desc("转速").
			Unit("rpm").
			Build()),
	).
	Wire(WireBuilder("stat").
		Pin(StatePin).
		Pin(RemainPin).
		Pin(DoorPin),
	).
	Done()

// Dishwasher 洗碗机
var Dishwasher = DeviceBuilder("dw", "洗碗机").
	Tags("appliance", "smart_home").
	Wire(WireBuilder("ctrl").
		Pin(CmdPin).
		Pin(PinBuilder("prog", dt.TypeU8, RW).
			Desc("程序").
			Build()),
	).
	Wire(WireBuilder("stat").
		Pin(StatePin).
		Pin(RemainPin).
		Pin(PinBuilder("salt", dt.TypeBool, RO).
			Desc("缺盐").
			Build()).
		Pin(PinBuilder("rinse", dt.TypeBool, RO).
			Desc("缺漂洗剂").
			Build()),
	).
	Done()

// -----------------------------------------------------------------------------
// 厨房设备
// -----------------------------------------------------------------------------

// SmartOven 智能烤箱
var SmartOven = DeviceBuilder("oven", "智能烤箱").
	Tags("kitchen", "smart_home").
	Wire(WireBuilder("ctrl").
		Pin(OnOffPin).
		Pin(PinBuilder("tgt", dt.TypeU16, RW).
			Desc("目标温度").
			Range(nson.U16(50), nson.U16(250)).
			Step(nson.U16(5)).
			Unit("°C").
			Build()).
		Pin(PinBuilder("time", dt.TypeU16, RW).
			Desc("定时").
			Unit("min").
			Build()).
		Pin(ModePin),
	).
	Wire(WireBuilder("stat").
		Pin(PinBuilder("temp", dt.TypeU16, RO).
			Desc("当前温度").
			Unit("°C").
			Build()).
		Pin(RemainPin).
		Pin(DoorPin),
	).
	Done()

// Refrigerator 冰箱
var Refrigerator = DeviceBuilder("fridge", "冰箱").
	Tags("kitchen", "smart_home").
	Wire(WireBuilder("ctrl").
		Pin(ModePin).
		Pin(PinBuilder("ftgt", dt.TypeI8, RW).
			Desc("冷藏室目标温度").
			Range(nson.I8(2), nson.I8(8)).
			Step(nson.I8(1)).
			Unit("°C").
			Build()).
		Pin(PinBuilder("ztgt", dt.TypeI8, RW).
			Desc("冷冻室目标温度").
			Range(nson.I8(-24), nson.I8(-16)).
			Step(nson.I8(1)).
			Unit("°C").
			Build()),
	).
	Wire(WireBuilder("sensor").
		Pin(PinBuilder("ftemp", dt.TypeI8, RO).
			Desc("冷藏室温度").
			Unit("°C").
			Build()).
		Pin(PinBuilder("ztemp", dt.TypeI8, RO).
			Desc("冷冻室温度").
			Unit("°C").
			Build()).
		Pin(PinBuilder("door", dt.TypeU8, RO).
			Desc("门状态 位图").
			Build()),
	).
	Done()

// CoffeeMaker 咖啡机
var CoffeeMaker = DeviceBuilder("coffee", "咖啡机").
	Tags("kitchen", "smart_home").
	Wire(WireBuilder("ctrl").
		Pin(PinBuilder("brew", dt.TypeBool, WO).
			Desc("开始冲泡").
			Build()).
		Pin(PinBuilder("type", dt.TypeU8, RW).
			Desc("类型").
			Build()).
		Pin(PinBuilder("size", dt.TypeU8, RW).
			Desc("杯量").
			Build()).
		Pin(PinBuilder("str", dt.TypeU8, RW).
			Desc("浓度").
			Range(nson.U8(0), nson.U8(10)).
			Step(nson.U8(1)).
			Build()),
	).
	Wire(WireBuilder("stat").
		Pin(PinBuilder("ready", dt.TypeBool, RO).
			Desc("就绪").
			Build()).
		Pin(PinBuilder("water", dt.TypeBool, RO).
			Desc("缺水").
			Build()).
		Pin(PinBuilder("bean", dt.TypeBool, RO).
			Desc("缺豆").
			Build()).
		Pin(PinBuilder("tray", dt.TypeBool, RO).
			Desc("残渣盒满").
			Build()),
	).
	Done()

// -----------------------------------------------------------------------------
// 网关/控制器
// -----------------------------------------------------------------------------

// Gateway 智能网关
var Gateway = DeviceBuilder("gw", "智能网关").
	Tags("gateway", "smart_home").
	Desc("智能家居网关").
	Wire(WireBuilder("info").
		Pin(OnlinePin).
		Pin(VerPin).
		Pin(MacPin).
		Pin(IpPin).
		Pin(UptimePin),
	).
	Wire(WireBuilder("devs").
		Desc("子设备管理").
		Pin(PinBuilder("list", dt.TypeArray, RO).
			Desc("子设备列表").
			Build()).
		Pin(PinBuilder("count", dt.TypeU16, RO).
			Desc("子设备数量").
			Build()),
	).
	Done()

// IRRemote 万能遥控器
var IRRemote = DeviceBuilder("ir", "万能遥控器").
	Tags("controller", "smart_home").
	Wire(WireBuilder("ctrl").
		Pin(PinBuilder("send", dt.TypeBinary, WO).
			Desc("发送红外码").
			Build()).
		Pin(PinBuilder("learn", dt.TypeBool, RW).
			Desc("学习模式").
			Build()).
		Pin(PinBuilder("code", dt.TypeBinary, RO).
			Desc("学习到的红外码").
			Build()),
	).
	Done()

// SceneController 场景控制器
var SceneController = DeviceBuilder("scene", "场景控制器").
	Tags("controller", "smart_home").
	Wire(WireBuilder("ctrl").
		Pin(PinBuilder("exec", dt.TypeString, WO).
			Desc("执行场景ID").
			Build()).
		Pin(PinBuilder("list", dt.TypeArray, RO).
			Desc("场景列表").
			Build()),
	).
	Wire(WireBuilder("sched").
		Desc("定时任务").
		Pin(PinBuilder("tasks", dt.TypeArray, RW).
			Desc("定时任务列表").
			Build()),
	).
	Done()
