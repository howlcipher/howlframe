package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Milestone 2: --max-request-bytes bounds a route's req.body read. An oversized
// body raises a catchable LIMIT_EXCEEDED so the application can answer 413.
func TestMaxRequestBytesIsCatchable(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	port := freeTCPPort(t)
	src := fmt.Sprintf(`(http_server %d
  (route "/echo" (lambda (req)
    (try_let (body (parse_json Any req.body))
      (catch err (if (regex_match "LIMIT_EXCEEDED" err) (res_json 413 (dict ("error" "too_large"))) (res_json 400 (dict ("error" "bad_json")))))
      (res_json 200 (dict ("ok" "yes")))))))`, port)
	artifact := buildHFBCArtifact(t, binary, t.TempDir(), "srv", src)
	log, _ := startHFBCServerCmd(t, binary, artifact, "network", "--max-request-bytes", "64")
	post := func(body string) (int, string) {
		return httpDoStatusBody(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/echo", port), "application/json", body)
	}
	code, out := pollHTTPDoStatusBody(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/echo", port), "application/json", `{"a":1}`)
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
