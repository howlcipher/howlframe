package javascript

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

// Headers are needed by real browser callers sending NDJSON to a validating
// same-origin API. They must use the existing mediated helper, including denial.
func TestJSFetchSourceHeadersReachServer(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || r.Header.Get("Content-Type") != "application/x-ndjson" || string(body) != "entry\n" {
			t.Errorf("request method=%s headers=%v body=%q", r.Method, r.Header, body)
		}
		_, _ = io.WriteString(w, "received")
	}))
	defer srv.Close()
	source := `(web_app (let (response (fetch "` + srv.URL + `" "POST" "entry\n" (dict ("Content-Type" "application/x-ndjson")))) (print response)))`
	root := parser.NewParser(lexer.NewLexer(source), "headers.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	if !strings.Contains(code, `"Content-Type": "application/x-ndjson"`) {
		t.Fatal("source headers not emitted")
	}
	_, denied, exit := runJS(t, code, "HOWLFRAME_ALLOW_CAPS=")
	if exit == 0 || !strings.Contains(denied, "CAPABILITY_DENIED") || hits.Load() != 0 {
		t.Fatalf("denial exit=%d stderr=%s hits=%d", exit, denied, hits.Load())
	}
	out, stderr, exit := runJS(t, code, "HOWLFRAME_ALLOW_CAPS=network:fetch="+strings.TrimPrefix(srv.URL, "http://"))
	if exit != 0 || strings.TrimSpace(out) != "received" || hits.Load() != 1 {
		t.Fatalf("allowed exit=%d out=%s stderr=%s hits=%d", exit, out, stderr, hits.Load())
	}
}

func TestJSURLEncodeSourceEscapesQueryValues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "a/b?%# café😀") }))
	defer srv.Close()
	source := `(web_app (let (value (fetch "` + srv.URL + `" "GET")) (print (url_encode value))))`
	root := parser.NewParser(lexer.NewLexer(source), "url.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	out, stderr, exit := runJS(t, code, "HOWLFRAME_ALLOW_CAPS=network:fetch="+strings.TrimPrefix(srv.URL, "http://"))
	if exit != 0 || strings.TrimSpace(out) != "a%2Fb%3F%25%23%20caf%C3%A9%F0%9F%98%80" {
		t.Fatalf("url encoding exit=%d out=%s stderr=%s", exit, out, stderr)
	}
}
