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

// time_now is supported by the bytecode VM and the Go backend. A web_app that
// renders relative timestamps needs the same reading of the clock, so the
// JavaScript backend must lower it rather than reject it as unknown.
func TestGenerateJSTimeNow(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(
		`(web_app (let (now (time_now)) (print (to_string now))))`), "time.howl").ParseExpression()
	checker.Check(root)
	appCode, _ := GenerateJSCode(root)
	if !strings.Contains(appCode, "Math.floor(Date.now() / 1000)") {
		t.Fatalf("time_now did not lower to unix seconds:\n%s", appCode)
	}
	if strings.Contains(appCode, "Unknown statement") {
		t.Fatalf("time_now still reported as unknown:\n%s", appCode)
	}
}

// A for loop may iterate any list-valued expression, not only a bound symbol.
// Emitting the iterable's raw node value produced "for (let m of )" - invalid
// JavaScript generated with no diagnostic at all.
func TestGenerateJSForOverExpression(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(
		`(web_app (let (d (dict ("rows" (list "a")))) (for r (map_get d "rows") (print r))))`),
		"for.howl").ParseExpression()
	checker.Check(root)
	appCode, _ := GenerateJSCode(root)
	if strings.Contains(appCode, "of )") {
		t.Fatalf("for loop lost its iterable expression:\n%s", appCode)
	}
	if !strings.Contains(appCode, `(d["rows"] ?? "")`) {
		t.Fatalf("for loop did not emit the iterable expression:\n%s", appCode)
	}
}

// A web_app's top-level statements normally await something. Emitting them at
// the top level of the file produced code a classic <script> cannot parse,
// while wrapping the whole file in a module would stop function declarations
// from being reachable as globals for inline handlers.
func TestGenerateJSTopLevelAwaitIsWrapped(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(
		`(web_app (defun boot () (type_hint return "void") (print "hi")) (call boot))`), "boot.howl").ParseExpression()
	checker.Check(root)
	appCode, _ := GenerateJSCode(root)
	if !strings.Contains(appCode, "(async () => {") {
		t.Fatalf("top-level statements were not wrapped:\n%s", appCode)
	}
	if !strings.HasPrefix(strings.TrimSpace(appCode), "async function boot") {
		t.Fatalf("function declarations must stay at top level:\n%s", appCode)
	}
}

// on_event previously emitted no trailing semicolon. Automatic semicolon
// insertion does not apply before a "(", so any following statement was parsed
// as a call of the addEventListener result.
func TestGenerateJSOnEventTerminatesStatement(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(
		`(web_app (defun boot () (type_hint return "void") (print "hi"))
		  (on_event (dom_query "#b") "click" (lambda (e) (print "clicked")))
		  (call boot))`), "evt.howl").ParseExpression()
	checker.Check(root)
	appCode, _ := GenerateJSCode(root)
	if !strings.Contains(appCode, "});") {
		t.Fatalf("on_event did not terminate its statement:\n%s", appCode)
	}
	body := appCode[strings.Index(appCode, "(async () => {"):]
	if !strings.Contains(body, "});\n") {
		t.Fatalf("on_event statement is not separated from the next:\n%s", body)
	}
}
