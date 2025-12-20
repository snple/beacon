package dt

import "github.com/danclive/nson-go"

// Queen 协议通讯约定：
// 1. REQUEST/RESPONSE：需要立即反馈的操作
//   - beacon.pin.write.sync: Edge 连接时全量同步待写入数据
//
// 2. PUBLISH/SUBSCRIBE：数据同步和命令下发
//
// Edge → Core（数据同步，使用 Publish）:
//   - beacon/push       : 配置数据推送（NSON 格式）
//   - beacon/pin/values : Pin 值批量推送（NSON Array）
//   - beacon/pin/value  : Pin 值推送（NSON Map）
//
// Core → Edge（命令下发，使用 Publish）:
//   - beacon/pin/writes : Pin 批量写入命令（NSON Array）
//   - beacon/pin/write  : Pin 写入命令（NSON Map）
const (
	TopicPush          = "beacon/push"
	TopicPinValue      = "beacon/pin/value"
	TopicPinValueBatch = "beacon/pin/values"
	TopicPinWrite      = "beacon/pin/write"
	TopicPinWriteBatch = "beacon/pin/writes"

	// Action 名称（用于 Request/Response）
	ActionPinWriteSync = "beacon.pin.write.sync"
)

// PinValueMessage Pin 值消息
type PinValueMessage struct {
	ID    string     `nson:"id,omitempty"`    // Pin ID
	Value nson.Value `nson:"value,omitempty"` // NSON 值
}
