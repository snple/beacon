package core

import (
	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// handleDeclare 处理 DECLARE：全量替换 channel 声明。
//
// 只影响之后的匹配；已入队消息不受影响（这不是退订）。
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
		MaxBatch:             opts.MaxBytes,
		MaxChannelsPerClient: uint16(min(opts.MaxChannelsPerClient, 65535)),
	}

	if opts.MaxChannelsPerClient > 0 && len(p.Channels) > opts.MaxChannelsPerClient {
		for _, d := range p.Channels {
			resp.Results = append(resp.Results, packet.ChannelResult{
				Channel: d.Channel,
				Reason:  packet.ReasonQuotaExceeded,
			})
		}
		pending, delivered, _ := cl.mb.depth()
		resp.Pending = pending
		resp.Delivered = delivered
		return cn.writePacket(resp)
	}

	identity := cl.getIdentity()
	accepted := make([]packet.ChannelDecl, 0, len(p.Channels))
	seen := make(map[string]struct{}, len(p.Channels))

	for _, d := range p.Channels {
		if _, dup := seen[d.Channel]; dup {
			resp.Results = append(resp.Results, packet.ChannelResult{
				Channel: d.Channel,
				Reason:  packet.ReasonSuccess,
			})
			continue
		}
		seen[d.Channel] = struct{}{}

		switch {
		case !packet.ValidateChannelFilter(d.Channel):
			resp.Results = append(resp.Results, packet.ChannelResult{
				Channel: d.Channel,
				Reason:  packet.ReasonChannelFilterInvalid,
			})
		case !c.canRead(identity, d.Channel):
			resp.Results = append(resp.Results, packet.ChannelResult{
				Channel: d.Channel,
				Reason:  packet.ReasonNotAuthorized,
			})
		default:
			resp.Results = append(resp.Results, packet.ChannelResult{
				Channel: d.Channel,
				Reason:  packet.ReasonSuccess,
			})
			accepted = append(accepted, d)
		}
	}

	cl.replaceChannels(accepted)

	pending, delivered, _ := cl.mb.depth()
	resp.Pending = pending
	resp.Delivered = delivered

	c.logger.Debug("channels declared",
		zap.String("clientID", cl.ID),
		zap.Int("accepted", len(accepted)),
		zap.Int("requested", len(p.Channels)))

	return cn.writePacket(resp)
}
