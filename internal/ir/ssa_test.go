package ir

import (
	"github.com/howlcipher/howlframe/internal/ast"
	"strings"
	"testing"
)

func TestLowerSSAStraightLine(t *testing.T) {
	root := ssaList("do",
		ssaList("+", ssaInt("1"), ssaInt("2")),
		ssaList("print", ssaString("done")),
	)

	graph, err := LowerSSA(root)
	if err != nil {
		t.Fatalf("LowerSSA() error = %v", err)
	}
	if err := graph.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(graph.Blocks) != 1 {
		t.Fatalf("block count = %d, want 1", len(graph.Blocks))
	}
	block := graph.Blocks[0]
	gotOps := instructionOps(block)
	wantOps := []SSAOp{OpConst, OpConst, OpAdd, OpConst, OpPrint}
	if strings.Join(ssaOpStrings(gotOps), ",") != strings.Join(ssaOpStrings(wantOps), ",") {
		t.Fatalf("instruction ops = %v, want %v", gotOps, wantOps)
	}
	if block.Terminator == nil || block.Terminator.Kind != TermReturn {
		t.Fatalf("terminator = %+v, want return", block.Terminator)
	}
	if block.Instructions[2].Result.Type.Kind != ast.Int {
		t.Fatalf("binary result type = %+v, want int", block.Instructions[2].Result.Type)
	}
}

func TestLowerSSAIfBuildsBranchAndMerge(t *testing.T) {
	root := ssaList("if",
		ssaBool(true),
		ssaList("+", ssaInt("1"), ssaInt("2")),
		ssaList("+", ssaInt("3"), ssaInt("4")),
	)

	graph, err := LowerSSA(root)
	if err != nil {
		t.Fatalf("LowerSSA() error = %v", err)
	}
	if err := graph.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(graph.Blocks) != 4 {
		t.Fatalf("block count = %d, want 4", len(graph.Blocks))
	}
	entry := graph.Blocks[0]
	if entry.Terminator == nil || entry.Terminator.Kind != TermBranch {
		t.Fatalf("entry terminator = %+v, want branch", entry.Terminator)
	}
	merge := graph.Blocks[3]
	if len(merge.Instructions) != 1 || merge.Instructions[0].Op != OpPhi {
		t.Fatalf("merge instructions = %+v, want one phi", merge.Instructions)
	}
	phi := merge.Instructions[0]
	if len(phi.Operands) != 2 || len(phi.Blocks) != 2 {
		t.Fatalf("phi = %+v, want two incoming values and blocks", phi)
	}
	if merge.Terminator == nil || merge.Terminator.Value != phi.Result.ID {
		t.Fatalf("merge terminator = %+v, want return of phi %d", merge.Terminator, phi.Result.ID)
	}
}

func TestLowerSSAWhileBuildsLoopCarriedPhi(t *testing.T) {
	root := ssaList("let",
		ssaPair(ssaSymbol("x"), ssaInt("0")),
		ssaList("do",
			ssaList("while",
				ssaList("<", ssaSymbol("x"), ssaInt("3")),
				ssaList("set", ssaSymbol("x"),
					ssaList("+", ssaSymbol("x"), ssaInt("1")),
				),
			),
			ssaSymbol("x"),
		),
	)

	graph, err := LowerSSA(root)
	if err != nil {
		t.Fatalf("LowerSSA() error = %v", err)
	}
	if err := graph.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(graph.Blocks) != 4 {
		t.Fatalf("block count = %d, want 4", len(graph.Blocks))
	}
	header := graph.Blocks[1]
	if len(header.Instructions) == 0 || header.Instructions[0].Op != OpPhi {
		t.Fatalf("header instructions = %+v, want leading loop phi", header.Instructions)
	}
	if header.Terminator == nil || header.Terminator.Kind != TermBranch {
		t.Fatalf("header terminator = %+v, want branch", header.Terminator)
	}
	body := graph.Blocks[2]
	if body.Terminator == nil || body.Terminator.Kind != TermJump || body.Terminator.Target != header.Label {
		t.Fatalf("body terminator = %+v, want jump to %q", body.Terminator, header.Label)
	}
	exit := graph.Blocks[3]
	if exit.Terminator == nil || exit.Terminator.Kind != TermReturn {
		t.Fatalf("exit terminator = %+v, want return", exit.Terminator)
	}
	if exit.Terminator.Value != header.Instructions[0].Result.ID {
		t.Fatalf("return value = %d, want loop phi %d", exit.Terminator.Value, header.Instructions[0].Result.ID)
	}
}

func TestLowerSSALetChainPreservesValueFlow(t *testing.T) {
	root := ssaList("let",
		ssaPair(ssaSymbol("x"), ssaInt("1")),
		ssaList("let",
			ssaPair(ssaSymbol("y"), ssaList("+", ssaSymbol("x"), ssaInt("2"))),
			ssaList("+", ssaSymbol("y"), ssaSymbol("x")),
		),
	)

	graph, err := LowerSSA(root)
	if err != nil {
		t.Fatalf("LowerSSA() error = %v", err)
	}
	block := graph.Blocks[0]
	if len(block.Instructions) != 4 {
		t.Fatalf("instruction count = %d, want 4", len(block.Instructions))
	}
	x := block.Instructions[0].Result.ID
	y := block.Instructions[2].Result.ID
	final := block.Instructions[3]
	if final.Op != OpAdd || len(final.Operands) != 2 || final.Operands[0] != y || final.Operands[1] != x {
		t.Fatalf("final instruction = %+v, want operands y=%d and x=%d", final, y, x)
	}
}

func TestLowerSSASupportedDataCallsAndConversions(t *testing.T) {
	list := ssaList("list", ssaInt("1"), ssaInt("2"))
	list.Inferred = ast.TypeInfo{Kind: ast.List}
	dict := ssaList("dict", ssaPair(ssaString("key"), ssaString("value")))
	dict.Inferred = ast.TypeInfo{Kind: ast.Dict}
	root := ssaList("let",
		ssaPair(ssaSymbol("items"), list),
		ssaList("let",
			ssaPair(ssaSymbol("values"), dict),
			ssaList("do",
				ssaList("call", ssaSymbol("consume"), ssaSymbol("items")),
				ssaList("list_get", ssaSymbol("items"), ssaInt("0")),
				ssaList("map_get", ssaSymbol("values"), ssaString("key")),
				ssaList("to_int", ssaInt("1")),
				ssaList("to_float", ssaInt("1")),
				ssaList("to_string", ssaInt("1")),
				ssaList("bytes_to_string", ssaSymbol("raw")),
			),
		),
	)

	graph, err := LowerSSA(root)
	if err != nil {
		t.Fatalf("LowerSSA() error = %v", err)
	}
	if err := graph.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	gotOps := instructionOps(graph.Blocks[0])
	for _, want := range []SSAOp{
		OpList, OpDict, OpCall, OpListGet, OpMapGet,
		OpToInt, OpToFloat, OpToString, OpBytesToString,
	} {
		if !containsSSAOp(gotOps, want) {
			t.Errorf("instruction ops = %v, missing %q", gotOps, want)
		}
	}
	call := findSSAInstruction(graph.Blocks[0], OpCall)
	if call == nil || call.Symbol != "consume" || len(call.Operands) != 1 {
		t.Errorf("call instruction = %+v, want consume with one operand", call)
	}
}

func TestGraphValidateRejectsMalformedGraphs(t *testing.T) {
	validReturn := &Terminator{Kind: TermReturn}
	tests := []struct {
		name  string
		graph *Graph
		want  string
	}{
		{
			name:  "missing entry",
			graph: &Graph{Entry: "missing", Blocks: []*BasicBlock{{Label: "entry", Terminator: validReturn}}},
			want:  `entry block "missing" does not exist`,
		},
		{
			name: "duplicate block",
			graph: &Graph{Entry: "entry", Blocks: []*BasicBlock{
				{Label: "entry", Terminator: validReturn},
				{Label: "entry", Terminator: validReturn},
			}},
			want: `duplicate block label "entry"`,
		},
		{
			name: "duplicate value",
			graph: &Graph{Entry: "entry", Blocks: []*BasicBlock{{
				Label: "entry",
				Instructions: []Instruction{
					{Result: Value{ID: 1}, Op: OpConst},
					{Result: Value{ID: 1}, Op: OpConst},
				},
				Terminator: validReturn,
			}}},
			want: "duplicate SSA value %1",
		},
		{
			name: "unknown jump target",
			graph: &Graph{Entry: "entry", Blocks: []*BasicBlock{{
				Label:      "entry",
				Terminator: &Terminator{Kind: TermJump, Target: "missing"},
			}}},
			want: `jump target "missing" does not exist`,
		},
		{
			name: "undefined operand",
			graph: &Graph{Entry: "entry", Blocks: []*BasicBlock{{
				Label: "entry",
				Instructions: []Instruction{{
					Result:   Value{ID: 1},
					Op:       OpAdd,
					Operands: []ValueID{99},
				}},
				Terminator: &Terminator{Kind: TermReturn, Value: 1},
			}}},
			want: "references undefined value %99",
		},
		{
			name:  "missing terminator",
			graph: &Graph{Entry: "entry", Blocks: []*BasicBlock{{Label: "entry"}}},
			want:  `block "entry" has no terminator`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.graph.Validate()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func instructionOps(block *BasicBlock) []SSAOp {
	ops := make([]SSAOp, len(block.Instructions))
	for index, instruction := range block.Instructions {
		ops[index] = instruction.Op
	}
	return ops
}

func ssaOpStrings(ops []SSAOp) []string {
	result := make([]string, len(ops))
	for index, op := range ops {
		result[index] = string(op)
	}
	return result
}

func containsSSAOp(ops []SSAOp, want SSAOp) bool {
	for _, op := range ops {
		if op == want {
			return true
		}
	}
	return false
}

func findSSAInstruction(block *BasicBlock, op SSAOp) *Instruction {
	for index := range block.Instructions {
		if block.Instructions[index].Op == op {
			return &block.Instructions[index]
		}
	}
	return nil
}

func ssaList(head string, children ...*ast.Node) *ast.Node {
	node := &ast.Node{
		Type:     "List",
		Line:     1,
		Column:   1,
		Filename: "test.howl",
		Children: append([]*ast.Node{ssaSymbol(head)}, children...),
	}
	switch head {
	case "+", "-", "*", "/":
		node.Inferred = ast.Layout(ast.Int)
	case "<", ">", "<=", ">=", "==", "!=", "=", "and", "or":
		node.Inferred = ast.Layout(ast.Bool)
	case "if":
		node.Inferred = ast.Layout(ast.Int)
	case "while", "set", "print":
		node.Inferred = ast.Layout(ast.Void)
	}
	return node
}

func ssaPair(left, right *ast.Node) *ast.Node {
	return &ast.Node{Type: "List", Line: 1, Column: 1, Filename: "test.howl", Children: []*ast.Node{left, right}}
}

func ssaSymbol(value string) *ast.Node {
	return &ast.Node{Type: "SYMBOL", Value: value, Line: 1, Column: 1, Filename: "test.howl"}
}

func ssaBool(value bool) *ast.Node {
	node := ssaSymbol("false")
	if value {
		node.Value = "true"
	}
	node.Inferred = ast.Layout(ast.Bool)
	return node
}

func ssaInt(value string) *ast.Node {
	return &ast.Node{Type: "INT", Value: value, Line: 1, Column: 1, Filename: "test.howl", Inferred: ast.Layout(ast.Int)}
}

func ssaString(value string) *ast.Node {
	return &ast.Node{Type: "STRING", Value: value, Line: 1, Column: 1, Filename: "test.howl", Inferred: ast.Layout(ast.String)}
}
