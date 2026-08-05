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
			Kind:  "symbol",
			Type:  ast.TypeInfo{Kind: ast.Unknown},
			Value: "my_var",
		})

		verifier := NewVerifier(g, "go")
		diags := verifier.Verify()
		if len(diags) != 1 || diags[0].Code != "ZIR_UNBOUND_REF" {
			t.Fatalf("Expected ZIR_UNBOUND_REF, got %v", diags)
		}
		if diags[0].ContractVersion != DiagnosticContractVersion {
			t.Errorf("expected ContractVersion %q, got %q", DiagnosticContractVersion, diags[0].ContractVersion)
		}
		if diags[0].Target != "go" {
			t.Errorf("expected Target %q, got %q", "go", diags[0].Target)
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
		if diags[0].ContractVersion != DiagnosticContractVersion {
			t.Errorf("expected ContractVersion %q, got %q", DiagnosticContractVersion, diags[0].ContractVersion)
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
		if diags[0].Target != "wasm" {
			t.Errorf("expected Target %q, got %q", "wasm", diags[0].Target)
		}
	})
}

// TestVerifierIsIdempotent confirms that calling Verify() twice on the same
// Verifier does not duplicate diagnostics or duplicate the capability
// effects Verify() appends to nodes as a side effect.
func TestVerifierIsIdempotent(t *testing.T) {
	g := NewGraph()
	g.AddNode(&Node{
		Kind:  "symbol",
		Type:  ast.TypeInfo{Kind: ast.Unknown},
		Value: "my_var",
	})
	g.AddNode(&Node{
		Kind: "read_file",
	})
	g.AddNode(&Node{
		Kind: "spawn_agent",
	})

	verifier := NewVerifier(g, "wasm")
	first := verifier.Verify()
	second := verifier.Verify()

	if len(first) != len(second) {
		t.Fatalf("expected the same diagnostic count across repeated Verify() calls, got %d then %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("diagnostic %d differs across repeated Verify() calls: %+v vs %+v", i, first[i], second[i])
		}
	}

	for _, node := range g.Nodes {
		if node.Kind != "read_file" {
			continue
		}
		if len(node.Effects) != 1 {
			t.Fatalf("expected exactly one capability effect after two Verify() calls, got %v", node.Effects)
		}
	}
}

// TestVerifierDiagnosticOrderIsDeterministic confirms diagnostics are
// ordered by Graph.Nodes slice order, repeatably across calls.
func TestVerifierDiagnosticOrderIsDeterministic(t *testing.T) {
	g := NewGraph()
	g.AddNode(&Node{Kind: "symbol", Type: ast.TypeInfo{Kind: ast.Unknown}, Value: "first_unbound"})
	g.AddNode(&Node{Kind: "print", DataInputs: []DataEdge{{SourceNode: NodeID("missing")}}})
	g.AddNode(&Node{Kind: "symbol", Type: ast.TypeInfo{Kind: ast.Unknown}, Value: "second_unbound"})

	verifier := NewVerifier(g, "go")
	for i := 0; i < 3; i++ {
		diags := verifier.Verify()
		if len(diags) != 3 {
			t.Fatalf("run %d: expected 3 diagnostics, got %d: %v", i, len(diags), diags)
		}
		wantCodes := []string{"ZIR_UNBOUND_REF", "ZIR_INVALID_REF", "ZIR_UNBOUND_REF"}
		for j, want := range wantCodes {
			if diags[j].Code != want {
				t.Fatalf("run %d: diagnostic %d: expected code %q, got %q", i, j, want, diags[j].Code)
			}
		}
	}
}
