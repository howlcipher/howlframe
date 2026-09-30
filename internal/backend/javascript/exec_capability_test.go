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

func TestJSExecRequiresGrantBeforeSpawn(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node runtime not available")
	}
	marker := filepath.Join(t.TempDir(), "phase2b-secret-out")
	source := `(web_app (exec "touch" "` + marker + `"))`
	root := parser.NewParser(lexer.NewLexer(source), "exec.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	if strings.Contains(code, `spawnSync("touch"`) || strings.Contains(code, "execFileSync") {
		t.Fatalf("exec spawns at the call site:\n%s", code)
	}
	if !strings.Contains(code, `howlFrameExec("touch", ["`+marker+`"])`) {
		t.Fatalf("exec is not mediated:\n%s", code)
	}
	helper := code[strings.Index(code, "function howlFrameExec"):]
	denyAt := strings.Index(helper, "CAPABILITY_DENIED")
	spawnAt := strings.Index(helper, "spawnSync")
	if denyAt < 0 || spawnAt < 0 || denyAt > spawnAt {
		t.Fatalf("grant check must precede the process spawn:\n%s", helper)
	}

	assertDenied := func(label, stdout, stderr string, code int) {
		t.Helper()
		if code == 0 {
			t.Fatalf("%s: exec succeeded: %s", label, stdout+stderr)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("%s: exec spawned touch before denial", label)
		}
		combined := stdout + stderr
		if strings.Contains(combined, marker) || strings.Contains(combined, "phase2b-secret-out") {
			t.Fatalf("%s: output leaked the command: stdout=%q stderr=%q", label, stdout, stderr)
		}
		if !strings.Contains(stderr, "CAPABILITY_DENIED") || !strings.Contains(stderr, "capability denied: process") {
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

	otherOut, otherErr, otherCode := runJS(t, code, "HOWLFRAME_ALLOW_CAPS=environment")
	assertDenied("environment grant", otherOut, otherErr, otherCode)

	grantedOut, grantedErr, grantedCode := runJS(t, code, "HOWLFRAME_ALLOW_CAPS=process")
	if grantedCode != 0 {
		t.Fatalf("process grant failed (%d): %s", grantedCode, grantedErr)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("process grant did not spawn touch: %v\n%s", err, grantedOut+grantedErr)
	}
	os.Remove(marker)
}

func TestJSExecGrantedPrintsCapturedOutput(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node runtime not available")
	}
	const source = `(web_app
  (let (value (exec "printf" "phase2b-exec-marker"))
    (print (bytes_to_string value))))`
	root := parser.NewParser(lexer.NewLexer(source), "exec.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	if !strings.Contains(code, `howlFrameExec("printf", ["phase2b-exec-marker"])`) {
		t.Fatalf("let-bound exec is not mediated:\n%s", code)
	}
	grantedOut, grantedErr, grantedCode := runJS(t, code, "HOWLFRAME_ALLOW_CAPS=process")
	if grantedCode != 0 {
		t.Fatalf("process grant failed (%d): %s", grantedCode, grantedErr)
	}
	if strings.TrimSpace(grantedOut) != "phase2b-exec-marker" {
		t.Fatalf("granted stdout = %q, want phase2b-exec-marker", grantedOut)
	}
}

func TestJSExecExpressionPositionIsMediated(t *testing.T) {
	const source = `(web_app (print (bytes_to_string (exec "printf" "phase2b-exec-marker"))))`
	root := parser.NewParser(lexer.NewLexer(source), "exec.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	if !strings.Contains(code, `howlFrameExec("printf", ["phase2b-exec-marker"])`) {
		t.Fatalf("expression-position exec is not mediated:\n%s", code)
	}
	if strings.Contains(code, `spawnSync("printf"`) {
		t.Fatalf("expression-position exec bypasses the grant:\n%s", code)
	}
}
