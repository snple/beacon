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
func (c *Core) pushRecord(cl *Client, rec *packet.Record, properties map[string]string, system bool) (res packet.PushResult, detail string, respProps map[string]string) {
	res = packet.PushResult{ID: rec.ID}

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
		return res, "", nil
	}

	validChannel := packet.ValidateChannelNameLimit(rec.Channel, c.options.MaxChannelLength)
	if validChannel && !system && packet.IsReservedChannel(rec.Channel) {
		// "$" 前缀保留给 broker 内部 channel，客户端不可发布。
		validChannel = false
	}
	if !validChannel {
		res.Reason = packet.ReasonChannelInvalid
		c.stats.MessagesRejected.Add(1)
		if cl != nil {
			cl.rejected.Add(1)
		}
		return res, "", nil
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
		return res, "", nil
	}

	// 定向投递需要单独的写权限：否则任何能写某 channel 的发布者都能
	// 绕过目标的订阅与读权限直接投递。
	if rec.Target != "" && !system && !c.canTarget(identity) {
		res.Reason = packet.ReasonNotAuthorized
		c.stats.MessagesRejected.Add(1)
		if cl != nil {
			cl.rejected.Add(1)
		}
		return res, "", nil
	}

	// 单条记录上限覆盖元数据（channel/target/source/headers），
	// 只检查 payload 会被超大 headers 绕过。
	if c.recordTooLarge(rec) {
		res.Reason = packet.ReasonPacketTooLarge
		c.stats.MessagesRejected.Add(1)
		if cl != nil {
			cl.rejected.Add(1)
		}
		return res, "", nil
	}

	// 发布钩子
	if c.hooks.Push != nil {
		ctx := &PushContext{
			ID:          rec.ID,
			ClientID:    rec.Source,
			FromServer:  system,
			Identity:    identity,
			Channel:     rec.Channel,
			Target:      rec.Target,
			Priority:    rec.Priority(),
			Headers:     rec.Headers,
			Payload:     rec.Payload,
			ExpiresAt:   rec.ExpiresAt,
			Reliable:    rec.Reliable(),
			PublishedAt: rec.PublishedAt,
			Properties:  properties,
		}
		if err := c.hooks.callOnPush(ctx); err != nil {
			reason, detail := hookRejection(err)
			res.Reason = reason
			c.stats.MessagesRejected.Add(1)
			if cl != nil {
				cl.rejected.Add(1)
			}
			c.logger.Debug("publish rejected by hook",
				zap.String("clientID", rec.Source),
				zap.String("channel", rec.Channel),
				zap.String("reason", reason.String()),
				zap.Error(err))
			return res, detail, nil
		}
		respProps = ctx.ResponseProperties
		// 应用钩子改写：Channel/Target/Source/ID/Reliable 不可改，
		// 因此这里只回写内容字段，并重新校验大小上限。
		rec.Headers = ctx.Headers
		rec.Payload = ctx.Payload
		rec.SetPriority(ctx.Priority)
		rec.SetExpiresAt(ctx.ExpiresAt)
		if c.recordTooLarge(rec) {
			res.Reason = packet.ReasonPacketTooLarge
			c.stats.MessagesRejected.Add(1)
			if cl != nil {
				cl.rejected.Add(1)
			}
			c.logger.Debug("publish rejected: hook output exceeds max record bytes",
				zap.String("clientID", rec.Source),
				zap.String("channel", rec.Channel))
			return res, "", respProps
		}
	}

	msg := newMessage(rec)
	msg.ReceivedAt = nowMillis()

	// 发布时就已经过期：不再入队，直接告知发布者。
	if msg.Expired(msg.ReceivedAt) {
		res.Reason = packet.ReasonExpired
		c.stats.MessagesExpired.Add(1)
		if cl != nil {
			cl.expired.Add(1)
		}
		c.logger.Debug("dropped expired message",
			zap.String("channel", rec.Channel),
			zap.Int64("expiresAt", msg.ExpiresAt))
		return res, "", respProps
	}

	// 定向投递：完全跳过匹配
	if rec.Target != "" {
		c.clientsMu.RLock()
		target := c.clients[rec.Target]
		c.clientsMu.RUnlock()

		if target == nil {
			c.stats.TargetMisses.Add(1)
			c.stats.MessagesNoMatch.Add(1)
			res.Reason = packet.ReasonTargetNotFound
			return res, "", respProps
		}
		// 目标必须声明过能匹配该 channel 的过滤器，定向投递不是订阅后门。
		if !target.matchesChannel(rec.Channel) {
			c.stats.MessagesNoMatch.Add(1)
			res.Reason = packet.ReasonNoMatch
			return res, "", respProps
		}
		// 目标当前连接声明的接收上限放不下这条记录：不入队，直接告知发布者，
		// 否则它会永久堵住目标队列（PULL 时无法跳过）。
		// 离线会话保留旧上限，可能在重连后提高，因此仍先入队，由 PULL 侧兜底。
		if target.currentConn() != nil && !fitsRecordInPacket(target.receiveLimit(), rec) {
			res.Reason = packet.ReasonPacketTooLarge
			res.Dropped = 1
			c.stats.MessagesDropped.Add(1)
			target.dropped.Add(1)
			return res, "", respProps
		}
		if c.overInflightLimit(msg.Size()) {
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
		return res, "", respProps
	}

	targets := c.matcher.match(rec.Channel)
	if len(targets) == 0 {
		c.stats.MessagesNoMatch.Add(1)
		res.Reason = packet.ReasonNoMatch
		return res, "", respProps
	}
	if c.overInflightLimit(msg.Size()) {
		return c.rejectOverInflight(cl, res)
	}

	c.takeOwnership(msg)
	backlog := false
	tooSmall := false
	for clientID := range targets {
		c.clientsMu.RLock()
		target := c.clients[clientID]
		c.clientsMu.RUnlock()
		if target == nil {
			continue
		}

		// 目标当前接收上限放不下这条记录：不为该客户端入队，计入丢弃。
		// 离线会话仍先入队（重连后可能提高上限），由 PULL 侧兜底丢弃。
		if target.currentConn() != nil && !fitsRecordInPacket(target.receiveLimit(), rec) {
			tooSmall = true
			res.Dropped++
			c.stats.MessagesDropped.Add(1)
			target.dropped.Add(1)
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
	if res.Matched == 0 && tooSmall {
		res.Reason = packet.ReasonPacketTooLarge
	} else if res.Matched == 0 && backlog {
		res.Reason = packet.ReasonConsumerBacklog
	}
	return res, "", respProps
}

// overInflightLimit 报告加入 size 字节后是否会超过全局在途消息上限。
func (c *Core) overInflightLimit(size int64) bool {
	limit := c.options.MaxInflightBytes
	return limit > 0 && c.inflightBytes.Load()+size > limit
}

// recordTooLarge 报告记录是否超过单条记录上限。
func (c *Core) recordTooLarge(rec *packet.Record) bool {
	limit := c.options.MaxRecordBytes
	return limit > 0 && int64(packet.RecordSize(rec)) > int64(limit)
}

func (c *Core) rejectOverInflight(cl *Client, res packet.PushResult) (packet.PushResult, string, map[string]string) {
	res.Reason = packet.ReasonQuotaExceeded
	c.stats.MessagesRejected.Add(1)
	if cl != nil {
		cl.rejected.Add(1)
	}
	return res, "", nil
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

	// 发布速率限制
	if cl.pushLimiter != nil && !cl.pushLimiter.Allow() {
		return cn.rejectPush(p, packet.ReasonRateLimited)
	}

	res, detail, respProps := cn.core.pushRecord(cl, &p.Record, p.Properties, false)

	// WantAck=0：客户端不等待结果，不回包以节省一个 RTT 与带宽。
	if p.Flags&packet.PushFlagWantAck == 0 {
		return nil
	}
	resp := packet.NewPushOkPacket(p.RequestID)
	resp.PushResult = res
	resp.Properties = mergeProperties(pushErrorProperties(res.Reason, detail), respProps)
	cn.fitPushOkProperties(resp)
	return cn.writePacket(resp)
}

// rejectPush 以指定原因拒绝记录，并填充服务端生成的 ID。
func (cn *conn) rejectPush(p *packet.PushPacket, reason packet.ReasonCode) error {
	cn.core.stats.MessagesRejected.Add(1)
	if p.Flags&packet.PushFlagWantAck == 0 {
		return nil
	}

	resp := packet.NewPushOkPacket(p.RequestID)
	resp.Reason = reason
	resp.ID = packet.NewPacketID()
	resp.Properties = pushErrorProperties(reason, "")
	cn.fitPushOkProperties(resp)
	return cn.writePacket(resp)
}

// fitPushOkProperties 在客户端接收上限过小时省略可选错误细节，
// 避免 PUSH_OK 本身超限导致连接被关闭。
func (cn *conn) fitPushOkProperties(resp *packet.PushOkPacket) {
	if len(resp.Properties) == 0 || cn.maxPacketSize == 0 {
		return
	}
	if resp.Size() > int(cn.maxPacketSize) {
		resp.Properties = nil
	}
}

// pushErrorProperties 返回错误原因的可读细节；成功时为空。
// detail 只来自钩子显式设置的 HookError.Detail，避免泄露内部错误。
func pushErrorProperties(reason packet.ReasonCode, detail string) map[string]string {
	if !reason.IsError() {
		return nil
	}
	if detail == "" {
		detail = reason.String()
	}
	return map[string]string{packet.PropertyError: detail}
}

func nowMillis() int64 { return time.Now().UnixMilli() }
