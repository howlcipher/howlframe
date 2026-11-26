package vm

import (
	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
	"testing"
)

func parseAndCompile(t *testing.T, code string) (*ast.Node, *bytecode.BCProgram) {
	t.Helper()
	l := lexer.NewLexer(code)
	p := parser.NewParser(l, "test.howl")
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
		for _, backend := range []struct {
			name        string
			useBytecode bool
		}{
			{"interpret", false},
			{"bytecode", true},
		} {
			t.Run(tt.name+"_"+backend.name, func(t *testing.T) {
				assertRuntimeResult(t, tt.code, nil, tt.stdin, tt.wantStdout, tt.wantStderr, tt.wantExit, backend.useBytecode)
			})
		}
	}
}
