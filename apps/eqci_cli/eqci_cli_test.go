package eqci_cli_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestEqciCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "eqci_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc",
		"eqci_cli.howl.bc.bin", "Hello", "hello")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "YES") {
		t.Fatalf("yes: %v %s", err, out)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc",
		"eqci_cli.howl.bc.bin", "Hello", "world")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "NO") {
		t.Fatalf("no: err=%v out=%s", err, out)
	}
}
