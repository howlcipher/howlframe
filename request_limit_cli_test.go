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
	"testing"
	"time"
)

// Milestone 2: --max-request-bytes bounds a route's req.body read. An oversized
// body raises a catchable LIMIT_EXCEEDED so the application can answer 413.
func TestMaxRequestBytesIsCatchable(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	src := fmt.Sprintf(`(http_server %d
  (route "/echo" (lambda (req)
    (try_let (body (parse_json Any req.body))
      (catch err (if (regex_match "LIMIT_EXCEEDED" err) (res_json 413 (dict ("error" "too_large"))) (res_json 400 (dict ("error" "bad_json")))))
      (res_json 200 (dict ("ok" "yes")))))))`, port)
	dir := t.TempDir()
	file := filepath.Join(dir, "srv.howl")
	if err := os.WriteFile(file, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(dir, "srv.hfbc")
	if out, err := exec.Command(binary, "build", file, "-o", artifact).CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	cmd := exec.Command(binary, "run", "--allow-caps", "network", "--max-request-bytes", "64", artifact)
	log := &bytes.Buffer{}
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	post := func(body string) (int, string) {
		resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/echo", port), "application/json", strings.NewReader(body))
		if err != nil {
			return 0, err.Error()
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	var code int
	var out string
	for i := 0; i < 100; i++ {
		code, out = post(`{"a":1}`)
		if code != 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if code != 200 {
		t.Fatalf("small body: %d %s\nlog: %s", code, out, log)
	}
	big := `{"a":"` + strings.Repeat("x", 200) + `"}`
	if code, out = post(big); code != 413 || !strings.Contains(out, "too_large") {
		t.Fatalf("oversize body: %d %s\nlog: %s", code, out, log)
	}
	if code, out = post(`{bad`); code != 400 {
		t.Fatalf("malformed body: %d %s", code, out)
	}
}

func TestMaxRequestBytesRejectsNonPositive(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "a.howl")
	os.WriteFile(file, []byte(`(cli_app (print 1))`), 0o600)
	artifact := filepath.Join(dir, "a.hfbc")
	if out, err := exec.Command(binary, "build", file, "-o", artifact).CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	out, err := exec.Command(binary, "run", "--max-request-bytes", "0", artifact).CombinedOutput()
	if err == nil || !strings.Contains(string(out), "max-request-bytes") {
		t.Fatalf("want rejection of zero, got err=%v out=%s", err, out)
	}
}
