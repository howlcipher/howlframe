package hfir

import (
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestLowerToBytecodeEmitsIfControlEdges(t *testing.T) {
	graph := lowerCheckedIf(t, `(cli_app
  (if false
    (print "no")
    (print "else"))
  (if true
    (print "then"))
  (for item (list "a")
    (print item)))`)
	var withElse, withoutElse, loop *Node
	for _, node := range graph.Nodes {
		switch node.Kind {
		case "if":
			if len(node.DataInputs) == 3 {
				if withElse != nil {
					t.Fatal("expected one if with else")
				}
				withElse = node
			} else {
				if withoutElse != nil {
					t.Fatal("expected one if without else")
				}
				withoutElse = node
			}
		case "for":
			loop = node
		case "while", "defun":
			t.Fatalf("unexpected %s", node.Kind)
		}
	}
	if withElse == nil || withoutElse == nil || loop == nil {
		t.Fatalf("withElse=%v withoutElse=%v for=%v", withElse != nil, withoutElse != nil, loop != nil)
	}
	if len(withElse.ControlEdges) != 3 || withElse.DataInputs[0].Name != "condition" || withElse.DataInputs[1].Name != "then" || withElse.DataInputs[2].Name != "else" {
		t.Fatalf("if else = %#v", withElse)
	}
	for index := range withElse.ControlEdges {
		if withElse.ControlEdges[index] != withElse.DataInputs[index].SourceNode {
			t.Fatalf("control %v data %#v", withElse.ControlEdges, withElse.DataInputs)
		}
	}
	if len(withoutElse.ControlEdges) != 2 || withoutElse.DataInputs[0].Name != "condition" || withoutElse.DataInputs[1].Name != "then" {
		t.Fatalf("if then = %#v", withoutElse)
	}
	if len(loop.ControlEdges) != 2 || loop.DataInputs[0].Name != "iterable" || loop.DataInputs[1].Name != "body" || loop.ControlEdges[0] != loop.DataInputs[0].SourceNode || loop.ControlEdges[1] != loop.DataInputs[1].SourceNode {
		t.Fatalf("for control %v data %#v", loop.ControlEdges, loop.DataInputs)
	}
	thenBranch := graph.NodeByID(withElse.ControlEdges[1])
	elseBranch := graph.NodeByID(withElse.ControlEdges[2])
	if thenBranch == nil || thenBranch.Kind != "print" || elseBranch == nil || elseBranch.Kind != "print" {
		t.Fatalf("successors then=%#v else=%#v", thenBranch, elseBranch)
	}
	relations := DerivePhase1ControlRelations(graph)
	var sawThen, sawElse, sawCond bool
	for _, relation := range relations {
		if relation.Controller != withElse.ID {
			continue
		}
		switch relation.Role {
		case "then":
			sawThen = relation.Controlled == withElse.ControlEdges[1] && relation.Ordinal == 1
		case "else":
			sawElse = relation.Controlled == withElse.ControlEdges[2] && relation.Ordinal == 2
		case "condition":
			sawCond = true
		}
	}
	if !sawThen || !sawElse || sawCond {
		t.Fatalf("if relations = %#v", relations)
	}

	program, diags := LowerToBytecode(graph)
	if len(diags) != 0 || program == nil {
		t.Fatalf("LowerToBytecode() diags=%#v", diags)
	}
	if !hasOp(program.Main, bytecode.OpJumpIfFalse) || !hasOp(program.Main, bytecode.OpJump) || !hasOp(program.Main, bytecode.OpForInit) {
		t.Fatalf("main = %#v", program.Main)
	}
	if err := bytecode.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
}

func TestLowerToBytecodeIfInsideDefun(t *testing.T) {
	graph := lowerCheckedIf(t, `(cli_app
  (defun choose (flag)
    (type_hints (flag bool) (return string))
    (if flag
      (return "yes")
      (return "no")))
  (print (call choose true)))`)
	var branch *Node
	for _, node := range graph.Nodes {
		if node.Kind == "if" {
			branch = node
		}
		if node.Kind == "defun" && len(node.ControlEdges) != 0 {
			t.Fatalf("defun has control edges %v", node.ControlEdges)
		}
		if node.Kind == "while" {
			t.Fatal("unexpected while")
		}
	}
	if branch == nil || len(branch.ControlEdges) != 3 {
		t.Fatalf("if = %#v", branch)
	}
	program, diags := LowerToBytecode(graph)
	if len(diags) != 0 || program == nil || program.Functions["choose"] == nil {
		t.Fatalf("program=%v diags=%#v", program != nil, diags)
	}
	fn := program.Functions["choose"].Instructions
	if !hasOp(fn, bytecode.OpJumpIfFalse) || !hasOp(fn, bytecode.OpJump) || !hasOp(fn, bytecode.OpReturn) {
		t.Fatalf("function = %#v", fn)
	}
}

func TestLowerToBytecodeIfFromGraphWithoutAST(t *testing.T) {
	graph := NewGraph()
	cond := graph.AddNode(&Node{Kind: "const", LiteralKind: "BOOL", Value: "true"})
	yesText := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "yes"})
	noText := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "no"})
	thenBranch := graph.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{Name: "value", SourceNode: yesText}}})
	elseBranch := graph.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{Name: "value", SourceNode: noText}}})
	branch := graph.AddNode(&Node{
		Kind: "if",
		DataInputs: []DataEdge{
			{Name: "condition", SourceNode: cond},
			{Name: "then", SourceNode: thenBranch},
			{Name: "else", SourceNode: elseBranch},
		},
		ControlEdges: []NodeID{cond, thenBranch, elseBranch},
	})
	graph.EntryNode = graph.AddNode(&Node{Kind: "program", DataInputs: []DataEdge{{Name: "body", SourceNode: branch}}})

	program, diags := LowerToBytecode(graph)
	if len(diags) != 0 || program == nil {
		t.Fatalf("LowerToBytecode() diags=%#v", diags)
	}
	if err := bytecode.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}

	onlyThen := NewGraph()
	cond = onlyThen.AddNode(&Node{Kind: "const", LiteralKind: "BOOL", Value: "false"})
	text := onlyThen.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "no"})
	body := onlyThen.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{Name: "value", SourceNode: text}}})
	arm := onlyThen.AddNode(&Node{
		Kind:         "if",
		DataInputs:   []DataEdge{{Name: "condition", SourceNode: cond}, {Name: "then", SourceNode: body}},
		ControlEdges: []NodeID{cond, body},
	})
	onlyThen.EntryNode = onlyThen.AddNode(&Node{Kind: "program", DataInputs: []DataEdge{{Name: "body", SourceNode: arm}}})
	program, diags = LowerToBytecode(onlyThen)
	if len(diags) != 0 || program == nil {
		t.Fatalf("then-only diags=%#v", diags)
	}
	if hasOp(program.Main, bytecode.OpJump) {
		t.Fatalf("then-only if emitted JUMP: %#v", program.Main)
	}
	if !hasOp(program.Main, bytecode.OpJumpIfFalse) {
		t.Fatalf("then-only if = %#v", program.Main)
	}
}

func TestLowerToBytecodeRejectsIfWithoutControlEdges(t *testing.T) {
	graph := NewGraph()
	cond := graph.AddNode(&Node{Kind: "const", LiteralKind: "BOOL", Value: "true"})
	yesText := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "yes"})
	noText := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "no"})
	thenBranch := graph.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{Name: "value", SourceNode: yesText}}})
	elseBranch := graph.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{Name: "value", SourceNode: noText}}})
	branch := graph.AddNode(&Node{
		Kind: "if",
		DataInputs: []DataEdge{
			{Name: "condition", SourceNode: cond},
			{Name: "then", SourceNode: thenBranch},
			{Name: "else", SourceNode: elseBranch},
		},
	})
	graph.EntryNode = graph.AddNode(&Node{Kind: "program", DataInputs: []DataEdge{{Name: "body", SourceNode: branch}}})

	program, diags := LowerToBytecode(graph)
	if program != nil || len(diags) != 1 || diags[0].Code != BytecodeUnsupportedCode || diags[0].RelatedNode != branch {
		t.Fatalf("program=%v diags=%#v", program != nil, diags)
	}
	for _, relation := range DerivePhase1ControlRelations(graph) {
		if relation.Controller == branch {
			t.Fatalf("if without control edges produced %#v", relation)
		}
	}

	swapped := NewGraph()
	cond = swapped.AddNode(&Node{Kind: "const", LiteralKind: "BOOL", Value: "true"})
	yesText = swapped.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "yes"})
	noText = swapped.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "no"})
	thenBranch = swapped.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{Name: "value", SourceNode: yesText}}})
	elseBranch = swapped.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{Name: "value", SourceNode: noText}}})
	swappedBranch := swapped.AddNode(&Node{
		Kind: "if",
		DataInputs: []DataEdge{
			{Name: "condition", SourceNode: cond},
			{Name: "then", SourceNode: thenBranch},
			{Name: "else", SourceNode: elseBranch},
		},
		ControlEdges: []NodeID{cond, elseBranch, thenBranch},
	})
	swapped.EntryNode = swapped.AddNode(&Node{Kind: "program", DataInputs: []DataEdge{{Name: "body", SourceNode: swappedBranch}}})
	program, diags = LowerToBytecode(swapped)
	if program != nil || len(diags) != 1 || diags[0].Code != BytecodeUnsupportedCode || diags[0].RelatedNode != swappedBranch {
		t.Fatalf("swapped program=%v diags=%#v", program != nil, diags)
	}
	for _, relation := range DerivePhase1ControlRelations(swapped) {
		if relation.Controller == swappedBranch {
			t.Fatalf("swapped if produced %#v", relation)
		}
	}
}

func TestModelAdapterIfDerivesControlEdgesFromRoles(t *testing.T) {
	candidate := mustCandidate(t, candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program", Nodes: []transportNode{
		testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "branch"}}),
		testNode("condition", "const", "true", "BOOL", nil),
		testNode("yes", "const", "yes", "STRING", nil),
		testNode("no", "const", "no", "STRING", nil),
		testNode("then", "print", "", "", []transportEdge{{Role: "value", NodeID: "yes"}}),
		testNode("else", "print", "", "", []transportEdge{{Role: "value", NodeID: "no"}}),
		testNode("branch", "if", "", "", []transportEdge{{Role: "condition", NodeID: "condition"}, {Role: "then", NodeID: "then"}, {Role: "else", NodeID: "else"}}),
	}})
	branch := candidate.Graph.NodeByID("branch")
	if branch == nil || len(branch.ControlEdges) != 3 || branch.ControlEdges[0] != "condition" || branch.ControlEdges[1] != "then" || branch.ControlEdges[2] != "else" {
		t.Fatalf("branch = %#v", branch)
	}
	program, diags := CompileCandidate(candidate)
	if len(diags) != 0 || program == nil {
		t.Fatalf("CompileCandidate() diags=%#v", diags)
	}
	if !hasOp(program.Main, bytecode.OpJumpIfFalse) || !hasOp(program.Main, bytecode.OpJump) {
		t.Fatalf("main = %#v", program.Main)
	}
}

func TestModelTransportRejectsControlEdgeField(t *testing.T) {
	payload := []byte(`{"schema_version":"hfir-model-adapter/v1","graph_version":"v1","entry_node":"program","nodes":[{"id":"program","kind":"program","value":"","inputs":[],"provenance":{"label":"t"},"control_edges":["program"]}]}`)
	candidate, diags := DecodeCandidate(payload)
	if candidate.Graph != nil || len(diags) != 1 || diags[0].Code != "HFIR_TRANSPORT_INVALID" {
		t.Fatalf("candidate=%v diags=%#v", candidate.Graph != nil, diags)
	}
}

func lowerCheckedIf(t *testing.T, source string) *Graph {
	t.Helper()
	root := parser.NewParser(lexer.NewLexer(source), "if.howl").ParseExpression()
	checker.Check(root)
	graph, err := LowerAST(root, "if.howl")
	if err != nil {
		t.Fatalf("LowerAST: %v", err)
	}
	if diags := NewVerifier(graph, TargetBytecode).Verify(); len(diags) != 0 {
		t.Fatalf("Verify() = %#v", diags)
	}
	return graph
}
