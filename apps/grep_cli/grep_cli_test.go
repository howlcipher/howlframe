package grep_cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrepCLI(t *testing.T) {
	tmp := t.TempDir()
	artifact := filepath.Join(tmp, "grep_cli.hfbc")
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "grep_cli.howl", "-o", artifact)
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	sample := filepath.Join(tmp, "sample.txt")
	if err := os.WriteFile(sample, []byte("alpha\nbeta foo\ngamma\nfoo bar\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		artifact, "foo", sample)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("grep hit: %v %s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "beta foo") || !strings.Contains(got, "foo bar") || !strings.Contains(got, "hits=2") {
		t.Fatalf("unexpected: %s", got)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		artifact, "zzz", sample)
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "hits=0") {
		t.Fatalf("want exit 1 hits=0, got err=%v out=%s", err, out)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc",
		artifact, "foo", sample)
	cmd.Dir = "."
	out, _ = cmd.CombinedOutput()
	if !strings.Contains(string(out), "capability denied: filesystem") {
		t.Fatalf("want denial, got %s", out)
	}
}
