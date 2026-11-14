package core

import (
	"sync/atomic"

	"github.com/snple/beacon/packet"
)

// messageOverhead 是每条消息除内容之外的近似固定开销，仅用于内存水位的粗略估算。
const messageOverhead int64 = 160

// Message 是一条已接收、正在各客户端队列之间共享的消息。
//
// 内容在一次发布的生命周期内不可变：channel、headers、payload 由所有命中的
// 客户端共享。每个客户端队列只持有 *Message 指针与自己的投递 ID，因此 1→N
// 分发不复制消息内容。
//
// 生命周期由 refs 引用计数管理：归零时从 core 的在途统计中扣减，内容随 GC 回收。
type Message struct {
	ID       packet.PacketID
	Channel  string
	Target   string // 定向投递目标；空表示普通发布
	Source   string
	Headers  map[string]string
	Payload  []byte
	Priority packet.Priority
	// Reliable 表示该消息需要 COMMIT；与 Priority 独立。
	Reliable bool
	// ExpiresAt 是过期时间（Unix 毫秒）；0 表示永不过期。
	ExpiresAt int64
	// PublishedAt 是发布者时间戳（Unix 毫秒）；0 表示未提供。
	PublishedAt int64

	// ReceivedAt 是 broker 接收该消息的时间（Unix 毫秒），仅用于观测。
	ReceivedAt int64

	// size 是近似内存占用，创建时算一次（多次出队/回退路径都会读它）。
	size int64

	refs atomic.Int32
}

// recordMessageSize 估算一条记录作为共享消息的内存占用。
func recordMessageSize(rec *packet.Record) int64 {
	size := int64(len(rec.Channel) + len(rec.Target) + len(rec.Source) + len(rec.Payload))
	for k, v := range rec.Headers {
		size += int64(len(k) + len(v))
	}
	if rec.ExpiresAt > 0 {
		size += 16
	}
	if rec.PublishedAt > 0 {
		size += 16
	}
	return size + messageOverhead
}

// newMessage 由解码后的记录创建共享消息。
func newMessage(rec *packet.Record) *Message {
	return &Message{
		ID:          rec.ID,
		Channel:     rec.Channel,
		Target:      rec.Target,
		Source:      rec.Source,
		Headers:     rec.Headers,
		Payload:     rec.Payload,
		Priority:    rec.Priority(),
		Reliable:    rec.Reliable(),
		ExpiresAt:   rec.ExpiresAt,
		PublishedAt: rec.PublishedAt,
		size:        recordMessageSize(rec),
	}
}

// retain 增加引用。
func (m *Message) retain() { m.refs.Add(1) }

// release 减少引用；归零时返回 true，调用方应据此扣减在途统计。
func (m *Message) release() bool { return m.refs.Add(-1) <= 0 }

// record 生成本次投递使用的记录视图（与共享内容共用底层数组，不复制）。
func (m *Message) record(dup bool) packet.Record {
	rec := packet.Record{
		ID:      m.ID,
		Channel: m.Channel,
		Target:  m.Target,
		Source:  m.Source,
		Headers: m.Headers,
		Payload: m.Payload,
	}
	rec.SetPriority(m.Priority)
	rec.SetReliable(m.Reliable)
	rec.SetExpiresAt(m.ExpiresAt)
	rec.SetPublishedAt(m.PublishedAt)
	rec.SetDup(dup)
	return rec
}

// Expired 报告消息是否已在 now（Unix 毫秒）之前过期。
func (m *Message) Expired(now int64) bool {
	return m.ExpiresAt > 0 && m.ExpiresAt <= now
}

// Size 返回消息内容的近似内存占用（创建时计算并缓存）。
func (m *Message) Size() int64 {
	if m == nil {
		return 0
	}
	return m.size
}
