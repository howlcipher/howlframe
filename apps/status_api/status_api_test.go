package status_api_test

import (
	"io"
	"net/http"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestStatusAPI(t *testing.T) {
	// Compile
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "status_api.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	// Start server
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "network,environment", "status_api.howl.bc.bin")
	cmd.Dir = "."
	cmd.Env = append(cmd.Environ(), "REQUIRED_ENV=yes", "PUBLIC_CONFIG=foo")

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	errPipe, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	errc := make(chan error, 1)
	go func() {
		errc <- cmd.Wait()
	}()

	defer func() {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-errc
	}()

	// Wait for start, but fail fast if it exits
	timeout := time.After(2 * time.Second)
	select {
	case err := <-errc:
		t.Fatalf("server process exited prematurely: %v", err)
	case <-timeout:
	}

	// Test endpoints
	resp, err := http.Get("http://localhost:8080/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), `"status":"ok"`) {
		t.Errorf("health unexpected: %s", b)
	}

	resp, err = http.Get("http://localhost:8080/ready")
	if err != nil {
		t.Fatalf("ready request failed: %v", err)
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), `"status":"ready"`) {
		t.Errorf("ready unexpected: %s", b)
	}

	resp, err = http.Get("http://localhost:8080/config")
	if err != nil {
		t.Fatalf("config request failed: %v", err)
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), `"public_config":"foo"`) {
		t.Errorf("config unexpected: %s", b)
	}

	// Test missing network
	cmdFail := exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "environment", "status_api.howl.bc.bin")
	cmdFail.Dir = "."
	out, _ := cmdFail.CombinedOutput()
	if !strings.Contains(string(out), "capability denied: network") {
		t.Errorf("expected network denied, got: %s", out)
	}

	_ = errPipe
}
