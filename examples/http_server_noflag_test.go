package receipt_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestHTTPServerNoFlagWritesBuildBytecode(t *testing.T) {
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	binaryPath := filepath.Join(t.TempDir(), "howlframe")
	build := exec.Command("go", "build", "-o", binaryPath, ".")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	source := filepath.Join(repoRoot, "examples", "hello.howl")
	toolDir := t.TempDir()
	invoked := filepath.Join(toolDir, "invoked")
	for _, name := range []string{"go", "node", "wat2wasm"} {
		body := "#!/bin/sh\necho \"$0\" >> " + invoked + "\nexit 99\n"
		if err := os.WriteFile(filepath.Join(toolDir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	workDir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binaryPath, args...)
		cmd.Dir = workDir
		cmd.Env = toolchainFreeEnv(t, toolDir)
		output, err := cmd.CombinedOutput()
		if ctx.Err() != nil {
			t.Fatalf("%v did not return before timeout: %v\n%s", args, ctx.Err(), output)
		}
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, output)
		}
		if _, err := os.Stat(invoked); !os.IsNotExist(err) {
			executed, _ := os.ReadFile(invoked)
			t.Fatalf("unexpected toolchain invocation: %s (stat: %v)", executed, err)
		}
		for _, name := range []string{"server.go", "server_test.go"} {
			if _, err := os.Stat(filepath.Join(workDir, name)); !os.IsNotExist(err) {
				t.Fatalf("unexpected %s: %v", name, err)
			}
		}
	}

	// The legacy invocation must write and return without executing HTTP_SERVER_SERVE.
	run(source)
	artifact := filepath.Join(workDir, "hello.hfbc")
	legacyBytes, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("read legacy artifact: %v", err)
	}
	if len(legacyBytes) == 0 {
		t.Fatal("hello.hfbc is empty")
	}
	if err := os.Remove(artifact); err != nil {
		t.Fatal(err)
	}
	run("build", source)
	buildBytes, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("read build artifact: %v", err)
	}
	if !bytes.Equal(legacyBytes, buildBytes) {
		t.Fatal("legacy artifact differs from build's default bytecode")
	}
}
