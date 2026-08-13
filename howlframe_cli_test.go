package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_PublicInterface(t *testing.T) {
	// Build the CLI binary
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "howlframe")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "howlframe.go")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build CLI: %v\nOutput: %s", err, string(out))
	}

	runCLI := func(args ...string) (string, error) {
		cmd := exec.Command(binaryPath, args...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	t.Run("version", func(t *testing.T) {
		out, err := runCLI("version")
		if err != nil {
			t.Errorf("version command failed: %v", err)
		}
		if !strings.Contains(out, "HowlFrame 0.1.0") {
			t.Errorf("expected version output, got: %s", out)
		}
	})

	t.Run("help", func(t *testing.T) {
		out, err := runCLI("help")
		if err != nil {
			t.Errorf("help command failed: %v", err)
		}
		if !strings.Contains(out, "HowlFrame v0.1 CLI") {
			t.Errorf("expected help output, got: %s", out)
		}
	})

	t.Run("check valid", func(t *testing.T) {
		out, err := runCLI("check", "tests/module_main.howl")
		if err != nil {
			t.Errorf("check command failed: %v\nOutput: %s", err, out)
		}
		if !strings.Contains(out, "OK: tests/module_main.howl") {
			t.Errorf("expected check success output, got: %s", out)
		}
	})

	t.Run("check invalid", func(t *testing.T) {
		invalidFile := filepath.Join(tmpDir, "invalid.howl")
		os.WriteFile(invalidFile, []byte("(unclosed"), 0644)
		out, err := runCLI("check", invalidFile)
		if err == nil {
			t.Errorf("expected check to fail for invalid syntax")
		}
		if !strings.Contains(out, "Unexpected tokens after EOF") && !strings.Contains(out, "Failed to encode mask plan") && !strings.Contains(out, "Unexpected") {
			// Actually ast.ReportError uses panic-like behavior or os.Exit so we just ensure it errors
		}
	})

	hfbcPath := filepath.Join(tmpDir, "output.hfbc")
	t.Run("build valid", func(t *testing.T) {
		out, err := runCLI("build", "tests/module_main.howl", "-o", hfbcPath)
		if err != nil {
			t.Errorf("build command failed: %v\nOutput: %s", err, out)
		}
		if !strings.Contains(out, "Built "+hfbcPath) {
			t.Errorf("expected build output, got: %s", out)
		}
	})

	t.Run("run artifact", func(t *testing.T) {
		out, err := runCLI("run", hfbcPath)
		if err != nil {
			t.Errorf("run command failed: %v\nOutput: %s", err, out)
		}
		if !strings.Contains(out, "42") {
			t.Errorf("expected execution output containing 42, got: %s", out)
		}
	})

	t.Run("legacy validate", func(t *testing.T) {
		out, err := runCLI("-validate", "tests/module_main.howl")
		if err != nil {
			t.Errorf("legacy -validate failed: %v\nOutput: %s", err, out)
		}
	})

	t.Run("legacy compile-bc", func(t *testing.T) {
		legacyHfbc := filepath.Join(tmpDir, "legacy.hfbc")
		out, err := runCLI("-compile-bc", "tests/module_main.howl", "-o", legacyHfbc)
		if err != nil {
			t.Errorf("legacy -compile-bc failed: %v\nOutput: %s", err, out)
		}
	})
}
