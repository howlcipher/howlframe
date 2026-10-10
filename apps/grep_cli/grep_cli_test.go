package grep_cli_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGrepCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "grep_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	os.WriteFile("sample.txt", []byte("alpha\nbeta foo\ngamma\nfoo bar\n"), 0644)
	defer os.Remove("sample.txt")

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		"grep_cli.howl.bc.bin", "foo", "sample.txt")
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
		"grep_cli.howl.bc.bin", "zzz", "sample.txt")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "hits=0") {
		t.Fatalf("want exit 1 hits=0, got err=%v out=%s", err, out)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc",
		"grep_cli.howl.bc.bin", "foo", "sample.txt")
	cmd.Dir = "."
	out, _ = cmd.CombinedOutput()
	if !strings.Contains(string(out), "capability denied: filesystem") {
		t.Fatalf("want denial, got %s", out)
	}
}
