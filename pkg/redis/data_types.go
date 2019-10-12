package redis

type Componenter interface {
	RedisTypeName() string
}

type Array []Componenter

func NewArrayFromComponenterSlice(v []Componenter) *Array {
	r := Array(v)
	return &r
}

func (r *Array) RedisTypeName() string {
	return "array"
}

type Int int

func (r Int) Int() int {
	return int(r)
}

func (r *Int) RedisTypeName() string {
	return "int"
}

func NewIntFromInt(v int) *Int {
	r := Int(v)
	return &r
}

type Null uint8

func (r *Null) RedisTypeName() string {
	return "null"
}

func NewNull() *Null {
	r := Null(0)
	return &r
}

type NullString uint8

func (r *NullString) RedisTypeName() string {
	return "nullString"
}

func NewNullString() *NullString {
	r := NullString(0)
	return &r
}

type BulkString string

func (r *BulkString) RedisTypeName() string {
	return "bulkString"
}

func (r BulkString) String() string {
	return string(r)
}

func NewBulkStringFromString(v string) *BulkString {
	r := BulkString(v)
	return &r
}

type SimpleString string

func (r *SimpleString) RedisTypeName() string {
	return "simpleString"
}

func (r SimpleString) String() string {
	return string(r)
}

func NewSimpleStringFromString(v string) *SimpleString {
	r := SimpleString(v)
	return &r
}

type ErrorComp string

func (r *ErrorComp) RedisTypeName() string {
	return "error"
}

func (r ErrorComp) String() string {
	return string(r)
}

func NewErrorFromString(v string) *ErrorComp {
	r := ErrorComp(v)
	return &r
}
