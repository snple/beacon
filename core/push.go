package core

import (
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// pushRecord 处理单条记录：校验、建消息、分发。
//
// 返回 PUSH_OK 中的一条结果。matched 是入队成功的客户端数，
// dropped 是「因入队导致旧消息被挤出」或「新消息本身被丢弃」的客户端数。
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

	if !packet.ValidateChannelName(rec.Channel) {
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

		c.takeOwnership(msg)
		enqueued, evicted := target.mb.enqueue(msg)
		c.releaseMessage(msg)
		c.notifyClient(target)

		switch {
		case !enqueued:
			res.Reason = packet.ReasonConsumerBacklog
			res.Dropped = 1
			c.stats.MessagesDropped.Add(1)
		case evicted:
			res.Reason = packet.ReasonSuccess
			res.Matched = 1
			res.Dropped = 1
			c.stats.MessagesDropped.Add(1)
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

	c.takeOwnership(msg)
	for clientID := range targets {
		c.clientsMu.RLock()
		target := c.clients[clientID]
		c.clientsMu.RUnlock()
		if target == nil {
			continue
		}

		enqueued, evicted := target.mb.enqueue(msg)
		if enqueued {
			res.Matched++
		}
		if !enqueued || evicted {
			res.Dropped++
			c.stats.MessagesDropped.Add(1)
		}
		c.notifyClient(target)
	}
	c.releaseMessage(msg)

	c.stats.MessagesReceived.Add(1)
	res.Reason = packet.ReasonSuccess
	if res.Matched == 0 {
		res.Reason = packet.ReasonConsumerBacklog
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

	if p.Flags&packet.PushFlagWantAck == 0 {
		resp.Results = nil
	}
	return cn.writePacket(resp)
}

// hasMessagePush 报告是否注册了 OnPush 钩子。
func (h *Hooks) hasMessagePush() bool {
	return h != nil && h.Message != nil
}

func nowMillis() int64 { return time.Now().UnixMilli() }
