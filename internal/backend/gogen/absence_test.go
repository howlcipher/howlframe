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

func TestGoMapGetMissIsEmptyString(t *testing.T) {
	const source = `(cli_app
  (let (strings (dict ("hit" "yes") ("empty" "")))
    (do
      (print "s-hit" (map_get strings "hit"))
      (print "s-empty" (map_get strings "empty"))
      (print "s-miss" (map_get strings "missing"))
      (print "s-empty-nil" (is_nil (map_get strings "empty")))
      (print "s-miss-nil" (is_nil (map_get strings "missing")))
    )))`
	const want = "s-hit yes\n" +
		"s-empty \n" +
		"s-miss \n" +
		"s-empty-nil false\n" +
		"s-miss-nil false\n"

	root := parser.NewParser(lexer.NewLexer(source), "absence.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if !strings.Contains(code, `howlFrameMapGet(strings, "missing")`) {
		t.Fatalf("generated Go does not route map_get through the absence helper:\n%s", code)
	}
	if strings.Contains(code, `strings["missing"]`) || strings.Contains(code, `strings["hit"]`) {
		t.Fatalf("generated Go still indexes the dict directly:\n%s", code)
	}
	helper := extractMapGetHelper(t, code)

	driver := "package main\n\nimport \"fmt\"\n\n" + helper + `
func show(label string, v any) {
	if v == nil {
		fmt.Printf("%s nil\n", label)
		return
	}
	fmt.Printf("%s %T %q\n", label, v, fmt.Sprint(v))
}

func main() {
	s := map[string]string{"hit": "yes", "empty": ""}
	show("s-hit", howlFrameMapGet(s, "hit"))
	show("s-empty", howlFrameMapGet(s, "empty"))
	show("s-miss", howlFrameMapGet(s, "miss"))
	a := map[string]any{"hit": "yes", "empty": "", "n": 7, "nested": map[string]any{"k": "v"}, "null": nil}
	show("a-hit", howlFrameMapGet(a, "hit"))
	show("a-empty", howlFrameMapGet(a, "empty"))
	show("a-miss", howlFrameMapGet(a, "miss"))
	show("a-n", howlFrameMapGet(a, "n"))
	show("a-nested", howlFrameMapGet(a, "nested"))
	show("a-null", howlFrameMapGet(a, "null"))
}
`
	const driverWant = "s-hit string \"yes\"\n" +
		"s-empty string \"\"\n" +
		"s-miss string \"\"\n" +
		"a-hit string \"yes\"\n" +
		"a-empty string \"\"\n" +
		"a-miss string \"\"\n" +
		"a-n int \"7\"\n" +
		"a-nested map[string]interface {} \"map[k:v]\"\n" +
		"a-null nil\n"
	if got := goRun(t, "driver.go", driver); got != driverWant {
		t.Fatalf("helper output = %q, want %q", got, driverWant)
	}

	if got := goRun(t, "server.go", code); got != want {
		t.Fatalf("generated program output = %q, want %q", got, want)
	}
}

func extractMapGetHelper(t *testing.T, code string) string {
	t.Helper()
	const sig = "func howlFrameMapGet["
	start := strings.Index(code, sig)
	if start < 0 {
		t.Fatalf("generated Go is missing howlFrameMapGet:\n%s", code)
	}
	rel := strings.Index(code[start:], "{")
	if rel < 0 {
		t.Fatal("howlFrameMapGet has no body")
	}
	depth := 0
	for i := start + rel; i < len(code); i++ {
		switch code[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return code[start : i+1]
			}
		}
	}
	t.Fatal("howlFrameMapGet body did not close")
	return ""
}

func goRun(t *testing.T, name string, source string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", path)
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run %s: %v\n%s", name, err, out)
	}
	return string(out)
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
