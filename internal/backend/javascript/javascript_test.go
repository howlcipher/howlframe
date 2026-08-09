package javascript

import (
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
	"strings"
	"testing"
)

func TestGenerateJSCodePreservesFloatComparison(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(web_app (let (score 1.0) (if (> score 0.8) (print "yes") (print "no"))))`), "float.howl").ParseExpression()
	checker.Check(root)
	appCode, _ := GenerateJSCode(root)
	if !strings.Contains(appCode, "score > 0.8") {
		t.Fatalf("generated code did not preserve float comparison:\n%s", appCode)
	}
}
