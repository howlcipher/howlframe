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

func TestGenerateJSTryLetParseJson(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(web_app (try_let (data (parse_json Map "{}")) (catch err (print err)) (print data)))`), "test.howl").ParseExpression()
	checker.Check(root)
	appCode, _ := GenerateJSCode(root)
	if !strings.Contains(appCode, `JSON.parse("{}")`) {
		t.Fatalf("generated code did not compile parse_json correctly:\n%s", appCode)
	}
}

// TestGenerateJSFetchWithBody covers the exact fetch composition used by the
// external board: a POST body is returned as text and can then be supplied to
// parse_json inside try_let. Browser execution remains an application-level
// concern; this test keeps the generator contract explicit.
func TestGenerateJSFetchWithBody(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(web_app
  (try_let (response (fetch "http://example.test/tasks" "POST" "{\"title\":\"Ship\"}"))
    (catch fetch_err (print fetch_err))
    (try_let (task (parse_json Task response))
      (catch parse_err (print parse_err))
      (print (map_get task "title")))))`), "fetch.howl").ParseExpression()
	checker.Check(root)
	appCode, _ := GenerateJSCode(root)
	for _, want := range []string{
		`await fetch("http://example.test/tasks", { method: "POST", body: "{\"title\":\"Ship\"}" })`,
		`JSON.parse(response)`,
	} {
		if !strings.Contains(appCode, want) {
			t.Fatalf("generated code is missing %q:\n%s", want, appCode)
		}
	}
}
