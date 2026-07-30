package wasm

import (
	"testing"
	"zero/internal/ast"
)

func TestDescribeLayoutUsesTypedAggregateRepresentations(t *testing.T) {
	structType := ast.Layout(ast.Struct)
	structType.Name = "User"
	structType.Size = 24
	structType.Align = 8

	tests := []struct {
		name     string
		info     ast.TypeInfo
		value    string
		size     uint64
		align    uint64
		indirect bool
	}{
		{name: "int", info: ast.Layout(ast.Int), value: "i64", size: 8, align: 8},
		{name: "float", info: ast.Layout(ast.Float), value: "f64", size: 8, align: 8},
		{name: "list", info: ast.Layout(ast.List), value: "i32", size: 24, align: 8, indirect: true},
		{name: "struct", info: structType, value: "i32", size: 24, align: 8, indirect: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			layout := describeLayout(test.info)
			if layout.ValueType != test.value || layout.Size != test.size || layout.Align != test.align || layout.Indirect != test.indirect {
				t.Fatalf("unexpected layout: %+v", layout)
			}
		})
	}
}
