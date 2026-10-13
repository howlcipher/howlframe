package hfir

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/vm"
)

func TestAdapterNodeKindsAllHaveRoles(t *testing.T) {
	kinds := AdapterNodeKinds()
	seen := map[string]bool{}
	for _, kind := range kinds {
		if seen[kind] {
			t.Errorf("duplicate kind %s", kind)
		}
		seen[kind] = true
		if nodeRoleAlternatives(kind) == nil {
			t.Errorf("kind %s is listed but has no roles", kind)
		}
	}
	for _, kind := range []string{"spawn", "spawn_agent", "grants", "llm_generate", "http_server", "res", "store_open", "empty_list", "type_hint", "achieve"} {
		if seen[kind] || nodeRoleAlternatives(kind) != nil {
			t.Errorf("kind %s must stay outside the adapter", kind)
		}
	}
}

func TestValidRolesOrderedShapes(t *testing.T) {
	e := func(roles ...string) []transportEdge {
		var out []transportEdge
		for _, r := range roles {
			out = append(out, transportEdge{Role: r, NodeID: "x"})
		}
		return out
	}
	cases := []struct {
		kind  string
		roles []string
		ok    bool
	}{
		{"exec", []string{"cmd"}, true},
		{"exec", []string{"cmd", "arg", "arg"}, true},
		{"exec", []string{"arg", "cmd"}, false},
		{"exec", nil, false},
		{"defun", []string{"param", "param", "body", "body"}, true},
		{"defun", []string{"body", "param"}, false},
		{"fetch", []string{"url", "method"}, true},
		{"fetch", []string{"url", "method", "body"}, true},
		{"fetch", []string{"url", "method", "body", "body"}, false},
		{"return", nil, true},
		{"return", []string{"value", "value"}, false},
		{"map_get", []string{"key"}, true},
		{"map_get", []string{"target", "key"}, true},
		{"map_get", []string{"key", "target"}, false},
		{"cli_args", nil, true},
		{"cli_args", []string{"index"}, true},
		{"try", []string{"expression", "success_body", "catch"}, true},
		{"try", []string{"expression", "catch", "success_body"}, false},
	}
	for _, c := range cases {
		if _, got := matchingRoles(c.kind, e(c.roles...)); got != c.ok {
			t.Errorf("%s %v: got %v want %v", c.kind, c.roles, got, c.ok)
		}
	}
}

func coverageCandidate(t *testing.T, nodes ...transportNode) Candidate {
	t.Helper()
	data, err := json.Marshal(candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program", Nodes: nodes})
	if err != nil {
		t.Fatal(err)
	}
	candidate, diagnostics := DecodeCandidate(data)
	if len(diagnostics) != 0 {
		t.Fatalf("DecodeCandidate: %#v", diagnostics)
	}
	return candidate
}

func runCandidate(t *testing.T, candidate Candidate, caps []capability.Capability) (string, string, int) {
	t.Helper()
	program, diagnostics := CompileCandidate(candidate)
	if len(diagnostics) != 0 {
		t.Fatalf("CompileCandidate: %#v", diagnostics)
	}
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	evidence := vm.RunBytecodeWithEvidence(program, nil, vm.DefaultExecutionPolicy(), caps, strings.NewReader(""), stdout, stderr, 0)
	if evidence.RuntimeFailure != nil {
		return stdout.String(), stderr.String() + fmt.Sprint(evidence.RuntimeFailure), 1
	}
	return stdout.String(), stderr.String(), evidence.ExitCode
}

// A model can now express loops and functions; they run as written.
func TestAdapterLoopsAndFunctionsRun(t *testing.T) {
	candidate := coverageCandidate(t,
		testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "fn"}, {Role: "body", NodeID: "loop"}}),
		testNode("fn", "defun", "double", "", []transportEdge{{Role: "param", NodeID: "p"}, {Role: "body", NodeID: "ret"}}),
		testNode("p", "param", "n", "", nil),
		testNode("ret", "return", "", "", []transportEdge{{Role: "value", NodeID: "mul"}}),
		testNode("mul", "binary", "*", "", []transportEdge{{Role: "left", NodeID: "n"}, {Role: "right", NodeID: "two"}}),
		testNode("n", "symbol", "n", "", nil),
		testNode("two", "const", "2", "INT", nil),
		testNode("loop", "for", "x", "", []transportEdge{{Role: "iterable", NodeID: "xs"}, {Role: "body", NodeID: "pr"}}),
		testNode("xs", "list", "", "", []transportEdge{{Role: "item", NodeID: "one"}, {Role: "item", NodeID: "three"}}),
		testNode("one", "const", "1", "INT", nil),
		testNode("three", "const", "3", "INT", nil),
		testNode("pr", "print", "", "", []transportEdge{{Role: "value", NodeID: "call"}}),
		testNode("call", "call", "double", "", []transportEdge{{Role: "arg", NodeID: "x"}}),
		testNode("x", "symbol", "x", "", nil),
	)
	out, stderr, exit := runCandidate(t, candidate, nil)
	if exit != 0 || out != "2\n6\n" {
		t.Fatalf("exit %d out %q stderr %q", exit, out, stderr)
	}
}

// Effectful kinds reached through the adapter carry no authority: with no
// grant every one of them is denied at run time and nothing runs.
func TestAdapterGovernedKindsFailClosedWithoutGrant(t *testing.T) {
	str := func(id NodeID, v string) transportNode { return testNode(id, "const", v, "STRING", nil) }
	cases := map[string][]transportNode{
		"exec":       {testNode("eff", "exec", "", "", []transportEdge{{Role: "cmd", NodeID: "a"}, {Role: "arg", NodeID: "b"}}), str("a", "/bin/echo"), str("b", "pwned")},
		"fetch":      {testNode("eff", "fetch", "", "", []transportEdge{{Role: "url", NodeID: "a"}, {Role: "method", NodeID: "b"}}), str("a", "http://127.0.0.1:9/"), str("b", "GET")},
		"read_file":  {testNode("eff", "read_file", "", "", []transportEdge{{Role: "path", NodeID: "a"}}), str("a", "/etc/hostname")},
		"write_file": {testNode("eff", "write_file", "", "", []transportEdge{{Role: "path", NodeID: "a"}, {Role: "data", NodeID: "b"}}), str("a", "/tmp/hfir-adapter-denied"), str("b", "x")},
		"mkdir":      {testNode("eff", "mkdir", "", "", []transportEdge{{Role: "path", NodeID: "a"}}), str("a", "/tmp/hfir-adapter-denied-dir")},
		"env":        {testNode("eff", "env", "", "", []transportEdge{{Role: "value", NodeID: "a"}}), str("a", "HOME")},
	}
	for name, nodes := range cases {
		t.Run(name, func(t *testing.T) {
			all := append([]transportNode{
				testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "pr"}}),
				testNode("pr", "print", "", "", []transportEdge{{Role: "value", NodeID: "eff"}}),
			}, nodes...)
			if name == "write_file" || name == "mkdir" {
				all[0] = testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "eff"}})
				all = append(all[:1], all[2:]...)
			}
			out, stderr, exit := runCandidate(t, coverageCandidate(t, all...), nil)
			if exit == 0 || out != "" || !strings.Contains(strings.ToLower(stderr+out), "denied") {
				t.Fatalf("must fail closed: exit %d out %q stderr %q", exit, out, stderr)
			}
		})
	}
}

// sleep's graph edge is unnamed; the transport names it duration and the
// round trip restores the graph exactly.
func TestAdapterSleepRoleRoundTrip(t *testing.T) {
	candidate := coverageCandidate(t,
		testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "s"}}),
		testNode("s", "sleep", "", "", []transportEdge{{Role: "duration", NodeID: "d"}}),
		testNode("d", "const", "0", "INT", nil),
	)
	if got := candidate.Graph.NodeByID("s").DataInputs[0].Name; got != "" {
		t.Fatalf("graph edge = %q, want unnamed", got)
	}
	data, err := EncodeCandidateTransport(candidate.Graph)
	if err != nil || !strings.Contains(string(data), `"role":"duration"`) {
		t.Fatalf("encode: %v %s", err, data)
	}
	if _, _, exit := runCandidate(t, candidate, nil); exit != 0 {
		t.Fatalf("sleep 0 exit %d", exit)
	}
}

func decodeCode(t *testing.T, nodes ...transportNode) string {
	t.Helper()
	data, _ := json.Marshal(candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program", Nodes: nodes})
	candidate, diagnostics := DecodeCandidate(data)
	if candidate.Graph != nil || len(diagnostics) != 1 {
		t.Fatalf("want one rejection, got graph=%v %#v", candidate.Graph != nil, diagnostics)
	}
	return diagnostics[0].Code
}

// Review blocker: a binary a<->b cycle used to overflow the stack in type
// inference before the cycle check ran.
func TestAdapterBinaryCycleRejectedBeforeTypes(t *testing.T) {
	code := decodeCode(t,
		testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "pr"}}),
		testNode("pr", "print", "", "", []transportEdge{{Role: "value", NodeID: "a"}}),
		testNode("a", "binary", "+", "", []transportEdge{{Role: "left", NodeID: "b"}, {Role: "right", NodeID: "b"}}),
		testNode("b", "binary", "+", "", []transportEdge{{Role: "left", NodeID: "a"}, {Role: "right", NodeID: "a"}}),
	)
	if code != "HFIR_TRANSPORT_CYCLE" {
		t.Fatalf("code = %s", code)
	}
}

func TestAdapterReturnOutsideDefunRejected(t *testing.T) {
	cases := map[string][]transportNode{
		"in loop body": {
			testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "w"}}),
			testNode("w", "while", "", "", []transportEdge{{Role: "condition", NodeID: "c"}, {Role: "body", NodeID: "r"}}),
			testNode("c", "const", "true", "BOOL", nil),
			testNode("r", "return", "", "", []transportEdge{{Role: "value", NodeID: "v"}}),
			testNode("v", "const", "1", "INT", nil),
		},
		"as an operand": {
			testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "pr"}}),
			testNode("pr", "print", "", "", []transportEdge{{Role: "value", NodeID: "r"}}),
			testNode("r", "return", "", "", []transportEdge{{Role: "value", NodeID: "v"}}),
			testNode("v", "const", "1", "INT", nil),
		},
	}
	for name, nodes := range cases {
		if code := decodeCode(t, nodes...); code != "HFIR_TRANSPORT_RETURN" {
			t.Fatalf("%s: code = %s", name, code)
		}
	}
	// A straight-line main-level return is valid source and stays accepted.
	coverageCandidate(t,
		testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "r"}}),
		testNode("r", "return", "", "", []transportEdge{{Role: "value", NodeID: "v"}}),
		testNode("v", "const", "1", "INT", nil),
	)
}

func TestAdapterSleepDurationTypeChecked(t *testing.T) {
	for _, bad := range []transportNode{testNode("d", "const", "x", "STRING", nil), testNode("d", "list", "", "", nil), testNode("d", "const", "true", "BOOL", nil)} {
		code := decodeCode(t,
			testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "s"}}),
			testNode("s", "sleep", "", "", []transportEdge{{Role: "duration", NodeID: "d"}}),
			bad,
		)
		if code != "HFIR_TRANSPORT_TYPE" {
			t.Fatalf("%s: code = %s", bad.Kind, code)
		}
	}
}

func TestAdapterInputsPerNodeBound(t *testing.T) {
	nodes := []transportNode{{}}
	var edges []transportEdge
	for i := 0; i < maxNodeInputs+1; i++ {
		id := NodeID(fmt.Sprintf("c%d", i))
		edges = append(edges, transportEdge{Role: "value", NodeID: id})
		nodes = append(nodes, testNode(id, "const", "1", "INT", nil))
	}
	nodes[0] = testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "pr"}})
	nodes = append(nodes, testNode("pr", "print", "", "", edges))
	if code := decodeCode(t, nodes...); code != "HFIR_TRANSPORT_BOUNDS" || maxNodeInputs != 64 {
		t.Fatalf("code = %s, maxNodeInputs = %d", code, maxNodeInputs)
	}
}
