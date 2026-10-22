package core

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snple/beacon/packet"
)

// EventType 事件类型。
type EventType string

const (
	EventClientConnected    EventType = "client.connected"
	EventClientDisconnected EventType = "client.disconnected"
	EventSubscribed         EventType = "subscription.added"
	EventUnsubscribed       EventType = "subscription.removed"
	EventMessagePublished   EventType = "message.published"
	EventMessageDropped     EventType = "message.dropped"
	EventMessageRejected    EventType = "message.rejected"
	EventSessionExpired     EventType = "session.expired"
	EventAdminAction        EventType = "admin.action"
)

const (
	// defaultEventBuffer 是每个订阅者默认的缓冲条数。
	defaultEventBuffer = 1024
	// maxEventBuffer 限制单个订阅者的内存占用（约 maxEventBuffer * 200B）。
	maxEventBuffer = 8192
)

// IsValid 报告事件类型是否为已知类型。
func (t EventType) IsValid() bool {
	switch t {
	case EventClientConnected, EventClientDisconnected,
		EventSubscribed, EventUnsubscribed,
		EventMessagePublished, EventMessageDropped, EventMessageRejected,
		EventSessionExpired, EventAdminAction:
		return true
	}
	return false
}

// Event 是管理面事件。不同 Type 使用不同字段：
//
//	client.connected     ClientID, RemoteAddr, SessionPresent
//	client.disconnected  ClientID, RemoteAddr, Reason, Duration
//	subscription.*       ClientID, Topic, QoS, Detail("api" 表示编程接口发起)
//	message.published    ClientID(发布者), Topic, Bytes, QoS, Retained
//	message.dropped      ClientID, Topic, Bytes, QoS, Reason, Detail
//	message.rejected     ClientID, Topic, Bytes, QoS, Reason, Detail
//	session.expired      ClientID, Reason
//	admin.action         Actor, Topic, Bytes, QoS, Reason("publish"), Retained
type Event struct {
	// Seq 是本订阅者内连续递增的序号（从 1 开始）。
	// 序号只对"匹配该订阅者过滤条件"的事件递增，因此跳号数量 == 丢弃数量。
	Seq  uint64
	Time time.Time
	Type EventType

	ClientID string // 事件主体：客户端 / 发布者
	Actor    string // 管理面操作者（目前等于 Publish 的 PublisherID）

	RemoteAddr     string // client.connected/disconnected：对端地址
	SessionPresent bool   // client.connected：是否恢复了旧会话

	Topic    string        // 消息/订阅事件的主题
	Bytes    int           // 消息事件的负载字节数
	QoS      packet.QoS    // 消息/订阅事件的 QoS
	Reason   string        // 原因码名或拒绝/丢弃原因
	Detail   string        // 细分原因（可选）：如 "pending_messages"、"store_bytes"、"api"
	Retained bool          // message.published/admin.action：是否为保留消息
	Duration time.Duration // client.disconnected：连接持续时长
}

// EventOptions 是事件订阅参数。零值表示"订阅全部事件、默认缓冲"。
type EventOptions struct {
	Types       []EventType // 事件类型白名单；空表示全部。未知类型返回 ErrInvalidEventType
	ClientID    string      // 仅该客户端的事件；空表示全部
	TopicPrefix string      // 仅该主题前缀的事件；空表示全部
	Buffer      int         // 每个订阅者的缓冲条数（默认 1024，上限 8192）
}

func (o EventOptions) validate() error {
	for _, t := range o.Types {
		if !t.IsValid() {
			return ErrInvalidEventType
		}
	}
	return nil
}

// EventStream 是一条事件订阅。
//
// 消费方式：
//
//	for ev := range stream.Events() {
//	    ...
//	}
//
// 事件是**尽力而为**的：消费者过慢时新事件被丢弃，`Dropped()` 与 `Event.Seq` 跳号都会反映这一点。
// 需要精确状态时以 `ListClients` / `GetClientInfo` 的快照为准。
type EventStream struct {
	core *Core
	sub  *eventSub
	done chan struct{}
	once sync.Once
}

// Events 返回只读事件通道。通道在 Close() 或 ctx 结束后关闭。
func (s *EventStream) Events() <-chan Event {
	return s.sub.ch
}

// Dropped 返回本订阅者因缓冲已满而丢弃的累计事件数。
func (s *EventStream) Dropped() uint64 {
	return s.sub.dropped.Load()
}

// Close 释放订阅并关闭事件通道。可重复调用。
func (s *EventStream) Close() {
	s.once.Do(func() {
		s.core.events.remove(s.sub)
		close(s.done)
	})
}

// eventSub 是一个订阅者，匹配与序号分配都在总线锁内完成。
type eventSub struct {
	id          uint64
	types       []EventType
	clientID    string
	topicPrefix string
	ch          chan Event

	seq     uint64 // 仅在总线锁内修改
	dropped atomic.Uint64
}

func (s *eventSub) match(ev Event) bool {
	if len(s.types) > 0 {
		matched := false
		for _, t := range s.types {
			if t == ev.Type {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if s.clientID != "" && ev.ClientID != s.clientID {
		return false
	}
	if s.topicPrefix != "" && !strings.HasPrefix(ev.Topic, s.topicPrefix) {
		return false
	}
	return true
}

// eventBus 是多生产者、多消费者的进程内事件总线。
// 发布路径绝不阻塞：缓冲满即丢弃并计数。
type eventBus struct {
	mu     sync.Mutex
	subs   map[uint64]*eventSub
	nextID uint64

	subCount atomic.Int32  // 订阅者数量，供 emit 快速返回
	dropped  atomic.Uint64 // 所有订阅者累计丢弃数（StatsSnapshot.EventsDropped）
}

func newEventBus() *eventBus {
	return &eventBus{subs: make(map[uint64]*eventSub)}
}

// publish 在总线锁内为每个匹配的订阅者分配连续序号并入队。
// 持锁保证同一订阅者看到的 Seq 与实际到达顺序一致（多生产者下也成立）。
func (b *eventBus) publish(ev Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.subs) == 0 {
		return
	}
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}

	for _, sub := range b.subs {
		if !sub.match(ev) {
			continue
		}
		sub.seq++
		out := ev
		out.Seq = sub.seq

		select {
		case sub.ch <- out:
		default:
			sub.dropped.Add(1)
			b.dropped.Add(1)
		}
	}
}

func (b *eventBus) subscribe(opts EventOptions) (*eventSub, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	buffer := opts.Buffer
	if buffer <= 0 {
		buffer = defaultEventBuffer
	}
	if buffer > maxEventBuffer {
		buffer = maxEventBuffer
	}

	b.mu.Lock()
	b.nextID++
	sub := &eventSub{
		id:          b.nextID,
		types:       append([]EventType(nil), opts.Types...),
		clientID:    opts.ClientID,
		topicPrefix: opts.TopicPrefix,
		ch:          make(chan Event, buffer),
	}
	b.subs[sub.id] = sub
	// 计数与注册在同一把锁内更新：emit 的快速返回要么发生在注册前（本事件不送达），
	// 要么发生在注册后（正常扇出），不会出现"已注册但被跳过"的中间态。
	b.subCount.Add(1)
	b.mu.Unlock()

	return sub, nil
}

// remove 注销订阅并关闭通道。与 publish 共用总线锁，避免向已关闭通道发送。
func (b *eventBus) remove(sub *eventSub) {
	b.mu.Lock()
	delete(b.subs, sub.id)
	b.subCount.Add(-1)
	close(sub.ch)
	b.mu.Unlock()
}

// SubscribeEvents 订阅管理面事件。
//
// ctx 结束或调用 Close() 时关闭通道（Close 可重复调用）。
// 消费者过慢时事件被丢弃（不阻塞 broker），可通过 EventStream.Dropped() 与 Seq 跳号观察；
// 需要精确状态时以 ListClients / GetClientInfo 快照为准。
func (c *Core) SubscribeEvents(ctx context.Context, opts EventOptions) (*EventStream, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}

	sub, err := c.events.subscribe(opts)
	if err != nil {
		return nil, err
	}

	s := &EventStream{core: c, sub: sub, done: make(chan struct{})}
	go func() {
		select {
		case <-ctx.Done():
			s.Close()
		case <-s.done:
		}
	}()

	return s, nil
}

// emit 发布一条事件。没有订阅者时直接返回（热路径零开销）。
func (c *Core) emit(ev Event) {
	if c.events.subCount.Load() == 0 {
		return
	}
	c.events.publish(ev)
}
