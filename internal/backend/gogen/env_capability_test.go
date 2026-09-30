package gogen

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

func TestGoEnvRequiresGrantBeforeRead(t *testing.T) {
	const source = `(cli_app
  (let (value (env "HOWLFRAME_ABI_SECRET"))
    (print value)))`
	root := parser.NewParser(lexer.NewLexer(source), "env.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if strings.Contains(code, `os.Getenv("HOWLFRAME_ABI_SECRET")`) {
		t.Fatalf("env still reads the key with os.Getenv:\n%s", code)
	}
	if !strings.Contains(code, `howlFrameEnv("HOWLFRAME_ABI_SECRET")`) {
		t.Fatalf("env is not mediated:\n%s", code)
	}
	helper := code[strings.Index(code, "func howlFrameEnv"):]
	denyAt := strings.Index(helper, "CAPABILITY_DENIED")
	readAt := strings.Index(helper, "return os.Getenv(key)")
	if denyAt < 0 || readAt < 0 || denyAt > readAt {
		t.Fatalf("grant check must precede the environment read:\n%s", helper)
	}

	rootDir := moduleRoot(t)
	crashPath := filepath.Join(rootDir, "crash.json")
	t.Cleanup(func() { os.Remove(crashPath) })

	denied, errText := runGenerated(t, rootDir, code, "HOWLFRAME_ALLOW_CAPS=", "HOWLFRAME_ABI_SECRET=phase1-token")
	if errText == "" {
		t.Fatalf("empty grant succeeded: %s", denied)
	}
	if strings.Contains(denied, "phase1-token") {
		t.Fatalf("empty grant leaked the secret in process output: %s", denied)
	}
	crash, err := os.ReadFile(crashPath)
	if err != nil {
		t.Fatalf("denial did not write crash.json: %v\n%s", err, denied)
	}
	if strings.Contains(string(crash), "phase1-token") {
		t.Fatalf("crash.json leaked the secret: %s", crash)
	}
	if !strings.Contains(string(crash), "CAPABILITY_DENIED") || !strings.Contains(string(crash), "capability denied: environment") {
		t.Fatalf("crash.json = %s, want CAPABILITY_DENIED", crash)
	}
	os.Remove(crashPath)

	other, otherErr := runGenerated(t, rootDir, code, "HOWLFRAME_ALLOW_CAPS=network", "HOWLFRAME_ABI_SECRET=phase1-token")
	if otherErr == "" {
		t.Fatalf("network grant succeeded for env: %s", other)
	}
	if strings.Contains(other, "phase1-token") {
		t.Fatalf("network grant leaked the secret: %s", other)
	}
	os.Remove(crashPath)

	granted, grantErr := runGenerated(t, rootDir, code, "HOWLFRAME_ALLOW_CAPS=environment", "HOWLFRAME_ABI_SECRET=phase1-token")
	if grantErr != "" {
		t.Fatalf("environment grant failed: %s\n%s", grantErr, granted)
	}
	if strings.TrimSpace(granted) != "phase1-token" {
		t.Fatalf("granted stdout = %q, want phase1-token", granted)
	}
}

func TestGoEnvExpressionPositionIsMediated(t *testing.T) {
	const source = `(cli_app (print (env "HOWLFRAME_ABI_SECRET")))`
	root := parser.NewParser(lexer.NewLexer(source), "env.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if !strings.Contains(code, `howlFrameEnv("HOWLFRAME_ABI_SECRET")`) {
		t.Fatalf("expression-position env is not mediated:\n%s", code)
	}
	if strings.Contains(code, `os.Getenv("HOWLFRAME_ABI_SECRET")`) {
		t.Fatalf("expression-position env bypasses the grant:\n%s", code)
	}
}

func runGenerated(t *testing.T, rootDir, source string, envPairs ...string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "server.go")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", path)
	cmd.Dir = rootDir
	env := os.Environ()
	filtered := env[:0]
	for _, entry := range env {
		if strings.HasPrefix(entry, "HOWLFRAME_ALLOW_CAPS=") || strings.HasPrefix(entry, "HOWLFRAME_ABI_SECRET=") {
			continue
		}
		filtered = append(filtered, entry)
	}
	cmd.Env = append(filtered, envPairs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err.Error()
	}
	return string(out), ""
}
