package capability_lab_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCapabilityLab(t *testing.T) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	scratchDir := t.TempDir()
	compiler := filepath.Join(scratchDir, "howlframe")
	artifact := filepath.Join(scratchDir, "capability_lab.hfbc")

	build := exec.Command("go", "build", "-o", compiler, "howlframe.go")
	build.Dir = repositoryRoot
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build HowlFrame scratch binary: %v\n%s", buildErr, output)
	}

	appSource := filepath.Join(repositoryRoot, "examples", "capability_lab", "capability_lab.howl")
	compile := exec.Command(compiler, "-compile-bc", appSource, "-o", artifact)
	compile.Dir = filepath.Join(repositoryRoot, "examples", "capability_lab")
	if output, compileErr := compile.CombinedOutput(); compileErr != nil {
		t.Fatalf("compile application to bytecode: %v\n%s", compileErr, output)
	}

	t.Run("denied without capability", func(t *testing.T) {
		cmd := exec.Command(compiler, "-run-bc", artifact)
		cmd.Dir = scratchDir
		out, err := cmd.CombinedOutput()
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("expected exit code 1, got %v", err)
		}
		if !strings.Contains(string(out), "capability denied: filesystem") {
			t.Fatalf("expected capability denial message, got: %s", string(out))
		}
	})

	t.Run("success with capability", func(t *testing.T) {
		cmd := exec.Command(compiler, "-run-bc", "-allow-caps", "filesystem", artifact)
		cmd.Dir = scratchDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("expected success, got %v\n%s", err, string(out))
		}
		if !strings.Contains(string(out), "Write succeeded!") {
			t.Fatalf("expected success message, got: %s", string(out))
		}

		// Verify file was written
		content, err := os.ReadFile(filepath.Join(scratchDir, "sensitive_data.txt"))
		if err != nil {
			t.Fatalf("failed to read output file: %v", err)
		}
		if string(content) != "Confidential info" {
			t.Fatalf("unexpected file content: %s", string(content))
		}
	})
}
