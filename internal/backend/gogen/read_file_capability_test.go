package gogen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestGoReadFileRequiresGrantBeforeRead(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "phase2c-secret-path.txt")
	const secretBody = "phase2c-secret-bytes"
	if err := os.WriteFile(secretPath, []byte(secretBody), 0o644); err != nil {
		t.Fatal(err)
	}
	source := `(cli_app
  (let (value (read_file "` + secretPath + `"))
    (print (bytes_to_string value))))`
	root := parser.NewParser(lexer.NewLexer(source), "read_file.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if strings.Contains(code, `os.ReadFile("`) {
		t.Fatalf("read_file reads at the call site:\n%s", code)
	}
	if !strings.Contains(code, `howlFrameReadFileBytes("`) {
		t.Fatalf("read_file is not mediated:\n%s", code)
	}
	helperAt := strings.Index(code, "func howlFrameReadFile(")
	if helperAt < 0 {
		t.Fatalf("missing howlFrameReadFile helper:\n%s", code)
	}
	helper := code[helperAt:]
	denyAt := strings.Index(helper, "CAPABILITY_DENIED")
	readAt := strings.Index(helper, "os.ReadFile")
	if denyAt < 0 || readAt < 0 || denyAt > readAt {
		t.Fatalf("grant check must precede the filesystem read:\n%s", helper)
	}

	rootDir := moduleRoot(t)
	crashPath := filepath.Join(rootDir, "crash.json")
	t.Cleanup(func() { os.Remove(crashPath) })

	assertDenied := func(label, output, errText string) {
		t.Helper()
		if errText == "" {
			t.Fatalf("%s: read_file succeeded: %s", label, output)
		}
		if strings.Contains(output, secretBody) || strings.Contains(output, "phase2c-secret-path") {
			t.Fatalf("%s: process output leaked the file: %s", label, output)
		}
		crash, err := os.ReadFile(crashPath)
		if err != nil {
			t.Fatalf("%s: denial did not write crash.json: %v\n%s", label, err, output)
		}
		if strings.Contains(string(crash), secretBody) || strings.Contains(string(crash), "phase2c-secret-path") {
			t.Fatalf("%s: crash.json leaked the file: %s", label, crash)
		}
		if !strings.Contains(string(crash), "CAPABILITY_DENIED") || !strings.Contains(string(crash), "capability denied: filesystem") {
			t.Fatalf("%s: crash.json = %s, want CAPABILITY_DENIED", label, crash)
		}
		os.Remove(crashPath)
	}

	unsetOut, unsetErr := runGenerated(t, rootDir, code)
	assertDenied("unset grant", unsetOut, unsetErr)

	emptyOut, emptyErr := runGenerated(t, rootDir, code, "HOWLFRAME_ALLOW_CAPS=")
	assertDenied("empty grant", emptyOut, emptyErr)

	otherOut, otherErr := runGenerated(t, rootDir, code, "HOWLFRAME_ALLOW_CAPS=process")
	assertDenied("process grant", otherOut, otherErr)

	granted, grantErr := runGenerated(t, rootDir, code, "HOWLFRAME_ALLOW_CAPS=filesystem")
	if grantErr != "" {
		t.Fatalf("filesystem grant failed: %s\n%s", grantErr, granted)
	}
	if strings.TrimSpace(granted) != secretBody {
		t.Fatalf("granted stdout = %q, want %s", granted, secretBody)
	}
}

func TestGoReadFileTryLetDeniesBeforeRead(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "phase2c-secret-path.txt")
	source := `(cli_app
  (try_let (value (read_file "` + missing + `"))
    (catch err (print "caught-io"))
    (print "read-ok")))`
	root := parser.NewParser(lexer.NewLexer(source), "read_file.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if strings.Contains(code, ":= howlFrameReadFileBytes(") {
		t.Fatalf("try_let read_file dropped the error return:\n%s", code)
	}
	if !strings.Contains(code, "value, err := howlFrameReadFile(") {
		t.Fatalf("try_let read_file is not mediated:\n%s", code)
	}
	if strings.Contains(code, `os.ReadFile("`) {
		t.Fatalf("try_let read_file reads at the call site:\n%s", code)
	}

	rootDir := moduleRoot(t)
	crashPath := filepath.Join(rootDir, "crash.json")
	t.Cleanup(func() { os.Remove(crashPath) })

	denied, errText := runGenerated(t, rootDir, code, "HOWLFRAME_ALLOW_CAPS=")
	if errText == "" {
		t.Fatalf("empty grant succeeded: %s", denied)
	}
	if strings.Contains(denied, "caught-io") || strings.Contains(denied, "read-ok") {
		t.Fatalf("empty grant treated the miss as an IO error: %s", denied)
	}
	crash, err := os.ReadFile(crashPath)
	if err != nil {
		t.Fatalf("denial did not write crash.json: %v\n%s", err, denied)
	}
	if strings.Contains(string(crash), "phase2c-secret-path") {
		t.Fatalf("crash.json leaked the path: %s", crash)
	}
	if !strings.Contains(string(crash), "CAPABILITY_DENIED") || !strings.Contains(string(crash), "capability denied: filesystem") {
		t.Fatalf("crash.json = %s, want CAPABILITY_DENIED", crash)
	}
	os.Remove(crashPath)

	granted, grantErr := runGenerated(t, rootDir, code, "HOWLFRAME_ALLOW_CAPS=filesystem")
	if grantErr != "" {
		t.Fatalf("filesystem grant failed: %s\n%s", grantErr, granted)
	}
	if strings.TrimSpace(granted) != "caught-io" {
		t.Fatalf("granted miss stdout = %q, want caught-io", granted)
	}
}

func TestGoReadFileExpressionPositionIsMediated(t *testing.T) {
	const source = `(cli_app (print (bytes_to_string (read_file "phase2c-expr.txt"))))`
	root := parser.NewParser(lexer.NewLexer(source), "read_file.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if !strings.Contains(code, `howlFrameReadFileBytes("phase2c-expr.txt")`) {
		t.Fatalf("expression-position read_file is not mediated:\n%s", code)
	}
	if strings.Contains(code, `os.ReadFile("`) {
		t.Fatalf("expression-position read_file bypasses the grant:\n%s", code)
	}
}
