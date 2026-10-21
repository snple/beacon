package packet

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/danclive/nson-go"
)

// validSeedPackets 构造覆盖各包类型（含属性与遗嘱段）的合法包，作为 fuzz 种子。
func validSeedPackets(t *testing.T) [][]byte {
	t.Helper()

	build := func(pkt Packet) []byte {
		var buf bytes.Buffer
		if err := WritePacket(&buf, pkt, 0); err != nil {
			t.Fatalf("编码种子包 %v 失败: %v", pkt.Type(), err)
		}
		return buf.Bytes()
	}

	conn := NewConnectPacket()
	conn.ClientID = "fuzz-client"
	conn.SessionTimeout = 60
	conn.CleanStart = true
	conn.Properties.AuthMethod = "token"
	conn.Properties.AuthData = []byte{1, 2, 3}
	conn.Properties.MaxPacketSize = 1024
	conn.Properties.ReceiveWindow = 32
	conn.Properties.TraceID = "trace-1"
	conn.Properties.UserProperties["b"] = "2"
	conn.Properties.UserProperties["a"] = "1"

	will := NewPublishPacket("will/topic", []byte("bye"))
	will.QoS = QoS1
	will.Retain = true
	will.Properties.ExpiryTime = 30
	will.Properties.ContentType = "text/plain"
	will.Properties.UserProperties["k"] = "v"
	conn.Will = true
	conn.WillPacket = will

	ping := NewPingPacket(7, 1234567890123456789)
	pong := NewPongPacket(7, 1234567890123456789, 42)

	pub := NewPublishPacket("sensor/a/**", []byte("payload"))
	pub.QoS = QoS1
	pub.Properties.TargetClientID = "target"
	pub.Properties.SourceClientID = "source"
	pub.Properties.ResponseTopic = "resp"
	pub.Properties.CorrelationData = []byte{9, 9}

	sub := NewSubscribePacket(nson.NewId())
	sub.AddSubscription("a/#", QoS1)
	sub.AddSubscription("b/*", QoS0)

	unsub := NewUnsubscribePacket(nson.NewId())
	unsub.Topics = []string{"a/#", "b/*"}

	suback := NewSubackPacket(nson.NewId())
	suback.ReasonCodes = []ReasonCode{ReasonCode(0x00), ReasonCode(0x80)}

	unsuback := NewUnsubackPacket(nson.NewId())
	unsuback.ReasonCodes = []ReasonCode{ReasonCode(0x00)}

	connack := NewConnackPacket(ReasonCode(0x00))
	connack.Properties.ClientID = "fuzz-client"
	connack.Properties.KeepAlive = 30
	connack.Properties.SessionTimeout = 60
	connack.Properties.TraceID = "trace-1"

	puback := NewPubackPacket(nson.NewId(), ReasonCode(0x00))
	puback.Properties.ReasonString = "ok"

	disc := NewDisconnectPacket(ReasonCode(0x00))
	disc.Properties.ReasonString = "bye"

	trace := NewTracePacket("trace-1", "deliver", "fuzz-client")
	trace.Topic = "a/b"
	trace.Timestamp = 1700000000000
	trace.Details["b"] = "2"
	trace.Details["a"] = "1"

	emptyDisc := NewDisconnectPacket(ReasonNormalDisconnect)
	emptyDisc.Properties = nil

	auth := NewAuthPacket(ReasonCode(0x00))
	auth.Properties.AuthMethod = "token"
	auth.Properties.AuthData = []byte{1}

	seeds := [][]byte{
		build(conn), build(connack), build(pub), build(puback),
		build(sub), build(suback), build(unsub), build(unsuback),
		build(ping), build(pong), build(disc), build(auth),
		build(trace), build(emptyDisc),
	}

	// 畸形/边界种子：截断、非法类型、只声明长度的空体
	seeds = append(seeds,
		nil,
		[]byte{},
		[]byte{byte(CONNECT)},
		[]byte{byte(PUBLISH), 0, 0, 0, 19},
		[]byte{byte(PUBLISH), 0xFF, 0xFF, 0xFF, 0xFF},
		[]byte{0x7F, 0, 0, 0, 0},
		append(seeds[0][:len(seeds[0])-3:len(seeds[0])-3], 0xAA),
	)
	return seeds
}

// FuzzReadPacket 对二进制协议解析器做模糊测试。
//
// 高风险区：长度字段（uint32 无上限，曾因 Remaining+5 回绕导致约 4GB 分配）、
// 属性位掩码（未知位/父子长度不一致）、Will 段（Flags 位与后续字段长度）。
// 断言：不 panic、不超量分配，且解码成功后的包能被重新编码。
func FuzzReadPacket(f *testing.F) {
	for _, seed := range validSeedPackets(&testing.T{}) {
		f.Add(seed)
	}

	// 限制上限，避免模糊输入直接触发数百 MB 的合法分配
	const maxSize = 64 << 10

	f.Fuzz(func(t *testing.T, data []byte) {
		pkt, err := ReadPacket(bytes.NewReader(data), maxSize)
		if err != nil {
			return
		}
		if pkt == nil {
			t.Fatal("ReadPacket 返回 nil 包与 nil 错误")
		}

		// 成功解析出的包必须可以重新编码（保证解码不变量与编码一致）
		var buf bytes.Buffer
		if err := WritePacket(&buf, pkt, 0); err != nil {
			t.Fatalf("重新编码 %v 失败: %v", pkt.Type(), err)
		}
		if buf.Len() < 5 {
			t.Fatalf("重新编码后长度异常: %d", buf.Len())
		}

		// 再解析一次，长度必须一致（编码确定性）
		pkt2, err := ReadPacket(bytes.NewReader(buf.Bytes()), maxSize*4)
		if err != nil {
			t.Fatalf("重新解析失败: %v", err)
		}
		var buf2 bytes.Buffer
		if err := WritePacket(&buf2, pkt2, 0); err != nil {
			t.Fatalf("二次编码失败: %v", err)
		}
		if !bytes.Equal(buf.Bytes(), buf2.Bytes()) {
			t.Fatalf("编码不稳定:\n%x\n%x", buf.Bytes(), buf2.Bytes())
		}
	})
}

// FuzzReadFixedHeader 单独对固定头（类型 + 4 字节长度）做模糊测试，
// 覆盖长度字段与包大小的边界判断。
func FuzzReadFixedHeader(f *testing.F) {
	f.Add([]byte{byte(PUBLISH), 0, 0, 0, 0})
	f.Add([]byte{byte(CONNECT), 0, 0, 0, 17})
	f.Add([]byte{byte(PUBLISH), 0xFF, 0xFF, 0xFF, 0xFF})
	f.Add([]byte{0x00, 0, 0, 0, 0})
	f.Add([]byte{byte(TRACE), 0, 0, 0, 1})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 5 {
			return
		}
		var header [5]byte
		copy(header[:], data[:5])

		var h FixedHeader
		if err := h.Decode(bytes.NewReader(header[:])); err != nil {
			return
		}
		if h.Type != PacketType(header[0]) {
			t.Fatalf("类型不匹配: %v != %v", h.Type, PacketType(header[0]))
		}
		if h.Remaining != binary.BigEndian.Uint32(header[1:]) {
			t.Fatalf("长度不匹配: %d != %d", h.Remaining, binary.BigEndian.Uint32(header[1:]))
		}
	})
}
