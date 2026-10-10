package mkdir_cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMkdirCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "mkdir_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		"mkdir_cli.howl.bc.bin", dir)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "OK") {
		t.Fatalf("mkdir: %v %s", err, out)
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("dir missing: %v", err)
	}
}
