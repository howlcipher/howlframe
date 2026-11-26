package javascript

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestJSFetchRequiresGrantBeforeRequest(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node runtime not available")
	}
	t.Setenv("NO_PROXY", "127.0.0.1,localhost")
	t.Setenv("no_proxy", "127.0.0.1,localhost")
	const secretBody = "phase2d-secret-bytes"
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = io.WriteString(w, secretBody)
	}))
	t.Cleanup(srv.Close)
	secretURL := srv.URL + "/phase2d-secret-url"

	source := `(web_app
  (let (value (fetch "` + secretURL + `" "GET"))
    (print (bytes_to_string value))))`
	root := parser.NewParser(lexer.NewLexer(source), "fetch.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	if strings.Contains(code, "await fetch(") {
		t.Fatalf("fetch dials at the call site:\n%s", code)
	}
	if !strings.Contains(code, "howlFrameFetch(") {
		t.Fatalf("fetch is not mediated:\n%s", code)
	}
	helperAt := strings.Index(code, "function howlFrameFetch")
	if helperAt < 0 {
		t.Fatalf("missing howlFrameFetch helper:\n%s", code)
	}
	helper := code[helperAt:]
	denyAt := strings.Index(helper, "CAPABILITY_DENIED")
	fetchAt := strings.Index(helper, "fetch(u.href, init)")
	if denyAt < 0 || fetchAt < 0 || denyAt > fetchAt {
		t.Fatalf("grant check must precede the HTTP request:\n%s", helper)
	}

	assertJSCapabilityDenied(t, code, "network", "the response or URL", []string{secretBody, "phase2d-secret-url"}, func(label string) {
		if hits.Load() != 0 {
			t.Fatalf("%s: denial performed %d HTTP request(s)", label, hits.Load())
		}
	}, [][2]string{
		{"unset grant", "__UNSET__"},
		{"empty grant", "HOWLFRAME_ALLOW_CAPS="},
		{"filesystem grant", "HOWLFRAME_ALLOW_CAPS=filesystem"},
		{"coarse network grant (#165)", "HOWLFRAME_ALLOW_CAPS=network"},
	})

	grantedOut, grantedErr, grantedCode := runJS(t, code, "HOWLFRAME_ALLOW_CAPS=network:fetch="+strings.TrimPrefix(srv.URL, "http://"))
	if grantedCode != 0 {
		t.Fatalf("network grant failed (%d): %s", grantedCode, grantedErr)
	}
	if hits.Load() != 1 {
		t.Fatalf("network grant performed %d HTTP request(s), want 1", hits.Load())
	}
	if strings.TrimSpace(grantedOut) != secretBody {
		t.Fatalf("granted stdout = %q, want %s", grantedOut, secretBody)
	}
}

func TestJSFetchExpressionPositionIsMediated(t *testing.T) {
	const source = `(web_app (print (bytes_to_string (fetch "http://phase2d-expr.invalid" "GET"))))`
	root := parser.NewParser(lexer.NewLexer(source), "fetch.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	if !strings.Contains(code, `howlFrameFetch("http://phase2d-expr.invalid", "GET")`) {
		t.Fatalf("expression-position fetch is not mediated:\n%s", code)
	}
	if strings.Contains(code, "await fetch(") {
		t.Fatalf("expression-position fetch bypasses the grant:\n%s", code)
	}
}

func TestJSFetchBodyIsMediated(t *testing.T) {
	const source = `(web_app (print (bytes_to_string (fetch "http://phase2d-expr.invalid" "POST" "payload"))))`
	root := parser.NewParser(lexer.NewLexer(source), "fetch.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	if !strings.Contains(code, `howlFrameFetch("http://phase2d-expr.invalid", "POST", "payload")`) {
		t.Fatalf("fetch body is not mediated:\n%s", code)
	}
	if strings.Contains(code, "await fetch(") {
		t.Fatalf("fetch body bypasses the grant:\n%s", code)
	}
	helperAt := strings.Index(code, "function howlFrameFetch")
	if helperAt < 0 || strings.Index(code[helperAt:], "CAPABILITY_DENIED") > strings.Index(code[helperAt:], "fetch(u.href, init)") {
		t.Fatalf("grant check must precede fetch:\n%s", code)
	}
}
