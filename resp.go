package resp

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
)

const (
	STRING  = "+"
	ERROR   = "-"
	INTEGER = ":"
	BULK    = "$"
	ARRAY   = "*"
	NULL    = "NULL"
)

type Value struct {
	typ      string
	strValue string
	intValue int
	values   []Value
}
type header struct {
	typ  string
	size int
}

type Reader struct {
	reader *bufio.Reader
}

type Writer struct {
	writer *bufio.Writer
}

func NewReader(r io.Reader) *Reader {
	return &Reader{reader: bufio.NewReader(r)}
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{writer: bufio.NewWriter(w)}
}

func (r *Reader) Read() (Value, error) {
	return r.read("", 0)
}

func (w *Writer) Write(value Value) error {
	res, err := w.write(value)
	if err != nil {
		return err
	}
	_, err = w.writer.Write([]byte(res))
	if err != nil {
		return err
	}

	return w.writer.Flush()
}

func (w *Writer) write(value Value) (message string, err error) {

	switch value.typ {
	case NULL:
		return w.writeNull(), nil
	case STRING:
		return w.writeString(value.strValue), nil
	case ERROR:
		return w.writeError(value.strValue), nil
	case INTEGER:
		return w.writeInt(value.intValue), nil
	case BULK:
		return w.writeBulk(value.strValue), nil
	case ARRAY:
		var res strings.Builder

		size := len(value.values)

		for i := range size {
			str, err := w.write(value.values[i])
			if err != nil {
				return "", err
			}
			res.WriteString(str)
		}
		return "*" + strconv.Itoa(size) + "\r\n" + res.String(), nil
	default:
		return "", errors.New("unknown_value")
	}
}

func (w *Writer) writeString(val string) string {
	return "+" + val + "\r\n"
}

func (w *Writer) writeNull() string {
	return "$-1\r\n"
}

func (w *Writer) writeInt(val int) string {
	return ":" + strconv.Itoa(val) + "\r\n"
}

func (w *Writer) writeError(val string) string {
	return "-" + val + "\r\n"
}

func (w *Writer) writeBulk(val string) string {
	size := len(val)
	return "$" + strconv.Itoa(size) + "\r\n" + val + "\r\n"
}

func (r *Reader) read(typ string, size int) (value Value, err error) {

	h := header{typ: typ, size: size}
	if typ == "" {
		h, err = r.readHeader()
		if err != nil {
			return Value{}, err
		}
	}

	value.typ = h.typ

	switch h.typ {
	case STRING:
		value.strValue, err = r.readString()
		if err != nil {
			return Value{}, err
		}

		return value, nil
	case ERROR:
		value.strValue, err = r.readString()
		if err != nil {
			return Value{}, err
		}

		return value, nil
	case INTEGER:
		value.intValue, err = r.readInt()
		if err != nil {
			return Value{}, err
		}

		return value, nil
	case BULK:

		if h.size == -1 {
			return value, nil
		}

		value.strValue, err = r.readBulk(h.size)
		if err != nil {
			return Value{}, err
		}

		return value, nil
	case ARRAY:

		if h.size == -1 {
			return value, nil
		}

		values := make([]Value, h.size)
		for i := 0; i < h.size; i++ {

			t, err := r.read("", 0)
			if err != nil {
				return Value{}, err
			}

			values[i] = t
		}

		value.values = values

		return value, nil
	default:
		return Value{}, errors.New("unknown_value")
	}
}

func (r *Reader) readHeader() (header, error) {

	h := header{}

	typ, err := r.readByte()
	if err != nil {
		return header{}, err
	}

	h.typ = typ

	if h.typ == "*" || h.typ == "$" {

		var intStrVal strings.Builder

		for {
			val, err := r.readByte()
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

		_, err = r.readByte()
		if err != nil {
			return header{}, err
		}

	}

	return h, nil
}

func (r *Reader) readByte() (string, error) {
	buf := make([]byte, 1)
	_, err := io.ReadFull(r.reader, buf)
	if err != nil {
		return "", err
	}
	return string(buf[0]), nil
}

func (r *Reader) readInt() (int, error) {
	numStr, err := r.readString()
	if err != nil {
		return 0, err
	}
	i, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, err
	}
	return i, nil
}

func (r *Reader) readString() (string, error) {
	res, err := r.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(res, "\r\n"), nil
}

func (r *Reader) readBulk(size int) (string, error) {

	buf := make([]byte, size+2)
	_, err := io.ReadFull(r.reader, buf)
	if err != nil {
		return "", err
	}

	return string(buf[:size]), nil
}
