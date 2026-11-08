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
	// Headers 自定义元数据。
	Headers map[string]string
	// Target 定向投递的客户端 ID；非空时跳过 channel 匹配
	// （服务端要求目标声明过该 channel，且发布方具备定向投递权限）。
	Target string
	// NoAck 为 true 时不等待 PUSH_OK（默认等待）。
	NoAck bool

	// priority 只能通过 WithPriority 设置：零值默认为 Medium，
	// 无法与「显式指定 Low」区分，因此不导出。
	priority    packet.Priority
	prioritySet bool
}

// WithPriority 设置优先级（包括显式设置为 Low）。
func (o PushOptions) WithPriority(p packet.Priority) PushOptions {
	o.priority = p
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

// effectivePriority 返回本次发布实际使用的优先级：默认 Medium。
func (o PushOptions) effectivePriority() packet.Priority {
	if o.prioritySet {
		return o.priority
	}
	return packet.PriorityMedium
}

func newRecord(channel string, payload []byte, priority packet.Priority, opts PushOptions) packet.Record {
	rec := packet.Record{Channel: channel, Payload: payload}
	rec.Target = opts.Target
	rec.Headers = opts.Headers
	rec.SetPriority(priority)
	return rec
}

// Push 发布一条消息到指定 channel。
func (c *Client) Push(ctx context.Context, channel string, payload []byte, opts PushOptions) error {
	pkt := packet.NewPushPacket(c.nextRequestID(),
		newRecord(channel, payload, opts.effectivePriority(), opts))
	if !opts.NoAck {
		pkt.Flags |= packet.PushFlagWantAck
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
	if ok.Reason.IsError() {
		return reasonToError(ok.Reason)
	}
	return nil
}
