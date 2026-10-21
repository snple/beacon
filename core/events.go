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
	// maxEventBuffer 限制单个订阅者可能占用的内存（条数）。
	maxEventBuffer = 65536
)

// Event 是管理面事件。
//
// 事件流是**尽力而为**的：订阅者消费过慢时事件会被丢弃并计入
// StatsSnapshot.EventsDropped，结合 Seq 跳号可判断是否丢过。
// 需要精确状态时请以 List* 系列 API 为准做对账。
type Event struct {
	Seq      uint64 // 单调递增，全进程唯一
	Time     time.Time
	Type     EventType
	ClientID string
	Topic    string
	Bytes    int
	QoS      packet.QoS
	Reason   string            // reason code 名或丢弃原因
	Actor    string            // 管理操作的操作者
	Extra    map[string]string // 预留扩展
}

// EventFilter 事件过滤条件，零值表示订阅全部事件。
type EventFilter struct {
	Types       []EventType
	ClientID    string
	TopicPrefix string
	Buffer      int // 每个订阅者的缓冲条数（默认 1024，上限 65536）
}

func (f EventFilter) match(ev Event) bool {
	if f.ClientID != "" && ev.ClientID != f.ClientID {
		return false
	}
	if f.TopicPrefix != "" && !strings.HasPrefix(ev.Topic, f.TopicPrefix) {
		return false
	}
	if len(f.Types) == 0 {
		return true
	}
	for _, t := range f.Types {
		if t == ev.Type {
			return true
		}
	}
	return false
}

// eventBus 是单生产者、多消费者的进程内事件总线。
// 发布路径绝不阻塞：缓冲满即丢弃并计数。
type eventBus struct {
	mu   sync.Mutex
	next uint64
	subs map[uint64]*eventSub

	seq     atomic.Uint64
	dropped atomic.Int64
}

type eventSub struct {
	filter EventFilter
	ch     chan Event
}

func newEventBus() *eventBus {
	return &eventBus{subs: make(map[uint64]*eventSub)}
}

func (b *eventBus) publish(ev Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, sub := range b.subs {
		if !sub.filter.match(ev) {
			continue
		}
		select {
		case sub.ch <- ev:
		default:
			b.dropped.Add(1)
		}
	}
}

func (b *eventBus) subscribe(filter EventFilter) (<-chan Event, uint64) {
	sub := &eventSub{
		filter: filter,
		ch:     make(chan Event, filter.Buffer),
	}

	b.mu.Lock()
	b.next++
	id := b.next
	b.subs[id] = sub
	b.mu.Unlock()

	return sub.ch, id
}

// unsubscribe 移除订阅并关闭通道。
// 关闭动作在总线锁内完成，与 publish 的发送互斥，避免向已关闭通道发送。
func (b *eventBus) unsubscribe(id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if sub, ok := b.subs[id]; ok {
		delete(b.subs, id)
		close(sub.ch)
	}
}

// SubscribeEvents 订阅管理面事件。
//
// 返回的通道在 ctx 结束后关闭，因此调用方必须取消 ctx 以释放订阅。
// 消费者过慢时事件被丢弃（不阻塞 broker），丢弃数量见 StatsSnapshot.EventsDropped。
func (c *Core) SubscribeEvents(ctx context.Context, filter EventFilter) (<-chan Event, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if filter.Buffer <= 0 {
		filter.Buffer = defaultEventBuffer
	}
	if filter.Buffer > maxEventBuffer {
		filter.Buffer = maxEventBuffer
	}

	ch, id := c.events.subscribe(filter)
	go func() {
		<-ctx.Done()
		c.events.unsubscribe(id)
	}()

	return ch, nil
}

// emit 发布一条事件。Time 为零时自动填充，Seq 由总线分配。
func (c *Core) emit(ev Event) {
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	ev.Seq = c.events.seq.Add(1)
	c.events.publish(ev)
}
