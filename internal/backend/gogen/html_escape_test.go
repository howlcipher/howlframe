package gogen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/htmlescape"
)

func TestGoHTMLEscapeMatchesBytecodeContract(t *testing.T) {
	source := string(mustReadRepo(t, "tests/parity/13_html_escape.howl"))
	code := generateCheckedGo(t, source)
	if !strings.Contains(code, "html.EscapeString") {
		t.Fatalf("generated Go does not use html.EscapeString:\n%s", code)
	}
	if !strings.Contains(code, `howlFrameHTMLEscape("html_escape"`) || !strings.Contains(code, `howlFrameHTMLEscape("attr_escape"`) {
		t.Fatalf("generated Go dropped an escape kind:\n%s", code)
	}
	names, violation, found := htmlescape.InspectBackendMarkup(code)
	if !found {
		t.Fatalf("generated Go has no markup list:\n%s", code)
	}
	if violation != "" {
		t.Fatalf("generated Go interpolates a handler: %s\n%s", violation, code)
	}
	if len(names) != 1 || names[0] != "onTask" {
		t.Fatalf("handler names = %#v, want constant onTask\n%s", names, code)
	}
	if got := goRun(t, "escape.go", code); got != htmlEscapeStdout {
		t.Fatalf("stdout = %q, want %q\nsource:\n%s", got, htmlEscapeStdout, code)
	}
}

func TestGoHTMLEscapeRejectsNonString(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{name: "list", source: `(cli_app (print (html_escape (list "a"))))`, want: "TYPE_ERROR: html_escape expected string"},
		{name: "number", source: `(cli_app (print (attr_escape 1)))`, want: "TYPE_ERROR: attr_escape expected string"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code := generateCheckedGo(t, tc.source)
			stdout, crash := goRunCrash(t, code)
			if strings.TrimSpace(stdout) != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(crash, tc.want) {
				t.Fatalf("crash = %q", crash)
			}
		})
	}
}

const htmlEscapeStdout = "&amp;\n" +
	"&lt;\n" +
	"&gt;\n" +
	"&#34;\n" +
	"&#39;\n" +
	"&amp;&lt;&gt;&#34;&#39;\n" +
	"&amp;&lt;&gt;&#34;&#39;\n" +
	"[]\n" +
	"<button data-id=\"a&lt;b&amp;c&#34;d&#39;e\" onclick=\"onTask\">x&lt;y&amp;z</button>\n"

func mustReadRepo(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(moduleRoot(t), path))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
