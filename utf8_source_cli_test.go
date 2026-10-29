package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Non-ASCII string literals must reach every target unchanged. The lexer used
// to read one byte per code point, so "é" printed as "Ã©" on the VM and in
// generated JavaScript alike.
func TestUTF8StringLiteralsSurviveEveryTarget(t *testing.T) {
	bin := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	want := "Héllo ✓ 日本 🐺"

	cli := mustWriteFile(t, dir, "cli.howl", `(cli_app (print "`+want+`"))`)
	out, err := exec.Command(bin, "run", cli).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != want {
		t.Fatalf("bytecode run: got %q (%v), want %q", out, err, want)
	}
	out, err = exec.Command(bin, "-run", cli).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != want {
		t.Fatalf("direct run: got %q (%v), want %q", out, err, want)
	}

	web := mustWriteFile(t, dir, "web.howl", `(web_app (print "`+want+`"))`)
	jsDir := filepath.Join(dir, "js")
	if out, err := exec.Command(bin, "build", web, "--target", "js", "-o", jsDir).CombinedOutput(); err != nil {
		t.Fatalf("js build: %v %s", err, out)
	}
	out, err = exec.Command("node", filepath.Join(jsDir, "app.js")).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != want {
		t.Fatalf("javascript: got %q (%v), want %q", out, err, want)
	}
}

func TestInvalidUTF8SourceIsRejected(t *testing.T) {
	bin := buildHowlFrameBinaryForTest(t)
	path := mustWriteFile(t, t.TempDir(), "bad.howl", "(cli_app (print \"\xff\xfe\"))")
	out, err := exec.Command(bin, "check", path).CombinedOutput()
	if err == nil || !strings.Contains(string(out), "not valid UTF-8") {
		t.Fatalf("invalid UTF-8 must be rejected, got %v %s", err, out)
	}
}
