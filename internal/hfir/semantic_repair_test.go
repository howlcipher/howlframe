package hfir

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/vm"
)

func TestSemanticRepairTransactionRepairsCoordinatedDictionaryUpdate(t *testing.T) {
	candidate := mustCandidate(t, candidateTransport{
		SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program",
		Nodes: []transportNode{
			testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "bind_record"}}),
			testNode("result_key", "const", "result", "STRING", nil),
			testNode("empty", "const", "empty", "STRING", nil),
			testNode("initial_entry", "dict_entry", "", "", []transportEdge{{Role: "key", NodeID: "result_key"}, {Role: "value", NodeID: "empty"}}),
			testNode("record", "dict", "", "", []transportEdge{{Role: "entry", NodeID: "initial_entry"}}),
			testNode("update_key", "const", "wrong", "STRING", nil),
			testNode("update_value", "const", "nope", "STRING", nil),
			testNode("write_result", "map_set", "record", "", []transportEdge{{Role: "key", NodeID: "update_key"}, {Role: "value", NodeID: "update_value"}}),
			testNode("read_result", "map_get", "record", "", []transportEdge{{Role: "key", NodeID: "result_key"}}),
			testNode("print_result", "print", "", "", []transportEdge{{Role: "value", NodeID: "read_result"}}),
			testNode("body", "sequence", "", "", []transportEdge{{Role: "body", NodeID: "write_result"}, {Role: "body", NodeID: "print_result"}}),
			testNode("bind_record", "let", "record", "", []transportEdge{{Role: "value", NodeID: "record"}, {Role: "body", NodeID: "body"}}),
		},
	})
	assertCandidateOutput(t, candidate, "empty\n")
	context, err := NewSemanticRepairContext(candidate, []NodeID{"update_key", "update_value"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := context.EditableNodeIDs, []NodeID{"update_key", "update_value", "write_result"}; !sameNodeIDSlice(got, want) {
		t.Fatalf("derived editable region = %#v, want %#v", got, want)
	}
	originalHash := candidate.Hash
	resultKeyHash := NodeHash(candidate.Graph.NodeByID("result_key"))
	delta := mustSemanticRepairDelta(t, candidate, context, []repairOperation{
		{Operation: "replace_node", TargetNodeID: "update_key", ExpectedNodeHash: NodeHash(candidate.Graph.NodeByID("update_key")), Replacement: testNode("update_key", "const", "result", "STRING", nil)},
		{Operation: "replace_node", TargetNodeID: "update_value", ExpectedNodeHash: NodeHash(candidate.Graph.NodeByID("update_value")), Replacement: testNode("update_value", "const", "ready", "STRING", nil)},
	})
	updated, diagnostics := ApplyRepair(candidate, context, delta)
	if len(diagnostics) != 0 {
		t.Fatalf("ApplyRepair() diagnostics = %#v", diagnostics)
	}
	assertCandidateOutput(t, updated, "ready\n")
	if candidate.Hash != originalHash || NodeHash(candidate.Graph.NodeByID("result_key")) != resultKeyHash {
		t.Fatal("repair mutated the original or an untouched node")
	}
}

func TestSemanticRepairTransactionIsAtomicWhenLaterPreconditionFails(t *testing.T) {
	candidate := mustCandidate(t, candidateTransport{
		SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program",
		Nodes: []transportNode{
			testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "print"}}),
			testNode("left", "const", "wrong", "STRING", nil),
			testNode("right", "const", "also-wrong", "STRING", nil),
			testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "left"}, {Role: "value", NodeID: "right"}}),
		},
	})
	context, err := NewSemanticRepairContext(candidate, []NodeID{"left", "right"})
	if err != nil {
		t.Fatal(err)
	}
	delta := mustSemanticRepairDelta(t, candidate, context, []repairOperation{
		{Operation: "replace_node", TargetNodeID: "left", ExpectedNodeHash: NodeHash(candidate.Graph.NodeByID("left")), Replacement: testNode("left", "const", "fixed", "STRING", nil)},
		{Operation: "replace_node", TargetNodeID: "right", ExpectedNodeHash: "stale", Replacement: testNode("right", "const", "fixed", "STRING", nil)},
	})
	updated, diagnostics := ApplyRepair(candidate, context, delta)
	if updated.Graph != nil || len(diagnostics) != 1 || diagnostics[0].Code != "HFIR_REPAIR_STALE" {
		t.Fatalf("ApplyRepair() = (%#v, %#v), want atomic stale rejection", updated, diagnostics)
	}
	if value := candidate.Graph.NodeByID("left").Value; value != "wrong" {
		t.Fatalf("failed transaction mutated original left node to %q", value)
	}
}

func mustSemanticRepairDelta(t *testing.T, candidate Candidate, context RepairContext, operations []repairOperation) []byte {
	t.Helper()
	data, err := json.Marshal(repairTransport{
		SchemaVersion: SemanticRepairSchemaVersion, ExpectedGraphHash: candidate.Hash,
		ExpectedGraphVersion: candidate.Graph.Version, ProtectedNodeHashes: context.ProtectedNodeHashes,
		Operations: operations,
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertCandidateOutput(t *testing.T, candidate Candidate, want string) {
	t.Helper()
	program, diagnostics := CompileCandidate(candidate)
	if len(diagnostics) != 0 {
		t.Fatalf("CompileCandidate() diagnostics = %#v", diagnostics)
	}
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	if exit := vm.RunBytecode(program, nil, nil, strings.NewReader(""), stdout, stderr); exit != 0 {
		t.Fatalf("RunBytecode() exit = %d, stderr = %q", exit, stderr.String())
	}
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}
