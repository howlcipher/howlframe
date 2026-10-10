package counter_api_test

import (
	"io"
	"net/http"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCounterAPI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "counter_api.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "network,database", "counter_api.howl.bc.bin")
	cmd.Dir = "."
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	errc := make(chan error, 1)
	go func() { errc <- cmd.Wait() }()
	defer func() {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-errc
	}()

	deadline := time.Now().Add(3 * time.Second)
	for {
		resp, err := http.Get("http://127.0.0.1:8103/count")
		if err == nil {
			resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server not up: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	resp, err := http.Get("http://127.0.0.1:8103/count")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(b), `"n":"0"`) {
		t.Fatalf("want n=0, got %d %s", resp.StatusCode, b)
	}

	resp, err = http.Post("http://127.0.0.1:8103/inc", "application/json", nil)
	if err != nil {
		t.Fatalf("inc: %v", err)
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(b), `"n":"1"`) {
		t.Fatalf("want n=1, got %d %s", resp.StatusCode, b)
	}

	resp, err = http.Post("http://127.0.0.1:8103/inc", "application/json", nil)
	if err != nil {
		t.Fatalf("inc2: %v", err)
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), `"n":"2"`) {
		t.Fatalf("want n=2, got %s", b)
	}

	resp, err = http.Post("http://127.0.0.1:8103/reset", "application/json", nil)
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), `"n":"0"`) {
		t.Fatalf("want reset 0, got %s", b)
	}
}
