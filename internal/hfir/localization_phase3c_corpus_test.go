package hfir

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/vm"
)

// phase3CScenario is deliberately test-only. GroundTruth and replacements are
// not available to LocalizeFailure, its repair context, or an external repair
// author. They make the scale corpus auditable without turning test labels into
// provenance inputs.
type phase3CScenario struct {
	name         string
	candidate    Candidate
	expected     string
	actual       string
	groundTruth  []NodeID
	replacements map[string]string
	multiNode    bool
}

// TestPhase3CLargeSemanticCorpusExecutes is a corpus gate, not a localization
// tuning test. It proves that each graph represents executed Phase-1 semantics
// and records the current localizer's result before any localization changes.
// The final output is intentionally wrong in exactly one semantic region.
func TestPhase3CLargeSemanticCorpusExecutes(t *testing.T) {
	for _, scenario := range phase3CLargeSemanticCorpus(t) {
		t.Run(scenario.name, func(t *testing.T) {
			if diagnostics := NewVerifier(scenario.candidate.Graph, TargetBytecode).Verify(); len(diagnostics) != 0 {
				t.Fatalf("Verify() diagnostics = %#v", diagnostics)
			}
			program, diagnostics := CompileCandidate(scenario.candidate)
			if len(diagnostics) != 0 {
				t.Fatalf("CompileCandidate() diagnostics = %#v", diagnostics)
			}
			var stdout bytes.Buffer
			evidence := vm.RunBytecodeWithEvidence(program, nil, vm.DefaultExecutionPolicy(), nil, strings.NewReader(""), &stdout, new(bytes.Buffer), 4096)
			if evidence.RuntimeFailure != nil {
				t.Fatalf("execution failure = %#v", evidence.RuntimeFailure)
			}
			if got := stdout.String(); got != scenario.actual {
				t.Fatalf("actual output = %q, want deterministic broken output %q", got, scenario.actual)
			}
			region, diagnostics := LocalizeFailure(scenario.candidate, LocalizationEvidence{
				GraphHash: scenario.candidate.Hash, GraphVersion: scenario.candidate.Graph.Version,
				Execution: &evidence, ExpectedOutcome: scenario.expected, ActualOutcome: stdout.String(),
			})
			if len(diagnostics) != 0 {
				t.Fatalf("LocalizeFailure() diagnostics = %#v", diagnostics)
			}

			// Read test-only truth only after production localization completes.
			matched := 0
			for _, id := range scenario.groundTruth {
				if containsNodeID(region.Context.EditableNodeIDs, id) {
					matched++
				}
			}
			fullGraph, err := json.Marshal(transportFromGraph(scenario.candidate.Graph))
			if err != nil {
				t.Fatal(err)
			}
			contextBytes, err := json.Marshal(region.Context)
			if err != nil {
				t.Fatal(err)
			}
			coreBytes, err := json.Marshal(region.Context.Nodes)
			if err != nil {
				t.Fatal(err)
			}
			closure := phase3CReverseDependencyClosure(scenario.candidate.Graph, scenario.groundTruth)
			deltaBytes := 0
			repaired := false
			if matched == len(scenario.groundTruth) {
				delta := restrictedAuthorDelta(t, scenario.candidate, region.Context, scenario.replacements)
				deltaBytes = len(delta)
				updated, diagnostics := ApplyRepair(scenario.candidate, region.Context, delta)
				if len(diagnostics) != 0 {
					t.Fatalf("ApplyRepair() diagnostics = %#v", diagnostics)
				}
				assertCandidateOutput(t, updated, scenario.expected)
				repaired = true
			}
			t.Logf("nodes=%d truth=%v editable=%v precision=%.3f recall=%.3f full_graph_bytes=%d context_bytes=%d core_bytes=%d delta_bytes=%d closure=%d/%d repaired=%t", len(scenario.candidate.Graph.Nodes), scenario.groundTruth, region.Context.EditableNodeIDs, float64(matched)/float64(len(region.Context.EditableNodeIDs)), float64(matched)/float64(len(scenario.groundTruth)), len(fullGraph), len(contextBytes), len(coreBytes), deltaBytes, len(closure), len(scenario.candidate.Graph.Nodes), repaired)
		})
	}
}

func phase3CLargeSemanticCorpus(t *testing.T) []phase3CScenario {
	t.Helper()
	return []phase3CScenario{
		phase3C25NodeStateAndListScenario(t),
		phase3C52NodeControlAndMapScenario(t),
		phase3C101NodeIndependentRegionsScenario(t),
	}
}

// The 26-node graph has four independent output regions. Its final arithmetic
// observation is wrong because both operands are wrong; map and list mutation
// regions execute before it and are not structural padding.
func phase3C25NodeStateAndListScenario(t *testing.T) phase3CScenario {
	t.Helper()
	b := phase3CBuilder{}
	math := b.arithmeticOutput("arithmetic", "2", "3")
	mapOutput := b.mapOutput("map", "status", "ready")
	listOutput := b.listAppendOutput("list", "a", "b", 1)
	defect := b.arithmeticOutput("region_c", "1", "2")
	candidate := b.candidate(t, "program", []NodeID{math, mapOutput, listOutput, defect})
	if got := len(candidate.Graph.Nodes); got != 26 {
		t.Fatalf("26-node corpus graph has %d nodes", got)
	}
	return phase3CScenario{
		name: "26 nodes: independent arithmetic, map, list, and multi-output defect", candidate: candidate,
		expected: "5\nready\nb\n5\n", actual: "5\nready\nb\n3\n", groundTruth: []NodeID{"region_c_left", "region_c_right"}, replacements: map[string]string{"1": "2", "2": "3"}, multiNode: true,
	}
}

// The ~50-node graph combines a set/read region, a list mutation/read region,
// nested executed conditionals, and multiple dictionary writers. Only the
// final map observation is wrong, and both its key and value require repair.
func phase3C52NodeControlAndMapScenario(t *testing.T) phase3CScenario {
	t.Helper()
	b := phase3CBuilder{}
	setOutput := b.setOutput("set", "seed", "next")
	listOutput := b.listAppendOutput("list", "left", "right", 1)
	nestedOutput := b.nestedConditionalOutput("nested", "nested")
	mathOutput := b.arithmeticOutput("arithmetic", "7", "8")
	mapDefect := b.multiWriterMapOutput("region_c", "wrong-key", "wrong-value")
	joinOutput := b.splitJoinOutput("join", "a,b", ",", "-")
	candidate := b.candidate(t, "program", []NodeID{setOutput, listOutput, nestedOutput, mathOutput, joinOutput, mapDefect})
	if got := len(candidate.Graph.Nodes); got < 45 || got > 60 {
		t.Fatalf("~50-node corpus graph has %d nodes", got)
	}
	return phase3CScenario{
		name: "~50 nodes: nested control, state mutation, multiple writers, and multi-output defect", candidate: candidate,
		expected: "next\nright\nnested\n15\na-b\nready\n", actual: "next\nright\nnested\n15\na-b\n\n", groundTruth: []NodeID{"region_c_update_key", "region_c_update_value"}, replacements: map[string]string{"wrong-key": "result", "wrong-value": "ready"}, multiNode: true,
	}
}

// The ~100-node graph intentionally has large unchanged semantic regions: nine
// arithmetic outputs, three independent map regions, two list regions, a set
// mutation/read, and a shared lexical binding. The final nested branch is the
// sole incorrect observation. This is the corpus's independent-subgraph probe.
func phase3C101NodeIndependentRegionsScenario(t *testing.T) phase3CScenario {
	t.Helper()
	b := phase3CBuilder{}
	var bodies []NodeID
	var expected strings.Builder
	var actual strings.Builder
	for index := 0; index < 9; index++ {
		prefix := fmt.Sprintf("arithmetic_%02d", index)
		bodies = append(bodies, b.arithmeticOutput(prefix, "2", "3"))
		expected.WriteString("5\n")
		actual.WriteString("5\n")
	}
	for index := 0; index < 3; index++ {
		prefix := fmt.Sprintf("map_%02d", index)
		bodies = append(bodies, b.mapOutput(prefix, fmt.Sprintf("key-%d", index), fmt.Sprintf("value-%d", index)))
		expected.WriteString(fmt.Sprintf("value-%d\n", index))
		actual.WriteString(fmt.Sprintf("value-%d\n", index))
	}
	for index := 0; index < 2; index++ {
		prefix := fmt.Sprintf("list_%02d", index)
		bodies = append(bodies, b.listAppendOutput(prefix, "start", fmt.Sprintf("item-%d", index), 1))
		expected.WriteString(fmt.Sprintf("item-%d\n", index))
		actual.WriteString(fmt.Sprintf("item-%d\n", index))
	}
	bodies = append(bodies, b.setOutput("set", "before", "after"))
	expected.WriteString("after\n")
	actual.WriteString("after\n")
	bodies = append(bodies, b.sharedSymbolOutput("shared_first", "shared"), b.sharedSymbolOutput("shared_second", "shared"))
	expected.WriteString("shared\nshared\n")
	actual.WriteString("shared\nshared\n")
	bodies = append(bodies, b.nestedConditionalArithmeticOutput("region_c", "1", "2"))
	expected.WriteString("5\n")
	actual.WriteString("3\n")

	sequence := b.node("shared_body", "sequence", "", "", b.edgesForBodies(bodies))
	sharedValue := b.node("shared_value", "const", "shared", "STRING", nil)
	shared := b.node("shared_binding", "let", "shared", "", b.edges("value", sharedValue, "body", sequence))
	candidate := b.candidate(t, "program", []NodeID{shared})
	if got := len(candidate.Graph.Nodes); got < 95 || got > 110 {
		t.Fatalf("~100-node corpus graph has %d nodes", got)
	}
	return phase3CScenario{
		name: "~100 nodes: independent regions, shared binding, and nested multi-node defect", candidate: candidate,
		expected: expected.String(), actual: actual.String(), groundTruth: []NodeID{"region_c_left", "region_c_right"}, replacements: map[string]string{"1": "2", "2": "3"}, multiNode: true,
	}
}

// phase3CReverseDependencyClosure is a test-only conservative work proxy. It
// follows canonical data-input consumers only and does not assert that current
// verification or lowering reuse any result; ApplyRepair still runs both in
// full. The test reads groundTruth only after localization, so this cannot
// influence the production repair region.
func phase3CReverseDependencyClosure(graph *Graph, changed []NodeID) map[NodeID]bool {
	closure := make(map[NodeID]bool, len(changed))
	queue := append([]NodeID(nil), changed...)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if closure[current] {
			continue
		}
		closure[current] = true
		for _, node := range graph.Nodes {
			for _, input := range node.DataInputs {
				if input.SourceNode == current {
					queue = append(queue, node.ID)
					break
				}
			}
		}
	}
	return closure
}

type phase3CBuilder struct{ nodes []transportNode }

func (b *phase3CBuilder) node(id NodeID, kind, value, literalKind string, inputs []transportEdge) NodeID {
	b.nodes = append(b.nodes, testNode(id, kind, value, literalKind, inputs))
	return id
}

func (b *phase3CBuilder) edges(parts ...any) []transportEdge {
	if len(parts)%2 != 0 {
		panic("phase3CBuilder edges require role/node pairs")
	}
	edges := make([]transportEdge, 0, len(parts)/2)
	for index := 0; index < len(parts); index += 2 {
		role, ok := parts[index].(string)
		if !ok {
			panic("phase3CBuilder edge role is not a string")
		}
		id, ok := parts[index+1].(NodeID)
		if !ok {
			panic("phase3CBuilder edge node is not a NodeID")
		}
		edges = append(edges, transportEdge{Role: role, NodeID: id})
	}
	return edges
}

func (b *phase3CBuilder) candidate(t *testing.T, entry NodeID, body []NodeID) Candidate {
	t.Helper()
	b.node(entry, "program", "", "", b.edgesForBodies(body))
	return mustCandidate(t, candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: entry, Nodes: b.nodes})
}

func (b *phase3CBuilder) edgesForBodies(body []NodeID) []transportEdge {
	edges := make([]transportEdge, 0, len(body))
	for _, id := range body {
		edges = append(edges, transportEdge{Role: "body", NodeID: id})
	}
	return edges
}

func (b *phase3CBuilder) arithmeticOutput(prefix, left, right string) NodeID {
	leftID := b.node(NodeID(prefix+"_left"), "const", left, "INT", nil)
	rightID := b.node(NodeID(prefix+"_right"), "const", right, "INT", nil)
	value := b.node(NodeID(prefix+"_value"), "binary", "+", "", b.edges("left", leftID, "right", rightID))
	return b.node(NodeID(prefix+"_print"), "print", "", "", b.edges("value", value))
}

func (b *phase3CBuilder) mapOutput(prefix, key, value string) NodeID {
	dict := b.node(NodeID(prefix+"_dict"), "dict", "", "", nil)
	keyID := b.node(NodeID(prefix+"_key"), "const", key, "STRING", nil)
	valueID := b.node(NodeID(prefix+"_value"), "const", value, "STRING", nil)
	write := b.node(NodeID(prefix+"_write"), "map_set", prefix+"_record", "", b.edges("key", keyID, "value", valueID))
	read := b.node(NodeID(prefix+"_read"), "map_get", prefix+"_record", "", b.edges("key", keyID))
	print := b.node(NodeID(prefix+"_print"), "print", "", "", b.edges("value", read))
	body := b.node(NodeID(prefix+"_body"), "sequence", "", "", b.edges("body", write, "body", print))
	return b.node(NodeID(prefix+"_binding"), "let", prefix+"_record", "", b.edges("value", dict, "body", body))
}

func (b *phase3CBuilder) multiWriterMapOutput(prefix, wrongKey, wrongValue string) NodeID {
	dict := b.node(NodeID(prefix+"_dict"), "dict", "", "", nil)
	goodKey := b.node(NodeID(prefix+"_good_key"), "const", "result", "STRING", nil)
	updateKey := b.node(NodeID(prefix+"_update_key"), "const", wrongKey, "STRING", nil)
	updateValue := b.node(NodeID(prefix+"_update_value"), "const", wrongValue, "STRING", nil)
	noiseKey := b.node(NodeID(prefix+"_noise_key"), "const", "noise", "STRING", nil)
	noiseValue := b.node(NodeID(prefix+"_noise_value"), "const", "ignored", "STRING", nil)
	update := b.node(NodeID(prefix+"_update"), "map_set", prefix+"_record", "", b.edges("key", updateKey, "value", updateValue))
	noise := b.node(NodeID(prefix+"_noise"), "map_set", prefix+"_record", "", b.edges("key", noiseKey, "value", noiseValue))
	read := b.node(NodeID(prefix+"_read"), "map_get", prefix+"_record", "", b.edges("key", goodKey))
	print := b.node(NodeID(prefix+"_print"), "print", "", "", b.edges("value", read))
	body := b.node(NodeID(prefix+"_body"), "sequence", "", "", b.edges("body", update, "body", noise, "body", print))
	return b.node(NodeID(prefix+"_binding"), "let", prefix+"_record", "", b.edges("value", dict, "body", body))
}

func (b *phase3CBuilder) listAppendOutput(prefix, initial, appended string, index int) NodeID {
	initialValue := b.node(NodeID(prefix+"_initial"), "const", initial, "STRING", nil)
	list := b.node(NodeID(prefix+"_list"), "list", "", "", b.edges("item", initialValue))
	item := b.node(NodeID(prefix+"_item"), "const", appended, "STRING", nil)
	appendNode := b.node(NodeID(prefix+"_append"), "append", prefix+"_items", "", b.edges("item", item))
	indexNode := b.node(NodeID(prefix+"_index"), "const", fmt.Sprintf("%d", index), "INT", nil)
	read := b.node(NodeID(prefix+"_read"), "list_get", prefix+"_items", "", b.edges("index", indexNode))
	print := b.node(NodeID(prefix+"_print"), "print", "", "", b.edges("value", read))
	body := b.node(NodeID(prefix+"_body"), "sequence", "", "", b.edges("body", appendNode, "body", print))
	return b.node(NodeID(prefix+"_binding"), "let", prefix+"_items", "", b.edges("value", list, "body", body))
}

func (b *phase3CBuilder) setOutput(prefix, initial, next string) NodeID {
	initialValue := b.node(NodeID(prefix+"_initial"), "const", initial, "STRING", nil)
	nextValue := b.node(NodeID(prefix+"_next"), "const", next, "STRING", nil)
	set := b.node(NodeID(prefix+"_set"), "set", prefix+"_value", "", b.edges("value", nextValue))
	read := b.node(NodeID(prefix+"_read"), "symbol", prefix+"_value", "", nil)
	print := b.node(NodeID(prefix+"_print"), "print", "", "", b.edges("value", read))
	body := b.node(NodeID(prefix+"_body"), "sequence", "", "", b.edges("body", set, "body", print))
	return b.node(NodeID(prefix+"_binding"), "let", prefix+"_value", "", b.edges("value", initialValue, "body", body))
}

func (b *phase3CBuilder) sharedSymbolOutput(prefix, name string) NodeID {
	value := b.node(NodeID(prefix+"_read"), "symbol", name, "", nil)
	return b.node(NodeID(prefix+"_print"), "print", "", "", b.edges("value", value))
}

func (b *phase3CBuilder) splitJoinOutput(prefix, input, splitSeparator, joinSeparator string) NodeID {
	inputID := b.node(NodeID(prefix+"_input"), "const", input, "STRING", nil)
	splitID := b.node(NodeID(prefix+"_split_separator"), "const", splitSeparator, "STRING", nil)
	joinID := b.node(NodeID(prefix+"_join_separator"), "const", joinSeparator, "STRING", nil)
	split := b.node(NodeID(prefix+"_split"), "str_split", "", "", b.edges("value", inputID, "separator", splitID))
	join := b.node(NodeID(prefix+"_join"), "str_join", "", "", b.edges("value", split, "separator", joinID))
	return b.node(NodeID(prefix+"_print"), "print", "", "", b.edges("value", join))
}

func (b *phase3CBuilder) nestedConditionalOutput(prefix, value string) NodeID {
	outerLeft := b.node(NodeID(prefix+"_outer_left"), "const", "1", "INT", nil)
	outerRight := b.node(NodeID(prefix+"_outer_right"), "const", "1", "INT", nil)
	outerCondition := b.node(NodeID(prefix+"_outer_condition"), "binary", "==", "", b.edges("left", outerLeft, "right", outerRight))
	innerLeft := b.node(NodeID(prefix+"_inner_left"), "const", "2", "INT", nil)
	innerRight := b.node(NodeID(prefix+"_inner_right"), "const", "2", "INT", nil)
	innerCondition := b.node(NodeID(prefix+"_inner_condition"), "binary", "==", "", b.edges("left", innerLeft, "right", innerRight))
	thenValue := b.node(NodeID(prefix+"_then_value"), "const", value, "STRING", nil)
	thenPrint := b.node(NodeID(prefix+"_then_print"), "print", "", "", b.edges("value", thenValue))
	innerElseValue := b.node(NodeID(prefix+"_inner_else_value"), "const", "inner-never", "STRING", nil)
	innerElsePrint := b.node(NodeID(prefix+"_inner_else_print"), "print", "", "", b.edges("value", innerElseValue))
	inner := b.node(NodeID(prefix+"_inner"), "if", "", "", b.edges("condition", innerCondition, "then", thenPrint, "else", innerElsePrint))
	outerElseValue := b.node(NodeID(prefix+"_outer_else_value"), "const", "outer-never", "STRING", nil)
	outerElsePrint := b.node(NodeID(prefix+"_outer_else_print"), "print", "", "", b.edges("value", outerElseValue))
	return b.node(NodeID(prefix+"_outer"), "if", "", "", b.edges("condition", outerCondition, "then", inner, "else", outerElsePrint))
}

func (b *phase3CBuilder) nestedConditionalArithmeticOutput(prefix, left, right string) NodeID {
	outerLeft := b.node(NodeID(prefix+"_outer_left"), "const", "1", "INT", nil)
	outerRight := b.node(NodeID(prefix+"_outer_right"), "const", "1", "INT", nil)
	outerCondition := b.node(NodeID(prefix+"_outer_condition"), "binary", "==", "", b.edges("left", outerLeft, "right", outerRight))
	innerLeft := b.node(NodeID(prefix+"_inner_left"), "const", "2", "INT", nil)
	innerRight := b.node(NodeID(prefix+"_inner_right"), "const", "2", "INT", nil)
	innerCondition := b.node(NodeID(prefix+"_inner_condition"), "binary", "==", "", b.edges("left", innerLeft, "right", innerRight))
	leftID := b.node(NodeID(prefix+"_left"), "const", left, "INT", nil)
	rightID := b.node(NodeID(prefix+"_right"), "const", right, "INT", nil)
	value := b.node(NodeID(prefix+"_value"), "binary", "+", "", b.edges("left", leftID, "right", rightID))
	thenPrint := b.node(NodeID(prefix+"_then_print"), "print", "", "", b.edges("value", value))
	innerElseValue := b.node(NodeID(prefix+"_inner_else_value"), "const", "inner-never", "STRING", nil)
	innerElsePrint := b.node(NodeID(prefix+"_inner_else_print"), "print", "", "", b.edges("value", innerElseValue))
	inner := b.node(NodeID(prefix+"_inner"), "if", "", "", b.edges("condition", innerCondition, "then", thenPrint, "else", innerElsePrint))
	outerElseValue := b.node(NodeID(prefix+"_outer_else_value"), "const", "outer-never", "STRING", nil)
	outerElsePrint := b.node(NodeID(prefix+"_outer_else_print"), "print", "", "", b.edges("value", outerElseValue))
	return b.node(NodeID(prefix+"_outer"), "if", "", "", b.edges("condition", outerCondition, "then", inner, "else", outerElsePrint))
}
