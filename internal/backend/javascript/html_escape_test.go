package javascript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/htmlescape"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestJSHTMLEscapeMatchesBytecodeContract(t *testing.T) {
	cli := string(mustReadRepoJS(t, "tests/parity/13_html_escape.howl"))
	source := strings.Replace(cli, "(cli_app", "(web_app", 1)
	code := generateCheckedJS(t, source)
	if !strings.Contains(code, "howlFrameHTMLEscape") || !strings.Contains(code, "&#34;") {
		t.Fatalf("generated JS does not encode the five characters:\n%s", code)
	}
	if !strings.Contains(code, `howlFrameHTMLEscape("html_escape"`) || !strings.Contains(code, `howlFrameHTMLEscape("attr_escape"`) {
		t.Fatalf("generated JS dropped an escape kind:\n%s", code)
	}
	names, violation, found := htmlescape.InspectBackendMarkup(code)
	if !found {
		t.Fatalf("generated JS has no markup list:\n%s", code)
	}
	if violation != "" {
		t.Fatalf("generated JS interpolates a handler: %s\n%s", violation, code)
	}
	if len(names) != 1 || names[0] != "onTask" {
		t.Fatalf("handler names = %#v, want constant onTask\n%s", names, code)
	}
	stdout, stderr, err := runNode(t, code)
	if err != nil {
		t.Fatalf("node: %v\nstderr=%s\nsource:\n%s", err, stderr, code)
	}
	if stdout != htmlEscapeStdoutJS {
		t.Fatalf("stdout = %q, want %q\nsource:\n%s", stdout, htmlEscapeStdoutJS, code)
	}
	if strings.Contains(stdout, "a<b") || strings.Contains(stdout, "<script") {
		t.Fatalf("raw markup survived encoding:\n%s", stdout)
	}
}

func TestJSHTMLEscapeRejectsNonString(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{name: "list", source: `(web_app (print (html_escape (list "a"))))`, want: "TYPE_ERROR: html_escape expected string"},
		{name: "number", source: `(web_app (print (attr_escape 1)))`, want: "TYPE_ERROR: attr_escape expected string"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code := generateCheckedJS(t, tc.source)
			stdout, stderr, err := runNode(t, code)
			if err == nil {
				t.Fatalf("non-string succeeded, stdout=%q stderr=%q", stdout, stderr)
			}
			if strings.TrimSpace(stdout) != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Fatalf("stderr = %q", stderr)
			}
		})
	}
}

func TestJSHTMLEscapeCheckerAcceptsWebApp(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(web_app (print (html_escape "a&b") (attr_escape "c<d")))`), "escape.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	if strings.Contains(code, "Unknown statement") {
		t.Fatalf("escape was unknown to the JS checker:\n%s", code)
	}
}

const htmlEscapeStdoutJS = "&amp;\n" +
	"&lt;\n" +
	"&gt;\n" +
	"&#34;\n" +
	"&#39;\n" +
	"&amp;&lt;&gt;&#34;&#39;\n" +
	"&amp;&lt;&gt;&#34;&#39;\n" +
	"[]\n" +
	"<button data-id=\"a&lt;b&amp;c&#34;d&#39;e\" onclick=\"onTask\">x&lt;y&amp;z</button>\n"

func mustReadRepoJS(t *testing.T, path string) []byte {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			data, readErr := os.ReadFile(filepath.Join(dir, path))
			if readErr != nil {
				t.Fatal(readErr)
			}
			return data
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
