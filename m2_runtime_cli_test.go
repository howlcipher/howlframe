package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func startM2Server(t *testing.T, src string, env []string, grants string, flags ...string) (*bytes.Buffer, func()) {
	t.Helper()
	binary := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "srv.howl")
	os.WriteFile(file, []byte(src), 0o600)
	artifact := filepath.Join(dir, "srv.hfbc")
	if out, err := exec.Command(binary, "build", file, "-o", artifact).CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	args := append([]string{"run", "--allow-caps", grants}, flags...)
	args = append(args, artifact)
	cmd := exec.Command(binary, args...)
	cmd.Env = append(os.Environ(), env...)
	log := &bytes.Buffer{}
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stop := func() { cmd.Process.Kill(); cmd.Wait() }
	t.Cleanup(stop)
	return log, stop
}

func waitHTTP(t *testing.T, url string, log *bytes.Buffer) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if resp, err := http.Get(url); err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server not ready at %s\nlog: %s", url, log)
}

// Milestone 2 runtime support in one server: the port comes from the
// environment, "*" is a JSON fallback route, req_body/utf8_valid/req_url_path
// and granted work, and (serialize_requests) makes read-modify-write safe.
func TestHTTPServerM2Features(t *testing.T) {
	port := freeTCPPort(t)
	src := `(http_server (to_int (env "M2_PORT"))
  (serialize_requests)
  (grants "database" "inherit:filesystem")
  (store_open kv "memory://counter")
  (store_put kv "n" (dict ("v" 0)))
  (store_put kv "g" (dict ("net" (granted "network")) ("proc" (granted "process"))))
  (route "/count" (lambda (req)
    (let (cur (store_get kv "n"))
      (do
        (sleep 5)
        (store_put kv "n" (dict ("v" (+ (map_get cur "v") 1))))
        (res_json 200 (dict ("v" (map_get cur "v"))))))))
  (route "/info/{id}" (lambda (req)
    (let (body (req_body req))
      (res_json 200 (dict ("path" (req_url_path req)) ("id" (req_path req "id")) ("len" (str_len body))
                          ("utf8" (utf8_valid body)) ("net" (map_get (store_get kv "g") "net")) ("proc" (map_get (store_get kv "g") "proc")))))))
  (route "*" (lambda (req)
    (res_json 404 (dict ("error" "no_route") ("path" (req_url_path req)))))))`
	log, _ := startM2Server(t, src, []string{fmt.Sprintf("M2_PORT=%d", port)}, "network,database,environment", "--max-request-bytes", "32")
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitHTTP(t, base+"/info/x", log)

	get := func(method, path, body string) (int, string) {
		req, _ := http.NewRequest(method, base+path, strings.NewReader(body))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	if code, out := get("POST", "/info/abc?q=1", "h\xc3\xa9llo"); code != 200 || !strings.Contains(out, `"id":"abc"`) || !strings.Contains(out, `"path":"/info/abc"`) ||
		!strings.Contains(out, `"utf8":true`) || !strings.Contains(out, `"net":true`) || !strings.Contains(out, `"proc":false`) {
		t.Fatalf("info: %d %s", code, out)
	}
	if _, out := get("POST", "/info/abc", "h\xffllo"); !strings.Contains(out, `"utf8":false`) {
		t.Fatalf("invalid utf8 not detected: %s", out)
	}
	if code, out := get("GET", "/v1/nope/", ""); code != 404 || !strings.Contains(out, "no_route") || !strings.Contains(out, "/v1/nope/") {
		t.Fatalf("fallback: %d %s", code, out)
	}
	// The path is reported percent-encoded as sent, so a log line can never be split by it.
	if code, out := get("GET", "/odd%20x%0Ay", ""); code != 404 || !strings.Contains(out, `/odd%20x%0Ay`) {
		t.Fatalf("encoded path: %d %s", code, out)
	}
	if code, out := get("PUT", "/info/abc", strings.Repeat("x", 100)); code == 200 {
		t.Fatalf("oversize body must raise LIMIT_EXCEEDED in the handler (500), got %d %s", code, out)
	}
	// serialize_requests: 20 concurrent read-modify-write requests must not lose updates.
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); get("GET", "/count", "") }()
	}
	wg.Wait()
	if _, out := get("GET", "/count", ""); !strings.Contains(out, `"v":20`) {
		t.Fatalf("lost updates under concurrency: %s", out)
	}
}

func TestHTTPServerPortMustBeValid(t *testing.T) {
	log, stop := startM2Server(t, `(http_server (to_int (env "M2_PORT")) (route "/a" (lambda (req) (res_json 200 (dict ("ok" "1"))))))`,
		[]string{"M2_PORT=70000"}, "network,environment")
	time.Sleep(600 * time.Millisecond)
	stop()
	if !strings.Contains(log.String(), "http_server port must be an integer 1..65535") {
		t.Fatalf("log: %s", log)
	}
}
