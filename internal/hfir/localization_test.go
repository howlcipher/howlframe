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

	// A caller cannot relabel trusted authority evidence as a semantic error.
	// Mutating any exported trace projection invalidates the runner seal.
	evidence.RuntimeFailure = &bytecode.RuntimeFailure{Code: "TYPE_ERROR", Instruction: 1, NodeID: "environment"}
	_, diagnostics = LocalizeFailure(candidate, LocalizationEvidence{GraphHash: candidate.Hash, GraphVersion: "v1", Execution: &evidence})
	if len(diagnostics) != 1 || diagnostics[0].Code != "HFIR_LOCALIZATION_PROVENANCE" {
		t.Fatalf("mutated runtime evidence diagnostics = %#v", diagnostics)
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

func TestLocalizationPrunesUnexecutedBranchAndUsesReadKeyLastWriter(t *testing.T) {
	t.Run("unexecuted branch", func(t *testing.T) {
		candidate := mustCandidate(t, candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program", Nodes: []transportNode{
			testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "if"}}),
			testNode("left", "const", "1", "INT", nil), testNode("right", "const", "1", "INT", nil),
			testNode("condition", "binary", "==", "", []transportEdge{{Role: "left", NodeID: "left"}, {Role: "right", NodeID: "right"}}),
			testNode("shared", "const", "wrong", "STRING", nil),
			testNode("then_print", "print", "", "", []transportEdge{{Role: "value", NodeID: "shared"}}),
			testNode("else_value", "const", "never", "STRING", nil),
			testNode("else_print", "print", "", "", []transportEdge{{Role: "value", NodeID: "shared"}, {Role: "value", NodeID: "else_value"}}),
			testNode("if", "if", "", "", []transportEdge{{Role: "condition", NodeID: "condition"}, {Role: "then", NodeID: "then_print"}, {Role: "else", NodeID: "else_print"}}),
		}})
		region := localizeBehavior(t, candidate, "fixed\n")
		if got, want := region.Context.EditableNodeIDs, []NodeID{"shared"}; !sameNodeIDSlice(got, want) {
			t.Fatalf("editable executed region = %#v, want %#v", got, want)
		}
		for _, forbidden := range []NodeID{"else_print", "else_value", "condition"} {
			if containsNodeID(region.Context.EditableNodeIDs, forbidden) {
				t.Fatalf("unexecuted or contextual node %q is editable: %#v", forbidden, region.Context.EditableNodeIDs)
			}
		}
	})

	t.Run("matching map writer", func(t *testing.T) {
		candidate := mustCandidate(t, candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program", Nodes: []transportNode{
			testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "bind"}}),
			testNode("dict", "dict", "", "", nil),
			testNode("wanted", "const", "wanted", "STRING", nil), testNode("other", "const", "other", "STRING", nil),
			testNode("correct", "const", "good", "STRING", nil), testNode("unrelated", "const", "noise", "STRING", nil),
			testNode("write_wanted", "map_set", "record", "", []transportEdge{{Role: "key", NodeID: "wanted"}, {Role: "value", NodeID: "correct"}}),
			testNode("write_other", "map_set", "record", "", []transportEdge{{Role: "key", NodeID: "other"}, {Role: "value", NodeID: "unrelated"}}),
			testNode("read", "map_get", "record", "", []transportEdge{{Role: "key", NodeID: "wanted"}}),
			testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "read"}}),
			testNode("body", "sequence", "", "", []transportEdge{{Role: "body", NodeID: "write_wanted"}, {Role: "body", NodeID: "write_other"}, {Role: "body", NodeID: "print"}}),
			testNode("bind", "let", "record", "", []transportEdge{{Role: "value", NodeID: "dict"}, {Role: "body", NodeID: "body"}}),
		}})
		region := localizeBehavior(t, candidate, "fixed\n")
		for _, required := range []NodeID{"wanted", "correct"} {
			if !containsNodeID(region.Context.EditableNodeIDs, required) {
				t.Fatalf("matching writer dependency %q missing from %#v", required, region.Context.EditableNodeIDs)
			}
		}
		for _, forbidden := range []NodeID{"other", "unrelated", "write_other"} {
			if containsNodeID(region.Context.EditableNodeIDs, forbidden) {
				t.Fatalf("unrelated-key writer leaked %q into %#v", forbidden, region.Context.EditableNodeIDs)
			}
		}
	})
}

func TestLocalizationRejectsForgedCanonicalEvidenceAndLimitFailures(t *testing.T) {
	candidate := scaledBehavioralCandidate(t, 10)
	for _, evidence := range []LocalizationEvidence{
		{GraphHash: candidate.Hash, GraphVersion: "v1", Execution: &bytecode.ExecutionEvidence{Trace: []bytecode.ExecutionTraceEvent{{Instruction: 0, Opcode: "PRINT", NodeID: "defect"}}}, ExpectedOutcome: "fixed\n", ActualOutcome: "wrong\n"},
		{GraphHash: candidate.Hash, GraphVersion: "v1", Execution: &bytecode.ExecutionEvidence{RuntimeFailure: &bytecode.RuntimeFailure{Code: "TYPE_ERROR", NodeID: "defect"}}},
	} {
		if _, diagnostics := LocalizeFailure(candidate, evidence); len(diagnostics) != 1 || diagnostics[0].Code != "HFIR_LOCALIZATION_PROVENANCE" {
			t.Fatalf("forged canonical evidence diagnostics = %#v", diagnostics)
		}
	}
	program, diagnostics := CompileCandidate(candidate)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	evidence := vm.RunBytecodeWithEvidence(program, nil, vm.ExecutionPolicy{Limits: vm.VMLimits{MaxInstructions: 1}}, nil, strings.NewReader(""), new(bytes.Buffer), new(bytes.Buffer), 16)
	if _, diagnostics := LocalizeFailure(candidate, LocalizationEvidence{GraphHash: candidate.Hash, GraphVersion: "v1", Execution: &evidence}); len(diagnostics) != 1 || diagnostics[0].Code != "HFIR_LOCALIZATION_AUTHORITY" {
		t.Fatalf("limit evidence diagnostics = %#v", diagnostics)
	}
}

func TestBehavioralLocalizationCorpusMeasuresPrecisionRecallAndAutomaticRepair(t *testing.T) {
	// GroundTruth and replacements are test-only oracle metadata. Localization
	// runs before either is inspected. The restricted author derives target IDs
	// only by scanning the automatically supplied core views for task values.
	scenarios := []struct {
		name         string
		candidate    Candidate
		expected     string
		groundTruth  []NodeID
		replacements map[string]string
		multiNode    bool
	}{
		{"dictionary write", semanticRepairDictionaryCandidate(t), "ready\n", []NodeID{"update_key", "update_value"}, map[string]string{"wrong": "result", "nope": "ready"}, true},
		{"binary expression", binaryOutputCandidate(t, "1", "2", ""), "5\n", []NodeID{"left", "right"}, map[string]string{"1": "2", "2": "3"}, true},
		{"list item and separator", listJoinCandidate(t), "a-b\n", []NodeID{"item_a", "separator"}, map[string]string{"x": "a", ",": "-"}, true},
		{"split then join", splitJoinCandidate(t), "a-b\n", []NodeID{"join_separator"}, map[string]string{".": "-"}, false},
		{"executed branch output", binaryOutputCandidate(t, "1", "2", "if"), "5\n", []NodeID{"left", "right"}, map[string]string{"1": "2", "2": "3"}, true},
		{"nested expression", nestedBinaryCandidate(t), "10\n", []NodeID{"left", "right"}, map[string]string{"1": "2", "2": "3"}, true},
		{"matching map writer", mapWriterCandidate(t), "fixed\n", []NodeID{"value"}, map[string]string{"wrong": "fixed"}, false},
		{"direct output", scaledBehavioralCandidate(t, 10), "fixed\n", []NodeID{"defect"}, map[string]string{"wrong": "fixed"}, false},
	}
	var selected, relevant, repaired, multi int
	var macroPrecision, macroRecall float64
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			region := localizeBehavior(t, scenario.candidate, scenario.expected)
			// Read truth only after the production localization call.
			matched := 0
			for _, id := range scenario.groundTruth {
				if containsNodeID(region.Context.EditableNodeIDs, id) {
					matched++
				}
			}
			if matched != len(scenario.groundTruth) {
				t.Fatalf("core %#v misses test-only truth %#v", region.Context.EditableNodeIDs, scenario.groundTruth)
			}
			selected += len(region.Context.EditableNodeIDs)
			relevant += matched
			macroPrecision += float64(matched) / float64(len(region.Context.EditableNodeIDs))
			macroRecall += float64(matched) / float64(len(scenario.groundTruth))
			if scenario.multiNode {
				multi++
				delta := restrictedAuthorDelta(t, scenario.candidate, region.Context, scenario.replacements)
				updated, diagnostics := ApplyRepair(scenario.candidate, region.Context, delta)
				if len(diagnostics) != 0 {
					t.Fatalf("restricted repair diagnostics = %#v", diagnostics)
				}
				assertCandidateOutput(t, updated, scenario.expected)
				repaired++
			}
		})
	}
	if len(scenarios) != 8 || multi != 5 || repaired != 5 {
		t.Fatalf("corpus counts scenarios=%d multi=%d repaired=%d", len(scenarios), multi, repaired)
	}
	if got := float64(relevant) / float64(selected); got < 0.65 {
		t.Fatalf("pooled precision = %.3f, want materially above Phase-3 baseline", got)
	}
	if macroPrecision/float64(len(scenarios)) < 0.70 || macroRecall/float64(len(scenarios)) != 1 {
		t.Fatalf("macro precision=%.3f recall=%.3f", macroPrecision/float64(len(scenarios)), macroRecall/float64(len(scenarios)))
	}
}

func restrictedAuthorDelta(t *testing.T, candidate Candidate, context RepairContext, replacements map[string]string) []byte {
	t.Helper()
	operations := make([]repairOperation, 0, len(replacements))
	for _, node := range context.Nodes {
		if replacement, ok := replacements[node.Value]; ok {
			operations = append(operations, repairOperation{Operation: "replace_node", TargetNodeID: node.ID, ExpectedNodeHash: NodeHash(candidate.Graph.NodeByID(node.ID)), Replacement: testNode(node.ID, node.Kind, replacement, candidate.Graph.NodeByID(node.ID).LiteralKind, nil)})
		}
	}
	if len(operations) != len(replacements) {
		t.Fatalf("restricted author saw core %#v but could not find all task values", context.Nodes)
	}
	return mustSemanticRepairDelta(t, candidate, context, operations)
}

func binaryOutputCandidate(t *testing.T, left, right, branch string) Candidate {
	t.Helper()
	nodes := []transportNode{
		testNode("left", "const", left, "INT", nil), testNode("right", "const", right, "INT", nil),
		testNode("value", "binary", "+", "", []transportEdge{{Role: "left", NodeID: "left"}, {Role: "right", NodeID: "right"}}),
		testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "value"}}),
	}
	body := NodeID("print")
	if branch != "" {
		nodes = append(nodes,
			testNode("condition_left", "const", "1", "INT", nil), testNode("condition_right", "const", "1", "INT", nil),
			testNode("condition", "binary", "==", "", []transportEdge{{Role: "left", NodeID: "condition_left"}, {Role: "right", NodeID: "condition_right"}}),
			testNode("else_value", "const", "never", "STRING", nil), testNode("else_print", "print", "", "", []transportEdge{{Role: "value", NodeID: "else_value"}}),
			testNode("branch", "if", "", "", []transportEdge{{Role: "condition", NodeID: "condition"}, {Role: "then", NodeID: "print"}, {Role: "else", NodeID: "else_print"}}),
		)
		body = "branch"
	}
	nodes = append(nodes, testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: body}}))
	return mustCandidate(t, candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program", Nodes: nodes})
}

func listJoinCandidate(t *testing.T) Candidate {
	t.Helper()
	return mustCandidate(t, candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program", Nodes: []transportNode{
		testNode("item_a", "const", "x", "STRING", nil), testNode("item_b", "const", "b", "STRING", nil), testNode("separator", "const", ",", "STRING", nil),
		testNode("list", "list", "", "", []transportEdge{{Role: "item", NodeID: "item_a"}, {Role: "item", NodeID: "item_b"}}),
		testNode("join", "str_join", "", "", []transportEdge{{Role: "value", NodeID: "list"}, {Role: "separator", NodeID: "separator"}}),
		testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "join"}}), testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "print"}}),
	}})
}

func splitJoinCandidate(t *testing.T) Candidate {
	t.Helper()
	return mustCandidate(t, candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program", Nodes: []transportNode{
		testNode("input", "const", "a,b", "STRING", nil), testNode("split_separator", "const", ",", "STRING", nil), testNode("join_separator", "const", ".", "STRING", nil),
		testNode("split", "str_split", "", "", []transportEdge{{Role: "value", NodeID: "input"}, {Role: "separator", NodeID: "split_separator"}}),
		testNode("join", "str_join", "", "", []transportEdge{{Role: "value", NodeID: "split"}, {Role: "separator", NodeID: "join_separator"}}), testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "join"}}), testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "print"}}),
	}})
}

func nestedBinaryCandidate(t *testing.T) Candidate {
	t.Helper()
	return mustCandidate(t, candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program", Nodes: []transportNode{
		testNode("left", "const", "1", "INT", nil), testNode("right", "const", "2", "INT", nil), testNode("tail", "const", "5", "INT", nil),
		testNode("inner", "binary", "+", "", []transportEdge{{Role: "left", NodeID: "left"}, {Role: "right", NodeID: "right"}}), testNode("value", "binary", "+", "", []transportEdge{{Role: "left", NodeID: "inner"}, {Role: "right", NodeID: "tail"}}), testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "value"}}), testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "print"}}),
	}})
}

func mapWriterCandidate(t *testing.T) Candidate {
	t.Helper()
	return mustCandidate(t, candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program", Nodes: []transportNode{
		testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "bind"}}), testNode("dict", "dict", "", "", nil), testNode("key", "const", "key", "STRING", nil), testNode("value", "const", "wrong", "STRING", nil),
		testNode("write", "map_set", "record", "", []transportEdge{{Role: "key", NodeID: "key"}, {Role: "value", NodeID: "value"}}), testNode("read", "map_get", "record", "", []transportEdge{{Role: "key", NodeID: "key"}}), testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "read"}}), testNode("body", "sequence", "", "", []transportEdge{{Role: "body", NodeID: "write"}, {Role: "body", NodeID: "print"}}), testNode("bind", "let", "record", "", []transportEdge{{Role: "value", NodeID: "dict"}, {Role: "body", NodeID: "body"}}),
	}})
}

func localizeBehavior(t *testing.T, candidate Candidate, expected string) CandidateRepairRegion {
	t.Helper()
	program, diagnostics := CompileCandidate(candidate)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	var stdout bytes.Buffer
	evidence := vm.RunBytecodeWithEvidence(program, nil, vm.DefaultExecutionPolicy(), nil, strings.NewReader(""), &stdout, new(bytes.Buffer), 128)
	if evidence.RuntimeFailure != nil {
		t.Fatalf("execution evidence = %#v", evidence)
	}
	region, diagnostics := LocalizeFailure(candidate, LocalizationEvidence{GraphHash: candidate.Hash, GraphVersion: candidate.Graph.Version, Execution: &evidence, ExpectedOutcome: expected, ActualOutcome: stdout.String()})
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	return region
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
