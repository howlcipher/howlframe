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

func TestGoWriteFileRequiresGrantBeforeWrite(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "phase2e-secret-path.txt")
	const kept = "phase2e-kept"
	const secretBody = "phase2e-secret-bytes"
	if err := os.WriteFile(secretPath, []byte(kept), 0o644); err != nil {
		t.Fatal(err)
	}
	source := `(cli_app
  (write_file "` + secretPath + `" "` + secretBody + `")
  (print "phase2e-wrote"))`
	root := parser.NewParser(lexer.NewLexer(source), "write_file.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if strings.Contains(code, `os.WriteFile("`+secretPath) {
		t.Fatalf("write_file writes at the call site:\n%s", code)
	}
	if !strings.Contains(code, `howlFrameWriteFile("`+secretPath) {
		t.Fatalf("write_file is not mediated:\n%s", code)
	}
	helperAt := strings.Index(code, "func howlFrameWriteFile(")
	if helperAt < 0 {
		t.Fatalf("missing howlFrameWriteFile helper:\n%s", code)
	}
	helper := code[helperAt:]
	denyAt := strings.Index(helper, "CAPABILITY_DENIED")
	writeAt := strings.Index(helper, "os.WriteFile")
	if denyAt < 0 || writeAt < 0 || denyAt > writeAt {
		t.Fatalf("grant check must precede the filesystem write:\n%s", helper)
	}

	rootDir := moduleRoot(t)
	crashPath := filepath.Join(rootDir, "crash.json")
	t.Cleanup(func() { os.Remove(crashPath) })

	assertDenied := func(label, output, errText string) {
		t.Helper()
		if errText == "" {
			t.Fatalf("%s: write_file succeeded: %s", label, output)
		}
		if strings.Contains(output, secretBody) || strings.Contains(output, "phase2e-secret-path") || strings.Contains(output, "phase2e-wrote") {
			t.Fatalf("%s: process output leaked the write: %s", label, output)
		}
		body, err := os.ReadFile(secretPath)
		if err != nil {
			t.Fatalf("%s: seed file missing: %v", label, err)
		}
		if string(body) != kept {
			t.Fatalf("%s: file body = %q, want unchanged %s", label, body, kept)
		}
		crash, err := os.ReadFile(crashPath)
		if err != nil {
			t.Fatalf("%s: denial did not write crash.json: %v\n%s", label, err, output)
		}
		if strings.Contains(string(crash), secretBody) || strings.Contains(string(crash), "phase2e-secret-path") {
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
	if strings.TrimSpace(granted) != "phase2e-wrote" {
		t.Fatalf("granted stdout = %q, want phase2e-wrote", granted)
	}
	body, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != secretBody {
		t.Fatalf("granted file = %q, want %s", body, secretBody)
	}
}

func TestGoWriteFileMissingPathDeniesBeforeWrite(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "phase2e-secret-path.txt")
	source := `(cli_app (write_file "` + missing + `" "phase2e-secret-bytes"))`
	root := parser.NewParser(lexer.NewLexer(source), "write_file.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)

	rootDir := moduleRoot(t)
	crashPath := filepath.Join(rootDir, "crash.json")
	t.Cleanup(func() { os.Remove(crashPath) })

	denied, errText := runGenerated(t, rootDir, code, "HOWLFRAME_ALLOW_CAPS=")
	if errText == "" {
		t.Fatalf("empty grant succeeded: %s", denied)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("empty grant created %s", missing)
	}
	crash, err := os.ReadFile(crashPath)
	if err != nil {
		t.Fatalf("denial did not write crash.json: %v\n%s", err, denied)
	}
	if strings.Contains(string(crash), "phase2e-secret-path") || strings.Contains(string(crash), "phase2e-secret-bytes") {
		t.Fatalf("crash.json leaked the write: %s", crash)
	}
	if !strings.Contains(string(crash), "CAPABILITY_DENIED") || !strings.Contains(string(crash), "capability denied: filesystem") {
		t.Fatalf("crash.json = %s, want CAPABILITY_DENIED", crash)
	}
}

func TestGoMkdirRequiresGrantBeforeCreate(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "phase2e-secret-dir")
	source := `(cli_app
  (mkdir "` + missing + `")
  (print "phase2e-made"))`
	root := parser.NewParser(lexer.NewLexer(source), "mkdir.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if strings.Contains(code, `os.MkdirAll("`+missing) {
		t.Fatalf("mkdir creates at the call site:\n%s", code)
	}
	if !strings.Contains(code, `howlFrameMkdir("`+missing) {
		t.Fatalf("mkdir is not mediated:\n%s", code)
	}
	helperAt := strings.Index(code, "func howlFrameMkdir(")
	if helperAt < 0 {
		t.Fatalf("missing howlFrameMkdir helper:\n%s", code)
	}
	helper := code[helperAt:]
	denyAt := strings.Index(helper, "CAPABILITY_DENIED")
	mkdirAt := strings.Index(helper, "os.MkdirAll")
	if denyAt < 0 || mkdirAt < 0 || denyAt > mkdirAt {
		t.Fatalf("grant check must precede directory creation:\n%s", helper)
	}

	rootDir := moduleRoot(t)
	crashPath := filepath.Join(rootDir, "crash.json")
	t.Cleanup(func() { os.Remove(crashPath) })

	assertDenied := func(label, output, errText string) {
		t.Helper()
		if errText == "" {
			t.Fatalf("%s: mkdir succeeded: %s", label, output)
		}
		if strings.Contains(output, "phase2e-secret-dir") || strings.Contains(output, "phase2e-made") {
			t.Fatalf("%s: process output leaked the directory: %s", label, output)
		}
		if _, err := os.Stat(missing); !os.IsNotExist(err) {
			t.Fatalf("%s: mkdir created %s", label, missing)
		}
		crash, err := os.ReadFile(crashPath)
		if err != nil {
			t.Fatalf("%s: denial did not write crash.json: %v\n%s", label, err, output)
		}
		if strings.Contains(string(crash), "phase2e-secret-dir") {
			t.Fatalf("%s: crash.json leaked the path: %s", label, crash)
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
	if strings.TrimSpace(granted) != "phase2e-made" {
		t.Fatalf("granted stdout = %q, want phase2e-made", granted)
	}
	info, err := os.Stat(missing)
	if err != nil {
		t.Fatalf("filesystem grant did not create the directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("filesystem grant created %s, which is not a directory", missing)
	}
}

func TestGoWriteFileAndMkdirStatementsAreMediated(t *testing.T) {
	const source = `(cli_app
  (write_file "phase2e-expr.txt" "phase2e-expr")
  (mkdir "phase2e-expr-dir"))`
	root := parser.NewParser(lexer.NewLexer(source), "write_file.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if !strings.Contains(code, `howlFrameWriteFile("phase2e-expr.txt", "phase2e-expr")`) {
		t.Fatalf("write_file is not mediated:\n%s", code)
	}
	if !strings.Contains(code, `howlFrameMkdir("phase2e-expr-dir")`) {
		t.Fatalf("mkdir is not mediated:\n%s", code)
	}
	if strings.Contains(code, `os.WriteFile("phase2e-expr.txt"`) || strings.Contains(code, `os.MkdirAll("phase2e-expr-dir"`) {
		t.Fatalf("filesystem write bypasses the grant:\n%s", code)
	}
}
