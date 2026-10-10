package rest_tasks_test

import (
	"bytes"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRestTasks(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "rest_tasks.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "network,database", "rest_tasks.howl.bc.bin")
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
		resp, err := http.Get("http://127.0.0.1:8097/tasks")
		if err == nil {
			resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server not up: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	resp, err := http.Post("http://127.0.0.1:8097/tasks", "application/json", bytes.NewBufferString(`{"title":"Ship"}`))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 201 || !strings.Contains(string(b), `"title":"Ship"`) {
		t.Fatalf("create unexpected %d %s", resp.StatusCode, b)
	}

	resp, err = http.Get("http://127.0.0.1:8097/tasks")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(b), `"tasks"`) || !strings.Contains(string(b), "Ship") {
		t.Fatalf("list unexpected %d %s", resp.StatusCode, b)
	}

	resp, err = http.Get("http://127.0.0.1:8097/tasks/1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(b), `"id":"1"`) {
		t.Fatalf("get unexpected %d %s", resp.StatusCode, b)
	}

	req, _ := http.NewRequest(http.MethodPatch, "http://127.0.0.1:8097/tasks/1", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(b), `"status":"done"`) {
		t.Fatalf("patch unexpected %d %s", resp.StatusCode, b)
	}

	req, _ = http.NewRequest(http.MethodDelete, "http://127.0.0.1:8097/tasks/1", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(b), `"deleted":"1"`) {
		t.Fatalf("delete unexpected %d %s", resp.StatusCode, b)
	}

	resp, err = http.Get("http://127.0.0.1:8097/tasks/1")
	if err != nil {
		t.Fatalf("get missing: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404 after delete, got %d", resp.StatusCode)
	}
}
