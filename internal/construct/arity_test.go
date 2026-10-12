package construct

import (
	"strings"
	"testing"
)

// TestArityTableCoversRegistry is the drift guard for #141: every registry
// entry must declare an arity or carry an exemption reason, and neither table
// may name a construct the registry no longer has.
func TestArityTableCoversRegistry(t *testing.T) {
	names := map[string]bool{}
	for _, e := range Table() {
		names[e.Name] = true
		a, hasArity := arities[e.Name]
		reason, exempt := arityExempt[e.Name]
		switch {
		case hasArity && exempt:
			t.Errorf("%s has both an arity and an exemption", e.Name)
		case !hasArity && !exempt:
			t.Errorf("%s has no arity; add it to arities in arity.go (or arityExempt with a reason)", e.Name)
		case exempt && strings.TrimSpace(reason) == "":
			t.Errorf("%s exemption has no reason", e.Name)
		case hasArity && (a.Min < 0 || (a.Max != Variadic && a.Max < a.Min)):
			t.Errorf("%s arity %+v is malformed", e.Name, a)
		}
		if e.Support == Unsupported && !exempt {
			t.Errorf("%s is Unsupported and should be exempt (the profile scan reports it)", e.Name)
		}
	}
	for name := range arities {
		if !names[name] {
			t.Errorf("arities has stale entry %s", name)
		}
	}
	for name := range arityExempt {
		if !names[name] {
			t.Errorf("arityExempt has stale entry %s", name)
		}
	}
	t.Logf("registry=%d arities=%d exempt=%d", len(names), len(arities), len(arityExempt))
}

func arityMessages(t *testing.T, src string) []string {
	t.Helper()
	var out []string
	for _, v := range CheckArity(parse(t, src).ParseExpression()) {
		out = append(out, v.Message())
	}
	return out
}

func TestArityMessages(t *testing.T) {
	cases := map[string]string{
		`(cli_app (read_file))`:                     "ARITY_ERROR: read_file expects 1 arg, got 0",
		`(cli_app (print (str_trim)))`:              "ARITY_ERROR: str_trim expects 1 arg, got 0",
		`(cli_app (fetch "u"))`:                     "ARITY_ERROR: fetch expects 2 to 3 args, got 1",
		`(cli_app (defun f (x)))`:                   "ARITY_ERROR: defun expects at least 3 args, got 2",
		`(cli_app (write_file "a" "b" "c"))`:        "ARITY_ERROR: write_file expects 2 args, got 3",
		`(cli_app (let (x (read_file)) (print x)))`: "ARITY_ERROR: read_file expects 1 arg, got 0",
	}
	for src, want := range cases {
		got := arityMessages(t, src)
		if len(got) == 0 || got[0] != want {
			t.Errorf("%s: got %v, want %q", src, got, want)
		}
	}
}

func TestArityAcceptsWellFormed(t *testing.T) {
	for _, src := range []string{
		`(cli_app (let (min 0 max 3) (print (+ min max))))`,
		`(cli_app (defun f (a b c d) (print a)) (f 1 2 3 4))`,
		`(cli_app (print (and true true false)))`,
		`(cli_app (print (fetch "u" "GET")))`,
		`(cli_app (let (d (dict ("a" 1) ("b" 2))) (print d)))`,
		`(cli_app (print))`,
	} {
		if got := arityMessages(t, src); len(got) != 0 {
			t.Errorf("%s: unexpected %v", src, got)
		}
	}
}
