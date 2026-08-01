package packet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"
)

// TestReadPacket_RejectsOverflowRemaining 验证 ReadPacket 对超大 Remaining 的拒绝行为。
//
// 回归：header.Remaining 直接来自对端（4 字节无符号），当 Remaining 在
// 0xFFFFFFFB ~ 0xFFFFFFFF 区间时，旧代码的 totalSize := Remaining + 5 会发生
// uint32 回绕，绕过大小检查并尝试 make([]byte, Remaining) 分配约 4GB 内存，
// 导致远程 OOM。
//
// 此测试只提供 5 字节固定头（无包体）：修复后的代码必须在读取/分配包体之前
// 就返回 PacketTooLargeError，而不是尝试分配超大缓冲区。
func TestReadPacket_RejectsOverflowRemaining(t *testing.T) {
	overflowValues := []uint32{
		0xFFFFFFF0,
		0xFFFFFFFB,
		0xFFFFFFFC,
		0xFFFFFFFD,
		0xFFFFFFFE,
		0xFFFFFFFF,
		MaxPacketSize - 4, // 略超协议硬上限（256MB - 4）
		MaxPacketSize + 100,
	}

	for _, remaining := range overflowValues {
		remaining := remaining
		t.Run(fmt.Sprintf("remaining=%x", remaining), func(t *testing.T) {
			// 仅提供 5 字节固定头，包体留空：修复后必须在校验阶段就拒绝
			var header [5]byte
			header[0] = byte(PUBLISH)
			binary.BigEndian.PutUint32(header[1:], remaining)

			_, err := ReadPacket(bytes.NewReader(header[:]), 0)
			if err == nil {
				t.Fatalf("expected ReadPacket to reject Remaining=%x, got nil error", remaining)
			}

			var tooLargeErr *PacketTooLargeError
			if !errors.As(err, &tooLargeErr) {
				t.Fatalf("expected PacketTooLargeError for Remaining=%x, got %v", remaining, err)
			}
		})
	}
}

// TestReadPacket_MaxPacketSizeBoundary 验证 maxPacketSize 边界逻辑（总大小含 5 字节固定头）：
// 恰好等于上限时接受，小一字节上限时拒绝。
func TestReadPacket_MaxPacketSizeBoundary(t *testing.T) {
	pkt := NewConnectPacket()
	pkt.ClientID = "boundary-check"

	var buf bytes.Buffer
	if err := WritePacket(&buf, pkt, 0); err != nil {
		t.Fatalf("编码失败: %v", err)
	}

	// 恰好等于上限：应接受
	if _, err := ReadPacket(bytes.NewReader(buf.Bytes()), uint32(buf.Len())); err != nil {
		t.Fatalf("expected packet of size %d to be accepted at maxPacketSize=%d, got %v",
			buf.Len(), buf.Len(), err)
	}

	// 上限小一字节：应拒绝并返回 PacketTooLargeError
	_, err := ReadPacket(bytes.NewReader(buf.Bytes()), uint32(buf.Len()-1))
	if err == nil {
		t.Fatal("expected packet to be rejected when maxPacketSize is one byte smaller")
	}

	var tooLargeErr *PacketTooLargeError
	if !errors.As(err, &tooLargeErr) {
		t.Fatalf("expected PacketTooLargeError, got %v", err)
	}
}
