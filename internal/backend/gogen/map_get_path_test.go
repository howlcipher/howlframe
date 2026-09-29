package gogen

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestGoChainedMapGetPath(t *testing.T) {
	const source = `(cli_app
  (let (row (dict ("user" (dict ("name" "ada") ("city" (dict ("id" "pdx")))))))
    (do
      (print "two" (map_get (map_get row "user") "name"))
      (print "three" (map_get (map_get (map_get row "user") "city") "id"))
      (print "leaf-miss" (map_get (map_get row "user") "missing"))
    )))`
	const want = "two ada\nthree pdx\nleaf-miss \n"

	code := generateCheckedGo(t, source)
	if !strings.Contains(code, `howlFrameMapGet(row, "user")`) {
		t.Fatalf("named map_get did not stay on the generic helper:\n%s", code)
	}
	if !strings.Contains(code, "howlFrameMapGetValue(") {
		t.Fatalf("nested map_get did not use the value helper:\n%s", code)
	}
	if !strings.Contains(code, `map[string]any{`) || !strings.Contains(code, `map[string]string{"id": "pdx"}`) {
		t.Fatalf("nested dict literal did not keep string maps inside map[string]any:\n%s", code)
	}
	if got := goRun(t, "path.go", code); got != want {
		t.Fatalf("stdout = %q, want %q\nsource:\n%s", got, want, code)
	}
}

func TestGoChainedMapGetMissingIntermediate(t *testing.T) {
	const source = `(cli_app
  (let (row (dict ("user" (dict ("name" "ada")))))
    (do
      (print "sentinel" (map_get row "missing"))
      (print "chained" (map_get (map_get row "missing") "name"))
    )))`
	code := generateCheckedGo(t, source)
	stdout, crash := goRunCrash(t, code)
	if stdout != "sentinel \n" {
		t.Fatalf("stdout = %q, want the #103 empty-string sentinel and no chained print", stdout)
	}
	if strings.Contains(stdout, "chained") {
		t.Fatalf("chained read of the sentinel printed %q", stdout)
	}
	if !strings.Contains(crash, "TYPE_ERROR: map_get expected dict, got string") {
		t.Fatalf("crash = %q", crash)
	}
}

func TestGoChainedMapGetNonDictIntermediate(t *testing.T) {
	const source = `(cli_app
  (let (row (dict ("user" "ada") ("profile" (dict ("name" "ada")))))
    (print "bad" (map_get (map_get row "user") "name"))))`
	code := generateCheckedGo(t, source)
	stdout, crash := goRunCrash(t, code)
	if strings.Contains(stdout, "bad") {
		t.Fatalf("non-dict path printed %q", stdout)
	}
	if !strings.Contains(crash, "TYPE_ERROR: map_get expected dict, got string") {
		t.Fatalf("crash = %q, stdout = %q", crash, stdout)
	}
}

func generateCheckedGo(t *testing.T, source string) string {
	t.Helper()
	root := parser.NewParser(lexer.NewLexer(source), "path.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	return code
}

// goRunCrash runs generated Go in its own module so a recovered TYPE_ERROR
// writes crash.json away from the repository.
func goRunCrash(t *testing.T, source string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	gomod := "module chainrun\n\ngo 1.21\n\nrequire github.com/howlcipher/howlframe v0.0.0\n\nreplace github.com/howlcipher/howlframe => " + moduleRoot(t) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected TYPE_ERROR, stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	crash, readErr := os.ReadFile(filepath.Join(dir, "crash.json"))
	if readErr != nil {
		t.Fatalf("missing crash.json: %v\nstdout=%q\nstderr=%q", readErr, stdout.String(), stderr.String())
	}
	return stdout.String(), string(crash)
}
