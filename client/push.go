package client

import (
	"context"
	"fmt"
	"maps"
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
	// Properties 是随 PUSH 发送的可选属性。
	Properties map[string]string

	// priority 只能通过 WithPriority 设置：零值默认为 Medium，
	// 无法与「显式指定 Low」区分，因此不导出。
	priority    packet.Priority
	prioritySet bool

	// reliable 只能通过 WithReliable 设置；默认为 false（不可靠）。
	reliable bool
	// ttl > 0 时设置绝对过期时间；publishedAt/expiresAt 为显式值。
	ttl         time.Duration
	expiresAt   int64
	publishedAt int64
}

// WithPriority 设置优先级（包括显式设置为 Low）。
func (o PushOptions) WithPriority(p packet.Priority) PushOptions {
	o.priority = p
	o.prioritySet = true
	return o
}

// WithReliable 设置是否需要 COMMIT，与优先级相互独立。
//
// 默认为 false（尽力而为、不重发、离线丢弃）；需要投递保证时显式传 true。
func (o PushOptions) WithReliable(reliable bool) PushOptions {
	o.reliable = reliable
	return o
}

// WithTTL 设置消息存活时长；发布时自动换算为绝对过期时间，
// 并记录发布者时间戳。
func (o PushOptions) WithTTL(ttl time.Duration) PushOptions {
	o.ttl = ttl
	return o
}

// WithExpiresAt 设置绝对过期时间（Unix 毫秒）；<= 0 表示永不过期。
func (o PushOptions) WithExpiresAt(ms int64) PushOptions {
	o.expiresAt = ms
	return o
}

// WithPublishedAt 设置发布者时间戳（Unix 毫秒）。
func (o PushOptions) WithPublishedAt(ms int64) PushOptions {
	o.publishedAt = ms
	return o
}

// WithProperties 设置随 PUSH 发送的可选属性。
func (o PushOptions) WithProperties(props map[string]string) PushOptions {
	o.Properties = maps.Clone(props)
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
	rec.SetReliable(opts.reliable)

	now := time.Now().UnixMilli()
	expiresAt := opts.expiresAt
	if expiresAt <= 0 && opts.ttl > 0 {
		expiresAt = now + opts.ttl.Milliseconds()
	}
	if expiresAt > 0 {
		rec.SetExpiresAt(expiresAt)
	}
	publishedAt := opts.publishedAt
	if publishedAt <= 0 && expiresAt > 0 {
		publishedAt = now
	}
	if publishedAt > 0 {
		rec.SetPublishedAt(publishedAt)
	}
	return rec
}

// PushResult 是 PUSH_OK 的结果摘要。
//
// NoAck 发布没有服务端响应，返回零值。
type PushResult struct {
	ID      packet.PacketID
	Reason  packet.ReasonCode
	Matched uint32 // 入队成功的客户端数
	Dropped uint32 // 未入队的客户端数（离线不可靠消息、队列溢出、目标接收上限过小等）
	// Properties 是 PUSH_OK 返回的可选属性（例如 error 细节）。
	Properties map[string]string
}

// Push 发布一条消息到指定 channel。
func (c *Client) Push(ctx context.Context, channel string, payload []byte, opts PushOptions) error {
	_, err := c.PushWithResult(ctx, channel, payload, opts)
	return err
}

// PushWithResult 与 Push 相同，但额外返回服务端的匹配/丢弃统计。
func (c *Client) PushWithResult(ctx context.Context, channel string, payload []byte, opts PushOptions) (PushResult, error) {
	return c.push(ctx, channel, payload, opts, opts.Target != "")
}

// PushTo 定向投递到指定客户端。
//
// 目标存在但没有声明匹配该 channel 的过滤器时返回 ErrTargetNotSubscribed；
// 目标不存在时返回 Reason=TargetNotFound 的 ServerError。
func (c *Client) PushTo(ctx context.Context, clientID, channel string, payload []byte, opts PushOptions) error {
	_, err := c.PushToWithResult(ctx, clientID, channel, payload, opts)
	return err
}

// PushToWithResult 与 PushTo 相同，但额外返回服务端的匹配/丢弃统计。
func (c *Client) PushToWithResult(ctx context.Context, clientID, channel string, payload []byte, opts PushOptions) (PushResult, error) {
	opts.Target = clientID
	return c.push(ctx, channel, payload, opts, true)
}

func (c *Client) push(ctx context.Context, channel string, payload []byte, opts PushOptions, targeted bool) (PushResult, error) {
	pkt := packet.NewPushPacket(c.nextRequestID(),
		newRecord(channel, payload, opts.effectivePriority(), opts))
	pkt.Properties = maps.Clone(opts.Properties)
	if !opts.NoAck {
		pkt.Flags |= packet.PushFlagWantAck
	}
	return c.sendPush(ctx, pkt, opts.NoAck, targeted)
}

func (c *Client) sendPush(ctx context.Context, pkt *packet.PushPacket, noAck, targeted bool) (PushResult, error) {
	if noAck {
		cn := c.currentConn()
		if cn == nil || cn.closed.Load() {
			return PushResult{}, ErrNotConnected
		}
		return PushResult{}, cn.write(pkt)
	}

	resp, err := c.roundTrip(ctx, pkt, pkt.RequestID, defaultPushAckTimeout)
	if err != nil {
		return PushResult{}, err
	}

	ok, valid := resp.(*packet.PushOkPacket)
	if !valid {
		return PushResult{}, fmt.Errorf("unexpected response %s", resp.Type())
	}

	result := PushResult{
		ID:         ok.ID,
		Reason:     ok.Reason,
		Matched:    ok.Matched,
		Dropped:    ok.Dropped,
		Properties: maps.Clone(ok.Properties),
	}
	if ok.Reason.IsError() {
		return result, reasonToErrorWithDetail(ok.Reason, ok.Properties[packet.PropertyError])
	}
	// NoMatch 是路由类原因码（非错误），但对定向投递意味着消息已被丢弃。
	if targeted && ok.Reason == packet.ReasonNoMatch {
		return result, ErrTargetNotSubscribed
	}
	return result, nil
}
