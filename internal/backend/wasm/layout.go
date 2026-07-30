package wasm

import "zero/internal/ast"

// NativeLayout is the representation selected by the Wasm backend for a
// typed Zero value. Aggregate values are represented by a linear-memory
// pointer until Wasm memory access/code generation is added for them.
type NativeLayout struct {
	ValueType    string
	Size         uint64
	Align        uint64
	Indirect     bool
	FieldOffsets map[string]uint64
}

func describeLayout(info ast.TypeInfo) NativeLayout {
	switch info.Kind {
	case ast.Int:
		return NativeLayout{ValueType: "i64", Size: info.Size, Align: info.Align}
	case ast.Float:
		return NativeLayout{ValueType: "f64", Size: info.Size, Align: info.Align}
	case ast.Bool:
		return NativeLayout{ValueType: "i32", Size: info.Size, Align: info.Align}
	case ast.Bytes, ast.List, ast.Dict, ast.Struct:
		return NativeLayout{ValueType: "i32", Size: info.Size, Align: info.Align, Indirect: true, FieldOffsets: info.FieldOffsets}
	default:
		return NativeLayout{ValueType: "i32", Size: info.Size, Align: info.Align}
	}
}
