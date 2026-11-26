package join_cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestJoinCLI(t *testing.T) {
	tmp := t.TempDir()
	artifact := filepath.Join(tmp, "join_cli.hfbc")
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "join_cli.howl", "-o", artifact)
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	sample := filepath.Join(tmp, "sample.txt")
	if err := os.WriteFile(sample, []byte("a\nb\nc\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		artifact, ",", sample)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v %s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if got != "a,b,c," {
		t.Fatalf("expected a,b,c, got %q", got)
	}
}
