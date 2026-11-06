package core

import (
	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// handleDeclare 处理 DECLARE：全量替换 channel 声明。
//
// 只影响之后的匹配；已入队消息不受影响（这不是退订）。
//
// DECLARE_RESP 只回显被拒绝的项（Accepted 给出接受数量），避免回显
// 大量成功项把响应撑过对端声明的 MaxPacketSize。
func (cn *conn) handleDeclare(p *packet.DeclarePacket) error {
	if err := cn.checkOwner(); err != nil {
		return err
	}

	c := cn.core
	cl := cn.client
	opts := c.options

	resp := packet.NewDeclareRespPacket(p.RequestID)
	resp.Limits = packet.Limits{
		MaxPacketSize:        opts.MaxPacketSize,
		MaxRecords:           opts.MaxRecords,
		MaxWaitMs:            opts.MaxWaitMs,
		MaxBatch:             opts.MaxBatch,
		MaxChannelsPerClient: uint16(min(opts.MaxChannelsPerClient, 65535)),
	}
	pending, delivered, _ := cl.mb.depth()
	resp.Pending = pending
	resp.Delivered = delivered

	// 数量超限整批拒绝：只返回一条紧凑错误，而不是逐项回显。
	if opts.MaxChannelsPerClient > 0 && len(p.Channels) > opts.MaxChannelsPerClient {
		resp.Results = []packet.ChannelResult{{
			Channel: "",
			Reason:  packet.ReasonQuotaExceeded,
		}}
		return cn.writePacket(resp)
	}

	identity := cl.getIdentity()
	accepted := make([]packet.ChannelDecl, 0, len(p.Channels))
	seen := make(map[string]struct{}, len(p.Channels))

	fail := func(channel string, reason packet.ReasonCode) {
		resp.Results = append(resp.Results, packet.ChannelResult{
			Channel: channel,
			Reason:  reason,
		})
	}

	for _, d := range p.Channels {
		if _, dup := seen[d.Channel]; dup {
			// 重复声明等同于已接受，不占用响应体积。
			continue
		}
		seen[d.Channel] = struct{}{}

		switch {
		case !packet.ValidateChannelFilterLimit(d.Channel, opts.MaxChannelLength):
			fail(d.Channel, packet.ReasonChannelFilterInvalid)
		case !c.canRead(identity, d.Channel):
			fail(d.Channel, packet.ReasonNotAuthorized)
		default:
			accepted = append(accepted, d)
		}
	}

	cl.replaceChannels(accepted)
	resp.Accepted = uint16(len(accepted))

	c.logger.Debug("channels declared",
		zap.String("clientID", cl.ID),
		zap.Int("accepted", len(accepted)),
		zap.Int("requested", len(p.Channels)))

	resp.Results = trimDeclareResults(resp.Results, cn.responseBudget())
	return cn.writePacket(resp)
}
