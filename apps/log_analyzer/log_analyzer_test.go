package log_analyzer_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestLogAnalyzer(t *testing.T) {
	// Compile
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "log_analyzer.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	// Clean log
	os.WriteFile("clean.log", []byte("INFO: Start\nINFO: End\n"), 0644)
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem", "log_analyzer.howl.bc.bin", "clean.log")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("clean log failed: %v, out: %s", err, out)
	}
	if !strings.Contains(string(out), "status=CLEAN") {
		t.Errorf("expected CLEAN, got: %s", out)
	}

	// Err log
	os.WriteFile("err.log", []byte("ERROR: Failed\n"), 0644)
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem", "log_analyzer.howl.bc.bin", "err.log")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("expected err log to fail with exit 1")
	}
	if !strings.Contains(string(out), "status=ATTENTION") {
		t.Errorf("expected ATTENTION, got: %s", out)
	}

	// Missing file
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem", "log_analyzer.howl.bc.bin", "missing.log")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("expected missing to fail with exit 2")
	}
	if !strings.Contains(string(out), "error=") {
		t.Errorf("expected error=, got: %s", out)
	}

	// Capability denied
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "log_analyzer.howl.bc.bin", "clean.log")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if !strings.Contains(string(out), "capability denied: filesystem") {
		t.Errorf("expected filesystem capability denied, got: %s", out)
	}

	// Cleanup
	os.Remove("clean.log")
	os.Remove("err.log")
}
