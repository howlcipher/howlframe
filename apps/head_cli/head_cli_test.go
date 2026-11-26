package head_cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHeadCLI(t *testing.T) {
	tmp := t.TempDir()
	artifact := filepath.Join(tmp, "head_cli.hfbc")
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "head_cli.howl", "-o", artifact)
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	sample := filepath.Join(tmp, "sample.txt")
	if err := os.WriteFile(sample, []byte("a\nb\nc\nd\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		artifact, "2", sample)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v %s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "a\n") || !strings.Contains(got, "b\n") {
		t.Fatalf("expected first two lines, got %q", got)
	}
	if strings.Count(got, "c") > 0 {
		t.Fatalf("did not expect c: %q", got)
	}
}
