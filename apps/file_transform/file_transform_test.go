package file_transform_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileTransform(t *testing.T) {
	tmp := t.TempDir()
	artifact := filepath.Join(tmp, "file_transform.hfbc")
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "file_transform.howl", "-o", artifact)
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	in := filepath.Join(tmp, "sample.csv")
	out := filepath.Join(tmp, "sample.jsonl")
	if err := os.WriteFile(in, []byte("name,age\nalice,30\nbob,25\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		artifact, in, out)
	cmd.Dir = "."
	got, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("transform failed: %v out=%s", err, got)
	}
	if !strings.Contains(string(got), "rows=3") {
		t.Fatalf("expected rows=3, got %s", got)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read out: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, `"c0":"alice"`) || !strings.Contains(text, `"c1":"30"`) {
		t.Fatalf("unexpected jsonl: %s", text)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		artifact, filepath.Join(tmp, "missing.csv"), out)
	cmd.Dir = "."
	got, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(got), "error=") {
		t.Fatalf("expected missing-file error, got err=%v out=%s", err, got)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc",
		artifact, in, out)
	cmd.Dir = "."
	got, _ = cmd.CombinedOutput()
	if !strings.Contains(string(got), "capability denied: filesystem") {
		t.Fatalf("expected filesystem denial, got %s", got)
	}
}
