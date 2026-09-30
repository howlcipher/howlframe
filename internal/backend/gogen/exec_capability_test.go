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

func TestGoExecRequiresGrantBeforeSpawn(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "phase2b-secret-out")
	source := `(cli_app (exec "touch" "` + marker + `"))`
	root := parser.NewParser(lexer.NewLexer(source), "exec.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if strings.Contains(code, `exec.Command("touch"`) {
		t.Fatalf("exec spawns at the call site:\n%s", code)
	}
	if !strings.Contains(code, `howlFrameExec("touch", "`+marker+`")`) {
		t.Fatalf("exec is not mediated:\n%s", code)
	}
	helper := code[strings.Index(code, "func howlFrameExec"):]
	denyAt := strings.Index(helper, "CAPABILITY_DENIED")
	spawnAt := strings.Index(helper, "exec.Command")
	if denyAt < 0 || spawnAt < 0 || denyAt > spawnAt {
		t.Fatalf("grant check must precede the process spawn:\n%s", helper)
	}

	rootDir := moduleRoot(t)
	crashPath := filepath.Join(rootDir, "crash.json")
	t.Cleanup(func() { os.Remove(crashPath) })

	assertDenied := func(label, output, errText string) {
		t.Helper()
		if errText == "" {
			t.Fatalf("%s: exec succeeded: %s", label, output)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("%s: exec spawned touch before denial", label)
		}
		crash, err := os.ReadFile(crashPath)
		if err != nil {
			t.Fatalf("%s: denial did not write crash.json: %v\n%s", label, err, output)
		}
		if strings.Contains(string(crash), marker) || strings.Contains(string(crash), "phase2b-secret-out") {
			t.Fatalf("%s: crash.json leaked the command: %s", label, crash)
		}
		if strings.Contains(output, marker) || strings.Contains(output, "phase2b-secret-out") {
			t.Fatalf("%s: process output leaked the command: %s", label, output)
		}
		if !strings.Contains(string(crash), "CAPABILITY_DENIED") || !strings.Contains(string(crash), "capability denied: process") {
			t.Fatalf("%s: crash.json = %s, want CAPABILITY_DENIED", label, crash)
		}
		os.Remove(crashPath)
	}

	unsetOut, unsetErr := runGenerated(t, rootDir, code)
	assertDenied("unset grant", unsetOut, unsetErr)

	emptyOut, emptyErr := runGenerated(t, rootDir, code, "HOWLFRAME_ALLOW_CAPS=")
	assertDenied("empty grant", emptyOut, emptyErr)

	otherOut, otherErr := runGenerated(t, rootDir, code, "HOWLFRAME_ALLOW_CAPS=environment")
	assertDenied("environment grant", otherOut, otherErr)

	granted, grantErr := runGenerated(t, rootDir, code, "HOWLFRAME_ALLOW_CAPS=process")
	if grantErr != "" {
		t.Fatalf("process grant failed: %s\n%s", grantErr, granted)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("process grant did not spawn touch: %v", err)
	}
	os.Remove(marker)
}

func TestGoExecGrantedPrintsCapturedOutput(t *testing.T) {
	const source = `(cli_app
  (let (value (exec "printf" "phase2b-exec-marker"))
    (print (bytes_to_string value))))`
	root := parser.NewParser(lexer.NewLexer(source), "exec.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if !strings.Contains(code, `howlFrameExec("printf", "phase2b-exec-marker")`) {
		t.Fatalf("let-bound exec is not mediated:\n%s", code)
	}
	rootDir := moduleRoot(t)
	granted, grantErr := runGenerated(t, rootDir, code, "HOWLFRAME_ALLOW_CAPS=process")
	if grantErr != "" {
		t.Fatalf("process grant failed: %s\n%s", grantErr, granted)
	}
	if strings.TrimSpace(granted) != "phase2b-exec-marker" {
		t.Fatalf("granted stdout = %q, want phase2b-exec-marker", granted)
	}
}

func TestGoExecExpressionPositionIsMediated(t *testing.T) {
	const source = `(cli_app (print (bytes_to_string (exec "printf" "phase2b-exec-marker"))))`
	root := parser.NewParser(lexer.NewLexer(source), "exec.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if !strings.Contains(code, `howlFrameExec("printf", "phase2b-exec-marker")`) {
		t.Fatalf("expression-position exec is not mediated:\n%s", code)
	}
	if strings.Contains(code, `exec.Command("printf"`) {
		t.Fatalf("expression-position exec bypasses the grant:\n%s", code)
	}
}

func TestGoExecZeroArgsIsMediated(t *testing.T) {
	const source = `(cli_app (exec "true") (print "zero-arg-ok"))`
	root := parser.NewParser(lexer.NewLexer(source), "exec.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if !strings.Contains(code, `howlFrameExec("true")`) {
		t.Fatalf("zero-arg exec is not mediated:\n%s", code)
	}
	if strings.Contains(code, `exec.Command("true"`) {
		t.Fatalf("zero-arg exec spawns at the call site:\n%s", code)
	}
	rootDir := moduleRoot(t)
	granted, grantErr := runGenerated(t, rootDir, code, "HOWLFRAME_ALLOW_CAPS=process")
	if grantErr != "" {
		t.Fatalf("process grant failed: %s\n%s", grantErr, granted)
	}
	if strings.TrimSpace(granted) != "zero-arg-ok" {
		t.Fatalf("granted stdout = %q, want zero-arg-ok", granted)
	}
}
