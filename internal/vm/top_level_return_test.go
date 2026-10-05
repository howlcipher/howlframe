package vm

import (
	"bytes"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestTopLevelReturnExitCodeBytecode(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{name: "cli_app return int", src: `(cli_app (return 7))`, want: 7},
		{name: "wasm_math return", src: `(wasm_app (if (> (+ 2 3) 4) (return (* 6 7)) (return 0)))`, want: 42},
		{name: "wasm_app return zero", src: `(wasm_app (return 0))`, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := parseRoot(t, tc.src)
			prog := bytecode.CompileToBytecode(root)
			var stdout, stderr bytes.Buffer
			evidence := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), nil, strings.NewReader(""), &stdout, &stderr, 0)
			if evidence.RuntimeFailure != nil {
				t.Fatalf("unexpected runtime failure: %+v stderr=%q", evidence.RuntimeFailure, stderr.String())
			}
			if evidence.ExitCode != tc.want {
				t.Fatalf("exit code = %d, want %d", evidence.ExitCode, tc.want)
			}
			if stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("expected empty stdout/stderr; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestWasmAppPrintBytecode(t *testing.T) {
	root := parseRoot(t, `(wasm_app (do (print (* 6 7)) 0))`)
	prog := bytecode.CompileToBytecode(root)
	var stdout, stderr bytes.Buffer
	evidence := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), nil, strings.NewReader(""), &stdout, &stderr, 0)
	if evidence.RuntimeFailure != nil {
		t.Fatalf("unexpected runtime failure: %+v", evidence.RuntimeFailure)
	}
	if evidence.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0", evidence.ExitCode)
	}
	if got := stdout.String(); got != "42\n" {
		t.Fatalf("stdout = %q, want %q", got, "42\n")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func parseRoot(t *testing.T, src string) *ast.Node {
	t.Helper()
	lx := lexer.NewLexer(src)
	p := parser.NewParser(lx, "test.howl")
	root := p.ParseExpression()
	if root == nil {
		t.Fatal("parse returned nil")
	}
	return root
}
