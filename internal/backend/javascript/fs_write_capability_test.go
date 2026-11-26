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
	assertJSMediatedBeforeRaw(t, code, `writeFileSync("`, `howlFrameWriteFile("`, "function howlFrameWriteFile", "writeFileSync")

	assertJSCapabilityDenied(t, code, "filesystem", "the write", []string{secretBody, "phase2e-secret-path", "phase2e-wrote"}, func(label string) {
		body, err := os.ReadFile(secretPath)
		if err != nil {
			t.Fatalf("%s: seed file missing: %v", label, err)
		}
		if string(body) != kept {
			t.Fatalf("%s: file body = %q, want unchanged %s", label, body, kept)
		}
	}, [][2]string{
		{"unset grant", "__UNSET__"},
		{"empty grant", "HOWLFRAME_ALLOW_CAPS="},
		{"process grant", "HOWLFRAME_ALLOW_CAPS=process"},
	})

	assertJSGrantedStdout(t, code, "filesystem", "phase2e-wrote")
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

	assertJSCapabilityDenied(t, code, "filesystem", "the path", []string{"phase2e-secret-path", "phase2e-secret-bytes", "ENOENT"}, func(string) {
		if _, err := os.Stat(missing); !os.IsNotExist(err) {
			t.Fatalf("empty grant created %s", missing)
		}
	}, [][2]string{{"empty grant", "HOWLFRAME_ALLOW_CAPS="}})
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
	assertJSMediatedBeforeRaw(t, code, `mkdirSync("`, `howlFrameMkdir("`, "function howlFrameMkdir", "mkdirSync")

	assertJSCapabilityDenied(t, code, "filesystem", "the directory", []string{"phase2e-secret-dir", "phase2e-made", "ENOENT"}, func(label string) {
		if _, err := os.Stat(missing); !os.IsNotExist(err) {
			t.Fatalf("%s: mkdir created %s", label, missing)
		}
	}, [][2]string{
		{"unset grant", "__UNSET__"},
		{"empty grant", "HOWLFRAME_ALLOW_CAPS="},
		{"process grant", "HOWLFRAME_ALLOW_CAPS=process"},
	})

	assertJSGrantedStdout(t, code, "filesystem", "phase2e-made")
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
