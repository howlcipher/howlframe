package hfir

import (
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestLowerToBytecodeEmitsDefunAndCall(t *testing.T) {
	graph := lowerChecked(t, `(cli_app
  (defun add (a b)
    (type_hints (a int) (b int) (return int))
    (return (+ a b)))
  (print (call add 2 3)))`)
	for _, node := range graph.Nodes {
		if len(node.ControlEdges) != 0 {
			t.Fatalf("node %s kind %s has control edges; this fixture has no while", node.ID, node.Kind)
		}
		if node.Kind == "type_hints" || node.Kind == "type_hint" || node.Kind == "while" {
			t.Fatalf("erased or deferred kind %s was lowered", node.Kind)
		}
	}
	var defun, call *Node
	for _, node := range graph.Nodes {
		switch node.Kind {
		case "defun":
			defun = node
		case "call":
			call = node
		}
	}
	if defun == nil || defun.Value != "add" {
		t.Fatalf("defun node = %#v", defun)
	}
	var params []string
	var bodies int
	for _, edge := range defun.DataInputs {
		switch edge.Name {
		case "param":
			params = append(params, graph.NodeByID(edge.SourceNode).Value)
		case "body":
			bodies++
		default:
			t.Fatalf("defun edge %q", edge.Name)
		}
	}
	if len(params) != 2 || params[0] != "a" || params[1] != "b" || bodies != 1 {
		t.Fatalf("defun edges params=%v bodies=%d", params, bodies)
	}
	if call == nil || call.Value != "add" || len(call.DataInputs) != 2 {
		t.Fatalf("call node = %#v", call)
	}

	program, diags := LowerToBytecode(graph)
	if len(diags) != 0 || program == nil {
		t.Fatalf("LowerToBytecode() program=%v diags=%#v", program != nil, diags)
	}
	fn := program.Functions["add"]
	if fn == nil || len(fn.Params) != 2 || fn.Params[0] != "a" || fn.Params[1] != "b" {
		t.Fatalf("function = %#v", fn)
	}
	if !hasOp(fn.Instructions, bytecode.OpReturn) || !hasOp(program.Main, bytecode.OpCall) {
		t.Fatalf("missing CALL or RETURN\nmain=%#v\nfn=%#v", program.Main, fn.Instructions)
	}
	if err := bytecode.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
}

func TestLowerToBytecodeTypedParamDefun(t *testing.T) {
	graph := lowerChecked(t, `(cli_app
  (defun add ((a int) (b int)) int
    (return (+ a b)))
  (print (call add 1 2)))`)
	for _, node := range graph.Nodes {
		if node.Kind == "symbol" && node.Value == "int" {
			t.Fatalf("return-type symbol was lowered: %#v", node)
		}
	}
	program, diags := LowerToBytecode(graph)
	if len(diags) != 0 {
		t.Fatalf("diags = %#v", diags)
	}
	fn := program.Functions["add"]
	if fn == nil || len(fn.Params) != 2 || fn.Params[0] != "a" || fn.Params[1] != "b" {
		t.Fatalf("function = %#v", fn)
	}
}

func TestLowerToBytecodeDefunFromGraphWithoutAST(t *testing.T) {
	graph := NewGraph()
	paramA := graph.AddNode(&Node{Kind: "param", Value: "a"})
	left := graph.AddNode(&Node{Kind: "symbol", Value: "a"})
	right := graph.AddNode(&Node{Kind: "const", LiteralKind: "INT", Value: "1"})
	sum := graph.AddNode(&Node{Kind: "binary", Value: "+", DataInputs: []DataEdge{
		{Name: "left", SourceNode: left},
		{Name: "right", SourceNode: right},
	}})
	ret := graph.AddNode(&Node{Kind: "return", DataInputs: []DataEdge{{Name: "value", SourceNode: sum}}})
	fn := graph.AddNode(&Node{Kind: "defun", Value: "inc", DataInputs: []DataEdge{
		{Name: "param", SourceNode: paramA},
		{Name: "body", SourceNode: ret},
	}})
	arg := graph.AddNode(&Node{Kind: "const", LiteralKind: "INT", Value: "4"})
	call := graph.AddNode(&Node{Kind: "call", Value: "inc", DataInputs: []DataEdge{{Name: "arg", SourceNode: arg}}})
	printNode := graph.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{Name: "value", SourceNode: call}}})
	graph.EntryNode = graph.AddNode(&Node{Kind: "program", DataInputs: []DataEdge{
		{Name: "body", SourceNode: fn},
		{Name: "body", SourceNode: printNode},
	}})

	program, diags := LowerToBytecode(graph)
	if len(diags) != 0 || program == nil {
		t.Fatalf("LowerToBytecode() diags=%#v", diags)
	}
	if program.Functions["inc"] == nil || !hasOp(program.Main, bytecode.OpCall) {
		t.Fatalf("program = %#v", program)
	}
}

func TestLowerToBytecodeStillRejectsLambda(t *testing.T) {
	for _, source := range []string{
		`(cli_app (lambda (x) (print x)))`,
	} {
		root := parser.NewParser(lexer.NewLexer(source), "deferred.howl").ParseExpression()
		graph, err := LowerAST(root, "deferred.howl")
		if err != nil {
			t.Fatalf("LowerAST(%q) error = %v", source, err)
		}
		program, diags := LowerToBytecode(graph)
		if program != nil {
			t.Fatalf("LowerToBytecode(%q) returned a program", source)
		}
		if len(diags) == 0 || diags[0].Code != BytecodeUnsupportedCode {
			t.Fatalf("LowerToBytecode(%q) diags = %#v", source, diags)
		}
	}
}

func TestASTBytecodeStillCompilesWhile(t *testing.T) {
	source := `(cli_app (let (n 0) (while (< n 1) (do (set n (+ n 1)) (print n)))))`
	root := parser.NewParser(lexer.NewLexer(source), "while.howl").ParseExpression()
	checker.Check(root)
	graph, err := LowerAST(root, "while.howl")
	if err != nil {
		t.Fatal(err)
	}
	program, diags := LowerToBytecode(graph)
	if program == nil || len(diags) != 0 {
		t.Fatalf("experimental while = (%v, %#v)", program != nil, diags)
	}
	astProgram := bytecode.CompileToBytecode(root)
	if astProgram == nil || len(astProgram.Main) == 0 {
		t.Fatal("production AST bytecode compiler rejected while")
	}
}

func lowerChecked(t *testing.T, source string) *Graph {
	t.Helper()
	root := parser.NewParser(lexer.NewLexer(source), "defun_call.howl").ParseExpression()
	checker.Check(root)
	graph, err := LowerAST(root, "defun_call.howl")
	if err != nil {
		t.Fatalf("LowerAST: %v", err)
	}
	return graph
}

func hasOp(insts []bytecode.BCInstruction, op bytecode.Opcode) bool {
	for _, inst := range insts {
		if inst.Op == op {
			return true
		}
	}
	return false
}
