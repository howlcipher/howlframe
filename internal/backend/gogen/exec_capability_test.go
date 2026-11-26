package gogen

import (
	"github.com/howlcipher/howlframe/internal/testutil"
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
	source := testutil.CanonicalExecFixture(t, `(cli_app (exec "/usr/bin/touch" "`) + marker + `"))`
	root := parser.NewParser(lexer.NewLexer(source), "exec.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if strings.Contains(code, testutil.CanonicalExecFixture(t, `exec.Command("/usr/bin/touch"`)) {
		t.Fatalf("exec spawns at the call site:\n%s", code)
	}
	if !strings.Contains(code, testutil.CanonicalExecFixture(t, `howlFrameExec("/usr/bin/touch", "`)+marker+`")`) {
		t.Fatalf("exec is not mediated:\n%s", code)
	}
	helper := code[strings.Index(code, "func howlFrameExec"):]
	denyAt := strings.Index(helper, "CAPABILITY_DENIED")
	spawnAt := strings.Index(helper, "exec.Command")
	if denyAt < 0 || spawnAt < 0 || denyAt > spawnAt {
		t.Fatalf("grant check must precede the process spawn:\n%s", helper)
	}

	rootDir := moduleRoot(t)
	assertGoCapabilityDenied(t, rootDir, code, "process", "the command", []string{marker, "phase2b-secret-out"}, func(label string) {
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("%s: exec spawned touch before denial", label)
		}
	}, [][2]string{
		{"unset grant", "__UNSET__"},
		{"empty grant", "HOWLFRAME_ALLOW_CAPS="},
		{"environment grant", "HOWLFRAME_ALLOW_CAPS=environment"},
		{"bare process grant (no allow-list)", "HOWLFRAME_ALLOW_CAPS=process"},
	})

	granted, grantErr := runGenerated(t, rootDir, code, testutil.CanonicalExecFixture(t, "HOWLFRAME_ALLOW_CAPS=process:exec=/usr/bin/touch **,process:exec=/usr/bin/printf **,process:exec=/usr/bin/true"))
	if grantErr != "" {
		t.Fatalf("process grant failed: %s\n%s", grantErr, granted)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("process grant did not spawn touch: %v", err)
	}
	os.Remove(marker)
}

func assertGoExecMediated(t *testing.T, source, wantCall string) string {
	t.Helper()
	root := parser.NewParser(lexer.NewLexer(source), "exec.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if !strings.Contains(code, wantCall) {
		t.Fatalf("exec is not mediated with %q:\n%s", wantCall, code)
	}
	return code
}

func TestGoExecGrantedPrintsCapturedOutput(t *testing.T) {
	var source = testutil.CanonicalExecFixture(t, `(cli_app
  (let (value (exec "/usr/bin/printf" "phase2b-exec-marker"))
    (print (bytes_to_string value))))`)
	code := assertGoExecMediated(t, source, testutil.CanonicalExecFixture(t, `howlFrameExec("/usr/bin/printf", "phase2b-exec-marker")`))
	assertGoGrantedStdout(t, moduleRoot(t), code, testutil.CanonicalExecFixture(t, "process:exec=/usr/bin/touch **,process:exec=/usr/bin/printf **,process:exec=/usr/bin/true"), "phase2b-exec-marker")
}

func TestGoExecExpressionPositionIsMediated(t *testing.T) {
	var source = testutil.CanonicalExecFixture(t, `(cli_app (print (bytes_to_string (exec "/usr/bin/printf" "phase2b-exec-marker"))))`)
	code := assertGoExecMediated(t, source, testutil.CanonicalExecFixture(t, `howlFrameExec("/usr/bin/printf", "phase2b-exec-marker")`))
	if strings.Contains(code, testutil.CanonicalExecFixture(t, `exec.Command("/usr/bin/printf"`)) {
		t.Fatalf("expression-position exec bypasses the grant:\n%s", code)
	}
}

func TestGoExecZeroArgsIsMediated(t *testing.T) {
	var source = testutil.CanonicalExecFixture(t, `(cli_app (exec "/usr/bin/true") (print "zero-arg-ok"))`)
	code := assertGoExecMediated(t, source, testutil.CanonicalExecFixture(t, `howlFrameExec("/usr/bin/true")`))
	if strings.Contains(code, testutil.CanonicalExecFixture(t, `exec.Command("/usr/bin/true"`)) {
		t.Fatalf("zero-arg exec spawns at the call site:\n%s", code)
	}
	assertGoGrantedStdout(t, moduleRoot(t), code, testutil.CanonicalExecFixture(t, "process:exec=/usr/bin/touch **,process:exec=/usr/bin/printf **,process:exec=/usr/bin/true"), "zero-arg-ok")
}

// Generated Go exec children start with an empty environment, and a "*" slot
// refuses flags, matching the bytecode runtime.
func TestGoExecEnvironmentAndFlagSlots(t *testing.T) {
	var source = testutil.CanonicalExecFixture(t, `(cli_app (print (bytes_to_string (exec "/usr/bin/env"))))`)
	root := parser.NewParser(lexer.NewLexer(source), "exec.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	rootDir := moduleRoot(t)
	out, errOut := runGenerated(t, rootDir, code, testutil.CanonicalExecFixture(t, "HOWLFRAME_ALLOW_CAPS=process:exec=/usr/bin/env"), "FOO=leakme")
	if errOut != "" || strings.Contains(out, "leakme") || strings.TrimSpace(out) != "" {
		t.Fatalf("default env leaked: out %q err %q", out, errOut)
	}
	out, errOut = runGenerated(t, rootDir, code, testutil.CanonicalExecFixture(t, "HOWLFRAME_ALLOW_CAPS=process:exec=/usr/bin/env,process:env=FOO"), "FOO=leakme")
	if errOut != "" || strings.TrimSpace(out) != "FOO=leakme" {
		t.Fatalf("granted env: out %q err %q", out, errOut)
	}

	var flagSource = testutil.CanonicalExecFixture(t, `(cli_app (print (bytes_to_string (exec "/usr/bin/printf" "%s" "-v"))))`)
	root = parser.NewParser(lexer.NewLexer(flagSource), "exec.howl").ParseExpression()
	checker.Check(root)
	code, _ = GenerateCode(root)
	if _, errOut := runGenerated(t, rootDir, code, testutil.CanonicalExecFixture(t, "HOWLFRAME_ALLOW_CAPS=process:exec=/usr/bin/printf %s *")); errOut == "" {
		t.Fatal("a * slot matched a flag")
	}
	if _, errOut := runGenerated(t, rootDir, code, testutil.CanonicalExecFixture(t, "HOWLFRAME_ALLOW_CAPS=process:exec=/usr/bin/printf ** %s")); errOut == "" {
		t.Fatal("a non-final ** grant was accepted")
	}
}
