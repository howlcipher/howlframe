package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Milestone 2 (server observability): every response carries a server-assigned
// X-Request-Id, (req_id req) exposes it to the handler, --access-log writes one
// line per request, and a faulting handler logs the same id next to its error.
func TestRequestIDAccessLogAndFaultLine(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	src := fmt.Sprintf(`(http_server %d
  (route "/id" (lambda (req) (res_json 200 (dict ("id" (req_id req))))))
  (route "/boom" (lambda (req) (res_json 200 (dict ("n" (to_int "not-a-number")))))))`, port)
	dir := t.TempDir()
	file := filepath.Join(dir, "srv.howl")
	os.WriteFile(file, []byte(src), 0o600)
	artifact := filepath.Join(dir, "srv.hfbc")
	if out, err := exec.Command(binary, "build", file, "-o", artifact).CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	cmd := exec.Command(binary, "run", "--allow-caps", "network", "--access-log", artifact)
	log := &syncBuffer{}
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	get := func(path string) (*http.Response, string) {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, path))
		if err != nil {
			return nil, err.Error()
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp, string(b)
	}
	var resp *http.Response
	var body string
	for i := 0; i < 100; i++ {
		if resp, body = get("/id?x=1"); resp != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if resp == nil || resp.StatusCode != 200 {
		t.Fatalf("/id: %v %s\nlog: %s", resp, body, log)
	}
	rid := resp.Header.Get("X-Request-Id")
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(rid) || !strings.Contains(body, rid) {
		t.Fatalf("request id header %q body %q", rid, body)
	}
	resp2, _ := get("/id")
	if resp2.Header.Get("X-Request-Id") == rid {
		t.Fatalf("request ids must be unique")
	}
	bad, badBody := get("/boom")
	brid := bad.Header.Get("X-Request-Id")
	if bad.StatusCode != 500 || !strings.Contains(badBody, brid) || strings.Contains(badBody, "to_int") {
		t.Fatalf("/boom: %d %q (id %q)", bad.StatusCode, badBody, brid)
	}
	time.Sleep(100 * time.Millisecond)
	logs := log.String()
	access := regexp.MustCompile(`access method=GET path=/id status=200 duration_ms=\d+ request_id=` + rid + `\n`)
	if !access.MatchString(logs) {
		t.Fatalf("missing access line for /id (query string must not be logged):\n%s", logs)
	}
	if !strings.Contains(logs, "access method=GET path=/boom status=500 ") || !strings.Contains(logs, "fault request_id="+brid+" method=GET path=/boom error=") {
		t.Fatalf("missing fault/access lines for /boom:\n%s", logs)
	}
}

func TestAccessLogIsOptIn(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	dir := t.TempDir()
	file := filepath.Join(dir, "s.howl")
	os.WriteFile(file, []byte(fmt.Sprintf(`(http_server %d (route "/a" (lambda (req) (res_json 200 (dict ("ok" "1"))))))`, port)), 0o600)
	artifact := filepath.Join(dir, "s.hfbc")
	if out, err := exec.Command(binary, "build", file, "-o", artifact).CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	cmd := exec.Command(binary, "run", "--allow-caps", "network", artifact)
	log := &syncBuffer{}
	cmd.Stdout, cmd.Stderr = log, log
	cmd.Start()
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	for i := 0; i < 100; i++ {
		if resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/a", port)); err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if strings.Contains(log.String(), "access ") {
		t.Fatalf("access lines must require --access-log:\n%s", log)
	}
}
