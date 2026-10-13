package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestIssue143FetchBodyEveryPath: a fetch body is sent with the same
// semantics on the interpreter, AST bytecode and HFIR bytecode (-run-bc).
// The body is stringified, so a number is sent as its decimal text, and a
// fetch without a body sends none. A denied network grant sends nothing.
// Local httptest server only.
func TestIssue143FetchBodyEveryPath(t *testing.T) {
	bin := c1c2Binary(t)
	var mu sync.Mutex
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, r.Method+" "+string(b))
		mu.Unlock()
		_, _ = w.Write([]byte("echo:" + string(b)))
	}))
	defer srv.Close()
	dir := t.TempDir()
	src := filepath.Join(dir, "fb.howl")
	program := `(cli_app
  (print (bytes_to_string (fetch "` + srv.URL + `/a" "POST" "hello")))
  (let (n 42) (print (bytes_to_string (fetch "` + srv.URL + `/b" "PUT" n))))
  (print (bytes_to_string (fetch "` + srv.URL + `/c" "GET"))))
`
	if err := os.WriteFile(src, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	const wantOut = "echo:hello\necho:42\necho:\n"
	wantReqs := []string{"POST hello", "PUT 42", "GET "}
	reset := func() []string {
		mu.Lock()
		defer mu.Unlock()
		r := got
		got = nil
		return r
	}
	fetchGrant := "network:fetch=" + strings.TrimPrefix(srv.URL, "http://")
	paths := map[string]func() (string, int){
		"interpreter": func() (string, int) { return issue141Run(bin, "-allow-caps", fetchGrant, "-run", src) },
		"ast_bytecode": func() (string, int) {
			art := filepath.Join(dir, "ast.hfbc")
			if out, code := issue141Run(bin, "build", src, "-o", art); code != 0 {
				return out, code
			}
			return issue141Run(bin, "-run-bc", "-allow-caps", fetchGrant, art)
		},
		"hfir_bytecode": func() (string, int) {
			art := filepath.Join(dir, "hfir.hfbc")
			if out, code := issue141Run(bin, "-compile-hfir-bc", src, "-o", art); code != 0 {
				return out, code
			}
			return issue141Run(bin, "-run-bc", "-allow-caps", fetchGrant, art)
		},
	}
	for _, name := range []string{"interpreter", "ast_bytecode", "hfir_bytecode"} {
		reset()
		out, code := paths[name]()
		reqs := reset()
		if code != 0 || out != wantOut {
			t.Errorf("%s: exit %d stdout %q, want %q", name, code, out, wantOut)
		}
		if strings.Join(reqs, "|") != strings.Join(wantReqs, "|") {
			t.Errorf("%s: requests %q, want %q", name, reqs, wantReqs)
		}
	}

	// Denied: the bytecode paths must not open a connection or send the body.
	for _, compile := range [][]string{{"build", src, "-o"}, {"-compile-hfir-bc", src, "-o"}} {
		art := filepath.Join(dir, "denied-"+strings.TrimLeft(compile[0], "-")+".hfbc")
		if out, code := issue141Run(bin, append(compile, art)...); code != 0 {
			t.Fatalf("%v: %s", compile, out)
		}
		reset()
		// #165: no grant, and the coarse network grant, both deny.
		for _, grant := range []string{"", "network"} {
			args := []string{"-run-bc", art}
			if grant != "" {
				args = []string{"-run-bc", "-allow-caps", grant, art}
			}
			out, code := issue141Run(bin, args...)
			if code == 0 || !strings.Contains(out, "capability denied: network") {
				t.Errorf("%v denied (%q): exit %d %s", compile, grant, code, out)
			}
			if reqs := reset(); len(reqs) != 0 {
				t.Errorf("%v denied (%q) sent %q", compile, grant, reqs)
			}
		}
	}
}
