package hfir

import (
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
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
			assertControlEdgesMatchInputs(t, node, 3, "condition", "then", "else")
		case "while", "defun":
			t.Fatalf("unexpected %s", node.Kind)
		}
	}
	if loop == nil {
		t.Fatal("missing for")
	}
	if loop.Value != "item" {
		t.Fatalf("for = %#v", loop)
	}
	assertControlEdgesMatchInputs(t, loop, 2, "iterable", "body")
	iter := graph.NodeByID(loop.ControlEdges[0])
	body := graph.NodeByID(loop.ControlEdges[1])
	if iter == nil || iter.Kind != "list" || body == nil || body.Kind != "if" {
		t.Fatalf("successors iter=%#v body=%#v", iter, body)
	}
	relations := DerivePhase1ControlRelations(graph)
	assertControlRelationsOrder(t, relations, loop.ID, loop.ControlEdges, "iterable", "body")

	mustLowerAndValidateGraph(t, graph, bytecode.OpForInit, bytecode.OpForNext, bytecode.OpJump)
}

func TestLowerToBytecodeForInsideDefun(t *testing.T) {
	graph := lowerCheckedFor(t, `(cli_app
  (defun show ((items any)) string
    (do
      (for item items
        (print item))
      (return "done")))
  (print (call show (list "a" "b"))))`)
	loop := findSingleKindOutsideWhile(t, graph, "for", 2)
	if loop.Value != "item" {
		t.Fatalf("for = %#v", loop)
	}
	mustLowerFunctionOps(t, graph, "show", bytecode.OpForInit, bytecode.OpForNext, bytecode.OpJump)
}

func makeForTestGraph(controlOrder func(list, body NodeID) []NodeID) (*Graph, NodeID) {
	graph := NewGraph()
	text := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "a"})
	list := graph.AddNode(&Node{Kind: "list", DataInputs: []DataEdge{{Name: "item", SourceNode: text}}})
	item := graph.AddNode(&Node{Kind: "symbol", Value: "item"})
	body := graph.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{Name: "value", SourceNode: item}}})
	var edges []NodeID
	if controlOrder != nil {
		edges = controlOrder(list, body)
	}
	loop := graph.AddNode(&Node{
		Kind:         "for",
		Value:        "item",
		DataInputs:   []DataEdge{{Name: "iterable", SourceNode: list}, {Name: "body", SourceNode: body}},
		ControlEdges: edges,
	})
	graph.EntryNode = graph.AddNode(&Node{Kind: "program", DataInputs: []DataEdge{{Name: "body", SourceNode: loop}}})
	return graph, loop
}

func TestLowerToBytecodeForFromGraphWithoutAST(t *testing.T) {
	graph, _ := makeForTestGraph(func(list, body NodeID) []NodeID { return []NodeID{list, body} })
	mustLowerAndValidateGraph(t, graph, bytecode.OpForInit, bytecode.OpForNext, bytecode.OpJump)
}

func TestLowerToBytecodeRejectsForWithoutControlEdges(t *testing.T) {
	graph, loop := makeForTestGraph(nil)
	assertControllerRejectedNoRelations(t, "missing", graph, loop)

	swapped, swappedLoop := makeForTestGraph(func(list, body NodeID) []NodeID { return []NodeID{body, list} })
	assertControllerRejectedNoRelations(t, "swapped", swapped, swappedLoop)
}

func TestVerifierRequiresForRoles(t *testing.T) {
	assertVerifierMissingRoleCount(t, "for", 2)
}

func lowerCheckedFor(t *testing.T, source string) *Graph {
	t.Helper()
	return lowerCheckedNamed(t, "for.howl", source)
}
