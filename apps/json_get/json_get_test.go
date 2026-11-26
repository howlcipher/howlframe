package json_get_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestJsonGet(t *testing.T) {
	tmp := t.TempDir()
	artifact := filepath.Join(tmp, "json_get.hfbc")
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "json_get.howl", "-o", artifact)
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	sample := filepath.Join(tmp, "sample.json")
	if err := os.WriteFile(sample, []byte(`{"name":"ada","n":"7"}`), 0644); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) (string, error) {
		cmdArgs := append([]string{"run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem", artifact}, args...)
		c := exec.Command("go", cmdArgs...)
		c.Dir = "."
		out, err := c.CombinedOutput()
		return string(out), err
	}

	out, err := run(sample, "name")
	if err != nil || !strings.Contains(out, "ada") {
		t.Fatalf("name: %v %s", err, out)
	}
	out, err = run(sample, "n")
	if err != nil || !strings.Contains(out, "7") {
		t.Fatalf("n: %v %s", err, out)
	}
	out, err = run(sample, "missing")
	if err == nil || !strings.Contains(out, "NOT FOUND") {
		t.Fatalf("missing: err=%v out=%s", err, out)
	}
}
