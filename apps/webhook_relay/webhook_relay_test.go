package webhook_relay_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestWebhookRelay(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Method == http.MethodGet {
			w.WriteHeader(201)
			w.Write([]byte(`{"up":"get"}`))
			return
		}
		w.WriteHeader(202)
		w.Write(append([]byte(`echo:`), body...))
	}))
	defer upstream.Close()

	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "webhook_relay.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "network,environment", "webhook_relay.howl.bc.bin")
	cmd.Dir = "."
	cmd.Env = append(cmd.Environ(), "UPSTREAM_URL="+upstream.URL+"/")
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
		resp, err := http.Get("http://127.0.0.1:8098/relay")
		if err == nil {
			resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("relay not up: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	resp, err := http.Get("http://127.0.0.1:8098/relay")
	if err != nil {
		t.Fatalf("GET relay: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 201 || !strings.Contains(string(b), `"up":"get"`) {
		t.Fatalf("GET relay unexpected %d %s", resp.StatusCode, b)
	}

	resp, err = http.Post("http://127.0.0.1:8098/relay", "application/json", bytes.NewBufferString(`{"x":1}`))
	if err != nil {
		t.Fatalf("POST relay: %v", err)
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 202 || !strings.Contains(string(b), `echo:{"x":1}`) {
		t.Fatalf("POST relay unexpected %d %s", resp.StatusCode, b)
	}
}
