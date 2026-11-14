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
	ID       packet.PacketID
	Channel  string
	Source   string
	Headers  map[string]string
	Payload  []byte
	Priority packet.Priority
	// Reliable 表示该消息需要 COMMIT；与 Priority 独立。
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
// 只有 COMMIT 成功后才标记为已确认；失败时调用方可以安全重试。
func (m *Message) Ack(ctx context.Context) error {
	if !m.NeedsCommit() || m.acked.Load() {
		return nil
	}
	if err := m.client.Commit(ctx, m.ID); err != nil {
		return err
	}
	m.acked.Store(true)
	return nil
}

// Nack 拒绝单条消息：服务端会从 delivered 移除并触发 Nack 钩子（死信处理）。
func (m *Message) Nack(ctx context.Context, reason packet.ReasonCode) error {
	if m.acked.Load() {
		return nil
	}
	if err := m.client.Nack(ctx, m.ID, reason); err != nil {
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
		Priority:    rec.Priority(),
		Reliable:    rec.Reliable(),
		Dup:         rec.Dup(),
		ExpiresAt:   rec.ExpiresAt,
		PublishedAt: rec.PublishedAt,
		client:      c,
	}
}

// PullOptions 拉取选项。
type PullOptions struct {
	// WaitMs 是长轮询时长（毫秒）。0 表示使用 ClientOptions.DefaultWaitMs
	// （默认 30s），因此默认行为是「队列为空时等待」。
	WaitMs uint32

	// Immediate 为 true 时无论 WaitMs 如何都立即返回（非阻塞拉取）。
	Immediate bool

	// MaxCount 是单次最多拉取的记录数（0 = 1）。
	MaxCount uint16
	// MaxBytes 是单次响应中记录编码大小的总和上限（0 = 服务端默认）。
	MaxBytes uint32
	// Properties 是随 PULL 发送的可选属性。
	Properties map[string]string
}

// effectiveWait 计算本次请求实际使用的长轮询时长。
// 0 表示立即返回。
func (c *Client) effectiveWait(opts PullOptions) uint32 {
	if opts.Immediate {
		return 0
	}
	if opts.WaitMs > 0 {
		return opts.WaitMs
	}
	return c.opts.DefaultWaitMs
}

// Pull 拉取一条消息；队列为空且等待超时时返回 (nil, nil)。
//
// 语义：服务端在窗口内优先投递新消息，窗口已满时重发「已投递未确认」的
// 可靠消息；因此客户端应在处理完后立即调用 Commit/Nack（或 Ack）。
func (c *Client) Pull(ctx context.Context, opts PullOptions) (*Message, error) {
	opts.MaxCount = 1
	msgs, err := c.PullBatch(ctx, opts)
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, nil
	}
	return msgs[0], nil
}

// PullBatch 拉取一批消息；队列为空且等待超时时返回空切片。
func (c *Client) PullBatch(ctx context.Context, opts PullOptions) ([]*Message, error) {
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
			return nil, err
		}

		ok, valid := resp.(*packet.PullOkPacket)
		if !valid {
			return nil, ErrNotConnected
		}

		if ok.Reason == packet.ReasonRateLimited {
			if attempt >= maxRateLimitedRetries {
				return nil, ErrRateLimited
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Second):
				continue
			}
		}

		if err := reasonToError(ok.Reason); err != nil {
			return nil, err
		}

		if len(ok.Records) == 0 {
			return nil, nil
		}
		msgs := make([]*Message, 0, len(ok.Records))
		for i := range ok.Records {
			msgs = append(msgs, newMessage(c, &ok.Records[i]))
		}
		c.trackUncommitted(msgs)
		return msgs, nil
	}
}

// Commit 确认一条已处理的消息 ID。
func (c *Client) Commit(ctx context.Context, id packet.PacketID) error {
	return c.CommitBatch(ctx, []packet.PacketID{id})
}

// CommitBatch 批量确认消息。
func (c *Client) CommitBatch(ctx context.Context, ids []packet.PacketID) error {
	entries := make([]packet.CommitEntry, len(ids))
	for i, id := range ids {
		entries[i] = packet.CommitEntry{ID: id, Action: packet.CommitActionCommit}
	}
	if _, err := c.commitEntries(ctx, entries); err != nil {
		return err
	}
	c.markCommittedBatch(ids)
	return nil
}

// NackEntry 是一条 NACK 请求。
type NackEntry struct {
	ID     packet.PacketID
	Reason packet.ReasonCode
}

// Nack 拒绝一条消息（不再重发，触发服务端 Nack 钩子）。
func (c *Client) Nack(ctx context.Context, id packet.PacketID, reason packet.ReasonCode) error {
	return c.NackBatch(ctx, []NackEntry{{ID: id, Reason: reason}})
}

// NackBatch 批量拒绝消息。
func (c *Client) NackBatch(ctx context.Context, entries []NackEntry) error {
	ce := make([]packet.CommitEntry, len(entries))
	for i, e := range entries {
		ce[i] = packet.CommitEntry{ID: e.ID, Action: packet.CommitActionNack, Reason: e.Reason}
	}
	if _, err := c.commitEntries(ctx, ce); err != nil {
		return err
	}
	ids := make([]packet.PacketID, len(entries))
	for i, e := range entries {
		ids[i] = e.ID
	}
	c.markCommittedBatch(ids)
	return nil
}

func (c *Client) commitEntries(ctx context.Context, entries []packet.CommitEntry) (*packet.CommitOkPacket, error) {
	req := packet.NewCommitBatchPacket(c.nextRequestID(), entries)
	resp, err := c.roundTrip(ctx, req, req.RequestID, 10*time.Second)
	if err != nil {
		return nil, err
	}
	ok, valid := resp.(*packet.CommitOkPacket)
	if !valid {
		return nil, fmt.Errorf("unexpected response %s", resp.Type())
	}
	return ok, nil
}

// markCommittedBatch 从本地未确认集合中移除成功提交/拒绝的消息。
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

// CommitAll 批量确认当前所有需要确认的消息。
//
// 返回实际提交的条数。提交失败时未确认集合保持可重试状态。
func (c *Client) CommitAll(ctx context.Context) (int, error) {
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
		return 0, nil
	}

	entries := make([]packet.CommitEntry, len(ids))
	for i, id := range ids {
		entries[i] = packet.CommitEntry{ID: id, Action: packet.CommitActionCommit}
	}
	if _, err := c.commitEntries(ctx, entries); err != nil {
		// 提交失败：全部未确认条目放回，调用方可以重试。
		c.mergeUncommitted(pending)
		return 0, err
	}
	for _, m := range pending {
		m.acked.Store(true)
	}
	return len(ids), nil
}

// PollMessage 逐条取消息；timeout<=0 表示一直等待。
func (c *Client) PollMessage(ctx context.Context, timeout time.Duration) (*Message, error) {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}

	for {
		if c.hasUncommitted() {
			if _, err := c.CommitAll(ctx); err != nil {
				return nil, err
			}
		}

		// 单次拉取的等待窗口：不超过剩余时间，也不超过客户端默认长轮询时长。
		var waitMs uint32
		if deadline.IsZero() {
			waitMs = c.opts.DefaultWaitMs
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
			if d := c.opts.DefaultWaitMs; d > 0 && waitMs > d {
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

// DeclareResult 单个 channel 的声明结果。
type DeclareResult struct {
	Channel string
	Reason  packet.ReasonCode
}

// Channel 是一项 channel 声明。
type Channel struct {
	// Filter 是 channel 过滤器（支持 "*" / "**" 通配符）。
	Filter string
	// TargetOnly 为 true 时只接收定向投递（PushTo），不参与广播匹配。
	// 默认同时接收广播与定向投递。
	TargetOnly bool
}

// Declare 全量替换 channel 声明：声明的过滤器同时接收广播与定向投递。
//
// 返回的切片只包含被拒绝的项（服务端为压缩响应只回显失败项）。
func (c *Client) Declare(ctx context.Context, channels ...string) ([]DeclareResult, error) {
	decls := make([]Channel, len(channels))
	for i, ch := range channels {
		decls[i] = Channel{Filter: ch}
	}
	return c.DeclareChannels(ctx, decls...)
}

// DeclareChannels 全量替换 channel 声明，可逐项指定是否只接收定向投递。
//
// 返回的切片只包含被拒绝的项（服务端为压缩响应只回显失败项）。
// 请求会被记住，并在每次重连后自动重放。
func (c *Client) DeclareChannels(ctx context.Context, channels ...Channel) ([]DeclareResult, error) {
	return c.DeclareChannelsWithProperties(ctx, nil, channels...)
}

// DeclareWithProperties 是 Declare 的属性版本：属性会随 DECLARE 发送并记录，
// 在每次重连后随声明一起重放。
func (c *Client) DeclareWithProperties(ctx context.Context, props map[string]string, channels ...string) ([]DeclareResult, error) {
	decls := make([]Channel, len(channels))
	for i, ch := range channels {
		decls[i] = Channel{Filter: ch}
	}
	return c.DeclareChannelsWithProperties(ctx, props, decls...)
}

// DeclareChannelsWithProperties 是 DeclareChannels 的属性版本。
func (c *Client) DeclareChannelsWithProperties(ctx context.Context, props map[string]string, channels ...Channel) ([]DeclareResult, error) {
	results, err := c.declareChannels(ctx, channels, props)
	if err != nil {
		return nil, err
	}
	c.setDesiredChannels(channels, props)
	return results, nil
}

func (c *Client) declareChannels(ctx context.Context, channels []Channel, props map[string]string) ([]DeclareResult, error) {
	decls := make([]packet.ChannelDecl, len(channels))
	for i, ch := range channels {
		decls[i] = packet.ChannelDecl{Channel: ch.Filter}
		decls[i].SetTargetOnly(ch.TargetOnly)
	}
	req := packet.NewDeclarePacket(c.nextRequestID(), decls)
	req.Properties = maps.Clone(props)

	resp, err := c.roundTrip(ctx, req, req.RequestID, 10*time.Second)
	if err != nil {
		return nil, err
	}
	ok, valid := resp.(*packet.DeclareRespPacket)
	if !valid {
		return nil, ErrNotConnected
	}

	out := make([]DeclareResult, len(ok.Results))
	for i, r := range ok.Results {
		out[i] = DeclareResult{Channel: r.Channel, Reason: r.Reason}
	}

	return out, nil
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
