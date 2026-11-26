package starts_cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartsCLI(t *testing.T) {
	tmp := t.TempDir()
	artifact := filepath.Join(tmp, "starts_cli.hfbc")
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "starts_cli.howl", "-o", artifact)
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	sample := filepath.Join(tmp, "sample.txt")
	if err := os.WriteFile(sample, []byte("hello world\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		artifact, "hello", sample)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "YES") {
		t.Fatalf("yes: %v %s", err, out)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		artifact, "world", sample)
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "NO") {
		t.Fatalf("no: err=%v out=%s", err, out)
	}
}
