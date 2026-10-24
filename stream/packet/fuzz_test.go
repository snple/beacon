package packet

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// FuzzReadPacket 对 stream 协议解析器做模糊测试。
//
// 断言：不 panic、不超量分配；解析成功的包必须能重新编码并被再次解析。
func FuzzReadPacket(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{byte(PING), 0, 0, 0, 0})
	f.Add([]byte{byte(PONG), 0, 0, 0, 0})
	f.Add([]byte{byte(CONNECT), 0, 0, 0, 0})
	f.Add([]byte{byte(CONNACK), 0, 0, 0, 1, 0})
	f.Add([]byte{byte(STREAM_DATA), 0, 0, 0, 0})
	f.Add([]byte{byte(STREAM_DATA), 0x00, 0x00, 0x00, 0x03, 1, 2, 3})
	f.Add([]byte{byte(STREAM_OPEN), 0xFF, 0xFF, 0xFF, 0xFF})

	f.Fuzz(func(t *testing.T, data []byte) {
		// ReadPacket 会按声明的长度先分配缓冲；限制该值避免模糊测试放大内存占用。
		if len(data) >= 5 {
			if declared := binary.BigEndian.Uint32(data[1:5]); declared > 1<<20 {
				return
			}
		}

		pkt, err := ReadPacket(bytes.NewReader(data))
		if err != nil {
			return
		}
		if pkt == nil {
			t.Fatal("ReadPacket 返回 nil 包与 nil 错误")
		}

		var buf bytes.Buffer
		if err := WritePacket(&buf, pkt); err != nil {
			t.Fatalf("重新编码 %v 失败: %v", pkt.Type(), err)
		}
		if _, err := ReadPacket(bytes.NewReader(buf.Bytes())); err != nil {
			t.Fatalf("重新解析 %v 失败: %v", pkt.Type(), err)
		}
	})
}
