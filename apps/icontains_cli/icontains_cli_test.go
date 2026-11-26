package icontains_cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIContainsCLI(t *testing.T) {
	tmp := t.TempDir()
	artifact := filepath.Join(tmp, "icontains_cli.hfbc")
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "icontains_cli.howl", "-o", artifact)
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	sample := filepath.Join(tmp, "sample.txt")
	if err := os.WriteFile(sample, []byte("Alpha Beta Gamma\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		artifact, "beta", sample)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "FOUND") {
		t.Fatalf("found: %v %s", err, out)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		artifact, "delta", sample)
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "MISSING") {
		t.Fatalf("missing: err=%v out=%s", err, out)
	}
}
