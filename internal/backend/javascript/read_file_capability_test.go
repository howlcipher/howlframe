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
	assertJSMediatedBeforeRaw(t, code, `readFileSync("`, `howlFrameReadFile("`, "function howlFrameReadFile", "readFileSync")

	assertJSCapabilityDenied(t, code, "filesystem", "the file", []string{secretBody, "phase2c-secret-path"}, nil, [][2]string{
		{"unset grant", "__UNSET__"},
		{"empty grant", "HOWLFRAME_ALLOW_CAPS="},
		{"process grant", "HOWLFRAME_ALLOW_CAPS=process"},
	})

	assertJSGrantedStdout(t, code, "filesystem", secretBody)
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

	assertJSCapabilityDenied(t, code, "filesystem", "the path", []string{"phase2c-secret-path", "ENOENT"}, nil, [][2]string{{"empty grant", "HOWLFRAME_ALLOW_CAPS="}})
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
