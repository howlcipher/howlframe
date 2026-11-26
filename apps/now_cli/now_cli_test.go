package now_cli_test

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNowCLI(t *testing.T) {
	artifact := filepath.Join(t.TempDir(), "now_cli.hfbc")
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "now_cli.howl", "-o", artifact)
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	before := time.Now().Unix() - 2
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", artifact)
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
