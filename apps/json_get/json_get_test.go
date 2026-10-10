package json_get_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestJsonGet(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "json_get.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	os.WriteFile("sample.json", []byte(`{"name":"ada","n":"7"}`), 0644)
	defer os.Remove("sample.json")

	run := func(args ...string) (string, error) {
		cmdArgs := append([]string{"run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem", "json_get.howl.bc.bin"}, args...)
		c := exec.Command("go", cmdArgs...)
		c.Dir = "."
		out, err := c.CombinedOutput()
		return string(out), err
	}

	out, err := run("sample.json", "name")
	if err != nil || !strings.Contains(out, "ada") {
		t.Fatalf("name: %v %s", err, out)
	}
	out, err = run("sample.json", "n")
	if err != nil || !strings.Contains(out, "7") {
		t.Fatalf("n: %v %s", err, out)
	}
	out, err = run("sample.json", "missing")
	if err == nil || !strings.Contains(out, "NOT FOUND") {
		t.Fatalf("missing: err=%v out=%s", err, out)
	}
}
