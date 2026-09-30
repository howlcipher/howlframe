package hfir

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestLoweredABIV1WasmRejectionSetIsClosed(t *testing.T) {
	if LoweredABIV1 != "lowered-hfir-abi/v1" {
		t.Fatalf("ABI version = %q", LoweredABIV1)
	}
	want := []string{"exec", "spawn_agent", "http_server_start"}
	if len(WasmInfeasibleKinds) != len(want) {
		t.Fatalf("wasm rejection set = %v, want %v", WasmInfeasibleKinds, want)
	}
	for i, kind := range want {
		if WasmInfeasibleKinds[i] != kind {
			t.Fatalf("wasm rejection set = %v, want %v", WasmInfeasibleKinds, want)
		}
		if isFeasible(kind, "wasm") {
			t.Errorf("isFeasible(%q, wasm) = true, want false", kind)
		}
	}

	graph := NewGraph()
	for _, kind := range WasmInfeasibleKinds {
		graph.AddNode(&Node{Kind: kind})
	}
	wasmDiags := NewVerifier(graph, "wasm").Verify()
	var infeasible int
	for _, diag := range wasmDiags {
		if diag.Code == "HFIR_TARGET_INFEASIBLE" {
			infeasible++
			if diag.ContractVersion != DiagnosticContractVersion {
				t.Errorf("contract version = %q", diag.ContractVersion)
			}
		}
	}
	if infeasible != len(WasmInfeasibleKinds) {
		t.Fatalf("wasm HFIR_TARGET_INFEASIBLE count = %d, diags = %#v", infeasible, wasmDiags)
	}

	for _, target := range []string{"", "bytecode", "interpreter", "go", "javascript"} {
		for _, diag := range NewVerifier(graph, target).Verify() {
			if diag.Code == "HFIR_TARGET_INFEASIBLE" {
				t.Errorf("target %q rejected a v1 wasm-only kind: %#v", target, diag)
			}
		}
	}

	for _, kind := range []string{"map_keys", "map_get", "print", "binary", "if", "let"} {
		for _, target := range []string{"wasm", "bytecode", "interpreter", "go", "javascript"} {
			if !isFeasible(kind, target) {
				t.Errorf("isFeasible(%q, %q) = false, want true", kind, target)
			}
		}
	}
}

func TestLoweredABIV1CoreFixtureHasNoControlEdges(t *testing.T) {
	graph := lowerFixture(t, filepath.Join("..", "..", "tests", "conformance", "abi_v1", "01_arith_if.howl"))
	if len(graph.Nodes) == 0 {
		t.Fatal("arith fixture lowered no nodes")
	}
	for _, node := range graph.Nodes {
		if len(node.ControlEdges) != 0 {
			t.Fatalf("node %s kind %s has control edges %v; the arith fixture has no while", node.ID, node.Kind, node.ControlEdges)
		}
	}
	program, diags := LowerToBytecode(graph)
	if len(diags) != 0 {
		t.Fatalf("experimental lowerer rejected the v1 arith fixture: %#v", diags)
	}
	if program == nil || len(program.Main) == 0 {
		t.Fatal("experimental lowerer emitted no bytecode for the v1 arith fixture")
	}
}

func TestLoweredABIV1WhileIsExecutable(t *testing.T) {
	source := `(cli_app (while false (print "no")))`
	root := parser.NewParser(lexer.NewLexer(source), "abi_while.howl").ParseExpression()
	checker.Check(root)
	graph, err := LowerAST(root, "abi_while.howl")
	if err != nil {
		t.Fatalf("LowerAST(%q) error = %v", source, err)
	}
	var loops int
	for _, node := range graph.Nodes {
		if node.Kind != "while" {
			if len(node.ControlEdges) != 0 {
				t.Fatalf("node %s kind %s has control edges; only while is in this slice", node.ID, node.Kind)
			}
			continue
		}
		loops++
		if len(node.ControlEdges) != 2 || len(node.DataInputs) != 2 || node.DataInputs[0].Name != "condition" || node.DataInputs[1].Name != "body" {
			t.Fatalf("while shape = %#v", node)
		}
		if node.ControlEdges[0] != node.DataInputs[0].SourceNode || node.ControlEdges[1] != node.DataInputs[1].SourceNode {
			t.Fatalf("while control edges = %v, data = %#v", node.ControlEdges, node.DataInputs)
		}
	}
	if loops != 1 {
		t.Fatalf("while nodes = %d", loops)
	}
	program, diags := LowerToBytecode(graph)
	if program == nil || len(diags) != 0 {
		t.Fatalf("LowerToBytecode(%q) program=%v diags=%#v", source, program != nil, diags)
	}
	if err := bytecode.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
}

func lowerFixture(t *testing.T, path string) *Graph {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	root := parser.NewParser(lexer.NewLexer(string(data)), filepath.Base(path)).ParseExpression()
	checker.Check(root)
	graph, err := LowerAST(root, filepath.Base(path))
	if err != nil {
		t.Fatalf("LowerAST: %v", err)
	}
	return graph
}
