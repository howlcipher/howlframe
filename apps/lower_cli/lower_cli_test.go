package lower_cli_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestLowerCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "lower_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc",
		"lower_cli.howl.bc.bin", "Hello World")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "hello world") {
		t.Fatalf("lower: %v %s", err, out)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "lower_cli.howl.bc.bin")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "Usage:") {
		t.Fatalf("usage: err=%v out=%s", err, out)
	}
}
