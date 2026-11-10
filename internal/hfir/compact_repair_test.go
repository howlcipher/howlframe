package hfir

import (
	"bytes"
	"encoding/json"
	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/vm"
	"strings"
	"testing"
	"time"
)

func compactFixture(t *testing.T) (Candidate, RepairContext, *CompactRepairSession) {
	t.Helper()
	c := programCandidateWithBody(t, "print", []transportNode{testNode("defect", "const", "wrong", "STRING", nil), testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "defect"}})})
	ctx, err := NewLocalizedRepairContext(c, []NodeID{"defect"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewCompactRepairSession(c, ctx)
	if err != nil {
		t.Fatal(err)
	}
	return c, ctx, s
}

func compactReply(t *testing.T, s *CompactRepairSession, c Candidate, target NodeID, value string) []byte {
	t.Helper()
	var request struct {
		ContextID string `json:"context_id"`
	}
	if err := json.Unmarshal(s.RequestJSON(), &request); err != nil {
		t.Fatal(err)
	}
	n := c.Graph.NodeByID(target)
	b, err := json.Marshal(compactRepairResponse{SchemaVersion: CompactRepairSchemaVersion, ContextID: request.ContextID, Operations: []repairOperation{{Operation: "replace_node", TargetNodeID: target, ExpectedNodeHash: NodeHash(n), Replacement: testNode(target, n.Kind, value, n.LiteralKind, nil)}}})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCompactRepairRoundTripAndAuthorityIsolation(t *testing.T) {
	c, ctx, s := compactFixture(t)
	before := GraphHash(c.Graph)
	originalRequest := s.RequestJSON()
	altered := s.RequestJSON()
	altered[0] = 'x'
	if !bytes.Equal(originalRequest, s.RequestJSON()) {
		t.Fatal("request aliases private state")
	}
	ctx.CoreNodeIDs[0] = "program"
	ctx.EditableNodeIDs[0] = "program"
	ctx.Nodes[0].Value = "alias"
	delete(ctx.ProtectedNodeHashes, "program")
	updated, ds := s.Apply(c, compactReply(t, s, c, "defect", "fixed"))
	if len(ds) != 0 {
		t.Fatalf("repair rejected: %#v", ds)
	}
	assertCandidateOutput(t, updated, "fixed\n")
	if GraphHash(c.Graph) != before {
		t.Fatal("original graph mutated")
	}
	for _, n := range c.Graph.Nodes {
		if n.ID != "defect" && NodeHash(n) != NodeHash(updated.Graph.NodeByID(n.ID)) {
			t.Fatalf("protected %s changed", n.ID)
		}
	}
	if _, ds := CompileCandidate(updated); len(ds) != 0 {
		t.Fatalf("recompile: %#v", ds)
	}
	if bytes.Contains(originalRequest, []byte("protected_node_hashes")) {
		t.Fatal("noncompact protected map exported")
	}
}

func TestCompactRepairRejectsUntrustedContext(t *testing.T) {
	c, ctx, _ := compactFixture(t)
	ctx.Nodes[0].Value = "forged"
	if _, err := NewCompactRepairSession(c, ctx); err == nil {
		t.Fatal("mutated context accepted")
	}
	var decoded RepairContext
	good, _ := NewLocalizedRepairContext(c, []NodeID{"defect"})
	data, _ := json.Marshal(good)
	_ = json.Unmarshal(data, &decoded)
	if _, err := NewCompactRepairSession(c, decoded); err == nil {
		t.Fatal("JSON context granted authority")
	}
}

func TestCompactRepairRejectsInvalidResponsesAtomically(t *testing.T) {
	c, _, s := compactFixture(t)
	valid := compactReply(t, s, c, "defect", "fixed")
	var response compactRepairResponse
	_ = json.Unmarshal(valid, &response)
	stale := response
	stale.ContextID = "altered"
	staleBytes, _ := json.Marshal(stale)
	outOfScope := compactReply(t, s, c, "program", "")
	badSecond := response
	badSecond.Operations = append(append([]repairOperation(nil), response.Operations...), response.Operations[0])
	badSecond.Operations[1].ExpectedNodeHash = "stale"
	badSecondBytes, _ := json.Marshal(badSecond)
	kind := response
	kind.Operations = append([]repairOperation(nil), response.Operations...)
	kind.Operations[0].Replacement.Kind = "print"
	kindBytes, _ := json.Marshal(kind)
	refs := response
	refs.Operations = append([]repairOperation(nil), response.Operations...)
	refs.Operations[0].Replacement.Inputs = []transportEdge{{Role: "value", NodeID: "program"}}
	refsBytes, _ := json.Marshal(refs)
	six := response
	six.Operations = nil
	for range 6 {
		six.Operations = append(six.Operations, response.Operations[0])
	}
	sixBytes, _ := json.Marshal(six)
	zero := response
	zero.Operations = nil
	zeroBytes, _ := json.Marshal(zero)
	for name, data := range map[string][]byte{"id": staleBytes, "scope": outOfScope, "atomic": badSecondBytes, "kind": kindBytes, "references": refsBytes, "six": sixBytes, "zero": zeroBytes, "unknown": []byte(strings.TrimSuffix(string(valid), "}") + `,"authority":true}`), "duplicate": []byte(strings.TrimSuffix(string(valid), "}") + `,"context_id":"x"}`), "trailing": append(append([]byte(nil), valid...), []byte(` {}`)...), "utf8": append([]byte{'"', 0xff, '"'}, valid...), "oversize": []byte(strings.Repeat(" ", maxTransportBytes+1)), "empty": nil} {
		t.Run(name, func(t *testing.T) {
			before := GraphHash(c.Graph)
			updated, ds := s.Apply(c, data)
			if updated.Graph != nil || len(ds) == 0 {
				t.Fatal("invalid response accepted")
			}
			if GraphHash(c.Graph) != before {
				t.Fatal("rejected operation mutated candidate")
			}
		})
	}
	changed := scaledRepairCandidate(t, 26)
	if updated, ds := s.Apply(changed, valid); updated.Graph != nil || len(ds) == 0 {
		t.Fatal("stale candidate accepted")
	}
	var empty CompactRepairSession
	if _, ds := empty.Apply(c, valid); len(ds) == 0 {
		t.Fatal("zero session accepted")
	}
}

func TestCompactRepairRejectsLossyCandidateText(t *testing.T) {
	c, _, _ := compactFixture(t)
	c.Graph.NodeByID("defect").Value = string([]byte{0xff})
	c.Hash = GraphHash(c.Graph)
	ctx, err := NewLocalizedRepairContext(c, []NodeID{"defect"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewCompactRepairSession(c, ctx); err == nil {
		t.Fatal("malformed candidate text accepted")
	}
}

func TestCompactRepairRejectsDifferentSessionAndAllowsFiveNodes(t *testing.T) {
	nodes := []transportNode{testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "a"}, {Role: "value", NodeID: "b"}, {Role: "value", NodeID: "c"}, {Role: "value", NodeID: "d"}, {Role: "value", NodeID: "e"}})}
	ids := []NodeID{"a", "b", "c", "d", "e"}
	for _, id := range ids {
		nodes = append(nodes, testNode(id, "const", "wrong", "STRING", nil))
	}
	c := programCandidateWithBody(t, "print", nodes)
	ctx, err := NewLocalizedRepairContext(c, ids)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewCompactRepairSession(c, ctx)
	if err != nil {
		t.Fatal(err)
	}
	var request compactRepairRequest
	if err := json.Unmarshal(s.RequestJSON(), &request); err != nil {
		t.Fatal(err)
	}
	response := compactRepairResponse{SchemaVersion: CompactRepairSchemaVersion, ContextID: request.ContextID}
	for _, node := range request.Nodes {
		replacement := node.Node
		replacement.Value = "fixed"
		response.Operations = append(response.Operations, repairOperation{Operation: "replace_node", TargetNodeID: replacement.ID, ExpectedNodeHash: node.ExpectedNodeHash, Replacement: replacement})
	}
	data, _ := json.Marshal(response)
	updated, ds := s.Apply(c, data)
	if len(ds) != 0 {
		t.Fatalf("five operations rejected: %#v", ds)
	}
	assertCandidateOutput(t, updated, "fixed fixed fixed fixed fixed\n")
	narrower, _ := NewLocalizedRepairContext(c, []NodeID{"a"})
	other, err := NewCompactRepairSession(c, narrower)
	if err != nil {
		t.Fatal(err)
	}
	if updated, ds := other.Apply(c, data); updated.Graph != nil || len(ds) == 0 {
		t.Fatal("different session authority accepted")
	}
}

// Payload totals compare context+delta boundaries only. Common task, diagnostic,
// provider metadata and outer transport framing are excluded from BOTH sides.
// This is not a measurement of a complete provider prompt or model efficiency.
func TestCompactRepairCorpusPayloads(t *testing.T) {
	for _, scenario := range phase3CLargeSemanticCorpus(t) {
		t.Run(scenario.name, func(t *testing.T) {
			c := scenario.candidate
			program, ds := CompileCandidate(c)
			if len(ds) != 0 {
				t.Fatal(ds)
			}
			var stdout bytes.Buffer
			evidence := vm.RunBytecodeWithEvidence(program, nil, vm.DefaultExecutionPolicy(), nil, strings.NewReader(""), &stdout, new(bytes.Buffer), 4096)
			region, ds := LocalizeFailure(c, LocalizationEvidence{GraphHash: c.Hash, GraphVersion: c.Graph.Version, Execution: &evidence, ExpectedOutcome: scenario.expected, ActualOutcome: stdout.String()})
			if len(ds) != 0 && !(len(ds) == 1 && ds[0].Code == "HFIR_LOCALIZATION_AMBIGUOUS_STATE_CAUSE") {
				t.Fatal(ds)
			}
			full, _ := json.Marshal(transportFromGraph(c.Graph))
			oldContext, _ := json.Marshal(region.Context)
			if len(ds) != 0 {
				// Ambiguous localization deliberately returns no repair authority.
				// A compact session must not manufacture a core for measurement.
				if region.Context.GraphHash != "" || len(region.Context.EditableNodeIDs) != 0 {
					t.Fatal("ambiguous localization unexpectedly granted authority")
				}
				if session, err := NewCompactRepairSession(c, region.Context); err == nil || session != nil {
					t.Fatal("ambiguous localization created authorized session")
				}
				t.Logf("nodes=%d full_graph_bytes=%d repair=unavailable localization=%s compact_request=unavailable unchanged_authority=true", len(c.Graph.Nodes), len(full), ds[0].Code)
				return
			}
			started := time.Now()
			session, err := NewCompactRepairSession(c, region.Context)
			constructTime := time.Since(started)
			if err != nil {
				t.Fatal(err)
			}
			var request compactRepairRequest
			if err := json.Unmarshal(session.RequestJSON(), &request); err != nil {
				t.Fatal(err)
			}
			matched := 0
			for _, id := range scenario.groundTruth {
				if containsNodeID(region.Context.EditableNodeIDs, id) {
					matched++
				}
			}
			if matched != len(scenario.groundTruth) {
				t.Logf("nodes=%d full_graph_bytes=%d old_context_bytes=%d compact_request_bytes=%d retained_context_bytes=%d repair=unavailable unchanged_authority=true construct=%s", len(c.Graph.Nodes), len(full), len(oldContext), len(session.RequestJSON()), len(oldContext), constructTime)
				return
			}
			oldDelta := restrictedAuthorDelta(t, c, region.Context, scenario.replacements)
			var parsed repairTransport
			if err := json.Unmarshal(oldDelta, &parsed); err != nil {
				t.Fatal(err)
			}
			compactDelta, err := json.Marshal(compactRepairResponse{SchemaVersion: CompactRepairSchemaVersion, ContextID: request.ContextID, Operations: parsed.Operations})
			if err != nil {
				t.Fatal(err)
			}
			started = time.Now()
			updated, ds := session.Apply(c, compactDelta)
			applyTime := time.Since(started)
			if len(ds) != 0 {
				t.Fatal(ds)
			}
			assertCandidateOutput(t, updated, scenario.expected)
			t.Logf("nodes=%d full_graph_bytes=%d old_context_bytes=%d old_delta_bytes=%d old_boundary_total=%d compact_request_bytes=%d compact_reply_bytes=%d compact_boundary_total=%d retained_context_bytes=%d construct=%s apply_full_gate=%s repaired=true", len(c.Graph.Nodes), len(full), len(oldContext), len(oldDelta), len(oldContext)+len(oldDelta), len(session.RequestJSON()), len(compactDelta), len(session.RequestJSON())+len(compactDelta), len(oldContext), constructTime, applyTime)
		})
	}
}

func TestCompactRepairRejectsLoneSurrogateEscapes(t *testing.T) {
	c, _, s := compactFixture(t)
	valid := compactReply(t, s, c, "defect", "fixed")
	for name, escaped := range map[string]string{"high": `\ud800`, "low": `\udc00`, "high_then_text": `\ud800x`, "high_then_high": `\ud800\ud800`} {
		t.Run(name, func(t *testing.T) {
			data := bytes.Replace(valid, []byte(`"fixed"`), []byte(`"`+escaped+`"`), 1)
			if !json.Valid(data) {
				t.Fatal("test must exercise syntactically valid JSON")
			}
			before := GraphHash(c.Graph)
			if updated, ds := s.Apply(c, data); updated.Graph != nil || len(ds) == 0 {
				t.Fatal("lossy surrogate response accepted")
			}
			if GraphHash(c.Graph) != before {
				t.Fatal("rejected response mutated original")
			}
		})
	}
	for name, escaped := range map[string]string{"pair": `\ud83d\ude00`, "replacement_character": `\ufffd`, "escaped_backslash": `\\ud800`} {
		t.Run(name, func(t *testing.T) {
			data := bytes.Replace(valid, []byte(`"fixed"`), []byte(`"`+escaped+`"`), 1)
			var decoded compactRepairResponse
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatal(err)
			}
			updated, ds := s.Apply(c, data)
			if len(ds) != 0 {
				t.Fatalf("lossless string rejected: %#v", ds)
			}
			assertCandidateOutput(t, updated, decoded.Operations[0].Replacement.Value+"\n")
		})
	}
}

func TestCompactRepairRetainsNestedTypeMetadataAndDeterministicRequests(t *testing.T) {
	c, _, _ := compactFixture(t)
	c.Graph.NodeByID("print").DataInputs[0].Type.Fields = map[string]ast.TypeInfo{"nested": {Name: "original"}}
	c.Hash = GraphHash(c.Graph)
	ctx, err := NewLocalizedRepairContext(c, []NodeID{"defect", "print"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := NewCompactRepairSession(c, ctx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewCompactRepairSession(c, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.RequestJSON(), second.RequestJSON()) {
		t.Fatal("independent sessions produced nondeterministic requests")
	}
	for _, view := range ctx.Nodes {
		if view.ID == "print" {
			view.Inputs[0].Type.Fields["nested"] = ast.TypeInfo{Name: "mutated"}
		}
	}
	for _, view := range first.context.Nodes {
		if view.ID == "print" && view.Inputs[0].Type.Fields["nested"].Name != "original" {
			t.Fatal("caller nested metadata aliases retained authority")
		}
	}
}

// Standard benchmarks report constructor/retention, request-copy and full-gate
// apply costs separately. Run repeated fixed-count observations; no model gain.
func BenchmarkCompactRepairSession(b *testing.B) {
	encoded, err := json.Marshal(candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program", Nodes: []transportNode{testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "print"}}), testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "defect"}}), testNode("defect", "const", "wrong", "STRING", nil)}})
	if err != nil {
		b.Fatal(err)
	}
	c, ds := DecodeCandidate(encoded)
	if len(ds) != 0 {
		b.Fatal(ds)
	}
	ctx, err := NewLocalizedRepairContext(c, []NodeID{"defect"})
	if err != nil {
		b.Fatal(err)
	}
	session, err := NewCompactRepairSession(c, ctx)
	if err != nil {
		b.Fatal(err)
	}
	reply, err := json.Marshal(compactRepairResponse{SchemaVersion: CompactRepairSchemaVersion, ContextID: session.id, Operations: []repairOperation{{Operation: "replace_node", TargetNodeID: "defect", ExpectedNodeHash: NodeHash(c.Graph.NodeByID("defect")), Replacement: testNode("defect", "const", "fixed", "STRING", nil)}}})
	if err != nil {
		b.Fatal(err)
	}
	b.Run("construct", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := NewCompactRepairSession(c, ctx); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("request_copy", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if len(session.RequestJSON()) == 0 {
				b.Fatal("empty request")
			}
		}
	})
	b.Run("apply_full_gate", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, ds := session.Apply(c, reply); len(ds) != 0 {
				b.Fatal(ds)
			}
		}
	})
}
