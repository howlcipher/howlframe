package javascript

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

func TestJSChainedMapGetPath(t *testing.T) {
	const source = `(web_app
  (let (row (dict ("user" (dict ("name" "ada") ("city" (dict ("id" "pdx")))))))
    (do
      (print "two" (map_get (map_get row "user") "name"))
      (print "three" (map_get (map_get (map_get row "user") "city") "id"))
      (print "leaf-miss" (map_get (map_get row "user") "missing"))
    )))`
	const want = "two ada\nthree pdx\nleaf-miss \n"

	code := generateCheckedJS(t, source)
	if !strings.Contains(code, `(row["user"] ?? "")`) {
		t.Fatalf("named map_get lost the empty-string miss:\n%s", code)
	}
	if !strings.Contains(code, "TYPE_ERROR: map_get expected dict") {
		t.Fatalf("expression map_get does not fail closed:\n%s", code)
	}
	stdout, stderr, err := runNode(t, code)
	if err != nil {
		t.Fatalf("node: %v\nstderr=%s\nsource:\n%s", err, stderr, code)
	}
	if stdout != want {
		t.Fatalf("stdout = %q, want %q\nsource:\n%s", stdout, want, code)
	}
}

func TestJSChainedMapGetMissingIntermediate(t *testing.T) {
	const source = `(web_app
  (let (row (dict ("user" (dict ("name" "ada")))))
    (do
      (print "sentinel" (map_get row "missing"))
      (print "chained" (map_get (map_get row "missing") "name"))
    )))`
	code := generateCheckedJS(t, source)
	stdout, stderr, err := runNode(t, code)
	if err == nil {
		t.Fatalf("missing intermediate succeeded, stdout=%q stderr=%q", stdout, stderr)
	}
	if stdout != "sentinel \n" {
		t.Fatalf("stdout = %q, want the #103 empty-string sentinel and no chained print", stdout)
	}
	if strings.Contains(stdout, "chained") {
		t.Fatalf("chained read of the sentinel printed %q", stdout)
	}
	if !strings.Contains(stderr, "TYPE_ERROR: map_get expected dict, got string") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestJSChainedMapGetNonDictIntermediate(t *testing.T) {
	const source = `(web_app
  (let (row (dict ("user" "ada") ("profile" (dict ("name" "ada")))))
    (print "bad" (map_get (map_get row "user") "name"))))`
	code := generateCheckedJS(t, source)
	stdout, stderr, err := runNode(t, code)
	if err == nil {
		t.Fatalf("non-dict intermediate succeeded, stdout=%q stderr=%q", stdout, stderr)
	}
	if strings.Contains(stdout, "bad") {
		t.Fatalf("non-dict path printed %q", stdout)
	}
	if !strings.Contains(stderr, "TYPE_ERROR: map_get expected dict, got string") {
		t.Fatalf("stderr = %q, stdout = %q", stderr, stdout)
	}
}

func generateCheckedJS(t *testing.T, source string) string {
	t.Helper()
	root := parser.NewParser(lexer.NewLexer(source), "path.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	return code
}

func runNode(t *testing.T, source string) (string, string, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "app.js")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
