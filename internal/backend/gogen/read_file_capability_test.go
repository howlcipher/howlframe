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
	assertGoCapabilityDenied(t, rootDir, code, "filesystem", "the file", []string{secretBody, "phase2c-secret-path"}, nil, [][2]string{
		{"unset grant", "__UNSET__"},
		{"empty grant", "HOWLFRAME_ALLOW_CAPS="},
		{"process grant", "HOWLFRAME_ALLOW_CAPS=process"},
	})

	assertGoGrantedStdout(t, rootDir, code, "filesystem", secretBody)
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
	assertGoCapabilityDenied(t, rootDir, code, "filesystem", "the IO catch path", []string{"phase2c-secret-path", "caught-io", "read-ok"}, nil, [][2]string{{"empty grant", "HOWLFRAME_ALLOW_CAPS="}})

	assertGoGrantedStdout(t, rootDir, code, "filesystem", "caught-io")
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
