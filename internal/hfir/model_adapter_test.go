package hfir

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/vm"
)

func TestDecodeCandidateDirectPipelineAndArtifact(t *testing.T) {
	candidate := mustCandidate(t, candidateTransport{
		SchemaVersion: ModelAdapterSchemaVersion,
		GraphVersion:  "v1",
		EntryNode:     "program",
		Nodes: []transportNode{
			testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "print"}}),
			testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "number"}}),
			testNode("number", "const", "42", "INT", nil),
		},
	})
	if candidate.Graph == nil || candidate.Hash == "" {
		t.Fatalf("DecodeCandidate() = %#v, want canonical graph and hash", candidate)
	}
	program, diagnostics := CompileCandidate(candidate)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	var artifact bytes.Buffer
	if err := bytecode.WriteArtifact(&artifact, program); err != nil {
		t.Fatalf("WriteArtifact() error = %v", err)
	}
	program, err := bytecode.ReadArtifact(&artifact)
	if err != nil {
		t.Fatalf("ReadArtifact() error = %v", err)
	}
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	if exit := vm.RunBytecode(program, nil, nil, strings.NewReader(""), stdout, stderr); exit != 0 {
		t.Fatalf("RunBytecode() exit = %d, stderr = %q", exit, stderr.String())
	}
	if got := stdout.String(); got != "42\n" {
		t.Fatalf("stdout = %q, want 42\\n", got)
	}
}

func TestDecodeCandidateFailsClosedForAdversarialOutput(t *testing.T) {
	valid := candidateTransport{
		SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program",
		Nodes: []transportNode{
			testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "print"}}),
			testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "value"}}),
			testNode("value", "const", "ok", "STRING", nil),
		},
	}
	encoded, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, payload, code string }{
		{"malformed JSON", `{`, "HFIR_TRANSPORT_INVALID"},
		{"duplicate field", `{"schema_version":"hfir-model-adapter/v1","schema_version":"hfir-model-adapter/v1"}`, "HFIR_TRANSPORT_INVALID"},
		{"unknown field", strings.Replace(string(encoded), `"nodes":`, `"opcode":"LOAD_CONST","nodes":`, 1), "HFIR_TRANSPORT_INVALID"},
		{"unsupported function", strings.Replace(string(encoded), `"kind":"print"`, `"kind":"defun"`, 1), "HFIR_TRANSPORT_KIND"},
		{"capability self grant", strings.Replace(string(encoded), `"nodes":`, `"allowed_caps":["environment"],"nodes":`, 1), "HFIR_TRANSPORT_INVALID"},
		{"budget self grant", strings.Replace(string(encoded), `"nodes":`, `"max_instructions":999999,"nodes":`, 1), "HFIR_TRANSPORT_INVALID"},
		{"source injection field", strings.Replace(string(encoded), `"nodes":`, `"go_source":"package main","nodes":`, 1), "HFIR_TRANSPORT_INVALID"},
		{"wrong operand role", strings.Replace(string(encoded), `"role":"value"`, `"role":"opcode"`, 1), "HFIR_TRANSPORT_ROLE"},
		{"cycle", strings.Replace(string(encoded), `"value"}`, `"program"}`, 1), "HFIR_TRANSPORT_CYCLE"},
		{"malformed graph version", strings.Replace(string(encoded), `"graph_version":"v1"`, `"graph_version":"v999"`, 1), "HFIR_TRANSPORT_VERSION"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			candidate, diagnostics := DecodeCandidate([]byte(test.payload))
			if candidate.Graph != nil || len(diagnostics) != 1 || diagnostics[0].Code != test.code {
				t.Fatalf("DecodeCandidate() = (%#v, %#v), want rejection %s", candidate, diagnostics, test.code)
			}
		})
	}
}

func TestCandidateCannotGrantEnvironmentCapability(t *testing.T) {
	candidate := mustCandidate(t, candidateTransport{
		SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program",
		Nodes: []transportNode{
			testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "print"}}),
			testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "environment"}}),
			testNode("environment", "env", "", "", []transportEdge{{Role: "value", NodeID: "key"}}),
			testNode("key", "const", "HFIR_MODEL_ADAPTER_TEST", "STRING", nil),
		},
	})
	if diagnostics := NewVerifier(candidate.Graph, TargetBytecode).Verify(); len(diagnostics) != 0 {
		t.Fatalf("Verify() diagnostics = %#v", diagnostics)
	}
	if effects := candidate.Graph.NodeByID("environment").Effects; len(effects) != 1 || effects[0].Capability != string(capability.Environment) {
		t.Fatalf("inferred effects = %#v", effects)
	}
	program, diagnostics := CompileCandidate(candidate)
	if len(diagnostics) != 0 {
		t.Fatalf("CompileCandidate() diagnostics = %#v", diagnostics)
	}
	if len(program.Main) < 2 || bytecode.Registry[program.Main[1].Op].Capability != capability.Environment {
		t.Fatalf("environment proposal did not lower to a capability-gated instruction: %#v", program.Main)
	}
}

func TestApplyRepairIsBoundedAndReverifies(t *testing.T) {
	candidate := mustCandidate(t, candidateTransport{
		SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program",
		Nodes: []transportNode{
			testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "print"}}),
			testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "message"}}),
			testNode("message", "const", "helo", "STRING", nil),
		},
	})
	context, err := NewRepairContext(candidate, "message")
	if err != nil {
		t.Fatal(err)
	}
	delta, err := json.Marshal(repairTransport{
		SchemaVersion: ModelRepairSchemaVersion, ExpectedGraphHash: candidate.Hash,
		Operations: []repairOperation{{Operation: "replace_node", TargetNodeID: "message", ExpectedNodeHash: NodeHash(candidate.Graph.NodeByID("message")), Replacement: testNode("message", "const", "hello", "STRING", nil)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, diagnostics := ApplyRepair(candidate, context, delta)
	if len(diagnostics) != 0 {
		t.Fatalf("ApplyRepair() diagnostics = %#v", diagnostics)
	}
	program, diagnostics := CompileCandidate(updated)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	if exit := vm.RunBytecode(program, nil, nil, strings.NewReader(""), stdout, stderr); exit != 0 {
		t.Fatalf("exit %d: %s", exit, stderr.String())
	}
	if got := stdout.String(); got != "hello\n" {
		t.Fatalf("repair output = %q", got)
	}

	for _, bad := range []repairOperation{
		{Operation: "replace_node", TargetNodeID: "print", ExpectedNodeHash: NodeHash(candidate.Graph.NodeByID("print")), Replacement: testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: "message"}})},
		{Operation: "replace_node", TargetNodeID: "message", ExpectedNodeHash: "stale", Replacement: testNode("message", "const", "hello", "STRING", nil)},
		{Operation: "replace_node", TargetNodeID: "message", ExpectedNodeHash: NodeHash(candidate.Graph.NodeByID("message")), Replacement: testNode("new-id", "const", "hello", "STRING", nil)},
	} {
		payload, err := json.Marshal(repairTransport{SchemaVersion: ModelRepairSchemaVersion, ExpectedGraphHash: candidate.Hash, Operations: []repairOperation{bad}})
		if err != nil {
			t.Fatal(err)
		}
		if updated, diagnostics := ApplyRepair(candidate, context, payload); updated.Graph != nil || len(diagnostics) != 1 {
			t.Fatalf("unsafe repair = (%#v, %#v), want rejection", updated, diagnostics)
		}
	}
}

type fixedAdapter struct {
	candidate CandidateResponse
	repair    RepairResponse
}

func (a fixedAdapter) Generate(context.Context, GenerationRequest) (CandidateResponse, error) {
	return a.candidate, nil
}
func (a fixedAdapter) Repair(context.Context, RepairRequest) (RepairResponse, error) {
	return a.repair, nil
}

func TestAdapterInterfaceMetadataIsNotProgramSemantics(t *testing.T) {
	transport, err := json.Marshal(candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program", Nodes: []transportNode{testNode("program", "program", "", "", nil)}})
	if err != nil {
		t.Fatal(err)
	}
	adapter := fixedAdapter{candidate: CandidateResponse{Transport: transport, Metadata: AdapterMetadata{Provider: "test-double-a", Model: "one", Attempt: 1, Tokens: 10}}}
	response, err := adapter.Generate(context.Background(), GenerationRequest{Task: "empty program"})
	if err != nil {
		t.Fatal(err)
	}
	first := mustDecode(t, response.Transport)
	response.Metadata = AdapterMetadata{Provider: "test-double-b", Model: "two", Attempt: 99, Tokens: 999}
	second := mustDecode(t, response.Transport)
	if first.Hash != second.Hash {
		t.Fatalf("metadata changed program hash: %s != %s", first.Hash, second.Hash)
	}
}

func testNode(id NodeID, kind, value, literal string, inputs []transportEdge) transportNode {
	return transportNode{ID: id, Kind: kind, Value: value, LiteralKind: literal, Inputs: inputs, Provenance: transportOrigin{Label: "black-box"}}
}

func mustCandidate(t *testing.T, transport candidateTransport) Candidate {
	t.Helper()
	encoded, err := json.Marshal(transport)
	if err != nil {
		t.Fatal(err)
	}
	return mustDecode(t, encoded)
}
func mustDecode(t *testing.T, encoded []byte) Candidate {
	t.Helper()
	candidate, diagnostics := DecodeCandidate(encoded)
	if len(diagnostics) != 0 {
		t.Fatalf("DecodeCandidate() diagnostics = %#v", diagnostics)
	}
	return candidate
}
