package packet

import (
	"bytes"
	"testing"
)

// TestPublishFlagsEncoding 测试 Publish Flags 的编码和解码
func TestPublishFlagsEncoding(t *testing.T) {
	tests := []struct {
		name   string
		qos    QoS
		retain bool
		dup    bool
	}{
		{"QoS0", QoS0, false, false},
		{"QoS1", QoS1, false, false},
		{"QoS0+Retain", QoS0, true, false},
		{"QoS1+Retain", QoS1, true, false},
		{"QoS0+Dup", QoS0, false, true},
		{"QoS1+Dup", QoS1, false, true},
		{"QoS1+Retain+Dup", QoS1, true, true},
		{"All flags", QoS1, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建 PUBLISH 包
			pub := &PublishPacket{
				Topic:   "test/topic",
				QoS:     tt.qos,
				Retain:  tt.retain,
				Dup:     tt.dup,
				Payload: []byte("test payload"),
			}

			if tt.qos > 0 {
				pub.PacketID = 123
			}

			// 编码
			var buf bytes.Buffer
			if err := pub.Encode(&buf); err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// 解码固定头部
			r := bytes.NewReader(buf.Bytes())
			var header FixedHeader
			if err := header.Decode(r); err != nil {
				t.Fatalf("Decode header failed: %v", err)
			}

			// 解码 PUBLISH 包
			decoded := &PublishPacket{}
			if err := decoded.Decode(r, header); err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// 验证
			if decoded.QoS != tt.qos {
				t.Errorf("QoS mismatch: got %v, want %v", decoded.QoS, tt.qos)
			}
			if decoded.Retain != tt.retain {
				t.Errorf("Retain mismatch: got %v, want %v", decoded.Retain, tt.retain)
			}
			if decoded.Dup != tt.dup {
				t.Errorf("Dup mismatch: got %v, want %v", decoded.Dup, tt.dup)
			}
			if decoded.Topic != pub.Topic {
				t.Errorf("Topic mismatch: got %v, want %v", decoded.Topic, pub.Topic)
			}
			if tt.qos > 0 && decoded.PacketID != pub.PacketID {
				t.Errorf("PacketID mismatch: got %v, want %v", decoded.PacketID, pub.PacketID)
			}
			if !bytes.Equal(decoded.Payload, pub.Payload) {
				t.Errorf("Payload mismatch: got %v, want %v", decoded.Payload, pub.Payload)
			}
		})
	}
}

// TestPublishFlagsCompactness 测试 Flags 的紧凑性
func TestPublishFlagsCompactness(t *testing.T) {
	// QoS 只需 1 bit (0 或 1)
	// Retain 需 1 bit
	// Dup 需 1 bit
	// 总共 3 bits，在 1 字节内

	tests := []struct {
		qos    QoS
		retain bool
		dup    bool
		expect uint8
	}{
		{QoS0, false, false, 0x00}, // 000
		{QoS1, false, false, 0x01}, // 001
		{QoS0, true, false, 0x02},  // 010
		{QoS1, true, false, 0x03},  // 011
		{QoS0, false, true, 0x04},  // 100
		{QoS1, false, true, 0x05},  // 101
		{QoS0, true, true, 0x06},   // 110
		{QoS1, true, true, 0x07},   // 111
	}

	for _, tt := range tests {
		var flags uint8
		flags |= uint8(tt.qos) & 0x01 // Bit 0: QoS
		if tt.retain {
			flags |= 0x02 // Bit 1: Retain
		}
		if tt.dup {
			flags |= 0x04 // Bit 2: Dup
		}

		if flags != tt.expect {
			t.Errorf("Flags mismatch for QoS=%d, Retain=%v, Dup=%v: got 0x%02X, want 0x%02X",
				tt.qos, tt.retain, tt.dup, flags, tt.expect)
		}

		// 反向验证
		decodedQoS := QoS(flags & 0x01)
		decodedRetain := flags&0x02 != 0
		decodedDup := flags&0x04 != 0

		if decodedQoS != tt.qos {
			t.Errorf("Decoded QoS mismatch: got %v, want %v", decodedQoS, tt.qos)
		}
		if decodedRetain != tt.retain {
			t.Errorf("Decoded Retain mismatch: got %v, want %v", decodedRetain, tt.retain)
		}
		if decodedDup != tt.dup {
			t.Errorf("Decoded Dup mismatch: got %v, want %v", decodedDup, tt.dup)
		}
	}
}

// TestConnectWillFlagsEncoding 测试 Connect 中 Will Flags 的编码和解码
func TestConnectWillFlagsEncoding(t *testing.T) {
	tests := []struct {
		name   string
		qos    QoS
		retain bool
		dup    bool
	}{
		{"Will QoS0", QoS0, false, false},
		{"Will QoS1", QoS1, false, false},
		{"Will QoS0+Retain", QoS0, true, false},
		{"Will QoS1+Retain", QoS1, true, false},
		{"Will QoS1+Dup", QoS1, false, true},
		{"Will All flags", QoS1, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建 CONNECT 包
			conn := NewConnectPacket()
			conn.ClientID = "test-client"
			conn.Will = true
			conn.WillPacket = &PublishPacket{
				Topic:   "will/topic",
				QoS:     tt.qos,
				Retain:  tt.retain,
				Dup:     tt.dup,
				Payload: []byte("will message"),
			}

			// 编码
			var buf bytes.Buffer
			if err := conn.Encode(&buf); err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// 解码固定头部
			r := bytes.NewReader(buf.Bytes())
			var header FixedHeader
			if err := header.Decode(r); err != nil {
				t.Fatalf("Decode header failed: %v", err)
			}

			// 解码 CONNECT 包
			decoded := &ConnectPacket{}
			if err := decoded.Decode(r, header); err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// 验证 Will 消息
			if !decoded.Will {
				t.Fatal("Will flag not set")
			}
			if decoded.WillPacket == nil {
				t.Fatal("WillPacket is nil")
			}
			if decoded.WillPacket.QoS != tt.qos {
				t.Errorf("Will QoS mismatch: got %v, want %v", decoded.WillPacket.QoS, tt.qos)
			}
			if decoded.WillPacket.Retain != tt.retain {
				t.Errorf("Will Retain mismatch: got %v, want %v", decoded.WillPacket.Retain, tt.retain)
			}
			if decoded.WillPacket.Dup != tt.dup {
				t.Errorf("Will Dup mismatch: got %v, want %v", decoded.WillPacket.Dup, tt.dup)
			}
			if decoded.WillPacket.Topic != conn.WillPacket.Topic {
				t.Errorf("Will Topic mismatch: got %v, want %v", decoded.WillPacket.Topic, conn.WillPacket.Topic)
			}
			if !bytes.Equal(decoded.WillPacket.Payload, conn.WillPacket.Payload) {
				t.Errorf("Will Payload mismatch: got %v, want %v", decoded.WillPacket.Payload, conn.WillPacket.Payload)
			}
		})
	}
}
