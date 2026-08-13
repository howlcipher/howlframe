package hfir

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/vm"
)

func TestAutomaticBehavioralLocalizationRepairsDictionaryWithoutManualTargets(t *testing.T) {
	candidate := semanticRepairDictionaryCandidate(t)
	program, diagnostics := CompileCandidate(candidate)
	if len(diagnostics) != 0 {
		t.Fatalf("CompileCandidate() diagnostics = %#v", diagnostics)
	}
	var stdout, stderr bytes.Buffer
	evidence := vm.RunBytecodeWithEvidence(program, nil, vm.DefaultExecutionPolicy(), nil, strings.NewReader(""), &stdout, &stderr, 64)
	if evidence.RuntimeFailure != nil || stdout.String() != "empty\n" {
		t.Fatalf("execution = %#v, stdout = %q, stderr = %q", evidence, stdout.String(), stderr.String())
	}
	region, diagnostics := LocalizeFailure(candidate, LocalizationEvidence{
		GraphHash: candidate.Hash, GraphVersion: candidate.Graph.Version, Execution: &evidence,
		ExpectedOutcome: "ready\n", ActualOutcome: stdout.String(),
	})
	if len(diagnostics) != 0 {
		t.Fatalf("LocalizeFailure() diagnostics = %#v", diagnostics)
	}
	// Test-only ground truth is evaluated after localization. It is not passed
	// into the localizer, repair context, or offline author fixture.
	groundTruth := []NodeID{"update_key", "update_value"}
	for _, id := range groundTruth {
		if !containsNodeID(region.Context.EditableNodeIDs, id) {
			t.Fatalf("automatic region = %#v, missing ground truth %q", region.Context.EditableNodeIDs, id)
		}
	}
	if len(region.Context.EditableNodeIDs) > 5 {
		t.Fatalf("automatic region size = %d, want a local region", len(region.Context.EditableNodeIDs))
	}

	// This is an external-model transcript payload. The test only proves that
	// its proposed nodes are inside the automatically derived context; it never
	// supplies their IDs to LocalizeFailure.
	delta := automaticDictionaryAuthorDelta(t, candidate, region.Context)
	updated, diagnostics := ApplyRepair(candidate, region.Context, delta)
	if len(diagnostics) != 0 {
		t.Fatalf("ApplyRepair() diagnostics = %#v", diagnostics)
	}
	assertCandidateOutput(t, updated, "ready\n")
}

func TestLocalizationUsesVerifiedRuntimeOriginAndRejectsAuthorityFailures(t *testing.T) {
	candidate := mustCandidate(t, candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program", Nodes: []transportNode{
		testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "print"}}),
		testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "environment"}}),
		testNode("environment", "env", "", "", []transportEdge{{Role: "value", NodeID: "key"}}),
		testNode("key", "const", "HFIR_LOCALIZATION_TEST", "STRING", nil),
	}})
	program, diagnostics := CompileCandidate(candidate)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	evidence := vm.RunBytecodeWithEvidence(program, nil, vm.DefaultExecutionPolicy(), nil, strings.NewReader(""), new(bytes.Buffer), new(bytes.Buffer), 32)
	if evidence.RuntimeFailure == nil || evidence.RuntimeFailure.Code != "CAPABILITY_DENIED" || evidence.RuntimeFailure.NodeID != "environment" {
		t.Fatalf("runtime evidence = %#v", evidence)
	}
	_, diagnostics = LocalizeFailure(candidate, LocalizationEvidence{GraphHash: candidate.Hash, GraphVersion: "v1", Execution: &evidence})
	if len(diagnostics) != 1 || diagnostics[0].Code != "HFIR_LOCALIZATION_AUTHORITY" {
		t.Fatalf("authority localization diagnostics = %#v", diagnostics)
	}

	// A structurally valid, trusted error against an ordinary runtime node does
	// localize to the lowerer-owned origin.
	evidence.RuntimeFailure = &bytecode.RuntimeFailure{Code: "TYPE_ERROR", Instruction: 1, NodeID: "environment"}
	region, diagnostics := LocalizeFailure(candidate, LocalizationEvidence{GraphHash: candidate.Hash, GraphVersion: "v1", Execution: &evidence})
	if len(diagnostics) != 0 || !containsNodeID(region.Context.EditableNodeIDs, "environment") {
		t.Fatalf("runtime region = %#v, diagnostics = %#v", region, diagnostics)
	}
}

func TestDerivedPhase1ControlRelationsAreRoleAware(t *testing.T) {
	candidate := mustCandidate(t, candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program", Nodes: []transportNode{
		testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "branch"}}),
		testNode("condition", "const", "1", "INT", nil), testNode("then", "print", "", "", []transportEdge{{Role: "value", NodeID: "yes"}}),
		testNode("else", "print", "", "", []transportEdge{{Role: "value", NodeID: "no"}}), testNode("yes", "const", "yes", "STRING", nil), testNode("no", "const", "no", "STRING", nil),
		testNode("branch", "if", "", "", []transportEdge{{Role: "condition", NodeID: "condition"}, {Role: "then", NodeID: "then"}, {Role: "else", NodeID: "else"}}),
	}})
	relations := DerivePhase1ControlRelations(candidate.Graph)
	if len(relations) != 3 {
		t.Fatalf("relations = %#v", relations)
	}
	if relations[0].Controller != "branch" || relations[0].Role != "then" || relations[1].Role != "else" {
		t.Fatalf("branch relations = %#v", relations)
	}
}

func TestLocalizationRejectsStaleAndForgedRuntimeMetadata(t *testing.T) {
	candidate := semanticRepairDictionaryCandidate(t)
	for _, evidence := range []LocalizationEvidence{
		{GraphHash: "stale", GraphVersion: "v1"},
		{GraphHash: candidate.Hash, GraphVersion: "v1", Execution: &bytecode.ExecutionEvidence{RuntimeFailure: &bytecode.RuntimeFailure{Code: "TYPE_ERROR", NodeID: "forged"}}},
	} {
		if _, diagnostics := LocalizeFailure(candidate, evidence); len(diagnostics) != 1 || (diagnostics[0].Code != "HFIR_LOCALIZATION_STALE" && diagnostics[0].Code != "HFIR_LOCALIZATION_PROVENANCE") {
			t.Fatalf("forged evidence diagnostics = %#v", diagnostics)
		}
	}
}

func TestAutomaticLocalizationStaysLocalAcrossGraphSizes(t *testing.T) {
	for _, size := range []int{10, 25, 50, 100} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			candidate := scaledBehavioralCandidate(t, size)
			program, diagnostics := CompileCandidate(candidate)
			if len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			var stdout bytes.Buffer
			evidence := vm.RunBytecodeWithEvidence(program, nil, vm.DefaultExecutionPolicy(), nil, strings.NewReader(""), &stdout, new(bytes.Buffer), 128)
			region, diagnostics := LocalizeFailure(candidate, LocalizationEvidence{GraphHash: candidate.Hash, GraphVersion: "v1", Execution: &evidence, ExpectedOutcome: "fixed\n", ActualOutcome: stdout.String()})
			if len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			if len(region.Context.EditableNodeIDs) > 3 {
				t.Fatalf("graph size %d localization region = %#v", size, region.Context.EditableNodeIDs)
			}
		})
	}
}

func scaledBehavioralCandidate(t *testing.T, size int) Candidate {
	t.Helper()
	nodes := []transportNode{
		testNode("defect", "const", "wrong", "STRING", nil),
		testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "defect"}}),
	}
	body := NodeID("print")
	for index := 0; index < size-3; index++ {
		id := NodeID("sequence_" + strconv.Itoa(index))
		nodes = append(nodes, testNode(id, "sequence", "", "", []transportEdge{{Role: "body", NodeID: body}}))
		body = id
	}
	nodes = append(nodes, testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: body}}))
	return mustCandidate(t, candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program", Nodes: nodes})
}

func automaticDictionaryAuthorDelta(t *testing.T, candidate Candidate, context RepairContext) []byte {
	t.Helper()
	operations := []repairOperation{
		{Operation: "replace_node", TargetNodeID: "update_key", ExpectedNodeHash: NodeHash(candidate.Graph.NodeByID("update_key")), Replacement: testNode("update_key", "const", "result", "STRING", nil)},
		{Operation: "replace_node", TargetNodeID: "update_value", ExpectedNodeHash: NodeHash(candidate.Graph.NodeByID("update_value")), Replacement: testNode("update_value", "const", "ready", "STRING", nil)},
	}
	for _, operation := range operations {
		if !containsNodeID(context.EditableNodeIDs, operation.TargetNodeID) {
			t.Fatalf("offline author proposed outside automatic context: %q", operation.TargetNodeID)
		}
	}
	data, err := json.Marshal(repairTransport{SchemaVersion: SemanticRepairSchemaVersion, ExpectedGraphHash: candidate.Hash, ExpectedGraphVersion: candidate.Graph.Version, ProtectedNodeHashes: context.ProtectedNodeHashes, Operations: operations})
	if err != nil {
		t.Fatal(err)
	}
	return data
}
