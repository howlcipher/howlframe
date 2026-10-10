package cut_cli_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCutCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "cut_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	os.WriteFile("sample.csv", []byte("a,b,c\n1,2,3\n"), 0644)
	defer os.Remove("sample.csv")

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		"cut_cli.howl.bc.bin", "1", "sample.csv")
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
