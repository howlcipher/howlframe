package zir

import (
	"testing"
	"zero/internal/ast"
)

func TestDeterministicSerialization(t *testing.T) {
	g := NewGraph()
	n1 := &Node{Kind: "const", Value: "1"}
	n2 := &Node{Kind: "const", Value: "2"}
	n3 := &Node{Kind: "add", DataInputs: []DataEdge{{SourceNode: "n1"}, {SourceNode: "n2"}}}

	g.AddNode(n1)
	g.AddNode(n2)
	g.AddNode(n3)
	g.EntryNode = "n3"

	b1, err := g.Serialize()
	if err != nil {
		t.Fatal(err)
	}

	b2, err := g.Serialize()
	if err != nil {
		t.Fatal(err)
	}

	if string(b1) != string(b2) {
		t.Errorf("serialization is not deterministic")
	}
}

func TestASTLowering(t *testing.T) {
	root := &ast.Node{
		Type: "List",
		Children: []*ast.Node{
			{Type: "SYMBOL", Value: "add"},
			{Type: "NUMBER", Value: "1"},
			{Type: "NUMBER", Value: "2"},
		},
	}

	g, err := LowerAST(root, "test")
	if err != nil {
		t.Fatal(err)
	}

	if len(g.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(g.Nodes))
	}

	if g.EntryNode != "n1" {
		t.Errorf("expected entry node n1, got %s", g.EntryNode)
	}
}
