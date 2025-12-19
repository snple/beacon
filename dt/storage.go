package dt

import "time"

// ============================================================================
// Storage 数据模型 - 用于 Core 和 Edge 的存储层
// ============================================================================

// Node 节点配置
type Node struct {
	ID      string    `nson:"id"`
	Name    string    `nson:"name"`
	Tags    []string  `nson:"tags,omitempty"`    // 节点标签列表
	Device  string    `nson:"device,omitempty"`  // 设备模板 ID（指向 device.Device.ID）
	Updated time.Time `nson:"updated,omitempty"` // Core 端使用
	Wires   []Wire    `nson:"wires"`
}

// Wire 通道配置
type Wire struct {
	ID   string   `nson:"id"`
	Name string   `nson:"name"`
	Tags []string `nson:"tags,omitempty"`
	Type string   `nson:"type"`
	Pins []Pin    `nson:"pins"`
}

// Pin 点位配置
type Pin struct {
	ID   string   `nson:"id"`
	Name string   `nson:"name"`
	Tags []string `nson:"tags,omitempty"`
	Addr string   `nson:"addr"`
	Type uint32   `nson:"type"` // nson.DataType
	Rw   int32    `nson:"rw"`
}
