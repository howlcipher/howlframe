package trim_cli_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestTrimCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "trim_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc",
		"trim_cli.howl.bc.bin", "  hello  ")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "hello" {
		t.Fatalf("trim: %v %s", err, out)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "trim_cli.howl.bc.bin")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "Usage:") {
		t.Fatalf("usage: err=%v out=%s", err, out)
	}
}
