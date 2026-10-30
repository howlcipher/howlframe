// Package wasmgc compiles HowlFrame programs to WebAssembly modules that use
// the GC proposal (structs, arrays, typed references). See
// docs/design/wasm_gc_backend.md.
//
// This file is the binary encoder: a Module holds every section the backend
// emits and Bytes writes the standard binary format. No external assembler
// is involved.
package wasmgc

import (
	"bytes"
	"fmt"
	"math"
)

// ValType is an encoded value type: a single byte for numeric types and
// abstract reference shorthands, or a reference prefix and heap type.
type ValType []byte

// Numeric types.
var (
	I32 = ValType{0x7F}
	I64 = ValType{0x7E}
	F32 = ValType{0x7D}
	F64 = ValType{0x7C}
)

// Abstract heap types, as signed LEB128 single bytes.
const (
	HeapFunc     = 0x70
	HeapExtern   = 0x6F
	HeapAny      = 0x6E
	HeapEq       = 0x6D
	HeapI31      = 0x6C
	HeapStruct   = 0x6B
	HeapArray    = 0x6A
	HeapNone     = 0x71
	HeapNoExtern = 0x72
	HeapNoFunc   = 0x73
)

// Nullable abstract references.
var (
	FuncRef   = ValType{0x70}
	ExternRef = ValType{0x6F}
	AnyRef    = ValType{0x6E}
	EqRef     = ValType{0x6D}
)

// RefNull is (ref null <type index>).
func RefNull(typeIndex int) ValType { return append(ValType{0x63}, sleb(int64(typeIndex))...) }

// Ref is (ref <type index>), non-nullable.
func Ref(typeIndex int) ValType { return append(ValType{0x64}, sleb(int64(typeIndex))...) }

// RefAbstract is (ref <abstract heap type>), non-nullable.
func RefAbstract(heap byte) ValType { return ValType{0x64, heap} }

// Packed storage types for struct fields and array elements.
var (
	PackedI8  = ValType{0x78}
	PackedI16 = ValType{0x77}
)

// Field is a struct field or array element type.
type Field struct {
	Type    ValType
	Mutable bool
}

// TypeDef is one composite type: a struct, an array or a function.
type TypeDef struct {
	Name    string // for the text dump and diagnostics only
	Struct  []Field
	Array   *Field
	Params  []ValType
	Results []ValType
	IsFunc  bool
	// Standalone types come first and are each their own recursion group,
	// so they are the same type as an identical one in another module (a
	// WASI import's signature must match the host's exactly). They may use
	// only value types, not the module's own types.
	Standalone bool
}

// Import is a function imported from the host.
type Import struct {
	Module, Name string
	Type         int
}

// Global is a module global with a constant initializer.
type Global struct {
	Type    ValType
	Mutable bool
	Init    []byte // constant expression, without the final end
}

// Func is a defined function: its type, extra locals and body.
type Func struct {
	Name   string
	Type   int
	Locals []ValType
	Body   *Code
}

// Export makes a function visible to the host.
type Export struct {
	Name string
	Func int // function index (imports first)
}

// Module is a complete WebAssembly module under construction.
type Module struct {
	Types   []TypeDef
	Imports []Import
	Funcs   []Func
	Globals []Global
	Exports []Export
	Datas   [][]byte // passive data segments (string literal bytes)
	Start   int      // function index, or -1
	// MemoryPages, when above zero, gives the module one linear memory of
	// that many 64 KiB pages, exported as MemoryExport (if named). Values
	// never live there; a WASI host reads output from it.
	MemoryPages  int
	MemoryExport string
}

// NewModule returns an empty module with no start function.
func NewModule() *Module { return &Module{Start: -1} }

// AddType appends a type and returns its index.
func (m *Module) AddType(t TypeDef) int { m.Types = append(m.Types, t); return len(m.Types) - 1 }

// AddImport appends a function import and returns its function index.
// Imports must be added before any defined function.
func (m *Module) AddImport(module, name string, typeIndex int) int {
	if len(m.Funcs) > 0 {
		panic("wasmgc: imports must precede defined functions")
	}
	m.Imports = append(m.Imports, Import{module, name, typeIndex})
	return len(m.Imports) - 1
}

// AddFunc appends a defined function and returns its function index.
func (m *Module) AddFunc(f Func) int {
	m.Funcs = append(m.Funcs, f)
	return len(m.Imports) + len(m.Funcs) - 1
}

// AddGlobal appends a global and returns its index.
func (m *Module) AddGlobal(g Global) int { m.Globals = append(m.Globals, g); return len(m.Globals) - 1 }

// AddData appends a passive data segment and returns its index.
func (m *Module) AddData(b []byte) int { m.Datas = append(m.Datas, b); return len(m.Datas) - 1 }

// Bytes encodes the module in the binary format.
func (m *Module) Bytes() ([]byte, error) {
	var out bytes.Buffer
	out.Write([]byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00})

	// Type section: the standalone types, then one recursion group, so every
	// other type can refer to any other.
	if len(m.Types) > 0 {
		lead := 0
		for lead < len(m.Types) && m.Types[lead].Standalone {
			lead++
		}
		for i, t := range m.Types[lead:] {
			if t.Standalone {
				return nil, fmt.Errorf("type %d (%s): standalone types must come first", lead+i, t.Name)
			}
		}
		var s bytes.Buffer
		groups := lead
		if lead < len(m.Types) {
			groups++
		}
		s.Write(uleb(uint64(groups)))
		for i, t := range m.Types[:lead] {
			if err := encodeType(&s, t); err != nil {
				return nil, fmt.Errorf("type %d (%s): %w", i, t.Name, err)
			}
		}
		if lead < len(m.Types) {
			s.WriteByte(0x4E)
			s.Write(uleb(uint64(len(m.Types) - lead)))
			for i, t := range m.Types[lead:] {
				if err := encodeType(&s, t); err != nil {
					return nil, fmt.Errorf("type %d (%s): %w", lead+i, t.Name, err)
				}
			}
		}
		section(&out, 1, s.Bytes())
	}
	if len(m.Imports) > 0 {
		var s bytes.Buffer
		s.Write(uleb(uint64(len(m.Imports))))
		for _, im := range m.Imports {
			name(&s, im.Module)
			name(&s, im.Name)
			s.WriteByte(0x00)
			s.Write(uleb(uint64(im.Type)))
		}
		section(&out, 2, s.Bytes())
	}
	if len(m.Funcs) > 0 {
		var s bytes.Buffer
		s.Write(uleb(uint64(len(m.Funcs))))
		for _, f := range m.Funcs {
			s.Write(uleb(uint64(f.Type)))
		}
		section(&out, 3, s.Bytes())
	}
	if m.MemoryPages > 0 {
		var s bytes.Buffer
		s.Write(uleb(1))
		s.WriteByte(0x00) // minimum only
		s.Write(uleb(uint64(m.MemoryPages)))
		section(&out, 5, s.Bytes())
	}
	if len(m.Globals) > 0 {
		var s bytes.Buffer
		s.Write(uleb(uint64(len(m.Globals))))
		for _, g := range m.Globals {
			s.Write(g.Type)
			if g.Mutable {
				s.WriteByte(0x01)
			} else {
				s.WriteByte(0x00)
			}
			s.Write(g.Init)
			s.WriteByte(0x0B)
		}
		section(&out, 6, s.Bytes())
	}
	if len(m.Exports) > 0 || m.MemoryExport != "" {
		var s bytes.Buffer
		n := len(m.Exports)
		if m.MemoryExport != "" {
			n++
		}
		s.Write(uleb(uint64(n)))
		for _, e := range m.Exports {
			name(&s, e.Name)
			s.WriteByte(0x00)
			s.Write(uleb(uint64(e.Func)))
		}
		if m.MemoryExport != "" {
			name(&s, m.MemoryExport)
			s.WriteByte(0x02)
			s.Write(uleb(0))
		}
		section(&out, 7, s.Bytes())
	}
	if m.Start >= 0 {
		section(&out, 8, uleb(uint64(m.Start)))
	}
	// Functions referenced by ref.func must be declared in an element
	// segment; one declarative segment lists every defined function.
	if len(m.Funcs) > 0 {
		var s bytes.Buffer
		s.Write(uleb(1))
		s.Write(uleb(3)) // declarative, function indices
		s.WriteByte(0x00)
		s.Write(uleb(uint64(len(m.Funcs))))
		for i := range m.Funcs {
			s.Write(uleb(uint64(len(m.Imports) + i)))
		}
		section(&out, 9, s.Bytes())
	}
	if len(m.Datas) > 0 {
		section(&out, 12, uleb(uint64(len(m.Datas))))
	}
	if len(m.Funcs) > 0 {
		var s bytes.Buffer
		s.Write(uleb(uint64(len(m.Funcs))))
		for i, f := range m.Funcs {
			body, err := f.encode()
			if err != nil {
				return nil, fmt.Errorf("function %d (%s): %w", i, f.Name, err)
			}
			s.Write(uleb(uint64(len(body))))
			s.Write(body)
		}
		section(&out, 10, s.Bytes())
	}
	if len(m.Datas) > 0 {
		var s bytes.Buffer
		s.Write(uleb(uint64(len(m.Datas))))
		for _, d := range m.Datas {
			s.Write(uleb(1)) // passive
			s.Write(uleb(uint64(len(d))))
			s.Write(d)
		}
		section(&out, 11, s.Bytes())
	}
	return out.Bytes(), nil
}

func encodeType(s *bytes.Buffer, t TypeDef) error {
	switch {
	case t.IsFunc:
		s.WriteByte(0x60)
		s.Write(uleb(uint64(len(t.Params))))
		for _, p := range t.Params {
			s.Write(p)
		}
		s.Write(uleb(uint64(len(t.Results))))
		for _, r := range t.Results {
			s.Write(r)
		}
	case t.Array != nil:
		s.WriteByte(0x5E)
		field(s, *t.Array)
	case t.Struct != nil:
		s.WriteByte(0x5F)
		s.Write(uleb(uint64(len(t.Struct))))
		for _, f := range t.Struct {
			field(s, f)
		}
	default:
		return fmt.Errorf("type is neither a function, an array nor a struct")
	}
	return nil
}

func field(s *bytes.Buffer, f Field) {
	s.Write(f.Type)
	if f.Mutable {
		s.WriteByte(0x01)
	} else {
		s.WriteByte(0x00)
	}
}

func (f Func) encode() ([]byte, error) {
	if f.Body == nil {
		return nil, fmt.Errorf("no body")
	}
	if f.Body.depth != 0 {
		return nil, fmt.Errorf("unbalanced blocks (%d open)", f.Body.depth)
	}
	var s bytes.Buffer
	// Locals, one entry per local (runs are not merged; simple and valid).
	s.Write(uleb(uint64(len(f.Locals))))
	for _, l := range f.Locals {
		s.Write(uleb(1))
		s.Write(l)
	}
	s.Write(f.Body.buf.Bytes())
	s.WriteByte(0x0B)
	return s.Bytes(), nil
}

func section(out *bytes.Buffer, id byte, payload []byte) {
	out.WriteByte(id)
	out.Write(uleb(uint64(len(payload))))
	out.Write(payload)
}

func name(s *bytes.Buffer, n string) {
	s.Write(uleb(uint64(len(n))))
	s.WriteString(n)
}

func uleb(v uint64) []byte {
	var out []byte
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if v != 0 {
			out = append(out, b|0x80)
		} else {
			return append(out, b)
		}
	}
}

func sleb(v int64) []byte {
	var out []byte
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if (v == 0 && b&0x40 == 0) || (v == -1 && b&0x40 != 0) {
			return append(out, b)
		}
		out = append(out, b|0x80)
	}
}

func f64bytes(v float64) []byte {
	bits := math.Float64bits(v)
	out := make([]byte, 8)
	for i := 0; i < 8; i++ {
		out[i] = byte(bits >> (8 * i))
	}
	return out
}
