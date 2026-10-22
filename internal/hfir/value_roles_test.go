package hfir

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/vm"
)

func valueRoleTransport(nodes []transportNode) []byte {
	encoded, _ := json.Marshal(candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program", Nodes: nodes})
	return encoded
}

// HF-05 on the model-adapter path: a value-less node in a value role used to
// pass decode and compile, perform its effect, and then underflow the VM.
// It must be rejected before lowering, so nothing runs.
func TestAdapterRejectsValueLessOperandBeforeExecution(t *testing.T) {
	target := filepath.Join(t.TempDir(), "fx.txt")
	write := func(id NodeID) []transportNode {
		return []transportNode{
			testNode(id, "write_file", "", "", []transportEdge{{Role: "path", NodeID: id + "_p"}, {Role: "data", NodeID: id + "_d"}}),
			testNode(id+"_p", "const", target, "STRING", nil),
			testNode(id+"_d", "const", "x", "STRING", nil),
		}
	}
	str := func(id NodeID, v string) transportNode { return testNode(id, "const", v, "STRING", nil) }
	program := func(body NodeID) transportNode {
		return testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: body}})
	}
	invalid := map[string][]transportNode{
		"try expression": append([]transportNode{program("try"),
			testNode("try", "try", "r", "", []transportEdge{{Role: "expression", NodeID: "w"}, {Role: "success_body", NodeID: "ok"}, {Role: "catch", NodeID: "c"}}),
			testNode("ok", "print", "", "", []transportEdge{{Role: "value", NodeID: "s1"}}), str("s1", "bound"),
			testNode("c", "catch", "e", "", []transportEdge{{Role: "body", NodeID: "cp"}}),
			testNode("cp", "print", "", "", []transportEdge{{Role: "value", NodeID: "s2"}}), str("s2", "caught"),
		}, write("w")...),
		"dict entry value": append([]transportNode{program("p"),
			testNode("p", "print", "", "", []transportEdge{{Role: "value", NodeID: "d"}}),
			testNode("d", "dict", "", "", []transportEdge{{Role: "entry", NodeID: "e"}}),
			testNode("e", "dict_entry", "", "", []transportEdge{{Role: "key", NodeID: "k"}, {Role: "value", NodeID: "w"}}), str("k", "k"),
		}, write("w")...),
		"let value": append([]transportNode{program("l"),
			testNode("l", "let", "r", "", []transportEdge{{Role: "value", NodeID: "w"}, {Role: "body", NodeID: "p"}}),
			testNode("p", "print", "", "", []transportEdge{{Role: "value", NodeID: "s"}}), str("s", "b"),
		}, write("w")...),
		"print value": append([]transportNode{program("p"),
			testNode("p", "print", "", "", []transportEdge{{Role: "value", NodeID: "w"}}),
		}, write("w")...),
		"if without else as a value": append([]transportNode{
			testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "w"}, {Role: "body", NodeID: "l"}}),
			testNode("l", "let", "r", "", []transportEdge{{Role: "value", NodeID: "i"}, {Role: "body", NodeID: "p"}}),
			testNode("i", "if", "", "", []transportEdge{{Role: "condition", NodeID: "f"}, {Role: "then", NodeID: "s"}}),
			testNode("f", "const", "false", "BOOL", nil), str("s", "x"),
			testNode("p", "print", "", "", []transportEdge{{Role: "value", NodeID: "s3"}}), str("s3", "b"),
		}, write("w")...),
		"sequence ending in an effect": append([]transportNode{program("p"),
			testNode("p", "print", "", "", []transportEdge{{Role: "value", NodeID: "q"}}),
			testNode("q", "sequence", "", "", []transportEdge{{Role: "body", NodeID: "w"}}),
		}, write("w")...),
	}
	for name, nodes := range invalid {
		t.Run(name, func(t *testing.T) {
			candidate, diagnostics := DecodeCandidate(valueRoleTransport(nodes))
			if candidate.Graph != nil || len(diagnostics) != 1 || diagnostics[0].Code != "HFIR_TRANSPORT_TYPE" ||
				!strings.Contains(diagnostics[0].Message, "produces no value") {
				t.Fatalf("DecodeCandidate() = %v, %#v; want one HFIR_TRANSPORT_TYPE rejection", candidate.Graph != nil, diagnostics)
			}
			if _, err := os.Stat(target); err == nil {
				t.Fatal("a rejected candidate wrote its file")
			}
		})
	}

	// The same effect as a statement, and a sequence that ends in a value, run.
	valid := append([]transportNode{
		testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "w"}, {Role: "body", NodeID: "p"}}),
		testNode("p", "print", "", "", []transportEdge{{Role: "value", NodeID: "q"}}),
		testNode("q", "sequence", "", "", []transportEdge{{Role: "body", NodeID: "s"}}), str("s", "done"),
	}, write("w")...)
	prog, diagnostics := CompileCandidate(mustDecode(t, valueRoleTransport(valid)))
	if len(diagnostics) != 0 {
		t.Fatalf("CompileCandidate() = %#v", diagnostics)
	}
	var stdout, stderr bytes.Buffer
	evidence := vm.RunBytecodeWithEvidence(prog, nil, vm.DefaultExecutionPolicy(), []capability.Capability{capability.Filesystem}, strings.NewReader(""), &stdout, &stderr, 0)
	if evidence.RuntimeFailure != nil || stdout.String() != "done\n" {
		t.Fatalf("valid candidate: failure %#v stdout %q", evidence.RuntimeFailure, stdout.String())
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("statement-position write did not run: %v", err)
	}
}

// valueLessKinds must agree with the opcode table: each straight-line kind
// lowers, as a statement, to a final opcode that pushes nothing, and the
// value-producing kinds checked here end in an opcode that pushes one value.
func TestValueLessKindsMatchOpcodeTable(t *testing.T) {
	str := func(id NodeID) transportNode { return testNode(id, "const", "x", "STRING", nil) }
	num := func(id NodeID) transportNode { return testNode(id, "const", "1", "INT", nil) }
	shapes := map[string]struct {
		value  string
		inputs []transportNode
	}{
		"write_file":  {"", []transportNode{str("a"), str("b")}},
		"mkdir":       {"", []transportNode{str("a")}},
		"print":       {"", []transportNode{str("a")}},
		"stderr":      {"", []transportNode{str("a")}},
		"exit":        {"", []transportNode{num("a")}},
		"set":         {"v", []transportNode{num("a")}},
		"map_set":     {"v", []transportNode{str("a"), num("b")}},
		"map_delete":  {"v", []transportNode{str("a")}},
		"append":      {"v", []transportNode{num("a")}},
		"sleep":       {"", []transportNode{num("a")}},
		"read_file":   {"", []transportNode{str("a")}},
		"str_join":    {"", []transportNode{str("a"), str("b")}},
		"list_len":    {"", []transportNode{str("a")}},
		"env":         {"", []transportNode{str("a")}},
		"html_escape": {"", []transportNode{str("a")}},
		"list_get":    {"v", []transportNode{num("a")}},
	}
	statementOnly := map[string]bool{"for": true, "while": true, "defun": true, "return": true}
	var kinds []string
	for kind := range valueLessKinds {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		if _, ok := shapes[kind]; !ok && !statementOnly[kind] {
			t.Errorf("value-less kind %s has no opcode check", kind)
		}
	}
	for kind, shape := range shapes {
		t.Run(kind, func(t *testing.T) {
			roles := nodeRoles(kind)
			var edges []transportEdge
			for i, input := range shape.inputs {
				edges = append(edges, transportEdge{Role: strings.TrimRight(roles[i], "?*"), NodeID: input.ID})
			}
			nodes := append([]transportNode{
				testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "n"}}),
				testNode("n", kind, shape.value, "", edges),
			}, shape.inputs...)
			prog, diagnostics := CompileCandidate(mustDecode(t, valueRoleTransport(nodes)))
			if len(diagnostics) != 0 {
				t.Fatalf("CompileCandidate() = %#v", diagnostics)
			}
			last := prog.Main[len(prog.Main)-1]
			want := 1
			if valueLessKinds[kind] {
				want = 0
			}
			if got := bytecode.Registry[last.Op].Pushes; got != want {
				t.Fatalf("%s lowers to %s, which pushes %d; valueLessKinds says %d", kind, last.OpString, got, want)
			}
		})
	}
}
