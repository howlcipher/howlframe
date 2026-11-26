package template_cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateCLI(t *testing.T) {
	tmp := t.TempDir()
	artifact := filepath.Join(tmp, "template_cli.hfbc")
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "template_cli.howl", "-o", artifact)
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	tpl := filepath.Join(tmp, "tpl.txt")
	if err := os.WriteFile(tpl, []byte("Hi {{name}}!\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		artifact, tpl, "name", "Ada")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("template: %v %s", err, out)
	}
	if !strings.Contains(string(out), "Hi Ada!") {
		t.Fatalf("unexpected: %s", out)
	}

	nop := filepath.Join(tmp, "nop.txt")
	if err := os.WriteFile(nop, []byte("no tokens\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		artifact, nop, "name", "Ada")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "NO PLACEHOLDER") {
		t.Fatalf("want NO PLACEHOLDER, got err=%v out=%s", err, out)
	}
}
