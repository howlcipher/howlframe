package hfir

import (
	"os"
	"path/filepath"
	"testing"

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
			t.Fatalf("node %s kind %s has control edges %v; v1 LowerAST does not populate CFG edges", node.ID, node.Kind, node.ControlEdges)
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

func TestLoweredABIV1DefunCallAndWhileAreNotExecutable(t *testing.T) {
	sources := []string{
		`(cli_app (defun add_values (a b) (type_hints (a int) (b int) (return int)) (return (+ a b))) (print 1))`,
		`(cli_app (while false (print "no")))`,
	}
	for _, source := range sources {
		root := parser.NewParser(lexer.NewLexer(source), "abi_deferred.howl").ParseExpression()
		checker.Check(root)
		graph, err := LowerAST(root, "abi_deferred.howl")
		if err != nil {
			t.Fatalf("LowerAST(%q) error = %v", source, err)
		}
		program, diags := LowerToBytecode(graph)
		if program != nil {
			t.Fatalf("LowerToBytecode(%q) returned a program; v1 must fail closed", source)
		}
		if len(diags) == 0 || diags[0].Code != BytecodeUnsupportedCode {
			t.Fatalf("LowerToBytecode(%q) diags = %#v, want %s", source, diags, BytecodeUnsupportedCode)
		}
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
