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

func TestJSMapGetMissIsEmptyString(t *testing.T) {
	const source = `(web_app
  (let (strings (dict ("hit" "yes") ("empty" "")))
    (let (raw "{\"hit\":\"yes\",\"empty\":\"\",\"n\":7,\"nested\":{\"k\":\"v\"}}")
      (let (records (parse_json Any raw))
        (let (nested (map_get records "nested"))
          (do
            (print "s-hit" (map_get strings "hit"))
            (print "s-empty" (map_get strings "empty"))
            (print "s-miss" (map_get strings "missing"))
            (print "s-empty-nil" (is_nil (map_get strings "empty")))
            (print "s-miss-nil" (is_nil (map_get strings "missing")))
            (print "a-hit" (map_get records "hit"))
            (print "a-n" (map_get records "n"))
            (print "a-empty" (map_get records "empty"))
            (print "a-miss" (map_get records "missing"))
            (print "a-empty-nil" (is_nil (map_get records "empty")))
            (print "a-miss-nil" (is_nil (map_get records "missing")))
            (print "nested" (map_get nested "k"))
            (print "nested-miss" (map_get nested "missing"))
            (print "nested-miss-nil" (is_nil (map_get nested "missing")))
          ))))))`
	const want = "s-hit yes\n" +
		"s-empty \n" +
		"s-miss \n" +
		"s-empty-nil false\n" +
		"s-miss-nil false\n" +
		"a-hit yes\n" +
		"a-n 7\n" +
		"a-empty \n" +
		"a-miss \n" +
		"a-empty-nil false\n" +
		"a-miss-nil false\n" +
		"nested v\n" +
		"nested-miss \n" +
		"nested-miss-nil false\n"

	root := parser.NewParser(lexer.NewLexer(source), "absence.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	if !strings.Contains(code, `(strings["missing"] ?? "")`) {
		t.Fatalf("generated JavaScript does not coerce a missing map_get to \"\":\n%s", code)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "app.js")
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", path).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s\n\nsource:\n%s", err, out, code)
	}
	if string(out) != want {
		t.Fatalf("stdout = %q, want %q\nsource:\n%s", string(out), want, code)
	}
}
