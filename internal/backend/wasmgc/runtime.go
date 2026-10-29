package wasmgc

import "math"

// The core runtime: value types and the operations every program needs,
// written with the instruction builder. Semantics follow the bytecode VM:
// checked 64-bit integer arithmetic ("integer overflow"), "/" always
// producing a float, "division by zero" for integers and floats alike,
// numeric equality across int and float, identity equality for lists and
// dicts, and text formatting that matches Go's fmt.Sprint.

// rt holds the indices of runtime types, globals, imports and functions.
type rt struct {
	m *Module

	// Types.
	tBytes, tStr, tInt, tFloat, tBool, tItems, tList, tDict int
	tBuf                                                    int
	tStrNull                                                int
	fnTypes                                                 map[int]int // HowlFrame function arity -> type
	tEqToI32, tEqEqToI32, tEqEqToEq, tEqToStr, tStrStrToStr int
	tI64ToEq, tF64ToEq, tI32ToEq, tEqToVoid, tVoid          int
	tF64ToI64, tF64ToI32, tEqI32ToI32, tI32ToEqRef          int

	// Globals.
	gErr, gTrue, gFalse int

	// Imports.
	iPrint, iFloatDigits, iFloatPoint, iParseFloat, iParseFloatOK int

	// Functions.
	fBoxInt, fBoxFloat, fBoxBool, fTruthy, fFail         int
	fAdd, fSub, fMul, fDiv, fMod, fCompare, fEq          int
	fConcat, fText, fItoa, fFtoa, fStrEq, fStrCmp        int
	fStrLen, fStrByte, fError                            int
	fToInt, fToFloat, fQuote, fTrim, fTypeName           int
	fListNew, fListAppend, fListGet, fListLen, fListText int

	strings map[string]int // literal -> data segment
}

func newRuntime(m *Module) *rt {
	r := &rt{m: m, fnTypes: map[int]int{}, strings: map[string]int{}}
	r.tBytes = m.AddType(TypeDef{Name: "Bytes", Array: &Field{Type: PackedI8, Mutable: true}})
	r.tStr = m.AddType(TypeDef{Name: "Str", Struct: []Field{{Type: Ref(r.tBytes)}}})
	r.tInt = m.AddType(TypeDef{Name: "Int", Struct: []Field{{Type: I64}}})
	r.tFloat = m.AddType(TypeDef{Name: "Float", Struct: []Field{{Type: F64}}})
	r.tBool = m.AddType(TypeDef{Name: "Bool", Struct: []Field{{Type: I32}}})
	r.tItems = m.AddType(TypeDef{Name: "Items", Array: &Field{Type: EqRef, Mutable: true}})
	// A list is an immutable (length, buffer) pair. Lists are values: the
	// buffer records how far it is used, and only a list whose length equals
	// that mark may append in place (Go-style append to the tip); any other
	// list copies first, so no list ever sees another's appends.
	r.tBuf = m.AddType(TypeDef{Name: "Buf", Struct: []Field{{Type: I32, Mutable: true}, {Type: Ref(r.tItems), Mutable: true}}})
	r.tList = m.AddType(TypeDef{Name: "List", Struct: []Field{{Type: I32}, {Type: Ref(r.tBuf)}}})
	// Dict: length, keys ($Str values in an Items array), values.
	r.tDict = m.AddType(TypeDef{Name: "Dict", Struct: []Field{{Type: I32, Mutable: true}, {Type: Ref(r.tItems), Mutable: true}, {Type: Ref(r.tItems), Mutable: true}}})
	r.tStrNull = -1

	fn := func(params []ValType, results ...ValType) int {
		return m.AddType(TypeDef{IsFunc: true, Params: params, Results: results})
	}
	r.tEqToI32 = fn([]ValType{EqRef}, I32)
	r.tEqEqToI32 = fn([]ValType{EqRef, EqRef}, I32)
	r.tEqEqToEq = fn([]ValType{EqRef, EqRef}, EqRef)
	r.tEqToStr = fn([]ValType{EqRef}, Ref(r.tStr))
	r.tStrStrToStr = fn([]ValType{Ref(r.tStr), Ref(r.tStr)}, Ref(r.tStr))
	r.tI64ToEq = fn([]ValType{I64}, EqRef)
	r.tF64ToEq = fn([]ValType{F64}, EqRef)
	r.tI32ToEq = fn([]ValType{I32}, EqRef)
	r.tEqToVoid = fn([]ValType{EqRef})
	r.tVoid = fn(nil)
	r.tF64ToI64 = fn([]ValType{F64}, I64)
	r.tF64ToI32 = fn([]ValType{F64}, I32)
	r.tEqI32ToI32 = fn([]ValType{EqRef, I32}, I32)
	r.tI32ToEqRef = fn(nil, EqRef)

	// Host imports. print receives a $Str; the host reads its bytes through
	// the exported __hf_str_len and __hf_str_byte. float_digits and
	// float_point give the shortest round-trip decimal digits of a finite,
	// positive float and the position of its decimal point (temporary: to
	// be replaced by a HowlFrame implementation, see the design plan).
	r.iPrint = m.AddImport("hf", "print", r.tEqToVoid)
	r.iFloatDigits = m.AddImport("hf", "float_digits", r.tF64ToI64)
	r.iFloatPoint = m.AddImport("hf", "float_point", r.tF64ToI32)
	// parse_float and parse_float_ok read a $Str with Go's ParseFloat
	// syntax (temporary, like float_digits).
	r.iParseFloat = m.AddImport("hf", "parse_float", r.funcType([]ValType{EqRef}, F64))
	r.iParseFloatOK = m.AddImport("hf", "parse_float_ok", r.tEqToI32)

	r.gErr = m.AddGlobal(Global{Type: EqRef, Mutable: true, Init: []byte{0xD0, HeapNone}})
	r.gTrue = m.AddGlobal(Global{Type: Ref(r.tBool), Init: append(append([]byte{0x41}, sleb(1)...), gcOp(0, r.tBool)...)})
	r.gFalse = m.AddGlobal(Global{Type: Ref(r.tBool), Init: append(append([]byte{0x41}, sleb(0)...), gcOp(0, r.tBool)...)})

	r.emitAll()
	return r
}

func gcOp(sub, typ int) []byte {
	return append(append([]byte{0xFB}, uleb(uint64(sub))...), uleb(uint64(typ))...)
}

// FuncType returns the type of a HowlFrame function of the given arity:
// eqref parameters, one eqref result.
func (r *rt) FuncType(arity int) int {
	if t, ok := r.fnTypes[arity]; ok {
		return t
	}
	params := make([]ValType, arity)
	for i := range params {
		params[i] = EqRef
	}
	t := r.m.AddType(TypeDef{IsFunc: true, Params: params, Results: []ValType{EqRef}})
	r.fnTypes[arity] = t
	return t
}

// StrLiteral pushes a new $Str holding s.
func (r *rt) StrLiteral(c *Code, s string) *Code {
	seg, ok := r.strings[s]
	if !ok {
		seg = r.m.AddData([]byte(s))
		r.strings[s] = seg
	}
	return c.I32Const(0).I32Const(int32(len(s))).ArrayNewData(r.tBytes, seg).StructNew(r.tStr)
}

// def adds a function with a builder body and returns its index.
func (r *rt) def(name string, typ int, locals []ValType, body func(c *Code)) int {
	c := NewCode()
	body(c)
	return r.m.AddFunc(Func{Name: name, Type: typ, Locals: locals, Body: c})
}

// Errors: fail sets $err to a message and returns null, so a caller can
// "return (call fail msg)". Callers test $err after anything that can fail.
func (r *rt) failWith(c *Code, msg string) *Code {
	return r.StrLiteral(c, msg).Call(r.fFail)
}

func (r *rt) emitAll() {
	m := r.m
	// Function indices are assigned in order; bodies may call functions
	// defined later because indices are reserved up front by declaring
	// placeholders first.
	type pending struct {
		idx  *int
		name string
		typ  int
	}
	plan := []pending{
		{&r.fBoxInt, "box_int", r.tI64ToEq},
		{&r.fBoxFloat, "box_float", r.tF64ToEq},
		{&r.fBoxBool, "box_bool", r.tI32ToEq},
		{&r.fTruthy, "truthy", r.tEqToI32},
		{&r.fFail, "fail", r.FuncTypeStrToEq()},
		{&r.fAdd, "add", r.tEqEqToEq},
		{&r.fSub, "sub", r.tEqEqToEq},
		{&r.fMul, "mul", r.tEqEqToEq},
		{&r.fDiv, "div", r.tEqEqToEq},
		{&r.fMod, "mod", r.tEqEqToEq},
		{&r.fCompare, "compare", r.tEqEqToI32},
		{&r.fEq, "eq", r.tEqEqToI32},
		{&r.fConcat, "concat", r.tStrStrToStr},
		{&r.fText, "text", r.tEqToStr},
		{&r.fItoa, "itoa", r.FuncTypeI64ToStr()},
		{&r.fFtoa, "ftoa", r.FuncTypeF64ToStr()},
		{&r.fStrEq, "str_eq", r.FuncTypeStrStrToI32()},
		{&r.fStrCmp, "str_cmp", r.FuncTypeStrStrToI32()},
		{&r.fStrLen, "__hf_str_len", r.tEqToI32},
		{&r.fStrByte, "__hf_str_byte", r.tEqI32ToI32},
		{&r.fError, "__hf_error", r.tI32ToEqRef},
		{&r.fToInt, "to_int", r.funcType([]ValType{EqRef}, EqRef)},
		{&r.fToFloat, "to_float", r.funcType([]ValType{EqRef}, EqRef)},
		{&r.fQuote, "quote", r.funcType([]ValType{Ref(r.tStr)}, Ref(r.tStr))},
		{&r.fTrim, "trim_space", r.funcType([]ValType{Ref(r.tStr)}, Ref(r.tStr))},
		{&r.fTypeName, "type_name", r.tEqToStr},
		{&r.fListNew, "list_new", r.funcType([]ValType{Ref(r.tItems)}, EqRef)},
		{&r.fListAppend, "list_append", r.tEqEqToEq},
		{&r.fListGet, "list_get", r.tEqEqToEq},
		{&r.fListLen, "list_len", r.funcType([]ValType{EqRef}, EqRef)},
		{&r.fListText, "list_text", r.tEqToStr},
	}
	base := len(m.Imports) + len(m.Funcs)
	for i, p := range plan {
		*p.idx = base + i
		m.Funcs = append(m.Funcs, Func{Name: p.name, Type: p.typ})
	}
	set := func(idx int, locals []ValType, body func(c *Code)) {
		c := NewCode()
		body(c)
		f := &m.Funcs[idx-len(m.Imports)]
		f.Locals = locals
		f.Body = c
	}
	m.Exports = append(m.Exports, Export{"__hf_str_len", r.fStrLen}, Export{"__hf_str_byte", r.fStrByte}, Export{"__hf_error", r.fError})

	set(r.fBoxInt, nil, func(c *Code) { c.LocalGet(0).StructNew(r.tInt) })
	set(r.fBoxFloat, nil, func(c *Code) { c.LocalGet(0).StructNew(r.tFloat) })
	set(r.fBoxBool, nil, func(c *Code) {
		c.LocalGet(0).If(Result(EqRef)).GlobalGet(r.gTrue).Else().GlobalGet(r.gFalse).End()
	})
	// truthy: a bool's value; nil is false; anything else is true.
	set(r.fTruthy, nil, func(c *Code) {
		c.LocalGet(0).RefTest(r.tBool).If(Result(I32)).
			LocalGet(0).RefCast(r.tBool).StructGet(r.tBool, 0).
			Else().LocalGet(0).RefIsNull().I32Eqz().End()
	})
	set(r.fFail, nil, func(c *Code) {
		c.LocalGet(0).GlobalSet(r.gErr).RefNull(HeapNone)
	})
	r.emitArith()
	r.emitStrings()
	r.emitText()
	set(r.fStrLen, nil, func(c *Code) {
		c.LocalGet(0).RefCast(r.tStr).StructGet(r.tStr, 0).ArrayLen()
	})
	set(r.fStrByte, nil, func(c *Code) {
		c.LocalGet(0).RefCast(r.tStr).StructGet(r.tStr, 0).LocalGet(1).ArrayGetU(r.tBytes)
	})
	set(r.fError, nil, func(c *Code) { c.GlobalGet(r.gErr) })
	r.emitConversions()
	r.emitLists()
}

func (r *rt) body(idx int, locals []ValType, fill func(c *Code)) {
	c := NewCode()
	fill(c)
	f := &r.m.Funcs[idx-len(r.m.Imports)]
	f.Locals = locals
	f.Body = c
}

func (r *rt) funcType(params []ValType, results ...ValType) int {
	for i, t := range r.m.Types {
		if t.IsFunc && sameTypes(t.Params, params) && sameTypes(t.Results, results) {
			return i
		}
	}
	return r.m.AddType(TypeDef{IsFunc: true, Params: params, Results: results})
}

func sameTypes(a, b []ValType) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if string(a[i]) != string(b[i]) {
			return false
		}
	}
	return true
}

func (r *rt) FuncTypeStrToEq() int     { return r.funcType([]ValType{Ref(r.tStr)}, EqRef) }
func (r *rt) FuncTypeI64ToStr() int    { return r.funcType([]ValType{I64}, Ref(r.tStr)) }
func (r *rt) FuncTypeF64ToStr() int    { return r.funcType([]ValType{F64}, Ref(r.tStr)) }
func (r *rt) FuncTypeStrStrToI32() int { return r.funcType([]ValType{Ref(r.tStr), Ref(r.tStr)}, I32) }

// emitArith writes add, sub, mul, div, mod, compare and eq.
//
// Locals in the binary operations: 0 a, 1 b, then 2 x (i64), 3 y (i64),
// 4 z (i64), 5 p (f64), 6 q (f64).
func (r *rt) emitArith() {
	nums := []ValType{I64, I64, I64, F64, F64}
	isInt := func(c *Code, l int) *Code { return c.LocalGet(l).RefTest(r.tInt) }
	isFloat := func(c *Code, l int) *Code { return c.LocalGet(l).RefTest(r.tFloat) }
	getInt := func(c *Code, l int) *Code { return c.LocalGet(l).RefCast(r.tInt).StructGet(r.tInt, 0) }
	// asFloat pushes a number operand as f64; callers have checked it is a number.
	asFloat := func(c *Code, l int) *Code {
		return isInt(c, l).If(Result(F64)).Do(func(c *Code) { getInt(c, l).F64ConvertI64S() }).
			Else().LocalGet(l).RefCast(r.tFloat).StructGet(r.tFloat, 0).End()
	}
	bothInt := func(c *Code) *Code { isInt(c, 0); isInt(c, 1); return c.I32And() }
	bothNum := func(c *Code) *Code {
		isInt(c, 0)
		isFloat(c, 0)
		c.I32Or()
		isInt(c, 1)
		isFloat(c, 1)
		return c.I32Or().I32And()
	}
	typeErr := func(c *Code, op string) *Code {
		return r.failWith(c, "TYPE_ERROR: cannot apply "+op+" to these values").Return()
	}
	overflow := func(c *Code) *Code { return r.failWith(c, "integer overflow").Return() }

	r.body(r.fAdd, nums, func(c *Code) {
		// Strings concatenate.
		c.LocalGet(0).RefTest(r.tStr).LocalGet(1).RefTest(r.tStr).I32And().If(NoResult).
			LocalGet(0).RefCast(r.tStr).LocalGet(1).RefCast(r.tStr).Call(r.fConcat).Return().End()
		bothInt(c).If(NoResult)
		getInt(c, 0).LocalSet(2)
		getInt(c, 1).LocalSet(3)
		c.LocalGet(2).LocalGet(3).I64Add().LocalSet(4)
		// Overflow when both operands share a sign the result lacks.
		c.LocalGet(2).LocalGet(4).I64Xor().LocalGet(3).LocalGet(4).I64Xor().I64And().I64Const(0).I64LtS().If(NoResult)
		overflow(c).End()
		c.LocalGet(4).Call(r.fBoxInt).Return().End()
		bothNum(c).If(NoResult)
		asFloat(c, 0)
		asFloat(c, 1)
		c.F64Add().Call(r.fBoxFloat).Return().End()
		typeErr(c, "+")
	})
	r.body(r.fSub, nums, func(c *Code) {
		bothInt(c).If(NoResult)
		getInt(c, 0).LocalSet(2)
		getInt(c, 1).LocalSet(3)
		c.LocalGet(2).LocalGet(3).I64Sub().LocalSet(4)
		// Overflow when the operands differ in sign and the result's sign
		// differs from the first operand's.
		c.LocalGet(2).LocalGet(3).I64Xor().LocalGet(2).LocalGet(4).I64Xor().I64And().I64Const(0).I64LtS().If(NoResult)
		overflow(c).End()
		c.LocalGet(4).Call(r.fBoxInt).Return().End()
		bothNum(c).If(NoResult)
		asFloat(c, 0)
		asFloat(c, 1)
		c.F64Sub().Call(r.fBoxFloat).Return().End()
		typeErr(c, "-")
	})
	r.body(r.fMul, nums, func(c *Code) {
		bothInt(c).If(NoResult)
		getInt(c, 0).LocalSet(2)
		getInt(c, 1).LocalSet(3)
		c.LocalGet(2).LocalGet(3).I64Mul().LocalSet(4)
		// Overflow unless a == 0, or z / a == b with the (-1, MIN) case excluded.
		c.LocalGet(2).I64Eqz().I32Eqz().If(NoResult)
		c.LocalGet(2).I64Const(-1).I64Eq().LocalGet(3).I64Const(-1 << 63).I64Eq().I32And().If(NoResult)
		overflow(c).End()
		c.LocalGet(3).I64Const(-1).I64Eq().LocalGet(2).I64Const(-1 << 63).I64Eq().I32And().If(NoResult)
		overflow(c).End()
		c.LocalGet(4).LocalGet(2).I64DivS().LocalGet(3).I64Ne().If(NoResult)
		overflow(c).End()
		c.End()
		c.LocalGet(4).Call(r.fBoxInt).Return().End()
		bothNum(c).If(NoResult)
		asFloat(c, 0)
		asFloat(c, 1)
		c.F64Mul().Call(r.fBoxFloat).Return().End()
		typeErr(c, "*")
	})
	// "/" always divides as floats, like the VM.
	r.body(r.fDiv, nums, func(c *Code) {
		bothNum(c).If(NoResult)
		asFloat(c, 1)
		c.LocalSet(6)
		c.LocalGet(6).F64Const(0).F64Eq().If(NoResult)
		r.failWith(c, "division by zero").Return().End()
		asFloat(c, 0)
		c.LocalGet(6).F64Div().Call(r.fBoxFloat).Return().End()
		typeErr(c, "/")
	})
	r.body(r.fMod, nums, func(c *Code) {
		bothInt(c).If(NoResult)
		getInt(c, 1).LocalSet(3)
		c.LocalGet(3).I64Eqz().If(NoResult)
		r.failWith(c, "division by zero").Return().End()
		// MIN % -1 traps in WebAssembly; its value is 0.
		c.LocalGet(3).I64Const(-1).I64Eq().If(NoResult).I64Const(0).Call(r.fBoxInt).Return().End()
		getInt(c, 0)
		c.LocalGet(3).I64RemS().Call(r.fBoxInt).Return().End()
		typeErr(c, "%")
	})
	// compare: -1, 0 or 1 for two numbers or two strings; sets $err otherwise.
	r.body(r.fCompare, nums, func(c *Code) {
		bothInt(c).If(NoResult)
		getInt(c, 0).LocalSet(2)
		getInt(c, 1).LocalSet(3)
		c.LocalGet(2).LocalGet(3).I64GtS().LocalGet(2).LocalGet(3).I64LtS().I32Sub().Return().End()
		bothNum(c).If(NoResult)
		asFloat(c, 0)
		c.LocalSet(5)
		asFloat(c, 1)
		c.LocalSet(6)
		c.LocalGet(5).LocalGet(6).F64Gt().LocalGet(5).LocalGet(6).F64Lt().I32Sub().Return().End()
		c.LocalGet(0).RefTest(r.tStr).LocalGet(1).RefTest(r.tStr).I32And().If(NoResult).
			LocalGet(0).RefCast(r.tStr).LocalGet(1).RefCast(r.tStr).Call(r.fStrCmp).Return().End()
		r.failWith(c, "TYPE_ERROR: cannot compare these values").Drop().I32Const(0)
	})
	// eq: numbers by value (int and float compare numerically), strings by
	// content, booleans by value, nil only to nil, lists and dicts by identity.
	r.body(r.fEq, nums, func(c *Code) {
		c.LocalGet(0).RefIsNull().LocalGet(1).RefIsNull().I32Or().If(NoResult).
			LocalGet(0).RefIsNull().LocalGet(1).RefIsNull().I32And().Return().End()
		bothNum(c).If(NoResult)
		bothInt(c).If(NoResult)
		getInt(c, 0)
		getInt(c, 1)
		c.I64Eq().Return().End()
		asFloat(c, 0)
		asFloat(c, 1)
		c.F64Eq().Return().End()
		c.LocalGet(0).RefTest(r.tStr).LocalGet(1).RefTest(r.tStr).I32And().If(NoResult).
			LocalGet(0).RefCast(r.tStr).LocalGet(1).RefCast(r.tStr).Call(r.fStrEq).Return().End()
		c.LocalGet(0).RefTest(r.tBool).LocalGet(1).RefTest(r.tBool).I32And().If(NoResult).
			LocalGet(0).RefCast(r.tBool).StructGet(r.tBool, 0).LocalGet(1).RefCast(r.tBool).StructGet(r.tBool, 0).I32Eq().Return().End()
		c.LocalGet(0).LocalGet(1).RefEq()
	})
}

// Do lets a builder fragment be written inline in a chain.
func (c *Code) Do(f func(c *Code)) *Code { f(c); return c }

func (r *rt) emitStrings() {
	// concat(a, b): a new $Str with a's bytes then b's.
	// locals: 0 a, 1 b, 2 out bytes, 3 la, 4 lb
	r.body(r.fConcat, []ValType{RefNull(r.tBytes), I32, I32}, func(c *Code) {
		c.LocalGet(0).StructGet(r.tStr, 0).ArrayLen().LocalSet(3)
		c.LocalGet(1).StructGet(r.tStr, 0).ArrayLen().LocalSet(4)
		c.LocalGet(3).LocalGet(4).I32Add().ArrayNewDefault(r.tBytes).LocalSet(2)
		c.LocalGet(2).I32Const(0).LocalGet(0).StructGet(r.tStr, 0).I32Const(0).LocalGet(3).ArrayCopy(r.tBytes, r.tBytes)
		c.LocalGet(2).LocalGet(3).LocalGet(1).StructGet(r.tStr, 0).I32Const(0).LocalGet(4).ArrayCopy(r.tBytes, r.tBytes)
		c.LocalGet(2).RefAsNonNull().StructNew(r.tStr)
	})
	// str_cmp(a, b): bytewise comparison, -1, 0 or 1 (UTF-8 byte order is
	// code point order, matching Go's string comparison).
	// locals: 0 a, 1 b, 2 i, 3 la, 4 lb, 5 x, 6 y
	r.body(r.fStrCmp, []ValType{I32, I32, I32, I32, I32}, func(c *Code) {
		c.LocalGet(0).StructGet(r.tStr, 0).ArrayLen().LocalSet(3)
		c.LocalGet(1).StructGet(r.tStr, 0).ArrayLen().LocalSet(4)
		c.Block(NoResult).Loop(NoResult)
		c.LocalGet(2).LocalGet(3).I32GeU().LocalGet(2).LocalGet(4).I32GeU().I32Or().BrIf(1)
		c.LocalGet(0).StructGet(r.tStr, 0).LocalGet(2).ArrayGetU(r.tBytes).LocalSet(5)
		c.LocalGet(1).StructGet(r.tStr, 0).LocalGet(2).ArrayGetU(r.tBytes).LocalSet(6)
		c.LocalGet(5).LocalGet(6).I32Ne().If(NoResult).
			LocalGet(5).LocalGet(6).I32GtU().LocalGet(5).LocalGet(6).I32LtU().I32Sub().Return().End()
		c.LocalGet(2).I32Const(1).I32Add().LocalSet(2).Br(0)
		c.End().End()
		c.LocalGet(3).LocalGet(4).I32GtU().LocalGet(3).LocalGet(4).I32LtU().I32Sub()
	})
	r.body(r.fStrEq, nil, func(c *Code) {
		c.LocalGet(0).LocalGet(1).Call(r.fStrCmp).I32Eqz()
	})
}

// emitText writes text (any value to $Str, as fmt.Sprint), itoa and ftoa.
func (r *rt) emitText() {
	// itoa(v): decimal digits of a signed 64-bit integer.
	// locals: 0 v, 1 buf, 2 pos, 3 neg, 4 u (unsigned magnitude)
	// 5 out
	r.body(r.fItoa, []ValType{RefNull(r.tBytes), I32, I32, I64, RefNull(r.tBytes)}, func(c *Code) {
		c.I32Const(20).ArrayNewDefault(r.tBytes).LocalSet(1)
		c.I32Const(20).LocalSet(2)
		c.LocalGet(0).I64Const(0).I64LtS().LocalSet(3)
		// Magnitude as unsigned: 0 - v for negatives (MIN wraps to itself, read as unsigned).
		c.LocalGet(3).If(Result(I64)).I64Const(0).LocalGet(0).I64Sub().Else().LocalGet(0).End().LocalSet(4)
		c.Loop(NoResult)
		c.LocalGet(2).I32Const(1).I32Sub().LocalSet(2)
		c.LocalGet(1).LocalGet(2).LocalGet(4).I64Const(10).I64RemU().I32WrapI64().I32Const('0').I32Add().ArraySet(r.tBytes)
		c.LocalGet(4).I64Const(10).I64DivU().LocalTee(4).I64Eqz().I32Eqz().BrIf(0)
		c.End()
		c.LocalGet(3).If(NoResult).
			LocalGet(2).I32Const(1).I32Sub().LocalSet(2).
			LocalGet(1).LocalGet(2).I32Const('-').ArraySet(r.tBytes).End()
		r.copyOut(c, 1, 2, 5, func(c *Code) { c.I32Const(20) })
	})
	r.emitFtoa()
	// text(v): "" for nil, true/false, decimal integers, Go-style floats,
	// strings as they are.
	r.body(r.fText, nil, func(c *Code) {
		c.LocalGet(0).RefIsNull().If(NoResult)
		r.StrLiteral(c, "").Return().End()
		c.LocalGet(0).RefTest(r.tStr).If(NoResult).LocalGet(0).RefCast(r.tStr).Return().End()
		c.LocalGet(0).RefTest(r.tInt).If(NoResult).LocalGet(0).RefCast(r.tInt).StructGet(r.tInt, 0).Call(r.fItoa).Return().End()
		c.LocalGet(0).RefTest(r.tFloat).If(NoResult).LocalGet(0).RefCast(r.tFloat).StructGet(r.tFloat, 0).Call(r.fFtoa).Return().End()
		c.LocalGet(0).RefTest(r.tBool).If(NoResult).
			LocalGet(0).RefCast(r.tBool).StructGet(r.tBool, 0).If(NoResult)
		r.StrLiteral(c, "true").Return().End()
		r.StrLiteral(c, "false").Return().End()
		c.LocalGet(0).RefTest(r.tList).If(NoResult).LocalGet(0).Call(r.fListText).Return().End()
		r.StrLiteral(c, "<value>")
	})
}

// copyOut pushes a new $Str holding buf[from:to]: buf and from are locals,
// to is pushed by toFn, and out is a scratch local of type (ref null $Bytes).
func (r *rt) copyOut(c *Code, buf, from, out int, toFn func(c *Code)) *Code {
	toFn(c)
	c.LocalGet(from).I32Sub().ArrayNewDefault(r.tBytes).LocalSet(out)
	c.LocalGet(out).I32Const(0).LocalGet(buf).LocalGet(from)
	toFn(c)
	c.LocalGet(from).I32Sub().ArrayCopy(r.tBytes, r.tBytes)
	return c.LocalGet(out).RefAsNonNull().StructNew(r.tStr)
}

// emitFtoa writes ftoa: a float as Go's fmt.Sprint shows it. The shortest
// round-trip digits and the decimal point position come from the host
// (exact in every JavaScript engine); the layout is Go's %v rule: exponent
// form when the exponent is below -4 or at least 6, with at least two
// exponent digits; otherwise plain decimal.
//
// locals: 0 f, 1 buf, 2 pos, 3 neg, 4 digits, 5 dp, 6 nd, 7 dbuf, 8 i,
// 9 exp, 10 out
func (r *rt) emitFtoa() {
	locals := []ValType{RefNull(r.tBytes), I32, I32, I64, I32, I32, RefNull(r.tBytes), I32, I32, RefNull(r.tBytes)}
	put := func(c *Code, b byte) {
		c.LocalGet(1).LocalGet(2).I32Const(int32(b)).ArraySet(r.tBytes)
		c.LocalGet(2).I32Const(1).I32Add().LocalSet(2)
	}
	// copyDigits appends dbuf[from:to] where from and to are pushed by fns.
	copyDigits := func(c *Code, from, to func(c *Code)) {
		from(c)
		c.LocalSet(8)
		c.Block(NoResult).Loop(NoResult)
		c.LocalGet(8)
		to(c)
		c.I32GeS().BrIf(1)
		c.LocalGet(1).LocalGet(2).LocalGet(7).LocalGet(8).ArrayGetU(r.tBytes).ArraySet(r.tBytes)
		c.LocalGet(2).I32Const(1).I32Add().LocalSet(2)
		c.LocalGet(8).I32Const(1).I32Add().LocalSet(8)
		c.Br(0).End().End()
	}
	zeros := func(c *Code, n func(c *Code)) {
		n(c)
		c.LocalSet(8)
		c.Block(NoResult).Loop(NoResult)
		c.LocalGet(8).I32Const(0).I32LeS().BrIf(1)
		put(c, '0')
		c.LocalGet(8).I32Const(1).I32Sub().LocalSet(8)
		c.Br(0).End().End()
	}
	r.body(r.fFtoa, locals, func(c *Code) {
		c.LocalGet(0).LocalGet(0).F64Ne().If(NoResult)
		r.StrLiteral(c, "NaN").Return().End()
		c.LocalGet(0).F64Const(posInf).F64Eq().If(NoResult)
		r.StrLiteral(c, "+Inf").Return().End()
		c.LocalGet(0).F64Const(-posInf).F64Eq().If(NoResult)
		r.StrLiteral(c, "-Inf").Return().End()
		c.LocalGet(0).F64Const(0).F64Eq().If(NoResult)
		c.LocalGet(0).I64ReinterpretF64().I64Const(0).I64LtS().If(NoResult)
		r.StrLiteral(c, "-0").Return().End()
		r.StrLiteral(c, "0").Return().End()

		c.I32Const(40).ArrayNewDefault(r.tBytes).LocalSet(1)
		c.LocalGet(0).F64Const(0).F64Lt().LocalSet(3)
		c.LocalGet(3).If(NoResult).LocalGet(0).F64Neg().LocalSet(0)
		put(c, '-')
		c.End()
		c.LocalGet(0).Call(r.iFloatDigits).LocalSet(4)
		c.LocalGet(0).Call(r.iFloatPoint).LocalSet(5)
		c.LocalGet(4).Call(r.fItoa).StructGet(r.tStr, 0).LocalTee(7).ArrayLen().LocalSet(6)
		c.LocalGet(5).I32Const(1).I32Sub().LocalSet(9)

		c.LocalGet(9).I32Const(-4).I32LtS().LocalGet(9).I32Const(6).I32GeS().I32Or().If(NoResult)
		// d[.ddd]e±XX
		copyDigits(c, func(c *Code) { c.I32Const(0) }, func(c *Code) { c.I32Const(1) })
		c.LocalGet(6).I32Const(1).I32GtS().If(NoResult)
		put(c, '.')
		copyDigits(c, func(c *Code) { c.I32Const(1) }, func(c *Code) { c.LocalGet(6) })
		c.End()
		put(c, 'e')
		c.LocalGet(9).I32Const(0).I32LtS().If(NoResult)
		put(c, '-')
		c.I32Const(0).LocalGet(9).I32Sub().LocalSet(9)
		c.Else()
		put(c, '+')
		c.End()
		c.LocalGet(9).I32Const(10).I32LtS().If(NoResult)
		put(c, '0')
		c.End()
		// Exponent digits (at most three): reuse dbuf for its text.
		c.LocalGet(9).I64ExtendI32S().Call(r.fItoa).StructGet(r.tStr, 0).LocalTee(7).ArrayLen().LocalSet(6)
		copyDigits(c, func(c *Code) { c.I32Const(0) }, func(c *Code) { c.LocalGet(6) })
		c.Else()
		c.LocalGet(5).I32Const(0).I32LeS().If(NoResult)
		// 0.000ddd
		put(c, '0')
		put(c, '.')
		zeros(c, func(c *Code) { c.I32Const(0).LocalGet(5).I32Sub() })
		copyDigits(c, func(c *Code) { c.I32Const(0) }, func(c *Code) { c.LocalGet(6) })
		c.Else()
		c.LocalGet(5).LocalGet(6).I32GeS().If(NoResult)
		// ddd000
		copyDigits(c, func(c *Code) { c.I32Const(0) }, func(c *Code) { c.LocalGet(6) })
		zeros(c, func(c *Code) { c.LocalGet(5).LocalGet(6).I32Sub() })
		c.Else()
		// dd.ddd
		copyDigits(c, func(c *Code) { c.I32Const(0) }, func(c *Code) { c.LocalGet(5) })
		put(c, '.')
		copyDigits(c, func(c *Code) { c.LocalGet(5) }, func(c *Code) { c.LocalGet(6) })
		c.End()
		c.End()
		c.End()
		c.I32Const(0).LocalSet(8)
		r.copyOut(c, 1, 8, 10, func(c *Code) { c.LocalGet(2) })
	})
}

var posInf = math.Inf(1)

// emitConversions writes to_int, to_float and their helpers with the VM's
// rules and messages: to_int truncates floats (failing outside int64), and
// parses trimmed decimal integers; to_float parses with Go's ParseFloat
// syntax; anything else is "cannot convert <type> to int|float".
func (r *rt) emitConversions() {
	// type_name: the Go-ish type word the VM uses in messages.
	r.body(r.fTypeName, nil, func(c *Code) {
		for _, tn := range []struct {
			t    int
			name string
		}{{r.tBool, "bool"}, {r.tList, "list"}, {r.tDict, "map"}} {
			c.LocalGet(0).RefTest(tn.t).If(NoResult)
			r.StrLiteral(c, tn.name).Return().End()
		}
		r.StrLiteral(c, "value")
	})
	// trim_space: ASCII whitespace off both ends.
	// locals: 0 s, 1 b, 2 from, 3 to, 4 out
	isSpace := func(c *Code) *Code {
		// stack: byte -> i32 (1 if space, tab, newline, vertical tab, form feed or return)
		t := 5
		c.LocalSet(t)
		return c.LocalGet(t).I32Const(' ').I32Eq().LocalGet(t).I32Const(9).I32GeU().LocalGet(t).I32Const(13).I32LeU().I32And().I32Or()
	}
	r.body(r.fTrim, []ValType{RefNull(r.tBytes), I32, I32, RefNull(r.tBytes), I32}, func(c *Code) {
		c.LocalGet(0).StructGet(r.tStr, 0).LocalTee(1).ArrayLen().LocalSet(3)
		c.Block(NoResult).Loop(NoResult)
		c.LocalGet(2).LocalGet(3).I32GeU().BrIf(1)
		c.LocalGet(1).LocalGet(2).ArrayGetU(r.tBytes)
		isSpace(c).I32Eqz().BrIf(1)
		c.LocalGet(2).I32Const(1).I32Add().LocalSet(2).Br(0).End().End()
		c.Block(NoResult).Loop(NoResult)
		c.LocalGet(3).LocalGet(2).I32LeU().BrIf(1)
		c.LocalGet(1).LocalGet(3).I32Const(1).I32Sub().ArrayGetU(r.tBytes)
		isSpace(c).I32Eqz().BrIf(1)
		c.LocalGet(3).I32Const(1).I32Sub().LocalSet(3).Br(0).End().End()
		r.copyOut(c, 1, 2, 4, func(c *Code) { c.LocalGet(3) })
	})
	// quote: Go %q for the characters messages meet: backslash, quote and
	// common control characters escaped; other bytes as they are.
	// locals: 0 s, 1 in, 2 i, 3 n, 4 buf, 5 pos, 6 b, 7 out
	r.body(r.fQuote, []ValType{RefNull(r.tBytes), I32, I32, RefNull(r.tBytes), I32, I32, RefNull(r.tBytes)}, func(c *Code) {
		put := func(c *Code, b func(c *Code)) {
			c.LocalGet(4).LocalGet(5)
			b(c)
			c.ArraySet(r.tBytes).LocalGet(5).I32Const(1).I32Add().LocalSet(5)
		}
		lit := func(b byte) func(c *Code) { return func(c *Code) { c.I32Const(int32(b)) } }
		c.LocalGet(0).StructGet(r.tStr, 0).LocalTee(1).ArrayLen().LocalSet(3)
		c.LocalGet(3).I32Const(2).I32Mul().I32Const(2).I32Add().ArrayNewDefault(r.tBytes).LocalSet(4)
		put(c, lit('"'))
		c.Block(NoResult).Loop(NoResult)
		c.LocalGet(2).LocalGet(3).I32GeU().BrIf(1)
		c.LocalGet(1).LocalGet(2).ArrayGetU(r.tBytes).LocalSet(6)
		for _, e := range []struct{ from, to byte }{{'"', '"'}, {'\\', '\\'}, {'\n', 'n'}, {'\t', 't'}, {'\r', 'r'}} {
			c.LocalGet(6).I32Const(int32(e.from)).I32Eq().If(NoResult)
			put(c, lit('\\'))
			put(c, lit(e.to))
			c.LocalGet(2).I32Const(1).I32Add().LocalSet(2).Br(2).End()
		}
		put(c, func(c *Code) { c.LocalGet(6) })
		c.LocalGet(2).I32Const(1).I32Add().LocalSet(2).Br(0).End().End()
		put(c, lit('"'))
		c.I32Const(0).LocalSet(2)
		r.copyOut(c, 4, 2, 7, func(c *Code) { c.LocalGet(5) })
	})
	cannot := func(c *Code, to string) {
		// message: cannot convert <type> to <to>
		r.StrLiteral(c, "cannot convert ")
		c.LocalGet(0).Call(r.fTypeName).Call(r.fConcat)
		r.StrLiteral(c, " to "+to).Call(r.fConcat).Call(r.fFail).Return()
	}
	// to_int. locals: 0 v, 1 s (trimmed bytes), 2 i, 3 n, 4 neg, 5 acc (i64), 6 d, 7 f
	r.body(r.fToInt, []ValType{RefNull(r.tBytes), I32, I32, I32, I64, I32, F64}, func(c *Code) {
		c.LocalGet(0).RefTest(r.tInt).If(NoResult).LocalGet(0).Return().End()
		c.LocalGet(0).RefTest(r.tFloat).If(NoResult)
		c.LocalGet(0).RefCast(r.tFloat).StructGet(r.tFloat, 0).LocalTee(7)
		// In range when -2^63 <= f < 2^63 (NaN fails both).
		c.F64Const(-9223372036854775808.0).F64Ge().LocalGet(7).F64Const(9223372036854775808.0).F64Lt().I32And().If(NoResult)
		c.LocalGet(7).I64TruncSatF64S().Call(r.fBoxInt).Return().End()
		r.StrLiteral(c, "cannot convert ")
		c.LocalGet(7).Call(r.fFtoa).Call(r.fConcat)
		r.StrLiteral(c, " to int").Call(r.fConcat).Call(r.fFail).Return()
		c.End()
		c.LocalGet(0).RefTest(r.tStr).If(NoResult)
		c.LocalGet(0).RefCast(r.tStr).Call(r.fTrim).StructGet(r.tStr, 0).LocalTee(1).ArrayLen().LocalSet(3)
		bad := func(c *Code) {
			r.StrLiteral(c, "cannot convert ")
			c.LocalGet(0).RefCast(r.tStr).Call(r.fQuote).Call(r.fConcat)
			r.StrLiteral(c, " to int").Call(r.fConcat).Call(r.fFail).Return()
		}
		c.LocalGet(3).I32Eqz().If(NoResult)
		bad(c)
		c.End()
		// Optional sign.
		c.LocalGet(1).I32Const(0).ArrayGetU(r.tBytes).LocalTee(6).I32Const('-').I32Eq().LocalGet(6).I32Const('+').I32Eq().I32Or().If(NoResult)
		c.LocalGet(6).I32Const('-').I32Eq().LocalSet(4)
		c.I32Const(1).LocalSet(2)
		c.LocalGet(3).I32Const(1).I32Eq().If(NoResult)
		bad(c)
		c.End().End()
		// Accumulate as a negative number so MIN fits: acc = acc*10 - d.
		c.Block(NoResult).Loop(NoResult)
		c.LocalGet(2).LocalGet(3).I32GeU().BrIf(1)
		c.LocalGet(1).LocalGet(2).ArrayGetU(r.tBytes).I32Const('0').I32Sub().LocalTee(6).I32Const(9).I32GtU().If(NoResult)
		bad(c)
		c.End()
		// Overflow: acc < MIN/10, or acc*10 < MIN + d.
		c.LocalGet(5).I64Const(-922337203685477580).I64LtS().If(NoResult)
		bad(c)
		c.End()
		c.LocalGet(5).I64Const(10).I64Mul().LocalTee(5).I64Const(-1 << 63).LocalGet(6).I64ExtendI32U().I64Add().I64LtS().If(NoResult)
		bad(c)
		c.End()
		c.LocalGet(5).LocalGet(6).I64ExtendI32U().I64Sub().LocalSet(5)
		c.LocalGet(2).I32Const(1).I32Add().LocalSet(2).Br(0).End().End()
		c.LocalGet(4).If(Result(I64)).LocalGet(5).Else()
		c.LocalGet(5).I64Const(-1 << 63).I64Eq().If(NoResult)
		bad(c)
		c.End()
		c.I64Const(0).LocalGet(5).I64Sub().End()
		c.Call(r.fBoxInt).Return()
		c.End()
		cannot(c, "int")
	})
	r.body(r.fToFloat, nil, func(c *Code) {
		c.LocalGet(0).RefTest(r.tFloat).If(NoResult).LocalGet(0).Return().End()
		c.LocalGet(0).RefTest(r.tInt).If(NoResult).LocalGet(0).RefCast(r.tInt).StructGet(r.tInt, 0).F64ConvertI64S().Call(r.fBoxFloat).Return().End()
		c.LocalGet(0).RefTest(r.tStr).If(NoResult)
		c.LocalGet(0).RefCast(r.tStr).Call(r.fTrim).Call(r.iParseFloatOK).If(NoResult)
		c.LocalGet(0).RefCast(r.tStr).Call(r.fTrim).Call(r.iParseFloat).Call(r.fBoxFloat).Return().End()
		r.StrLiteral(c, "cannot convert ")
		c.LocalGet(0).RefCast(r.tStr).Call(r.fQuote).Call(r.fConcat)
		r.StrLiteral(c, " to float").Call(r.fConcat).Call(r.fFail).Return()
		c.End()
		cannot(c, "float")
	})
}

// emitLists writes the list primitives. Everything else on lists
// (list_slice, list_contains, map_list, filter, ...) is desugared into these.
func (r *rt) emitLists() {
	typeErr := func(c *Code, what string) {
		r.failWith(c, "TYPE_ERROR: "+what).Return()
	}
	// list_new(items): a list owning a fresh buffer holding items.
	r.body(r.fListNew, nil, func(c *Code) {
		c.LocalGet(0).ArrayLen()
		c.LocalGet(0).ArrayLen().LocalGet(0).StructNew(r.tBuf)
		c.StructNew(r.tList)
	})
	// list_append(l, v): a new list one longer. locals: 0 l, 1 v, 2 list,
	// 3 buf, 4 n, 5 items
	r.body(r.fListAppend, []ValType{RefNull(r.tList), RefNull(r.tBuf), I32, RefNull(r.tItems)}, func(c *Code) {
		c.LocalGet(0).RefTest(r.tList).I32Eqz().If(NoResult)
		typeErr(c, "append expects a list")
		c.End()
		c.LocalGet(0).RefCast(r.tList).LocalTee(2).StructGet(r.tList, 0).LocalSet(4)
		c.LocalGet(2).StructGet(r.tList, 1).LocalSet(3)
		// At the tip of its buffer: append in place (growing the array).
		c.LocalGet(4).LocalGet(3).StructGet(r.tBuf, 0).I32Eq().If(NoResult)
		c.LocalGet(4).LocalGet(3).StructGet(r.tBuf, 1).ArrayLen().I32Eq().If(NoResult)
		c.LocalGet(4).I32Const(2).I32Mul().I32Const(4).I32Add().ArrayNewDefault(r.tItems).LocalSet(5)
		c.LocalGet(5).I32Const(0).LocalGet(3).StructGet(r.tBuf, 1).I32Const(0).LocalGet(4).ArrayCopy(r.tItems, r.tItems)
		c.LocalGet(3).LocalGet(5).RefAsNonNull().StructSet(r.tBuf, 1)
		c.End()
		c.LocalGet(3).StructGet(r.tBuf, 1).LocalGet(4).LocalGet(1).ArraySet(r.tItems)
		c.LocalGet(3).LocalGet(4).I32Const(1).I32Add().StructSet(r.tBuf, 0)
		c.LocalGet(4).I32Const(1).I32Add().LocalGet(3).RefAsNonNull().StructNew(r.tList).Return()
		c.End()
		// Behind the tip: copy into a new buffer.
		c.LocalGet(4).I32Const(2).I32Mul().I32Const(4).I32Add().ArrayNewDefault(r.tItems).LocalSet(5)
		c.LocalGet(5).I32Const(0).LocalGet(3).StructGet(r.tBuf, 1).I32Const(0).LocalGet(4).ArrayCopy(r.tItems, r.tItems)
		c.LocalGet(5).LocalGet(4).LocalGet(1).ArraySet(r.tItems)
		c.LocalGet(4).I32Const(1).I32Add()
		c.LocalGet(4).I32Const(1).I32Add().LocalGet(5).RefAsNonNull().StructNew(r.tBuf)
		c.StructNew(r.tList)
	})
	// list_get(l, i): the element, or nil when i is out of range.
	// locals: 0 l, 1 i, 2 idx
	r.body(r.fListGet, []ValType{I64}, func(c *Code) {
		c.LocalGet(0).RefTest(r.tList).I32Eqz().If(NoResult)
		typeErr(c, "list_get expects a list")
		c.End()
		c.LocalGet(1).RefTest(r.tInt).I32Eqz().If(NoResult)
		typeErr(c, "list_get expects an integer index")
		c.End()
		c.LocalGet(1).RefCast(r.tInt).StructGet(r.tInt, 0).LocalTee(2)
		c.I64Const(0).I64LtS().LocalGet(2).LocalGet(0).RefCast(r.tList).StructGet(r.tList, 0).I64ExtendI32U().I64GeS().I32Or().If(NoResult)
		c.RefNull(HeapNone).Return().End()
		c.LocalGet(0).RefCast(r.tList).StructGet(r.tList, 1).StructGet(r.tBuf, 1).LocalGet(2).I32WrapI64().ArrayGet(r.tItems)
	})
	r.body(r.fListLen, nil, func(c *Code) {
		c.LocalGet(0).RefTest(r.tList).I32Eqz().If(NoResult)
		typeErr(c, "list_len expects a list")
		c.End()
		c.LocalGet(0).RefCast(r.tList).StructGet(r.tList, 0).I64ExtendI32U().Call(r.fBoxInt)
	})
	// list_text(l): "[" elements as text, separated by spaces "]".
	// locals: 0 l, 1 list, 2 i, 3 acc
	r.body(r.fListText, []ValType{RefNull(r.tList), I32, RefNull(r.tStr)}, func(c *Code) {
		c.LocalGet(0).RefCast(r.tList).LocalSet(1)
		r.StrLiteral(c, "[").LocalSet(3)
		c.Block(NoResult).Loop(NoResult)
		c.LocalGet(2).LocalGet(1).StructGet(r.tList, 0).I32GeU().BrIf(1)
		c.LocalGet(2).If(NoResult)
		c.LocalGet(3).RefAsNonNull()
		r.StrLiteral(c, " ").Call(r.fConcat).LocalSet(3)
		c.End()
		c.LocalGet(3).RefAsNonNull()
		c.LocalGet(1).StructGet(r.tList, 1).StructGet(r.tBuf, 1).LocalGet(2).ArrayGet(r.tItems).Call(r.fText)
		c.Call(r.fConcat).LocalSet(3)
		c.LocalGet(2).I32Const(1).I32Add().LocalSet(2).Br(0).End().End()
		c.LocalGet(3).RefAsNonNull()
		r.StrLiteral(c, "]").Call(r.fConcat)
	})
}
