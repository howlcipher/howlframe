package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// --timing-log adds a separate diagnostic "timing" line per request. The
// frozen access-line format must not change, and timing is opt-in.
func TestTimingLogIsSeparateAndOptIn(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	run := func(t *testing.T, flags ...string) string {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := ln.Addr().(*net.TCPAddr).Port
		ln.Close()
		file := filepath.Join(dir, fmt.Sprintf("s%d.howl", port))
		os.WriteFile(file, []byte(fmt.Sprintf(`(http_server %d (route "/a" (lambda (req) (res_json 200 (dict ("ok" "1"))))))`, port)), 0o600)
		artifact := file + ".hfbc"
		if out, err := exec.Command(binary, "build", file, "-o", artifact).CombinedOutput(); err != nil {
			t.Fatalf("build: %v %s", err, out)
		}
		logPath := file + ".log"
		logFile, err := os.Create(logPath)
		if err != nil {
			t.Fatal(err)
		}
		defer logFile.Close()
		args := append([]string{"run", "--allow-caps", "network"}, flags...)
		cmd := exec.Command(binary, append(args, artifact)...)
		cmd.Stdout, cmd.Stderr = logFile, logFile
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		defer func() { cmd.Process.Kill(); cmd.Wait() }()
		served := 0
		for i := 0; i < 100 && served < 2; i++ {
			if resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/a?q=1", port)); err == nil {
				resp.Body.Close()
				served++
				continue
			}
			time.Sleep(50 * time.Millisecond)
		}
		if served < 2 {
			t.Fatalf("server did not answer")
		}
		time.Sleep(100 * time.Millisecond)
		out, _ := os.ReadFile(logPath)
		return string(out)
	}

	t.Run("on", func(t *testing.T) {
		logs := run(t, "--access-log", "--timing-log")
		access := regexp.MustCompile(`^access method=([A-Z]+) path=(\S+) status=([0-9]{3}) duration_ms=([0-9]+) request_id=([0-9a-f]{32})$`)
		timing := regexp.MustCompile(`^timing request_id=([0-9a-f]{32}) method=GET path=/a start_unix_ns=(\d+) start_mono_ms=(\d+\.\d{3}) locked_mono_ms=(\d+\.\d{3}) end_mono_ms=(\d+\.\d{3}) wait_ms=(\d+\.\d{3}) handler_ms=(\d+\.\d{3}) total_ms=(\d+\.\d{3})$`)
		accessIDs, timingIDs := map[string]bool{}, map[string]bool{}
		for _, ln := range strings.Split(strings.TrimSpace(logs), "\n") {
			if strings.HasPrefix(ln, "access ") {
				m := access.FindStringSubmatch(ln)
				if m == nil {
					t.Fatalf("access line format changed: %q", ln)
				}
				accessIDs[m[5]] = true
			}
			if strings.HasPrefix(ln, "timing ") {
				m := timing.FindStringSubmatch(ln)
				if m == nil {
					t.Fatalf("bad timing line: %q", ln)
				}
				f := func(i int) float64 { v, _ := strconv.ParseFloat(m[i], 64); return v }
				if !(f(3) <= f(4) && f(4) <= f(5)) {
					t.Fatalf("monotonic marks out of order: %q", ln)
				}
				timingIDs[m[1]] = true
			}
		}
		if len(timingIDs) < 2 || len(accessIDs) < 2 {
			t.Fatalf("want >=2 access and timing lines:\n%s", logs)
		}
		for id := range timingIDs {
			if !accessIDs[id] {
				t.Fatalf("timing id %s has no access line:\n%s", id, logs)
			}
		}
	})
	t.Run("off", func(t *testing.T) {
		if logs := run(t, "--access-log"); strings.Contains(logs, "timing ") || strings.Contains(logs, "store_timing ") {
			t.Fatalf("timing lines must require --timing-log:\n%s", logs)
		}
	})
}
