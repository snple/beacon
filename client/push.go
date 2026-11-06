package client

import (
	"context"
	"fmt"
	"time"

	"github.com/snple/beacon/packet"
)

const defaultPushAckTimeout = 30 * time.Second

// pushPacketOverhead 是 PUSH 包除记录之外的固定开销（固定头 + RequestID + Flags + Count）。
const pushPacketOverhead = 5 + 8 + 1 + 2

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

// PushRecord 描述批量发布中的一条记录。
type PushRecord struct {
	Channel  string
	Payload  []byte
	Target   string
	Headers  map[string]string
	Priority packet.Priority
	// HasPriority 为 true 时使用 Priority，否则使用 PushOptions 的优先级。
	HasPriority bool
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
	pkt := packet.NewPushPacket(c.nextRequestID())
	if !opts.NoAck {
		pkt.Flags |= packet.PushFlagWantAck
	}
	pkt.Records = append(pkt.Records, newRecord(channel, payload, opts.effectivePriority(), opts))
	return c.sendPush(ctx, pkt, opts.NoAck)
}

// PushBatch 批量发布。
//
// 记录数超过服务端 MaxBatch 或编码后超过 MaxPacketSize 时自动拆分为多个
// PUSH 包顺序发送；任一包失败即返回，不保证跨包原子性。
func (c *Client) PushBatch(ctx context.Context, records []PushRecord, opts PushOptions) error {
	if len(records) == 0 {
		return nil
	}

	// 服务端上限未知时按协议硬上限发送，避免必然被整批拒绝。
	maxRecords := packet.MaxPushBatch
	if limit := c.serverMaxBatch(); limit > 0 && limit < maxRecords {
		maxRecords = limit
	}
	maxBytes := c.maxPushFrameBytes()

	for start := 0; start < len(records); {
		pkt := packet.NewPushPacket(c.nextRequestID())
		if !opts.NoAck {
			pkt.Flags |= packet.PushFlagWantAck
		}
		pkt.Records = make([]packet.Record, 0, min(maxRecords, len(records)-start))

		size := pushPacketOverhead
		for start < len(records) && len(pkt.Records) < maxRecords {
			r := records[start]
			priority := opts.effectivePriority()
			if r.HasPriority {
				priority = r.Priority
			}
			rec := packet.Record{
				Channel: r.Channel,
				Target:  r.Target,
				Headers: r.Headers,
				Payload: r.Payload,
			}
			rec.SetPriority(priority)
			if rec.Target == "" {
				rec.Target = opts.Target
			}

			recSize := packet.RecordSize(&rec)
			// 至少放一条：单条超限时交给服务端判定（会返回 PacketTooLarge）。
			if len(pkt.Records) > 0 && maxBytes > 0 && size+recSize > maxBytes {
				break
			}
			pkt.Records = append(pkt.Records, rec)
			size += recSize
			start++
		}

		if err := c.sendPush(ctx, pkt, opts.NoAck); err != nil {
			return err
		}
	}
	return nil
}

// PushTo 定向投递到指定客户端。
func (c *Client) PushTo(ctx context.Context, clientID, channel string, payload []byte, opts PushOptions) error {
	opts.Target = clientID
	return c.Push(ctx, channel, payload, opts)
}

// maxPushFrameBytes 返回单个 PUSH 帧允许的字节数（对端声明与本地配置取小）。
func (c *Client) maxPushFrameBytes() int {
	limit := int(c.opts.MaxPacketSize)
	if cn := c.currentConn(); cn != nil {
		if v := int(cn.maxPacketSize); v > 0 && (limit == 0 || v < limit) {
			limit = v
		}
	}
	return limit
}

// serverMaxBatch 返回服务端在 DECLARE_RESP 中公布的 MaxBatch（未知时为 0）。
func (c *Client) serverMaxBatch() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.limits == nil {
		return 0
	}
	return int(c.limits.MaxBatch)
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
