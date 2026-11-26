package hfir

import (
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
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
			assertControlEdgesMatchInputs(t, node, 3, "condition", "then", "else")
			continue
		}
		if node.Kind == "for" && len(node.ControlEdges) != 0 {
			t.Fatalf("for %s has control edges %v", node.ID, node.ControlEdges)
		}
	}
	if loop == nil {
		t.Fatal("missing while")
	}
	assertControlEdgesMatchInputs(t, loop, 2, "condition", "body")
	cond := graph.NodeByID(loop.ControlEdges[0])
	body := graph.NodeByID(loop.ControlEdges[1])
	if cond == nil || cond.Kind != "binary" || body == nil || body.Kind != "sequence" {
		t.Fatalf("successors cond=%#v body=%#v", cond, body)
	}
	relations := DerivePhase1ControlRelations(graph)
	assertControlRelationsOrder(t, relations, loop.ID, loop.ControlEdges, "condition", "body")

	mustLowerAndValidateGraph(t, graph, bytecode.OpJumpIfFalse, bytecode.OpJump)
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
	loop := findSingleKindOutsideWhile(t, graph, "while", 2)
	_ = loop
	mustLowerFunctionOps(t, graph, "steps", bytecode.OpJumpIfFalse, bytecode.OpReturn)
}

func makeWhileTestGraph(controlOrder func(cond, body NodeID) []NodeID) (*Graph, NodeID) {
	graph := NewGraph()
	cond := graph.AddNode(&Node{Kind: "const", LiteralKind: "BOOL", Value: "false"})
	text := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "no"})
	body := graph.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{Name: "value", SourceNode: text}}})
	var edges []NodeID
	if controlOrder != nil {
		edges = controlOrder(cond, body)
	}
	loop := graph.AddNode(&Node{
		Kind:         "while",
		DataInputs:   []DataEdge{{Name: "condition", SourceNode: cond}, {Name: "body", SourceNode: body}},
		ControlEdges: edges,
	})
	graph.EntryNode = graph.AddNode(&Node{Kind: "program", DataInputs: []DataEdge{{Name: "body", SourceNode: loop}}})
	return graph, loop
}

func TestLowerToBytecodeWhileFromGraphWithoutAST(t *testing.T) {
	graph, _ := makeWhileTestGraph(func(cond, body NodeID) []NodeID { return []NodeID{cond, body} })
	mustLowerAndValidateGraph(t, graph)
}

func TestLowerToBytecodeRejectsWhileWithoutControlEdges(t *testing.T) {
	graph, loop := makeWhileTestGraph(nil)
	assertControllerRejectedNoRelations(t, "missing", graph, loop)

	swapped, swappedLoop := makeWhileTestGraph(func(cond, body NodeID) []NodeID { return []NodeID{body, cond} })
	assertControllerRejectedNoRelations(t, "swapped", swapped, swappedLoop)
}

func TestVerifierRequiresWhileRoles(t *testing.T) {
	assertVerifierMissingRoleCount(t, "while", 2)
}

func lowerCheckedWhile(t *testing.T, source string) *Graph {
	t.Helper()
	return lowerCheckedNamed(t, "while.howl", source)
}
