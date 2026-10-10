package wc_cli_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestWcCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "wc_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	os.WriteFile("sample.txt", []byte("one two\nthree\nfour five six\n"), 0644)
	defer os.Remove("sample.txt")

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		"wc_cli.howl.bc.bin", "sample.txt")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wc: %v %s", err, out)
	}
	got := string(out)
	// lines() keeps a trailing empty from final newline → 4 lines; words=6; bytes=28
	if !strings.Contains(got, "lines=4") || !strings.Contains(got, "words=6") || !strings.Contains(got, "bytes=28") {
		t.Fatalf("unexpected: %s", got)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc",
		"wc_cli.howl.bc.bin", "sample.txt")
	cmd.Dir = "."
	out, _ = cmd.CombinedOutput()
	if !strings.Contains(string(out), "capability denied: filesystem") {
		t.Fatalf("want denial, got %s", out)
	}
}
