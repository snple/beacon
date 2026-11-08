package client

import (
	"context"
	"fmt"
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
	// Dup 表示这是服务端重发的消息（此前已投递过但未确认）。
	Dup bool

	client *Client
	acked  atomic.Bool
}

// NeedsCommit 报告该消息是否需要 COMMIT 确认。
func (m *Message) NeedsCommit() bool { return m.Priority.RequiresCommit() }

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

func newMessage(c *Client, rec *packet.Record) *Message {
	return &Message{
		ID:       rec.ID,
		Channel:  rec.Channel,
		Source:   rec.Source,
		Headers:  rec.Headers,
		Payload:  rec.Payload,
		Priority: rec.Priority(),
		Dup:      rec.Dup(),
		client:   c,
	}
}

// PullOptions 拉取选项。
type PullOptions struct {
	// WaitMs 是长轮询时长（毫秒）。0 表示使用 ClientOptions.DefaultWaitMs
	// （默认 30s），因此默认行为是「队列为空时等待」。
	WaitMs uint32

	// Immediate 为 true 时无论 WaitMs 如何都立即返回（非阻塞拉取）。
	Immediate bool
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
// 语义：服务端优先重发「已投递未确认」的消息；因此客户端应在处理完后
// 立即调用 Commit（或 Ack），否则会收到重复内容。
func (c *Client) Pull(ctx context.Context, opts PullOptions) (*Message, error) {
	// 同一时刻只允许一个在途 PULL：并发拉取会拿到重复内容，
	// 且会让本地未确认集合互相覆盖。
	c.pullMu.Lock()
	defer c.pullMu.Unlock()

	waitMs := c.effectiveWait(opts)

	// 长轮询可能在服务端停留 waitMs，客户端等待窗口需要覆盖它。
	timeout := time.Duration(waitMs)*time.Millisecond + 5*time.Second

	// 服务端可能因上一条 PULL 仍在途而返回 RateLimited；退避后重试。
	const maxRateLimitedRetries = 3
	for attempt := 0; ; attempt++ {
		req := packet.NewPullPacket(c.nextRequestID(), waitMs)
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

		if !ok.HasRecord() {
			return nil, nil
		}
		msg := newMessage(c, &ok.Record)
		c.trackUncommitted([]*Message{msg})
		return msg, nil
	}
}

// Commit 确认一条已处理的消息 ID。
func (c *Client) Commit(ctx context.Context, id packet.PacketID) error {
	req := packet.NewCommitPacket(c.nextRequestID(), id)
	resp, err := c.roundTrip(ctx, req, req.RequestID, 10*time.Second)
	if err != nil {
		return err
	}
	if _, ok := resp.(*packet.CommitOkPacket); !ok {
		return fmt.Errorf("unexpected response %s", resp.Type())
	}
	c.markCommitted(id)
	return nil
}

// markCommitted 从本地未确认集合中移除成功提交的消息。
func (c *Client) markCommitted(id packet.PacketID) {
	c.uncommittedMu.Lock()
	defer c.uncommittedMu.Unlock()

	if len(c.uncommitted) == 0 {
		return
	}
	kept := c.uncommitted[:0]
	for _, m := range c.uncommitted {
		if m.ID == id {
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

// CommitAll 逐条确认当前所有需要确认的消息。
//
// 返回实际提交的条数。提交失败时未确认集合保持可重试状态。
func (c *Client) CommitAll(ctx context.Context) (int, error) {
	c.uncommittedMu.Lock()
	pending := c.uncommitted
	c.uncommitted = nil
	c.uncommittedMu.Unlock()

	n := 0
	for i, m := range pending {
		if !m.NeedsCommit() || m.acked.Load() {
			continue
		}
		if err := c.Commit(ctx, m.ID); err != nil {
			// 提交失败：未处理的条目放回，调用方可以重试。
			c.mergeUncommitted(pending[i:])
			return n, err
		}
		m.acked.Store(true)
		n++
	}
	return n, nil
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
func (c *Client) DeclareChannels(ctx context.Context, channels ...Channel) ([]DeclareResult, error) {
	decls := make([]packet.ChannelDecl, len(channels))
	for i, ch := range channels {
		decls[i] = packet.ChannelDecl{Channel: ch.Filter}
		decls[i].SetTargetOnly(ch.TargetOnly)
	}
	req := packet.NewDeclarePacket(c.nextRequestID(), decls)

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
