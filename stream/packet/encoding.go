package packet

import (
	"encoding/binary"
	"fmt"
	"io"
)

// EncodeUint8 编码 uint8
func EncodeUint8(w io.Writer, v uint8) error {
	_, err := w.Write([]byte{v})
	return err
}

// DecodeUint8 解码 uint8
func DecodeUint8(r io.Reader) (uint8, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return b[0], nil
}

// EncodeUint16 编码 uint16
func EncodeUint16(w io.Writer, v uint16) error {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], v)
	_, err := w.Write(b[:])
	return err
}

// DecodeUint16 解码 uint16
func DecodeUint16(r io.Reader) (uint16, error) {
	var b [2]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(b[:]), nil
}

// EncodeUint32 编码 uint32
func EncodeUint32(w io.Writer, v uint32) error {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	_, err := w.Write(b[:])
	return err
}

// DecodeUint32 解码 uint32
func DecodeUint32(r io.Reader) (uint32, error) {
	var b [4]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(b[:]), nil
}

// EncodeUint64 编码 uint64
func EncodeUint64(w io.Writer, v uint64) error {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], v)
	_, err := w.Write(b[:])
	return err
}

// DecodeUint64 解码 uint64
func DecodeUint64(r io.Reader) (uint64, error) {
	var b [8]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(b[:]), nil
}

// EncodeString 编码字符串（2字节长度 + 内容）
func EncodeString(w io.Writer, s string) error {
	if len(s) > 65535 {
		return fmt.Errorf("string too long: %d", len(s))
	}
	if err := EncodeUint16(w, uint16(len(s))); err != nil {
		return err
	}
	_, err := w.Write([]byte(s))
	return err
}

// DecodeString 解码字符串
func DecodeString(r io.Reader) (string, error) {
	length, err := DecodeUint16(r)
	if err != nil {
		return "", err
	}
	if length == 0 {
		return "", nil
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// EncodeBinary 编码二进制数据（4字节长度 + 内容）
func EncodeBinary(w io.Writer, data []byte) error {
	if len(data) > 1<<32-1 {
		return fmt.Errorf("binary data too long: %d", len(data))
	}
	if err := EncodeUint32(w, uint32(len(data))); err != nil {
		return err
	}
	if len(data) > 0 {
		_, err := w.Write(data)
		return err
	}
	return nil
}

// DecodeBinary 解码二进制数据
func DecodeBinary(r io.Reader) ([]byte, error) {
	length, err := DecodeUint32(r)
	if err != nil {
		return nil, err
	}
	if length == 0 {
		return nil, nil
	}
	if length > MaxPacketSize {
		return nil, fmt.Errorf("binary too long: %d", length)
	}
	buf := make([]byte, int(length))
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// EncodeProperties 编码属性字典
func EncodeProperties(w io.Writer, props map[string]string) error {
	if err := EncodeUint16(w, uint16(len(props))); err != nil {
		return err
	}
	for k, v := range props {
		if err := EncodeString(w, k); err != nil {
			return err
		}
		if err := EncodeString(w, v); err != nil {
			return err
		}
	}
	return nil
}

// DecodeProperties 解码属性字典
func DecodeProperties(r io.Reader) (map[string]string, error) {
	count, err := DecodeUint16(r)
	if err != nil {
		return nil, err
	}
	if count > 1024 {
		return nil, fmt.Errorf("too many properties: %d", count)
	}
	props := make(map[string]string, count)
	for i := 0; i < int(count); i++ {
		key, err := DecodeString(r)
		if err != nil {
			return nil, err
		}
		value, err := DecodeString(r)
		if err != nil {
			return nil, err
		}
		props[key] = value
	}
	return props, nil
}
