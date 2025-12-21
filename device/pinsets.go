package device

import (
	"github.com/danclive/nson-go"
)

// ============================================================================
// 常用 Pin 定义
// ============================================================================

// --- 基础控制 ---

// OnOffPin 开关控制
var OnOffPin = PinBuilder("on", nson.DataTypeBOOL, RW).
	Desc("开关").
	Default(nson.Bool(false)).
	Build()

// DimPin 调光控制
var DimPin = PinBuilder("dim", nson.DataTypeU8, RW).
	Desc("亮度").
	Default(nson.U8(100)).
	Range(nson.U8(0), nson.U8(100)).
	Step(nson.U8(1)).
	Unit("%").
	Build()

// CCTPin 色温控制
var CCTPin = PinBuilder("cct", nson.DataTypeU16, RW).
	Desc("色温").
	Default(nson.U16(4000)).
	Range(nson.U16(2700), nson.U16(6500)).
	Step(nson.U16(100)).
	Unit("K").
	Build()

// RGBPin RGB颜色控制
var RGBPin = PinBuilder("rgb", nson.DataTypeU32, RW).
	Desc("RGB颜色 0xRRGGBB").
	Default(nson.U32(0xFFFFFF)).
	Range(nson.U32(0x000000), nson.U32(0xFFFFFF)).
	Build()

// HuePin 色相
var HuePin = PinBuilder("hue", nson.DataTypeU16, RW).
	Desc("色相").
	Default(nson.U16(0)).
	Range(nson.U16(0), nson.U16(360)).
	Step(nson.U16(1)).
	Unit("°").
	Build()

// SatPin 饱和度
var SatPin = PinBuilder("sat", nson.DataTypeU8, RW).
	Desc("饱和度").
	Default(nson.U8(100)).
	Range(nson.U8(0), nson.U8(100)).
	Step(nson.U8(1)).
	Unit("%").
	Build()

// ModePin 模式控制
var ModePin = PinBuilder("mode", nson.DataTypeU8, RW).
	Desc("工作模式").
	Default(nson.U8(0)).
	Build()

// --- 温湿度 ---

// TempPin 温度（0.1°C）
var TempPin = PinBuilder("temp", nson.DataTypeI16, RO).
	Desc("温度").
	Default(nson.I16(250)).
	Range(nson.I16(-400), nson.I16(800)). // -40°C ~ 80°C
	Step(nson.I16(1)).
	Precision(1).
	Unit("°C").
	Build()

// HumiPin 湿度
var HumiPin = PinBuilder("humi", nson.DataTypeU8, RO).
	Desc("湿度").
	Default(nson.U8(50)).
	Range(nson.U8(0), nson.U8(100)).
	Step(nson.U8(1)).
	Unit("%").
	Build()

// TgtTempPin 目标温度
var TgtTempPin = PinBuilder("tgt", nson.DataTypeI16, RW).
	Desc("目标温度").
	Default(nson.I16(250)).
	Range(nson.I16(160), nson.I16(320)). // 16°C ~ 32°C
	Step(nson.I16(5)).
	Precision(1).
	Unit("°C").
	Build()

// --- 电量统计 ---

// PwrPin 实时功率
var PwrPin = PinBuilder("pwr", nson.DataTypeF32, RO).
	Desc("实时功率").
	Default(nson.F32(0)).
	Precision(1).
	Unit("W").
	Build()

// VoltPin 电压
var VoltPin = PinBuilder("v", nson.DataTypeF32, RO).
	Desc("电压").
	Default(nson.F32(220)).
	Precision(1).
	Unit("V").
	Build()

// CurrPin 电流
var CurrPin = PinBuilder("i", nson.DataTypeF32, RO).
	Desc("电流").
	Default(nson.F32(0)).
	Precision(2).
	Unit("A").
	Build()

// KwhPin 累计电量
var KwhPin = PinBuilder("kwh", nson.DataTypeF32, RO).
	Desc("累计电量").
	Default(nson.F32(0)).
	Precision(2).
	Unit("kWh").
	Build()

// --- 电池 ---

// BattPin 电池电量
var BattPin = PinBuilder("batt", nson.DataTypeU8, RO).
	Desc("电量").
	Default(nson.U8(100)).
	Range(nson.U8(0), nson.U8(100)).
	Unit("%").
	Build()

// --- 状态 ---

// OnlinePin 在线状态
var OnlinePin = PinBuilder("online", nson.DataTypeBOOL, RO).
	Desc("在线状态").
	Default(nson.Bool(false)).
	Build()

// AlarmPin 报警状态
var AlarmPin = PinBuilder("alarm", nson.DataTypeBOOL, RO).
	Desc("报警状态").
	Default(nson.Bool(false)).
	Build()

// RunningPin 运行状态
var RunningPin = PinBuilder("run", nson.DataTypeBOOL, RO).
	Desc("运行中").
	Default(nson.Bool(false)).
	Build()

// StatePin 状态码
var StatePin = PinBuilder("state", nson.DataTypeU8, RO).
	Desc("状态").
	Default(nson.U8(0)).
	Build()

// ErrPin 故障码
var ErrPin = PinBuilder("err", nson.DataTypeU8, RO).
	Desc("故障码").
	Default(nson.U8(0)).
	Build()

// --- 时间戳 ---

// LastPin 最后触发时间
var LastPin = PinBuilder("last", nson.DataTypeTIMESTAMP, RO).
	Desc("最后触发时间").
	Build()

// --- 设备信息 ---

// VerPin 固件版本
var VerPin = PinBuilder("ver", nson.DataTypeSTRING, RO).
	Desc("固件版本").
	Default(nson.String("")).
	Build()

// MacPin MAC地址
var MacPin = PinBuilder("mac", nson.DataTypeSTRING, RO).
	Desc("MAC地址").
	Default(nson.String("")).
	Build()

// IpPin IP地址
var IpPin = PinBuilder("ip", nson.DataTypeSTRING, RO).
	Desc("IP地址").
	Default(nson.String("")).
	Build()

// UptimePin 运行时长
var UptimePin = PinBuilder("uptime", nson.DataTypeU32, RO).
	Desc("运行时长").
	Default(nson.U32(0)).
	Unit("s").
	Build()

// --- 光照 ---

// LuxPin 光照度
var LuxPin = PinBuilder("lux", nson.DataTypeU16, RO).
	Desc("光照度").
	Default(nson.U16(0)).
	Unit("lux").
	Build()

// --- 空气质量 ---

// PM25Pin PM2.5
var PM25Pin = PinBuilder("pm25", nson.DataTypeU16, RO).
	Desc("PM2.5").
	Default(nson.U16(0)).
	Unit("μg/m³").
	Build()

// PM10Pin PM10
var PM10Pin = PinBuilder("pm10", nson.DataTypeU16, RO).
	Desc("PM10").
	Default(nson.U16(0)).
	Unit("μg/m³").
	Build()

// CO2Pin CO2浓度
var CO2Pin = PinBuilder("co2", nson.DataTypeU16, RO).
	Desc("CO2").
	Default(nson.U16(400)).
	Unit("ppm").
	Build()

// TVOCPin TVOC
var TVOCPin = PinBuilder("tvoc", nson.DataTypeU16, RO).
	Desc("TVOC").
	Default(nson.U16(0)).
	Unit("ppb").
	Build()

// AQIPin AQI指数
var AQIPin = PinBuilder("aqi", nson.DataTypeU16, RO).
	Desc("AQI指数").
	Default(nson.U16(0)).
	Build()

// HCHOPin 甲醛
var HCHOPin = PinBuilder("hcho", nson.DataTypeU16, RO).
	Desc("甲醛").
	Default(nson.U16(0)).
	Unit("ppb").
	Build()

// --- 位置/进度 ---

// PosPin 位置 0-100%
var PosPin = PinBuilder("pos", nson.DataTypeU8, RW).
	Desc("位置").
	Default(nson.U8(0)).
	Range(nson.U8(0), nson.U8(100)).
	Step(nson.U8(1)).
	Unit("%").
	Build()

// RemainPin 剩余时间
var RemainPin = PinBuilder("remain", nson.DataTypeU16, RO).
	Desc("剩余时间").
	Default(nson.U16(0)).
	Unit("min").
	Build()

// --- 命令 ---

// CmdPin 命令
var CmdPin = PinBuilder("cmd", nson.DataTypeU8, WO).
	Desc("命令").
	Build()

// --- 风扇/风速 ---

// FanPin 风速
var FanPin = PinBuilder("fan", nson.DataTypeU8, RW).
	Desc("风速").
	Default(nson.U8(0)).
	Range(nson.U8(0), nson.U8(100)).
	Step(nson.U8(1)).
	Unit("%").
	Build()

// SwingPin 摆风
var SwingPin = PinBuilder("swing", nson.DataTypeU8, RW).
	Desc("摆风").
	Default(nson.U8(0)).
	Enum(
		Enum(nson.U8(0), "关闭"),
		Enum(nson.U8(1), "上下"),
		Enum(nson.U8(2), "左右"),
		Enum(nson.U8(3), "全向"),
	).
	Build()

// --- 门/锁 ---

// DoorPin 门状态
var DoorPin = PinBuilder("door", nson.DataTypeBOOL, RO).
	Desc("门状态").
	Default(nson.Bool(false)).
	Build()

// LockPin 锁状态
var LockPin = PinBuilder("lock", nson.DataTypeBOOL, RW).
	Desc("锁定/解锁").
	Default(nson.Bool(true)).
	Build()

// OpenPin 开合状态
var OpenPin = PinBuilder("open", nson.DataTypeBOOL, RO).
	Desc("开/合状态").
	Default(nson.Bool(false)).
	Build()

// --- 滤芯/耗材 ---

// FilterPin 滤芯寿命
var FilterPin = PinBuilder("filter", nson.DataTypeU8, RO).
	Desc("滤芯寿命").
	Default(nson.U8(100)).
	Range(nson.U8(0), nson.U8(100)).
	Unit("%").
	Build()
