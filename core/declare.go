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
	resp.Limits = c.limits()
	pending, _, _ := cl.mb.depth()
	resp.Pending = pending

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
	index := make(map[string]int, len(p.Channels))
	// acceptedCount 统计「被接受的请求项数」，重复项按接受计，
	// 使客户端可以用 Accepted+len(Results) 校验请求项是否全部有结果。
	acceptedCount := 0

	fail := func(channel string, reason packet.ReasonCode) {
		resp.Results = append(resp.Results, packet.ChannelResult{
			Channel: channel,
			Reason:  reason,
		})
	}

	for _, d := range p.Channels {
		if pos, dup := index[d.Channel]; dup {
			// 同一过滤器重复声明等同已接受，不占用响应体积；
			// 模式取更宽松的广播可达（它覆盖只定向投递，见 docs）。
			// 带未知标志位的重复项仍视为非法声明。
			if d.Flags.Unknown() != 0 {
				fail(d.Channel, packet.ReasonChannelFilterInvalid)
				continue
			}
			if pos >= 0 {
				if !d.TargetOnly() {
					accepted[pos].SetTargetOnly(false)
				}
				acceptedCount++
			}
			continue
		}

		switch {
		case d.Flags.Unknown() != 0:
			index[d.Channel] = -1
			fail(d.Channel, packet.ReasonChannelFilterInvalid)
		case !packet.ValidateClientChannelFilter(d.Channel, opts.MaxChannelLength):
			index[d.Channel] = -1
			fail(d.Channel, packet.ReasonChannelFilterInvalid)
		case !c.canRead(identity, d.Channel):
			index[d.Channel] = -1
			fail(d.Channel, packet.ReasonNotAuthorized)
		default:
			index[d.Channel] = len(accepted)
			accepted = append(accepted, d)
			acceptedCount++
		}
	}

	cl.replaceChannels(accepted)
	resp.Accepted = uint16(acceptedCount)

	c.logger.Debug("channels declared",
		zap.String("clientID", cl.ID),
		zap.Int("accepted", acceptedCount),
		zap.Int("requested", len(p.Channels)))

	// 声明变更事件（通知型，不能否决本次声明），同时由它设置响应属性。
	declCtx := &DeclareContext{
		ClientID:   cl.ID,
		Identity:   cl.getIdentity(),
		Properties: p.Properties,
		// Requested 保留客户端原始请求（含重复项），Accepted/Rejected 是处理结果。
		Requested: append([]packet.ChannelDecl(nil), p.Channels...),
		Accepted:  append([]packet.ChannelDecl(nil), accepted...),
		Rejected:  append([]packet.ChannelResult(nil), resp.Results...),
	}
	if c.hooks.Declare != nil {
		c.hooks.callOnDeclare(declCtx)
	}

	// 响应属性先计入预算，再裁剪 Results。
	resp.Properties = cn.fitProperties("DECLARE_RESP", declCtx.ResponseProperties, declareRespOverhead)
	budget := cn.responseBudget()
	if budget > 0 && len(resp.Properties) > 0 {
		budget -= packet.StringMapSize(resp.Properties)
	}
	resp.Results = trimDeclareResults(resp.Results, budget)
	return cn.writePacket(resp)
}
