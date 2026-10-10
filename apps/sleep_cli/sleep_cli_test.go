package sleep_cli_test

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestSleepCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "sleep_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	start := time.Now()
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "sleep_cli.howl.bc.bin", "50")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "OK") {
		t.Fatalf("sleep: %v %s", err, out)
	}
	if time.Since(start) < 40*time.Millisecond {
		t.Fatalf("slept too briefly: %v", time.Since(start))
	}
}
