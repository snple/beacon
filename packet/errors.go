package packet

import (
	"errors"
	"fmt"
)

var (
	ErrMalformedPacket   = errors.New("malformed packet")
	ErrInvalidPacketType = errors.New("invalid packet type")
)

// PacketTooLargeError 数据包过大错误
type PacketTooLargeError struct {
	Size       uint32
	MaxAllowed uint32
}

func (e *PacketTooLargeError) Error() string {
	return fmt.Sprintf("packet size %d exceeds maximum allowed %d", e.Size, e.MaxAllowed)
}

func (e *PacketTooLargeError) Is(target error) bool {
	_, ok := target.(*PacketTooLargeError)
	return ok
}

func NewPacketTooLargeError(size, maxAllowed uint32) error {
	return &PacketTooLargeError{
		Size:       size,
		MaxAllowed: maxAllowed,
	}
}

// checkUnknownFlags 拒绝对端设置的保留标志位。
func checkUnknownFlags(flags, known uint8) error {
	if unknown := flags &^ known; unknown != 0 {
		return fmt.Errorf("%w: unknown flags 0x%02x", ErrMalformedPacket, unknown)
	}
	return nil
}
