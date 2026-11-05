package core

import (
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// pushRecord 处理单条记录：校验、建消息、分发。
//
// 返回 PUSH_OK 中的一条结果。matched 是入队成功的客户端数，
// dropped 是「因入队导致旧消息被挤出」「新消息本身被丢弃」或
// 「目标离线且消息为 Low 而被丢弃」的客户端数。
func (c *Core) pushRecord(cl *Client, rec *packet.Record, system bool) packet.PushResult {
	res := packet.PushResult{ID: rec.ID}

	// 服务端生成 ID（客户端不可指定），并填充可信字段
	newID := packet.NewPacketID()
	rec.ID = newID
	res.ID = newID
	if cl != nil {
		// 客户端不可伪造 Source
		rec.Source = cl.ID
	}

	if !rec.Priority().IsValid() {
		res.Reason = packet.ReasonMalformedPacket
		c.stats.MessagesRejected.Add(1)
		if cl != nil {
			cl.rejected.Add(1)
		}
		return res
	}

	if !packet.ValidateChannelNameLimit(rec.Channel, c.options.MaxChannelLength) {
		res.Reason = packet.ReasonChannelInvalid
		c.stats.MessagesRejected.Add(1)
		if cl != nil {
			cl.rejected.Add(1)
		}
		return res
	}

	var identity *Identity
	if cl != nil {
		identity = cl.getIdentity()
	}
	if !system && !c.canWrite(identity, rec.Channel) {
		res.Reason = packet.ReasonNotAuthorized
		c.stats.MessagesRejected.Add(1)
		if cl != nil {
			cl.rejected.Add(1)
		}
		return res
	}

	if size := int64(len(rec.Payload)); c.options.MaxRecordBytes > 0 && size > int64(c.options.MaxRecordBytes) {
		res.Reason = packet.ReasonPacketTooLarge
		c.stats.MessagesRejected.Add(1)
		if cl != nil {
			cl.rejected.Add(1)
		}
		return res
	}

	// 发布钩子
	if c.options.Hooks.hasMessagePush() {
		ctx := &PushContext{
			ClientID: rec.Source,
			Identity: identity,
			Channel:  rec.Channel,
			Target:   rec.Target,
			Priority: rec.Priority(),
			Headers:  rec.Headers,
			Payload:  rec.Payload,
		}
		if err := c.options.Hooks.callOnPush(ctx); err != nil {
			res.Reason = packet.ReasonNotAuthorized
			c.stats.MessagesRejected.Add(1)
			if cl != nil {
				cl.rejected.Add(1)
			}
			c.logger.Debug("publish rejected by hook",
				zap.String("channel", rec.Channel), zap.Error(err))
			return res
		}
	}

	msg := newMessage(rec)
	msg.ReceivedAt = nowMillis()

	// 定向投递：完全跳过匹配
	if rec.Target != "" {
		c.clientsMu.RLock()
		target := c.clients[rec.Target]
		c.clientsMu.RUnlock()

		if target == nil {
			c.stats.TargetMisses.Add(1)
			c.stats.MessagesNoMatch.Add(1)
			res.Reason = packet.ReasonTargetNotFound
			return res
		}
		if c.overInflightLimit() {
			return c.rejectOverInflight(cl, res)
		}

		c.takeOwnership(msg)
		enqueued, evicted, skipped := target.enqueue(msg)
		c.releaseMessage(msg)
		c.notifyClient(target)

		switch {
		case skipped:
			// 目标离线且消息为 Low：不占用离线队列，按尽力而为处理。
			res.Reason = packet.ReasonSuccess
			res.Dropped = 1
			c.stats.MessagesDropped.Add(1)
			target.dropped.Add(1)
		case !enqueued:
			res.Reason = packet.ReasonConsumerBacklog
			res.Dropped = 1
			c.stats.MessagesDropped.Add(1)
			target.dropped.Add(1)
		case evicted:
			res.Reason = packet.ReasonSuccess
			res.Matched = 1
			res.Dropped = 1
			c.stats.MessagesDropped.Add(1)
			target.dropped.Add(1)
		default:
			res.Reason = packet.ReasonSuccess
			res.Matched = 1
		}

		c.stats.MessagesReceived.Add(1)
		return res
	}

	targets := c.matcher.match(rec.Channel)
	if len(targets) == 0 {
		c.stats.MessagesNoMatch.Add(1)
		res.Reason = packet.ReasonNoMatch
		return res
	}
	if c.overInflightLimit() {
		return c.rejectOverInflight(cl, res)
	}

	c.takeOwnership(msg)
	backlog := false
	for clientID := range targets {
		c.clientsMu.RLock()
		target := c.clients[clientID]
		c.clientsMu.RUnlock()
		if target == nil {
			continue
		}

		enqueued, evicted, skipped := target.enqueue(msg)
		if enqueued {
			res.Matched++
		}
		if !enqueued && !skipped {
			backlog = true
		}
		if skipped || !enqueued || evicted {
			res.Dropped++
			c.stats.MessagesDropped.Add(1)
			target.dropped.Add(1)
		}
		c.notifyClient(target)
	}
	c.releaseMessage(msg)

	c.stats.MessagesReceived.Add(1)
	res.Reason = packet.ReasonSuccess
	if res.Matched == 0 && backlog {
		res.Reason = packet.ReasonConsumerBacklog
	}
	return res
}

// overInflightLimit 报告全局在途消息是否已达上限。
func (c *Core) overInflightLimit() bool {
	return c.options.MaxInflightBytes > 0 && c.inflightBytes.Load() >= c.options.MaxInflightBytes
}

func (c *Core) rejectOverInflight(cl *Client, res packet.PushResult) packet.PushResult {
	res.Reason = packet.ReasonQuotaExceeded
	c.stats.MessagesRejected.Add(1)
	if cl != nil {
		cl.rejected.Add(1)
	}
	return res
}

func (c *Core) notifyClient(cl *Client) {
	cl.notify()
}

// handlePush 处理 PUSH 请求。
func (cn *conn) handlePush(p *packet.PushPacket) error {
	if err := cn.checkOwner(); err != nil {
		return err
	}

	cl := cn.client

	if len(p.Records) > 0 && cn.core.options.MaxBytes > 0 {
		var total uint32
		for i := range p.Records {
			total += uint32(packet.RecordSize(&p.Records[i]))
		}
		if total > cn.core.options.MaxBytes {
			resp := packet.NewPushOkPacket(p.RequestID)
			for range p.Records {
				resp.Results = append(resp.Results, packet.PushResult{
					Reason: packet.ReasonQuotaExceeded,
				})
			}
			cn.core.stats.MessagesRejected.Add(int64(len(p.Records)))
			return cn.writePacket(resp)
		}
	}

	// 发布速率限制
	if cl.pushLimiter != nil && !cl.pushLimiter.Allow() {
		resp := packet.NewPushOkPacket(p.RequestID)
		for range p.Records {
			resp.Results = append(resp.Results, packet.PushResult{
				Reason: packet.ReasonRateLimited,
			})
		}
		cn.core.stats.MessagesRejected.Add(int64(len(p.Records)))
		return cn.writePacket(resp)
	}

	resp := packet.NewPushOkPacket(p.RequestID)
	resp.Results = make([]packet.PushResult, 0, len(p.Records))
	for i := range p.Records {
		resp.Results = append(resp.Results, cn.core.pushRecord(cl, &p.Records[i], false))
	}

	// WantAck=0：客户端不等待结果，不回包以节省一个 RTT 与带宽。
	if p.Flags&packet.PushFlagWantAck == 0 {
		return nil
	}
	return cn.writePacket(resp)
}

// hasMessagePush 报告是否注册了 OnPush 钩子。
func (h *Hooks) hasMessagePush() bool {
	return h != nil && h.Message != nil
}

func nowMillis() int64 { return time.Now().UnixMilli() }
