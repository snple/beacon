package packet

import (
	"encoding/binary"
	"io"

	"github.com/danclive/nson-go"
)

// FixedHeader 固定头部
type FixedHeader struct {
	Type      PacketType // 数据包类型 (8 bit)
	Remaining uint32     // 剩余长度
}

// Encode 编码固定头部
func (h *FixedHeader) Encode(w io.Writer) error {
	// 第一个字节: 类型 (8 bit)
	if _, err := w.Write([]byte{byte(h.Type)}); err != nil {
		return err
	}

	// 剩余长度 (固定4字节)
	return EncodeUint32(w, h.Remaining)
}

// Decode 解码固定头部
func (h *FixedHeader) Decode(r io.Reader) error {
	// 一次性读取固定头部的5个字节
	var buf [5]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return err
	}

	h.Type = PacketType(buf[0])
	if h.Type == RESERVED {
		return ErrInvalidPacketType
	}

	// 剩余长度 (固定4字节，大端序)
	h.Remaining = binary.BigEndian.Uint32(buf[1:])
	return nil
}

// Size 返回固定头部编码后的大小
func (h *FixedHeader) Size() int {
	return 5 // 1字节类型 + 4字节剩余长度
}

// EncodeString 编码 UTF-8 字符串
func EncodeString(w io.Writer, s string) error {
	length := uint16(len(s))
	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		return err
	}
	if length > 0 {
		_, err := w.Write([]byte(s))
		return err
	}
	return nil
}

// DecodeString 解码 UTF-8 字符串
func DecodeString(r io.Reader) (string, error) {
	var length uint16
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return "", err
	}
	if length == 0 {
		return "", nil
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return "", err
	}
	return string(data), nil
}

// EncodeBinary 编码二进制数据
func EncodeBinary(w io.Writer, data []byte) error {
	length := uint16(len(data))
	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		return err
	}
	if length > 0 {
		_, err := w.Write(data)
		return err
	}
	return nil
}

// DecodeBinary 解码二进制数据
func DecodeBinary(r io.Reader) ([]byte, error) {
	var length uint16
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	if length == 0 {
		return nil, nil
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}

// EncodeUint16 编码 uint16
func EncodeUint16(w io.Writer, v uint16) error {
	return binary.Write(w, binary.BigEndian, v)
}

// DecodeUint16 解码 uint16
func DecodeUint16(r io.Reader) (uint16, error) {
	var v uint16
	err := binary.Read(r, binary.BigEndian, &v)
	return v, err
}

// EncodeUint32 编码 uint32
func EncodeUint32(w io.Writer, v uint32) error {
	return binary.Write(w, binary.BigEndian, v)
}

// DecodeUint32 解码 uint32
func DecodeUint32(r io.Reader) (uint32, error) {
	var v uint32
	err := binary.Read(r, binary.BigEndian, &v)
	return v, err
}

// EncodeInt64 编码 int64
func EncodeInt64(w io.Writer, v int64) error {
	return binary.Write(w, binary.BigEndian, v)
}

// EncodeUint64 编码 uint64
func EncodeUint64(w io.Writer, v uint64) error {
	return binary.Write(w, binary.BigEndian, v)
}

// DecodeInt64 解码 int64
func DecodeInt64(r io.Reader) (int64, error) {
	var v int64
	err := binary.Read(r, binary.BigEndian, &v)
	return v, err
}

// DecodeUint64 解码 uint64
func DecodeUint64(r io.Reader) (uint64, error) {
	var v uint64
	err := binary.Read(r, binary.BigEndian, &v)
	return v, err
}

// StringSize 返回编码后字符串的大小
func StringSize(s string) int {
	return 2 + len(s)
}

// BinarySize 返回编码后二进制数据的大小
func BinarySize(data []byte) int {
	return 2 + len(data)
}

// EncodeId 编码 nson.Id
func EncodeId(w io.Writer, id nson.Id) error {
	_, err := w.Write(id[:])
	return err
}

// DecodeId 解码 nson.Id
func DecodeId(r io.Reader) (nson.Id, error) {
	var id nson.Id
	_, err := io.ReadFull(r, id[:])
	return id, err
}

// WriteByte 写入单个字节
func WriteByte(w io.Writer, b byte) error {
	_, err := w.Write([]byte{b})
	return err
}

// ReadByte 读取单个字节
func ReadByte(r io.Reader) (byte, error) {
	var b [1]byte
	_, err := io.ReadFull(r, b[:])
	return b[0], err
}
