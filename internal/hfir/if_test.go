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
	assertControlEdgesMatchInputs(t, withElse, 3, "condition", "then", "else")
	assertControlEdgesMatchInputs(t, withoutElse, 2, "condition", "then")
	assertControlEdgesMatchInputs(t, loop, 2, "iterable", "body")
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

	mustLowerAndValidateGraph(t, graph, bytecode.OpJumpIfFalse, bytecode.OpJump, bytecode.OpForInit)
}

func TestLowerToBytecodeIfInsideDefun(t *testing.T) {
	graph := lowerCheckedIf(t, `(cli_app
  (defun choose (flag)
    (type_hints (flag bool) (return string))
    (if flag
      (return "yes")
      (return "no")))
  (print (call choose true)))`)
	branch := findSingleKindOutsideWhile(t, graph, "if", 3)
	mustLowerFunctionOps(t, graph, "choose", bytecode.OpJumpIfFalse, bytecode.OpJump, bytecode.OpReturn)
	_ = branch
}

func mustLowerFunctionOps(t *testing.T, graph *Graph, fnName string, wantOps ...bytecode.Opcode) {
	t.Helper()
	program, diags := LowerToBytecode(graph)
	if len(diags) != 0 || program == nil || program.Functions[fnName] == nil {
		t.Fatalf("program=%v fn=%s diags=%#v", program != nil, fnName, diags)
	}
	fn := program.Functions[fnName].Instructions
	for _, op := range wantOps {
		if !hasOp(fn, op) {
			t.Fatalf("function %s missing op %v: %#v", fnName, op, fn)
		}
	}
}

func findSingleKindOutsideWhile(t *testing.T, graph *Graph, kind string, wantEdges int) *Node {
	t.Helper()
	var target *Node
	for _, node := range graph.Nodes {
		if node.Kind == kind {
			target = node
		}
		if node.Kind == "defun" && len(node.ControlEdges) != 0 {
			t.Fatalf("defun has control edges %v", node.ControlEdges)
		}
		if kind != "while" && node.Kind == "while" {
			t.Fatal("unexpected while")
		}
	}
	if target == nil || len(target.ControlEdges) != wantEdges {
		t.Fatalf("%s = %#v, want %d control edges", kind, target, wantEdges)
	}
	return target
}

func assertControlRelationsOrder(t *testing.T, relations []DerivedControlRelation, controller NodeID, edges []NodeID, roles ...string) {
	t.Helper()
	seen := make([]bool, len(roles))
	for _, relation := range relations {
		if relation.Controller != controller {
			continue
		}
		for i, role := range roles {
			if relation.Role == role {
				seen[i] = relation.Controlled == edges[i] && relation.Ordinal == i
			}
		}
	}
	for i, ok := range seen {
		if !ok {
			t.Fatalf("%s relations = %#v (missing/wrong role %q)", controller, relations, roles[i])
		}
	}
}

func makeIfTestGraph(includeElse bool, controlOrder func(cond, thenBranch, elseBranch NodeID) []NodeID) (*Graph, NodeID) {
	graph := NewGraph()
	cond := graph.AddNode(&Node{Kind: "const", LiteralKind: "BOOL", Value: "true"})
	yesText := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "yes"})
	thenBranch := graph.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{Name: "value", SourceNode: yesText}}})
	inputs := []DataEdge{{Name: "condition", SourceNode: cond}, {Name: "then", SourceNode: thenBranch}}
	var elseBranch NodeID
	if includeElse {
		noText := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "no"})
		elseBranch = graph.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{Name: "value", SourceNode: noText}}})
		inputs = append(inputs, DataEdge{Name: "else", SourceNode: elseBranch})
	}
	var edges []NodeID
	if controlOrder != nil {
		edges = controlOrder(cond, thenBranch, elseBranch)
	}
	branch := graph.AddNode(&Node{Kind: "if", DataInputs: inputs, ControlEdges: edges})
	graph.EntryNode = graph.AddNode(&Node{Kind: "program", DataInputs: []DataEdge{{Name: "body", SourceNode: branch}}})
	return graph, branch
}

func assertControllerRejectedNoRelations(t *testing.T, label string, graph *Graph, controller NodeID) {
	t.Helper()
	program, diags := LowerToBytecode(graph)
	if program != nil || len(diags) != 1 || diags[0].Code != BytecodeUnsupportedCode || diags[0].RelatedNode != controller {
		t.Fatalf("%s: program=%v diags=%#v", label, program != nil, diags)
	}
	for _, relation := range DerivePhase1ControlRelations(graph) {
		if relation.Controller == controller {
			t.Fatalf("%s: controller without valid edges produced %#v", label, relation)
		}
	}
}

func assertVerifierMissingRoleCount(t *testing.T, kind string, want int) {
	t.Helper()
	graph := NewGraph()
	graph.AddNode(&Node{Kind: kind})
	var missing int
	for _, diag := range NewVerifier(graph, TargetBytecode).Verify() {
		if diag.Code == "HFIR_MISSING_ROLE" {
			missing++
		}
	}
	if missing != want {
		t.Fatalf("%s missing-role diagnostics = %d, want %d", kind, missing, want)
	}
}

func TestLowerToBytecodeIfFromGraphWithoutAST(t *testing.T) {
	graph, _ := makeIfTestGraph(true, func(cond, thenBranch, elseBranch NodeID) []NodeID {
		return []NodeID{cond, thenBranch, elseBranch}
	})
	mustLowerAndValidateGraph(t, graph)

	onlyThen, _ := makeIfTestGraph(false, func(cond, thenBranch, _ NodeID) []NodeID {
		return []NodeID{cond, thenBranch}
	})
	program := mustLowerAndValidateGraph(t, onlyThen, bytecode.OpJumpIfFalse)
	if hasOp(program.Main, bytecode.OpJump) {
		t.Fatalf("then-only if emitted JUMP: %#v", program.Main)
	}
}

func TestLowerToBytecodeRejectsIfWithoutControlEdges(t *testing.T) {
	graph, branch := makeIfTestGraph(true, nil)
	assertControllerRejectedNoRelations(t, "missing", graph, branch)

	swapped, swappedBranch := makeIfTestGraph(true, func(cond, thenBranch, elseBranch NodeID) []NodeID {
		return []NodeID{cond, elseBranch, thenBranch}
	})
	assertControllerRejectedNoRelations(t, "swapped", swapped, swappedBranch)
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
	return lowerCheckedNamed(t, "if.howl", source)
}

func lowerCheckedNamed(t *testing.T, filename, source string) *Graph {
	t.Helper()
	root := parser.NewParser(lexer.NewLexer(source), filename).ParseExpression()
	checker.Check(root)
	graph, err := LowerAST(root, filename)
	if err != nil {
		t.Fatalf("LowerAST(%s): %v", filename, err)
	}
	if diags := NewVerifier(graph, TargetBytecode).Verify(); len(diags) != 0 {
		t.Fatalf("Verify(%s) = %#v", filename, diags)
	}
	return graph
}
