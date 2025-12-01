package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/consts"
	"github.com/snple/beacon/edge/model"
	"github.com/snple/beacon/util"
)

// ConfigGenerator 配置生成工具
func main() {
	var (
		output   = flag.String("o", "edge_config.nson", "输出文件路径")
		nodeID   = flag.String("node-id", "", "节点 ID（为空则自动生成）")
		nodeName = flag.String("node-name", "edge_device", "节点名称")
		secret   = flag.String("secret", "", "节点密钥（为空则自动生成）")
		format   = flag.String("format", "binary", "输出格式：binary 或 json")
	)

	flag.Parse()

	// 生成节点 ID
	if *nodeID == "" {
		*nodeID = util.RandomID()
	}

	// 生成密钥
	if *secret == "" {
		*secret = util.RandomID()
	}

	// 创建配置
	config := createDefaultConfig(*nodeID, *nodeName, *secret)

	// 序列化
	var data []byte
	var err error

	if *format == "json" {
		// JSON 格式（人类可读）
		data, err = config.JSON()
	} else {
		// 二进制格式（性能更好）
		data, err = config.Bytes()
	}

	if err != nil {
		log.Fatalf("序列化失败: %v", err)
	}

	// 写入文件
	err = os.WriteFile(*output, data, 0644)
	if err != nil {
		log.Fatalf("写入文件失败: %v", err)
	}

	fmt.Printf("✅ 配置文件已生成: %s\n", *output)
	fmt.Printf("📝 节点信息:\n")
	fmt.Printf("   ID:     %s\n", *nodeID)
	fmt.Printf("   Name:   %s\n", *nodeName)
	fmt.Printf("   Secret: %s\n", *secret)
	fmt.Printf("   Format: %s\n", *format)
}

// createDefaultConfig 创建默认配置
func createDefaultConfig(nodeID, nodeName, secret string) nson.Map {
	now := time.Now()

	// 节点
	node := nson.Map{
		"id":      nson.String(nodeID),
		"name":    nson.String(nodeName),
		"status":  nson.I32(consts.ON),
		"updated": nson.Time(now),
	}

	// 示例 Wire - Modbus RTU
	modbusWire := nson.Map{
		"id":       nson.String(util.RandomID()),
		"node_id":  nson.String(nodeID),
		"name":     nson.String("modbus_rtu"),
		"type":     nson.String("modbus_rtu"),
		"tags":     nson.String("serial,sensors"),
		"clusters": nson.String(""),
		"updated":  nson.Time(now),
	}

	modbusWireID := modbusWire["id"].(nson.String).Value()

	// 示例 Wire - GPIO
	gpioWire := nson.Map{
		"id":       nson.String(util.RandomID()),
		"node_id":  nson.String(nodeID),
		"name":     nson.String("gpio"),
		"type":     nson.String("gpio"),
		"tags":     nson.String("digital,control"),
		"clusters": nson.String(""),
		"updated":  nson.Time(now),
	}

	gpioWireID := gpioWire["id"].(nson.String).Value()

	// 示例 Pin - 温度传感器（Modbus）
	tempPin := nson.Map{
		"id":      nson.String(util.RandomID()),
		"node_id": nson.String(nodeID),
		"wire_id": nson.String(modbusWireID),
		"name":    nson.String("temp_sensor_1"),
		"tags":    nson.String("temperature,sensor"),
		"addr":    nson.String("40001"), // Modbus 地址
		"type":    nson.String("float32"),
		"rw":      nson.I32(consts.READ),
		"updated": nson.Time(now),
	}

	// 示例 Pin - 湿度传感器（Modbus）
	humidPin := nson.Map{
		"id":      nson.String(util.RandomID()),
		"node_id": nson.String(nodeID),
		"wire_id": nson.String(modbusWireID),
		"name":    nson.String("humid_sensor_1"),
		"tags":    nson.String("humidity,sensor"),
		"addr":    nson.String("40002"),
		"type":    nson.String("float32"),
		"rw":      nson.I32(consts.READ),
		"updated": nson.Time(now),
	}

	// 示例 Pin - LED 控制（GPIO）
	ledPin := nson.Map{
		"id":      nson.String(util.RandomID()),
		"node_id": nson.String(nodeID),
		"wire_id": nson.String(gpioWireID),
		"name":    nson.String("led_1"),
		"tags":    nson.String("led,output"),
		"addr":    nson.String("GPIO17"),
		"type":    nson.String("bool"),
		"rw":      nson.I32(consts.WRITE),
		"updated": nson.Time(now),
	}

	// 示例 Pin - 按钮输入（GPIO）
	buttonPin := nson.Map{
		"id":      nson.String(util.RandomID()),
		"node_id": nson.String(nodeID),
		"wire_id": nson.String(gpioWireID),
		"name":    nson.String("button_1"),
		"tags":    nson.String("button,input"),
		"addr":    nson.String("GPIO27"),
		"type":    nson.String("bool"),
		"rw":      nson.I32(consts.READ),
		"updated": nson.Time(now),
	}

	// 组装配置
	config := nson.Map{
		"version": nson.String("1.0"),
		"node":    node,
		"wires":   nson.Array{modbusWire, gpioWire},
		"pins":    nson.Array{tempPin, humidPin, ledPin, buttonPin},
		"secret":  nson.String(secret),
	}

	return config
}
