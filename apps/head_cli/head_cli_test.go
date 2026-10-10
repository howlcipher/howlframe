package head_cli_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestHeadCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "head_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	os.WriteFile("sample.txt", []byte("a\nb\nc\nd\n"), 0644)
	defer os.Remove("sample.txt")

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		"head_cli.howl.bc.bin", "2", "sample.txt")
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
