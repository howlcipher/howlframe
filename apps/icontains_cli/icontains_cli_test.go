package icontains_cli_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestIContainsCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "icontains_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	os.WriteFile("sample.txt", []byte("Alpha Beta Gamma\n"), 0644)
	defer os.Remove("sample.txt")

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		"icontains_cli.howl.bc.bin", "beta", "sample.txt")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "FOUND") {
		t.Fatalf("found: %v %s", err, out)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		"icontains_cli.howl.bc.bin", "delta", "sample.txt")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "MISSING") {
		t.Fatalf("missing: err=%v out=%s", err, out)
	}
}
