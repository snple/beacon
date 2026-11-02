package packet

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
)

// PublishBody 是 PUBLISH 帧中可被多个接收者共享的部分。
//
// PUBLISH 的线格式（见 PublishPacket.encode）为：
//
//	[type(1)][remaining(4)][flags(1)][topic][PacketID(12)][properties][payload]
//
// 其中 flags（QoS/Retain/Dup）与 PacketID 是逐接收者的投递层字段，其余字节
// 对所有接收者完全相同。PublishBody 在发布时把 topic 与 properties 各编码一次，
// payload 直接引用原切片；配合 PublishFrame.Segments 分段写出，向 N 个订阅者
// 投递同一条消息不需要复制任何消息内容。
type PublishBody struct {
	topic   []byte
	props   []byte
	payload []byte
}

// NewPublishBody 预编码 PUBLISH 的可共享部分。
// payload 不被复制：调用方必须保证在消息生命周期内不修改它。
func NewPublishBody(topic string, props *PublishProperties, payload []byte) (*PublishBody, error) {
	if props == nil {
		props = NewPublishProperties()
	}

	var topicWire bytes.Buffer
	if err := EncodeString(&topicWire, topic); err != nil {
		return nil, err
	}

	var propsWire bytes.Buffer
	if err := props.Encode(&propsWire); err != nil {
		return nil, err
	}

	return &PublishBody{
		topic:   topicWire.Bytes(),
		props:   propsWire.Bytes(),
		payload: payload,
	}, nil
}

// RemainingLength 返回不含固定头的剩余长度（从 flags 起算）。
func (b *PublishBody) RemainingLength() uint32 {
	return uint32(1 + len(b.topic) + PacketIDSize + len(b.props) + len(b.payload))
}

// TotalSize 返回完整帧长度（含 5 字节固定头）。
func (b *PublishBody) TotalSize() uint32 {
	return 5 + b.RemainingLength()
}

// Frame 生成一次投递的帧描述：共享 body，只携带逐接收者字段。
func (b *PublishBody) Frame(id PacketID, qos QoS, retain, dup bool) PublishFrame {
	var head [18]byte
	head[0] = byte(PUBLISH)
	binary.BigEndian.PutUint32(head[1:5], b.RemainingLength())

	var flags uint8
	flags |= uint8(qos) & 0x01
	if retain {
		flags |= 0x02
	}
	if dup {
		flags |= 0x04
	}
	head[5] = flags

	copy(head[6:], id[:])

	return PublishFrame{head: head, body: b}
}

// PublishFrame 是一次投递的 PUBLISH 帧。
//
// head 中同时保存固定头、flags 与 PacketID：线格式要求 PacketID 排在 topic
// 之后，因此这三段被拆成 head[0:6] 与 head[6:18] 两个分段。
type PublishFrame struct {
	head [18]byte
	body *PublishBody
}

// Size 返回帧的总字节数。
func (f *PublishFrame) Size() uint32 {
	return f.body.TotalSize()
}

// Segments 返回按线格式顺序排列的分段。分段直接引用共享内存，不做复制。
func (f PublishFrame) Segments() [][]byte {
	segs := make([][]byte, 0, 5)
	segs = append(segs, f.head[0:6])
	if len(f.body.topic) > 0 {
		segs = append(segs, f.body.topic)
	}
	segs = append(segs, f.head[6:18])
	if len(f.body.props) > 0 {
		segs = append(segs, f.body.props)
	}
	if len(f.body.payload) > 0 {
		segs = append(segs, f.body.payload)
	}
	return segs
}

// WriteTo 分段写出整个帧。
func (f PublishFrame) WriteTo(w io.Writer) (int64, error) {
	return WriteSegments(w, f.Segments())
}

// SegmentedWriter 是支持一次写出多段数据的 Writer（writev 语义）。
//
// *net.TCPConn、*net.UnixConn 以及 internal/wsconn.Conn 都满足该接口。
type SegmentedWriter interface {
	WriteSegments(segs [][]byte) (int64, error)
}

// WriteSegments 按线格式顺序写出多段数据。
//
// 若 Writer 支持分段写（TCP writev、WebSocket 单条消息），一次性写出以免接收端
// 看到不完整的分帧；否则退化为顺序 Write：仍然不复制数据，只是多几次系统调用
// （TLS 就是这种情况）。
func WriteSegments(w io.Writer, segs [][]byte) (int64, error) {
	if sw, ok := w.(SegmentedWriter); ok {
		return sw.WriteSegments(segs)
	}

	bufs := net.Buffers(segs)
	return bufs.WriteTo(w)
}
