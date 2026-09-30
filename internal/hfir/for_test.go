package hfir

import (
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestLowerToBytecodeEmitsForControlEdges(t *testing.T) {
	graph := lowerCheckedFor(t, `(cli_app
  (for item (list "a" "b")
    (if (= item "a")
      (print item)
      (print "other"))))`)
	var loop *Node
	for _, node := range graph.Nodes {
		switch node.Kind {
		case "for":
			if loop != nil {
				t.Fatal("expected one for")
			}
			loop = node
		case "if":
			if len(node.ControlEdges) != 3 || len(node.DataInputs) != 3 {
				t.Fatalf("if %s = %#v", node.ID, node)
			}
			for index := range node.ControlEdges {
				if node.ControlEdges[index] != node.DataInputs[index].SourceNode {
					t.Fatalf("if control %v data %#v", node.ControlEdges, node.DataInputs)
				}
			}
		case "while", "defun":
			t.Fatalf("unexpected %s", node.Kind)
		}
	}
	if loop == nil {
		t.Fatal("missing for")
	}
	if loop.Value != "item" || len(loop.ControlEdges) != 2 || loop.DataInputs[0].Name != "iterable" || loop.DataInputs[1].Name != "body" {
		t.Fatalf("for = %#v", loop)
	}
	if loop.ControlEdges[0] != loop.DataInputs[0].SourceNode || loop.ControlEdges[1] != loop.DataInputs[1].SourceNode {
		t.Fatalf("control %v data %#v", loop.ControlEdges, loop.DataInputs)
	}
	iter := graph.NodeByID(loop.ControlEdges[0])
	body := graph.NodeByID(loop.ControlEdges[1])
	if iter == nil || iter.Kind != "list" || body == nil || body.Kind != "if" {
		t.Fatalf("successors iter=%#v body=%#v", iter, body)
	}
	relations := DerivePhase1ControlRelations(graph)
	var sawIter, sawBody bool
	for _, relation := range relations {
		if relation.Controller != loop.ID {
			continue
		}
		switch relation.Role {
		case "iterable":
			sawIter = relation.Controlled == loop.ControlEdges[0] && relation.Ordinal == 0
		case "body":
			sawBody = relation.Controlled == loop.ControlEdges[1] && relation.Ordinal == 1
		}
	}
	if !sawIter || !sawBody {
		t.Fatalf("for relations = %#v", relations)
	}

	program, diags := LowerToBytecode(graph)
	if len(diags) != 0 || program == nil {
		t.Fatalf("LowerToBytecode() diags=%#v", diags)
	}
	if !hasOp(program.Main, bytecode.OpForInit) || !hasOp(program.Main, bytecode.OpForNext) || !hasOp(program.Main, bytecode.OpJump) {
		t.Fatalf("main = %#v", program.Main)
	}
	if err := bytecode.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
}

func TestLowerToBytecodeForInsideDefun(t *testing.T) {
	graph := lowerCheckedFor(t, `(cli_app
  (defun show ((items any)) string
    (do
      (for item items
        (print item))
      (return "done")))
  (print (call show (list "a" "b"))))`)
	var loop *Node
	for _, node := range graph.Nodes {
		if node.Kind == "for" {
			loop = node
		}
		if node.Kind == "defun" && len(node.ControlEdges) != 0 {
			t.Fatalf("defun has control edges %v", node.ControlEdges)
		}
		if node.Kind == "while" {
			t.Fatal("unexpected while")
		}
	}
	if loop == nil || loop.Value != "item" || len(loop.ControlEdges) != 2 {
		t.Fatalf("for = %#v", loop)
	}
	program, diags := LowerToBytecode(graph)
	if len(diags) != 0 || program == nil || program.Functions["show"] == nil {
		t.Fatalf("program=%v diags=%#v", program != nil, diags)
	}
	fn := program.Functions["show"].Instructions
	if !hasOp(fn, bytecode.OpForInit) || !hasOp(fn, bytecode.OpForNext) || !hasOp(fn, bytecode.OpJump) {
		t.Fatalf("function = %#v", fn)
	}
}

func TestLowerToBytecodeForFromGraphWithoutAST(t *testing.T) {
	graph := NewGraph()
	text := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "a"})
	list := graph.AddNode(&Node{Kind: "list", DataInputs: []DataEdge{{Name: "item", SourceNode: text}}})
	item := graph.AddNode(&Node{Kind: "symbol", Value: "item"})
	body := graph.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{Name: "value", SourceNode: item}}})
	loop := graph.AddNode(&Node{
		Kind:  "for",
		Value: "item",
		DataInputs: []DataEdge{
			{Name: "iterable", SourceNode: list},
			{Name: "body", SourceNode: body},
		},
		ControlEdges: []NodeID{list, body},
	})
	graph.EntryNode = graph.AddNode(&Node{Kind: "program", DataInputs: []DataEdge{{Name: "body", SourceNode: loop}}})

	program, diags := LowerToBytecode(graph)
	if len(diags) != 0 || program == nil {
		t.Fatalf("LowerToBytecode() diags=%#v", diags)
	}
	if err := bytecode.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
	if !hasOp(program.Main, bytecode.OpForInit) || !hasOp(program.Main, bytecode.OpForNext) || !hasOp(program.Main, bytecode.OpJump) {
		t.Fatalf("main = %#v", program.Main)
	}
}

func TestLowerToBytecodeRejectsForWithoutControlEdges(t *testing.T) {
	graph := NewGraph()
	text := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "a"})
	list := graph.AddNode(&Node{Kind: "list", DataInputs: []DataEdge{{Name: "item", SourceNode: text}}})
	item := graph.AddNode(&Node{Kind: "symbol", Value: "item"})
	body := graph.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{Name: "value", SourceNode: item}}})
	loop := graph.AddNode(&Node{
		Kind:  "for",
		Value: "item",
		DataInputs: []DataEdge{
			{Name: "iterable", SourceNode: list},
			{Name: "body", SourceNode: body},
		},
	})
	graph.EntryNode = graph.AddNode(&Node{Kind: "program", DataInputs: []DataEdge{{Name: "body", SourceNode: loop}}})

	program, diags := LowerToBytecode(graph)
	if program != nil || len(diags) != 1 || diags[0].Code != BytecodeUnsupportedCode || diags[0].RelatedNode != loop {
		t.Fatalf("program=%v diags=%#v", program != nil, diags)
	}
	for _, relation := range DerivePhase1ControlRelations(graph) {
		if relation.Controller == loop {
			t.Fatalf("for without control edges produced %#v", relation)
		}
	}

	swapped := NewGraph()
	text = swapped.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "a"})
	list = swapped.AddNode(&Node{Kind: "list", DataInputs: []DataEdge{{Name: "item", SourceNode: text}}})
	item = swapped.AddNode(&Node{Kind: "symbol", Value: "item"})
	body = swapped.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{Name: "value", SourceNode: item}}})
	swappedLoop := swapped.AddNode(&Node{
		Kind:  "for",
		Value: "item",
		DataInputs: []DataEdge{
			{Name: "iterable", SourceNode: list},
			{Name: "body", SourceNode: body},
		},
		ControlEdges: []NodeID{body, list},
	})
	swapped.EntryNode = swapped.AddNode(&Node{Kind: "program", DataInputs: []DataEdge{{Name: "body", SourceNode: swappedLoop}}})
	program, diags = LowerToBytecode(swapped)
	if program != nil || len(diags) != 1 || diags[0].Code != BytecodeUnsupportedCode || diags[0].RelatedNode != swappedLoop {
		t.Fatalf("swapped program=%v diags=%#v", program != nil, diags)
	}
	for _, relation := range DerivePhase1ControlRelations(swapped) {
		if relation.Controller == swappedLoop {
			t.Fatalf("swapped for produced %#v", relation)
		}
	}
}

func TestVerifierRequiresForRoles(t *testing.T) {
	graph := NewGraph()
	graph.AddNode(&Node{Kind: "for"})
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

func lowerCheckedFor(t *testing.T, source string) *Graph {
	t.Helper()
	root := parser.NewParser(lexer.NewLexer(source), "for.howl").ParseExpression()
	checker.Check(root)
	graph, err := LowerAST(root, "for.howl")
	if err != nil {
		t.Fatalf("LowerAST: %v", err)
	}
	if diags := NewVerifier(graph, TargetBytecode).Verify(); len(diags) != 0 {
		t.Fatalf("Verify() = %#v", diags)
	}
	return graph
}
