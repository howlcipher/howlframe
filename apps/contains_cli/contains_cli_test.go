package contains_cli_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestContainsCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "contains_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	os.WriteFile("sample.txt", []byte("alpha beta gamma\n"), 0644)
	defer os.Remove("sample.txt")
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		"contains_cli.howl.bc.bin", "beta", "sample.txt")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "FOUND") {
		t.Fatalf("found: %v %s", err, out)
	}
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		"contains_cli.howl.bc.bin", "zzz", "sample.txt")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "MISSING") {
		t.Fatalf("missing: err=%v out=%s", err, out)
	}
}
