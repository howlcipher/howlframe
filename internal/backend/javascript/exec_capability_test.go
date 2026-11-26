package javascript

import (
	"github.com/howlcipher/howlframe/internal/testutil"
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
	source := testutil.CanonicalExecFixture(t, `(web_app (exec "/usr/bin/touch" "`) + marker + `"))`
	root := parser.NewParser(lexer.NewLexer(source), "exec.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	if strings.Contains(code, `spawnSync("touch"`) || strings.Contains(code, "execFileSync") {
		t.Fatalf("exec spawns at the call site:\n%s", code)
	}
	if !strings.Contains(code, testutil.CanonicalExecFixture(t, `howlFrameExec("/usr/bin/touch", ["`)+marker+`"])`) {
		t.Fatalf("exec is not mediated:\n%s", code)
	}
	helper := code[strings.Index(code, "function howlFrameExec"):]
	denyAt := strings.Index(helper, "CAPABILITY_DENIED")
	spawnAt := strings.Index(helper, "spawnSync")
	if denyAt < 0 || spawnAt < 0 || denyAt > spawnAt {
		t.Fatalf("grant check must precede the process spawn:\n%s", helper)
	}

	assertJSCapabilityDenied(t, code, "process", "the command", []string{marker, "phase2b-secret-out"}, func(label string) {
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("%s: exec spawned touch before denial", label)
		}
	}, [][2]string{
		{"unset grant", "__UNSET__"},
		{"empty grant", "HOWLFRAME_ALLOW_CAPS="},
		{"environment grant", "HOWLFRAME_ALLOW_CAPS=environment"},
		{"bare process grant (no allow-list)", "HOWLFRAME_ALLOW_CAPS=process"},
	})

	grantedOut, grantedErr, grantedCode := runJS(t, code, testutil.CanonicalExecFixture(t, "HOWLFRAME_ALLOW_CAPS=process:exec=/usr/bin/touch **,process:exec=/usr/bin/printf **,process:exec=/usr/bin/true"))
	if grantedCode != 0 {
		t.Fatalf("process grant failed (%d): %s", grantedCode, grantedErr)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("process grant did not spawn touch: %v\n%s", err, grantedOut+grantedErr)
	}
	os.Remove(marker)
}

func generateCheckedJSExec(t *testing.T, source, wantCall string) string {
	t.Helper()
	root := parser.NewParser(lexer.NewLexer(source), "exec.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	if !strings.Contains(code, wantCall) {
		t.Fatalf("exec is not mediated with %q:\n%s", wantCall, code)
	}
	return code
}

func TestJSExecGrantedPrintsCapturedOutput(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node runtime not available")
	}
	var source = testutil.CanonicalExecFixture(t, `(web_app
  (let (value (exec "/usr/bin/printf" "phase2b-exec-marker"))
    (print (bytes_to_string value))))`)
	code := generateCheckedJSExec(t, source, testutil.CanonicalExecFixture(t, `howlFrameExec("/usr/bin/printf", ["phase2b-exec-marker"])`))
	assertJSGrantedStdout(t, code, testutil.CanonicalExecFixture(t, "process:exec=/usr/bin/touch **,process:exec=/usr/bin/printf **,process:exec=/usr/bin/true"), "phase2b-exec-marker")
}

func TestJSExecExpressionPositionIsMediated(t *testing.T) {
	var source = testutil.CanonicalExecFixture(t, `(web_app (print (bytes_to_string (exec "/usr/bin/printf" "phase2b-exec-marker"))))`)
	code := generateCheckedJSExec(t, source, testutil.CanonicalExecFixture(t, `howlFrameExec("/usr/bin/printf", ["phase2b-exec-marker"])`))
	if strings.Contains(code, `spawnSync("printf"`) {
		t.Fatalf("expression-position exec bypasses the grant:\n%s", code)
	}
}

// Generated JS exec children start with an empty environment, and a "*" slot
// refuses flags, matching the bytecode runtime.
func TestJSExecEnvironmentAndFlagSlots(t *testing.T) {
	gen := func(source string) string {
		root := parser.NewParser(lexer.NewLexer(source), "exec.howl").ParseExpression()
		checker.Check(root)
		code, _ := GenerateJSCode(root)
		return code
	}
	code := gen(testutil.CanonicalExecFixture(t, `(web_app (print (bytes_to_string (exec "/usr/bin/env"))))`))
	out, errOut, exit := runJS(t, code, testutil.CanonicalExecFixture(t, "HOWLFRAME_ALLOW_CAPS=process:exec=/usr/bin/env"), "FOO=leakme")
	if exit != 0 || strings.Contains(out, "leakme") || strings.TrimSpace(out) != "" {
		t.Fatalf("default env leaked: %d out %q err %q", exit, out, errOut)
	}
	out, errOut, exit = runJS(t, code, testutil.CanonicalExecFixture(t, "HOWLFRAME_ALLOW_CAPS=process:exec=/usr/bin/env,process:env=FOO"), "FOO=leakme")
	if exit != 0 || strings.TrimSpace(out) != "FOO=leakme" {
		t.Fatalf("granted env: %d out %q err %q", exit, out, errOut)
	}
	code = gen(testutil.CanonicalExecFixture(t, `(web_app (print (bytes_to_string (exec "/usr/bin/printf" "%s" "-v"))))`))
	if _, _, exit := runJS(t, code, testutil.CanonicalExecFixture(t, "HOWLFRAME_ALLOW_CAPS=process:exec=/usr/bin/printf %s *")); exit == 0 {
		t.Fatal("a * slot matched a flag")
	}
	if _, _, exit := runJS(t, code, testutil.CanonicalExecFixture(t, "HOWLFRAME_ALLOW_CAPS=process:exec=/usr/bin/printf ** %s")); exit == 0 {
		t.Fatal("a non-final ** grant was accepted")
	}
}
