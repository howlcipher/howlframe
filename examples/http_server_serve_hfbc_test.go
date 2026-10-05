package receipt_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
)

func TestHTTPServerServeHFBC(t *testing.T) {
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "howlframe")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	source := filepath.Join(repoRoot, "examples", "hello.howl")
	toolDir := t.TempDir()
	invoked := filepath.Join(toolDir, "invoked")
	for _, name := range []string{"go", "node", "wat2wasm"} {
		body := "#!/bin/sh\necho \"$0\" >> \"" + invoked + "\"\nexit 99\n"
		if err := os.WriteFile(filepath.Join(toolDir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	workDir, outputDir := t.TempDir(), t.TempDir()
	env := toolchainFreeEnv(t, toolDir)
	assertNotListening := func() {
		t.Helper()
		conn, err := net.DialTimeout("tcp", "127.0.0.1:8080", 200*time.Millisecond)
		if err == nil {
			conn.Close()
			t.Fatal("unexpected listener on port 8080")
		}
	}
	assertNotListening()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	compile := exec.CommandContext(ctx, binary, source, "-o", outputDir)
	compile.Dir, compile.Env = workDir, env
	output, err := compile.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("compile-only invocation did not return: %v\n%s", ctx.Err(), output)
	}
	if err != nil {
		t.Fatalf("compile-only invocation: %v\n%s", err, output)
	}
	assertNotListening()
	artifact := filepath.Join(outputDir, "hello.hfbc")
	if data, err := os.ReadFile(artifact); err != nil || len(data) == 0 {
		t.Fatalf("read nonempty artifact: %v", err)
	}
	for _, dir := range []string{workDir, outputDir} {
		for _, name := range []string{"server.go", "server_test.go"} {
			if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
				t.Fatalf("unexpected %s in %s: %v", name, dir, err)
			}
		}
	}
	serve := func(args ...string) map[string]string {
		t.Helper()
		cmd := exec.Command(binary, args...)
		cmd.Dir, cmd.Env = workDir, env
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		var log bytes.Buffer
		cmd.Stdout, cmd.Stderr = &log, &log
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{})
		var waitErr error
		go func() { waitErr = cmd.Wait(); close(done) }()
		defer func() { syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); <-done }()
		client := &http.Client{Timeout: 500 * time.Millisecond}
		defer client.CloseIdleConnections()
		bodies := make(map[string]string)
		deadline := time.Now().Add(5 * time.Second)
		for {
			select {
			case <-done:
				t.Fatalf("server exited: %v\n%s", waitErr, log.String())
			default:
			}
			resp, err := client.Get("http://127.0.0.1:8080/")
			if err == nil {
				data, readErr := io.ReadAll(resp.Body)
				resp.Body.Close()
				if readErr != nil {
					t.Fatal(readErr)
				}
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("/ status: %d", resp.StatusCode)
				}
				bodies["/"] = string(data)
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("server did not listen: %v", err)
			}
			time.Sleep(25 * time.Millisecond)
		}
		resp, err := client.Get("http://127.0.0.1:8080/json")
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("/json status: %d", resp.StatusCode)
		}
		bodies["/json"] = string(data)
		return bodies
	}
	artifactBodies := serve("-run-bc", "-allow-caps", "network", artifact)
	assertNotListening()
	sourceBodies := serve("run", "--allow-caps", "network", source)
	assertNotListening()
	for _, bodies := range []map[string]string{artifactBodies, sourceBodies} {
		if bodies["/"] != "Hello, World! HowlFrame language is alive!" {
			t.Fatalf("unexpected / body: %q", bodies["/"])
		}
		var decoded map[string]string
		if err := json.Unmarshal([]byte(bodies["/json"]), &decoded); err != nil {
			t.Fatal(err)
		}
		expected := map[string]string{"status": "success", "message": "Hello from HowlFrame JSON endpoint!"}
		if !reflect.DeepEqual(decoded, expected) {
			t.Fatalf("unexpected /json body: %v", decoded)
		}
	}
	if artifactBodies["/"] != sourceBodies["/"] {
		t.Fatal("artifact and source / bodies differ")
	}
	var artifactJSON, sourceJSON any
	if err := json.Unmarshal([]byte(artifactBodies["/json"]), &artifactJSON); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(sourceBodies["/json"]), &sourceJSON); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(artifactJSON, sourceJSON) {
		t.Fatal("artifact and source JSON bodies differ")
	}
	if _, err := os.Stat(invoked); !os.IsNotExist(err) {
		executed, _ := os.ReadFile(invoked)
		t.Fatalf("unexpected toolchain invocation: %s (stat: %v)", executed, err)
	}
}
