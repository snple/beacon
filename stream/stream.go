package stream

import (
	"net"
	"time"
)

// Stream 是建立在 core stream 端口上的一条点对点双向字节流。
//
// 它包装数据面连接：Read/Write/Close 与 SetDeadline 等语义与 net.Conn 一致。
// 流的生命周期与双方主连接绑定：任一方会话断开或被接管，服务端会关闭这条流。
type Stream struct {
	net.Conn

	id            uint64
	topic         string
	localClientID string
	peerClientID  string
	metadata      []byte
	properties    map[string]string
	createdAt     time.Time
}

// New 创建 Stream（由客户端库在数据面握手完成后调用）。
func New(
	id uint64,
	topic string,
	localClientID string,
	peerClientID string,
	metadata []byte,
	properties map[string]string,
	conn net.Conn,
) *Stream {
	return &Stream{
		Conn:          conn,
		id:            id,
		topic:         topic,
		localClientID: localClientID,
		peerClientID:  peerClientID,
		metadata:      cloneBytes(metadata),
		properties:    cloneMap(properties),
		createdAt:     time.Now(),
	}
}

// ID 返回流 ID。
func (s *Stream) ID() uint64 { return s.id }

// Topic 返回建流时声明的主题。
func (s *Stream) Topic() string { return s.topic }

// LocalClientID 返回本端 clientID。
func (s *Stream) LocalClientID() string { return s.localClientID }

// PeerClientID 返回对端 clientID。
func (s *Stream) PeerClientID() string { return s.peerClientID }

// Metadata 返回对端在建流时携带的元数据。
func (s *Stream) Metadata() []byte { return cloneBytes(s.metadata) }

// Properties 返回对端在建流时携带的属性。
func (s *Stream) Properties() map[string]string { return cloneMap(s.properties) }

// CreatedAt 返回流建立时间。
func (s *Stream) CreatedAt() time.Time { return s.createdAt }

// CloseWrite 关闭写方向（尽力而为），对端 Read 会得到 io.EOF。
func (s *Stream) CloseWrite() error {
	if cw, ok := s.Conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return s.Conn.Close()
}

func cloneBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

func cloneMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
