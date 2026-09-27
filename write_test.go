package resp

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

func write(t *testing.T, v Value) (string, error) {
	t.Helper()
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("parseToString panicked: %v", p)
		}
	}()
	var w Writer
	return w.parseToString(v)
}

func TestParseToString(t *testing.T) {
	cases := []struct {
		name  string
		value Value
		want  string
	}{
		// Simple string
		{"simple string", str(StringType, "OK"), "+OK\r\n"},
		{"simple string empty", str(StringType, ""), "+\r\n"},

		// Simple error
		{"simple error", str(ErrorType, "ERR unknown command 'foo'"), "-ERR unknown command 'foo'\r\n"},

		// Integer
		{"integer zero", integer(0), ":0\r\n"},
		{"integer positive", integer(1000), ":1000\r\n"},
		{"integer negative", integer(-1000), ":-1000\r\n"},

		// Bulk string
		{"bulk string", str(BulkType, "hello"), "$5\r\nhello\r\n"},
		{"bulk string empty", str(BulkType, ""), "$0\r\n\r\n"},
		{"bulk string binary safe", str(BulkType, "a\r\nb"), "$4\r\na\r\nb\r\n"},
		{"bulk string null (RESP2)", Value{Typ: BulkType}, "$-1\r\n"},
		{"NULL type", Value{Typ: NullType}, "$-1\r\n"},

		// Array
		{"array empty", aggregate(ArrayType), "*0\r\n"},
		{"array of integers", aggregate(ArrayType, integer(1), integer(2), integer(3)), "*3\r\n:1\r\n:2\r\n:3\r\n"},
		{"array nested",
			aggregate(ArrayType,
				aggregate(ArrayType, integer(1), integer(2), integer(3)),
				aggregate(ArrayType, str(StringType, "Hello"), str(ErrorType, "World"))),
			"*2\r\n*3\r\n:1\r\n:2\r\n:3\r\n*2\r\n+Hello\r\n-World\r\n"},
		{"array with null element",
			aggregate(ArrayType, str(BulkType, "hello"), Value{Typ: BulkType}, str(BulkType, "world")),
			"*3\r\n$5\r\nhello\r\n$-1\r\n$5\r\nworld\r\n"},
		{"array null (RESP2)", Value{Typ: ArrayType}, "*-1\r\n"},

		// Null
		{"null", Value{Typ: Null3Type}, "_\r\n"},

		// Boolean
		{"boolean true", boolean(true), "#t\r\n"},
		{"boolean false", boolean(false), "#f\r\n"},

		// Double
		{"double", double(1.23), ",1.23\r\n"},
		{"double integral", double(10), ",10\r\n"},
		{"double zero", double(0), ",0\r\n"},
		{"double negative", double(-1.5), ",-1.5\r\n"},
		{"double exponent", double(1.5e300), ",1.5e+300\r\n"},
		{"double inf", double(math.Inf(1)), ",inf\r\n"},
		{"double -inf", double(math.Inf(-1)), ",-inf\r\n"},
		{"double nan", double(math.NaN()), ",nan\r\n"},

		// Big number
		{"big number", str(BigNumber, "3492890328409238509324850943850943825024385"),
			"(3492890328409238509324850943850943825024385\r\n"},

		// Bulk error
		{"bulk error", str(BlobError, "SYNTAX invalid syntax"), "!21\r\nSYNTAX invalid syntax\r\n"},
		{"bulk error empty", str(BlobError, ""), "!0\r\n\r\n"},

		// Verbatim string
		{"verbatim txt", verbatim("txt", "Some string"), "=15\r\ntxt:Some string\r\n"},
		{"verbatim empty text", verbatim("txt", ""), "=4\r\ntxt:\r\n"},

		// Map
		{"map", mapOf(MapType, str(StringType, "first"), integer(1)), "%1\r\n+first\r\n:1\r\n"},
		{"map empty", mapOf(MapType), "%0\r\n"},
		{"map with null key", mapOf(MapType, Value{Typ: Null3Type}, integer(1)), "%1\r\n_\r\n:1\r\n"},

		// Attribute
		{"attribute", mapOf(AttrType, str(StringType, "ttl"), integer(3600)), "|1\r\n+ttl\r\n:3600\r\n"},

		// Set
		{"set", setOf(str(StringType, "orange")), "~1\r\n+orange\r\n"},
		{"set empty", setOf(), "~0\r\n"},

		// Push
		{"push",
			aggregate(PushType, str(StringType, "pubsub"), str(StringType, "message"), str(StringType, "somechannel"),
				str(StringType, "this is the message")),
			">4\r\n+pubsub\r\n+message\r\n+somechannel\r\n+this is the message\r\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := write(t, tc.value)
			if err != nil {
				t.Fatalf("parseToString error: %v", err)
			}
			if got != tc.want {
				t.Errorf("parseToString\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestParseToStringUnordered(t *testing.T) {
	cases := []struct {
		name   string
		value  Value
		header string
	}{
		{"map", mapOf(MapType, str(StringType, "first"), integer(1), str(StringType, "second"), integer(2)), "%2\r\n"},
		{"attribute", mapOf(AttrType, str(StringType, "a"), integer(1), str(StringType, "b"), integer(2)), "|2\r\n"},
		{"set", setOf(str(StringType, "orange"), str(StringType, "apple"), boolean(true), integer(100)), "~4\r\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := write(t, tc.value)
			if err != nil {
				t.Fatalf("parseToString error: %v", err)
			}
			if !strings.HasPrefix(got, tc.header) {
				t.Fatalf("parseToString = %q, want it to start with %q", got, tc.header)
			}
			back, err := parse(t, strings.NewReader(got))
			if err != nil {
				t.Fatalf("reading back %q: error: %v", got, err)
			}
			if !sameValue(back, tc.value) {
				t.Errorf("reading back %q\n got: %+v\nwant: %+v", got, back, tc.value)
			}
		})
	}
}

func TestParseToStringInvalid(t *testing.T) {
	cases := []struct {
		name  string
		value Value
	}{
		{"empty type", Value{}},
		{"unknown type", Value{Typ: "@"}},
		{"simple string nil", Value{Typ: StringType}},
		{"simple error nil", Value{Typ: ErrorType}},
		{"big number nil", Value{Typ: BigNumber}},
		{"bulk error nil", Value{Typ: BlobError}},
		{"push nil", Value{Typ: PushType}},
		{"map nil", Value{Typ: MapType}},
		{"attribute nil", Value{Typ: AttrType}},
		{"set nil", Value{Typ: SetType}},
		{"simple string with \\r\\n", str(StringType, "a\r\nb")},
		{"simple error with \\n", str(ErrorType, "ERR a\nb")},
		{"verbatim format not 3 bytes", verbatim("text", "hello")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := write(t, tc.value)
			if err == nil {
				t.Errorf("parseToString(%+v): want an error, got %q", tc.value, got)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range readCases {
		if (tc.want.Typ == MapType || tc.want.Typ == AttrType) && tc.want.MapValues.Size() > 1 ||
			tc.want.Typ == SetType && tc.want.SetValues.Size() > 1 {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			v, err := parse(t, strings.NewReader(tc.in))
			if err != nil {
				t.Fatalf("parseToValue(%q) error: %v", tc.in, err)
			}
			got, err := write(t, v)
			if err != nil {
				t.Fatalf("parseToString error: %v", err)
			}
			if normalizeDouble(got) != normalizeDouble(tc.in) {
				t.Errorf("round trip\n got: %q\nwant: %q", got, tc.in)
			}
		})
	}
}

func normalizeDouble(s string) string {
	switch s {
	case ",1.5e3\r\n":
		return ",1500\r\n"
	case ",1.5E-3\r\n":
		return ",0.0015\r\n"
	case ":+5\r\n":
		return ":5\r\n"
	}
	return s
}

func TestWriterWrite(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.Write(aggregate(ArrayType, str(BulkType, "GET"), str(BulkType, "key"))); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	want := "*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n"
	if buf.String() != want {
		t.Errorf("Write wrote %q, want %q", buf.String(), want)
	}
}
