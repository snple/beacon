package dt

import (
	"bytes"
	"fmt"

	"github.com/danclive/nson-go"
)

// castInt32ToNson 根据 DataType 将 int32 转换为对应的 nson 整数类型
func CastInt32ToNson(val int32, dt nson.DataType) (nson.Value, error) {
	switch dt {
	case nson.DataTypeI8:
		if val < -128 || val > 127 {
			return nil, fmt.Errorf("value %d out of range for I8", val)
		}
		return nson.I8(int8(val)), nil
	case nson.DataTypeI16:
		if val < -32768 || val > 32767 {
			return nil, fmt.Errorf("value %d out of range for I16", val)
		}
		return nson.I16(int16(val)), nil
	case nson.DataTypeI32:
		return nson.I32(val), nil
	default:
		return nil, fmt.Errorf("unexpected DataType %d for int32 value", dt)
	}
}

// castUint32ToNson 根据 DataType 将 uint32 转换为对应的 nson 整数类型
func CastUint32ToNson(val uint32, dt nson.DataType) (nson.Value, error) {
	switch dt {
	case nson.DataTypeU8:
		if val > 255 {
			return nil, fmt.Errorf("value %d out of range for U8", val)
		}
		return nson.U8(uint8(val)), nil
	case nson.DataTypeU16:
		if val > 65535 {
			return nil, fmt.Errorf("value %d out of range for U16", val)
		}
		return nson.U16(uint16(val)), nil
	case nson.DataTypeU32:
		return nson.U32(val), nil
	default:
		return nil, fmt.Errorf("unexpected DataType %d for uint32 value", dt)
	}
}

// ============================================================================
// NSON 字节编解码 - 用于 Badger 存储
// ============================================================================

// EncodeNsonValue 将 pb.NsonValue 编码为 NSON 字节
func EncodeNsonValue(value nson.Value) ([]byte, error) {
	if value == nil {
		return nil, nil
	}

	// 编码为字节
	buf := new(bytes.Buffer)
	if err := nson.EncodeValue(buf, value); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// DecodeNsonValue 从 NSON 字节解码为 pb.NsonValue
func DecodeNsonValue(data []byte) (nson.Value, error) {
	if len(data) == 0 {
		return nil, nil
	}

	// 解码为 nson.Value
	buf := bytes.NewBuffer(data)
	nsonVal, err := nson.DecodeValue(buf)
	if err != nil {
		return nil, err
	}

	return nsonVal, nil
}

// EncodeNode 将 dt.Node 编码为 NSON 字节
func EncodeNode(node *Node) ([]byte, error) {
	m, err := nson.Marshal(node)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	if err := nson.EncodeMap(m, buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// DecodeNode 从 NSON 字节解码为 dt.Node
func DecodeNode(data []byte) (*Node, error) {
	buf := bytes.NewBuffer(data)
	m, err := nson.DecodeMap(buf)
	if err != nil {
		return nil, err
	}

	var node Node
	if err := nson.Unmarshal(m, &node); err != nil {
		return nil, err
	}

	return &node, nil
}
