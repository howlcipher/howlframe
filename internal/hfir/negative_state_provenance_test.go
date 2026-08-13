package hfir

import (
	"bytes"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/vm"
)

type negativeMapWrite struct{ key, value string }

// TestNegativeStateProvenanceCorpus is intentionally a state-semantic corpus,
// not fixture-name guidance. Ground truth is checked only after localization.
func TestNegativeStateProvenanceCorpus(t *testing.T) {
	cases := []struct {
		name           string
		writes         []negativeMapWrite
		deleteWanted   bool
		rewriteWanted  bool
		postRead       bool
		branchWrite    bool
		expected       string
		wantCode       string
		wantCause      string
		wantCandidates []NodeID
		repair         bool
		multiNode      bool
	}{
		{"unique wrong key among unrelated writers", []negativeMapWrite{{"wrong", "ready"}, {"noise", "ignored"}}, false, false, false, false, "ready", "", "VALUE_MATCH", []NodeID{"write_0"}, true, false},
		{"requested key never written", nil, false, false, false, false, "ready", "HFIR_LOCALIZATION_AMBIGUOUS_STATE_CAUSE", "NEVER_PRESENT", nil, false, false},
		{"requested key written then deleted", []negativeMapWrite{{"result", "ready"}}, true, false, false, false, "ready", "", "DELETED", []NodeID{"delete_wanted"}, true, false},
		{"requested key deleted then correctly rewritten", []negativeMapWrite{{"result", "old"}}, true, true, false, false, "ready", "HFIR_LOCALIZATION_UNAVAILABLE", "", nil, false, false},
		{"two wrong keys carry expected value", []negativeMapWrite{{"wrong_a", "ready"}, {"wrong_b", "ready"}}, false, false, false, false, "ready", "HFIR_LOCALIZATION_AMBIGUOUS_STATE_CAUSE", "VALUE_MATCH", []NodeID{"write_0", "write_1"}, false, true},
		{"expected writer plus later unrelated writer", []negativeMapWrite{{"wrong", "ready"}, {"noise", "ignored"}}, false, false, false, false, "ready", "", "VALUE_MATCH", []NodeID{"write_0"}, true, false},
		{"writer after failing read", []negativeMapWrite{{"wrong", "ready"}}, false, false, true, false, "ready", "HFIR_LOCALIZATION_AMBIGUOUS_STATE_CAUSE", "NEVER_PRESENT", nil, false, true},
		{"writer in unexecuted branch", []negativeMapWrite{{"wrong", "ready"}}, false, false, false, true, "ready", "HFIR_LOCALIZATION_AMBIGUOUS_STATE_CAUSE", "NEVER_PRESENT", nil, false, true},
	}
	for _, scenario := range cases {
		t.Run(scenario.name, func(t *testing.T) {
			candidate := negativeMapCandidate(t, scenario.writes, scenario.deleteWanted, scenario.rewriteWanted, scenario.postRead, scenario.branchWrite)
			region, diagnostics := localizeNegativeMap(t, candidate, scenario.expected)
			if scenario.wantCode == "" {
				if len(diagnostics) != 0 {
					t.Fatalf("LocalizeFailure() diagnostics = %#v", diagnostics)
				}
			} else if len(diagnostics) != 1 || diagnostics[0].Code != scenario.wantCode {
				t.Fatalf("LocalizeFailure() diagnostics = %#v, want %s", diagnostics, scenario.wantCode)
			}
			if scenario.wantCause == "" {
				if region.NegativeState != nil {
					t.Fatalf("unexpected absence witness %#v", region.NegativeState)
				}
				return
			}
			if region.NegativeState == nil || region.NegativeState.Cause != scenario.wantCause || !sameNodeIDSlice(region.NegativeState.CandidateNodeIDs, scenario.wantCandidates) {
				t.Fatalf("negative witness = %#v, want cause=%s candidates=%#v", region.NegativeState, scenario.wantCause, scenario.wantCandidates)
			}
			if scenario.wantCode != "" && len(region.Context.EditableNodeIDs) != 0 {
				t.Fatalf("ambiguous absence granted edit authority %#v", region.Context.EditableNodeIDs)
			}
			if scenario.repair {
				replacements := map[string]string{"wrong": "result"}
				if scenario.wantCause == "DELETED" {
					replacements = map[string]string{"result": "preserve"}
				}
				delta := restrictedAuthorDelta(t, candidate, region.Context, replacements)
				updated, repairDiagnostics := ApplyRepair(candidate, region.Context, delta)
				if len(repairDiagnostics) != 0 {
					t.Fatalf("ApplyRepair() diagnostics = %#v", repairDiagnostics)
				}
				assertCandidateOutput(t, updated, "ready\n")
			}
		})
	}
}

func TestNegativeStateProvenanceRejectsTamperingAndOverflow(t *testing.T) {
	candidate := negativeMapCandidate(t, []negativeMapWrite{{"wrong", "ready"}}, false, false, false, false)
	program, diagnostics := CompileCandidate(candidate)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	var stdout bytes.Buffer
	evidence := vm.RunBytecodeWithEvidence(program, nil, vm.DefaultExecutionPolicy(), nil, strings.NewReader(""), &stdout, new(bytes.Buffer), 256)
	oracle := bytecode.NewExpectedValueObservation("ready")
	input := LocalizationEvidence{GraphHash: candidate.Hash, GraphVersion: candidate.Graph.Version, Execution: &evidence, ExpectedOutcome: "ready\n", ActualOutcome: stdout.String(), ExpectedValue: &oracle}
	if region, diagnostics := LocalizeFailure(candidate, input); len(diagnostics) != 0 || !containsNodeID(region.Context.EditableNodeIDs, "key_0") {
		t.Fatalf("baseline negative localization = %#v, %#v", region, diagnostics)
	}

	t.Run("forged ledger event", func(t *testing.T) {
		forged := evidence
		forged.MapStateEvents = append([]bytecode.MapStateEvent(nil), evidence.MapStateEvents...)
		forged.MapStateEvents[len(forged.MapStateEvents)-1].KeyFingerprint = "forged"
		_, diagnostics := LocalizeFailure(candidate, LocalizationEvidence{GraphHash: candidate.Hash, GraphVersion: candidate.Graph.Version, Execution: &forged, ExpectedOutcome: "ready\n", ActualOutcome: stdout.String(), ExpectedValue: &oracle})
		if len(diagnostics) != 1 || diagnostics[0].Code != "HFIR_LOCALIZATION_PROVENANCE" {
			t.Fatalf("forged ledger diagnostics = %#v", diagnostics)
		}
	})

	t.Run("forged expected observation cannot grant authority", func(t *testing.T) {
		forged := oracle
		forged.Value = "forged"
		region, diagnostics := LocalizeFailure(candidate, LocalizationEvidence{GraphHash: candidate.Hash, GraphVersion: candidate.Graph.Version, Execution: &evidence, ExpectedOutcome: "forged\n", ActualOutcome: stdout.String(), ExpectedValue: &forged})
		if len(diagnostics) != 1 || diagnostics[0].Code != "HFIR_LOCALIZATION_AMBIGUOUS_STATE_CAUSE" || len(region.Context.EditableNodeIDs) != 0 {
			t.Fatalf("forged expected result = %#v, %#v", region, diagnostics)
		}
	})

	t.Run("bytecode identical graph cannot reuse evidence", func(t *testing.T) {
		graph := *candidate.Graph
		graph.Nodes = append([]*Node(nil), candidate.Graph.Nodes...)
		graph.nodeMap = nil
		clone := *graph.NodeByID("key_0")
		clone.Module = "different-canonical-module"
		for index, node := range graph.Nodes {
			if node.ID == clone.ID {
				graph.Nodes[index] = &clone
			}
		}
		stale := Candidate{Graph: &graph, Hash: GraphHash(&graph)}
		_, diagnostics := LocalizeFailure(stale, LocalizationEvidence{GraphHash: stale.Hash, GraphVersion: stale.Graph.Version, Execution: &evidence, ExpectedOutcome: "ready\n", ActualOutcome: stdout.String(), ExpectedValue: &oracle})
		if len(diagnostics) != 1 || diagnostics[0].Code != "HFIR_LOCALIZATION_PROVENANCE" {
			t.Fatalf("stale graph diagnostics = %#v", diagnostics)
		}
	})

	t.Run("candidate overflow fails closed", func(t *testing.T) {
		writes := []negativeMapWrite{{"a", "ready"}, {"b", "ready"}, {"c", "ready"}, {"d", "ready"}, {"e", "ready"}}
		overflow := negativeMapCandidate(t, writes, false, false, false, false)
		region, diagnostics := localizeNegativeMap(t, overflow, "ready")
		if len(diagnostics) != 1 || diagnostics[0].Code != "HFIR_LOCALIZATION_BOUNDS" || len(region.Context.EditableNodeIDs) != 0 {
			t.Fatalf("overflow result = %#v, %#v", region, diagnostics)
		}
	})
}

// Three coordinated key-construction repairs prove that the precise
// wrong-key rule is not restricted to one literal-node edit.
func TestNegativeStateProvenanceRepairsMultiNodeWrongKeys(t *testing.T) {
	for _, parts := range [][]string{{"wr", "ong"}, {"no", "pe"}, {"bad", "key"}} {
		t.Run(strings.Join(parts, "+"), func(t *testing.T) {
			candidate := computedWrongKeyCandidate(t, parts)
			region, diagnostics := localizeNegativeMap(t, candidate, "ready")
			if len(diagnostics) != 0 || region.NegativeState == nil || region.NegativeState.Cause != "VALUE_MATCH" {
				t.Fatalf("multi-node localization = %#v, %#v", region, diagnostics)
			}
			replacements := map[string]string{parts[0]: "res", parts[1]: "ult"}
			delta := restrictedAuthorDelta(t, candidate, region.Context, replacements)
			updated, repairDiagnostics := ApplyRepair(candidate, region.Context, delta)
			if len(repairDiagnostics) != 0 {
				t.Fatalf("ApplyRepair() diagnostics = %#v", repairDiagnostics)
			}
			assertCandidateOutput(t, updated, "ready\n")
		})
	}
}

func computedWrongKeyCandidate(t *testing.T, parts []string) Candidate {
	t.Helper()
	b := phase3CBuilder{}
	dict := b.node("dict", "dict", "", "", nil)
	partIDs := make([]NodeID, 0, len(parts))
	for index, part := range parts {
		partIDs = append(partIDs, b.node(NodeID("part_"+string(rune('0'+index))), "const", part, "STRING", nil))
	}
	items := b.node("items", "list", "", "", b.edgesForItems(partIDs))
	separator := b.node("separator", "const", "", "STRING", nil)
	key := b.node("wrong_key", "str_join", "", "", b.edges("value", items, "separator", separator))
	value := b.node("value", "const", "ready", "STRING", nil)
	write := b.node("write", "map_set", "record", "", b.edges("key", key, "value", value))
	wanted := b.node("wanted", "const", "result", "STRING", nil)
	read := b.node("read", "map_get", "record", "", b.edges("key", wanted))
	print := b.node("print", "print", "", "", b.edges("value", read))
	body := b.node("body", "sequence", "", "", b.edges("body", write, "body", print))
	bind := b.node("bind", "let", "record", "", b.edges("value", dict, "body", body))
	return b.candidate(t, "program", []NodeID{bind})
}

func (b *phase3CBuilder) edgesForItems(items []NodeID) []transportEdge {
	edges := make([]transportEdge, 0, len(items))
	for _, item := range items {
		edges = append(edges, transportEdge{Role: "item", NodeID: item})
	}
	return edges
}

func localizeNegativeMap(t *testing.T, candidate Candidate, expected string) (CandidateRepairRegion, []Diagnostic) {
	t.Helper()
	program, diagnostics := CompileCandidate(candidate)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	var stdout bytes.Buffer
	evidence := vm.RunBytecodeWithEvidence(program, nil, vm.DefaultExecutionPolicy(), nil, strings.NewReader(""), &stdout, new(bytes.Buffer), 256)
	if evidence.RuntimeFailure != nil {
		t.Fatal(evidence.RuntimeFailure)
	}
	oracle := bytecode.NewExpectedValueObservation(expected)
	return LocalizeFailure(candidate, LocalizationEvidence{GraphHash: candidate.Hash, GraphVersion: candidate.Graph.Version, Execution: &evidence, ExpectedOutcome: expected + "\n", ActualOutcome: stdout.String(), ExpectedValue: &oracle})
}

func negativeMapCandidate(t *testing.T, writes []negativeMapWrite, deleteWanted, rewriteWanted, postRead, branchWrite bool) Candidate {
	t.Helper()
	b := phase3CBuilder{}
	dict := b.node("dict", "dict", "", "", nil)
	wanted := b.node("wanted", "const", "result", "STRING", nil)
	var before, after []NodeID
	for index, write := range writes {
		key := b.node(NodeID("key_"+string(rune('0'+index))), "const", write.key, "STRING", nil)
		value := b.node(NodeID("value_"+string(rune('0'+index))), "const", write.value, "STRING", nil)
		set := b.node(NodeID("write_"+string(rune('0'+index))), "map_set", "record", "", b.edges("key", key, "value", value))
		if postRead {
			after = append(after, set)
		} else if branchWrite && index == 0 {
			left := b.node("branch_left", "const", "1", "INT", nil)
			right := b.node("branch_right", "const", "2", "INT", nil)
			condition := b.node("branch_condition", "binary", "==", "", b.edges("left", left, "right", right))
			empty := b.node("branch_else", "sequence", "", "", nil)
			branch := b.node("branch", "if", "", "", b.edges("condition", condition, "then", set, "else", empty))
			before = append(before, branch)
		} else {
			before = append(before, set)
		}
	}
	if deleteWanted {
		deleteKey := b.node("delete_key", "const", "result", "STRING", nil)
		before = append(before, b.node("delete_wanted", "map_delete", "record", "", b.edges("key", deleteKey)))
	}
	if rewriteWanted {
		value := b.node("rewrite_value", "const", "ready", "STRING", nil)
		before = append(before, b.node("rewrite_wanted", "map_set", "record", "", b.edges("key", wanted, "value", value)))
	}
	read := b.node("read", "map_get", "record", "", b.edges("key", wanted))
	print := b.node("print", "print", "", "", b.edges("value", read))
	body := append(before, print)
	body = append(body, after...)
	sequence := b.node("body", "sequence", "", "", b.edgesForBodies(body))
	bind := b.node("bind", "let", "record", "", b.edges("value", dict, "body", sequence))
	return b.candidate(t, "program", []NodeID{bind})
}
