package kv_cli_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestKVCLI(t *testing.T) {
	// Compile
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "kv_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	// Test state sharing sequence
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "database", "kv_cli.howl.bc.bin", "set", "theme", "dark", "get", "theme", "get", "missing", "delete", "theme", "get", "theme")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("sequence failed: %v, out: %s", err, out)
	}
	expected := "OK\ndark\nNOT FOUND\nOK\nNOT FOUND\n"
	if string(out) != expected {
		t.Errorf("expected %q, got %q", expected, string(out))
	}

	// Test increment sequence
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "database", "kv_cli.howl.bc.bin", "increment", "visits", "increment", "visits", "get", "visits")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("increment sequence failed: %v, out: %s", err, out)
	}
	expected = "1\n2\n2\n"
	if string(out) != expected {
		t.Errorf("expected %q, got %q", expected, string(out))
	}

	// Capability denied
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "kv_cli.howl.bc.bin", "set", "theme", "dark")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("expected capability denial to fail exit status")
	}
	if !strings.Contains(string(out), "capability denied: database") {
		t.Errorf("expected capability denied message, got: %s", out)
	}
}
