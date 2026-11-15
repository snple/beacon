package client

import "github.com/snple/beacon/packet"

// Priority 是消息优先级。
//
// 零值 PriorityDefault 表示「未指定」：发布时按 Medium 处理；接收到的消息
// 只会是 Low/Medium/High。用客户端自有类型而不是 packet.Priority，是因为
// 协议枚举的零值是 Low，无法表达「未指定」。
type Priority uint8

const (
	// PriorityDefault 表示未指定，发布时按 Medium 处理。
	PriorityDefault Priority = iota
	// PriorityLow 是最低优先级：队列满时最先被淘汰。
	PriorityLow
	// PriorityMedium 是默认优先级。
	PriorityMedium
	// PriorityHigh 是最高优先级：队列满时最后被淘汰。
	PriorityHigh
)

// String 返回优先级名称。
func (p Priority) String() string {
	switch p {
	case PriorityDefault:
		return "default"
	case PriorityLow:
		return "low"
	case PriorityMedium:
		return "medium"
	case PriorityHigh:
		return "high"
	default:
		return "unknown"
	}
}

// valid 报告优先级是否可编码到线上。
func (p Priority) valid() bool { return p <= PriorityHigh }

// packet 返回线上编码。
func (p Priority) packet() packet.Priority {
	switch p {
	case PriorityLow:
		return packet.PriorityLow
	case PriorityHigh:
		return packet.PriorityHigh
	default:
		return packet.PriorityMedium
	}
}

// priorityFromPacket 把线上编码转换为客户端类型。
func priorityFromPacket(p packet.Priority) Priority {
	switch p {
	case packet.PriorityLow:
		return PriorityLow
	case packet.PriorityHigh:
		return PriorityHigh
	default:
		return PriorityMedium
	}
}
