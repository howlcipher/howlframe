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

func TestGoWriteFileRequiresGrantBeforeWrite(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "phase2e-secret-path.txt")
	const kept = "phase2e-kept"
	const secretBody = "phase2e-secret-bytes"
	if err := os.WriteFile(secretPath, []byte(kept), 0o644); err != nil {
		t.Fatal(err)
	}
	source := `(cli_app
  (write_file "` + secretPath + `" "` + secretBody + `")
  (print "phase2e-wrote"))`
	root := parser.NewParser(lexer.NewLexer(source), "write_file.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	assertGoMediatedBeforeRaw(t, code, `os.WriteFile("`+secretPath, `howlFrameWriteFile("`+secretPath, "func howlFrameWriteFile(", "os.WriteFile")

	rootDir := moduleRoot(t)
	assertGoCapabilityDenied(t, rootDir, code, "filesystem", "the write", []string{secretBody, "phase2e-secret-path", "phase2e-wrote"}, func(label string) {
		body, err := os.ReadFile(secretPath)
		if err != nil {
			t.Fatalf("%s: seed file missing: %v", label, err)
		}
		if string(body) != kept {
			t.Fatalf("%s: file body = %q, want unchanged %s", label, body, kept)
		}
	}, standardGoFSGrants)

	assertGoGrantedStdout(t, rootDir, code, "filesystem", "phase2e-wrote")
	body, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != secretBody {
		t.Fatalf("granted file = %q, want %s", body, secretBody)
	}
}

func TestGoWriteFileMissingPathDeniesBeforeWrite(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "phase2e-secret-path.txt")
	source := `(cli_app (write_file "` + missing + `" "phase2e-secret-bytes"))`
	root := parser.NewParser(lexer.NewLexer(source), "write_file.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)

	rootDir := moduleRoot(t)
	assertGoCapabilityDenied(t, rootDir, code, "filesystem", "the write", []string{"phase2e-secret-path", "phase2e-secret-bytes"}, func(string) {
		if _, err := os.Stat(missing); !os.IsNotExist(err) {
			t.Fatalf("empty grant created %s", missing)
		}
	}, [][2]string{{"empty grant", "HOWLFRAME_ALLOW_CAPS="}})
}

func TestGoMkdirRequiresGrantBeforeCreate(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "phase2e-secret-dir")
	source := `(cli_app
  (mkdir "` + missing + `")
  (print "phase2e-made"))`
	root := parser.NewParser(lexer.NewLexer(source), "mkdir.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	assertGoMediatedBeforeRaw(t, code, `os.MkdirAll("`+missing, `howlFrameMkdir("`+missing, "func howlFrameMkdir(", "os.MkdirAll")

	rootDir := moduleRoot(t)
	assertGoCapabilityDenied(t, rootDir, code, "filesystem", "the directory", []string{"phase2e-secret-dir", "phase2e-made"}, func(label string) {
		if _, err := os.Stat(missing); !os.IsNotExist(err) {
			t.Fatalf("%s: mkdir created %s", label, missing)
		}
	}, standardGoFSGrants)

	assertGoGrantedStdout(t, rootDir, code, "filesystem", "phase2e-made")
	info, err := os.Stat(missing)
	if err != nil {
		t.Fatalf("filesystem grant did not create the directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("filesystem grant created %s, which is not a directory", missing)
	}
}

func TestGoWriteFileAndMkdirStatementsAreMediated(t *testing.T) {
	const source = `(cli_app
  (write_file "phase2e-expr.txt" "phase2e-expr")
  (mkdir "phase2e-expr-dir"))`
	root := parser.NewParser(lexer.NewLexer(source), "write_file.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if !strings.Contains(code, `howlFrameWriteFile("phase2e-expr.txt", "phase2e-expr")`) {
		t.Fatalf("write_file is not mediated:\n%s", code)
	}
	if !strings.Contains(code, `howlFrameMkdir("phase2e-expr-dir")`) {
		t.Fatalf("mkdir is not mediated:\n%s", code)
	}
	if strings.Contains(code, `os.WriteFile("phase2e-expr.txt"`) || strings.Contains(code, `os.MkdirAll("phase2e-expr-dir"`) {
		t.Fatalf("filesystem write bypasses the grant:\n%s", code)
	}
}
