package zir

import (
	"testing"
	"zero/internal/ast"
)

func TestVerifier(t *testing.T) {
	t.Run("Valid graph", func(t *testing.T) {
		g := NewGraph()
		id1 := g.AddNode(&Node{
			Kind: "const",
			Type: ast.TypeInfo{Kind: ast.String},
		})
		id2 := g.AddNode(&Node{
			Kind: "print",
			Type: ast.TypeInfo{Kind: ast.String},
			DataInputs: []DataEdge{
				{SourceNode: id1},
			},
		})
		g.EntryNode = id2

		verifier := NewVerifier(g, "go")
		diags := verifier.Verify()
		if len(diags) > 0 {
			t.Fatalf("Expected no diagnostics, got %v", diags)
		}
	})

	t.Run("Unbound reference", func(t *testing.T) {
		g := NewGraph()
		g.AddNode(&Node{
			Kind: "symbol",
			Type: ast.TypeInfo{Kind: ast.Unknown},
			Value: "my_var",
		})

		verifier := NewVerifier(g, "go")
		diags := verifier.Verify()
		if len(diags) != 1 || diags[0].Code != "ZIR_UNBOUND_REF" {
			t.Fatalf("Expected ZIR_UNBOUND_REF, got %v", diags)
		}
	})

	t.Run("Invalid reference", func(t *testing.T) {
		g := NewGraph()
		g.AddNode(&Node{
			Kind: "print",
			DataInputs: []DataEdge{
				{SourceNode: NodeID("n999")},
			},
		})

		verifier := NewVerifier(g, "go")
		diags := verifier.Verify()
		if len(diags) != 1 || diags[0].Code != "ZIR_INVALID_REF" {
			t.Fatalf("Expected ZIR_INVALID_REF, got %v", diags)
		}
	})

	t.Run("Effect inference", func(t *testing.T) {
		g := NewGraph()
		id := g.AddNode(&Node{
			Kind: "read_file",
		})

		verifier := NewVerifier(g, "go")
		diags := verifier.Verify()
		if len(diags) != 0 {
			t.Fatalf("Expected no diagnostics, got %v", diags)
		}
		
		node := g.nodeMap[id]
		if len(node.Effects) != 1 || node.Effects[0].Capability != "filesystem" {
			t.Fatalf("Expected filesystem capability effect, got %v", node.Effects)
		}
	})

	t.Run("Target infeasible", func(t *testing.T) {
		g := NewGraph()
		g.AddNode(&Node{
			Kind: "spawn_agent",
		})

		verifier := NewVerifier(g, "wasm")
		diags := verifier.Verify()
		if len(diags) != 1 || diags[0].Code != "ZIR_TARGET_INFEASIBLE" {
			t.Fatalf("Expected ZIR_TARGET_INFEASIBLE, got %v", diags)
		}
	})
}
