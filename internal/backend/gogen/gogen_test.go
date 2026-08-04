package gogen

import (
	"strings"
	"testing"
	"zero/internal/checker"
	"zero/internal/lexer"
	"zero/internal/parser"
)

func TestGenerateCodePreservesFloatComparison(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(cli_app (let (score 1.0) (if (> score 0.8) (print "yes") (print "no"))))`), "float.zero").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if !strings.Contains(code, "score > 0.8") {
		t.Fatalf("generated code did not preserve float comparison:\n%s", code)
	}
}
