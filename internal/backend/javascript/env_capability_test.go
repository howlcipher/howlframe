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

func TestJSEnvRequiresGrantBeforeRead(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node runtime not available")
	}
	const source = `(web_app
  (let (value (env "HOWLFRAME_ABI_SECRET"))
    (print value)))`
	root := parser.NewParser(lexer.NewLexer(source), "env.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	if strings.Contains(code, `process.env["HOWLFRAME_ABI_SECRET"]`) || strings.Contains(code, "process.env.HOWLFRAME_ABI_SECRET") {
		t.Fatalf("env reads the secret key before mediation:\n%s", code)
	}
	if !strings.Contains(code, `howlFrameEnv("HOWLFRAME_ABI_SECRET")`) {
		t.Fatalf("env is not mediated:\n%s", code)
	}
	helper := code[strings.Index(code, "function howlFrameEnv"):]
	denyAt := strings.Index(helper, "CAPABILITY_DENIED")
	readAt := strings.Index(helper, "process.env[key]")
	if denyAt < 0 || readAt < 0 || denyAt > readAt {
		t.Fatalf("grant check must precede the environment read:\n%s", helper)
	}

	deniedOut, deniedErr, deniedCode := runJS(t, code, "HOWLFRAME_ALLOW_CAPS=", "HOWLFRAME_ABI_SECRET=phase1-token")
	if deniedCode == 0 {
		t.Fatalf("empty grant succeeded: %s", deniedOut+deniedErr)
	}
	if strings.Contains(deniedOut+deniedErr, "phase1-token") {
		t.Fatalf("empty grant leaked the secret: stdout=%q stderr=%q", deniedOut, deniedErr)
	}
	if !strings.Contains(deniedErr, "CAPABILITY_DENIED") || !strings.Contains(deniedErr, "capability denied: environment") {
		t.Fatalf("stderr = %q, want CAPABILITY_DENIED", deniedErr)
	}
	if strings.TrimSpace(deniedOut) != "" {
		t.Fatalf("denied stdout = %q, want empty", deniedOut)
	}

	otherOut, otherErr, otherCode := runJS(t, code, "HOWLFRAME_ALLOW_CAPS=network", "HOWLFRAME_ABI_SECRET=phase1-token")
	if otherCode == 0 {
		t.Fatalf("network grant succeeded for env: %s", otherOut+otherErr)
	}
	if strings.Contains(otherOut+otherErr, "phase1-token") {
		t.Fatalf("network grant leaked the secret: stdout=%q stderr=%q", otherOut, otherErr)
	}

	grantedOut, grantedErr, grantedCode := runJS(t, code, "HOWLFRAME_ALLOW_CAPS=environment", "HOWLFRAME_ABI_SECRET=phase1-token")
	if grantedCode != 0 {
		t.Fatalf("environment grant failed (%d): %s", grantedCode, grantedErr)
	}
	if strings.TrimSpace(grantedOut) != "phase1-token" {
		t.Fatalf("granted stdout = %q, want phase1-token", grantedOut)
	}
}

func runJS(t *testing.T, source string, envPairs ...string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "app.js")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", path)
	filtered := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "HOWLFRAME_ALLOW_CAPS=") || strings.HasPrefix(entry, "HOWLFRAME_ABI_SECRET=") {
			continue
		}
		filtered = append(filtered, entry)
	}
	cmd.Env = append(filtered, envPairs...)
	out, err := cmd.Output()
	stderr := ""
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
			stderr = string(exitErr.Stderr)
		} else {
			t.Fatal(err)
		}
	}
	return string(out), stderr, code
}
