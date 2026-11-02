package client

import (
	"context"
	"fmt"
	"time"

	"github.com/snple/beacon/packet"
)

const defaultPushAckTimeout = 30 * time.Second

// PushOptions 发布选项。
type PushOptions struct {
	// Priority 消息优先级（默认 Medium）。Medium/High 需要消费端 COMMIT。
	Priority packet.Priority
	// Headers 自定义元数据。
	Headers map[string]string
	// Target 定向投递的客户端 ID；非空时跳过 channel 匹配。
	Target string
	// NoAck 为 true 时不等待 PUSH_OK（默认等待）。
	NoAck bool

	prioritySet bool
}

// WithPriority 设置优先级（允许显式设置为 Low）。
func (o PushOptions) WithPriority(p packet.Priority) PushOptions {
	o.Priority = p
	o.prioritySet = true
	return o
}

// WithHeaders 设置元数据。
func (o PushOptions) WithHeaders(h map[string]string) PushOptions {
	o.Headers = h
	return o
}

// WithTarget 设置定向投递目标。
func (o PushOptions) WithTarget(clientID string) PushOptions {
	o.Target = clientID
	return o
}

// WithoutAck 不等待服务端确认。
func (o PushOptions) WithoutAck() PushOptions {
	o.NoAck = true
	return o
}

// PushRecord 描述批量发布中的一条记录。
type PushRecord struct {
	Channel     string
	Payload     []byte
	Target      string
	Headers     map[string]string
	Priority    packet.Priority
	HasPriority bool
}

func newRecord(channel string, payload []byte, opts PushOptions) packet.Record {
	rec := packet.Record{Channel: channel, Payload: payload}
	rec.Target = opts.Target
	rec.Headers = opts.Headers
	switch {
	case opts.prioritySet:
		rec.SetPriority(opts.Priority)
	case opts.Priority != packet.PriorityLow:
		rec.SetPriority(opts.Priority)
	}
	return rec
}

// Push 发布一条消息到指定 channel。
func (c *Client) Push(ctx context.Context, channel string, payload []byte, opts PushOptions) error {
	pkt := packet.NewPushPacket(c.nextRequestID())
	if !opts.NoAck {
		pkt.Flags |= packet.PushFlagWantAck
	}
	pkt.Records = append(pkt.Records, newRecord(channel, payload, opts))
	return c.sendPush(ctx, pkt, opts.NoAck)
}

// PushBatch 批量发布。
func (c *Client) PushBatch(ctx context.Context, records []PushRecord, opts PushOptions) error {
	if len(records) == 0 {
		return nil
	}

	pkt := packet.NewPushPacket(c.nextRequestID())
	if !opts.NoAck {
		pkt.Flags |= packet.PushFlagWantAck
	}
	for _, r := range records {
		rec := packet.Record{
			Channel: r.Channel,
			Target:  r.Target,
			Headers: r.Headers,
			Payload: r.Payload,
		}
		switch {
		case r.HasPriority:
			rec.SetPriority(r.Priority)
		case opts.prioritySet:
			rec.SetPriority(opts.Priority)
		case opts.Priority != packet.PriorityLow:
			rec.SetPriority(opts.Priority)
		}
		if rec.Target == "" {
			rec.Target = opts.Target
		}
		pkt.Records = append(pkt.Records, rec)
	}
	return c.sendPush(ctx, pkt, opts.NoAck)
}

// PushTo 定向投递到指定客户端。
func (c *Client) PushTo(ctx context.Context, clientID, channel string, payload []byte, opts PushOptions) error {
	opts.Target = clientID
	return c.Push(ctx, channel, payload, opts)
}

func (c *Client) sendPush(ctx context.Context, pkt *packet.PushPacket, noAck bool) error {
	if noAck {
		cn := c.currentConn()
		if cn == nil || cn.closed.Load() {
			return ErrNotConnected
		}
		return cn.write(pkt)
	}

	resp, err := c.roundTrip(ctx, pkt, pkt.RequestID, defaultPushAckTimeout)
	if err != nil {
		return err
	}

	ok, valid := resp.(*packet.PushOkPacket)
	if !valid {
		return fmt.Errorf("unexpected response %s", resp.Type())
	}
	for _, r := range ok.Results {
		if r.Reason.IsError() {
			return reasonToError(r.Reason)
		}
	}
	return nil
}
