package resp

import (
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
)

func str(typ, s string) Value {
	return Value{Typ: typ, StrValue: &s}
}

func integer(i int) Value {
	return Value{Typ: IntegerType, IntValue: i}
}

func double(f float64) Value {
	return Value{Typ: DoubleType, DoubleValue: f}
}

func boolean(b bool) Value {
	return Value{Typ: BooleanType, BoolValue: b}
}

func verbatim(format, text string) Value {
	return Value{Typ: VerbatimType, Verbatim: Verbatim{Verbatim: format, StrValue: text}}
}

func aggregate(typ string, vs ...Value) Value {
	if vs == nil {
		vs = []Value{}
	}
	return Value{Typ: typ, ArrValues: &vs}
}

func mapOf(typ string, kv ...Value) Value {
	m := NewMap(len(kv) / 2)
	for i := 0; i+1 < len(kv); i += 2 {
		if err := m.Add(kv[i], kv[i+1]); err != nil {
			panic(err)
		}
	}
	return Value{Typ: typ, MapValues: m}
}

func setOf(vs ...Value) Value {
	s := NewSet(len(vs))
	for _, v := range vs {
		if err := s.Add(v); err != nil {
			panic(err)
		}
	}
	return Value{Typ: SetType, SetValues: s}
}

func parse(t *testing.T, r io.Reader) (Value, error) {
	t.Helper()
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("parseToValue panicked: %v", p)
		}
	}()
	return parseToValue(r)
}

func sameValue(a, b Value) bool {
	if a.Typ == DoubleType && b.Typ == DoubleType && math.IsNaN(a.DoubleValue) && math.IsNaN(b.DoubleValue) {
		return true
	}
	return reflect.DeepEqual(a, b)
}

type readCase struct {
	name string
	in   string
	want Value
}

var readCases = []readCase{
	// Simple string
	{"simple string", "+OK\r\n", str(StringType, "OK")},
	{"simple string empty", "+\r\n", str(StringType, "")},
	{"simple string with spaces", "+hello world\r\n", str(StringType, "hello world")},

	// Simple error
	{"simple error", "-ERR unknown command 'foo'\r\n", str(ErrorType, "ERR unknown command 'foo'")},
	{"simple error wrongtype", "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n",
		str(ErrorType, "WRONGTYPE Operation against a key holding the wrong kind of value")},

	// Integer
	{"integer zero", ":0\r\n", integer(0)},
	{"integer positive", ":1000\r\n", integer(1000)},
	{"integer negative", ":-1000\r\n", integer(-1000)},
	{"integer explicit plus", ":+5\r\n", integer(5)},

	// Bulk string
	{"bulk string", "$5\r\nhello\r\n", str(BulkType, "hello")},
	{"bulk string empty", "$0\r\n\r\n", str(BulkType, "")},
	{"bulk string binary safe", "$4\r\na\r\nb\r\n", str(BulkType, "a\r\nb")},
	{"bulk string null (RESP2)", "$-1\r\n", Value{Typ: BulkType}},

	// Array
	{"array empty", "*0\r\n", aggregate(ArrayType)},
	{"array of bulk strings", "*2\r\n$5\r\nhello\r\n$5\r\nworld\r\n",
		aggregate(ArrayType, str(BulkType, "hello"), str(BulkType, "world"))},
	{"array of integers", "*3\r\n:1\r\n:2\r\n:3\r\n",
		aggregate(ArrayType, integer(1), integer(2), integer(3))},
	{"array mixed", "*5\r\n:1\r\n:2\r\n:3\r\n:4\r\n$5\r\nhello\r\n",
		aggregate(ArrayType, integer(1), integer(2), integer(3), integer(4), str(BulkType, "hello"))},
	{"array nested", "*2\r\n*3\r\n:1\r\n:2\r\n:3\r\n*2\r\n+Hello\r\n-World\r\n",
		aggregate(ArrayType,
			aggregate(ArrayType, integer(1), integer(2), integer(3)),
			aggregate(ArrayType, str(StringType, "Hello"), str(ErrorType, "World")))},
	{"array with null element", "*3\r\n$5\r\nhello\r\n$-1\r\n$5\r\nworld\r\n",
		aggregate(ArrayType, str(BulkType, "hello"), Value{Typ: BulkType}, str(BulkType, "world"))},
	{"array null (RESP2)", "*-1\r\n", Value{Typ: ArrayType}},

	// Null
	{"null", "_\r\n", Value{Typ: Null3Type}},

	// Boolean
	{"boolean true", "#t\r\n", boolean(true)},
	{"boolean false", "#f\r\n", boolean(false)},

	// Double
	{"double", ",1.23\r\n", double(1.23)},
	{"double integral only", ",10\r\n", double(10)},
	{"double negative", ",-1.5\r\n", double(-1.5)},
	{"double exponent", ",1.5e3\r\n", double(1500)},
	{"double exponent upper negative", ",1.5E-3\r\n", double(0.0015)},
	{"double inf", ",inf\r\n", double(math.Inf(1))},
	{"double -inf", ",-inf\r\n", double(math.Inf(-1))},
	{"double nan", ",nan\r\n", double(math.NaN())},

	// Big number
	{"big number", "(3492890328409238509324850943850943825024385\r\n",
		str(BigNumber, "3492890328409238509324850943850943825024385")},
	{"big number negative", "(-3492890328409238509324850943850943825024385\r\n",
		str(BigNumber, "-3492890328409238509324850943850943825024385")},

	// Bulk error
	{"bulk error", "!21\r\nSYNTAX invalid syntax\r\n", str(BlobError, "SYNTAX invalid syntax")},
	{"bulk error empty", "!0\r\n\r\n", str(BlobError, "")},

	// Verbatim string
	{"verbatim txt", "=15\r\ntxt:Some string\r\n", verbatim("txt", "Some string")},
	{"verbatim mkd", "=11\r\nmkd:# Title\r\n", verbatim("mkd", "# Title")},
	{"verbatim empty text", "=4\r\ntxt:\r\n", verbatim("txt", "")},

	// Map
	{"map", "%2\r\n+first\r\n:1\r\n+second\r\n:2\r\n",
		mapOf(MapType, str(StringType, "first"), integer(1), str(StringType, "second"), integer(2))},
	{"map empty", "%0\r\n", mapOf(MapType)},
	{"map with aggregate value", "%1\r\n+list\r\n*2\r\n:1\r\n:2\r\n",
		mapOf(MapType, str(StringType, "list"), aggregate(ArrayType, integer(1), integer(2)))},
	{"map with null key", "%1\r\n_\r\n:1\r\n", mapOf(MapType, Value{Typ: Null3Type}, integer(1))},
	{"map with double nan key", "%1\r\n,nan\r\n:1\r\n", mapOf(MapType, double(math.NaN()), integer(1))},

	// Attribute
	{"attribute", "|1\r\n+key-popularity\r\n%2\r\n$1\r\na\r\n,0.1923\r\n$1\r\nb\r\n,0.0012\r\n",
		mapOf(AttrType, str(StringType, "key-popularity"),
			mapOf(MapType, str(BulkType, "a"), double(0.1923), str(BulkType, "b"), double(0.0012)))},

	// Set
	{"set", "~5\r\n+orange\r\n+apple\r\n#t\r\n:100\r\n:999\r\n",
		setOf(str(StringType, "orange"), str(StringType, "apple"), boolean(true), integer(100), integer(999))},
	{"set empty", "~0\r\n", setOf()},
	{"set with null", "~1\r\n_\r\n", setOf(Value{Typ: Null3Type})},

	// Push
	{"push", ">4\r\n+pubsub\r\n+message\r\n+somechannel\r\n+this is the message\r\n",
		aggregate(PushType, str(StringType, "pubsub"), str(StringType, "message"), str(StringType, "somechannel"),
			str(StringType, "this is the message"))},
}

func TestParseToValue(t *testing.T) {
	for _, tc := range readCases {
		t.Run(tc.name, func(t *testing.T) {
			r := strings.NewReader(tc.in)
			got, err := parse(t, r)
			if err != nil {
				t.Fatalf("parseToValue(%q) error: %v", tc.in, err)
			}
			if !sameValue(got, tc.want) {
				t.Errorf("parseToValue(%q)\n got: %+v\nwant: %+v", tc.in, got, tc.want)
			}
			if r.Len() != 0 {
				t.Errorf("parseToValue(%q) left %d unread bytes", tc.in, r.Len())
			}
		})
	}
}

func TestParseToValueStream(t *testing.T) {
	in := "+OK\r\n" + "_\r\n" + "$5\r\nhello\r\n" + "*2\r\n:1\r\n$-1\r\n" + "=15\r\ntxt:Some string\r\n" + "#t\r\n"
	want := []Value{
		str(StringType, "OK"),
		{Typ: Null3Type},
		str(BulkType, "hello"),
		aggregate(ArrayType, integer(1), Value{Typ: BulkType}),
		verbatim("txt", "Some string"),
		boolean(true),
	}

	r := strings.NewReader(in)
	for i, w := range want {
		got, err := parse(t, r)
		if err != nil {
			t.Fatalf("value %d: error: %v", i, err)
		}
		if !sameValue(got, w) {
			t.Fatalf("value %d:\n got: %+v\nwant: %+v", i, got, w)
		}
	}

	if _, err := parse(t, r); err != io.EOF {
		t.Errorf("after the last value: got error %v, want io.EOF", err)
	}
}

func TestParseToValueAttributeBeforeReply(t *testing.T) {
	in := "|1\r\n+key-popularity\r\n%2\r\n$1\r\na\r\n,0.1923\r\n$1\r\nb\r\n,0.0012\r\n" +
		"*2\r\n:2039123\r\n:9543892\r\n"
	r := strings.NewReader(in)

	attr, err := parse(t, r)
	if err != nil {
		t.Fatalf("attribute: error: %v", err)
	}
	if attr.Typ != AttrType {
		t.Fatalf("attribute: got Typ %q, want %q", attr.Typ, AttrType)
	}

	reply, err := parse(t, r)
	if err != nil {
		t.Fatalf("reply: error: %v", err)
	}
	want := aggregate(ArrayType, integer(2039123), integer(9543892))
	if !sameValue(reply, want) {
		t.Errorf("reply:\n got: %+v\nwant: %+v", reply, want)
	}
}

func TestParseToValueStreamed(t *testing.T) {
	cases := []readCase{
		{"streamed string", "$?\r\n;4\r\nHell\r\n;5\r\no wor\r\n;1\r\nd\r\n;0\r\n", str(BulkType, "Hello word")},
		{"streamed array", "*?\r\n:1\r\n:2\r\n:3\r\n.\r\n", aggregate(ArrayType, integer(1), integer(2), integer(3))},
		{"streamed map", "%?\r\n+a\r\n:1\r\n+b\r\n:2\r\n.\r\n",
			mapOf(MapType, str(StringType, "a"), integer(1), str(StringType, "b"), integer(2))},
		{"streamed set", "~?\r\n+a\r\n+b\r\n.\r\n", setOf(str(StringType, "a"), str(StringType, "b"))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := strings.NewReader(tc.in)
			got, err := parse(t, r)
			if err != nil {
				t.Fatalf("parseToValue(%q) error: %v", tc.in, err)
			}
			if !sameValue(got, tc.want) {
				t.Errorf("parseToValue(%q)\n got: %+v\nwant: %+v", tc.in, got, tc.want)
			}
			if r.Len() != 0 {
				t.Errorf("parseToValue(%q) left %d unread bytes", tc.in, r.Len())
			}
		})
	}
}

func TestParseToValueInvalid(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"empty input", ""},
		{"unknown type byte", "@foo\r\n"},
		{"simple string missing \\r", "+OK\n"},
		{"integer not a number", ":abc\r\n"},
		{"integer empty", ":\r\n"},
		{"boolean invalid", "#x\r\n"},
		{"double not a number", ",abc\r\n"},
		{"bulk length not a number", "$abc\r\n"},
		{"bulk length negative", "$-5\r\n"},
		{"bulk length line missing \\n", "$3\rXfoo\r\n"},
		{"bulk truncated", "$5\r\nhel"},
		{"bulk wrong terminator", "$3\r\nfooXX"},
		{"array length negative", "*-3\r\n"},
		{"array truncated", "*2\r\n:1\r\n"},
		{"map truncated", "%2\r\n+a\r\n:1\r\n+b\r\n"},
		{"set truncated", "~2\r\n+a\r\n"},
		{"null with data", "_x\r\n"},
		{"verbatim length under 4", "=2\r\nabc:\r\n"},
		{"verbatim length 0", "=0\r\nabc:\r\n"},
		{"verbatim missing colon", "=9\r\ntxtxhello\r\n"},
		{"verbatim truncated", "=15\r\ntxt:Some"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parse(t, strings.NewReader(tc.in))
			if err == nil {
				t.Errorf("parseToValue(%q): want an error, got value %+v", tc.in, got)
			}
		})
	}
}
