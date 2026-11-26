package sleep_cli_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSleepCLI(t *testing.T) {
	artifact := filepath.Join(t.TempDir(), "sleep_cli.hfbc")
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "sleep_cli.howl", "-o", artifact)
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	start := time.Now()
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", artifact, "50")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "OK") {
		t.Fatalf("sleep: %v %s", err, out)
	}
	if time.Since(start) < 40*time.Millisecond {
		t.Fatalf("slept too briefly: %v", time.Since(start))
	}
}
