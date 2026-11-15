package client

import (
	"context"
	"fmt"
	"maps"
	"sync/atomic"
	"time"

	"github.com/snple/beacon/packet"
)

// Message 是客户端收到的一条消息。
type Message struct {
	// ID 是消息 ID；重发时保持不变，可用于去重。
	ID packet.PacketID
	// Channel 是消息所属 channel。
	Channel string
	// Source 是原始发布者；服务端发布时为空。
	Source string
	// Headers 是发布时携带的消息头。
	Headers map[string]string
	// Payload 是消息内容。
	Payload []byte
	// Priority 是消息优先级，只会是 Low/Medium/High。
	Priority Priority
	// Reliable 表示该消息需要 COMMIT；与优先级独立。
	Reliable bool
	// Dup 表示这是服务端重发的消息（此前已投递过但未确认）。
	Dup bool
	// ExpiresAt 是消息过期时间（Unix 毫秒）；0 表示永不过期。
	ExpiresAt int64
	// PublishedAt 是发布者时间戳（Unix 毫秒）；0 表示未提供。
	PublishedAt int64

	client *Client
	acked  atomic.Bool
}

// NeedsCommit 报告该消息是否需要 COMMIT 确认。
func (m *Message) NeedsCommit() bool { return m.Reliable }

// Expired 报告消息是否已过期。
func (m *Message) Expired() bool {
	return m.ExpiresAt > 0 && m.ExpiresAt <= time.Now().UnixMilli()
}

// Ack 立即确认单条消息。
//
// 不可靠消息无需确认，直接返回 nil。只有 COMMIT 成功后才标记为已确认；
// 失败时调用方可以安全重试。
func (m *Message) Ack(ctx context.Context) error {
	if !m.NeedsCommit() || m.acked.Load() {
		return nil
	}
	if _, err := m.client.Commit(ctx, m.ID); err != nil {
		return err
	}
	m.acked.Store(true)
	return nil
}

// Nack 拒绝单条消息：服务端会从 delivered 移除并触发 Nack 钩子（死信处理）。
//
// 不可靠消息没有 delivered 状态，直接返回 nil。
func (m *Message) Nack(ctx context.Context, reason packet.ReasonCode) error {
	if !m.NeedsCommit() || m.acked.Load() {
		return nil
	}
	if _, err := m.client.Nack(ctx, NackEntry{ID: m.ID, Reason: reason}); err != nil {
		return err
	}
	m.acked.Store(true)
	return nil
}

func newMessage(c *Client, rec *packet.Record) *Message {
	return &Message{
		ID:          rec.ID,
		Channel:     rec.Channel,
		Source:      rec.Source,
		Headers:     rec.Headers,
		Payload:     rec.Payload,
		Priority:    priorityFromPacket(rec.Priority()),
		Reliable:    rec.Reliable(),
		Dup:         rec.Dup(),
		ExpiresAt:   rec.ExpiresAt,
		PublishedAt: rec.PublishedAt,
		client:      c,
	}
}

// PullOptions 是拉取选项。
type PullOptions struct {
	// WaitMs 是长轮询时长（毫秒）。0 表示使用 ClientOptions.DefaultPullWaitMs
	// （默认 30s），因此默认行为是「队列为空时等待」。
	WaitMs uint32
	// Immediate 为 true 时无论 WaitMs 如何都立即返回（非阻塞拉取）。
	Immediate bool
	// MaxCount 是单次最多拉取的记录数（0 = 1）。
	MaxCount uint16
	// MaxBytes 是单次响应中记录编码大小的总和上限（0 = 服务端默认）。
	MaxBytes uint32
	// Properties 是随 PULL 发送的属性。
	Properties map[string]string
}

// effectiveWait 计算本次请求实际使用的长轮询时长；0 表示立即返回。
func (c *Client) effectiveWait(opts PullOptions) uint32 {
	if opts.Immediate {
		return 0
	}
	if opts.WaitMs > 0 {
		return opts.WaitMs
	}
	return c.opts.DefaultPullWaitMs
}

// PullResult 是一次 PULL 的完整结果。
type PullResult struct {
	// Messages 是本批消息；窗口已满时可能只包含重发的未确认消息。
	Messages []*Message
	// Properties 是服务端随 PULL_OK 返回的属性。
	Properties map[string]string
}

// Pull 拉取一条消息；队列为空且等待超时时返回 (nil, nil)。
//
// 语义：服务端在窗口内优先投递新消息，窗口已满时重发「已投递未确认」的
// 可靠消息；因此客户端应在处理完后立即调用 Ack/Nack 或 Commit。
func (c *Client) Pull(ctx context.Context, opts PullOptions) (*Message, error) {
	opts.MaxCount = 1
	res, err := c.PullBatch(ctx, opts)
	if err != nil {
		return nil, err
	}
	if len(res.Messages) == 0 {
		return nil, nil
	}
	return res.Messages[0], nil
}

// PullBatch 拉取一批消息并返回服务端响应属性。
func (c *Client) PullBatch(ctx context.Context, opts PullOptions) (PullResult, error) {
	// 同一时刻只允许一个在途 PULL：并发拉取会拿到重复内容，
	// 且会让本地未确认集合互相覆盖。
	c.pullMu.Lock()
	defer c.pullMu.Unlock()

	waitMs := c.effectiveWait(opts)
	maxCount := opts.MaxCount
	if maxCount == 0 {
		maxCount = 1
	}

	// 长轮询可能在服务端停留 waitMs，客户端等待窗口需要覆盖它。
	timeout := time.Duration(waitMs)*time.Millisecond + 5*time.Second

	// 服务端可能因上一条 PULL 仍在途而返回 RateLimited；退避后重试。
	const maxRateLimitedRetries = 3
	for attempt := 0; ; attempt++ {
		req := packet.NewPullPacket(c.nextRequestID(), waitMs)
		req.MaxCount = maxCount
		req.MaxBytes = opts.MaxBytes
		req.Properties = maps.Clone(opts.Properties)

		resp, err := c.roundTrip(ctx, req, req.RequestID, timeout)
		if err != nil {
			return PullResult{}, err
		}

		ok, valid := resp.(*packet.PullOkPacket)
		if !valid {
			return PullResult{}, ErrNotConnected
		}

		if ok.Reason == packet.ReasonRateLimited {
			if attempt >= maxRateLimitedRetries {
				return PullResult{}, ErrRateLimited
			}
			select {
			case <-ctx.Done():
				return PullResult{}, ctx.Err()
			case <-time.After(time.Second):
				continue
			}
		}

		if err := reasonToError(ok.Reason); err != nil {
			return PullResult{}, err
		}

		result := PullResult{Properties: maps.Clone(ok.Properties)}
		if len(ok.Records) == 0 {
			return result, nil
		}
		msgs := make([]*Message, 0, len(ok.Records))
		for i := range ok.Records {
			msgs = append(msgs, newMessage(c, &ok.Records[i]))
		}
		c.trackUncommitted(msgs)
		result.Messages = msgs
		return result, nil
	}
}

// PollMessage 反复拉取直到拿到一条消息或到达 timeout（timeout <= 0 表示一直等待）。
//
// 不会自动确认任何消息：调用方必须对取到的消息调用 Ack/Nack，
// 否则未确认窗口会被占满，后续拉取只会拿到重发消息。
func (c *Client) PollMessage(ctx context.Context, timeout time.Duration) (*Message, error) {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}

	for {
		// 单次拉取的等待窗口：不超过剩余时间，也不超过客户端默认长轮询时长。
		var waitMs uint32
		if deadline.IsZero() {
			waitMs = c.opts.DefaultPullWaitMs
			if waitMs == 0 {
				// 默认长轮询被关闭时仍然兜底，避免忙循环
				waitMs = 30000
			}
		} else {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return nil, ErrPollTimeout
			}
			waitMs = uint32(remaining.Milliseconds())
			if waitMs == 0 {
				return nil, ErrPollTimeout
			}
			if d := c.opts.DefaultPullWaitMs; d > 0 && waitMs > d {
				waitMs = d
			}
		}

		msg, err := c.Pull(ctx, PullOptions{WaitMs: waitMs})
		if err != nil {
			return nil, err
		}
		if msg != nil {
			return msg, nil
		}
	}
}

// CommitResult 是 COMMIT_OK 的结果摘要。
type CommitResult struct {
	// Committed 是服务端确认的条数。
	Committed uint16
	// Nacked 是服务端接受的 NACK 条数。
	Nacked uint16
	// Unknown 是服务端不认识的 ID 数（已确认、已丢弃或不可靠消息）。
	Unknown uint16
	// Pending 是本次确认后剩余的未投递积压。
	Pending [3]uint32
}

// Commit 确认若干消息 ID；可变参数支持单条与整批调用。
func (c *Client) Commit(ctx context.Context, ids ...packet.PacketID) (CommitResult, error) {
	if len(ids) == 0 {
		return CommitResult{}, nil
	}
	entries := make([]packet.CommitEntry, len(ids))
	for i, id := range ids {
		entries[i] = packet.CommitEntry{ID: id, Action: packet.CommitActionCommit}
	}
	res, err := c.commitEntries(ctx, entries)
	if err != nil {
		return CommitResult{}, err
	}
	c.markCommittedBatch(ids)
	return res, nil
}

// NackEntry 是一条 NACK 请求。
type NackEntry struct {
	// ID 是要拒绝的消息 ID。
	ID packet.PacketID
	// Reason 是给服务端的原因码，供死信/告警使用；协议不改变处理方式。
	Reason packet.ReasonCode
}

// Nack 拒绝若干消息（不再重发，并触发服务端 Nack 钩子）。
func (c *Client) Nack(ctx context.Context, entries ...NackEntry) (CommitResult, error) {
	if len(entries) == 0 {
		return CommitResult{}, nil
	}
	ce := make([]packet.CommitEntry, len(entries))
	ids := make([]packet.PacketID, len(entries))
	for i, e := range entries {
		ce[i] = packet.CommitEntry{ID: e.ID, Action: packet.CommitActionNack, Reason: e.Reason}
		ids[i] = e.ID
	}
	res, err := c.commitEntries(ctx, ce)
	if err != nil {
		return CommitResult{}, err
	}
	c.markCommittedBatch(ids)
	return res, nil
}

// CommitAll 确认当前所有未确认的可靠消息。
//
// 提交失败时未确认集合保持可重试状态；成功与否以服务端返回的
// CommitResult 为准（Unknown 表示该 ID 已不在服务端 delivered 中）。
func (c *Client) CommitAll(ctx context.Context) (CommitResult, error) {
	c.uncommittedMu.Lock()
	pending := c.uncommitted
	c.uncommitted = nil
	c.uncommittedMu.Unlock()

	ids := make([]packet.PacketID, 0, len(pending))
	for _, m := range pending {
		if m.NeedsCommit() && !m.acked.Load() {
			ids = append(ids, m.ID)
		}
	}
	if len(ids) == 0 {
		return CommitResult{}, nil
	}

	entries := make([]packet.CommitEntry, len(ids))
	for i, id := range ids {
		entries[i] = packet.CommitEntry{ID: id, Action: packet.CommitActionCommit}
	}
	res, err := c.commitEntries(ctx, entries)
	if err != nil {
		// 提交失败：全部未确认条目放回，调用方可以重试。
		c.mergeUncommitted(pending)
		return CommitResult{}, err
	}
	for _, m := range pending {
		m.acked.Store(true)
	}
	return res, nil
}

func (c *Client) commitEntries(ctx context.Context, entries []packet.CommitEntry) (CommitResult, error) {
	req := packet.NewCommitBatchPacket(c.nextRequestID(), entries)
	resp, err := c.roundTrip(ctx, req, req.RequestID, 10*time.Second)
	if err != nil {
		return CommitResult{}, err
	}
	ok, valid := resp.(*packet.CommitOkPacket)
	if !valid {
		return CommitResult{}, fmt.Errorf("unexpected response %s", resp.Type())
	}
	return CommitResult{
		Committed: ok.Committed,
		Nacked:    ok.Nacked,
		Unknown:   ok.Unknown,
		Pending:   ok.Pending,
	}, nil
}

// markCommittedBatch 从本地未确认集合中移除已提交/拒绝的消息。
func (c *Client) markCommittedBatch(ids []packet.PacketID) {
	if len(ids) == 0 {
		return
	}
	done := make(map[packet.PacketID]struct{}, len(ids))
	for _, id := range ids {
		done[id] = struct{}{}
	}

	c.uncommittedMu.Lock()
	defer c.uncommittedMu.Unlock()

	if len(c.uncommitted) == 0 {
		return
	}
	kept := c.uncommitted[:0]
	for _, m := range c.uncommitted {
		if _, ok := done[m.ID]; ok {
			m.acked.Store(true)
			continue
		}
		kept = append(kept, m)
	}
	clear(c.uncommitted[len(kept):])
	c.uncommitted = kept
}

// mergeUncommitted 把额外的未确认消息放回集合，按 ID 去重并保持原顺序。
func (c *Client) mergeUncommitted(extra []*Message) {
	if len(extra) == 0 {
		return
	}

	c.uncommittedMu.Lock()
	defer c.uncommittedMu.Unlock()

	known := make(map[packet.PacketID]struct{}, len(extra)+len(c.uncommitted))
	merged := make([]*Message, 0, len(extra)+len(c.uncommitted))
	for _, m := range extra {
		if _, ok := known[m.ID]; ok {
			continue
		}
		known[m.ID] = struct{}{}
		merged = append(merged, m)
	}
	for _, m := range c.uncommitted {
		if _, ok := known[m.ID]; ok {
			continue
		}
		known[m.ID] = struct{}{}
		merged = append(merged, m)
	}
	c.uncommitted = merged
}

// Channel 是一项 channel 声明。
type Channel struct {
	// Filter 是 channel 过滤器（支持 "*" 单层与 "**" 多层通配符）。
	Filter string
	// TargetOnly 为 true 时只接收定向投递（PushTo），不参与广播匹配。
	TargetOnly bool
}

// Filter 构造同时接收广播与定向投递的声明。
func Filter(filter string) Channel { return Channel{Filter: filter} }

// TargetOnly 构造只接收定向投递的声明。
func TargetOnly(filter string) Channel { return Channel{Filter: filter, TargetOnly: true} }

// ChannelResult 是单个 channel 声明的结果。
type ChannelResult struct {
	// Channel 是被拒绝的过滤器。
	Channel string
	// Reason 是拒绝原因。
	Reason packet.ReasonCode
}

// DeclareResult 是 DECLARE_RESP 的完整结果。
type DeclareResult struct {
	// Rejected 只包含被拒绝的项（服务端为压缩响应不回显成功项）。
	Rejected []ChannelResult
	// Accepted 是被接受的项数。
	Accepted uint16
	// Pending 是 Low / Medium / High 三个优先级的未投递积压。
	Pending [3]uint32
	// Properties 是服务端随 DECLARE_RESP 返回的属性。
	Properties map[string]string
}

// Declare 全量替换 channel 声明并返回完整结果。
//
// 空切片表示取消全部兴趣。请求会被记住，并在每次重连后自动重放。
func (c *Client) Declare(ctx context.Context, channels ...Channel) (DeclareResult, error) {
	return c.DeclareWithProperties(ctx, nil, channels...)
}

// DeclareWithProperties 是 Declare 的属性版本：属性会随 DECLARE 发送，
// 并在每次重连后随声明一起重放。
func (c *Client) DeclareWithProperties(ctx context.Context, props map[string]string, channels ...Channel) (DeclareResult, error) {
	res, err := c.declare(ctx, channels, props)
	if err != nil {
		return DeclareResult{}, err
	}
	c.setDesiredChannels(channels, props)
	return res, nil
}

// declare 发送 DECLARE 但不更新期望声明（重连重放使用，避免覆盖用户意图）。
func (c *Client) declare(ctx context.Context, channels []Channel, props map[string]string) (DeclareResult, error) {
	decls := make([]packet.ChannelDecl, len(channels))
	for i, ch := range channels {
		if !packet.ValidateChannelFilterLimit(ch.Filter, packet.MaxChannelLength) {
			return DeclareResult{}, fmt.Errorf("%w: channel filter %q", ErrInvalidOption, ch.Filter)
		}
		decls[i] = packet.ChannelDecl{Channel: ch.Filter}
		decls[i].SetTargetOnly(ch.TargetOnly)
	}

	req := packet.NewDeclarePacket(c.nextRequestID(), decls)
	req.Properties = maps.Clone(props)

	resp, err := c.roundTrip(ctx, req, req.RequestID, 10*time.Second)
	if err != nil {
		return DeclareResult{}, err
	}
	ok, valid := resp.(*packet.DeclareRespPacket)
	if !valid {
		return DeclareResult{}, ErrNotConnected
	}

	out := make([]ChannelResult, len(ok.Results))
	for i, r := range ok.Results {
		out[i] = ChannelResult{Channel: r.Channel, Reason: r.Reason}
	}

	return DeclareResult{
		Rejected:   out,
		Accepted:   ok.Accepted,
		Pending:    ok.Pending,
		Properties: maps.Clone(ok.Properties),
	}, nil
}

// setDesiredChannels 记住最近一次声明请求（保持用户意图，失败项也保留，
// 以便 ACL 恢复后重连时重试）。
func (c *Client) setDesiredChannels(channels []Channel, props map[string]string) {
	merged := mergeChannels(channels)
	c.declMu.Lock()
	c.desiredChannels = merged
	c.desiredDeclareProps = maps.Clone(props)
	c.declMu.Unlock()
}

func (c *Client) desiredChannelSnapshot() ([]Channel, map[string]string) {
	c.declMu.Lock()
	defer c.declMu.Unlock()
	return append([]Channel(nil), c.desiredChannels...), maps.Clone(c.desiredDeclareProps)
}

// mergeChannels 按过滤器去重；同一过滤器重复声明时保留更宽松的广播模式，
// 与服务端的合并规则一致。
func mergeChannels(channels []Channel) []Channel {
	if len(channels) == 0 {
		return nil
	}
	out := make([]Channel, 0, len(channels))
	index := make(map[string]int, len(channels))
	for _, ch := range channels {
		if pos, ok := index[ch.Filter]; ok {
			if !ch.TargetOnly {
				out[pos].TargetOnly = false
			}
			continue
		}
		index[ch.Filter] = len(out)
		out = append(out, ch)
	}
	return out
}
