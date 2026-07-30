package wasm

import (
	"strings"
	"testing"
	"zero/internal/ast"
	"zero/internal/checker"
	"zero/internal/ir"
)

func TestSerializeSSAUsesBranchBlocksAndPhi(t *testing.T) {
	root := serializerList("let",
		serializerPair(serializerSymbol("x"), serializerInt("4")),
		serializerList("if",
			serializerList(">", serializerSymbol("x"), serializerInt("2")),
			serializerList("+", serializerSymbol("x"), serializerInt("3")),
			serializerList("-", serializerSymbol("x"), serializerInt("1")),
		),
	)
	analysis := checker.Analyze(serializerList("cli_app", root))
	if len(analysis.Diagnostics) != 0 {
		t.Fatalf("Analyze() diagnostics = %+v", analysis.Diagnostics)
	}
	graph, err := ir.LowerSSA(root)
	if err != nil {
		t.Fatalf("LowerSSA() error = %v", err)
	}
	if len(graph.Blocks) != 4 || graph.Blocks[0].Terminator.Kind != ir.TermBranch {
		t.Fatalf("LowerSSA() did not produce the expected branch CFG: %+v", graph.Blocks)
	}
	merge := graph.Blocks[3]
	if len(merge.Instructions) != 1 || merge.Instructions[0].Op != ir.OpPhi {
		t.Fatalf("merge block = %+v, want one phi instruction", merge)
	}

	wat, err := SerializeSSA(graph)
	if err != nil {
		t.Fatalf("SerializeSSA() error = %v", err)
	}
	for _, fragment := range []string{
		`(func (export "main") (result i64)`,
		`(if (result i64)`,
		`(i64.gt_s (i64.const 4) (i64.const 2))`,
		`(then (i64.add (i64.const 4) (i64.const 3)))`,
		`(else (i64.sub (i64.const 4) (i64.const 1)))`,
	} {
		if !strings.Contains(wat, fragment) {
			t.Errorf("serialized WAT is missing %q:\n%s", fragment, wat)
		}
	}
	if err := ValidateWAT(wat); err != nil {
		t.Fatalf("ValidateWAT() error = %v\n%s", err, wat)
	}
}

func TestSerializeSSASupportedOperators(t *testing.T) {
	tests := []struct {
		name     string
		expr     *ast.Node
		operator string
		result   string
	}{
		{"add", serializerList("+", serializerInt("8"), serializerInt("2")), "i64.add", "i64"},
		{"subtract", serializerList("-", serializerInt("8"), serializerInt("2")), "i64.sub", "i64"},
		{"multiply", serializerList("*", serializerInt("8"), serializerInt("2")), "i64.mul", "i64"},
		{"divide", serializerList("/", serializerInt("8"), serializerInt("2")), "i64.div_s", "i64"},
		{"less", serializerList("<", serializerInt("8"), serializerInt("2")), "i64.lt_s", "i32"},
		{"greater", serializerList(">", serializerInt("8"), serializerInt("2")), "i64.gt_s", "i32"},
		{"less equal", serializerList("<=", serializerInt("8"), serializerInt("2")), "i64.le_s", "i32"},
		{"greater equal", serializerList(">=", serializerInt("8"), serializerInt("2")), "i64.ge_s", "i32"},
		{"equal", serializerList("=", serializerInt("8"), serializerInt("2")), "i64.eq", "i32"},
		{"double equal", serializerList("==", serializerInt("8"), serializerInt("2")), "i64.eq", "i32"},
		{"not equal", serializerList("!=", serializerInt("8"), serializerInt("2")), "i64.ne", "i32"},
		{"and", serializerList("and", serializerBool(true), serializerBool(false)), "i32.and", "i32"},
		{"or", serializerList("or", serializerBool(true), serializerBool(false)), "i32.or", "i32"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checker.Analyze(serializerList("cli_app", test.expr))
			graph, err := ir.LowerSSA(test.expr)
			if err != nil {
				t.Fatalf("LowerSSA() error = %v", err)
			}
			wat, err := SerializeSSA(graph)
			if err != nil {
				t.Fatalf("SerializeSSA() error = %v", err)
			}
			if !strings.Contains(wat, "(result "+test.result+")") ||
				!strings.Contains(wat, "("+test.operator+" ") {
				t.Fatalf("SerializeSSA() output does not contain %s with result %s:\n%s", test.operator, test.result, wat)
			}
		})
	}
}

func TestSerializeSSARejectsUnsupportedOperations(t *testing.T) {
	graph := &ir.Graph{
		Entry: "entry",
		Blocks: []*ir.BasicBlock{{
			Label: "entry",
			Instructions: []ir.Instruction{{
				Result: ir.Value{ID: 1, Type: ast.Layout(ast.Int)},
				Op:     ir.OpCall,
				Symbol: "work",
			}},
			Terminator: &ir.Terminator{Kind: ir.TermReturn, Value: 1},
		}},
	}
	_, err := SerializeSSA(graph)
	if err == nil || !strings.Contains(err.Error(), `does not support operation "call"`) {
		t.Fatalf("SerializeSSA() error = %v, want clear unsupported operation diagnostic", err)
	}
}

func TestSerializeSSASerializesReturnTerminatorsInBothBranches(t *testing.T) {
	root := serializerList("if",
		serializerBool(true),
		serializerList("return", serializerInt("7")),
		serializerList("return", serializerInt("9")),
	)
	checker.Analyze(serializerList("cli_app", root))
	graph, err := ir.LowerSSA(root)
	if err != nil {
		t.Fatalf("LowerSSA() error = %v", err)
	}
	wat, err := SerializeSSA(graph)
	if err != nil {
		t.Fatalf("SerializeSSA() error = %v", err)
	}
	if !strings.Contains(wat, "(then (return (i64.const 7)))") ||
		!strings.Contains(wat, "(else (return (i64.const 9)))") {
		t.Fatalf("SerializeSSA() did not preserve branch return terminators:\n%s", wat)
	}
}

func TestSerializeSSARejectsLoopsClearly(t *testing.T) {
	graph := &ir.Graph{
		Entry: "entry",
		Blocks: []*ir.BasicBlock{
			{
				Label: "entry",
				Instructions: []ir.Instruction{{
					Result:  ir.Value{ID: 1, Type: ast.Layout(ast.Bool)},
					Op:      ir.OpConst,
					Literal: "true",
				}},
				Terminator: &ir.Terminator{
					Kind:        ir.TermBranch,
					Cond:        1,
					TrueTarget:  "entry",
					FalseTarget: "exit",
				},
			},
			{
				Label: "exit",
				Instructions: []ir.Instruction{{
					Result:  ir.Value{ID: 2, Type: ast.Layout(ast.Int)},
					Op:      ir.OpConst,
					Literal: "0",
				}},
				Terminator: &ir.Terminator{Kind: ir.TermReturn, Value: 2},
			},
		},
	}
	_, err := SerializeSSA(graph)
	if err == nil || !strings.Contains(err.Error(), "does not support loops") {
		t.Fatalf("SerializeSSA() error = %v, want clear loop diagnostic", err)
	}
}

func serializerList(head string, children ...*ast.Node) *ast.Node {
	return &ast.Node{
		Type:     "List",
		Line:     1,
		Column:   1,
		Filename: "serializer.zero",
		Children: append([]*ast.Node{serializerSymbol(head)}, children...),
	}
}

func serializerPair(left, right *ast.Node) *ast.Node {
	return &ast.Node{
		Type:     "List",
		Line:     1,
		Column:   1,
		Filename: "serializer.zero",
		Children: []*ast.Node{left, right},
	}
}

func serializerSymbol(value string) *ast.Node {
	return &ast.Node{Type: "SYMBOL", Value: value, Line: 1, Column: 1, Filename: "serializer.zero"}
}

func serializerInt(value string) *ast.Node {
	return &ast.Node{Type: "INT", Value: value, Line: 1, Column: 1, Filename: "serializer.zero"}
}

func serializerBool(value bool) *ast.Node {
	if value {
		return serializerSymbol("true")
	}
	return serializerSymbol("false")
}
