package htmlescape

import (
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestEscapeEncodesTheFiveCharacters(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"&", "&amp;"},
		{"<", "&lt;"},
		{">", "&gt;"},
		{`"`, "&#34;"},
		{"'", "&#39;"},
		{`&<>"'`, "&amp;&lt;&gt;&#34;&#39;"},
		{"plain", "plain"},
	}
	for _, tc := range cases {
		if got := Escape(tc.in); got != tc.want {
			t.Errorf("Escape(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestHandlerContractRejectsScriptAndInlineInterpolation(t *testing.T) {
	safe := parseHowl(t, `(cli_app
  (let (id "a<b&c\"d'e")
    (let (title "x<y&z")
      (print (str_join (list
        "<button data-id=\""
        (attr_escape id)
        "\" onclick=\""
        "onTask"
        "\">"
        (html_escape title)
        "</button>") "")))))`)
	pieces, ok := ProgramMarkupPieces(safe)
	if !ok {
		t.Fatal("safe program has no markup join")
	}
	names, violation := ScanHandlers(pieces)
	if violation != "" {
		t.Fatalf("safe handler encode: %s", violation)
	}
	if len(names) != 1 || names[0] != "onTask" {
		t.Fatalf("handler names = %#v, want constant onTask", names)
	}
	if !hasEscape(pieces, "attr_escape") || !hasEscape(pieces, "html_escape") {
		t.Fatalf("safe pieces lost an escape call: %#v", pieces)
	}

	unsafe := []string{
		`(cli_app (let (id "x") (print (str_join (list "<script>" id "</script>") ""))))`,
		`(cli_app (let (id "x") (print (str_join (list "<button onclick=\"" id "\">") ""))))`,
		`(cli_app (let (id "x") (print (str_join (list "<script>go('" (html_escape id) "')</script>") ""))))`,
		`(cli_app (let (id "x") (print (str_join (list "<button onclick=\"" (attr_escape id) "\">") ""))))`,
		`(cli_app (let (id "x") (print (str_join (list "<img onerror='" id "'>") ""))))`,
	}
	for _, source := range unsafe {
		root := parseHowl(t, source)
		got, found := ProgramMarkupPieces(root)
		if !found {
			t.Fatalf("unsafe program has no markup join:\n%s", source)
		}
		_, bad := ScanHandlers(got)
		if bad == "" {
			t.Fatalf("contract missed interpolation:\n%s", source)
		}
		if !strings.Contains(bad, "<script>") && !strings.Contains(bad, "inline handler") {
			t.Fatalf("violation %q does not name the handler context", bad)
		}
	}
}

func TestBackendMarkupContract(t *testing.T) {
	safe := `strings.Join([]string{"<button data-id=\"", howlFrameHTMLEscape("attr_escape", id), "\" onclick=\"", "onTask", "\">", howlFrameHTMLEscape("html_escape", title), "</button>"}, "")`
	names, violation, found := InspectBackendMarkup(safe)
	if !found {
		t.Fatal("safe backend markup was not found")
	}
	if violation != "" {
		t.Fatalf("safe backend markup: %s", violation)
	}
	if len(names) != 1 || names[0] != "onTask" {
		t.Fatalf("backend handler names = %#v", names)
	}

	jsSafe := `["<button data-id=\"", howlFrameHTMLEscape("attr_escape", id), "\" onclick=\"", "onTask", "\">", howlFrameHTMLEscape("html_escape", title), "</button>"].join("")`
	names, violation, found = InspectBackendMarkup(jsSafe)
	if !found || violation != "" || len(names) != 1 || names[0] != "onTask" {
		t.Fatalf("js markup found=%v violation=%q names=%#v", found, violation, names)
	}

	bad := []string{
		`[]string{"<script>", id, "</script>"}`,
		`[]string{"<button onclick=\"", id, "\">"}`,
		`[]string{"<button onclick=\"", howlFrameHTMLEscape("attr_escape", id), "\">"}`,
		`"<script>" + id + "</script>"`,
		`"<button onclick=\"" + id + "\">"`,
		`"<img onerror='" + taskID + "'>"`,
	}
	for _, expr := range bad {
		pieces, ok := ParseMarkupExpr(expr)
		if !ok {
			t.Fatalf("did not parse %s", expr)
		}
		_, violation := ScanHandlers(pieces)
		if violation == "" {
			t.Fatalf("contract missed %s", expr)
		}
	}
}

func hasEscape(pieces []Piece, kind string) bool {
	for _, piece := range pieces {
		if piece.Escape == kind {
			return true
		}
	}
	return false
}

func parseHowl(t *testing.T, source string) *ast.Node {
	t.Helper()
	return parser.NewParser(lexer.NewLexer(source), "escape.howl").ParseExpression()
}
