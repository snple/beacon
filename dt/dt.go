package dt

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/danclive/nson-go"
)

const (
	I32       = "I32"
	I64       = "I64"
	U32       = "U32"
	U64       = "U64"
	F32       = "F32"
	F64       = "F64"
	BOOL      = "BOOL"
	STRING    = "STRING"
	NULL      = "NULL"
	BINARY    = "BINARY"
	TIMESTAMP = "TIMESTAMP"
	ID        = "ID"
	MAP       = "MAP"
)

func ParseTypeTag(typeName string) (nson.DataType, error) {
	switch typeName {
	case I32:
		return nson.DataTypeI32, nil
	case I64:
		return nson.DataTypeI64, nil
	case U32:
		return nson.DataTypeU32, nil
	case U64:
		return nson.DataTypeU64, nil
	case F32:
		return nson.DataTypeF32, nil
	case F64:
		return nson.DataTypeF64, nil
	case BOOL:
		return nson.DataTypeBOOL, nil
	case STRING:
		return nson.DataTypeSTRING, nil
	case NULL:
		return nson.DataTypeNULL, nil
	case BINARY:
		return nson.DataTypeBINARY, nil
	case TIMESTAMP:
		return nson.DataTypeTIMESTAMP, nil
	case ID:
		return nson.DataTypeID, nil
	case MAP:
		return nson.DataTypeMAP, nil
	default:
		return 0, fmt.Errorf("unsupported type name: %s", typeName)
	}
}

func ValidateType(typeName string) bool {
	switch typeName {
	case I32, I64, U32, U64, F32, F64, BOOL, STRING, NULL, BINARY, TIMESTAMP, ID, MAP:
		return true
	default:
		return false
	}
}

func ValidateValue(value, typeName string) bool {
	tag, err := ParseTypeTag(typeName)
	if err != nil {
		return false
	}

	_, err = DecodeNsonValue(value, tag)
	if err != nil {
		return false
	}

	return true
}

func EncodeNsonValue(value nson.Value) (string, error) {
	v := ""

	if value == nil {
		return v, errors.New("value is nil")
	}

	switch value.DataType() {
	case nson.DataTypeI32:
		v = fmt.Sprintf("%d", int32(value.(nson.I32)))
	case nson.DataTypeI64:
		v = fmt.Sprintf("%d", int64(value.(nson.I64)))
	case nson.DataTypeU32:
		v = fmt.Sprintf("%d", uint32(value.(nson.U32)))
	case nson.DataTypeU64:
		v = fmt.Sprintf("%d", uint64(value.(nson.U64)))
	case nson.DataTypeF32:
		v = fmt.Sprintf("%f", float32(value.(nson.F32)))
	case nson.DataTypeF64:
		v = fmt.Sprintf("%f", float64(value.(nson.F64)))
	case nson.DataTypeBOOL:
		v = fmt.Sprintf("%t", bool(value.(nson.Bool)))
	case nson.DataTypeSTRING:
		v = string(value.(nson.String))
	case nson.DataTypeNULL:
		v = ""
	case nson.DataTypeBINARY:
		v = value.(nson.Binary).Hex()
	case nson.DataTypeTIMESTAMP:
		v = fmt.Sprintf("%d", value.(nson.Timestamp))
	case nson.DataTypeID:
		v = value.(nson.Id).Hex()
	default:
		return v, fmt.Errorf("unsupported value type: %v", value.DataType())
	}

	return v, nil
}

func DecodeNsonValue(value string, tag nson.DataType) (nson.Value, error) {
	var nsonValue nson.Value

	switch tag {
	case nson.DataTypeI32:
		value, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return nsonValue, err
		}

		nsonValue = nson.I32(value)
	case nson.DataTypeI64:
		value, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nsonValue, err
		}

		nsonValue = nson.I64(value)
	case nson.DataTypeU32:
		value, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return nsonValue, err
		}

		nsonValue = nson.U32(value)
	case nson.DataTypeU64:
		value, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nsonValue, err
		}

		nsonValue = nson.U64(value)
	case nson.DataTypeF32:
		value, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return nsonValue, err
		}

		nsonValue = nson.F32(value)
	case nson.DataTypeF64:
		value, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nsonValue, err
		}

		nsonValue = nson.F64(value)
	case nson.DataTypeBOOL:
		if value == "true" || value == "1" {
			nsonValue = nson.Bool(true)
		} else {
			nsonValue = nson.Bool(false)
		}
	case nson.DataTypeSTRING:
		nsonValue = nson.String(value)
	case nson.DataTypeNULL:
		nsonValue = nson.Null{}
	case nson.DataTypeBINARY:
		binary, err := nson.BinaryFromHex(value)
		if err != nil {
			return nsonValue, err
		}
		nsonValue = nson.Binary(binary)
	case nson.DataTypeTIMESTAMP:
		value, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nsonValue, err
		}
		nsonValue = nson.Timestamp(value)
	case nson.DataTypeID:
		id, err := nson.IdFromHex(value)
		if err != nil {
			return nsonValue, err
		}
		nsonValue = nson.Id(id)
	default:
		return nsonValue, fmt.Errorf("unsupported value type: %v", tag)
	}

	return nsonValue, nil
}
