package main

import (
	"bufio"
	"encoding/json"
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

// A runner can narrow the listener to loopback. A serial server must still
// correlate and log a request rejected before entering the application's VM.
func TestLoopbackListenerAndTruncatedUpload(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	dir := t.TempDir()
	source, artifact := filepath.Join(dir, "server.howl"), filepath.Join(dir, "server.hfbc")
	program := fmt.Sprintf(`(http_server %d (serialize_requests)
  (route "/" (lambda (req) (let (body (req_body req)) (res_json 200 (dict ("status" "ok")))))))`, port)
	if err := os.WriteFile(source, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(binary, "build", source, "-o", artifact).CombinedOutput(); err != nil {
		t.Fatalf("compile: %v: %s", err, out)
	}
	for _, host := range []string{"localhost", "127.0.0.1:8080", "not-an-address"} {
		out, err := exec.Command(binary, "run", "--listen-host", host, artifact).CombinedOutput()
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 2 || !strings.Contains(string(out), "IP literal") {
			t.Fatalf("invalid host %q: %v: %s", host, err, out)
		}
	}
	logPath := filepath.Join(dir, "server.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	cmd := exec.Command(binary, "run", "--allow-caps", "network", "--listen-host", "127.0.0.1", "--access-log", artifact)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	address := fmt.Sprintf("127.0.0.1:%d", port)
	ready := false
	for attempt := 0; attempt < 100; attempt++ {
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			ready = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		t.Fatal("loopback listener not ready")
	}
	// 127.0.0.2 also belongs to loopback, but must not be included by a
	// listener restricted to exactly 127.0.0.1.
	if conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.2:%d", port), time.Second); err == nil {
		conn.Close()
		t.Fatal("listener bound beyond the requested IP")
	}
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, err = io.WriteString(conn, "POST / HTTP/1.1\r\nHost: local\r\nContent-Length: 100\r\nConnection: close\r\n\r\n{}")
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	id := response.Header.Get("X-Request-Id")
	if response.StatusCode != 400 || body.Error.Code != "invalid_json" || body.Error.Message == "" ||
		!regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(id) || body.Error.RequestID != id {
		t.Fatalf("uncorrelated or unsafe rejection: status=%d id=%q body=%+v", response.StatusCode, id, body)
	}
	for attempt := 0; attempt < 100; attempt++ {
		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "access method=POST path=/ status=400 ") && strings.Contains(string(data), "request_id="+id) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("pre-handler rejection was omitted from access log")
}
