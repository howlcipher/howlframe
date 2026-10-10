package uniq_cli_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestUniqCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "uniq_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	os.WriteFile("sample.txt", []byte("a\nb\na\nc\nb\n"), 0644)
	defer os.Remove("sample.txt")

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		"uniq_cli.howl.bc.bin", "sample.txt")
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
