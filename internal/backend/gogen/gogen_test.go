package gogen

import (
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
	"strings"
	"testing"
)

func TestGenerateCodePreservesFloatComparison(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(cli_app (let (score 1.0) (if (> score 0.8) (print "yes") (print "no"))))`), "float.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if !strings.Contains(code, "score > 0.8") {
		t.Fatalf("generated code did not preserve float comparison:\n%s", code)
	}
}
