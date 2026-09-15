package testutil

import (
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

// ParseTestProgram parses a single HowlFrame source expression for use in tests.
// It fails the test if the parser does not reach EOF.
func ParseTestProgram(t *testing.T, source, filename string) *ast.Node {
	t.Helper()
	p := parser.NewParser(lexer.NewLexer(source), filename)
	root := p.ParseExpression()
	if p.Cur.Type != lexer.TokenEOF {
		t.Fatalf("parser stopped at %s", p.Cur.Value)
	}
	return root
}

// AssertStringsContain verifies that every string in want appears at least once
// in got.
func AssertStringsContain(t *testing.T, got, want []string) {
	t.Helper()
	for _, expected := range want {
		found := false
		for _, value := range got {
			if value == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected value %q in %v", expected, got)
		}
	}
}
