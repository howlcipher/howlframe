package receipt_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWasmAppNoFlagAndFlaggedWriteBuildBytecode(t *testing.T) {
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

	source := filepath.Join(repoRoot, "examples", "wasm_math.howl")
	toolDir := t.TempDir()
	invoked := filepath.Join(toolDir, "invoked")
	for _, name := range []string{"go", "node", "wat2wasm"} {
		body := "#!/bin/sh\necho \"$0\" >> \"" + invoked + "\"\nexit 99\n"
		if err := os.WriteFile(filepath.Join(toolDir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	workDir := t.TempDir()
	outputDir := t.TempDir()
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
		for _, dir := range []string{workDir, outputDir} {
			for _, name := range []string{"app.wat", "app.wasm", "app.js", "server.go"} {
				if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
					t.Fatalf("unexpected %s in %s: %v", name, dir, err)
				}
			}
		}
	}
	readArtifact := func(path string) []byte {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read artifact %s: %v", path, err)
		}
		if len(data) == 0 {
			t.Fatalf("artifact %s is empty", path)
		}
		return data
	}

	run("build", source)
	artifact := filepath.Join(workDir, "wasm_math.hfbc")
	buildBytes := readArtifact(artifact)
	if err := os.Remove(artifact); err != nil {
		t.Fatal(err)
	}

	run(source, "-o", outputDir)
	if !bytes.Equal(readArtifact(filepath.Join(outputDir, "wasm_math.hfbc")), buildBytes) {
		t.Fatal("flagged artifact differs from build's default bytecode")
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatalf("flagged invocation unexpectedly wrote work directory artifact: %v", err)
	}

	run(source)
	if !bytes.Equal(readArtifact(artifact), buildBytes) {
		t.Fatal("no-flag artifact differs from build's default bytecode")
	}
}

func TestWasmAppBuildTargetWasmStillEmitsWAT(t *testing.T) {
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

	workDir := t.TempDir()
	outputDir := filepath.Join(workDir, "wat")
	source := filepath.Join(repoRoot, "examples", "wasm_math.howl")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath, "build", "--target=wasm", source, "-o", outputDir)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("build --target=wasm did not return before timeout: %v\n%s", ctx.Err(), output)
	}
	if err != nil {
		t.Fatalf("build --target=wasm failed: %v\n%s", err, output)
	}

	wat, err := os.ReadFile(filepath.Join(outputDir, "app.wat"))
	if err != nil {
		t.Fatalf("read generated app.wat: %v", err)
	}
	if len(wat) == 0 || !strings.Contains(string(wat), "(module") {
		t.Fatalf("generated WAT is missing module content:\n%s", wat)
	}
}

func TestWasmAppBuildTargetWasmRefusesPrint(t *testing.T) {
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

	workDir := t.TempDir()
	outputDir := filepath.Join(workDir, "wat")
	source := filepath.Join(repoRoot, "examples", "wasm_app_print.howl")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath, "build", "--target=wasm", source, "-o", outputDir)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("build --target=wasm did not return before timeout: %v\n%s", ctx.Err(), output)
	}
	if err == nil {
		t.Fatalf("expected build --target=wasm to refuse print, got success:\n%s", output)
	}
	out := string(output)
	if !strings.Contains(out, "Wasm backend does not support") || !strings.Contains(out, "print") {
		t.Fatalf("expected print fail-closed diagnostic, got: %s", output)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "app.wat")); !os.IsNotExist(err) {
		t.Fatalf("print refusal unexpectedly wrote app.wat: %v", err)
	}
}
