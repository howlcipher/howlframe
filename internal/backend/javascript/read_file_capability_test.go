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

func TestJSReadFileRequiresGrantBeforeRead(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node runtime not available")
	}
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "phase2c-secret-path.txt")
	const secretBody = "phase2c-secret-bytes"
	if err := os.WriteFile(secretPath, []byte(secretBody), 0o644); err != nil {
		t.Fatal(err)
	}
	source := `(web_app
  (let (value (read_file "` + secretPath + `"))
    (print (bytes_to_string value))))`
	root := parser.NewParser(lexer.NewLexer(source), "read_file.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	if strings.Contains(code, `readFileSync("`) {
		t.Fatalf("read_file reads at the call site:\n%s", code)
	}
	if !strings.Contains(code, `howlFrameReadFile("`) {
		t.Fatalf("read_file is not mediated:\n%s", code)
	}
	helperAt := strings.Index(code, "function howlFrameReadFile")
	if helperAt < 0 {
		t.Fatalf("missing howlFrameReadFile helper:\n%s", code)
	}
	helper := code[helperAt:]
	denyAt := strings.Index(helper, "CAPABILITY_DENIED")
	readAt := strings.Index(helper, "readFileSync")
	if denyAt < 0 || readAt < 0 || denyAt > readAt {
		t.Fatalf("grant check must precede the filesystem read:\n%s", helper)
	}

	assertDenied := func(label, stdout, stderr string, code int) {
		t.Helper()
		if code == 0 {
			t.Fatalf("%s: read_file succeeded: %s", label, stdout+stderr)
		}
		combined := stdout + stderr
		if strings.Contains(combined, secretBody) || strings.Contains(combined, "phase2c-secret-path") {
			t.Fatalf("%s: output leaked the file: stdout=%q stderr=%q", label, stdout, stderr)
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
	if strings.TrimSpace(grantedOut) != secretBody {
		t.Fatalf("granted stdout = %q, want %s", grantedOut, secretBody)
	}
}

func TestJSReadFileMissingPathDeniesBeforeRead(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node runtime not available")
	}
	missing := filepath.Join(t.TempDir(), "phase2c-secret-path.txt")
	source := `(web_app (print (bytes_to_string (read_file "` + missing + `"))))`
	root := parser.NewParser(lexer.NewLexer(source), "read_file.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)

	stdout, stderr, exitCode := runJS(t, code, "HOWLFRAME_ALLOW_CAPS=")
	if exitCode == 0 {
		t.Fatalf("empty grant succeeded: %s", stdout+stderr)
	}
	combined := stdout + stderr
	if strings.Contains(combined, "phase2c-secret-path") || strings.Contains(combined, "ENOENT") {
		t.Fatalf("denial read the path: stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "CAPABILITY_DENIED") || !strings.Contains(stderr, "capability denied: filesystem") {
		t.Fatalf("stderr = %q, want CAPABILITY_DENIED", stderr)
	}
}

func TestJSReadFileExpressionPositionIsMediated(t *testing.T) {
	const source = `(web_app (print (bytes_to_string (read_file "phase2c-expr.txt"))))`
	root := parser.NewParser(lexer.NewLexer(source), "read_file.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	if !strings.Contains(code, `howlFrameReadFile("phase2c-expr.txt")`) {
		t.Fatalf("expression-position read_file is not mediated:\n%s", code)
	}
	if strings.Contains(code, `readFileSync("`) {
		t.Fatalf("expression-position read_file bypasses the grant:\n%s", code)
	}
}
