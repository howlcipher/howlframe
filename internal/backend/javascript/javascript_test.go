package javascript

import (
	"strings"
	"testing"
	"zero/internal/checker"
	"zero/internal/lexer"
	"zero/internal/parser"
)

func TestGenerateJSCodePreservesFloatComparison(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(web_app (let (score 1.0) (if (> score 0.8) (print "yes") (print "no"))))`), "float.zero").ParseExpression()
	checker.Check(root)
	appCode, _ := GenerateJSCode(root)
	if !strings.Contains(appCode, "score > 0.8") {
		t.Fatalf("generated code did not preserve float comparison:\n%s", appCode)
	}
}
