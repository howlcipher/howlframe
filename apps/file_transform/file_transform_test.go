package file_transform_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestFileTransform(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "file_transform.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	in := "sample.csv"
	out := "sample.jsonl"
	os.WriteFile(in, []byte("name,age\nalice,30\nbob,25\n"), 0644)
	defer os.Remove(in)
	defer os.Remove(out)

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		"file_transform.howl.bc.bin", in, out)
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
		"file_transform.howl.bc.bin", "missing.csv", out)
	cmd.Dir = "."
	got, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(got), "error=") {
		t.Fatalf("expected missing-file error, got err=%v out=%s", err, got)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc",
		"file_transform.howl.bc.bin", in, out)
	cmd.Dir = "."
	got, _ = cmd.CombinedOutput()
	if !strings.Contains(string(got), "capability denied: filesystem") {
		t.Fatalf("expected filesystem denial, got %s", got)
	}
}
