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

func TestGenerateCodeMapKeysSortsBothDictShapes(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(
		`(cli_app (let (counts (dict ("beta" "2") ("alpha" "1"))) (print (str_join (map_keys counts) ","))))`),
		"keys.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	for _, want := range []string{
		"case map[string]string:",
		"case map[string]any:",
		"sort.Strings(_keys)",
		"TYPE_ERROR: map_keys expected dict",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("generated Go is missing %q:\n%s", want, code)
		}
	}
}
