package core

import (
	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// handleTrace 处理 TRACE 包（用于分布式追踪）
func (c *Client) handleTrace(p *packet.TracePacket) error {
	c.core.logger.Debug("Received TRACE packet",
		zap.String("clientID", c.ID),
		zap.String("traceID", p.TraceID),
		zap.String("event", p.Event))

	// 构建追踪上下文（直接引用原始 packet）
	ctx := &TraceContext{
		ClientID: c.ID,
		Packet:   p,
	}

	// 调用追踪钩子
	c.core.options.Hooks.callOnTrace(ctx)

	return nil
}
