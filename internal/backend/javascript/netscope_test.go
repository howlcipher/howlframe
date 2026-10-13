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

// #165 on generated JavaScript (Node): exact host grants, IP literals and
// every redirect hop are checked before a request reaches the target.
func TestJSFetchHostScope(t *testing.T) {
	t.Setenv("NO_PROXY", "127.0.0.1,localhost")
	const secret = "netscope-js-secret"
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

	for _, tc := range []struct {
		name, url, grant string
		allowed          bool
	}{
		{"exact", target.URL + "/a", "network:fetch=" + tHost, true},
		{"coarse network", target.URL + "/a", "network", false},
		{"other port", target.URL + "/a", "network:fetch=127.0.0.1:1", false},
		{"localhost name for ip grant", "http://localhost:" + port + "/a", "network:fetch=" + tHost, false},
		{"metadata ip", "http://169.254.169.254/latest/meta-data/", "network:fetch=" + tHost, false},
		{"userinfo trick", "http://" + tHost + "@169.254.169.254/", "network:fetch=" + tHost, false},
		{"redirect to ungranted", redirector.URL + "/x", "network:fetch=" + rHost, false},
		{"redirect to metadata", redirector.URL + "/to-meta", "network:fetch=" + rHost, false},
		{"redirect to granted", redirector.URL + "/x", "network:fetch=" + rHost + ",network:fetch=" + tHost, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := parser.NewParser(lexer.NewLexer(`(web_app (print (bytes_to_string (fetch "`+tc.url+`" "GET"))))`), "fetch.howl").ParseExpression()
			checker.Check(root)
			code, _ := GenerateJSCode(root)
			before := hits.Load()
			out, errText, exit := runJS(t, code, "HOWLFRAME_ALLOW_CAPS="+tc.grant)
			if tc.allowed {
				if exit != 0 || strings.TrimSpace(out) != secret || hits.Load()-before != 1 {
					t.Fatalf("allowed fetch: exit=%d out=%q err=%q hits=%d", exit, out, errText, hits.Load()-before)
				}
				return
			}
			if exit == 0 || !strings.Contains(errText, "CAPABILITY_DENIED") || strings.Contains(out, secret) || hits.Load() != before {
				t.Fatalf("want denial: exit=%d out=%q err=%q hits=%d", exit, out, errText, hits.Load()-before)
			}
		})
	}
}
