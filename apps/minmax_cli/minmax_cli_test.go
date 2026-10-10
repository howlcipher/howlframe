package minmax_cli_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestMinmaxCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "minmax_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc",
		"minmax_cli.howl.bc.bin", "3", "9")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "min=3") || !strings.Contains(string(out), "max=9") {
		t.Fatalf("minmax: %v %s", err, out)
	}
}
