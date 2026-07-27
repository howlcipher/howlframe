package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutputDirectoryFlag(t *testing.T) {
	// Build the zero binary
	cmd := exec.Command("go", "build", "-o", "zero", "zero.go")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	// Create a temporary directory for output
	outDir, err := os.MkdirTemp("", "zero-test-out-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(outDir)

	// Create a dummy .zero file
	inputFile := filepath.Join(outDir, "dummy.zero")
	if err := os.WriteFile(inputFile, []byte("(cli_app (print \"Hello\"))"), 0644); err != nil {
		t.Fatalf("Failed to write dummy file: %v", err)
	}

	// Run the zero binary with -o flag
	cmd = exec.Command("./zero", "-o", outDir, inputFile)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to run zero binary: %v", err)
	}

	// Check if server.go was created in the output directory
	serverFile := filepath.Join(outDir, "server.go")
	if _, err := os.Stat(serverFile); os.IsNotExist(err) {
		t.Errorf("Expected server.go to be created in %s, but it was not", outDir)
	}
}

func TestCrashStateSerialization(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "zero", "zero.go")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	outDir, err := os.MkdirTemp("", "zero-crash-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(outDir)

	inputFile := filepath.Join(outDir, "panic.zero")
	if err := os.WriteFile(inputFile, []byte(`(cli_app (call panic "test crash dump"))`), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cmd = exec.Command("./zero", "-o", outDir, inputFile)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to transpilation: %v", err)
	}

	serverFile := filepath.Join(outDir, "server.go")
	appBin := filepath.Join(outDir, "app")
	cmd = exec.Command("go", "build", "-o", appBin, serverFile)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build generated server.go: %v", err)
	}

	cmd = exec.Command(appBin)
	cmd.Dir = outDir
	_ = cmd.Run() // expected to exit with non-zero code

	crashFile := filepath.Join(outDir, "crash.json")
	data, err := os.ReadFile(crashFile)
	if err != nil {
		t.Fatalf("Failed to read crash.json: %v", err)
	}

	if len(data) == 0 {
		t.Fatalf("crash.json is empty")
	}
}

func TestWasmBackendWritesPortableWAT(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "zero", "zero.go")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "math.zero")
	input := "(wasm_app (if (> (+ 2 3) 4) (return (* 6 7)) (return 0)))"
	if err := os.WriteFile(inputFile, []byte(input), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cmd = exec.Command("./zero", "-o", outDir, inputFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to generate WAT: %v: %s", err, output)
	}

	wat, err := os.ReadFile(filepath.Join(outDir, "app.wat"))
	if err != nil {
		t.Fatalf("Failed to read generated WAT: %v", err)
	}
	for _, fragment := range []string{
		"(module", "(func (export \"main\") (result i32)", "(if (result i32)", "(i32.gt_s", "(i32.add", "(i32.mul",
	} {
		if !strings.Contains(string(wat), fragment) {
			t.Errorf("Generated WAT is missing %q:\n%s", fragment, wat)
		}
	}
}

func TestWasmBackendRejectsUnsupportedNodes(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "zero", "zero.go")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "unsupported.zero")
	if err := os.WriteFile(inputFile, []byte("(wasm_app \"not supported\")"), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cmd = exec.Command("./zero", "-o", outDir, inputFile)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("Expected unsupported wasm node to fail")
	}
	if !strings.Contains(string(output), "Wasm backend does not support") {
		t.Errorf("Expected a clear unsupported-node diagnostic, got: %s", output)
	}
}
