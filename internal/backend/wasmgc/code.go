package wasmgc

import "bytes"

// Code builds one function body. Methods append an instruction and return
// the builder, so bodies read in instruction order:
//
//	c.LocalGet(0).StructGet(tInt, 0).I64Const(1).I64Add()
//
// Block, Loop and If open a structured block that End closes; Code tracks
// the nesting so an unbalanced body is reported when the module is encoded.
type Code struct {
	buf   bytes.Buffer
	depth int
}

// NewCode returns an empty body.
func NewCode() *Code { return &Code{} }

func (c *Code) op(b ...byte) *Code { c.buf.Write(b); return c }

func (c *Code) u(v uint64) *Code { c.buf.Write(uleb(v)); return c }

func (c *Code) gc(sub uint64) *Code { c.buf.WriteByte(0xFB); return c.u(sub) }

// BlockType is the result shape of a block: none, one value type, or a
// function type index.
type BlockType []byte

// NoResult is the empty block type.
var NoResult = BlockType{0x40}

// Result is a block producing one value of type t.
func Result(t ValType) BlockType { return BlockType(t) }

// Control flow.

func (c *Code) Block(bt BlockType) *Code { c.depth++; c.buf.WriteByte(0x02); c.buf.Write(bt); return c }
func (c *Code) Loop(bt BlockType) *Code  { c.depth++; c.buf.WriteByte(0x03); c.buf.Write(bt); return c }
func (c *Code) If(bt BlockType) *Code    { c.depth++; c.buf.WriteByte(0x04); c.buf.Write(bt); return c }
func (c *Code) Else() *Code              { return c.op(0x05) }
func (c *Code) End() *Code               { c.depth--; return c.op(0x0B) }
func (c *Code) Br(depth int) *Code       { return c.op(0x0C).u(uint64(depth)) }
func (c *Code) BrIf(depth int) *Code     { return c.op(0x0D).u(uint64(depth)) }
func (c *Code) Return() *Code            { return c.op(0x0F) }
func (c *Code) Unreachable() *Code       { return c.op(0x00) }
func (c *Code) Drop() *Code              { return c.op(0x1A) }
func (c *Code) Call(fn int) *Code        { return c.op(0x10).u(uint64(fn)) }
func (c *Code) CallRef(typ int) *Code    { return c.op(0x14).u(uint64(typ)) }

// Typed select (needed for reference operands).
func (c *Code) Select(t ValType) *Code { c.op(0x1C).u(1); c.buf.Write(t); return c }

// Locals and globals.

func (c *Code) LocalGet(i int) *Code  { return c.op(0x20).u(uint64(i)) }
func (c *Code) LocalSet(i int) *Code  { return c.op(0x21).u(uint64(i)) }
func (c *Code) LocalTee(i int) *Code  { return c.op(0x22).u(uint64(i)) }
func (c *Code) GlobalGet(i int) *Code { return c.op(0x23).u(uint64(i)) }
func (c *Code) GlobalSet(i int) *Code { return c.op(0x24).u(uint64(i)) }

// Constants.

func (c *Code) I32Const(v int32) *Code { c.buf.WriteByte(0x41); c.buf.Write(sleb(int64(v))); return c }
func (c *Code) I64Const(v int64) *Code { c.buf.WriteByte(0x42); c.buf.Write(sleb(v)); return c }
func (c *Code) F64Const(v float64) *Code {
	c.buf.WriteByte(0x44)
	c.buf.Write(f64bytes(v))
	return c
}

// i32 arithmetic and comparison.

func (c *Code) I32Eqz() *Code  { return c.op(0x45) }
func (c *Code) I32Eq() *Code   { return c.op(0x46) }
func (c *Code) I32Ne() *Code   { return c.op(0x47) }
func (c *Code) I32LtS() *Code  { return c.op(0x48) }
func (c *Code) I32LtU() *Code  { return c.op(0x49) }
func (c *Code) I32GtS() *Code  { return c.op(0x4A) }
func (c *Code) I32GtU() *Code  { return c.op(0x4B) }
func (c *Code) I32LeS() *Code  { return c.op(0x4C) }
func (c *Code) I32LeU() *Code  { return c.op(0x4D) }
func (c *Code) I32GeS() *Code  { return c.op(0x4E) }
func (c *Code) I32GeU() *Code  { return c.op(0x4F) }
func (c *Code) I32Add() *Code  { return c.op(0x6A) }
func (c *Code) I32Sub() *Code  { return c.op(0x6B) }
func (c *Code) I32Mul() *Code  { return c.op(0x6C) }
func (c *Code) I32DivU() *Code { return c.op(0x6E) }
func (c *Code) I32RemU() *Code { return c.op(0x70) }
func (c *Code) I32And() *Code  { return c.op(0x71) }
func (c *Code) I32Or() *Code   { return c.op(0x72) }
func (c *Code) I32Xor() *Code  { return c.op(0x73) }
func (c *Code) I32Shl() *Code  { return c.op(0x74) }
func (c *Code) I32ShrU() *Code { return c.op(0x76) }

// i64 arithmetic and comparison.

func (c *Code) I64Eqz() *Code  { return c.op(0x50) }
func (c *Code) I64Eq() *Code   { return c.op(0x51) }
func (c *Code) I64Ne() *Code   { return c.op(0x52) }
func (c *Code) I64LtS() *Code  { return c.op(0x53) }
func (c *Code) I64GtS() *Code  { return c.op(0x55) }
func (c *Code) I64LeS() *Code  { return c.op(0x57) }
func (c *Code) I64GeS() *Code  { return c.op(0x59) }
func (c *Code) I64Add() *Code  { return c.op(0x7C) }
func (c *Code) I64Sub() *Code  { return c.op(0x7D) }
func (c *Code) I64Mul() *Code  { return c.op(0x7E) }
func (c *Code) I64DivS() *Code { return c.op(0x7F) }
func (c *Code) I64RemS() *Code { return c.op(0x81) }
func (c *Code) I64DivU() *Code { return c.op(0x80) }
func (c *Code) I64RemU() *Code { return c.op(0x82) }
func (c *Code) I64And() *Code  { return c.op(0x83) }
func (c *Code) I64Or() *Code   { return c.op(0x84) }
func (c *Code) I64Xor() *Code  { return c.op(0x85) }

// f64 arithmetic and comparison.

func (c *Code) F64Eq() *Code    { return c.op(0x61) }
func (c *Code) F64Ne() *Code    { return c.op(0x62) }
func (c *Code) F64Lt() *Code    { return c.op(0x63) }
func (c *Code) F64Gt() *Code    { return c.op(0x64) }
func (c *Code) F64Le() *Code    { return c.op(0x65) }
func (c *Code) F64Ge() *Code    { return c.op(0x66) }
func (c *Code) F64Add() *Code   { return c.op(0xA0) }
func (c *Code) F64Sub() *Code   { return c.op(0xA1) }
func (c *Code) F64Mul() *Code   { return c.op(0xA2) }
func (c *Code) F64Div() *Code   { return c.op(0xA3) }
func (c *Code) F64Floor() *Code { return c.op(0x9C) }
func (c *Code) F64Neg() *Code   { return c.op(0x9A) }
func (c *Code) F64Abs() *Code   { return c.op(0x99) }
func (c *Code) F64Trunc() *Code { return c.op(0x9D) }

// Conversions.

func (c *Code) I32WrapI64() *Code        { return c.op(0xA7) }
func (c *Code) I64ExtendI32S() *Code     { return c.op(0xAC) }
func (c *Code) I64ExtendI32U() *Code     { return c.op(0xAD) }
func (c *Code) F64ConvertI64S() *Code    { return c.op(0xB9) }
func (c *Code) I64TruncSatF64S() *Code   { return c.op(0xFC).u(6) }
func (c *Code) I64ReinterpretF64() *Code { return c.op(0xBD) }

// References.

func (c *Code) RefNull(heap byte) *Code { return c.op(0xD0, heap) }
func (c *Code) RefNullType(typ int) *Code {
	c.buf.WriteByte(0xD0)
	c.buf.Write(sleb(int64(typ)))
	return c
}
func (c *Code) RefIsNull() *Code     { return c.op(0xD1) }
func (c *Code) RefFunc(fn int) *Code { return c.op(0xD2).u(uint64(fn)) }
func (c *Code) RefEq() *Code         { return c.op(0xD3) }
func (c *Code) RefAsNonNull() *Code  { return c.op(0xD4) }
func (c *Code) BrOnNull(d int) *Code { return c.op(0xD5).u(uint64(d)) }

// GC: structs.

func (c *Code) StructNew(typ int) *Code        { return c.gc(0).u(uint64(typ)) }
func (c *Code) StructNewDefault(typ int) *Code { return c.gc(1).u(uint64(typ)) }
func (c *Code) StructGet(typ, field int) *Code { return c.gc(2).u(uint64(typ)).u(uint64(field)) }
func (c *Code) StructSet(typ, field int) *Code { return c.gc(5).u(uint64(typ)).u(uint64(field)) }

// GC: arrays.

func (c *Code) ArrayNew(typ int) *Code           { return c.gc(6).u(uint64(typ)) }
func (c *Code) ArrayNewDefault(typ int) *Code    { return c.gc(7).u(uint64(typ)) }
func (c *Code) ArrayNewFixed(typ, n int) *Code   { return c.gc(8).u(uint64(typ)).u(uint64(n)) }
func (c *Code) ArrayNewData(typ, data int) *Code { return c.gc(9).u(uint64(typ)).u(uint64(data)) }
func (c *Code) ArrayGet(typ int) *Code           { return c.gc(11).u(uint64(typ)) }
func (c *Code) ArrayGetU(typ int) *Code          { return c.gc(13).u(uint64(typ)) }
func (c *Code) ArraySet(typ int) *Code           { return c.gc(14).u(uint64(typ)) }
func (c *Code) ArrayLen() *Code                  { return c.gc(15) }
func (c *Code) ArrayCopy(dst, src int) *Code     { return c.gc(17).u(uint64(dst)).u(uint64(src)) }

// GC: casts and tests against a concrete type (nullable forms).

func (c *Code) RefTest(typ int) *Code {
	c.gc(20)
	c.buf.Write(sleb(int64(typ)))
	return c
}
func (c *Code) RefTestNull(typ int) *Code {
	c.gc(21)
	c.buf.Write(sleb(int64(typ)))
	return c
}
func (c *Code) RefCast(typ int) *Code {
	c.gc(22)
	c.buf.Write(sleb(int64(typ)))
	return c
}
func (c *Code) RefCastNull(typ int) *Code {
	c.gc(23)
	c.buf.Write(sleb(int64(typ)))
	return c
}

// Bytes returns the encoded instructions (without the function's final end).
func (c *Code) Bytes() []byte { return c.buf.Bytes() }
