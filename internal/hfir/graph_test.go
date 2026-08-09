package hfir

import (
	"testing"
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
