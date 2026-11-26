package upper_cli_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpperCLI(t *testing.T) {
	artifact := filepath.Join(t.TempDir(), "upper_cli.hfbc")
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "upper_cli.howl", "-o", artifact)
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc",
		artifact, "Hello World")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "HELLO WORLD") {
		t.Fatalf("upper: %v %s", err, out)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", artifact)
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "Usage:") {
		t.Fatalf("usage: err=%v out=%s", err, out)
	}
}
