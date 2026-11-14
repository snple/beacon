package packet

import (
	"encoding/binary"
	"encoding/hex"
	"math/rand"
	"sync/atomic"
	"time"
)

// PacketIDSize 是 PacketID 的字节长度。
const PacketIDSize = 12

// PacketID 是 12 字节的消息/报文标识，布局为：
//
//	+---------------------------+-----------+-----------+
//	|      timestamp (6B)       | count(2B) | random(4B)|
//	+---------------------------+-----------+-----------+
//	  0                          6           8           12
//
// 前 6 字节是毫秒时间戳，因此 PacketID 大致按时间有序，可用于追踪/去重；
// 投递顺序仍以队列位置为准，不应依赖 ID 排序。
type PacketID [PacketIDSize]byte

var lastPacketIDCount atomic.Uint32

func init() {
	// 以启动时的时间初始化计数器，降低同毫秒内跨进程重启的碰撞概率。
	lastPacketIDCount.Store(uint32(time.Now().Nanosecond()))
}

// NewPacketID 生成新的 PacketID：毫秒时间戳 + 递增计数 + 随机数。
func NewPacketID() PacketID {
	var id PacketID

	ms := uint64(time.Now().UnixNano() / int64(time.Millisecond))
	id[0] = byte(ms >> 40)
	id[1] = byte(ms >> 32)
	id[2] = byte(ms >> 24)
	id[3] = byte(ms >> 16)
	id[4] = byte(ms >> 8)
	id[5] = byte(ms)

	binary.BigEndian.PutUint16(id[6:8], uint16(lastPacketIDCount.Add(1)%65536))
	binary.BigEndian.PutUint32(id[8:12], rand.Uint32())

	return id
}

// Hex 返回 PacketID 的小写十六进制表示（24 个字符）。
func (id PacketID) Hex() string {
	return hex.EncodeToString(id[:])
}

// IsZero 报告 PacketID 是否为零值。
func (id PacketID) IsZero() bool {
	return id == PacketID{}
}

func (id PacketID) String() string {
	return id.Hex()
}
