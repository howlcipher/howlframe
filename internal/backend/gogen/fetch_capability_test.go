package gogen

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestGoFetchRequiresGrantBeforeRequest(t *testing.T) {
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

	source := `(cli_app
  (let (value (fetch "` + secretURL + `" "GET"))
    (print (bytes_to_string value))))`
	root := parser.NewParser(lexer.NewLexer(source), "fetch.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if strings.Contains(code, "http.NewRequest(") && strings.Count(code, "http.NewRequest(") != 1 {
		t.Fatalf("fetch builds a request outside the helper:\n%s", code)
	}
	if strings.Contains(code, "howlFrameFetchClient.Do(") && strings.Count(code, "howlFrameFetchClient.Do(") != 1 {
		t.Fatalf("fetch dials outside the helper:\n%s", code)
	}
	if !strings.Contains(code, "howlFrameFetchBytes(") {
		t.Fatalf("fetch is not mediated:\n%s", code)
	}
	helperAt := strings.Index(code, "func howlFrameFetch(")
	if helperAt < 0 {
		t.Fatalf("missing howlFrameFetch helper:\n%s", code)
	}
	helper := code[helperAt:]
	denyAt := strings.Index(helper, "CAPABILITY_DENIED")
	reqAt := strings.Index(helper, "http.NewRequest")
	doAt := strings.Index(helper, "howlFrameFetchClient.Do")
	if denyAt < 0 || reqAt < 0 || doAt < 0 || denyAt > reqAt || denyAt > doAt {
		t.Fatalf("grant check must precede the HTTP request:\n%s", helper)
	}

	rootDir := moduleRoot(t)
	assertGoCapabilityDenied(t, rootDir, code, "network", "the response or URL", []string{secretBody, "phase2d-secret-url"}, func(label string) {
		if hits.Load() != 0 {
			t.Fatalf("%s: denial performed %d HTTP request(s)", label, hits.Load())
		}
	}, [][2]string{
		{"unset grant", "__UNSET__"},
		{"empty grant", "HOWLFRAME_ALLOW_CAPS="},
		{"filesystem grant", "HOWLFRAME_ALLOW_CAPS=filesystem"},
		{"coarse network grant (#165)", "HOWLFRAME_ALLOW_CAPS=network"},
		{"other host grant", "HOWLFRAME_ALLOW_CAPS=network:fetch=localhost:" + srv.URL[strings.LastIndex(srv.URL, ":")+1:]},
	})

	granted, grantErr := runGenerated(t, rootDir, code, "HOWLFRAME_ALLOW_CAPS=network:fetch="+strings.TrimPrefix(srv.URL, "http://"))
	if grantErr != "" {
		t.Fatalf("network grant failed: %s\n%s", grantErr, granted)
	}
	if hits.Load() != 1 {
		t.Fatalf("network grant performed %d HTTP request(s), want 1", hits.Load())
	}
	if strings.TrimSpace(granted) != secretBody {
		t.Fatalf("granted stdout = %q, want %s", granted, secretBody)
	}
}

func TestGoFetchTryLetDeniesBeforeRequest(t *testing.T) {
	t.Setenv("NO_PROXY", "127.0.0.1,localhost")
	t.Setenv("no_proxy", "127.0.0.1,localhost")
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = io.WriteString(w, "phase2d-secret-bytes")
	}))
	t.Cleanup(srv.Close)
	secretURL := srv.URL + "/phase2d-secret-url"

	source := `(cli_app
  (try_let (value (fetch "` + secretURL + `" "GET"))
    (catch err (print "caught-io"))
    (print "fetched-ok")))`
	root := parser.NewParser(lexer.NewLexer(source), "fetch.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if strings.Contains(code, ":= howlFrameFetchBytes(") {
		t.Fatalf("try_let fetch dropped the error return:\n%s", code)
	}
	if !strings.Contains(code, "value, err := howlFrameFetch(") {
		t.Fatalf("try_let fetch is not mediated:\n%s", code)
	}
	if strings.Count(code, "howlFrameFetchClient.Do(") != 1 {
		t.Fatalf("try_let fetch dials at the call site:\n%s", code)
	}

	rootDir := moduleRoot(t)
	assertGoCapabilityDenied(t, rootDir, code, "network", "the denial as an IO error", []string{"phase2d-secret-url", "caught-io", "fetched-ok"}, func(string) {
		if hits.Load() != 0 {
			t.Fatalf("empty grant performed %d HTTP request(s)", hits.Load())
		}
	}, [][2]string{{"empty grant", "HOWLFRAME_ALLOW_CAPS="}})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedURL := "http://" + ln.Addr().String() + "/phase2d-closed"
	ln.Close()
	missSource := `(cli_app
  (try_let (value (fetch "` + closedURL + `" "GET"))
    (catch err (print "caught-io"))
    (print "fetched-ok")))`
	missRoot := parser.NewParser(lexer.NewLexer(missSource), "fetch.howl").ParseExpression()
	checker.Check(missRoot)
	missCode, _ := GenerateCode(missRoot)
	granted, grantErr := runGenerated(t, rootDir, missCode, "HOWLFRAME_ALLOW_CAPS=network:fetch="+ln.Addr().String())
	if grantErr != "" {
		t.Fatalf("network grant failed: %s\n%s", grantErr, granted)
	}
	if strings.TrimSpace(granted) != "caught-io" {
		t.Fatalf("granted miss stdout = %q, want caught-io", granted)
	}
}

func TestGoFetchExpressionPositionIsMediated(t *testing.T) {
	const source = `(cli_app (print (bytes_to_string (fetch "http://phase2d-expr.invalid" "GET"))))`
	root := parser.NewParser(lexer.NewLexer(source), "fetch.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if !strings.Contains(code, `howlFrameFetchBytes("http://phase2d-expr.invalid", "GET", nil)`) {
		t.Fatalf("expression-position fetch is not mediated:\n%s", code)
	}
	if strings.Count(code, "http.NewRequest(") != 1 || strings.Count(code, "howlFrameFetchClient.Do(") != 1 {
		t.Fatalf("expression-position fetch bypasses the grant:\n%s", code)
	}
}

func TestGoFetchBodyIsMediated(t *testing.T) {
	const source = `(cli_app
  (try_let (value (fetch "http://phase2d-expr.invalid" "POST" "payload"))
    (catch err (print err))
    (print (bytes_to_string value))))`
	root := parser.NewParser(lexer.NewLexer(source), "fetch.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if !strings.Contains(code, `howlFrameFetch("http://phase2d-expr.invalid", "POST", strings.NewReader("payload"))`) {
		t.Fatalf("fetch body is not mediated:\n%s", code)
	}
	if strings.Count(code, "http.NewRequest(") != 1 {
		t.Fatalf("fetch body builds a request at the call site:\n%s", code)
	}
}
