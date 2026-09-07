package bytecode

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestCompileStoreOperations(t *testing.T) {
	source := `(cli_app
		(store_open store "memory://session")
		(store_put store (str_join (list "task" "1") ":")
			(dict ("title" (str_join (list "Add" "login") " "))
				("status" "open")))
		(store_get store (str_join (list "task" "1") ":"))
		(store_delete store (str_join (list "task" "1") ":")))`

	lx := lexer.NewLexer(source)
	p := parser.NewParser(lx, "store_test.howl")
	prog := CompileToBytecode(p.ParseExpression())

	var storeOps []BCInstruction
	for _, inst := range prog.Main {
		switch inst.Op {
		case OpStoreOpen, OpStorePut, OpStoreGet, OpStoreDelete:
			storeOps = append(storeOps, inst)
		}
	}

	if len(storeOps) != 4 {
		t.Fatalf("got %d store instructions, want 4", len(storeOps))
	}
	if storeOps[0].Op != OpStoreOpen || storeOps[0].StringOperand != "store" ||
		storeOps[0].StringOperand2 != "memory://session" {
		t.Fatalf("unexpected STORE_OPEN instruction: %#v", storeOps[0])
	}
	if storeOps[1].Op != OpStorePut || storeOps[1].StringOperand != "store" {
		t.Fatalf("unexpected STORE_PUT instruction: %#v", storeOps[1])
	}
	if storeOps[2].Op != OpStoreGet || storeOps[2].StringOperand != "store" {
		t.Fatalf("unexpected STORE_GET instruction: %#v", storeOps[2])
	}
	if storeOps[3].Op != OpStoreDelete || storeOps[3].StringOperand != "store" {
		t.Fatalf("unexpected STORE_DELETE instruction: %#v", storeOps[3])
	}

	assertStoreSpec(t, OpStoreOpen, "STORE_OPEN", 0, 0)
	assertStoreSpec(t, OpStorePut, "STORE_PUT", 2, 0)
	assertStoreSpec(t, OpStoreGet, "STORE_GET", 1, 1)
	assertStoreSpec(t, OpStoreDelete, "STORE_DELETE", 1, 0)
}

func TestCompileFloatLiteral(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(cli_app (print 0.8))`), "float.howl").ParseExpression()
	program := CompileToBytecode(root)
	if len(program.Main) < 2 || program.Main[1].Op != OpPrint {
		t.Fatalf("unexpected bytecode: %#v", program.Main)
	}
	if program.Main[0].ValueOperand != float64(0.8) {
		t.Fatalf("got float constant %#v", program.Main[0].ValueOperand)
	}
}

func assertStoreSpec(t *testing.T, op Opcode, name string, pops int, pushes int) {
	t.Helper()

	spec, ok := Registry[op]
	if !ok {
		t.Fatalf("opcode %d is not registered", op)
	}
	if spec.Name != name || spec.Pops != pops || spec.Pushes != pushes {
		t.Fatalf("unexpected %s spec: %#v", name, spec)
	}
	if spec.Capability != capability.Database {
		t.Fatalf("%s capability = %q, want %q", name, spec.Capability, capability.Database)
	}
}

func TestCompileDefunWithTypedParametersAndReturnType(t *testing.T) {
	source := `(cli_app
		(defun add ((a int) (b int)) int
			(return (+ a b)))
		(print (call add 40 2)))`

	lx := lexer.NewLexer(source)
	p := parser.NewParser(lx, "defun_test.howl")
	prog := CompileToBytecode(p.ParseExpression())

	fn, ok := prog.Functions["add"]
	if !ok {
		t.Fatalf("expected function add in prog.Functions")
	}
	if len(fn.Params) != 2 || fn.Params[0] != "a" || fn.Params[1] != "b" {
		t.Fatalf("expected params [a b], got %v", fn.Params)
	}
	for _, inst := range fn.Instructions {
		if inst.Op == OpLoadVar && inst.StringOperand == "int" {
			t.Fatalf("found erroneous LOAD_VAR int in function body instructions: %#v", fn.Instructions)
		}
	}
}

func TestCompileDefunAdversarialEdgeCases(t *testing.T) {
	source := `(cli_app
		(defun zero_param () int
			(return 42))
		(defun mixed_params (x (y int) z) string
			(print x)
			(return z))
		(lazy_synthesize synth ((a int) (b string)) "prompt docstring")
	)`

	lx := lexer.NewLexer(source)
	p := parser.NewParser(lx, "adversarial_defun.howl")
	prog := CompileToBytecode(p.ParseExpression())

	fn0, ok := prog.Functions["zero_param"]
	if !ok {
		t.Fatalf("missing zero_param function")
	}
	if len(fn0.Params) != 0 {
		t.Fatalf("zero_param expected 0 params, got %v", fn0.Params)
	}
	for _, inst := range fn0.Instructions {
		if inst.Op == OpLoadVar && inst.StringOperand == "int" {
			t.Fatalf("zero_param has leaked return type instruction: %#v", fn0.Instructions)
		}
	}

	fnMixed, ok := prog.Functions["mixed_params"]
	if !ok {
		t.Fatalf("missing mixed_params function")
	}
	if len(fnMixed.Params) != 3 || fnMixed.Params[0] != "x" || fnMixed.Params[1] != "y" || fnMixed.Params[2] != "z" {
		t.Fatalf("mixed_params expected [x y z], got %v", fnMixed.Params)
	}
	for _, inst := range fnMixed.Instructions {
		if inst.Op == OpLoadVar && inst.StringOperand == "string" {
			t.Fatalf("mixed_params has leaked return type instruction: %#v", fnMixed.Instructions)
		}
	}

	fnSynth, ok := prog.Functions["synth"]
	if !ok {
		t.Fatalf("missing synth function")
	}
	if len(fnSynth.Params) != 2 || fnSynth.Params[0] != "a" || fnSynth.Params[1] != "b" {
		t.Fatalf("synth expected [a b], got %v", fnSynth.Params)
	}
	if !fnSynth.LazySynthesize || fnSynth.Docstring != "prompt docstring" {
		t.Fatalf("synth lazy synthesize fields corrupted: %#v", fnSynth)
	}
}

// TestUnmarshalJSONRejectsUnknownOpcode verifies that BCInstruction.UnmarshalJSON
// returns an error for an unrecognized opcode name instead of silently setting
// Op to OpUnknown.
func TestUnmarshalJSONRejectsUnknownOpcode(t *testing.T) {
	raw := `{"op":"DOES_NOT_EXIST","string_operand":"x"}`
	var inst BCInstruction
	err := json.Unmarshal([]byte(raw), &inst)
	if err == nil {
		t.Fatalf("expected error for unknown opcode, got nil (Op=%d)", inst.Op)
	}
	if !strings.Contains(err.Error(), "unknown opcode") {
		t.Fatalf("expected 'unknown opcode' error, got: %v", err)
	}
}

// TestUnmarshalJSONAcceptsKnownOpcode verifies that a valid opcode name
// round-trips correctly through JSON.
func TestUnmarshalJSONAcceptsKnownOpcode(t *testing.T) {
	raw := `{"op":"LOAD_CONST","value_operand":"hello"}`
	var inst BCInstruction
	err := json.Unmarshal([]byte(raw), &inst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.Op != OpLoadConst {
		t.Fatalf("expected OpLoadConst (%d), got %d", OpLoadConst, inst.Op)
	}
}

// TestUnmarshalJSONAcceptsEmptyOpcode verifies that an empty op string does
// not trigger the unknown-opcode error (backwards compatibility for
// zero-value instructions).
func TestUnmarshalJSONAcceptsEmptyOpcode(t *testing.T) {
	raw := `{"op":""}`
	var inst BCInstruction
	err := json.Unmarshal([]byte(raw), &inst)
	if err != nil {
		t.Fatalf("unexpected error for empty op: %v", err)
	}
	if inst.Op != OpUnknown {
		t.Fatalf("expected OpUnknown for empty op, got %d", inst.Op)
	}
}
