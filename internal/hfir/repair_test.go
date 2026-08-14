package hfir

import (
	"github.com/howlcipher/howlframe/internal/ast"
	"testing"
)

// TestHFIRSemanticRepair demonstrates that HFIR's explicit graph structure
// allows for targeted, programmatic semantic repairs without textual parsing
// or re-compilation.
func TestHFIRSemanticRepair(t *testing.T) {
	// 1. Construct a broken graph: a `try` node missing its `catch` data edge.
	g := NewGraph()

	// Create body branches
	exprID := g.AddNode(&Node{Kind: "const", Value: "1", LiteralKind: "INT", Type: ast.TypeInfo{Kind: ast.Int}})
	successID := g.AddNode(&Node{Kind: "const", Value: "0", LiteralKind: "INT", Type: ast.TypeInfo{Kind: ast.Int}})

	// Create the broken `try` node
	tryID := g.AddNode(&Node{
		Kind: "try",
		DataInputs: []DataEdge{
			// Missing "catch" edge!
			{Name: "expression", SourceNode: exprID},
			{Name: "success_body", SourceNode: successID},
		},
	})
	g.EntryNode = tryID

	// 2. Verify BEFORE repair
	verifierBefore := NewVerifier(g, "bytecode")
	diagsBefore := verifierBefore.Verify()

	hasMissingRole := false
	for _, d := range diagsBefore {
		if d.Code == "HFIR_MISSING_ROLE" {
			hasMissingRole = true
			break
		}
	}
	if !hasMissingRole {
		t.Fatalf("Expected HFIR_MISSING_ROLE diagnostic before repair, got: %v", diagsBefore)
	}

	// 3. Perform programmatic semantic repair
	// We locate the broken `try` node, notice the missing catch, and inject a fallback catch.
	tryNode := g.NodeByID(tryID)
	hasCatch := false
	for _, edge := range tryNode.DataInputs {
		if edge.Name == "catch" {
			hasCatch = true
		}
	}

	if !hasCatch {
		// Create a fallback `catch` node to repair the graph
		catchBodyID := g.AddNode(&Node{
			Kind:        "const",
			Value:       "0",
			LiteralKind: "INT",
			Type:        ast.TypeInfo{Kind: ast.Int},
		})
		fallbackCatchID := g.AddNode(&Node{
			Kind:       "catch",
			Value:      "err",
			DataInputs: []DataEdge{{Name: "body", SourceNode: catchBodyID}},
		})

		// Inject the new catch edge
		tryNode.DataInputs = append(tryNode.DataInputs, DataEdge{Name: "catch", SourceNode: fallbackCatchID})
	}

	// 4. Verify AFTER repair
	verifierAfter := NewVerifier(g, "bytecode")
	diagsAfter := verifierAfter.Verify()

	for _, d := range diagsAfter {
		if d.Code == "HFIR_MISSING_ROLE" {
			t.Fatalf("Expected no HFIR_MISSING_ROLE diagnostic after repair, but it was still present: %v", diagsAfter)
		}
	}
}
