package env_cli_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestEnvCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "env_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	os.Setenv("HOWLFRAME_ENV_CLI", "hello")
	defer os.Unsetenv("HOWLFRAME_ENV_CLI")

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "environment",
		"env_cli.howl.bc.bin", "HOWLFRAME_ENV_CLI")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "hello") {
		t.Fatalf("get: %v %s", err, out)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "environment",
		"env_cli.howl.bc.bin", "HOWLFRAME_ENV_CLI_MISSING")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "MISSING") {
		t.Fatalf("missing: err=%v out=%s", err, out)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc",
		"env_cli.howl.bc.bin", "HOWLFRAME_ENV_CLI")
	cmd.Dir = "."
	out, _ = cmd.CombinedOutput()
	if !strings.Contains(string(out), "capability denied: environment") {
		t.Fatalf("expected environment denial, got %s", out)
	}
}
