package gogen

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

// #165 on generated Go: exact host grants, IP literals and redirect hops are
// checked by the helper before a request reaches the target.
func TestGoFetchHostScope(t *testing.T) {
	t.Setenv("NO_PROXY", "127.0.0.1,localhost")
	t.Setenv("no_proxy", "127.0.0.1,localhost")
	const secret = "netscope-go-secret"
	var hits atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		io.WriteString(w, secret)
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/to-meta" {
			http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
			return
		}
		http.Redirect(w, r, target.URL+"/hop", http.StatusFound)
	}))
	defer redirector.Close()
	tHost := strings.TrimPrefix(target.URL, "http://")
	rHost := strings.TrimPrefix(redirector.URL, "http://")
	port := tHost[strings.LastIndex(tHost, ":")+1:]
	rootDir := moduleRoot(t)
	crashPath := filepath.Join(rootDir, "crash.json")
	t.Cleanup(func() { os.Remove(crashPath) })

	for _, tc := range []struct {
		name, url, grant string
		allowed          bool
	}{
		{"exact", target.URL + "/a", "network:fetch=" + tHost, true},
		{"coarse network", target.URL + "/a", "network", false},
		{"localhost name for ip grant", "http://localhost:" + port + "/a", "network:fetch=" + tHost, false},
		{"short ipv4", "http://127.1:" + port + "/a", "network:fetch=" + tHost, false},
		{"metadata ip", "http://169.254.169.254/latest/meta-data/", "network:fetch=" + tHost, false},
		{"redirect to ungranted", redirector.URL + "/x", "network:fetch=" + rHost, false},
		{"redirect to metadata", redirector.URL + "/to-meta", "network:fetch=" + rHost, false},
		{"redirect to granted", redirector.URL + "/x", "network:fetch=" + rHost + ",network:fetch=" + tHost, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := `(cli_app (print (bytes_to_string (fetch "` + tc.url + `" "GET"))))`
			root := parser.NewParser(lexer.NewLexer(source), "fetch.howl").ParseExpression()
			checker.Check(root)
			code, _ := GenerateCode(root)
			before := hits.Load()
			os.Remove(crashPath)
			out, errText := runGenerated(t, rootDir, code, "HOWLFRAME_ALLOW_CAPS="+tc.grant)
			if tc.allowed {
				if errText != "" || strings.TrimSpace(out) != secret || hits.Load()-before != 1 {
					t.Fatalf("allowed fetch: err=%q out=%q hits=%d", errText, out, hits.Load()-before)
				}
				return
			}
			if errText == "" || strings.Contains(out, secret) || hits.Load() != before {
				t.Fatalf("want denial: err=%q out=%q hits=%d", errText, out, hits.Load()-before)
			}
			crash, _ := os.ReadFile(crashPath)
			if !strings.Contains(string(crash)+out, "CAPABILITY_DENIED") {
				t.Fatalf("denial is not CAPABILITY_DENIED: %s %s", crash, out)
			}
		})
	}
}
