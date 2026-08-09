package bytecode

import (
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
	"testing"
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
