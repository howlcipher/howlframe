package echo_api_test

import (
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestEchoAPI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "echo_api.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	srv := exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "network",
		"echo_api.howl.bc.bin")
	srv.Dir = "."
	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() {
		_ = srv.Process.Kill()
		_, _ = srv.Process.Wait()
	}()

	deadline := time.Now().Add(3 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://127.0.0.1:8106/health")
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if strings.Contains(string(body), "ok") {
				last = nil
				break
			}
			last = err
		} else {
			last = err
		}
		time.Sleep(50 * time.Millisecond)
	}
	if last != nil {
		t.Fatalf("health: %v", last)
	}

	resp, err := http.Get("http://127.0.0.1:8106/echo?q=hi")
	if err != nil {
		t.Fatalf("echo: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	text := string(body)
	if !strings.Contains(text, "GET") || !strings.Contains(text, "hi") {
		t.Fatalf("echo body: %s", text)
	}

	resp, err = http.Get("http://127.0.0.1:8106/echo/ada")
	if err != nil {
		t.Fatalf("echo name: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "ada") {
		t.Fatalf("echo name body: %s", body)
	}
}
