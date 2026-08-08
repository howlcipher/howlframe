package construct

import (
	"testing"

	"zero/internal/lexer"
	"zero/internal/parser"
)

func parse(t *testing.T, source string) *parser.Parser {
	t.Helper()
	return parser.NewParser(lexer.NewLexer(source), "scan_test.zero")
}

func scan(t *testing.T, source string) []Violation {
	t.Helper()
	return Scan(parse(t, source).ParseExpression())
}

func scanNames(t *testing.T, source string) []string {
	t.Helper()
	var names []string
	for _, violation := range scan(t, source) {
		names = append(names, violation.Name)
	}
	return names
}

func assertClean(t *testing.T, name, source string) {
	t.Helper()
	if got := scanNames(t, source); len(got) != 0 {
		t.Errorf("%s: expected no violations, got %v", name, got)
	}
}

// TestScanAcceptsStructuralListsThatAreNotConstructs is the regression guard
// for bugs.md #45's implementation caveat. Each of these programs contains a
// list whose head symbol is NOT a construct - a bound variable name, a
// parameter, a dict key, or a sub-form the parent destructures itself. A
// support check matching on head symbols alone (as a deny-list over ZIR node
// kinds would) rejects all of them.
func TestScanAcceptsStructuralListsThatAreNotConstructs(t *testing.T) {
	cases := []struct{ name, source string }{
		{
			// The binding list's head is the variable name.
			"let binding",
			`(cli_app (let (x 0) (print x)))`,
		},
		{
			// Worse: a variable deliberately named after an
			// unsupported construct.
			"let binding shadowing an unsupported construct name",
			`(cli_app (let (match 1) (print match)))`,
		},
		{
			// catch is destructured by compileNode's try_let case and
			// never reaches head dispatch.
			"try_let catch clause",
			`(cli_app (try_let (b (read_file "f.txt")) (catch e (print "err")) (print "ok")))`,
		},
		{
			// Parameter lists name identifiers, including ones that
			// collide with construct names.
			"defun parameter list",
			`(cli_app (defun f (test match) (return test)) (print (call f 1 2)))`,
		},
		{
			"defun parameter list with type_hint annotations",
			`(cli_app (defun f ((type_hint a "int")) (return a)) (print (call f 1)))`,
		},
		{
			"type_hints annotation block",
			`(cli_app (defun f (a) (type_hints (a int) (return int)) (return a)) (print (call f 1)))`,
		},
		{
			"dict pair with a compound value",
			`(cli_app (defun one () (return 1)) (print (dict ("k" (call one)))))`,
		},
		{
			// route and lambda are destructured by the http_server case.
			"http_server route and lambda",
			`(http_server 8080 (route "/x" (lambda (req) (res 200 "text/plain" "ok"))))`,
		},
		{
			"spawn lambda",
			`(cli_app (spawn (lambda () (print "bg"))))`,
		},
		{
			// (test "cmd") here is optimize_signature metadata, not a
			// test block. tests/optimization_signature.zero relies on
			// this and is NOT exempt in tools/difftest/manifest.json.
			"optimize_signature metadata",
			`(cli_app (optimize_signature p (metric "accuracy") (test "go test ./...") (candidate "a" "A") (print "body")))`,
		},
		{
			// achieve's operands are stringified via ast.Stringify, so
			// their heads are never compiled.
			"achieve stringified operands",
			`(cli_app (let (r (achieve (is_sorted items) (using "quick sort"))) (print r)))`,
		},
		{
			"ephemeral_circuit argument list",
			`(cli_app (let (r (ephemeral_circuit (2 3) "multiply")) (print r)))`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { assertClean(t, tc.name, tc.source) })
	}
}

func TestScanRejectsUnsupportedConstructs(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		want    string
		tracker string
	}{
		{
			name:   "match",
			source: `(cli_app (let (x 0) (match x (0 (print "zero")) (default (print "other")))))`,
			want:   "match",
		},
		{
			name:    "cli_app level test block",
			source:  `(cli_app (defun f (a) (print a)) (test "void" (call f "hi")))`,
			want:    "test",
			tracker: "improvements.md #96",
		},
		{
			name:    "module import",
			source:  `(http_server 8081 (use "routes.zero" as routes))`,
			want:    "use",
			tracker: "improvements.md #95",
		},
		{
			name:   "struct declaration",
			source: `(cli_app (struct User (name string)) (print "x"))`,
			want:   "struct",
		},
		{
			name:   "invented head",
			source: `(cli_app (do (print "before") (totally_made_up_head "x" 42) (print "after")))`,
			want:   "totally_made_up_head",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			violations := scan(t, tc.source)
			if len(violations) != 1 {
				t.Fatalf("got %d violations, want 1: %+v", len(violations), violations)
			}
			if violations[0].Name != tc.want {
				t.Errorf("violation name = %q, want %q", violations[0].Name, tc.want)
			}
			if violations[0].Tracker != tc.tracker {
				t.Errorf("violation tracker = %q, want %q", violations[0].Tracker, tc.tracker)
			}
			if violations[0].Line == 0 && violations[0].Column == 0 {
				t.Error("violation carries no source location")
			}
		})
	}
}

// TestScanDoesNotCascade proves one unsupported construct produces exactly one
// violation rather than one per identifier nested inside it - a struct's field
// lists and a match's arms are named after user identifiers and literals.
func TestScanDoesNotCascade(t *testing.T) {
	source := `(cli_app (struct User (name string) (age int) (email string)) (print "x"))`
	if got := scanNames(t, source); len(got) != 1 || got[0] != "struct" {
		t.Errorf("got %v, want exactly [struct]", got)
	}
}

func TestScanReportsInSourceOrder(t *testing.T) {
	source := `(cli_app (trace "a") (print "b") (middleware "c"))`
	violations := scan(t, source)
	if len(violations) != 2 {
		t.Fatalf("got %d violations, want 2: %+v", len(violations), violations)
	}
	if violations[0].Name != "trace" || violations[1].Name != "middleware" {
		t.Errorf("got %q then %q, want trace then middleware", violations[0].Name, violations[1].Name)
	}
	if violations[0].Column >= violations[1].Column && violations[0].Line >= violations[1].Line {
		t.Errorf("violations are not in source order: %+v", violations)
	}
}

func TestScanFindsNestedUnsupportedConstructs(t *testing.T) {
	source := `(cli_app (let (x 0) (while (< x 3) (do (trace x) (set x (+ x 1))))))`
	if got := scanNames(t, source); len(got) != 1 || got[0] != "trace" {
		t.Errorf("got %v, want exactly [trace]", got)
	}
}

func TestScanAcceptsNilAndLeafNodes(t *testing.T) {
	if got := Scan(nil); len(got) != 0 {
		t.Errorf("Scan(nil) = %v, want none", got)
	}
	if got := scanNames(t, `(cli_app (print "hello"))`); len(got) != 0 {
		t.Errorf("got %v, want none", got)
	}
}

// TestScanIsDeterministic guards the diagnostic ordering contract that
// zero.go's reportZirDiagnostics depends on.
func TestScanIsDeterministic(t *testing.T) {
	source := `(cli_app (trace "a") (middleware "b") (struct S (f int)))`
	first := scanNames(t, source)
	for i := 0; i < 20; i++ {
		next := scanNames(t, source)
		if len(next) != len(first) {
			t.Fatalf("run %d returned %d violations, first returned %d", i, len(next), len(first))
		}
		for j := range first {
			if next[j] != first[j] {
				t.Fatalf("run %d differs at %d: %q vs %q", i, j, next[j], first[j])
			}
		}
	}
}
