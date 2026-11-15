package client

import (
	"context"
	"fmt"
	"maps"
	"time"

	"github.com/snple/beacon/packet"
)

const defaultPushAckTimeout = 30 * time.Second

// PushOptions 是发布选项；零值表示使用默认行为。
type PushOptions struct {
	// Priority 是消息优先级；零值 PriorityDefault 按 Medium 处理。
	Priority Priority
	// Reliable 表示消息是否需要 COMMIT 确认（默认 false，尽力而为）。
	Reliable bool
	// NoAck 为 true 时不等待 PUSH_OK，仅尽力写入连接。
	NoAck bool
	// Headers 是随消息投递的消息头。
	Headers map[string]string
	// Properties 是随 PUSH 发送给服务端的属性，不会投递给消费者。
	Properties map[string]string
	// TTL 是相对存活时长；ExpiresAt 未设置时换算为绝对过期时间。
	TTL time.Duration
	// ExpiresAt 是绝对过期时间（Unix 毫秒）；<= 0 表示未设置。
	ExpiresAt int64
	// PublishedAt 是发布者时间戳（Unix 毫秒）；<= 0 表示未设置。
	PublishedAt int64
}

// WithPriority 设置消息优先级。
func (o PushOptions) WithPriority(p Priority) PushOptions {
	o.Priority = p
	return o
}

// WithReliable 设置是否需要 COMMIT 确认。
//
// 默认为 false（尽力而为、不重发、离线丢弃）；需要投递保证时显式传 true。
func (o PushOptions) WithReliable(reliable bool) PushOptions {
	o.Reliable = reliable
	return o
}

// WithTTL 设置相对存活时长；设置 ExpiresAt 时以 ExpiresAt 为准。
func (o PushOptions) WithTTL(ttl time.Duration) PushOptions {
	o.TTL = ttl
	return o
}

// WithExpiresAt 设置绝对过期时间（Unix 毫秒）；<= 0 表示永不过期。
func (o PushOptions) WithExpiresAt(ms int64) PushOptions {
	o.ExpiresAt = ms
	return o
}

// WithPublishedAt 设置发布者时间戳（Unix 毫秒）。
func (o PushOptions) WithPublishedAt(ms int64) PushOptions {
	o.PublishedAt = ms
	return o
}

// WithHeaders 设置消息头。
func (o PushOptions) WithHeaders(h map[string]string) PushOptions {
	o.Headers = maps.Clone(h)
	return o
}

// WithProperties 设置随 PUSH 发送的属性。
func (o PushOptions) WithProperties(props map[string]string) PushOptions {
	o.Properties = maps.Clone(props)
	return o
}

// WithoutAck 不等待服务端确认。
func (o PushOptions) WithoutAck() PushOptions {
	o.NoAck = true
	return o
}

// validate 校验发布选项。
func (o PushOptions) validate() error {
	if !o.Priority.valid() {
		return fmt.Errorf("%w: priority %d", ErrInvalidOption, o.Priority)
	}
	if o.TTL < 0 {
		return fmt.Errorf("%w: negative TTL", ErrInvalidOption)
	}
	return nil
}

// newRecord 构造待发送的记录。
func newRecord(channel, target string, payload []byte, opts PushOptions) packet.Record {
	rec := packet.Record{Channel: channel, Target: target, Payload: payload}
	rec.Headers = opts.Headers
	rec.SetPriority(opts.Priority.packet())
	rec.SetReliable(opts.Reliable)

	now := time.Now().UnixMilli()
	expiresAt := opts.ExpiresAt
	if expiresAt <= 0 && opts.TTL > 0 {
		expiresAt = now + opts.TTL.Milliseconds()
	}
	if expiresAt > 0 {
		rec.SetExpiresAt(expiresAt)
	}
	publishedAt := opts.PublishedAt
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
// NoAck 发布没有服务端响应，返回零值（Reason 为零值即 Success）。
type PushResult struct {
	// ID 是服务端为这条记录分配的 ID。
	ID packet.PacketID
	// Reason 是服务端的处理结果；成功为 ReasonSuccess。
	Reason packet.ReasonCode
	// Matched 是入队成功的客户端数。
	Matched uint32
	// Dropped 是未入队的客户端数（离线不可靠消息、队列溢出、目标接收上限过小等）。
	Dropped uint32
	// Properties 是 PUSH_OK 返回的属性，失败时含 error 细节。
	Properties map[string]string
}

// Push 发布一条消息到指定 channel。
//
// 即使返回错误，PushResult 也可能带有服务端给出的 Reason 与 ID，便于排查。
func (c *Client) Push(ctx context.Context, channel string, payload []byte, opts PushOptions) (PushResult, error) {
	return c.push(ctx, "", channel, payload, opts)
}

// PushTo 定向投递到指定客户端。
//
// 目标存在但没有声明匹配该 channel 的过滤器时返回 ErrTargetNotSubscribed；
// 目标不存在时返回匹配 ErrTargetNotFound 的错误。
func (c *Client) PushTo(ctx context.Context, target, channel string, payload []byte, opts PushOptions) (PushResult, error) {
	if target == "" {
		return PushResult{}, fmt.Errorf("%w: empty target", ErrInvalidOption)
	}
	return c.push(ctx, target, channel, payload, opts)
}

func (c *Client) push(ctx context.Context, target, channel string, payload []byte, opts PushOptions) (PushResult, error) {
	if !packet.ValidateClientChannelName(channel, packet.MaxChannelLength) {
		return PushResult{}, fmt.Errorf("%w: channel %q", ErrInvalidOption, channel)
	}
	if err := opts.validate(); err != nil {
		return PushResult{}, err
	}

	pkt := packet.NewPushPacket(c.nextRequestID(), newRecord(channel, target, payload, opts))
	pkt.Properties = maps.Clone(opts.Properties)
	if !opts.NoAck {
		pkt.Flags |= packet.PushFlagWantAck
	}
	return c.sendPush(ctx, pkt, opts.NoAck, target != "")
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
