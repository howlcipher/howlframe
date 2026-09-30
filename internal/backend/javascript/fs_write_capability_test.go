package javascript

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestJSWriteFileRequiresGrantBeforeWrite(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node runtime not available")
	}
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "phase2e-secret-path.txt")
	const kept = "phase2e-kept"
	const secretBody = "phase2e-secret-bytes"
	if err := os.WriteFile(secretPath, []byte(kept), 0o644); err != nil {
		t.Fatal(err)
	}
	source := `(web_app
  (write_file "` + secretPath + `" "` + secretBody + `")
  (print "phase2e-wrote"))`
	root := parser.NewParser(lexer.NewLexer(source), "write_file.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	if strings.Contains(code, `writeFileSync("`) {
		t.Fatalf("write_file writes at the call site:\n%s", code)
	}
	if !strings.Contains(code, `howlFrameWriteFile("`) {
		t.Fatalf("write_file is not mediated:\n%s", code)
	}
	helperAt := strings.Index(code, "function howlFrameWriteFile")
	if helperAt < 0 {
		t.Fatalf("missing howlFrameWriteFile helper:\n%s", code)
	}
	helper := code[helperAt:]
	denyAt := strings.Index(helper, "CAPABILITY_DENIED")
	writeAt := strings.Index(helper, "writeFileSync")
	if denyAt < 0 || writeAt < 0 || denyAt > writeAt {
		t.Fatalf("grant check must precede the filesystem write:\n%s", helper)
	}

	assertDenied := func(label, stdout, stderr string, code int) {
		t.Helper()
		if code == 0 {
			t.Fatalf("%s: write_file succeeded: %s", label, stdout+stderr)
		}
		combined := stdout + stderr
		if strings.Contains(combined, secretBody) || strings.Contains(combined, "phase2e-secret-path") || strings.Contains(combined, "phase2e-wrote") {
			t.Fatalf("%s: output leaked the write: stdout=%q stderr=%q", label, stdout, stderr)
		}
		body, err := os.ReadFile(secretPath)
		if err != nil {
			t.Fatalf("%s: seed file missing: %v", label, err)
		}
		if string(body) != kept {
			t.Fatalf("%s: file body = %q, want unchanged %s", label, body, kept)
		}
		if !strings.Contains(stderr, "CAPABILITY_DENIED") || !strings.Contains(stderr, "capability denied: filesystem") {
			t.Fatalf("%s: stderr = %q, want CAPABILITY_DENIED", label, stderr)
		}
		if strings.TrimSpace(stdout) != "" {
			t.Fatalf("%s: denied stdout = %q, want empty", label, stdout)
		}
	}

	unsetOut, unsetErr, unsetCode := runJS(t, code)
	assertDenied("unset grant", unsetOut, unsetErr, unsetCode)

	emptyOut, emptyErr, emptyCode := runJS(t, code, "HOWLFRAME_ALLOW_CAPS=")
	assertDenied("empty grant", emptyOut, emptyErr, emptyCode)

	otherOut, otherErr, otherCode := runJS(t, code, "HOWLFRAME_ALLOW_CAPS=process")
	assertDenied("process grant", otherOut, otherErr, otherCode)

	grantedOut, grantedErr, grantedCode := runJS(t, code, "HOWLFRAME_ALLOW_CAPS=filesystem")
	if grantedCode != 0 {
		t.Fatalf("filesystem grant failed (%d): %s", grantedCode, grantedErr)
	}
	if strings.TrimSpace(grantedOut) != "phase2e-wrote" {
		t.Fatalf("granted stdout = %q, want phase2e-wrote", grantedOut)
	}
	body, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != secretBody {
		t.Fatalf("granted file = %q, want %s", body, secretBody)
	}
}

func TestJSWriteFileMissingPathDeniesBeforeWrite(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node runtime not available")
	}
	missing := filepath.Join(t.TempDir(), "phase2e-secret-path.txt")
	source := `(web_app (write_file "` + missing + `" "phase2e-secret-bytes"))`
	root := parser.NewParser(lexer.NewLexer(source), "write_file.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)

	stdout, stderr, exitCode := runJS(t, code, "HOWLFRAME_ALLOW_CAPS=")
	if exitCode == 0 {
		t.Fatalf("empty grant succeeded: %s", stdout+stderr)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("empty grant created %s", missing)
	}
	combined := stdout + stderr
	if strings.Contains(combined, "phase2e-secret-path") || strings.Contains(combined, "phase2e-secret-bytes") || strings.Contains(combined, "ENOENT") {
		t.Fatalf("denial wrote the path: stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "CAPABILITY_DENIED") || !strings.Contains(stderr, "capability denied: filesystem") {
		t.Fatalf("stderr = %q, want CAPABILITY_DENIED", stderr)
	}
}

func TestJSMkdirRequiresGrantBeforeCreate(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node runtime not available")
	}
	missing := filepath.Join(t.TempDir(), "phase2e-secret-dir")
	source := `(web_app
  (mkdir "` + missing + `")
  (print "phase2e-made"))`
	root := parser.NewParser(lexer.NewLexer(source), "mkdir.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	if strings.Contains(code, `mkdirSync("`) {
		t.Fatalf("mkdir creates at the call site:\n%s", code)
	}
	if !strings.Contains(code, `howlFrameMkdir("`) {
		t.Fatalf("mkdir is not mediated:\n%s", code)
	}
	helperAt := strings.Index(code, "function howlFrameMkdir")
	if helperAt < 0 {
		t.Fatalf("missing howlFrameMkdir helper:\n%s", code)
	}
	helper := code[helperAt:]
	denyAt := strings.Index(helper, "CAPABILITY_DENIED")
	mkdirAt := strings.Index(helper, "mkdirSync")
	if denyAt < 0 || mkdirAt < 0 || denyAt > mkdirAt {
		t.Fatalf("grant check must precede directory creation:\n%s", helper)
	}

	assertDenied := func(label, stdout, stderr string, code int) {
		t.Helper()
		if code == 0 {
			t.Fatalf("%s: mkdir succeeded: %s", label, stdout+stderr)
		}
		combined := stdout + stderr
		if strings.Contains(combined, "phase2e-secret-dir") || strings.Contains(combined, "phase2e-made") || strings.Contains(combined, "ENOENT") {
			t.Fatalf("%s: output leaked the directory: stdout=%q stderr=%q", label, stdout, stderr)
		}
		if _, err := os.Stat(missing); !os.IsNotExist(err) {
			t.Fatalf("%s: mkdir created %s", label, missing)
		}
		if !strings.Contains(stderr, "CAPABILITY_DENIED") || !strings.Contains(stderr, "capability denied: filesystem") {
			t.Fatalf("%s: stderr = %q, want CAPABILITY_DENIED", label, stderr)
		}
		if strings.TrimSpace(stdout) != "" {
			t.Fatalf("%s: denied stdout = %q, want empty", label, stdout)
		}
	}

	unsetOut, unsetErr, unsetCode := runJS(t, code)
	assertDenied("unset grant", unsetOut, unsetErr, unsetCode)

	emptyOut, emptyErr, emptyCode := runJS(t, code, "HOWLFRAME_ALLOW_CAPS=")
	assertDenied("empty grant", emptyOut, emptyErr, emptyCode)

	otherOut, otherErr, otherCode := runJS(t, code, "HOWLFRAME_ALLOW_CAPS=process")
	assertDenied("process grant", otherOut, otherErr, otherCode)

	grantedOut, grantedErr, grantedCode := runJS(t, code, "HOWLFRAME_ALLOW_CAPS=filesystem")
	if grantedCode != 0 {
		t.Fatalf("filesystem grant failed (%d): %s", grantedCode, grantedErr)
	}
	if strings.TrimSpace(grantedOut) != "phase2e-made" {
		t.Fatalf("granted stdout = %q, want phase2e-made", grantedOut)
	}
	info, err := os.Stat(missing)
	if err != nil {
		t.Fatalf("filesystem grant did not create the directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("filesystem grant created %s, which is not a directory", missing)
	}
}

func TestJSWriteFileAndMkdirStatementsAreMediated(t *testing.T) {
	const source = `(web_app
  (write_file "phase2e-expr.txt" "phase2e-expr")
  (mkdir "phase2e-expr-dir"))`
	root := parser.NewParser(lexer.NewLexer(source), "write_file.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	if !strings.Contains(code, `howlFrameWriteFile("phase2e-expr.txt", "phase2e-expr")`) {
		t.Fatalf("write_file is not mediated:\n%s", code)
	}
	if !strings.Contains(code, `howlFrameMkdir("phase2e-expr-dir")`) {
		t.Fatalf("mkdir is not mediated:\n%s", code)
	}
	if strings.Contains(code, `writeFileSync("`) || strings.Contains(code, `mkdirSync("`) {
		t.Fatalf("filesystem write bypasses the grant:\n%s", code)
	}
}
