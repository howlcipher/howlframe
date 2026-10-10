package join_cli_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestJoinCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "join_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	os.WriteFile("sample.txt", []byte("a\nb\nc\n"), 0644)
	defer os.Remove("sample.txt")

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		"join_cli.howl.bc.bin", ",", "sample.txt")
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
