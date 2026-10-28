package echo_api_test

import (
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const echoAddr = "127.0.0.1:8106"

// failAfterStartEnv makes TestEchoAPI fail once the server answers, so
// TestEchoAPIStopsServerWhenTestFails can check cleanup on the failure path.
const failAfterStartEnv = "ECHO_API_TEST_FAIL_AFTER_START"

// startEchoServer compiles echo_api.howl and runs it until the test ends. The
// server runs in its own process group: `go run` starts the compiled howlframe
// as a child, so killing only the wrapper left that child listening on 8106.
func startEchoServer(t *testing.T) (stop func()) {
	t.Helper()
	// A server left over from another run would answer every request below.
	if conn, err := net.DialTimeout("tcp", echoAddr, 200*time.Millisecond); err == nil {
		conn.Close()
		t.Fatalf("%s is already in use; refusing to test against a server this test did not start", echoAddr)
	}

	artifact := filepath.Join(t.TempDir(), "echo_api.hfbc")
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "echo_api.howl", "-o", artifact)
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}

	srv := exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "network", artifact)
	srv.Dir = "."
	srv.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Wait() }()
	var once sync.Once
	stop = func() {
		once.Do(func() {
			syscall.Kill(-srv.Process.Pid, syscall.SIGKILL)
			<-errc
		})
	}
	t.Cleanup(stop)
	return stop
}

// waitPortReleased fails the test if something still accepts connections on
// echoAddr shortly after the server was stopped.
func waitPortReleased(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", echoAddr, 200*time.Millisecond)
		if err != nil {
			return
		}
		conn.Close()
		if time.Now().After(deadline) {
			t.Fatalf("%s still accepts connections after the server was stopped", echoAddr)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestEchoAPI(t *testing.T) {
	stop := startEchoServer(t)

	deadline := time.Now().Add(3 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + echoAddr + "/health")
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
	if os.Getenv(failAfterStartEnv) != "" {
		t.Fatal("forced failure after the server started")
	}

	resp, err := http.Get("http://" + echoAddr + "/echo?q=hi")
	if err != nil {
		t.Fatalf("echo: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	text := string(body)
	if !strings.Contains(text, "GET") || !strings.Contains(text, "hi") {
		t.Fatalf("echo body: %s", text)
	}

	resp, err = http.Get("http://" + echoAddr + "/echo/ada")
	if err != nil {
		t.Fatalf("echo name: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "ada") {
		t.Fatalf("echo name body: %s", body)
	}

	stop()
	waitPortReleased(t)
}

// TestEchoAPIStopsServerWhenTestFails runs TestEchoAPI in a child test process
// that fails as soon as the server answers, then checks that the port is free:
// the server must be stopped on the failure path too.
func TestEchoAPIStopsServerWhenTestFails(t *testing.T) {
	if os.Getenv(failAfterStartEnv) != "" {
		t.Skip("running as the child of this test")
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestEchoAPI$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), failAfterStartEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "forced failure after the server started") {
		t.Fatalf("child TestEchoAPI should fail after starting the server; err=%v\n%s", err, out)
	}
	waitPortReleased(t)
}
