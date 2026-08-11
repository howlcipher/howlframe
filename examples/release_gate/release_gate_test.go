package release_gate_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseGate(t *testing.T) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	scratchDir := t.TempDir()
	compiler := filepath.Join(scratchDir, "howlframe")
	artifact := filepath.Join(scratchDir, "release_gate.hfbc")

	build := exec.Command("go", "build", "-o", compiler, "howlframe.go")
	build.Dir = repositoryRoot
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build HowlFrame scratch binary: %v\n%s", buildErr, output)
	}

	appSource := filepath.Join(repositoryRoot, "examples", "release_gate", "release_gate.howl")
	compile := exec.Command(compiler, "-compile-bc", appSource, "-o", artifact)
	compile.Dir = filepath.Join(repositoryRoot, "examples", "release_gate")
	if output, compileErr := compile.CombinedOutput(); compileErr != nil {
		t.Fatalf("compile application to bytecode: %v\n%s", compileErr, output)
	}

	t.Run("deploy decision", func(t *testing.T) {
		signalsPath := filepath.Join(scratchDir, "signals_deploy.txt")
		os.WriteFile(signalsPath, []byte("tests=PASS\nsecurity=PASS\n"), 0o600)

		cmd := exec.Command(compiler, "-run-bc", "-allow-caps", "filesystem", artifact, signalsPath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("expected success, got %v\n%s", err, string(out))
		}
		expected := "RELEASE READINESS\nScore: 100\nDecision: DEPLOY\n"
		if !strings.Contains(string(out), expected) {
			t.Fatalf("output mismatch.\nGot:\n%s\nExpected to contain:\n%s", string(out), expected)
		}
	})

	t.Run("block decision", func(t *testing.T) {
		signalsPath := filepath.Join(scratchDir, "signals_block.txt")
		os.WriteFile(signalsPath, []byte("tests=PASS\nsecurity=FAIL\n"), 0o600)

		cmd := exec.Command(compiler, "-run-bc", "-allow-caps", "filesystem", artifact, signalsPath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("expected success (app handles failure gracefully with exit 0), got %v\n%s", err, string(out))
		}
		expected := "RELEASE READINESS\nScore: 50\nDecision: BLOCK\n"
		if !strings.Contains(string(out), expected) {
			t.Fatalf("output mismatch.\nGot:\n%s\nExpected to contain:\n%s", string(out), expected)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		cmd := exec.Command(compiler, "-run-bc", "-allow-caps", "filesystem", artifact, filepath.Join(scratchDir, "missing.txt"))
		out, err := cmd.CombinedOutput()
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() != 3 {
			t.Fatalf("expected exit code 3, got %v", err)
		}
		if !strings.Contains(string(out), "Failed to read config file") {
			t.Fatalf("expected failure message, got: %s", string(out))
		}
	})
}
