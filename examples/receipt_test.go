package receipt_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReceiptNoFlagBytecode(t *testing.T) {
	repoRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	// This file lives in examples/, so the module root is the parent.
	repoRoot = filepath.Dir(repoRoot)

	binaryPath := filepath.Join(t.TempDir(), "howlframe")
	build := exec.Command("go", "build", "-o", binaryPath, ".")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	source := filepath.Join(repoRoot, "examples", "receipt.howl")
	toolDir := t.TempDir()
	for _, name := range []string{"go", "node", "wat2wasm"} {
		path := filepath.Join(toolDir, name)
		body := "#!/bin/sh\necho \"$0\" >> " + filepath.Join(toolDir, "invoked") + "\nexit 99\n"
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	run := func(t *testing.T, stdin string) (stdout, stderr string, exit int) {
		t.Helper()
		workDir := t.TempDir()
		cmd := exec.Command(binaryPath, source)
		cmd.Dir = workDir
		cmd.Stdin = strings.NewReader(stdin)
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		cmd.Env = toolchainFreeEnv(t, toolDir)
		err := cmd.Run()
		if err != nil {
			exitErr, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatalf("howlframe examples/receipt.howl: %v\nstderr: %s", err, errBuf.String())
			}
			exit = exitErr.ExitCode()
		}
		if _, statErr := os.Stat(filepath.Join(toolDir, "invoked")); statErr == nil {
			invoked, _ := os.ReadFile(filepath.Join(toolDir, "invoked"))
			t.Fatalf("transpiler toolchain was invoked: %s", invoked)
		}
		artifact := filepath.Join(workDir, "receipt.hfbc")
		info, statErr := os.Stat(artifact)
		if statErr != nil {
			t.Fatalf("receipt.hfbc missing: %v\nexit %d\nstdout %q\nstderr %q", statErr, exit, outBuf.String(), errBuf.String())
		}
		if info.Size() == 0 {
			t.Fatal("receipt.hfbc is empty")
		}
		if _, statErr := os.Stat(filepath.Join(workDir, "server.go")); !os.IsNotExist(statErr) {
			t.Fatalf("server.go was written: %v", statErr)
		}
		if _, statErr := os.Stat(filepath.Join(workDir, "server_test.go")); !os.IsNotExist(statErr) {
			t.Fatalf("server_test.go was written: %v", statErr)
		}
		return outBuf.String(), errBuf.String(), exit
	}

	t.Run("totals the receipt the source describes", func(t *testing.T) {
		stdout, stderr, exit := run(t, "apples 2 150\nbread 1 325\nmilk 1 007\n")
		if exit != 0 {
			t.Fatalf("exit %d, stderr %q", exit, stderr)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}
		want := "apples 300\nbread 325\nmilk 7\nlines: 3\nunits: 4\ntotal_cents: 632\n"
		if stdout != want {
			t.Fatalf("stdout = %q, want %q", stdout, want)
		}
	})

	t.Run("empty stdin is an empty receipt", func(t *testing.T) {
		stdout, stderr, exit := run(t, "")
		if exit != 0 || stderr != "" {
			t.Fatalf("exit %d, stderr %q, stdout %q", exit, stderr, stdout)
		}
		want := "lines: 0\nunits: 0\ntotal_cents: 0\n"
		if stdout != want {
			t.Fatalf("stdout = %q, want %q", stdout, want)
		}
	})

	t.Run("malformed line exits 2 before the summary", func(t *testing.T) {
		stdout, stderr, exit := run(t, "apples 2 150\nbread 1\n")
		if exit != 2 {
			t.Fatalf("exit %d, stdout %q, stderr %q", exit, stdout, stderr)
		}
		if stdout != "apples 300\n" {
			t.Fatalf("stdout = %q, want the accepted line only", stdout)
		}
		if stderr != "bad line: bread 1\n" {
			t.Fatalf("stderr = %q", stderr)
		}
	})
}

func toolchainFreeEnv(t *testing.T, toolDir string) []string {
	t.Helper()
	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "PATH=") || strings.HasPrefix(entry, "GOROOT=") || strings.HasPrefix(entry, "GOPATH=") {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, "PATH="+toolDir)
	return env
}
