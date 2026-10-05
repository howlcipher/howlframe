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

func TestWasmAppRunHFBC(t *testing.T) {
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
	source := filepath.Join(repoRoot, "examples", "wasm_app_hello.howl")
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
	run := func(t *testing.T, args ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir, cmd.Env = workDir, env
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		if ctx.Err() != nil {
			t.Fatalf("%v did not return: %v\nstdout: %s\nstderr: %s", args, ctx.Err(), &stdout, &stderr)
		}
		if err != nil {
			t.Fatalf("%v failed: %v\nstdout: %s\nstderr: %s", args, err, &stdout, &stderr)
		}
		if _, err := os.Stat(invoked); !os.IsNotExist(err) {
			executed, _ := os.ReadFile(invoked)
			t.Fatalf("unexpected toolchain invocation: %s (stat: %v)", executed, err)
		}
		for _, dir := range []string{workDir, outputDir} {
			for _, name := range []string{"app.wat", "app.wasm", "app.js", "server.go"} {
				if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
					t.Fatalf("unexpected %s in %s: %v", name, dir, err)
				}
			}
		}
		if stderr.Len() != 0 {
			t.Fatalf("%v produced stderr: %q", args, stderr.String())
		}
		return stdout.String()
	}
	readArtifact := func(t *testing.T, path string) []byte {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 {
			t.Fatalf("read nonempty artifact %s: %v", path, err)
		}
		return data
	}
	run(t, "build", source)
	artifact := filepath.Join(workDir, "wasm_app_hello.hfbc")
	buildBytes := readArtifact(t, artifact)
	if err := os.Remove(artifact); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"flagged", "no-flag"} {
		t.Run(mode, func(t *testing.T) {
			args := []string{source}
			path := artifact
			if mode == "flagged" {
				args = append(args, "-o", outputDir)
				path = filepath.Join(outputDir, "wasm_app_hello.hfbc")
			}
			if stdout := run(t, args...); stdout != "" {
				t.Fatalf("compile-only invocation ran program: stdout %q", stdout)
			}
			if !bytes.Equal(readArtifact(t, path), buildBytes) {
				t.Fatal("artifact differs from build's default bytecode")
			}
			if mode == "flagged" {
				if _, err := os.Stat(artifact); !os.IsNotExist(err) {
					t.Fatalf("flagged invocation wrote work directory artifact: %v", err)
				}
			}
			// Pure arithmetic requires no capabilities; execute with the default policy.
			artifactStdout := run(t, "-run-bc", path)
			sourceStdout := run(t, "run", source)
			const expected = ""
			if artifactStdout != expected || sourceStdout != expected {
				t.Fatalf("expected %q; artifact stdout %q, source stdout %q", expected, artifactStdout, sourceStdout)
			}
			if artifactStdout != sourceStdout {
				t.Fatal("artifact and source stdout differ")
			}
		})
	}
}

func TestWasmAppMathRunHFBC(t *testing.T) {
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
	source := filepath.Join(repoRoot, "examples", "wasm_math.howl")
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
	runExpect := func(t *testing.T, wantExit int, args ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir, cmd.Env = workDir, env
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		if ctx.Err() != nil {
			t.Fatalf("%v did not return: %v\nstdout: %s\nstderr: %s", args, ctx.Err(), &stdout, &stderr)
		}
		gotExit := 0
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				gotExit = ee.ExitCode()
			} else {
				t.Fatalf("%v failed: %v\nstdout: %s\nstderr: %s", args, err, &stdout, &stderr)
			}
		}
		if gotExit != wantExit {
			t.Fatalf("%v exit %d, want %d\nstdout: %s\nstderr: %s", args, gotExit, wantExit, &stdout, &stderr)
		}
		if _, err := os.Stat(invoked); !os.IsNotExist(err) {
			executed, _ := os.ReadFile(invoked)
			t.Fatalf("unexpected toolchain invocation: %s (stat: %v)", executed, err)
		}
		for _, dir := range []string{workDir, outputDir} {
			for _, name := range []string{"app.wat", "app.wasm", "app.js", "server.go"} {
				if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
					t.Fatalf("unexpected %s in %s: %v", name, dir, err)
				}
			}
		}
		if stderr.Len() != 0 {
			t.Fatalf("%v produced stderr: %q", args, stderr.String())
		}
		return stdout.String()
	}
	readArtifact := func(t *testing.T, path string) []byte {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 {
			t.Fatalf("read nonempty artifact %s: %v", path, err)
		}
		return data
	}
	runExpect(t, 0, "build", source)
	artifact := filepath.Join(workDir, "wasm_math.hfbc")
	buildBytes := readArtifact(t, artifact)
	if err := os.Remove(artifact); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"flagged", "no-flag"} {
		t.Run(mode, func(t *testing.T) {
			args := []string{source}
			path := artifact
			if mode == "flagged" {
				args = append(args, "-o", outputDir)
				path = filepath.Join(outputDir, "wasm_math.hfbc")
			}
			if stdout := runExpect(t, 0, args...); stdout != "" {
				t.Fatalf("compile-only invocation ran program: stdout %q", stdout)
			}
			if !bytes.Equal(readArtifact(t, path), buildBytes) {
				t.Fatal("artifact differs from build's default bytecode")
			}
			artifactStdout := runExpect(t, 42, "-run-bc", path)
			sourceStdout := runExpect(t, 42, "run", source)
			if artifactStdout != "" || sourceStdout != "" {
				t.Fatalf("expected empty stdout; artifact %q source %q", artifactStdout, sourceStdout)
			}
		})
	}
}

func TestWasmAppPrintRunHFBC(t *testing.T) {
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
	source := filepath.Join(repoRoot, "examples", "wasm_app_print.howl")
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
	run := func(t *testing.T, args ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir, cmd.Env = workDir, env
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		if ctx.Err() != nil {
			t.Fatalf("%v did not return: %v\nstdout: %s\nstderr: %s", args, ctx.Err(), &stdout, &stderr)
		}
		if err != nil {
			t.Fatalf("%v failed: %v\nstdout: %s\nstderr: %s", args, err, &stdout, &stderr)
		}
		if _, err := os.Stat(invoked); !os.IsNotExist(err) {
			executed, _ := os.ReadFile(invoked)
			t.Fatalf("unexpected toolchain invocation: %s (stat: %v)", executed, err)
		}
		for _, dir := range []string{workDir, outputDir} {
			for _, name := range []string{"app.wat", "app.wasm", "app.js", "server.go"} {
				if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
					t.Fatalf("unexpected %s in %s: %v", name, dir, err)
				}
			}
		}
		if stderr.Len() != 0 {
			t.Fatalf("%v produced stderr: %q", args, stderr.String())
		}
		return stdout.String()
	}
	readArtifact := func(t *testing.T, path string) []byte {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 {
			t.Fatalf("read nonempty artifact %s: %v", path, err)
		}
		return data
	}
	run(t, "build", source)
	artifact := filepath.Join(workDir, "wasm_app_print.hfbc")
	buildBytes := readArtifact(t, artifact)
	if err := os.Remove(artifact); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"flagged", "no-flag"} {
		t.Run(mode, func(t *testing.T) {
			args := []string{source}
			path := artifact
			if mode == "flagged" {
				args = append(args, "-o", outputDir)
				path = filepath.Join(outputDir, "wasm_app_print.hfbc")
			}
			if stdout := run(t, args...); stdout != "" {
				t.Fatalf("compile-only invocation ran program: stdout %q", stdout)
			}
			if !bytes.Equal(readArtifact(t, path), buildBytes) {
				t.Fatal("artifact differs from build's default bytecode")
			}
			const expected = "42\n"
			artifactStdout := run(t, "-run-bc", path)
			sourceStdout := run(t, "run", source)
			if artifactStdout != expected || sourceStdout != expected {
				t.Fatalf("expected %q; artifact stdout %q, source stdout %q", expected, artifactStdout, sourceStdout)
			}
		})
	}
}
