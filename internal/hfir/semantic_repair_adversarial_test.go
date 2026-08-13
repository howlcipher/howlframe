package hfir

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSemanticRepairRejectsAdversarialTransactions(t *testing.T) {
	candidate := semanticRepairAdversarialCandidate(t)
	context, err := NewSemanticRepairContext(candidate, []NodeID{"left", "right"})
	if err != nil {
		t.Fatal(err)
	}
	base := repairTransport{
		SchemaVersion: SemanticRepairSchemaVersion, ExpectedGraphHash: candidate.Hash,
		ExpectedGraphVersion: candidate.Graph.Version, ProtectedNodeHashes: context.ProtectedNodeHashes,
		Operations: []repairOperation{{
			Operation: "replace_node", TargetNodeID: "left", ExpectedNodeHash: NodeHash(candidate.Graph.NodeByID("left")),
			Replacement: testNode("left", "const", "fixed", "STRING", nil),
		}},
	}
	encode := func(delta repairTransport) []byte {
		data, err := json.Marshal(delta)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	clone := func() repairTransport {
		delta := base
		delta.Operations = append([]repairOperation(nil), base.Operations...)
		return delta
	}
	assertRejected := func(t *testing.T, payload []byte, want string) {
		t.Helper()
		originalHash := candidate.Hash
		originalLeft := NodeHash(candidate.Graph.NodeByID("left"))
		updated, diagnostics := ApplyRepair(candidate, context, payload)
		if updated.Graph != nil || len(diagnostics) != 1 || diagnostics[0].Code != want {
			t.Fatalf("ApplyRepair() = (%#v, %#v), want %s rejection", updated, diagnostics, want)
		}
		if candidate.Hash != originalHash || NodeHash(candidate.Graph.NodeByID("left")) != originalLeft {
			t.Fatal("rejected repair mutated its input candidate")
		}
	}

	t.Run("stale graph", func(t *testing.T) {
		delta := clone()
		delta.ExpectedGraphHash = strings.Repeat("a", 64)
		assertRejected(t, encode(delta), "HFIR_REPAIR_STALE")
	})
	t.Run("stale node", func(t *testing.T) {
		delta := clone()
		delta.Operations[0].ExpectedNodeHash = strings.Repeat("b", 64)
		assertRejected(t, encode(delta), "HFIR_REPAIR_STALE")
	})
	t.Run("outside region", func(t *testing.T) {
		delta := clone()
		delta.Operations[0].TargetNodeID = "program"
		delta.Operations[0].ExpectedNodeHash = NodeHash(candidate.Graph.NodeByID("program"))
		delta.Operations[0].Replacement = testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "print"}})
		assertRejected(t, encode(delta), "HFIR_REPAIR_SCOPE")
	})
	t.Run("protected precondition omitted", func(t *testing.T) {
		delta := clone()
		delta.ProtectedNodeHashes = nil
		assertRejected(t, encode(delta), "HFIR_REPAIR_PROTECTED")
	})
	t.Run("self widening and backend injection", func(t *testing.T) {
		payload := strings.Replace(string(encode(clone())), `"operations":`, `"backend":"wasm","editable_node_ids":["program"],"operations":`, 1)
		assertRejected(t, []byte(payload), "HFIR_TRANSPORT_INVALID")
	})
	t.Run("entry and version substitution", func(t *testing.T) {
		payload := strings.Replace(string(encode(clone())), `"operations":`, `"entry_node":"other","graph_version":"v2","operations":`, 1)
		assertRejected(t, []byte(payload), "HFIR_TRANSPORT_INVALID")
	})
	t.Run("undeclared reference", func(t *testing.T) {
		delta := clone()
		delta.Operations[0].Replacement = testNode("left", "const", "fixed", "STRING", []transportEdge{{Role: "value", NodeID: "undeclared"}})
		assertRejected(t, encode(delta), "HFIR_REPAIR_SCOPE")
	})
	t.Run("atomic combined invalid repair", func(t *testing.T) {
		delta := clone()
		delta.Operations = append(delta.Operations, repairOperation{
			Operation: "replace_node", TargetNodeID: "right", ExpectedNodeHash: NodeHash(candidate.Graph.NodeByID("right")),
			Replacement: testNode("right", "const", "not-an-int", "INT", nil),
		})
		assertRejected(t, encode(delta), "HFIR_TRANSPORT_LITERAL")
	})
}

func semanticRepairAdversarialCandidate(t *testing.T) Candidate {
	t.Helper()
	return mustCandidate(t, candidateTransport{
		SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program",
		Nodes: []transportNode{
			testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "print"}}),
			testNode("left", "const", "1", "INT", nil),
			testNode("right", "const", "2", "INT", nil),
			testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "left"}, {Role: "value", NodeID: "right"}}),
		},
	})
}
