package resp

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
)

type header struct {
	typ  string
	size int
}

type Reader struct {
	reader *bufio.Reader
}

func NewReader(r io.Reader) *Reader {
	return &Reader{reader: bufio.NewReader(r)}
}

func (r *Reader) Read() (Value, error) {
	return parseToValue(r.reader)
}

func parseToValue(reader io.Reader) (value Value, err error) {

	h, err := readHeader(reader)
	if err != nil {
		return Value{}, err
	}

	if h.size < 0 && h.size != -1 {
		return Value{}, errors.New("invalid input")
	}

	value.Typ = h.typ

	switch h.typ {
	case NullType:
		return value, nil
	case Null3Type:
		return value, nil
	case StringType, BigNumber:
		res, err := readString(reader)
		if err != nil {
			return Value{}, err
		}

		value.StrValue = &res

		return value, nil
	case BooleanType:
		value.BoolValue, err = readBoolean(reader)
		if err != nil {
			return Value{}, err
		}
		return value, nil
	case DoubleType:
		value.DoubleValue, err = readDouble(reader)
		if err != nil {
			return Value{}, err
		}

		return value, nil
	case ErrorType:
		res, err := readString(reader)
		if err != nil {
			return Value{}, err
		}

		value.StrValue = &res

		return value, nil
	case IntegerType:
		value.IntValue, err = readInt(reader)
		if err != nil {
			return Value{}, err
		}

		return value, nil
	case BulkType, BlobError:

		if h.size == -1 {
			return value, nil
		}

		res, err := readBulk(reader, h.size)
		if err != nil {
			return Value{}, err
		}

		value.StrValue = &res

		return value, nil
	case VerbatimType:

		if h.size == -1 {
			return value, nil
		}

		value.Verbatim, err = readVerbatim(reader, h.size)
		if err != nil {
			return Value{}, err
		}

		return value, nil
	case ArrayType, PushType:

		if h.size == -1 {
			return value, nil
		}

		values := make([]Value, h.size)
		for i := 0; i < h.size; i++ {

			t, err := parseToValue(reader)
			if err != nil {
				return Value{}, err
			}

			values[i] = t
		}

		value.ArrValues = &values

		return value, nil
	case MapType, AttrType:

		if h.size == -1 {
			return value, nil
		}

		valMap := NewMap(h.size)
		for i := 0; i < h.size; i++ {

			key, err := parseToValue(reader)
			if err != nil {
				return Value{}, err
			}

			val, err := parseToValue(reader)
			if err != nil {
				return Value{}, err
			}

			err = valMap.Add(key, val)
			if err != nil {
				return Value{}, err
			}
		}

		value.MapValues = valMap
		return value, nil
	case SetType:

		if h.size == -1 {
			return value, nil
		}

		valSet := NewSet(h.size)
		for i := 0; i < h.size; i++ {

			val, err := parseToValue(reader)
			if err != nil {
				return Value{}, err
			}

			err = valSet.Add(val)
			if err != nil {
				return Value{}, err
			}
		}

		value.SetValues = valSet
		return value, nil
	default:
		return Value{}, errors.New("unknown_value")
	}
}

func readHeader(reader io.Reader) (header, error) {

	h := header{}

	typ, err := readByteString(reader)
	if err != nil {
		return header{}, err
	}

	h.typ = typ

	if h.typ == ArrayType ||
		h.typ == BulkType ||
		h.typ == BlobError ||
		h.typ == VerbatimType ||
		h.typ == SetType ||
		h.typ == PushType ||
		h.typ == MapType ||
		h.typ == AttrType {

		var intStrVal strings.Builder

		for {
			val, err := readByteString(reader)
			if err != nil {
				return header{}, err
			}

			if val == "\r" {
				break
			}

			intStrVal.WriteString(val)
		}

		size, err := strconv.Atoi(string(intStrVal.String()))
		if err != nil {
			return header{}, err
		}

		h.size = size

		_, err = readByteString(reader)
		if err != nil {
			return header{}, err
		}

	} else if h.typ == Null3Type {
		c, err := readByteString(reader)
		if err != nil {
			return header{}, err
		}

		if c != "\r" {
			return header{}, errors.New("invalid null input")
		}

		c2, err := readByteString(reader)
		if err != nil {
			return header{}, err
		}

		if c2 != "\n" {
			return header{}, errors.New("invalid null input")
		}

	}

	return h, nil
}

func readByteString(reader io.Reader) (string, error) {
	b, err := readByte(reader)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func readByte(reader io.Reader) (byte, error) {
	buf := make([]byte, 1)
	_, err := io.ReadFull(reader, buf)
	if err != nil {
		return 0, err
	}
	return buf[0], nil
}

func readInt(reader io.Reader) (int, error) {
	numStr, err := readString(reader)
	if err != nil {
		return 0, err
	}
	i, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, err
	}
	return i, nil
}

func readDouble(reader io.Reader) (float64, error) {
	numStr, err := readString(reader)
	if err != nil {
		return 0, err
	}
	i, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, err
	}
	return i, nil
}

func readString(reader io.Reader) (string, error) {
	var res strings.Builder

	for {
		val, err := readByteString(reader)
		if err != nil {
			return "", err
		}

		res.WriteString(val)

		if val == "\n" {
			break
		}
	}

	return strings.TrimSuffix(res.String(), "\r\n"), nil
}

func readBoolean(reader io.Reader) (bool, error) {
	res, err := readString(reader)
	if err != nil {
		return false, err
	}

	if res == "t" {
		return true, nil
	} else if res == "f" {
		return false, nil
	}

	return false, errors.New("unparsable bool value")
}

func readBulk(reader io.Reader, size int) (string, error) {

	buf := make([]byte, size+EscapeCharsSize)
	_, err := io.ReadFull(reader, buf)
	if err != nil {
		return "", err
	}

	return string(buf[:size]), nil
}

func readVerbatim(reader io.Reader, size int) (Verbatim, error) {

	verbatim := make([]byte, VerbatimSize-1)

	for i := range VerbatimSize - 1 {
		c, err := readByte(reader)
		if err != nil {
			return Verbatim{}, err
		}

		if string(c) == ":" {
			return Verbatim{}, errors.New("verbatim should be 3 bytes long")
		}

		verbatim[i] = c
	}

	c, err := readByte(reader)
	if err != nil {
		return Verbatim{}, err
	}
	if string(c) != ":" {
		return Verbatim{}, errors.New("invalid verbatim input")
	}

	strLength := size - VerbatimSize

	if strLength < 0 {
		return Verbatim{}, errors.New("invalid verbatim length")
	}

	buf := make([]byte, strLength+EscapeCharsSize)
	_, err = io.ReadFull(reader, buf)
	if err != nil {
		return Verbatim{}, err
	}

	return Verbatim{
		Verbatim: string(verbatim[:3]),
		StrValue: string(buf[:strLength]),
	}, nil
}
