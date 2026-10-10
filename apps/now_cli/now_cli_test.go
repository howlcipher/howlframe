package now_cli_test

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNowCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "now_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	before := time.Now().Unix() - 2
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "now_cli.howl.bc.bin")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v %s", err, out)
	}
	after := time.Now().Unix() + 2
	n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil || n < before || n > after {
		t.Fatalf("unexpected unix now: %q err=%v", out, err)
	}
}
