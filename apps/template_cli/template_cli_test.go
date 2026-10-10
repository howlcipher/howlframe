package template_cli_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestTemplateCLI(t *testing.T) {
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "template_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	os.WriteFile("tpl.txt", []byte("Hi {{name}}!\n"), 0644)
	defer os.Remove("tpl.txt")

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		"template_cli.howl.bc.bin", "tpl.txt", "name", "Ada")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("template: %v %s", err, out)
	}
	if !strings.Contains(string(out), "Hi Ada!") {
		t.Fatalf("unexpected: %s", out)
	}

	os.WriteFile("nop.txt", []byte("no tokens\n"), 0644)
	defer os.Remove("nop.txt")
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem",
		"template_cli.howl.bc.bin", "nop.txt", "name", "Ada")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "NO PLACEHOLDER") {
		t.Fatalf("want NO PLACEHOLDER, got err=%v out=%s", err, out)
	}
}
