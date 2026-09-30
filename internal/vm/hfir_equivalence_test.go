package vm

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/hfir"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

type bytecodeOutcome struct {
	stdout   string
	stderr   string
	exitCode int
	vmError  *VMError
	panicVal any
}

func TestHFIRBytecodeEquivalence(t *testing.T) {
	tests := []struct {
		name   string
		source string
		caps   []capability.Capability
		stdin  string
	}{
		{
			name: "scalar control",
			source: `(cli_app
  (let (count 3)
    (do
      (set count (+ count 2))
      (if (and (> count 4) (= "ok" "ok"))
        (print "count:" (to_string count))
        (stderr "unexpected")))))`,
		},
		{
			name: "collections",
			source: `(cli_app
  (let (items (list "a" "b"))
    (let (record (dict ("status" "open") ("owner" "team")))
      (do
        (map_set record "status" "closed")
        (map_delete record "missing")
        (print (list_get items 1) (map_get record "status") (list_len items))))))`,
		},
		{
			name:   "string transforms",
			source: `(cli_app (let (parts (str_split "alpha,beta" ",")) (print (str_join parts "|"))))`,
		},
		{
			name: "dict keys",
			source: `(cli_app
  (let (counts (dict ("beta" "2") ("alpha" "1")))
    (print (str_join (map_keys counts) ","))))`,
		},
		{
			name:   "stderr and exit",
			source: `(cli_app (stderr "halt\n") (exit 7) (print "unreachable"))`,
		},
		{
			name:   "capability guarded environment",
			source: `(cli_app (print (env "HFIR_EQ_TEST_VALUE")))`,
			caps:   []capability.Capability{capability.Environment},
		},
		{
			name:   "request query rejects a non-request",
			source: `(cli_app (let (req "nope") (print (req_query req "status"))))`,
		},
		{
			name: "defun and call",
			source: `(cli_app
  (defun add (a b)
    (type_hints (a int) (b int) (return int))
    (return (+ a b)))
  (defun label ()
    (type_hint return "string")
    (return "phase3a"))
  (print (call add (call add 20 1) 21))
  (print (call label)))`,
		},
		{
			name: "defun after its call",
			source: `(cli_app
  (print (call add 2 3))
  (defun add ((a int) (b int)) int
    (return (+ a b))))`,
		},
		{
			name: "recursive defun",
			source: `(cli_app
  (defun fact (n)
    (type_hints (n int) (return int))
    (if (< n 2)
      (return 1)
      (return (* n (call fact (- n 1))))))
  (print (call fact 5)))`,
		},
		{
			name: "while counts",
			source: `(cli_app
  (let (n 0)
    (while (< n 3)
      (do
        (set n (+ n 1))
        (print n)))))`,
		},
		{
			name:   "while does not enter",
			source: `(cli_app (while false (print "no")) (print "done"))`,
		},
		{
			name: "nested while",
			source: `(cli_app
  (let (i 0)
    (while (< i 2)
      (do
        (let (j 0)
          (while (< j 2)
            (do
              (print i j)
              (set j (+ j 1)))))
        (set i (+ i 1))))))`,
		},
		{
			name: "while inside defun",
			source: `(cli_app
  (defun steps (limit)
    (type_hints (limit int) (return int))
    (let (n 0)
      (do
        (while (< n limit)
          (set n (+ n 1)))
        (return n))))
  (print (call steps 4)))`,
		},
		{
			name: "if then else",
			source: `(cli_app
  (if false
    (print "no")
    (print "else"))
  (if true
    (print "then")
    (print "no"))
  (if (> 2 1)
    (print "greater")
    (print "no")))`,
		},
		{
			name: "if without else",
			source: `(cli_app
  (if false (print "no"))
  (if true (print "only")))`,
		},
		{
			name: "nested if",
			source: `(cli_app
  (if true
    (if false
      (print "no")
      (print "inner"))
    (print "no")))`,
		},
		{
			name: "if inside defun",
			source: `(cli_app
  (defun choose (flag)
    (type_hints (flag bool) (return string))
    (if flag
      (return "yes")
      (return "no")))
  (print (call choose true))
  (print (call choose false)))`,
		},
		{
			name: "for iterates",
			source: `(cli_app
  (for item (list "a" "b" "c")
    (print item)))`,
		},
		{
			name:   "for does not enter",
			source: `(cli_app (for item (list) (print "no")) (print "done"))`,
		},
		{
			name: "nested for",
			source: `(cli_app
  (for row (list (list "a" "b") (list "c"))
    (for item row
      (print item))))`,
		},
		{
			name: "for inside defun",
			source: `(cli_app
  (defun show ((items any)) string
    (do
      (for item items
        (print item))
      (return "done")))
  (print (call show (list "a" "b"))))`,
		},
	}

	t.Setenv("HFIR_EQ_TEST_VALUE", "expected")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, graph := checkedHFIRGraph(t, tt.source)
			legacy := bytecode.CompileToBytecode(root)
			direct, diagnostics := hfir.LowerToBytecode(graph)
			if len(diagnostics) != 0 {
				t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
			}
			legacy = roundTripArtifact(t, legacy)
			direct = roundTripArtifact(t, direct)

			legacyOutcome := runBytecodeOutcome(legacy, tt.stdin, tt.caps)
			directOutcome := runBytecodeOutcome(direct, tt.stdin, tt.caps)
			if !reflect.DeepEqual(legacyOutcome, directOutcome) {
				t.Fatalf("AST bytecode outcome = %#v\nHFIR bytecode outcome = %#v", legacyOutcome, directOutcome)
			}
		})
	}
}

// TestHFIRBytecodeNestedIfWhileDefunFixture compares production AST bytecode
// with the experimental HFIR lowerer on one program that nests the Phase
// 3a–3c surface: a defun call, while inside if, and if inside while.
// The conformance harness runs the same file through -compile-bc and
// -compile-hfir-bc. This test also round-trips both artifacts.
func TestHFIRBytecodeNestedIfWhileDefunFixture(t *testing.T) {
	path := filepath.Join("..", "..", "tests", "conformance", "abi_v1", "12_nested_if_while_defun.howl")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root, graph := checkedHFIRGraph(t, string(source))
	var defuns, calls, whiles, ifThenOnly, ifWithElse int
	for _, node := range graph.Nodes {
		switch node.Kind {
		case "defun":
			defuns++
			if len(node.ControlEdges) != 0 {
				t.Fatalf("defun %s has control edges %v", node.ID, node.ControlEdges)
			}
		case "call":
			calls++
		case "while":
			whiles++
			if len(node.ControlEdges) != 2 || len(node.DataInputs) != 2 || node.DataInputs[0].Name != "condition" || node.DataInputs[1].Name != "body" || node.ControlEdges[0] != node.DataInputs[0].SourceNode || node.ControlEdges[1] != node.DataInputs[1].SourceNode {
				t.Fatalf("while %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
			}
		case "if":
			if len(node.ControlEdges) != len(node.DataInputs) || len(node.DataInputs) < 2 || node.DataInputs[0].Name != "condition" || node.DataInputs[1].Name != "then" {
				t.Fatalf("if %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
			}
			for index := range node.ControlEdges {
				if node.ControlEdges[index] != node.DataInputs[index].SourceNode {
					t.Fatalf("if %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
				}
			}
			switch len(node.ControlEdges) {
			case 2:
				ifThenOnly++
			case 3:
				if node.DataInputs[2].Name != "else" {
					t.Fatalf("if %s else edge name = %q", node.ID, node.DataInputs[2].Name)
				}
				ifWithElse++
			default:
				t.Fatalf("if %s has %d control edges", node.ID, len(node.ControlEdges))
			}
		case "for":
			t.Fatalf("dogfood fixture lowered a for node %s", node.ID)
		}
	}
	if defuns != 1 || calls != 2 || whiles != 3 || ifThenOnly != 1 || ifWithElse != 3 {
		t.Fatalf("surface counts defun=%d call=%d while=%d if-then=%d if-else=%d, want 1, 2, 3, 1, 3", defuns, calls, whiles, ifThenOnly, ifWithElse)
	}

	legacy := roundTripArtifact(t, bytecode.CompileToBytecode(root))
	direct, diagnostics := hfir.LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	direct = roundTripArtifact(t, direct)
	legacyOutcome := runBytecodeOutcome(legacy, "", nil)
	directOutcome := runBytecodeOutcome(direct, "", nil)
	if !reflect.DeepEqual(legacyOutcome, directOutcome) {
		t.Fatalf("AST bytecode outcome = %#v\nHFIR bytecode outcome = %#v", legacyOutcome, directOutcome)
	}
	const want = "low 0\nlow 1\nmid 2\nhit 3\nmid 4\nagain 1\nresult 2\nmiss\nempty 0\n"
	if legacyOutcome.stdout != want {
		t.Fatalf("stdout = %q, want %q", legacyOutcome.stdout, want)
	}
	if strings.Contains(legacyOutcome.stdout, "no") || legacyOutcome.exitCode != 0 || legacyOutcome.vmError != nil || legacyOutcome.panicVal != nil {
		t.Fatalf("unexpected outcome %#v", legacyOutcome)
	}
}

// TestHFIRBytecodeNestedForIfWhileDefunFixture compares production AST
// bytecode with the experimental HFIR lowerer on one program that nests the
// Phase 3a–3d surface: for inside for, for inside if, for inside while, and
// while inside for, all inside one defun. The conformance harness runs the
// same file through -compile-bc and -compile-hfir-bc. This test also
// round-trips both artifacts.
func TestHFIRBytecodeNestedForIfWhileDefunFixture(t *testing.T) {
	path := filepath.Join("..", "..", "tests", "conformance", "abi_v1", "14_nested_for_if_while_defun.howl")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root, graph := checkedHFIRGraph(t, string(source))
	var defuns, calls, whiles, ifThenOnly, ifWithElse int
	fors := map[string]*hfir.Node{}
	for _, node := range graph.Nodes {
		switch node.Kind {
		case "defun":
			defuns++
			if len(node.ControlEdges) != 0 {
				t.Fatalf("defun %s has control edges %v", node.ID, node.ControlEdges)
			}
		case "call":
			calls++
		case "while":
			whiles++
			if len(node.ControlEdges) != 2 || len(node.DataInputs) != 2 || node.DataInputs[0].Name != "condition" || node.DataInputs[1].Name != "body" || node.ControlEdges[0] != node.DataInputs[0].SourceNode || node.ControlEdges[1] != node.DataInputs[1].SourceNode {
				t.Fatalf("while %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
			}
		case "if":
			if len(node.ControlEdges) != len(node.DataInputs) || len(node.DataInputs) < 2 || node.DataInputs[0].Name != "condition" || node.DataInputs[1].Name != "then" {
				t.Fatalf("if %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
			}
			for index := range node.ControlEdges {
				if node.ControlEdges[index] != node.DataInputs[index].SourceNode {
					t.Fatalf("if %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
				}
			}
			switch len(node.ControlEdges) {
			case 2:
				ifThenOnly++
			case 3:
				if node.DataInputs[2].Name != "else" {
					t.Fatalf("if %s else edge name = %q", node.ID, node.DataInputs[2].Name)
				}
				ifWithElse++
			default:
				t.Fatalf("if %s has %d control edges", node.ID, len(node.ControlEdges))
			}
		case "for":
			if len(node.ControlEdges) != 2 || len(node.DataInputs) != 2 || node.DataInputs[0].Name != "iterable" || node.DataInputs[1].Name != "body" || node.ControlEdges[0] != node.DataInputs[0].SourceNode || node.ControlEdges[1] != node.DataInputs[1].SourceNode {
				t.Fatalf("for %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
			}
			if _, ok := fors[node.Value]; ok {
				t.Fatalf("duplicate for iterator %q", node.Value)
			}
			fors[node.Value] = node
		}
	}
	if defuns != 1 || calls != 2 || whiles != 3 || ifThenOnly != 1 || ifWithElse != 3 || len(fors) != 4 {
		t.Fatalf("surface counts defun=%d call=%d while=%d if-then=%d if-else=%d for=%d, want 1, 2, 3, 1, 3, 4", defuns, calls, whiles, ifThenOnly, ifWithElse, len(fors))
	}
	label := fors["label"]
	item := fors["item"]
	word := fors["word"]
	absent := fors["absent"]
	if label == nil || item == nil || word == nil || absent == nil {
		t.Fatalf("for iterators = %v, want label, item, word, absent", forIteratorNames(fors))
	}
	itemIter := graph.NodeByID(item.DataInputs[0].SourceNode)
	if itemIter == nil || itemIter.Kind != "symbol" || itemIter.Value != "cells" {
		t.Fatalf("inner for iterable = %#v, want symbol cells", itemIter)
	}
	if !hfirSubtreeHas(graph, label.DataInputs[1].SourceNode, func(node *hfir.Node) bool { return node == item }) {
		t.Fatal("label for body does not contain the item for")
	}
	if !hfirSubtreeHas(graph, label.DataInputs[1].SourceNode, func(node *hfir.Node) bool { return node.Kind == "if" }) || !hfirSubtreeHas(graph, label.DataInputs[1].SourceNode, func(node *hfir.Node) bool { return node.Kind == "while" }) {
		t.Fatal("label for body does not contain both if and while")
	}
	if !hfirSubtreeHas(graph, item.DataInputs[1].SourceNode, func(node *hfir.Node) bool { return node.Kind == "if" }) || !hfirSubtreeHas(graph, item.DataInputs[1].SourceNode, func(node *hfir.Node) bool { return node.Kind == "while" }) {
		t.Fatal("item for body does not contain both if and while")
	}
	absentIter := graph.NodeByID(absent.DataInputs[0].SourceNode)
	if absentIter == nil || absentIter.Kind != "list" || len(absentIter.DataInputs) != 0 {
		t.Fatalf("empty for iterable = %#v, want a list with no items", absentIter)
	}
	var whileHoldsFor, ifHoldsFor, defunHoldsFor bool
	for _, node := range graph.Nodes {
		switch node.Kind {
		case "while":
			if hfirSubtreeHas(graph, node.DataInputs[1].SourceNode, func(child *hfir.Node) bool { return child.Kind == "for" }) {
				whileHoldsFor = true
			}
		case "if":
			for _, edge := range node.DataInputs[1:] {
				if hfirSubtreeHas(graph, edge.SourceNode, func(child *hfir.Node) bool { return child.Kind == "for" }) {
					ifHoldsFor = true
				}
			}
		case "defun":
			if hfirSubtreeHas(graph, node.ID, func(child *hfir.Node) bool { return child.Kind == "for" }) {
				defunHoldsFor = true
			}
		}
	}
	if !whileHoldsFor || !ifHoldsFor || !defunHoldsFor {
		t.Fatalf("nesting while-for=%v if-for=%v defun-for=%v", whileHoldsFor, ifHoldsFor, defunHoldsFor)
	}

	legacy := roundTripArtifact(t, bytecode.CompileToBytecode(root))
	direct, diagnostics := hfir.LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	direct = roundTripArtifact(t, direct)
	legacyOutcome := runBytecodeOutcome(legacy, "", nil)
	directOutcome := runBytecodeOutcome(direct, "", nil)
	if !reflect.DeepEqual(legacyOutcome, directOutcome) {
		t.Fatalf("AST bytecode outcome = %#v\nHFIR bytecode outcome = %#v", legacyOutcome, directOutcome)
	}
	const want = "L a 0\nstep a\nL b 0\nstep b\nL a 1\nstep a\nL b 1\nstep b\nkept 4\nresult 4\nmiss\nempty 0\n"
	if legacyOutcome.stdout != want {
		t.Fatalf("stdout = %q, want %q", legacyOutcome.stdout, want)
	}
	if strings.Contains(legacyOutcome.stdout, "no") || legacyOutcome.stderr != "" || legacyOutcome.exitCode != 0 || legacyOutcome.vmError != nil || legacyOutcome.panicVal != nil {
		t.Fatalf("unexpected outcome %#v", legacyOutcome)
	}
}

// TestHFIRBytecodeWriteFileFixture compares production AST bytecode with
// the experimental HFIR lowerer on the Phase 2e write_file fixture. The
// conformance harness runs that file through -compile-bc and -compile-hfir-bc.
// This test rewrites the absolute path into a temp directory and compares
// the filesystem each compiler leaves behind.
func TestHFIRBytecodeWriteFileFixture(t *testing.T) {
	source := readAbiFixture(t, "15_write_file_capability.howl")
	dir := t.TempDir()
	path := filepath.Join(dir, "write.txt")
	source = strings.ReplaceAll(source, "/tmp/howlframe-abi-v1-phase2e-write.txt", path)
	compareFilesystemCompilers(t, source, []string{path}, "WRITE_FILE", "phase2e-wrote\n", map[string]fsEntry{
		path: {kind: "file", body: "phase2e-write-marker"},
	})
}

// TestHFIRBytecodeMkdirFixture compares production AST bytecode with the
// experimental HFIR lowerer on the Phase 2e mkdir fixture.
func TestHFIRBytecodeMkdirFixture(t *testing.T) {
	source := readAbiFixture(t, "16_mkdir_capability.howl")
	dir := t.TempDir()
	path := filepath.Join(dir, "made")
	source = strings.ReplaceAll(source, "/tmp/howlframe-abi-v1-phase2e-dir", path)
	compareFilesystemCompilers(t, source, []string{path}, "MKDIR", "phase2e-made\n", map[string]fsEntry{
		path: {kind: "dir"},
	})
}

// TestHFIRBytecodeNestedFsWriteFixture compares production AST bytecode
// with the experimental HFIR lowerer on one program that nests write_file
// and mkdir inside if, while, for, and defun. The conformance harness runs
// the same file through -compile-bc and -compile-hfir-bc, granted and denied.
// This test round-trips both artifacts and snapshots the filesystem each
// compiler leaves behind.
func TestHFIRBytecodeNestedFsWriteFixture(t *testing.T) {
	source := readAbiFixture(t, "17_nested_fs_write.howl")
	dir := t.TempDir()
	prefix := filepath.Join(dir, "dogfood")
	source = strings.ReplaceAll(source, "/tmp/howlframe-abi-v1-dogfood", prefix)
	root, graph := checkedHFIRGraph(t, source)
	assertNestedFsWriteShape(t, graph)

	legacy := roundTripArtifact(t, bytecode.CompileToBytecode(root))
	direct, diagnostics := hfir.LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	direct = roundTripArtifact(t, direct)
	if got, want := graphCapabilities(graph), []capability.Capability{capability.Filesystem}; !reflect.DeepEqual(got, want) {
		t.Fatalf("HFIR effects = %v, want filesystem", got)
	}
	if got, want := programCapabilities(legacy), programCapabilities(direct); !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy capabilities = %v, HFIR capabilities = %v", got, want)
	}
	if !reflect.DeepEqual(programCapabilities(direct), []capability.Capability{capability.Filesystem}) {
		t.Fatalf("emitted capabilities = %v, want filesystem", programCapabilities(direct))
	}

	paths := []string{
		prefix + "-write.txt",
		prefix + "-loop.txt",
		prefix + "-skip.txt",
		prefix + "-else.txt",
		prefix + "-empty.txt",
		prefix + "-dir",
		prefix + "-miss",
		prefix + "-skip",
		prefix + "-dead",
	}
	const wantStdout = "L a 0\nL b 0\nkept 2\nresult 2\nmiss\nempty 0\n"
	granted := map[string]fsEntry{
		prefix + "-write.txt": {kind: "file", body: "dogfood-write-marker"},
		prefix + "-loop.txt":  {kind: "file", body: "dogfood-loop-marker"},
		prefix + "-dir":       {kind: "dir"},
		prefix + "-miss":      {kind: "dir"},
		prefix + "-skip.txt":  {kind: "absent"},
		prefix + "-else.txt":  {kind: "absent"},
		prefix + "-empty.txt": {kind: "absent"},
		prefix + "-skip":      {kind: "absent"},
		prefix + "-dead":      {kind: "absent"},
	}
	compareCompilerFilesystem(t, legacy, direct, paths, "WRITE_FILE", prefix, wantStdout, granted)
}

// TestHFIRBytecodeNestedEnvReadFixture compares production AST bytecode with
// the experimental HFIR lowerer on one program that nests env and read_file
// inside if, while, for, and defun. The conformance harness runs the same
// file through -compile-bc and -compile-hfir-bc, granted and denied.
// This test round-trips both artifacts.
func TestHFIRBytecodeNestedEnvReadFixture(t *testing.T) {
	t.Setenv("HOWLFRAME_ABI_SECRET", "phase1-token")
	source := readAbiFixture(t, "18_nested_env_read.howl")
	dir := t.TempDir()
	prefix := filepath.Join(dir, "hostread")
	source = strings.ReplaceAll(source, "/tmp/howlframe-abi-v1-hostread", prefix)
	taken := map[string]string{
		prefix + "-read.txt": "dogfood-read-marker",
		prefix + "-miss.txt": "dogfood-miss-marker",
	}
	untaken := []string{
		prefix + "-skip.txt",
		prefix + "-else.txt",
		prefix + "-empty.txt",
		prefix + "-dead.txt",
	}
	for path, body := range taken {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	root, graph := checkedHFIRGraph(t, source)
	assertNestedEnvReadShape(t, graph)

	legacy := roundTripArtifact(t, bytecode.CompileToBytecode(root))
	direct, diagnostics := hfir.LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	direct = roundTripArtifact(t, direct)
	wantCaps := []capability.Capability{capability.Environment, capability.Filesystem}
	if got := graphCapabilities(graph); !reflect.DeepEqual(got, wantCaps) {
		t.Fatalf("HFIR effects = %v, want environment and filesystem", got)
	}
	if got, want := programCapabilities(legacy), programCapabilities(direct); !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy capabilities = %v, HFIR capabilities = %v", got, want)
	}
	if !reflect.DeepEqual(programCapabilities(direct), wantCaps) {
		t.Fatalf("emitted capabilities = %v, want environment and filesystem", programCapabilities(direct))
	}

	const wantStdout = "phase1-token\ndogfood-read-marker\nL a 0\nL b 0\nkept 2\nphase1-token\nresult 2\nmiss\ndogfood-miss-marker\nempty 0\n"
	legacyDenied := runBytecodeOutcome(legacy, "", nil)
	directDenied := runBytecodeOutcome(direct, "", nil)
	requireSameBytecodeOutcome(t, legacyDenied, directDenied)
	if legacyDenied.vmError == nil || legacyDenied.vmError.Code != "CAPABILITY_DENIED" || legacyDenied.vmError.Opcode != "ENV" || legacyDenied.vmError.Message != "capability denied: environment" {
		t.Fatalf("denial = %#v, want CAPABILITY_DENIED ENV", legacyDenied)
	}
	if legacyDenied.stdout != "" || legacyDenied.stderr != "" {
		t.Fatalf("denial produced output %#v", legacyDenied)
	}
	denialText := legacyDenied.vmError.Message
	for _, secret := range []string{"phase1-token", prefix, "dogfood-read-marker", "dogfood-miss-marker"} {
		if strings.Contains(denialText, secret) {
			t.Fatalf("denial leaked %q in %q", secret, denialText)
		}
	}

	caps := []capability.Capability{capability.Environment, capability.Filesystem}
	legacyGranted := runBytecodeOutcome(legacy, "", caps)
	directGranted := runBytecodeOutcome(direct, "", caps)
	requireSameBytecodeOutcome(t, legacyGranted, directGranted)
	if legacyGranted.stdout != wantStdout || legacyGranted.stderr != "" || legacyGranted.exitCode != 0 || legacyGranted.vmError != nil || legacyGranted.panicVal != nil {
		t.Fatalf("granted outcome %#v", legacyGranted)
	}
	if strings.Contains(legacyGranted.stdout, "no") || strings.Contains(legacyGranted.stdout, prefix) {
		t.Fatalf("granted stdout leaked an untaken branch or path: %q", legacyGranted.stdout)
	}
	for path, body := range taken {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != body {
			t.Fatalf("read marker %s changed to %q", path, got)
		}
	}
	for _, path := range untaken {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("untaken path exists: %s (%v)", path, err)
		}
	}
}

// TestHFIRBytecodeExecFixture compares production AST bytecode with the
// experimental HFIR lowerer on the Phase 2b exec fixture. The conformance
// harness runs that file through -compile-bc and -compile-hfir-bc, granted
// and denied. This test round-trips both artifacts and checks that EXEC
// carries the command and then the argument.
func TestHFIRBytecodeExecFixture(t *testing.T) {
	source := readAbiFixture(t, "06_exec_capability.howl")
	root, graph := checkedHFIRGraph(t, source)
	var execs int
	for _, node := range graph.Nodes {
		if node.Kind != "exec" {
			continue
		}
		execs++
		if len(node.ControlEdges) != 0 || len(node.DataInputs) != 2 || node.DataInputs[0].Name != "cmd" || node.DataInputs[1].Name != "arg" {
			t.Fatalf("exec %s edges %#v control %v", node.ID, node.DataInputs, node.ControlEdges)
		}
		if got := fsPathValue(t, graph, node); got != "printf" {
			t.Fatalf("exec command = %q, want printf", got)
		}
		arg := graph.NodeByID(node.DataInputs[1].SourceNode)
		if arg == nil || arg.Kind != "const" || arg.Value != "phase2b-exec-marker" {
			t.Fatalf("exec argument = %#v, want phase2b-exec-marker", arg)
		}
	}
	if execs != 1 {
		t.Fatalf("exec nodes = %d, want 1", execs)
	}

	legacy := roundTripArtifact(t, bytecode.CompileToBytecode(root))
	direct, diagnostics := hfir.LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	direct = roundTripArtifact(t, direct)
	if got, want := graphCapabilities(graph), []capability.Capability{capability.Process}; !reflect.DeepEqual(got, want) {
		t.Fatalf("HFIR effects = %v, want process", got)
	}
	if got, want := programCapabilities(legacy), programCapabilities(direct); !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy capabilities = %v, HFIR capabilities = %v", got, want)
	}
	if !reflect.DeepEqual(programCapabilities(direct), []capability.Capability{capability.Process}) {
		t.Fatalf("emitted capabilities = %v, want process", programCapabilities(direct))
	}
	if got, want := execOperandShapes(legacy), execOperandShapes(direct); !reflect.DeepEqual(got, want) {
		t.Fatalf("EXEC operands\nAST %#v\nHFIR %#v", got, want)
	}
	if !reflect.DeepEqual(execOperandShapes(direct), []string{"main\x00printf\x00phase2b-exec-marker"}) {
		t.Fatalf("EXEC operands = %#v", execOperandShapes(direct))
	}

	legacyDenied := runBytecodeOutcome(legacy, "", nil)
	directDenied := runBytecodeOutcome(direct, "", nil)
	requireSameBytecodeOutcome(t, legacyDenied, directDenied)
	requireProcessDenial(t, legacyDenied, "printf", "phase2b-exec-marker")

	caps := []capability.Capability{capability.Process}
	legacyGranted := runBytecodeOutcome(legacy, "", caps)
	directGranted := runBytecodeOutcome(direct, "", caps)
	requireSameBytecodeOutcome(t, legacyGranted, directGranted)
	if legacyGranted.stdout != "phase2b-exec-marker\n" || legacyGranted.stderr != "" || legacyGranted.exitCode != 0 || legacyGranted.vmError != nil || legacyGranted.panicVal != nil {
		t.Fatalf("granted outcome %#v", legacyGranted)
	}
}

// TestHFIRBytecodeExecOperandOrder locks zero, one, and three arguments onto
// the same EXEC instruction the AST compiler emits: command, then arguments,
// with IntOperand equal to the argument count. The three-argument form is a
// printf format plus two values, so a swapped operand changes the text.
func TestHFIRBytecodeExecOperandOrder(t *testing.T) {
	source := `(cli_app
  (print (bytes_to_string (exec "true")))
  (print (bytes_to_string (exec "printf" "only")))
  (print (bytes_to_string (exec "printf" "%s:%s" "left" "right"))))`
	root, graph := checkedHFIRGraph(t, source)
	var counts []int
	for _, node := range graph.Nodes {
		if node.Kind != "exec" {
			continue
		}
		if len(node.DataInputs) == 0 || node.DataInputs[0].Name != "cmd" {
			t.Fatalf("exec %s edges %#v", node.ID, node.DataInputs)
		}
		for _, edge := range node.DataInputs[1:] {
			if edge.Name != "arg" {
				t.Fatalf("exec %s edges %#v", node.ID, node.DataInputs)
			}
		}
		counts = append(counts, len(node.DataInputs)-1)
	}
	if !reflect.DeepEqual(counts, []int{0, 1, 3}) {
		t.Fatalf("argument counts = %v, want 0, 1, 3", counts)
	}

	legacy := roundTripArtifact(t, bytecode.CompileToBytecode(root))
	direct, diagnostics := hfir.LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	direct = roundTripArtifact(t, direct)
	wantShapes := []string{
		"main\x00true",
		"main\x00printf\x00only",
		"main\x00printf\x00%s:%s\x00left\x00right",
	}
	if got, want := execOperandShapes(legacy), execOperandShapes(direct); !reflect.DeepEqual(got, want) || !reflect.DeepEqual(got, wantShapes) {
		t.Fatalf("EXEC operands\nAST %#v\nHFIR %#v\nwant %#v", got, want, wantShapes)
	}
	caps := []capability.Capability{capability.Process}
	legacyOutcome := runBytecodeOutcome(legacy, "", caps)
	directOutcome := runBytecodeOutcome(direct, "", caps)
	requireSameBytecodeOutcome(t, legacyOutcome, directOutcome)
	const wantStdout = "\nonly\nleft:right\n"
	if legacyOutcome.stdout != wantStdout || legacyOutcome.stderr != "" || legacyOutcome.exitCode != 0 || legacyOutcome.vmError != nil {
		t.Fatalf("granted outcome %#v, want stdout %q", legacyOutcome, wantStdout)
	}
}

// TestHFIRBytecodeNestedExecFixture compares production AST bytecode with
// the experimental HFIR lowerer on one program that nests exec inside if,
// while, for, and defun. The conformance harness runs the same file through
// -compile-bc and -compile-hfir-bc, granted and denied.
func TestHFIRBytecodeNestedExecFixture(t *testing.T) {
	source := readAbiFixture(t, "19_nested_exec.howl")
	root, graph := checkedHFIRGraph(t, source)
	assertNestedExecShape(t, graph)

	legacy := roundTripArtifact(t, bytecode.CompileToBytecode(root))
	direct, diagnostics := hfir.LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	direct = roundTripArtifact(t, direct)
	if got, want := graphCapabilities(graph), []capability.Capability{capability.Process}; !reflect.DeepEqual(got, want) {
		t.Fatalf("HFIR effects = %v, want process", got)
	}
	if got, want := programCapabilities(legacy), programCapabilities(direct); !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy capabilities = %v, HFIR capabilities = %v", got, want)
	}
	if !reflect.DeepEqual(programCapabilities(direct), []capability.Capability{capability.Process}) {
		t.Fatalf("emitted capabilities = %v, want process", programCapabilities(direct))
	}
	if got, want := execOperandShapes(legacy), execOperandShapes(direct); !reflect.DeepEqual(got, want) {
		t.Fatalf("EXEC operands\nAST %#v\nHFIR %#v", got, want)
	}

	const wantStdout = "phase2b-exec-marker\nL a 0\nL b 0\nkept 2\nphase2b-exec-kept\nresult 2\nmiss\nphase2b-exec-miss\nempty 0\n"
	legacyDenied := runBytecodeOutcome(legacy, "", nil)
	directDenied := runBytecodeOutcome(direct, "", nil)
	requireSameBytecodeOutcome(t, legacyDenied, directDenied)
	requireProcessDenial(t, legacyDenied, "printf", "phase2b-exec-marker")

	caps := []capability.Capability{capability.Process}
	legacyGranted := runBytecodeOutcome(legacy, "", caps)
	directGranted := runBytecodeOutcome(direct, "", caps)
	requireSameBytecodeOutcome(t, legacyGranted, directGranted)
	if legacyGranted.stdout != wantStdout || legacyGranted.stderr != "" || legacyGranted.exitCode != 0 || legacyGranted.vmError != nil || legacyGranted.panicVal != nil {
		t.Fatalf("granted outcome %#v", legacyGranted)
	}
	if strings.Contains(legacyGranted.stdout, "no") {
		t.Fatalf("granted stdout ran an untaken branch: %q", legacyGranted.stdout)
	}
}

// TestHFIRBytecodeFetchStaysUnsupported locks the fail-closed boundary for
// fetch. exec now lowers onto the existing EXEC opcode. The fetch
// conformance cases stay on the AST hosts.
func TestHFIRBytecodeFetchStaysUnsupported(t *testing.T) {
	_, graph := checkedHFIRGraph(t, readAbiFixture(t, "08_fetch_capability.howl"))
	program, diagnostics := hfir.LowerToBytecode(graph)
	if program != nil || len(diagnostics) != 1 || diagnostics[0].Code != hfir.BytecodeUnsupportedCode {
		t.Fatalf("LowerToBytecode() program=%v diags=%#v", program != nil, diagnostics)
	}
}

func assertNestedFsWriteShape(t *testing.T, graph *hfir.Graph) {
	t.Helper()
	var defuns, calls, whiles, ifWithElse, writes, mkdirs int
	fors := map[string]*hfir.Node{}
	writePaths := map[string]*hfir.Node{}
	mkdirPaths := map[string]*hfir.Node{}
	for _, node := range graph.Nodes {
		switch node.Kind {
		case "defun":
			defuns++
			if len(node.ControlEdges) != 0 {
				t.Fatalf("defun %s has control edges %v", node.ID, node.ControlEdges)
			}
		case "call":
			calls++
		case "while":
			whiles++
			if len(node.ControlEdges) != 2 || len(node.DataInputs) != 2 || node.DataInputs[0].Name != "condition" || node.DataInputs[1].Name != "body" || node.ControlEdges[0] != node.DataInputs[0].SourceNode || node.ControlEdges[1] != node.DataInputs[1].SourceNode {
				t.Fatalf("while %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
			}
		case "if":
			if len(node.ControlEdges) != 3 || len(node.DataInputs) != 3 || node.DataInputs[0].Name != "condition" || node.DataInputs[1].Name != "then" || node.DataInputs[2].Name != "else" {
				t.Fatalf("if %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
			}
			for index := range node.ControlEdges {
				if node.ControlEdges[index] != node.DataInputs[index].SourceNode {
					t.Fatalf("if %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
				}
			}
			ifWithElse++
		case "for":
			if len(node.ControlEdges) != 2 || len(node.DataInputs) != 2 || node.DataInputs[0].Name != "iterable" || node.DataInputs[1].Name != "body" || node.ControlEdges[0] != node.DataInputs[0].SourceNode || node.ControlEdges[1] != node.DataInputs[1].SourceNode {
				t.Fatalf("for %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
			}
			if _, ok := fors[node.Value]; ok {
				t.Fatalf("duplicate for iterator %q", node.Value)
			}
			fors[node.Value] = node
		case "write_file":
			writes++
			if len(node.ControlEdges) != 0 || len(node.DataInputs) != 2 || node.DataInputs[0].Name != "path" || node.DataInputs[1].Name != "data" {
				t.Fatalf("write_file %s edges %#v control %v", node.ID, node.DataInputs, node.ControlEdges)
			}
			writePaths[fsPathValue(t, graph, node)] = node
		case "mkdir":
			mkdirs++
			if len(node.ControlEdges) != 0 || len(node.DataInputs) != 1 || node.DataInputs[0].Name != "path" {
				t.Fatalf("mkdir %s edges %#v control %v", node.ID, node.DataInputs, node.ControlEdges)
			}
			mkdirPaths[fsPathValue(t, graph, node)] = node
		}
	}
	if defuns != 1 || calls != 2 || whiles != 2 || ifWithElse != 3 || len(fors) != 3 || writes != 5 || mkdirs != 4 {
		t.Fatalf("surface counts defun=%d call=%d while=%d if-else=%d for=%d write=%d mkdir=%d, want 1, 2, 2, 3, 3, 5, 4", defuns, calls, whiles, ifWithElse, len(fors), writes, mkdirs)
	}
	label := fors["label"]
	name := fors["name"]
	absent := fors["absent"]
	if label == nil || name == nil || absent == nil {
		t.Fatalf("for iterators = %v, want label, name, absent", forIteratorNames(fors))
	}
	if !hfirSubtreeHas(graph, label.DataInputs[1].SourceNode, func(node *hfir.Node) bool { return node == name }) {
		t.Fatal("label for does not contain the name for")
	}
	taken := graph.NodeByID(label.DataInputs[1].SourceNode)
	// The label for's body is the if. Its else is the taken write.
	if taken == nil || taken.Kind != "if" {
		t.Fatalf("label for body = %#v, want if", taken)
	}
	takenElse := taken.DataInputs[2].SourceNode
	if !hfirSubtreeHas(graph, takenElse, func(node *hfir.Node) bool { return node.Kind == "write_file" }) || !hfirSubtreeHas(graph, takenElse, func(node *hfir.Node) bool { return node.Kind == "mkdir" }) {
		t.Fatal("taken branch does not contain write_file and mkdir")
	}
	if !hfirSubtreeHas(graph, takenElse, func(node *hfir.Node) bool { return node == name }) || !hfirSubtreeHas(graph, takenElse, func(node *hfir.Node) bool { return node == absent }) {
		t.Fatal("taken branch does not contain both inner fors")
	}
	var countingWhile *hfir.Node
	for _, node := range graph.Nodes {
		if node.Kind != "while" {
			continue
		}
		cond := graph.NodeByID(node.DataInputs[0].SourceNode)
		if cond != nil && cond.Kind == "const" && cond.Value == "false" {
			if !hfirSubtreeHas(graph, node.DataInputs[1].SourceNode, func(child *hfir.Node) bool { return child.Kind == "mkdir" }) {
				t.Fatal("false while body does not mkdir")
			}
			continue
		}
		countingWhile = node
	}
	if countingWhile == nil || !hfirSubtreeHas(graph, countingWhile.DataInputs[1].SourceNode, func(node *hfir.Node) bool { return node == label }) {
		t.Fatal("counting while does not contain the label for")
	}
	if !hfirSubtreeHas(graph, name.DataInputs[1].SourceNode, func(node *hfir.Node) bool { return node.Kind == "write_file" }) {
		t.Fatal("name for does not contain write_file")
	}
	if !hfirSubtreeHas(graph, absent.DataInputs[1].SourceNode, func(node *hfir.Node) bool { return node.Kind == "write_file" }) {
		t.Fatal("empty for does not contain write_file")
	}
	var defunHoldsWrite bool
	for _, node := range graph.Nodes {
		if node.Kind == "defun" && hfirSubtreeHas(graph, node.ID, func(child *hfir.Node) bool {
			return child.Kind == "write_file" || child.Kind == "mkdir"
		}) {
			defunHoldsWrite = true
		}
	}
	if !defunHoldsWrite {
		t.Fatal("defun does not contain the filesystem effects")
	}
	for _, path := range []string{"-write.txt", "-loop.txt", "-skip.txt", "-else.txt", "-empty.txt"} {
		if !hasPathSuffix(writePaths, path) {
			t.Fatalf("missing write_file %s in %v", path, pathKeys(writePaths))
		}
	}
	for _, path := range []string{"-dir", "-miss", "-skip", "-dead"} {
		if !hasPathSuffix(mkdirPaths, path) {
			t.Fatalf("missing mkdir %s in %v", path, pathKeys(mkdirPaths))
		}
	}
}

func assertNestedEnvReadShape(t *testing.T, graph *hfir.Graph) {
	t.Helper()
	var defuns, calls, whiles, ifWithElse, reads int
	envKeys := map[string]int{}
	fors := map[string]*hfir.Node{}
	readPaths := map[string]*hfir.Node{}
	for _, node := range graph.Nodes {
		switch node.Kind {
		case "defun":
			defuns++
			if len(node.ControlEdges) != 0 {
				t.Fatalf("defun %s has control edges %v", node.ID, node.ControlEdges)
			}
		case "call":
			calls++
		case "while":
			whiles++
			if len(node.ControlEdges) != 2 || len(node.DataInputs) != 2 || node.DataInputs[0].Name != "condition" || node.DataInputs[1].Name != "body" || node.ControlEdges[0] != node.DataInputs[0].SourceNode || node.ControlEdges[1] != node.DataInputs[1].SourceNode {
				t.Fatalf("while %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
			}
		case "if":
			if len(node.ControlEdges) != 3 || len(node.DataInputs) != 3 || node.DataInputs[0].Name != "condition" || node.DataInputs[1].Name != "then" || node.DataInputs[2].Name != "else" {
				t.Fatalf("if %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
			}
			for index := range node.ControlEdges {
				if node.ControlEdges[index] != node.DataInputs[index].SourceNode {
					t.Fatalf("if %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
				}
			}
			ifWithElse++
		case "for":
			if len(node.ControlEdges) != 2 || len(node.DataInputs) != 2 || node.DataInputs[0].Name != "iterable" || node.DataInputs[1].Name != "body" || node.ControlEdges[0] != node.DataInputs[0].SourceNode || node.ControlEdges[1] != node.DataInputs[1].SourceNode {
				t.Fatalf("for %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
			}
			if _, ok := fors[node.Value]; ok {
				t.Fatalf("duplicate for iterator %q", node.Value)
			}
			fors[node.Value] = node
		case "env":
			if len(node.ControlEdges) != 0 || len(node.DataInputs) != 1 || node.DataInputs[0].Name != "value" {
				t.Fatalf("env %s edges %#v control %v", node.ID, node.DataInputs, node.ControlEdges)
			}
			envKeys[fsPathValue(t, graph, node)]++
		case "read_file":
			reads++
			if len(node.ControlEdges) != 0 || len(node.DataInputs) != 1 || node.DataInputs[0].Name != "path" {
				t.Fatalf("read_file %s edges %#v control %v", node.ID, node.DataInputs, node.ControlEdges)
			}
			readPaths[fsPathValue(t, graph, node)] = node
		}
	}
	if defuns != 1 || calls != 2 || whiles != 2 || ifWithElse != 3 || len(fors) != 3 || len(envKeys) != 4 || reads != 6 {
		t.Fatalf("surface counts defun=%d call=%d while=%d if-else=%d for=%d env-keys=%d read=%d, want 1, 2, 2, 3, 3, 4, 6", defuns, calls, whiles, ifWithElse, len(fors), len(envKeys), reads)
	}
	if envKeys["HOWLFRAME_ABI_SECRET"] != 2 || envKeys["HOWLFRAME_ABI_SKIP"] != 1 || envKeys["HOWLFRAME_ABI_ELSE"] != 1 || envKeys["HOWLFRAME_ABI_DEAD"] != 1 {
		t.Fatalf("env keys = %#v", envKeys)
	}
	label := fors["label"]
	name := fors["name"]
	absent := fors["absent"]
	if label == nil || name == nil || absent == nil {
		t.Fatalf("for iterators = %v, want label, name, absent", forIteratorNames(fors))
	}
	if !hfirSubtreeHas(graph, label.DataInputs[1].SourceNode, func(node *hfir.Node) bool { return node == name }) {
		t.Fatal("label for does not contain the name for")
	}
	taken := graph.NodeByID(label.DataInputs[1].SourceNode)
	if taken == nil || taken.Kind != "if" {
		t.Fatalf("label for body = %#v, want if", taken)
	}
	takenElse := taken.DataInputs[2].SourceNode
	if !hfirSubtreeHas(graph, takenElse, func(node *hfir.Node) bool { return node.Kind == "env" }) || !hfirSubtreeHas(graph, takenElse, func(node *hfir.Node) bool { return node.Kind == "read_file" }) {
		t.Fatal("taken branch does not contain env and read_file")
	}
	if !hfirSubtreeHas(graph, takenElse, func(node *hfir.Node) bool { return node == name }) || !hfirSubtreeHas(graph, takenElse, func(node *hfir.Node) bool { return node == absent }) {
		t.Fatal("taken branch does not contain both inner fors")
	}
	var countingWhile *hfir.Node
	for _, node := range graph.Nodes {
		if node.Kind != "while" {
			continue
		}
		cond := graph.NodeByID(node.DataInputs[0].SourceNode)
		if cond != nil && cond.Kind == "const" && cond.Value == "false" {
			body := node.DataInputs[1].SourceNode
			if !hfirSubtreeHas(graph, body, func(child *hfir.Node) bool { return child.Kind == "env" }) || !hfirSubtreeHas(graph, body, func(child *hfir.Node) bool { return child.Kind == "read_file" }) {
				t.Fatal("false while body does not contain env and read_file")
			}
			continue
		}
		countingWhile = node
	}
	if countingWhile == nil || !hfirSubtreeHas(graph, countingWhile.DataInputs[1].SourceNode, func(node *hfir.Node) bool { return node == label }) {
		t.Fatal("counting while does not contain the label for")
	}
	if !hfirSubtreeHas(graph, name.DataInputs[1].SourceNode, func(node *hfir.Node) bool { return node.Kind == "env" }) || !hfirSubtreeHas(graph, name.DataInputs[1].SourceNode, func(node *hfir.Node) bool { return node.Kind == "read_file" }) {
		t.Fatal("name for does not contain env and read_file")
	}
	if !hfirSubtreeHas(graph, absent.DataInputs[1].SourceNode, func(node *hfir.Node) bool { return node.Kind == "read_file" }) {
		t.Fatal("empty for does not contain read_file")
	}
	var defunHoldsRead bool
	for _, node := range graph.Nodes {
		if node.Kind == "defun" && hfirSubtreeHas(graph, node.ID, func(child *hfir.Node) bool {
			return child.Kind == "env" || child.Kind == "read_file"
		}) {
			defunHoldsRead = true
		}
	}
	if !defunHoldsRead {
		t.Fatal("defun does not contain env or read_file")
	}
	for _, path := range []string{"-read.txt", "-miss.txt", "-skip.txt", "-else.txt", "-empty.txt", "-dead.txt"} {
		if !hasPathSuffix(readPaths, path) {
			t.Fatalf("missing read_file %s in %v", path, pathKeys(readPaths))
		}
	}
}

func assertNestedExecShape(t *testing.T, graph *hfir.Graph) {
	t.Helper()
	var defuns, calls, whiles, ifWithElse, execs int
	args := map[string]int{}
	fors := map[string]*hfir.Node{}
	for _, node := range graph.Nodes {
		switch node.Kind {
		case "defun":
			defuns++
			if len(node.ControlEdges) != 0 {
				t.Fatalf("defun %s has control edges %v", node.ID, node.ControlEdges)
			}
		case "call":
			calls++
		case "while":
			whiles++
			if len(node.ControlEdges) != 2 || len(node.DataInputs) != 2 || node.DataInputs[0].Name != "condition" || node.DataInputs[1].Name != "body" || node.ControlEdges[0] != node.DataInputs[0].SourceNode || node.ControlEdges[1] != node.DataInputs[1].SourceNode {
				t.Fatalf("while %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
			}
		case "if":
			if len(node.ControlEdges) != 3 || len(node.DataInputs) != 3 || node.DataInputs[0].Name != "condition" || node.DataInputs[1].Name != "then" || node.DataInputs[2].Name != "else" {
				t.Fatalf("if %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
			}
			for index := range node.ControlEdges {
				if node.ControlEdges[index] != node.DataInputs[index].SourceNode {
					t.Fatalf("if %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
				}
			}
			ifWithElse++
		case "for":
			if len(node.ControlEdges) != 2 || len(node.DataInputs) != 2 || node.DataInputs[0].Name != "iterable" || node.DataInputs[1].Name != "body" || node.ControlEdges[0] != node.DataInputs[0].SourceNode || node.ControlEdges[1] != node.DataInputs[1].SourceNode {
				t.Fatalf("for %s control %v data %#v", node.ID, node.ControlEdges, node.DataInputs)
			}
			if _, ok := fors[node.Value]; ok {
				t.Fatalf("duplicate for iterator %q", node.Value)
			}
			fors[node.Value] = node
		case "exec":
			execs++
			if len(node.ControlEdges) != 0 || len(node.DataInputs) != 2 || node.DataInputs[0].Name != "cmd" || node.DataInputs[1].Name != "arg" {
				t.Fatalf("exec %s edges %#v control %v", node.ID, node.DataInputs, node.ControlEdges)
			}
			if got := fsPathValue(t, graph, node); got != "printf" {
				t.Fatalf("exec command = %q, want printf", got)
			}
			arg := graph.NodeByID(node.DataInputs[1].SourceNode)
			if arg == nil || arg.Kind != "const" || arg.LiteralKind != "STRING" || arg.Value == "" {
				t.Fatalf("exec argument = %#v, want a string const", arg)
			}
			args[arg.Value]++
		}
	}
	if defuns != 1 || calls != 2 || whiles != 2 || ifWithElse != 3 || len(fors) != 3 || execs != 7 {
		t.Fatalf("surface counts defun=%d call=%d while=%d if-else=%d for=%d exec=%d, want 1, 2, 2, 3, 3, 7", defuns, calls, whiles, ifWithElse, len(fors), execs)
	}
	for _, arg := range []string{"no-skip", "phase2b-exec-marker", "no-else", "no-empty", "no-dead", "phase2b-exec-kept", "phase2b-exec-miss"} {
		if args[arg] != 1 {
			t.Fatalf("exec args = %#v, missing %s", args, arg)
		}
	}
	label := fors["label"]
	name := fors["name"]
	absent := fors["absent"]
	if label == nil || name == nil || absent == nil {
		t.Fatalf("for iterators = %v, want label, name, absent", forIteratorNames(fors))
	}
	if !hfirSubtreeHas(graph, label.DataInputs[1].SourceNode, func(node *hfir.Node) bool { return node == name }) {
		t.Fatal("label for does not contain the name for")
	}
	taken := graph.NodeByID(label.DataInputs[1].SourceNode)
	if taken == nil || taken.Kind != "if" {
		t.Fatalf("label for body = %#v, want if", taken)
	}
	takenElse := taken.DataInputs[2].SourceNode
	if !hfirSubtreeHas(graph, takenElse, func(node *hfir.Node) bool { return node.Kind == "exec" }) {
		t.Fatal("taken branch does not contain exec")
	}
	if !hfirSubtreeHas(graph, takenElse, func(node *hfir.Node) bool { return node == name }) || !hfirSubtreeHas(graph, takenElse, func(node *hfir.Node) bool { return node == absent }) {
		t.Fatal("taken branch does not contain both inner fors")
	}
	var countingWhile *hfir.Node
	for _, node := range graph.Nodes {
		if node.Kind != "while" {
			continue
		}
		cond := graph.NodeByID(node.DataInputs[0].SourceNode)
		if cond != nil && cond.Kind == "const" && cond.Value == "false" {
			if !hfirSubtreeHas(graph, node.DataInputs[1].SourceNode, func(child *hfir.Node) bool { return child.Kind == "exec" }) {
				t.Fatal("false while body does not contain exec")
			}
			continue
		}
		countingWhile = node
	}
	if countingWhile == nil || !hfirSubtreeHas(graph, countingWhile.DataInputs[1].SourceNode, func(node *hfir.Node) bool { return node == label }) {
		t.Fatal("counting while does not contain the label for")
	}
	if !hfirSubtreeHas(graph, name.DataInputs[1].SourceNode, func(node *hfir.Node) bool { return node.Kind == "exec" }) {
		t.Fatal("name for does not contain exec")
	}
	if !hfirSubtreeHas(graph, absent.DataInputs[1].SourceNode, func(node *hfir.Node) bool { return node.Kind == "exec" }) {
		t.Fatal("empty for does not contain exec")
	}
	var defunHoldsExec bool
	for _, node := range graph.Nodes {
		if node.Kind == "defun" && hfirSubtreeHas(graph, node.ID, func(child *hfir.Node) bool { return child.Kind == "exec" }) {
			defunHoldsExec = true
		}
	}
	if !defunHoldsExec {
		t.Fatal("defun does not contain exec")
	}
}

func hasPathSuffix(paths map[string]*hfir.Node, suffix string) bool {
	for path := range paths {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func pathKeys(paths map[string]*hfir.Node) []string {
	keys := make([]string, 0, len(paths))
	for path := range paths {
		keys = append(keys, path)
	}
	sort.Strings(keys)
	return keys
}

func fsPathValue(t *testing.T, graph *hfir.Graph, node *hfir.Node) string {
	t.Helper()
	pathNode := graph.NodeByID(node.DataInputs[0].SourceNode)
	if pathNode == nil || pathNode.Kind != "const" || pathNode.LiteralKind != "STRING" || pathNode.Value == "" {
		t.Fatalf("%s path = %#v, want a string const", node.Kind, pathNode)
	}
	return pathNode.Value
}

type fsEntry struct {
	kind string
	body string
}

func compareFilesystemCompilers(t *testing.T, source string, paths []string, denyOpcode, wantStdout string, granted map[string]fsEntry) {
	t.Helper()
	root, graph := checkedHFIRGraph(t, source)
	legacy := roundTripArtifact(t, bytecode.CompileToBytecode(root))
	direct, diagnostics := hfir.LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	direct = roundTripArtifact(t, direct)
	if got, want := programCapabilities(legacy), programCapabilities(direct); !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy capabilities = %v, HFIR capabilities = %v", got, want)
	}
	if !reflect.DeepEqual(programCapabilities(direct), []capability.Capability{capability.Filesystem}) {
		t.Fatalf("emitted capabilities = %v, want filesystem", programCapabilities(direct))
	}
	var secret string
	if len(paths) > 0 {
		secret = paths[0]
	}
	compareCompilerFilesystem(t, legacy, direct, paths, denyOpcode, secret, wantStdout, granted)
}

func compareCompilerFilesystem(t *testing.T, legacy, direct *bytecode.BCProgram, paths []string, denyOpcode, secret, wantStdout string, granted map[string]fsEntry) {
	t.Helper()
	t.Cleanup(func() { removePaths(paths) })
	deniedAbsent := map[string]fsEntry{}
	for _, path := range paths {
		deniedAbsent[path] = fsEntry{kind: "absent"}
	}
	legacyDenied, legacyDeniedFS := runFilesystemOutcome(t, legacy, nil, paths)
	directDenied, directDeniedFS := runFilesystemOutcome(t, direct, nil, paths)
	requireSameBytecodeOutcome(t, legacyDenied, directDenied)
	requireFilesystemDenial(t, legacyDenied, denyOpcode, secret)
	if !reflect.DeepEqual(legacyDeniedFS, deniedAbsent) || !reflect.DeepEqual(directDeniedFS, deniedAbsent) {
		t.Fatalf("denial filesystem AST %#v HFIR %#v", legacyDeniedFS, directDeniedFS)
	}

	legacyGranted, legacyFS := runFilesystemOutcome(t, legacy, []capability.Capability{capability.Filesystem}, paths)
	directGranted, directFS := runFilesystemOutcome(t, direct, []capability.Capability{capability.Filesystem}, paths)
	requireSameBytecodeOutcome(t, legacyGranted, directGranted)
	if wantStdout != "" && legacyGranted.stdout != wantStdout {
		t.Fatalf("stdout = %q, want %q", legacyGranted.stdout, wantStdout)
	}
	if strings.Contains(legacyGranted.stdout, "no") || legacyGranted.stderr != "" || legacyGranted.exitCode != 0 || legacyGranted.vmError != nil || legacyGranted.panicVal != nil {
		t.Fatalf("granted outcome %#v", legacyGranted)
	}
	if !reflect.DeepEqual(legacyFS, granted) || !reflect.DeepEqual(directFS, granted) {
		t.Fatalf("grant filesystem\nAST %#v\nHFIR %#v\nwant %#v", legacyFS, directFS, granted)
	}
}

func runFilesystemOutcome(t *testing.T, program *bytecode.BCProgram, caps []capability.Capability, paths []string) (bytecodeOutcome, map[string]fsEntry) {
	t.Helper()
	removePaths(paths)
	outcome := runBytecodeOutcome(program, "", caps)
	return outcome, snapshotPaths(t, paths)
}

func snapshotPaths(t *testing.T, paths []string) map[string]fsEntry {
	t.Helper()
	snap := make(map[string]fsEntry, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			snap[path] = fsEntry{kind: "absent"}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if info.IsDir() {
			snap[path] = fsEntry{kind: "dir"}
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		snap[path] = fsEntry{kind: "file", body: string(body)}
	}
	return snap
}

func removePaths(paths []string) {
	for _, path := range paths {
		os.RemoveAll(path)
	}
}

func requireSameBytecodeOutcome(t *testing.T, legacy, direct bytecodeOutcome) {
	t.Helper()
	if legacy.stdout != direct.stdout || legacy.stderr != direct.stderr || legacy.exitCode != direct.exitCode || (legacy.panicVal != nil || direct.panicVal != nil) {
		t.Fatalf("AST bytecode outcome = %#v\nHFIR bytecode outcome = %#v", legacy, direct)
	}
	if legacy.vmError == nil && direct.vmError == nil {
		return
	}
	if legacy.vmError == nil || direct.vmError == nil || legacy.vmError.Code != direct.vmError.Code || legacy.vmError.Message != direct.vmError.Message || legacy.vmError.Opcode != direct.vmError.Opcode {
		t.Fatalf("AST error = %#v\nHFIR error = %#v", legacy.vmError, direct.vmError)
	}
}

func requireProcessDenial(t *testing.T, outcome bytecodeOutcome, command, marker string) {
	t.Helper()
	if outcome.vmError == nil || outcome.vmError.Code != "CAPABILITY_DENIED" || outcome.vmError.Opcode != "EXEC" || outcome.vmError.Message != "capability denied: process" {
		t.Fatalf("denial = %#v, want CAPABILITY_DENIED EXEC", outcome)
	}
	if outcome.stdout != "" || outcome.stderr != "" {
		t.Fatalf("denial produced output %#v", outcome)
	}
	blob := outcome.vmError.Message
	for _, secret := range []string{command, marker} {
		if secret != "" && strings.Contains(blob, secret) {
			t.Fatalf("denial leaked %q in %q", secret, blob)
		}
	}
}

func requireFilesystemDenial(t *testing.T, outcome bytecodeOutcome, opcode, secret string) {
	t.Helper()
	if outcome.vmError == nil || outcome.vmError.Code != "CAPABILITY_DENIED" || outcome.vmError.Opcode != opcode || outcome.vmError.Message != "capability denied: filesystem" {
		t.Fatalf("denial = %#v, want CAPABILITY_DENIED %s", outcome, opcode)
	}
	if outcome.stdout != "" || outcome.stderr != "" {
		t.Fatalf("denial produced output %#v", outcome)
	}
	if secret != "" && strings.Contains(outcome.stdout+outcome.stderr+outcome.vmError.Message, secret) {
		t.Fatalf("denial leaked %q in %#v", secret, outcome)
	}
}

func readAbiFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "tests", "conformance", "abi_v1", name)
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(source)
}

func forIteratorNames(fors map[string]*hfir.Node) []string {
	names := make([]string, 0, len(fors))
	for name := range fors {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// hfirSubtreeHas reports whether id or any data-edge descendant matches pred.
func hfirSubtreeHas(graph *hfir.Graph, id hfir.NodeID, pred func(*hfir.Node) bool) bool {
	seen := map[hfir.NodeID]bool{}
	var walk func(hfir.NodeID) bool
	walk = func(current hfir.NodeID) bool {
		if current == "" || seen[current] {
			return false
		}
		seen[current] = true
		node := graph.NodeByID(current)
		if node == nil {
			return false
		}
		if pred(node) {
			return true
		}
		for _, edge := range node.DataInputs {
			if walk(edge.SourceNode) {
				return true
			}
		}
		return false
	}
	return walk(id)
}

func TestHFIRBytecodeCapabilityConsistencyAndDenial(t *testing.T) {
	t.Setenv("HFIR_EQ_TEST_VALUE", "expected")
	root, graph := checkedHFIRGraph(t, `(cli_app (print (env "HFIR_EQ_TEST_VALUE")))`)
	legacy := bytecode.CompileToBytecode(root)
	direct, diagnostics := hfir.LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	if got, want := graphCapabilities(graph), programCapabilities(direct); !reflect.DeepEqual(got, want) {
		t.Fatalf("HFIR effects = %v, emitted bytecode capabilities = %v", got, want)
	}
	if got, want := programCapabilities(legacy), programCapabilities(direct); !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy capabilities = %v, HFIR capabilities = %v", got, want)
	}

	legacyOutcome := runBytecodeOutcome(legacy, "", nil)
	directOutcome := runBytecodeOutcome(direct, "", nil)
	if !reflect.DeepEqual(legacyOutcome, directOutcome) {
		t.Fatalf("AST denial = %#v\nHFIR denial = %#v", legacyOutcome, directOutcome)
	}
	if legacyOutcome.vmError == nil || legacyOutcome.vmError.Code != "CAPABILITY_DENIED" {
		t.Fatalf("denial = %#v, want CAPABILITY_DENIED", legacyOutcome)
	}
}

func checkedHFIRGraph(t *testing.T, source string) (*ast.Node, *hfir.Graph) {
	t.Helper()
	parsed := parser.NewParser(lexer.NewLexer(source), "hfir_equivalence.howl")
	root := parsed.ParseExpression()
	if parsed.Cur.Type != lexer.TokenEOF {
		t.Fatal("parser did not consume source")
	}
	ast.ApplyPatches(root)
	root = ast.ApplyWithContext(root, nil)
	root = ast.ApplyWithContext(root, nil)
	checker.Check(root)
	graph, err := hfir.LowerAST(root, "hfir_equivalence.howl")
	if err != nil {
		t.Fatalf("LowerAST() error = %v", err)
	}
	if diagnostics := hfir.NewVerifier(graph, hfir.TargetBytecode).Verify(); len(diagnostics) != 0 {
		t.Fatalf("Verify() diagnostics = %#v", diagnostics)
	}
	return root, graph
}

func roundTripArtifact(t *testing.T, program *bytecode.BCProgram) *bytecode.BCProgram {
	t.Helper()
	if err := bytecode.ValidateProgram(program); err != nil {
		t.Fatalf("ValidateProgram() error = %v", err)
	}
	var artifact bytes.Buffer
	if err := bytecode.WriteArtifact(&artifact, program); err != nil {
		t.Fatalf("WriteArtifact() error = %v", err)
	}
	decoded, err := bytecode.ReadArtifact(&artifact)
	if err != nil {
		t.Fatalf("ReadArtifact() error = %v", err)
	}
	return decoded
}

func runBytecodeOutcome(program *bytecode.BCProgram, stdin string, caps []capability.Capability) (outcome bytecodeOutcome) {
	var stdout, stderr bytes.Buffer
	machine := &BCVM{
		prog:        program,
		env:         NewBcEnv(nil),
		insts:       program.Main,
		stores:      newBCStoreRegistry(),
		Limits:      DefaultLimits,
		AllowedCaps: caps,
		In:          strings.NewReader(stdin),
		Out:         &stdout,
		ErrOut:      &stderr,
	}
	defer func() {
		outcome.stdout = stdout.String()
		outcome.stderr = stderr.String()
		if recovered := recover(); recovered != nil {
			switch value := recovered.(type) {
			case VmExit:
				outcome.exitCode = value.code
			case *VMError:
				copy := *value
				outcome.vmError = &copy
			default:
				outcome.panicVal = value
			}
		}
	}()
	machine.run(machine.insts, machine.env)
	return outcome
}

func TestHFIRBytecodeForRejectsNonListLikeAST(t *testing.T) {
	// The checker rejects this before either compiler on the CLI. This
	// comparison is the two bytecode emitters, which both still emit
	// FOR_INIT and then fail in the VM.
	source := `(cli_app (for item 1 (print "no")))`
	parsed := parser.NewParser(lexer.NewLexer(source), "hfir_equivalence.howl")
	root := parsed.ParseExpression()
	graph, err := hfir.LowerAST(root, "hfir_equivalence.howl")
	if err != nil {
		t.Fatal(err)
	}
	legacy := roundTripArtifact(t, bytecode.CompileToBytecode(root))
	direct, diagnostics := hfir.LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	direct = roundTripArtifact(t, direct)
	legacyOutcome := runBytecodeOutcome(legacy, "", nil)
	directOutcome := runBytecodeOutcome(direct, "", nil)
	if !reflect.DeepEqual(legacyOutcome, directOutcome) {
		t.Fatalf("AST bytecode outcome = %#v\nHFIR bytecode outcome = %#v", legacyOutcome, directOutcome)
	}
	if legacyOutcome.vmError == nil || legacyOutcome.vmError.Code != "TYPE_ERROR" {
		t.Fatalf("non-list for = %#v, want TYPE_ERROR", legacyOutcome)
	}
	if legacyOutcome.stdout != "" || directOutcome.stdout != "" {
		t.Fatalf("non-list for printed stdout: AST %q HFIR %q", legacyOutcome.stdout, directOutcome.stdout)
	}
}

func TestHFIRBytecodeWhileRejectsNonBoolLikeAST(t *testing.T) {
	// The checker rejects this before either compiler on the CLI. This
	// comparison is the two bytecode emitters, which both still emit the
	// while jumps and then fail in the VM.
	source := `(cli_app (while 1 (print "no")))`
	parsed := parser.NewParser(lexer.NewLexer(source), "hfir_equivalence.howl")
	root := parsed.ParseExpression()
	graph, err := hfir.LowerAST(root, "hfir_equivalence.howl")
	if err != nil {
		t.Fatal(err)
	}
	legacy := roundTripArtifact(t, bytecode.CompileToBytecode(root))
	direct, diagnostics := hfir.LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	direct = roundTripArtifact(t, direct)
	legacyOutcome := runBytecodeOutcome(legacy, "", nil)
	directOutcome := runBytecodeOutcome(direct, "", nil)
	if !reflect.DeepEqual(legacyOutcome, directOutcome) {
		t.Fatalf("AST bytecode outcome = %#v\nHFIR bytecode outcome = %#v", legacyOutcome, directOutcome)
	}
	if legacyOutcome.vmError == nil || legacyOutcome.vmError.Code != "TYPE_ERROR" {
		t.Fatalf("non-bool while = %#v, want TYPE_ERROR", legacyOutcome)
	}
	if legacyOutcome.stdout != "" || directOutcome.stdout != "" {
		t.Fatalf("non-bool while printed stdout: AST %q HFIR %q", legacyOutcome.stdout, directOutcome.stdout)
	}
}

func TestHFIRBytecodeCallArityRejectsLikeAST(t *testing.T) {
	// The checker rejects this arity before either compiler. This comparison
	// is the two bytecode emitters, which both still emit CALL.
	source := `(cli_app
  (defun id (n)
    (type_hints (n int) (return int))
    (return n))
  (print (call id)))`
	parsed := parser.NewParser(lexer.NewLexer(source), "hfir_equivalence.howl")
	root := parsed.ParseExpression()
	graph, err := hfir.LowerAST(root, "hfir_equivalence.howl")
	if err != nil {
		t.Fatal(err)
	}
	legacy := roundTripArtifact(t, bytecode.CompileToBytecode(root))
	direct, diagnostics := hfir.LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	direct = roundTripArtifact(t, direct)
	legacyOutcome := runBytecodeOutcome(legacy, "", nil)
	directOutcome := runBytecodeOutcome(direct, "", nil)
	if legacyOutcome.vmError == nil || directOutcome.vmError == nil {
		t.Fatalf("AST error = %#v\nHFIR error = %#v", legacyOutcome, directOutcome)
	}
	if legacyOutcome.vmError.Code != directOutcome.vmError.Code || legacyOutcome.vmError.Message != directOutcome.vmError.Message {
		t.Fatalf("AST error = %#v\nHFIR error = %#v", legacyOutcome.vmError, directOutcome.vmError)
	}
	if legacyOutcome.stdout != "" || directOutcome.stdout != "" {
		t.Fatalf("arity rejection printed stdout: AST %q HFIR %q", legacyOutcome.stdout, directOutcome.stdout)
	}
}

// execOperandShapes records each EXEC as its function, then the command,
// then each argument. Those values are the LOAD_CONST instructions
// immediately before EXEC, which is the operand order CompileToBytecode
// already emits. IntOperand is the argument count, so the window is that
// count plus the command.
func execOperandShapes(program *bytecode.BCProgram) []string {
	var shapes []string
	collect := func(where string, insts []bytecode.BCInstruction) {
		for index, inst := range insts {
			if inst.Op != bytecode.OpExec {
				continue
			}
			argc := int(inst.IntOperand)
			if argc < 0 || index < argc+1 {
				shapes = append(shapes, where+"\x00short")
				continue
			}
			parts := []string{where}
			literal := true
			for _, prev := range insts[index-argc-1 : index] {
				text, ok := prev.ValueOperand.(string)
				if prev.Op != bytecode.OpLoadConst || !ok {
					literal = false
					break
				}
				parts = append(parts, text)
			}
			if !literal {
				shapes = append(shapes, where+"\x00nonliteral")
				continue
			}
			shapes = append(shapes, strings.Join(parts, "\x00"))
		}
	}
	collect("main", program.Main)
	names := make([]string, 0, len(program.Functions))
	for name := range program.Functions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fn := program.Functions[name]
		if fn != nil {
			collect(name, fn.Instructions)
		}
	}
	return shapes
}

func graphCapabilities(graph *hfir.Graph) []capability.Capability {
	seen := make(map[capability.Capability]bool)
	for _, node := range graph.Nodes {
		for _, effect := range node.Effects {
			if effect.Type == "capability" && effect.Capability != "" {
				seen[capability.Capability(effect.Capability)] = true
			}
		}
	}
	return sortedCapabilities(seen)
}

func programCapabilities(program *bytecode.BCProgram) []capability.Capability {
	seen := make(map[capability.Capability]bool)
	collect := func(insts []bytecode.BCInstruction) {
		for _, inst := range insts {
			if spec, ok := bytecode.Registry[inst.Op]; ok && spec.Capability != capability.None {
				seen[spec.Capability] = true
			}
		}
	}
	collect(program.Main)
	for _, fn := range program.Functions {
		if fn != nil {
			collect(fn.Instructions)
		}
	}
	return sortedCapabilities(seen)
}

func sortedCapabilities(seen map[capability.Capability]bool) []capability.Capability {
	result := make([]capability.Capability, 0, len(seen))
	for cap := range seen {
		result = append(result, cap)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
