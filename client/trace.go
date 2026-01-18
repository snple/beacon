package client

import (
	"time"

	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// handleTrace 处理服务器发来的 TRACE 包
func (c *Client) handleTrace(p *packet.TracePacket) {
	c.logger.Debug("Received TRACE packet",
		zap.String("traceID", p.TraceID),
		zap.String("event", p.Event),
		zap.String("clientID", p.ClientID))

	// 调用 OnTrace 钩子（直接传递原始 packet）
	c.options.Hooks.callOnTrace(&TraceContext{
		ClientID: c.ClientID(),
		Packet:   p,
	})
}

// SendTrace 发送 TRACE 包（用于分布式追踪）
func (c *Client) SendTrace(traceID, event string, details map[string]string) error {
	if !c.connected.Load() {
		return ErrNotConnected
	}

	trace := packet.NewTracePacket(traceID, event, c.ClientID())
	trace.Timestamp = uint64(time.Now().UnixMilli())
	trace.Details = details

	if err := c.writePacket(trace); err != nil {
		c.logger.Error("Failed to send TRACE packet", zap.Error(err))
		return err
	}

	c.logger.Debug("Sent TRACE packet",
		zap.String("traceID", traceID),
		zap.String("event", event))
	return nil
}
