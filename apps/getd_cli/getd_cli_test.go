package getd_cli_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGetdCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "getd_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	os.WriteFile("sample.json", []byte(`{"name":"ada"}`), 0644)
	defer os.Remove("sample.json")

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		"getd_cli.howl.bc.bin", "sample.json", "name")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "ada") {
		t.Fatalf("hit: %v %s", err, out)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		"getd_cli.howl.bc.bin", "sample.json", "missing", "fallback")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "fallback") {
		t.Fatalf("default: %v %s", err, out)
	}
}
