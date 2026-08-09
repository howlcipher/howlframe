package wasm

import (
	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/ir"
	"strings"
	"testing"
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
		{"float add", serializerList("+", serializerFloat("1.5"), serializerFloat("2.5")), "f64.add", "f64"},
		{"float greater", serializerList(">", serializerFloat("1.5"), serializerFloat("2.5")), "f64.gt", "i32"},
		{"float equal", serializerList("=", serializerFloat("1.5"), serializerFloat("2.5")), "f64.eq", "i32"},
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
				Result: ir.Value{ID: 1, Type: ast.Layout(ast.List)},
				Op:     ir.OpList,
			}},
			Terminator: &ir.Terminator{Kind: ir.TermReturn, Value: 1},
		}},
	}
	_, err := SerializeSSA(graph)
	if err == nil || !strings.Contains(err.Error(), `does not support operation "list"`) {
		t.Fatalf("SerializeSSA() error = %v, want clear unsupported operation diagnostic", err)
	}
}

func TestSerializeSSARejectsCallToUndefinedFunction(t *testing.T) {
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
	if err == nil || !strings.Contains(err.Error(), `does not support calling undefined function "work"`) {
		t.Fatalf("SerializeSSA() error = %v, want clear undefined function diagnostic", err)
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

// TestSerializeSSAWhileLoopAccumulator tests that a simple counting while loop
// (let (i 0) (do (while (< i 5) (set i (+ i 1))) (return i)))
// lowers to valid WAT containing loop, local.set, and local.get instructions.
func TestSerializeSSAWhileLoopAccumulator(t *testing.T) {
	// Build: (let (i 0) (do (while (< i 5) (set i (+ i 1))) (return i)))
	root := serializerList("let",
		serializerPair(serializerSymbol("i"), serializerInt("0")),
		serializerList("do",
			serializerList("while",
				serializerList("<", serializerSymbol("i"), serializerInt("5")),
				serializerList("set", serializerSymbol("i"),
					serializerList("+", serializerSymbol("i"), serializerInt("1"))),
			),
			serializerList("return", serializerSymbol("i")),
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
	wat, err := SerializeSSA(graph)
	if err != nil {
		t.Fatalf("SerializeSSA() error = %v", err)
	}
	for _, fragment := range []string{
		`(func (export "main") (result i64)`,
		`(loop`,
		`(local.set `,
		`(local.get `,
		`(br_if `,
		`(i32.eqz `,
		`(br 0)`,
	} {
		if !strings.Contains(wat, fragment) {
			t.Errorf("serialized WAT is missing %q:\n%s", fragment, wat)
		}
	}
	if err := ValidateWAT(wat); err != nil {
		t.Fatalf("ValidateWAT() error = %v\n%s", err, wat)
	}
}

func serializerList(head string, children ...*ast.Node) *ast.Node {
	return &ast.Node{
		Type:     "List",
		Line:     1,
		Column:   1,
		Filename: "serializer.howl",
		Children: append([]*ast.Node{serializerSymbol(head)}, children...),
	}
}

func serializerPair(left, right *ast.Node) *ast.Node {
	return &ast.Node{
		Type:     "List",
		Line:     1,
		Column:   1,
		Filename: "serializer.howl",
		Children: []*ast.Node{left, right},
	}
}

func serializerSymbol(value string) *ast.Node {
	return &ast.Node{Type: "SYMBOL", Value: value, Line: 1, Column: 1, Filename: "serializer.howl"}
}

func serializerInt(value string) *ast.Node {
	return &ast.Node{Type: "INT", Value: value, Line: 1, Column: 1, Filename: "serializer.howl"}
}

func serializerFloat(value string) *ast.Node {
	return &ast.Node{Type: "FLOAT", Value: value, Line: 1, Column: 1, Filename: "serializer.howl"}
}

func serializerBool(value bool) *ast.Node {
	if value {
		return serializerSymbol("true")
	}
	return serializerSymbol("false")
}

func serializerString(value string) *ast.Node {
	return &ast.Node{Type: "STRING", Value: value, Line: 1, Column: 1, Filename: "serializer.howl"}
}

// TestSerializeSSAProgramCallsHelperFunction builds the equivalent of
//
//	(cli_app
//	  (defun square (x) (type_hint x "int") (type_hint return "int") (* x x))
//	  (call square 3))
//
// through the same checker → ir.LowerSSAFunction/ir.LowerSSA → SerializeSSAProgram
// path howlframe.go's -compile-wasm flag uses, and checks the callee is emitted
// as its own WAT function and the call site resolves to it.
func TestSerializeSSAProgramCallsHelperFunction(t *testing.T) {
	defunNode := serializerList("defun",
		serializerSymbol("square"),
		serializerList("x"),
		serializerList("type_hint", serializerSymbol("x"), serializerString("int")),
		serializerList("type_hint", serializerSymbol("return"), serializerString("int")),
		serializerList("*", serializerSymbol("x"), serializerSymbol("x")),
	)
	entryNode := serializerList("call", serializerSymbol("square"), serializerInt("3"))
	root := serializerList("cli_app", defunNode, entryNode)

	analysis := checker.Analyze(root)
	if len(analysis.Diagnostics) != 0 {
		t.Fatalf("Analyze() diagnostics = %+v", analysis.Diagnostics)
	}
	info, ok := analysis.Functions["square"]
	if !ok {
		t.Fatalf("Analyze() did not collect a signature for square")
	}

	params := []ir.Param{{Name: "x", Type: info.Params[0]}}
	functionGraph, err := ir.LowerSSAFunction(params, defunNode.Children[len(defunNode.Children)-1])
	if err != nil {
		t.Fatalf("LowerSSAFunction() error = %v", err)
	}
	entryGraph, err := ir.LowerSSA(entryNode)
	if err != nil {
		t.Fatalf("LowerSSA() error = %v", err)
	}

	wat, err := SerializeSSAProgram([]Function{{
		Name:   "square",
		Params: params,
		Return: info.Return,
		Graph:  functionGraph,
	}}, entryGraph)
	if err != nil {
		t.Fatalf("SerializeSSAProgram() error = %v", err)
	}
	for _, fragment := range []string{
		`(func $square (param $x i64) (result i64)`,
		`(i64.mul (local.get $x) (local.get $x))`,
		`(func $main (export "main") (result i64)`,
		`(call $square (i64.const 3))`,
	} {
		if !strings.Contains(wat, fragment) {
			t.Errorf("serialized WAT is missing %q:\n%s", fragment, wat)
		}
	}
	if err := ValidateWAT(wat); err != nil {
		t.Fatalf("ValidateWAT() error = %v\n%s", err, wat)
	}
}

// TestSerializeSSAProgramWithNoFunctionsMatchesSerializeSSA proves
// SerializeSSAProgram degrades to byte-identical output to SerializeSSA when
// there are no functions, so -compile-wasm's existing single-expression
// fixtures are unaffected by this feature.
func TestSerializeSSAProgramWithNoFunctionsMatchesSerializeSSA(t *testing.T) {
	root := serializerList("+", serializerInt("1"), serializerInt("2"))
	checker.Analyze(serializerList("cli_app", root))
	graph, err := ir.LowerSSA(root)
	if err != nil {
		t.Fatalf("LowerSSA() error = %v", err)
	}
	direct, err := SerializeSSA(graph)
	if err != nil {
		t.Fatalf("SerializeSSA() error = %v", err)
	}
	viaProgram, err := SerializeSSAProgram(nil, graph)
	if err != nil {
		t.Fatalf("SerializeSSAProgram() error = %v", err)
	}
	if direct != viaProgram {
		t.Fatalf("SerializeSSAProgram(nil, graph) = %q, want byte-identical to SerializeSSA(graph) = %q", viaProgram, direct)
	}
}

func TestSerializeSSAProgramRejectsNonScalarParameter(t *testing.T) {
	params := []ir.Param{{Name: "s", Type: ast.Layout(ast.String)}}
	body := serializerSymbol("s")
	functionGraph, err := ir.LowerSSAFunction(params, body)
	if err != nil {
		t.Fatalf("LowerSSAFunction() error = %v", err)
	}
	entryGraph, err := ir.LowerSSA(serializerInt("1"))
	if err != nil {
		t.Fatalf("LowerSSA() error = %v", err)
	}
	_, err = SerializeSSAProgram([]Function{{
		Name:   "identity",
		Params: params,
		Return: ast.Layout(ast.String),
		Graph:  functionGraph,
	}}, entryGraph)
	if err == nil || !strings.Contains(err.Error(), "unsupported primitive type") {
		t.Fatalf("SerializeSSAProgram() error = %v, want a clear non-scalar rejection", err)
	}
}

func TestSerializeSSAProgramRejectsDuplicateFunctionName(t *testing.T) {
	graphA, err := ir.LowerSSAFunction(nil, serializerInt("1"))
	if err != nil {
		t.Fatalf("LowerSSAFunction() error = %v", err)
	}
	graphB, err := ir.LowerSSAFunction(nil, serializerInt("2"))
	if err != nil {
		t.Fatalf("LowerSSAFunction() error = %v", err)
	}
	entryGraph, err := ir.LowerSSA(serializerInt("0"))
	if err != nil {
		t.Fatalf("LowerSSA() error = %v", err)
	}
	_, err = SerializeSSAProgram([]Function{
		{Name: "dup", Return: ast.Layout(ast.Int), Graph: graphA},
		{Name: "dup", Return: ast.Layout(ast.Int), Graph: graphB},
	}, entryGraph)
	if err == nil || !strings.Contains(err.Error(), `duplicate function "dup"`) {
		t.Fatalf("SerializeSSAProgram() error = %v, want a clear duplicate function diagnostic", err)
	}
}
