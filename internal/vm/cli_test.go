package vm

import (
	"bytes"
	"strings"
	"testing"
	"zero/internal/ast"
	"zero/internal/bytecode"
	"zero/internal/lexer"
	"zero/internal/parser"
)

func parseAndCompile(t *testing.T, code string) (*ast.Node, *bytecode.BCProgram) {
	t.Helper()
	l := lexer.NewLexer(code)
	p := parser.NewParser(l, "test.zero")
	node := p.ParseExpression()
	ast.ApplyPatches(node)
	node = ast.ApplyWithContext(node, nil)
	node = ast.ApplyWithContext(node, nil)
	prog := bytecode.CompileToBytecode(node)
	return node, prog
}

func TestStandaloneCLISemantics(t *testing.T) {
	tests := []struct {
		name       string
		code       string
		stdin      string
		wantStdout string
		wantStderr string
		wantExit   int
	}{
		{
			name:       "stdin",
			code:       `(cli_app (let (name (read_line)) (print name)))`,
			stdin:      "Ada\n",
			wantStdout: "Ada\n",
			wantExit:   0,
		},
		{
			name:       "stderr",
			code:       `(cli_app (stderr "warning\n") (print "ok"))`,
			wantStdout: "ok\n",
			wantStderr: "warning\n",
			wantExit:   0,
		},
		{
			name:     "exit 0",
			code:     `(cli_app (exit 0))`,
			wantExit: 0,
		},
		{
			name:       "exit 7",
			code:       `(cli_app (exit 7) (print "never"))`,
			wantExit:   7,
			wantStdout: "",
		},
		{
			name: "combined",
			code: `(cli_app
  (let (value (read_line))
    (if (= value "")
      (do
        (stderr "input required")
        (exit 2))
      (print value))))`,
			stdin:      "",
			wantStderr: "input required",
			wantExit:   2,
		},
		{
			name: "combined success",
			code: `(cli_app
  (let (value (read_line))
    (if (= value "")
      (do
        (stderr "input required")
        (exit 2))
      (print value))))`,
			stdin:      "hello\n",
			wantStdout: "hello\n",
			wantExit:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_interpret", func(t *testing.T) {
			node, _ := parseAndCompile(t, tt.code)
			in := strings.NewReader(tt.stdin)
			var out, errOut bytes.Buffer

			exitCode := Interpret(node, nil, in, &out, &errOut)
			if exitCode != tt.wantExit {
				t.Errorf("exitCode = %d, want %d", exitCode, tt.wantExit)
			}
			if out.String() != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", out.String(), tt.wantStdout)
			}
			if errOut.String() != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", errOut.String(), tt.wantStderr)
			}
		})

		t.Run(tt.name+"_bytecode", func(t *testing.T) {
			_, prog := parseAndCompile(t, tt.code)
			in := strings.NewReader(tt.stdin)
			var out, errOut bytes.Buffer

			exitCode := RunBytecode(prog, nil, nil, in, &out, &errOut)
			if exitCode != tt.wantExit {
				t.Errorf("exitCode = %d, want %d", exitCode, tt.wantExit)
			}
			if out.String() != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", out.String(), tt.wantStdout)
			}
			if errOut.String() != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", errOut.String(), tt.wantStderr)
			}
		})
	}
}
