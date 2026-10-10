package notes_cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNotesCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "notes_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	dir := t.TempDir()
	store := "file://" + filepath.Join(dir, "notes.store")
	run := func(args ...string) (string, error) {
		cmdArgs := append([]string{"run", "../../howlframe.go", "-run-bc", "-allow-caps", "database,filesystem,environment", "notes_cli.howl.bc.bin"}, args...)
		c := exec.Command("go", cmdArgs...)
		c.Dir = "."
		c.Env = append(os.Environ(), "NOTES_STORE="+store)
		out, err := c.CombinedOutput()
		return string(out), err
	}

	out, err := run("add", "first", "add", "second", "list")
	if err != nil {
		t.Fatalf("add/list: %v %s", err, out)
	}
	if !strings.Contains(out, "ADDED 1 first") || !strings.Contains(out, "ADDED 2 second") ||
		!strings.Contains(out, "1 first") || !strings.Contains(out, "2 second") {
		t.Fatalf("unexpected: %s", out)
	}

	// Persistence across process
	out, err = run("get", "1", "delete", "2", "list")
	if err != nil {
		t.Fatalf("persist: %v %s", err, out)
	}
	if !strings.Contains(out, "1 first") || !strings.Contains(out, "DELETED 2") || strings.Contains(out, "2 second") {
		t.Fatalf("persist unexpected: %s", out)
	}

	out, _ = run("get", "missing")
	if !strings.Contains(out, "NOT FOUND") {
		t.Fatalf("missing: %s", out)
	}
}
