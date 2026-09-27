package resp

import (
	"bufio"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
)

type Writer struct {
	writer *bufio.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{writer: bufio.NewWriter(w)}
}

func (w *Writer) Write(value Value) error {
	res, err := w.parseToString(value)
	if err != nil {
		return err
	}
	_, err = w.writer.Write([]byte(res))
	if err != nil {
		return err
	}

	return w.writer.Flush()
}

func (w *Writer) parseToString(value Value) (message string, err error) {

	switch value.Typ {
	case NullType:
		return w.writeNull(), nil
	case StringType:
		return w.writeString(*value.StrValue), nil
	case ErrorType:
		return w.writeError(*value.StrValue), nil
	case IntegerType:
		return w.writeInt(value.IntValue), nil
	case BulkType:

		if value.StrValue == nil {
			return BulkType + "-1" + "\r\n", nil
		}

		return w.writeBulk(*value.StrValue), nil
	case BigNumber:
		return w.writeBigNumber(*value.StrValue), nil
	case BooleanType:
		return w.writeBool(value.BoolValue), nil
	case BlobError:
		return w.writeBlobError(*value.StrValue), nil
	case Null3Type:
		return w.writeNull3(), nil
	case DoubleType:
		return w.writeDouble(value.DoubleValue), nil
	case VerbatimType:
		return w.writeVerbatim(value.Verbatim), nil
	case ArrayType:

		if value.ArrValues == nil {
			return ArrayType + "-1" + "\r\n", nil
		}

		res, err := w.arrValuesToString(*value.ArrValues)
		if err != nil {
			return "", err
		}

		return ArrayType + strconv.Itoa(len(*value.ArrValues)) + "\r\n" + res, nil
	case PushType:
		res, err := w.arrValuesToString(*value.ArrValues)
		if err != nil {
			return "", err
		}

		return PushType + strconv.Itoa(len(*value.ArrValues)) + "\r\n" + res, nil
	case MapType, AttrType:
		var res strings.Builder

		for k, v := range value.MapValues.Values() {

			keyAsVal, err := parseToValue(strings.NewReader(k))
			if err != nil {
				return "", err
			}

			keyStr, err := w.parseToString(keyAsVal)
			if err != nil {
				return "", err
			}
			res.WriteString(keyStr)

			val, err := w.parseToString(v)
			if err != nil {
				return "", err
			}
			res.WriteString(val)
		}

		if value.Typ == MapType {
			return MapType + strconv.Itoa(value.MapValues.Size()) + "\r\n" + res.String(), nil
		}
		return AttrType + strconv.Itoa(value.MapValues.Size()) + "\r\n" + res.String(), nil
	case SetType:
		var res strings.Builder

		for _, v := range value.SetValues.Values() {
			val, err := w.parseToString(v)
			if err != nil {
				return "", err
			}
			res.WriteString(val)
		}
		return SetType + strconv.Itoa(value.SetValues.Size()) + "\r\n" + res.String(), nil
	default:
		return "", errors.New("unknown_value")
	}
}

func (w *Writer) arrValuesToString(values []Value) (string, error) {
	var res strings.Builder

	size := len(values)

	for i := range size {
		str, err := w.parseToString(values[i])
		if err != nil {
			return "", err
		}
		res.WriteString(str)
	}
	return res.String(), nil
}

func (w *Writer) writeString(val string) string {
	return StringType + val + "\r\n"
}

func (w *Writer) writeNull() string {
	return BulkType + "-1\r\n"
}

func (w *Writer) writeInt(val int) string {
	return IntegerType + strconv.Itoa(val) + "\r\n"
}

func (w *Writer) writeDouble(val float64) string {

	if math.IsInf(val, 1) {
		return DoubleType + "inf" + "\r\n"
	} else if math.IsInf(val, -1) {
		return DoubleType + "-inf" + "\r\n"
	} else if math.IsNaN(val) {
		return DoubleType + "nan" + "\r\n"
	}

	return DoubleType + strconv.FormatFloat(val, 'g', -1, 64) + "\r\n"
}

func (w *Writer) writeVerbatim(val Verbatim) string {
	return VerbatimType + strconv.Itoa(len(val.StrValue)+4) + "\r\n" + val.Verbatim + ":" + val.StrValue + "\r\n"
}

func (w *Writer) writeError(val string) string {
	return ErrorType + val + "\r\n"
}

func (w *Writer) writeBigNumber(val string) string {
	return BigNumber + val + "\r\n"
}

func (w *Writer) writeBool(val bool) string {
	if val {
		return BooleanType + "t\r\n"
	}
	return BooleanType + "f\r\n"
}

func (w *Writer) writeNull3() string {
	return Null3Type + "\r\n"
}

func (w *Writer) writeBlobError(val string) string {
	return BlobError + strconv.Itoa(len(val)) + "\r\n" + val + "\r\n"
}

func (w *Writer) writeBulk(val string) string {
	size := len(val)
	return BulkType + strconv.Itoa(size) + "\r\n" + val + "\r\n"
}
