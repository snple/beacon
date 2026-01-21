package packet

import (
	"errors"
	"fmt"
)

var (
	ErrMalformedPacket   = errors.New("malformed packet")
	ErrInvalidQoS        = errors.New("invalid QoS level")
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
