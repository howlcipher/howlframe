package uniq_cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUniqCLI(t *testing.T) {
	tmp := t.TempDir()
	artifact := filepath.Join(tmp, "uniq_cli.hfbc")
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "uniq_cli.howl", "-o", artifact)
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	sample := filepath.Join(tmp, "sample.txt")
	if err := os.WriteFile(sample, []byte("a\nb\na\nc\nb\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		artifact, sample)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v %s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "a\n") || !strings.Contains(got, "b\n") || !strings.Contains(got, "c\n") {
		t.Fatalf("expected a/b/c, got %q", got)
	}
	if strings.Count(got, "a\n") != 1 || strings.Count(got, "b\n") != 1 {
		t.Fatalf("expected unique lines, got %q", got)
	}
}
