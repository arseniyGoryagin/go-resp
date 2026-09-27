package resp

const (
	VerbatimSize    = 4
	EscapeCharsSize = 2
)

const (
	StringType   = "+"
	ErrorType    = "-"
	IntegerType  = ":"
	BulkType     = "$"
	ArrayType    = "*"
	NullType     = "NULL"
	Null3Type    = "_"
	BooleanType  = "#"
	DoubleType   = ","
	BigNumber    = "("
	BlobError    = "!"
	VerbatimType = "="
	MapType      = "%"
	SetType      = "~"
	AttrType     = "|"
	PushType     = ">"
)
