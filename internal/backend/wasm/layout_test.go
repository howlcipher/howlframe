package wasm

import (
	"github.com/howlcipher/howlframe/internal/ast"
	"testing"
)

func TestDescribeLayoutUsesTypedAggregateRepresentations(t *testing.T) {
	structType := ast.Layout(ast.Struct)
	structType.Name = "User"
	structType.Size = 24
	structType.Align = 8
	structType.FieldOffsets = map[string]uint64{"name": 0, "Name": 0, "age": 16, "Age": 16}

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
	structLayout := describeLayout(structType)
	if structLayout.FieldOffsets["name"] != 0 || structLayout.FieldOffsets["age"] != 16 {
		t.Fatalf("aggregate field offsets were not preserved: %+v", structLayout)
	}
}

func TestAggregatePayloadLayoutGrowsPastReservedTable(t *testing.T) {
	if got := aggregatePayloadStart(8 + 8*40); got != 328 {
		t.Fatalf("dictionary payload overlaps its table: got %d, want 328", got)
	}

	element := ast.Layout(ast.String)
	largeList := &ast.Node{
		Type: "List",
		Children: []*ast.Node{
			{Type: "SYMBOL", Value: "list"},
			{Type: "STRING", Value: string(make([]byte, int(wasmPage)))},
		},
		Inferred: ast.TypeInfo{Kind: ast.List, Element: &element},
	}
	generator := newWasmGenerator(largeList)
	if generator.memorySize <= wasmPage {
		t.Fatalf("large aggregate string payload must require more than one page, got %d bytes", generator.memorySize)
	}
}
