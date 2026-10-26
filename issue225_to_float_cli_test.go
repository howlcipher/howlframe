package main

import (
	"strconv"
	"strings"
	"testing"
)

// Issue #225: a range check such as (or (< x 0.0) (> x 1.0)) is false for
// NaN, so a to_float that returned NaN for "nan" let invalid input through
// as accepted. NUMERIC_CONTRACT section 2.3 makes every NaN and infinity
// spelling a CONVERSION_ERROR; finite input keeps its range decision. The
// check runs on the interpreter, governed AST bytecode and HFIR bytecode.
func TestIssue225RangeCheckRefusesNonFiniteText(t *testing.T) {
	bin := c1c2Binary(t)
	program := func(text string) string {
		return `(cli_app (let (x (to_float ` + strconv.Quote(text) + `))
  (if (or (< x 0.0) (> x 1.0)) (print "reject") (print "accept"))))`
	}
	for _, text := range []string{"nan", "NaN", " NAN ", "inf", "-Infinity", "+INF", "1e400"} {
		for _, args := range issue149Paths(t, bin, program(text)) {
			out, err := issue151Run(t, bin, args)
			if err == nil || !strings.Contains(out, "CONVERSION_ERROR") || strings.Contains(out, "accept") {
				t.Errorf("%q on %v: err=%v out=%q, want CONVERSION_ERROR", text, args[0], err, out)
			}
		}
	}
	for text, want := range map[string]string{"0.5": "accept\n", " 1e-1 ": "accept\n", "-0.0": "accept\n", "1.5": "reject\n", "-1e-3": "reject\n"} {
		for _, args := range issue149Paths(t, bin, program(text)) {
			if out, err := issue151Run(t, bin, args); err != nil || out != want {
				t.Errorf("%q on %v: err=%v out=%q, want %q", text, args[0], err, out, want)
			}
		}
	}
}
