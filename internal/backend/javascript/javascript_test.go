package javascript

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestGenerateJSRequestReads(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(web_app
  (let (req "request")
    (let (status (req_query req "status"))
      (let (auth (req_header req "Authorization"))
        (let (id (req_path req "id"))
          (print status auth id))))))`), "request.howl").ParseExpression()
	checker.Check(root)
	appCode, _ := GenerateJSCode(root)
	for _, want := range []string{
		"function howlRequestRead",
		`howlRequestRead("req_query"`,
		`howlRequestRead("req_header"`,
		`howlRequestRead("req_path"`,
		"TYPE_ERROR:",
	} {
		if !strings.Contains(appCode, want) {
			t.Fatalf("generated JavaScript is missing %q:\n%s", want, appCode)
		}
	}

	script := requestReadJSHelper() + `
const assert = require("node:assert");
const req = {
  url: "http://localhost/tasks/9?status=open&status=closed&empty=",
  headers: { Authorization: "Bearer z" },
  pathParams: { id: "9" }
};
assert.strictEqual(howlRequestRead("req_query", req, "status"), "open");
assert.strictEqual(howlRequestRead("req_query", req, "missing"), "");
assert.strictEqual(howlRequestRead("req_query", req, "empty"), "");
assert.strictEqual(howlRequestRead("req_query", req, "Status"), "");
assert.strictEqual(howlRequestRead("req_header", req, "authorization"), "Bearer z");
assert.strictEqual(howlRequestRead("req_header", req, "X-Missing"), "");
assert.strictEqual(howlRequestRead("req_path", req, "id"), "9");
assert.strictEqual(howlRequestRead("req_path", req, "nope"), "");
assert.throws(() => howlRequestRead("req_query", null, "status"), /TYPE_ERROR/);
assert.throws(() => howlRequestRead("req_query", req, ""), /TYPE_ERROR/);
assert.throws(() => howlRequestRead("req_query", req, 1), /TYPE_ERROR/);
assert.throws(() => howlRequestRead("req_path", { pathParams: { id: 9 } }, "id"), /TYPE_ERROR/);
if (typeof Request === "function") {
  const live = new Request("http://localhost/tasks/9?status=open", { headers: { Authorization: "Bearer z" } });
  assert.strictEqual(howlRequestRead("req_query", live, "status"), "open");
  assert.strictEqual(howlRequestRead("req_header", live, "authorization"), "Bearer z");
  assert.strictEqual(howlRequestRead("req_path", live, "id"), "");
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "request_read.js")
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", path).CombinedOutput()
	if err != nil {
		t.Fatalf("node request reads: %v\n%s", err, out)
	}
}

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
	if !strings.Contains(appCode, `howlFrameParseJSON("{}")`) {
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
		`howlFrameFetch("http://example.test/tasks", "POST", "{\"title\":\"Ship\"}")`,
		`return fetch(url, init).then(function (r) { return r.text(); })`,
		`howlFrameParseJSON(response)`,
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
	if !strings.Contains(appCode, `howlFrameMapGet(d, "rows")`) || !strings.Contains(appCode, `?? ""`) {
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

func TestGenerateJSMapKeysSortsObjectKeys(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(
		`(web_app (let (counts (dict ("beta" "2") ("alpha" "1"))) (for name (map_keys counts) (print name))))`),
		"keys.howl").ParseExpression()
	checker.Check(root)
	appCode, _ := GenerateJSCode(root)
	if !strings.Contains(appCode, "Object.keys(_d).sort(howlFrameCompareUTF8)") {
		t.Fatalf("map_keys did not sort object keys in UTF-8 byte order:\n%s", appCode)
	}
	if strings.Contains(appCode, ".sort()") {
		t.Fatalf("map_keys still uses JavaScript's default UTF-16 sort:\n%s", appCode)
	}
	if !strings.Contains(appCode, "TYPE_ERROR: map_keys expected dict") {
		t.Fatalf("map_keys did not fail closed on a non-dict:\n%s", appCode)
	}
}
