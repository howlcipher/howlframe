package hfir

import (
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestLowerToBytecodeEmitsWhileControlEdges(t *testing.T) {
	graph := lowerCheckedWhile(t, `(cli_app
  (let (n 0)
    (while (< n 2)
      (do
        (set n (+ n 1))
        (if (< n 2)
          (print n)
          (print "last"))))))`)
	var loop *Node
	for _, node := range graph.Nodes {
		if node.Kind == "while" {
			if loop != nil {
				t.Fatal("expected one while")
			}
			loop = node
			continue
		}
		if node.Kind == "if" {
			if len(node.ControlEdges) != 3 || len(node.DataInputs) != 3 {
				t.Fatalf("if %s = %#v", node.ID, node)
			}
			for index := range node.ControlEdges {
				if node.ControlEdges[index] != node.DataInputs[index].SourceNode {
					t.Fatalf("if control %v data %#v", node.ControlEdges, node.DataInputs)
				}
			}
			continue
		}
		if node.Kind == "for" && len(node.ControlEdges) != 0 {
			t.Fatalf("for %s has control edges %v", node.ID, node.ControlEdges)
		}
	}
	if loop == nil {
		t.Fatal("missing while")
	}
	if len(loop.ControlEdges) != 2 || loop.DataInputs[0].Name != "condition" || loop.DataInputs[1].Name != "body" {
		t.Fatalf("while = %#v", loop)
	}
	if loop.ControlEdges[0] != loop.DataInputs[0].SourceNode || loop.ControlEdges[1] != loop.DataInputs[1].SourceNode {
		t.Fatalf("control %v data %#v", loop.ControlEdges, loop.DataInputs)
	}
	cond := graph.NodeByID(loop.ControlEdges[0])
	body := graph.NodeByID(loop.ControlEdges[1])
	if cond == nil || cond.Kind != "binary" || body == nil || body.Kind != "sequence" {
		t.Fatalf("successors cond=%#v body=%#v", cond, body)
	}
	relations := DerivePhase1ControlRelations(graph)
	var sawCond, sawBody bool
	for _, relation := range relations {
		if relation.Controller != loop.ID {
			continue
		}
		switch relation.Role {
		case "condition":
			sawCond = relation.Controlled == loop.ControlEdges[0] && relation.Ordinal == 0
		case "body":
			sawBody = relation.Controlled == loop.ControlEdges[1] && relation.Ordinal == 1
		}
	}
	if !sawCond || !sawBody {
		t.Fatalf("while relations = %#v", relations)
	}

	program, diags := LowerToBytecode(graph)
	if len(diags) != 0 || program == nil {
		t.Fatalf("LowerToBytecode() diags=%#v", diags)
	}
	if !hasOp(program.Main, bytecode.OpJumpIfFalse) || !hasOp(program.Main, bytecode.OpJump) {
		t.Fatalf("main = %#v", program.Main)
	}
	if err := bytecode.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
}

func TestLowerToBytecodeWhileInsideDefun(t *testing.T) {
	graph := lowerCheckedWhile(t, `(cli_app
  (defun steps (limit)
    (type_hints (limit int) (return int))
    (let (n 0)
      (do
        (while (< n limit)
          (set n (+ n 1)))
        (return n))))
  (print (call steps 4)))`)
	var loop *Node
	for _, node := range graph.Nodes {
		if node.Kind == "while" {
			loop = node
		}
		if node.Kind == "defun" && len(node.ControlEdges) != 0 {
			t.Fatalf("defun has control edges %v", node.ControlEdges)
		}
	}
	if loop == nil || len(loop.ControlEdges) != 2 {
		t.Fatalf("while = %#v", loop)
	}
	program, diags := LowerToBytecode(graph)
	if len(diags) != 0 || program == nil || program.Functions["steps"] == nil {
		t.Fatalf("program=%v diags=%#v", program != nil, diags)
	}
	fn := program.Functions["steps"].Instructions
	if !hasOp(fn, bytecode.OpJumpIfFalse) || !hasOp(fn, bytecode.OpReturn) {
		t.Fatalf("function = %#v", fn)
	}
}

func TestLowerToBytecodeWhileFromGraphWithoutAST(t *testing.T) {
	graph := NewGraph()
	cond := graph.AddNode(&Node{Kind: "const", LiteralKind: "BOOL", Value: "false"})
	text := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "no"})
	body := graph.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{Name: "value", SourceNode: text}}})
	loop := graph.AddNode(&Node{
		Kind:         "while",
		DataInputs:   []DataEdge{{Name: "condition", SourceNode: cond}, {Name: "body", SourceNode: body}},
		ControlEdges: []NodeID{cond, body},
	})
	graph.EntryNode = graph.AddNode(&Node{Kind: "program", DataInputs: []DataEdge{{Name: "body", SourceNode: loop}}})

	program, diags := LowerToBytecode(graph)
	if len(diags) != 0 || program == nil {
		t.Fatalf("LowerToBytecode() diags=%#v", diags)
	}
	if err := bytecode.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
}

func TestLowerToBytecodeRejectsWhileWithoutControlEdges(t *testing.T) {
	graph := NewGraph()
	cond := graph.AddNode(&Node{Kind: "const", LiteralKind: "BOOL", Value: "false"})
	text := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "no"})
	body := graph.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{Name: "value", SourceNode: text}}})
	loop := graph.AddNode(&Node{
		Kind:       "while",
		DataInputs: []DataEdge{{Name: "condition", SourceNode: cond}, {Name: "body", SourceNode: body}},
	})
	graph.EntryNode = graph.AddNode(&Node{Kind: "program", DataInputs: []DataEdge{{Name: "body", SourceNode: loop}}})

	program, diags := LowerToBytecode(graph)
	if program != nil || len(diags) != 1 || diags[0].Code != BytecodeUnsupportedCode || diags[0].RelatedNode != loop {
		t.Fatalf("program=%v diags=%#v", program != nil, diags)
	}
	for _, relation := range DerivePhase1ControlRelations(graph) {
		if relation.Controller == loop {
			t.Fatalf("while without control edges produced %#v", relation)
		}
	}

	swapped := NewGraph()
	cond = swapped.AddNode(&Node{Kind: "const", LiteralKind: "BOOL", Value: "false"})
	text = swapped.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "no"})
	body = swapped.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{Name: "value", SourceNode: text}}})
	swappedLoop := swapped.AddNode(&Node{
		Kind:         "while",
		DataInputs:   []DataEdge{{Name: "condition", SourceNode: cond}, {Name: "body", SourceNode: body}},
		ControlEdges: []NodeID{body, cond},
	})
	swapped.EntryNode = swapped.AddNode(&Node{Kind: "program", DataInputs: []DataEdge{{Name: "body", SourceNode: swappedLoop}}})
	program, diags = LowerToBytecode(swapped)
	if program != nil || len(diags) != 1 || diags[0].Code != BytecodeUnsupportedCode {
		t.Fatalf("swapped program=%v diags=%#v", program != nil, diags)
	}
}

func TestVerifierRequiresWhileRoles(t *testing.T) {
	graph := NewGraph()
	graph.AddNode(&Node{Kind: "while"})
	var missing int
	for _, diag := range NewVerifier(graph, TargetBytecode).Verify() {
		if diag.Code == "HFIR_MISSING_ROLE" {
			missing++
		}
	}
	if missing != 2 {
		t.Fatalf("missing-role diagnostics = %d", missing)
	}
}

func lowerCheckedWhile(t *testing.T, source string) *Graph {
	t.Helper()
	root := parser.NewParser(lexer.NewLexer(source), "while.howl").ParseExpression()
	checker.Check(root)
	graph, err := LowerAST(root, "while.howl")
	if err != nil {
		t.Fatalf("LowerAST: %v", err)
	}
	if diags := NewVerifier(graph, TargetBytecode).Verify(); len(diags) != 0 {
		t.Fatalf("Verify() = %#v", diags)
	}
	return graph
}
