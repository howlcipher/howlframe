package cut_cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCutCLI(t *testing.T) {
	tmp := t.TempDir()
	artifact := filepath.Join(tmp, "cut_cli.hfbc")
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "cut_cli.howl", "-o", artifact)
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	sample := filepath.Join(tmp, "sample.csv")
	if err := os.WriteFile(sample, []byte("a,b,c\n1,2,3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		artifact, "1", sample)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v %s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "b\n") || !strings.Contains(got, "2\n") {
		t.Fatalf("expected column 1, got %q", got)
	}
}
