package hfir

import (
	"bytes"
	"encoding/json"
	"os"
	"strconv"
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

func TestSemanticRepairRegionStaysBoundedAcrossGraphSizes(t *testing.T) {
	for _, size := range []int{10, 25, 50, 100} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			candidate := scaledRepairCandidate(t, size)
			context, err := NewSemanticRepairContext(candidate, []NodeID{"defect"})
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(candidate.Graph.Nodes), size; got != want {
				t.Fatalf("graph nodes = %d, want %d", got, want)
			}
			if got := len(context.EditableNodeIDs); got != 2 {
				t.Fatalf("repair region nodes = %d, want 2", got)
			}
			if got := len(context.ProtectedNodeHashes); got != size-2 {
				t.Fatalf("protected nodes = %d, want %d", got, size-2)
			}
		})
	}
}

func TestBlackBoxSemanticRepairTranscriptReplaysOffline(t *testing.T) {
	data, err := os.ReadFile("../../docs/fixtures/hfir_semantic_repair_black_box_phase2.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Attempts []struct {
			Attempt int    `json:"attempt"`
			Result  string `json:"result"`
			Delta   struct {
				Operations []struct {
					TargetNodeID string        `json:"target_node_id"`
					Replacement  transportNode `json:"replacement"`
				} `json:"operations"`
			} `json:"delta"`
		} `json:"attempts"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Attempts) != 3 || fixture.Attempts[0].Result != "rejected" || fixture.Attempts[1].Result != "rejected" || fixture.Attempts[2].Result != "accepted" {
		t.Fatalf("unexpected black-box attempt history: %#v", fixture.Attempts)
	}
	candidate := semanticRepairDictionaryCandidate(t)
	context, err := NewSemanticRepairContext(candidate, []NodeID{"update_key", "update_value"})
	if err != nil {
		t.Fatal(err)
	}
	operations := make([]repairOperation, 0, len(fixture.Attempts[2].Delta.Operations))
	for _, operation := range fixture.Attempts[2].Delta.Operations {
		operations = append(operations, repairOperation{
			Operation: "replace_node", TargetNodeID: NodeID(operation.TargetNodeID),
			ExpectedNodeHash: NodeHash(candidate.Graph.NodeByID(NodeID(operation.TargetNodeID))),
			Replacement:      operation.Replacement,
		})
	}
	updated, diagnostics := ApplyRepair(candidate, context, mustSemanticRepairDelta(t, candidate, context, operations))
	if len(diagnostics) != 0 {
		t.Fatalf("ApplyRepair() diagnostics = %#v", diagnostics)
	}
	assertCandidateOutput(t, updated, "ready\n")
}

func semanticRepairDictionaryCandidate(t *testing.T) Candidate {
	t.Helper()
	return mustCandidate(t, candidateTransport{
		SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program",
		Nodes: []transportNode{
			testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "bind_record"}}),
			testNode("result_key", "const", "result", "STRING", nil), testNode("empty", "const", "empty", "STRING", nil),
			testNode("initial_entry", "dict_entry", "", "", []transportEdge{{Role: "key", NodeID: "result_key"}, {Role: "value", NodeID: "empty"}}),
			testNode("record", "dict", "", "", []transportEdge{{Role: "entry", NodeID: "initial_entry"}}),
			testNode("update_key", "const", "wrong", "STRING", nil), testNode("update_value", "const", "nope", "STRING", nil),
			testNode("write_result", "map_set", "record", "", []transportEdge{{Role: "key", NodeID: "update_key"}, {Role: "value", NodeID: "update_value"}}),
			testNode("read_result", "map_get", "record", "", []transportEdge{{Role: "key", NodeID: "result_key"}}),
			testNode("print_result", "print", "", "", []transportEdge{{Role: "value", NodeID: "read_result"}}),
			testNode("body", "sequence", "", "", []transportEdge{{Role: "body", NodeID: "write_result"}, {Role: "body", NodeID: "print_result"}}),
			testNode("bind_record", "let", "record", "", []transportEdge{{Role: "value", NodeID: "record"}, {Role: "body", NodeID: "body"}}),
		},
	})
}

func scaledRepairCandidate(t *testing.T, size int) Candidate {
	t.Helper()
	nodes := []transportNode{testNode("defect", "const", "wrong", "STRING", nil)}
	previous := NodeID("defect")
	for index := 1; index < size-1; index++ {
		id := NodeID("sequence_" + strconv.Itoa(index))
		nodes = append(nodes, testNode(id, "sequence", "", "", []transportEdge{{Role: "body", NodeID: previous}}))
		previous = id
	}
	nodes = append(nodes, testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: previous}}))
	return mustCandidate(t, candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program", Nodes: nodes})
}

func TestSemanticRepairTransactionFailsClosedForBoundaryAttacks(t *testing.T) {
	candidate := mustCandidate(t, candidateTransport{
		SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program",
		Nodes: []transportNode{
			testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "print"}}),
			testNode("message", "const", "wrong", "STRING", nil),
			testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "message"}}),
		},
	})
	context, err := NewSemanticRepairContext(candidate, []NodeID{"message"})
	if err != nil {
		t.Fatal(err)
	}
	validOperation := repairOperation{Operation: "replace_node", TargetNodeID: "message", ExpectedNodeHash: NodeHash(candidate.Graph.NodeByID("message")), Replacement: testNode("message", "const", "fixed", "STRING", nil)}
	valid := mustSemanticRepairDelta(t, candidate, context, []repairOperation{validOperation})
	protected := make(map[NodeID]string, len(context.ProtectedNodeHashes))
	for id, hash := range context.ProtectedNodeHashes {
		protected[id] = hash
	}
	protected["program"] = "stale"
	staleGraph, err := json.Marshal(repairTransport{SchemaVersion: SemanticRepairSchemaVersion, ExpectedGraphHash: "stale", ExpectedGraphVersion: "v1", ProtectedNodeHashes: context.ProtectedNodeHashes, Operations: []repairOperation{validOperation}})
	if err != nil {
		t.Fatal(err)
	}
	protectedMismatch, err := json.Marshal(repairTransport{SchemaVersion: SemanticRepairSchemaVersion, ExpectedGraphHash: candidate.Hash, ExpectedGraphVersion: "v1", ProtectedNodeHashes: protected, Operations: []repairOperation{validOperation}})
	if err != nil {
		t.Fatal(err)
	}
	outside := validOperation
	outside.TargetNodeID = "program"
	outside.ExpectedNodeHash = NodeHash(candidate.Graph.NodeByID("program"))
	outside.Replacement = testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "print"}})
	identity := validOperation
	identity.Replacement.ID = "program"
	authority := validOperation
	authority.Replacement = testNode("message", "env", "", "", []transportEdge{{Role: "value", NodeID: "message"}})
	widenedContext := context
	widenedContext.EditableNodeIDs = append(append([]NodeID(nil), context.EditableNodeIDs...), "program")
	cases := []struct {
		name     string
		context  RepairContext
		payload  []byte
		wantCode string
	}{
		{"stale graph", context, staleGraph, "HFIR_REPAIR_STALE"},
		{"protected hash changed", context, protectedMismatch, "HFIR_REPAIR_PROTECTED"},
		{"outside derived region", context, mustSemanticRepairDelta(t, candidate, context, []repairOperation{outside}), "HFIR_REPAIR_SCOPE"},
		{"context widens itself", widenedContext, valid, "HFIR_REPAIR_SCOPE"},
		{"node identity collision", context, mustSemanticRepairDelta(t, candidate, context, []repairOperation{identity}), "HFIR_REPAIR_IMMUTABLE"},
		{"capability self grant", context, mustSemanticRepairDelta(t, candidate, context, []repairOperation{authority}), "HFIR_REPAIR_IMMUTABLE"},
		{"backend injection", context, []byte(strings.Replace(string(valid), `"operations":`, `"opcode":"LOAD_CONST","operations":`, 1)), "HFIR_TRANSPORT_INVALID"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			updated, diagnostics := ApplyRepair(candidate, test.context, test.payload)
			if updated.Graph != nil || len(diagnostics) != 1 || diagnostics[0].Code != test.wantCode {
				t.Fatalf("ApplyRepair() = (%#v, %#v), want fail-closed %s", updated, diagnostics, test.wantCode)
			}
			if candidate.Graph.NodeByID("message").Value != "wrong" {
				t.Fatal("rejected repair mutated the original candidate")
			}
		})
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
