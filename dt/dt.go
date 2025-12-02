package dt

import (
	"bytes"
	"fmt"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/pb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ============================================================================
// 类型常量 - 避免使用字符串类型名
// ============================================================================

const (
	TypeBool      = uint32(nson.DataTypeBOOL)
	TypeNull      = uint32(nson.DataTypeNULL)
	TypeI8        = uint32(nson.DataTypeI8)
	TypeI16       = uint32(nson.DataTypeI16)
	TypeI32       = uint32(nson.DataTypeI32)
	TypeI64       = uint32(nson.DataTypeI64)
	TypeU8        = uint32(nson.DataTypeU8)
	TypeU16       = uint32(nson.DataTypeU16)
	TypeU32       = uint32(nson.DataTypeU32)
	TypeU64       = uint32(nson.DataTypeU64)
	TypeF32       = uint32(nson.DataTypeF32)
	TypeF64       = uint32(nson.DataTypeF64)
	TypeString    = uint32(nson.DataTypeSTRING)
	TypeBinary    = uint32(nson.DataTypeBINARY)
	TypeArray     = uint32(nson.DataTypeARRAY)
	TypeMap       = uint32(nson.DataTypeMAP)
	TypeTimestamp = uint32(nson.DataTypeTIMESTAMP)
	TypeId        = uint32(nson.DataTypeID)
)

// ============================================================================
// 辅助函数 - 创建 pb.NsonValue
// ============================================================================

// 这个文件提供便捷的辅助函数，用于创建 pb.NsonValue

// NewBoolValue 创建布尔值
func NewBoolValue(v bool) *pb.NsonValue {
	return &pb.NsonValue{
		Type:  uint32(nson.DataTypeBOOL),
		Value: &pb.NsonValue_BoolValue{BoolValue: v},
	}
}

// NewNullValue 创建空值
func NewNullValue() *pb.NsonValue {
	return &pb.NsonValue{
		Type:  uint32(nson.DataTypeNULL),
		Value: &pb.NsonValue_NullValue{NullValue: structpb.NullValue_NULL_VALUE},
	}
}

// NewI8Value 创建 I8 值
func NewI8Value(v int8) *pb.NsonValue {
	return &pb.NsonValue{
		Type:  uint32(nson.DataTypeI8),
		Value: &pb.NsonValue_Int32Value{Int32Value: int32(v)},
	}
}

// NewI16Value 创建 I16 值
func NewI16Value(v int16) *pb.NsonValue {
	return &pb.NsonValue{
		Type:  uint32(nson.DataTypeI16),
		Value: &pb.NsonValue_Int32Value{Int32Value: int32(v)},
	}
}

// NewI32Value 创建 I32 值
func NewI32Value(v int32) *pb.NsonValue {
	return &pb.NsonValue{
		Type:  uint32(nson.DataTypeI32),
		Value: &pb.NsonValue_Int32Value{Int32Value: v},
	}
}

// NewI64Value 创建 I64 值
func NewI64Value(v int64) *pb.NsonValue {
	return &pb.NsonValue{
		Type:  uint32(nson.DataTypeI64),
		Value: &pb.NsonValue_Int64Value{Int64Value: v},
	}
}

// NewU8Value 创建 U8 值
func NewU8Value(v uint8) *pb.NsonValue {
	return &pb.NsonValue{
		Type:  uint32(nson.DataTypeU8),
		Value: &pb.NsonValue_Uint32Value{Uint32Value: uint32(v)},
	}
}

// NewU16Value 创建 U16 值
func NewU16Value(v uint16) *pb.NsonValue {
	return &pb.NsonValue{
		Type:  uint32(nson.DataTypeU16),
		Value: &pb.NsonValue_Uint32Value{Uint32Value: uint32(v)},
	}
}

// NewU32Value 创建 U32 值
func NewU32Value(v uint32) *pb.NsonValue {
	return &pb.NsonValue{
		Type:  uint32(nson.DataTypeU32),
		Value: &pb.NsonValue_Uint32Value{Uint32Value: v},
	}
}

// NewU64Value 创建 U64 值
func NewU64Value(v uint64) *pb.NsonValue {
	return &pb.NsonValue{
		Type:  uint32(nson.DataTypeU64),
		Value: &pb.NsonValue_Uint64Value{Uint64Value: v},
	}
}

// NewF32Value 创建 F32 值
func NewF32Value(v float32) *pb.NsonValue {
	return &pb.NsonValue{
		Type:  uint32(nson.DataTypeF32),
		Value: &pb.NsonValue_FloatValue{FloatValue: v},
	}
}

// NewF64Value 创建 F64 值
func NewF64Value(v float64) *pb.NsonValue {
	return &pb.NsonValue{
		Type:  uint32(nson.DataTypeF64),
		Value: &pb.NsonValue_DoubleValue{DoubleValue: v},
	}
}

// NewStringValue 创建字符串值
func NewStringValue(v string) *pb.NsonValue {
	return &pb.NsonValue{
		Type:  uint32(nson.DataTypeSTRING),
		Value: &pb.NsonValue_StringValue{StringValue: v},
	}
}

// NewBinaryValue 创建二进制值
func NewBinaryValue(v []byte) *pb.NsonValue {
	return &pb.NsonValue{
		Type:  uint32(nson.DataTypeBINARY),
		Value: &pb.NsonValue_BinaryValue{BinaryValue: v},
	}
}

// NewArrayValue 创建数组值
func NewArrayValue(values ...*pb.NsonValue) *pb.NsonValue {
	return &pb.NsonValue{
		Type: uint32(nson.DataTypeARRAY),
		Value: &pb.NsonValue_ArrayValue{
			ArrayValue: &pb.NsonArray{Values: values},
		},
	}
}

// NewMapValue 创建映射值
func NewMapValue(entries map[string]*pb.NsonValue) *pb.NsonValue {
	return &pb.NsonValue{
		Type: uint32(nson.DataTypeMAP),
		Value: &pb.NsonValue_MapValue{
			MapValue: &pb.NsonMap{Entries: entries},
		},
	}
}

// NsonToProto 将 nson.Value 转换为 pb.NsonValue（递归）
func NsonToProto(v nson.Value) (*pb.NsonValue, error) {
	if v == nil {
		return nil, fmt.Errorf("nil value")
	}

	result := &pb.NsonValue{
		Type: uint32(v.DataType()),
	}

	switch val := v.(type) {
	case nson.Bool:
		result.Value = &pb.NsonValue_BoolValue{BoolValue: bool(val)}
	case nson.Null:
		result.Value = &pb.NsonValue_NullValue{NullValue: structpb.NullValue_NULL_VALUE}
	case nson.I8:
		result.Value = &pb.NsonValue_Int32Value{Int32Value: int32(val)}
	case nson.I16:
		result.Value = &pb.NsonValue_Int32Value{Int32Value: int32(val)}
	case nson.I32:
		result.Value = &pb.NsonValue_Int32Value{Int32Value: int32(val)}
	case nson.I64:
		result.Value = &pb.NsonValue_Int64Value{Int64Value: int64(val)}
	case nson.U8:
		result.Value = &pb.NsonValue_Uint32Value{Uint32Value: uint32(val)}
	case nson.U16:
		result.Value = &pb.NsonValue_Uint32Value{Uint32Value: uint32(val)}
	case nson.U32:
		result.Value = &pb.NsonValue_Uint32Value{Uint32Value: uint32(val)}
	case nson.U64:
		result.Value = &pb.NsonValue_Uint64Value{Uint64Value: uint64(val)}
	case nson.F32:
		result.Value = &pb.NsonValue_FloatValue{FloatValue: float32(val)}
	case nson.F64:
		result.Value = &pb.NsonValue_DoubleValue{DoubleValue: float64(val)}
	case nson.String:
		result.Value = &pb.NsonValue_StringValue{StringValue: string(val)}
	case nson.Binary:
		result.Value = &pb.NsonValue_BinaryValue{BinaryValue: []byte(val)}
	case nson.Array:
		// 递归转换数组元素
		pbArray := &pb.NsonArray{
			Values: make([]*pb.NsonValue, 0, len(val)),
		}
		for _, elem := range val {
			pbElem, err := NsonToProto(elem)
			if err != nil {
				return nil, fmt.Errorf("array element conversion failed: %w", err)
			}
			pbArray.Values = append(pbArray.Values, pbElem)
		}
		result.Value = &pb.NsonValue_ArrayValue{ArrayValue: pbArray}
	case nson.Map:
		// 递归转换映射条目
		pbMap := &pb.NsonMap{
			Entries: make(map[string]*pb.NsonValue, len(val)),
		}
		for key, elem := range val {
			pbElem, err := NsonToProto(elem)
			if err != nil {
				return nil, fmt.Errorf("map entry '%s' conversion failed: %w", key, err)
			}
			pbMap.Entries[key] = pbElem
		}
		result.Value = &pb.NsonValue_MapValue{MapValue: pbMap}
	case nson.Timestamp:
		t := time.UnixMilli(int64(val))
		result.Value = &pb.NsonValue_TimestampValue{TimestampValue: timestamppb.New(t)}
	case nson.Id:
		result.Value = &pb.NsonValue_IdValue{IdValue: []byte(val)}
	default:
		return nil, fmt.Errorf("unsupported nson type: %T", v)
	}

	return result, nil
}

// ProtoToNson 将 pb.NsonValue 转换为 nson.Value（递归）
// 使用 type 字段来正确转换到具体的 nson 类型
func ProtoToNson(v *pb.NsonValue) (nson.Value, error) {
	if v == nil {
		return nil, fmt.Errorf("nil proto value")
	}

	dataType := nson.DataType(v.Type)

	switch val := v.Value.(type) {
	case *pb.NsonValue_BoolValue:
		return nson.Bool(val.BoolValue), nil
	case *pb.NsonValue_NullValue:
		return nson.Null{}, nil
	case *pb.NsonValue_Int32Value:
		// 根据 type 字段判断是 I8/I16/I32
		return castInt32ToNson(val.Int32Value, dataType)
	case *pb.NsonValue_Int64Value:
		return nson.I64(val.Int64Value), nil
	case *pb.NsonValue_Uint32Value:
		// 根据 type 字段判断是 U8/U16/U32
		return castUint32ToNson(val.Uint32Value, dataType)
	case *pb.NsonValue_Uint64Value:
		return nson.U64(val.Uint64Value), nil
	case *pb.NsonValue_FloatValue:
		return nson.F32(val.FloatValue), nil
	case *pb.NsonValue_DoubleValue:
		return nson.F64(val.DoubleValue), nil
	case *pb.NsonValue_StringValue:
		return nson.String(val.StringValue), nil
	case *pb.NsonValue_BinaryValue:
		return nson.Binary(val.BinaryValue), nil
	case *pb.NsonValue_ArrayValue:
		// 递归转换数组元素
		result := make(nson.Array, 0, len(val.ArrayValue.Values))
		for i, pbElem := range val.ArrayValue.Values {
			elem, err := ProtoToNson(pbElem)
			if err != nil {
				return nil, fmt.Errorf("array element [%d] conversion failed: %w", i, err)
			}
			result = append(result, elem)
		}
		return result, nil
	case *pb.NsonValue_MapValue:
		// 递归转换映射条目
		result := make(nson.Map, len(val.MapValue.Entries))
		for key, pbElem := range val.MapValue.Entries {
			elem, err := ProtoToNson(pbElem)
			if err != nil {
				return nil, fmt.Errorf("map entry '%s' conversion failed: %w", key, err)
			}
			result[key] = elem
		}
		return result, nil
	case *pb.NsonValue_TimestampValue:
		return nson.Timestamp(val.TimestampValue.AsTime().UnixMilli()), nil
	case *pb.NsonValue_IdValue:
		return nson.Id(val.IdValue), nil
	default:
		return nil, fmt.Errorf("unsupported proto value type: %T", val)
	}
}

// castInt32ToNson 根据 DataType 将 int32 转换为对应的 nson 整数类型
func castInt32ToNson(val int32, dt nson.DataType) (nson.Value, error) {
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
func castUint32ToNson(val uint32, dt nson.DataType) (nson.Value, error) {
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

// ProtoMapToNson 将 pb.NsonMap 转换为 nson.Map
func ProtoMapToNson(pbMap *pb.NsonMap) (nson.Map, error) {
	if pbMap == nil || len(pbMap.Entries) == 0 {
		return nil, nil
	}

	result := make(nson.Map, len(pbMap.Entries))
	for key, pbValue := range pbMap.Entries {
		value, err := ProtoToNson(pbValue)
		if err != nil {
			return nil, fmt.Errorf("failed to convert map entry '%s': %w", key, err)
		}
		result[key] = value
	}
	return result, nil
}

// NsonMapToProto 将 nson.Map 转换为 pb.NsonMap
func NsonMapToProto(nsonMap nson.Map) *pb.NsonMap {
	if len(nsonMap) == 0 {
		return nil
	}

	result := &pb.NsonMap{
		Entries: make(map[string]*pb.NsonValue, len(nsonMap)),
	}
	for key, value := range nsonMap {
		pbValue, err := NsonToProto(value)
		if err != nil {
			// 如果转换失败,使用 null 值
			pbValue = &pb.NsonValue{
				Type:  uint32(nson.DataTypeNULL),
				Value: &pb.NsonValue_NullValue{NullValue: structpb.NullValue_NULL_VALUE},
			}
		}
		result.Entries[key] = pbValue
	}
	return result
}

// ValidateNsonValue 验证 pb.NsonValue 是否与指定的 DataType 兼容
// pinType 是 nson.DataType 的 uint32 表示
func ValidateNsonValue(value *pb.NsonValue, pinType uint32) bool {
	if value == nil {
		return false
	}
	// 值的类型必须与 Pin 定义的类型匹配
	return value.Type == pinType
}

// ============================================================================
// NSON 字节编解码 - 用于 Badger 存储
// ============================================================================

// EncodeNsonValue 将 pb.NsonValue 编码为 NSON 字节
func EncodeNsonValue(value *pb.NsonValue) ([]byte, error) {
	if value == nil {
		return nil, nil
	}

	// 先转换为 nson.Value
	nsonVal, err := ProtoToNson(value)
	if err != nil {
		return nil, err
	}

	// 编码为字节
	buf := new(bytes.Buffer)
	if err := nson.EncodeValue(buf, nsonVal); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// DecodeNsonValue 从 NSON 字节解码为 pb.NsonValue
func DecodeNsonValue(data []byte) (*pb.NsonValue, error) {
	if len(data) == 0 {
		return nil, nil
	}

	// 解码为 nson.Value
	buf := bytes.NewBuffer(data)
	nsonVal, err := nson.DecodeValue(buf)
	if err != nil {
		return nil, err
	}

	// 转换为 pb.NsonValue
	return NsonToProto(nsonVal)
}
