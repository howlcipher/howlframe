package language_tour_test

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLanguageTour(t *testing.T) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	scratchDir := t.TempDir()
	compiler := filepath.Join(scratchDir, "howlframe")
	artifact := filepath.Join(scratchDir, "language_tour.hfbc")

	build := exec.Command("go", "build", "-o", compiler, "howlframe.go")
	build.Dir = repositoryRoot
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build HowlFrame scratch binary: %v\n%s", buildErr, output)
	}

	appSource := filepath.Join(repositoryRoot, "examples", "language_tour", "language_tour.howl")
	compile := exec.Command(compiler, "-compile-bc", appSource, "-o", artifact)
	compile.Dir = filepath.Join(repositoryRoot, "examples", "language_tour")
	if output, compileErr := compile.CombinedOutput(); compileErr != nil {
		t.Fatalf("compile application to bytecode: %v\n%s", compileErr, output)
	}

	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
	}{
		{
			name:     "all pass",
			args:     []string{"proj1", "PASS", "PASS", "PASS"},
			wantCode: 0,
			wantOut:  "RELEASE READINESS\nProject: proj1\nScore:   100\nDecision: PASS\n",
		},
		{
			name:     "security fail",
			args:     []string{"proj2", "PASS", "FAIL", "PASS"},
			wantCode: 1,
			wantOut:  "RELEASE READINESS\nProject: proj2\nScore:   60\nDecision: FAIL\n",
		},
		{
			name:     "missing docs review",
			args:     []string{"proj3", "PASS", "PASS", "FAIL"},
			wantCode: 0,
			wantOut:  "RELEASE READINESS\nProject: proj3\nScore:   80\nDecision: PASS\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commandArgs := append([]string{"-run-bc", artifact}, tt.args...)
			cmd := exec.Command(compiler, commandArgs...)
			out, err := cmd.CombinedOutput()

			if tt.wantCode == 0 {
				if err != nil {
					t.Fatalf("expected success, got %v\n%s", err, string(out))
				}
			} else {
				var exitError *exec.ExitError
				if !errors.As(err, &exitError) {
					t.Fatalf("expected exit error, got %v", err)
				}
				if exitError.ExitCode() != tt.wantCode {
					t.Fatalf("expected exit code %d, got %d", tt.wantCode, exitError.ExitCode())
				}
			}

			if !strings.Contains(string(out), tt.wantOut) {
				t.Fatalf("output mismatch.\nGot:\n%s\nExpected to contain:\n%s", string(out), tt.wantOut)
			}
		})
	}
}
