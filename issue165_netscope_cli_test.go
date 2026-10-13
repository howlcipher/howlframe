package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// #165: route handlers hold no implicit network. Response forms need no
// grant; outbound fetch needs an exact network:fetch=host[:port] grant on
// every path, and every redirect hop is re-checked.

const netscopeSecret = "netscope-admin-secret-7f3a"

// serveNetscope starts src on path (bytecode or hfir) and returns a getter
// for the server's routes plus the server's combined log.
func serveNetscope(t *testing.T, binary, path, src, grant string, port int) (func(string) (int, string), *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "srv.howl")
	if err := os.WriteFile(file, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	flag := "-compile-bc"
	if path == "hfir" {
		flag = "-compile-hfir-bc"
	}
	artifact := filepath.Join(dir, "srv.hfbc")
	if out, err := exec.Command(binary, flag, file, "-o", artifact).CombinedOutput(); err != nil {
		if strings.Contains(string(out), "not in the Phase-1 executable subset") {
			t.Skipf("%s does not lower http_server yet: %s", path, out)
		}
		t.Fatalf("compile: %v %s", err, out)
	}
	cmd := exec.Command(binary, "-allow-caps", grant, "-run-bc", artifact)
	log := &bytes.Buffer{}
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	get := func(route string) (int, string) {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, route))
		if err != nil {
			return 0, err.Error()
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	for i := 0; i < 100; i++ {
		if code, _ := get("/plain"); code != 0 {
			return get, log
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server never answered: %s", log.String())
	return nil, nil
}

func TestIssue165RouteNetworkScope(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	for i, path := range []string{"bytecode", "hfir"} {
		t.Run(path, func(t *testing.T) {
			port := 18720 + i*10
			self := fmt.Sprintf("127.0.0.1:%d", port)
			src := fmt.Sprintf(`(http_server %d
  (route "/plain" (lambda (req) (do (res_header "X-Scope" "none") (res_json 200 (dict ("method" (req_method req)))))))
  (route "/admin" (lambda (req) (res 200 "text/plain" %q)))
  (route "/self" (lambda (req) (res 200 "text/plain" (bytes_to_string (fetch "http://%s/admin" "GET")))))
  (route "/meta" (lambda (req) (res 200 "text/plain" (bytes_to_string (fetch "http://169.254.169.254/latest/meta-data/" "GET")))))
  (route "/granted" (grants "network:fetch=%s") (lambda (req) (res 200 "text/plain" (bytes_to_string (fetch "http://%s/admin" "GET"))))))`,
				port, netscopeSecret, self, self, self)
			// The runner holds the fetch grants; routes get only what they list.
			grant := "network,network:fetch=" + self + ",network:fetch=169.254.169.254"
			get, log := serveNetscope(t, binary, path, src, grant, port)

			// 1. Response forms work with no grant at all on the route.
			if code, body := get("/plain"); code != 200 || !strings.Contains(body, `"method":"GET"`) {
				t.Fatalf("/plain = %d %q", code, body)
			}
			// 2. A route fetching its own /admin is denied; the client sees a
			// generic 500 (#166) and never the secret or the runtime error.
			for _, route := range []string{"/self", "/meta"} {
				code, body := get(route)
				if code != 500 || strings.Contains(body, netscopeSecret) || strings.Contains(body, "CAPABILITY_DENIED") || strings.Contains(body, "169.254") {
					t.Fatalf("%s = %d %q, want generic 500", route, code, body)
				}
			}
			// 3. Positive control: the same fetch with an explicit route grant works,
			// so the denials above are the grant check and not a broken fetch.
			if code, body := get("/granted"); code != 200 || body != netscopeSecret {
				t.Fatalf("/granted = %d %q", code, body)
			}
			// 4. The details stay in the server log.
			time.Sleep(100 * time.Millisecond)
			if !strings.Contains(log.String(), "CAPABILITY_DENIED") || !strings.Contains(log.String(), "network:fetch=169.254.169.254:80") {
				t.Fatalf("server log lacks the typed denial: %s", log.String())
			}
		})
	}
}

// A route cannot request a fetch grant the runner does not hold.
func TestIssue165RouteCannotEscalateFetch(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	for _, path := range []string{"bytecode", "hfir"} {
		t.Run(path, func(t *testing.T) {
			src := `(http_server 18790 (route "/x" (grants "network:fetch=169.254.169.254") (lambda (req) (res 200 "text/plain" "ran"))))`
			r := runAuthority(t, binary, path, src, "network,network:fetch=127.0.0.1")
			if r.code == -1 {
				t.Skipf("%s does not compile routes: %s", path, r.stdout)
			}
			requireDenied(t, path, r, "HTTP_ROUTE")
		})
	}
}

// Top-level fetch on the interpreter, AST bytecode and HFIR bytecode: the
// coarse network grant, a suffix grant, a different port, a metadata IP and an
// ungranted redirect hop are all denied before a request reaches the target.
func TestIssue165FetchHostScope(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	var targetHits, redirectHits atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHits.Add(1)
		io.WriteString(w, netscopeSecret)
	}))
	defer target.Close()
	targetHost := strings.TrimPrefix(target.URL, "http://")
	// "localhost" reaches the same listener under a different name, so a grant
	// for the IP must not cover it (and vice versa).
	targetByName := "http://localhost:" + targetHost[strings.LastIndex(targetHost, ":")+1:]
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectHits.Add(1)
		switch r.URL.Path {
		case "/to-target":
			http.Redirect(w, r, target.URL+"/hop", http.StatusFound)
		case "/to-meta":
			http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
		default:
			io.WriteString(w, "redirector-ok")
		}
	}))
	defer redirector.Close()
	redirectorHost := strings.TrimPrefix(redirector.URL, "http://")

	fetch := func(u string) string {
		return fmt.Sprintf(`(cli_app (print (bytes_to_string (fetch %q "GET"))))`, u)
	}
	cases := []struct {
		name, src, grant string
		allowed          bool
		targetHits       int64
	}{
		{"exact grant", fetch(target.URL + "/a"), "network:fetch=" + targetHost, true, 1},
		{"coarse network", fetch(target.URL + "/a"), "network", false, 0},
		{"no grant", fetch(target.URL + "/a"), "", false, 0},
		{"other port", fetch(target.URL + "/a"), "network:fetch=127.0.0.1:1", false, 0},
		{"default port only", fetch(target.URL + "/a"), "network:fetch=127.0.0.1", false, 0},
		{"name vs ip", fetch(targetByName + "/a"), "network:fetch=" + targetHost, false, 0},
		{"short ipv4", fetch("http://127.1:" + targetHost[strings.LastIndex(targetHost, ":")+1:] + "/a"), "network:fetch=" + targetHost, false, 0},
		{"userinfo trick", fetch("http://" + targetHost + "@169.254.169.254/latest/"), "network:fetch=" + targetHost, false, 0},
		{"metadata ip", fetch("http://169.254.169.254/latest/meta-data/"), "network:fetch=" + targetHost, false, 0},
		{"suffix host", fetch("http://evil-127.0.0.1.nip.invalid/"), "network:fetch=127.0.0.1", false, 0},
		{"redirect to ungranted host", fetch(redirector.URL + "/to-target"), "network:fetch=" + redirectorHost, false, 0},
		{"redirect to granted host", fetch(redirector.URL + "/to-target"), "network:fetch=" + redirectorHost + ",network:fetch=" + targetHost, true, 1},
		{"redirect to metadata", fetch(redirector.URL + "/to-meta"), "network:fetch=" + redirectorHost, false, 0},
	}
	for _, path := range authorityPaths() {
		for _, tc := range cases {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				before := targetHits.Load()
				r := runAuthority(t, binary, path, tc.src, tc.grant)
				hits := targetHits.Load() - before
				if hits != tc.targetHits {
					t.Fatalf("target hits = %d, want %d (%q %q)", hits, tc.targetHits, r.stdout, r.stderr)
				}
				if tc.allowed {
					if r.code != 0 || !strings.Contains(r.stdout, netscopeSecret) {
						t.Fatalf("allowed fetch failed: %d %q %q", r.code, r.stdout, r.stderr)
					}
					return
				}
				all := r.stdout + r.stderr
				if r.code == 0 || !strings.Contains(all, "CAPABILITY_DENIED") || strings.Contains(r.stdout, netscopeSecret) {
					t.Fatalf("want CAPABILITY_DENIED: %d %q %q", r.code, r.stdout, r.stderr)
				}
				if strings.Contains(all, "/latest/meta-data") || strings.Contains(all, "/hop") {
					t.Fatalf("denial leaked a URL path: %q", all)
				}
				if path == "interpreter" {
					return
				}
				if r.receipt == nil || r.receipt.Exit.Error == nil || r.receipt.Exit.Error.Code != "CAPABILITY_DENIED" || r.receipt.Exit.Error.Opcode != "FETCH" {
					t.Fatalf("receipt exit = %+v", r.receipt)
				}
				denied := ""
				for _, e := range r.receipt.Effects {
					if e.Op == "FETCH" && e.Capability == "network" && e.Decision == "denied" {
						denied = e.Grant
					}
				}
				if !strings.HasPrefix(denied, "network:fetch=") {
					t.Fatalf("receipt lacks a denied FETCH effect with a grant field: %+v", r.receipt.Effects)
				}
				if tc.name == "metadata ip" || tc.name == "redirect to metadata" || tc.name == "userinfo trick" {
					if denied != "network:fetch=169.254.169.254:80" {
						t.Fatalf("denied grant = %q, want network:fetch=169.254.169.254:80", denied)
					}
				}
			})
		}
	}
}

// #166: process:env= refuses dynamic-loader variables.
func TestIssue166EnvGrantRefusesLoaderVars(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	for _, name := range []string{"LD_PRELOAD", "LD_LIBRARY_PATH", "LD_AUDIT", "DYLD_INSERT_LIBRARIES", "ld_preload"} {
		r := runAuthority(t, binary, "bytecode", `(cli_app (print "ran"))`, "process:env="+name)
		if r.code == 0 || strings.Contains(r.stdout, "ran") {
			t.Fatalf("process:env=%s accepted: %d %q %q", name, r.code, r.stdout, r.stderr)
		}
	}
	// #177: HOME, PATH and GIT_* steer config or program lookup and are refused.
	for _, name := range []string{"HOME", "PATH", "GIT_DIR", "XDG_CONFIG_HOME", "JAVA_TOOL_OPTIONS"} {
		if r := runAuthority(t, binary, "bytecode", `(cli_app (print "ran"))`, "process:env="+name); r.code == 0 || strings.Contains(r.stdout, "ran") {
			t.Fatalf("process:env=%s accepted: %d %q %q", name, r.code, r.stdout, r.stderr)
		}
	}
	if r := runAuthority(t, binary, "bytecode", `(cli_app (print "ran"))`, "process:env=LANG"); r.code != 0 {
		t.Fatalf("process:env=LANG refused: %q %q", r.stdout, r.stderr)
	}
}

// Generated Go runs every handler with process-wide grants, so it cannot give
// a route its own (empty by default) fetch scope: it refuses outbound HTTP
// inside a route instead of building it.
func TestIssue165GoRouteFetchFailsClosed(t *testing.T) {
	bin := c1c2Binary(t)
	for _, src := range []string{
		`(http_server 18799 (route "/x" (lambda (req) (res 200 "text/plain" (bytes_to_string (fetch "http://127.0.0.1:18799/admin" "GET"))))))`,
		`(http_server 18799 (route "/x" (lambda (req) (let (r (http_request "http://169.254.169.254/" "GET")) (res 200 "text/plain" "x")))))`,
		`(http_server 18799 (middleware (lambda (req) (fetch "http://169.254.169.254/" "GET")) (route "/x" (lambda (req) (res 200 "text/plain" "x")))))`,
	} {
		dir := t.TempDir()
		p := writeTemp(t, dir, "r.howl", src)
		o := filepath.Join(dir, "out")
		out, err := exec.Command(bin, "build", "--target", "go", p, "-o", o).CombinedOutput()
		if err == nil || !strings.Contains(string(out), "UNSUPPORTED_ON_TARGET") {
			t.Errorf("route fetch built for Go: err=%v out=%s", err, out)
		}
		if _, statErr := os.Stat(o); statErr == nil {
			t.Errorf("route fetch wrote Go output")
		}
	}
	// Routes without outbound HTTP still build.
	dir := t.TempDir()
	p := writeTemp(t, dir, "ok.howl", `(http_server 18799 (route "/x" (lambda (req) (res 200 "text/plain" "x"))))`)
	if out, err := exec.Command(bin, "build", "--target", "go", p, "-o", filepath.Join(dir, "out")).CombinedOutput(); err != nil {
		t.Fatalf("plain route failed on Go: %v %s", err, out)
	}
}
