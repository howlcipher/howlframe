package stamp_api_test

import (
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestStampAPI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "stamp_api.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "network", "stamp_api.howl.bc.bin")
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
		resp, err := http.Get("http://127.0.0.1:8104/health")
		if err == nil {
			resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server not up: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	resp, err := http.Get("http://127.0.0.1:8104/now")
	if err != nil {
		t.Fatalf("/now: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	body := string(b)
	if resp.StatusCode != 200 || !strings.Contains(body, `"unix"`) {
		t.Fatalf("want unix, got %d %s", resp.StatusCode, body)
	}
	start := strings.Index(body, `"unix":"`)
	if start < 0 {
		t.Fatalf("no unix field: %s", body)
	}
	rest := body[start+len(`"unix":"`):]
	end := strings.Index(rest, `"`)
	n, err := strconv.ParseInt(rest[:end], 10, 64)
	if err != nil || n < 1700000000 {
		t.Fatalf("bad unix %q in %s", rest[:end], body)
	}

	resp, err = http.Get("http://127.0.0.1:8104/echo/ada")
	if err != nil {
		t.Fatalf("echo: %v", err)
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), `"name":"ada"`) {
		t.Fatalf("echo: %s", b)
	}
}
