package main

import (
	"math"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// NUMERIC_CONTRACT sections 2.3 and 7: float +, - and * that leave the finite
// float64 range raise a numeric error on the interpreter, governed AST
// bytecode and HFIR bytecode. They used to print +Inf or -Inf, and Inf - Inf
// NaN, with exit status 0.
func TestNumericFloatOverflowRefusedOnEveryPath(t *testing.T) {
	bin := c1c2Binary(t)
	for src, want := range map[string]string{
		`(do (let (x (to_float "1e308")) (print (* x 10.0))))`:                                     "floating-point overflow: 1e+308 * 10 is not finite",
		`(do (let (x (to_float "1.7976931348623157e308")) (print (+ x x))))`:                       "floating-point overflow: 1.7976931348623157e+308 + 1.7976931348623157e+308",
		`(do (let (x (to_float "1.7976931348623157e308")) (print (- (- 0.0 x) x))))`:               "floating-point overflow: -1.7976931348623157e+308 - 1.7976931348623157e+308",
		`(do (let (x (to_float "1e300")) (print (* 9223372036854775807 x))))`:                      "floating-point overflow: 9.223372036854776e+18 * 1e+300",
		`(do (print (* 1` + strings.Repeat("0", 200) + `.0 1` + strings.Repeat("0", 200) + `.0)))`: "floating-point overflow",
	} {
		for _, args := range issue149Paths(t, bin, src) {
			out, err := issue151Run(t, bin, args)
			if err == nil || !strings.Contains(out, want) || strings.Contains(out, "Inf") || strings.Contains(out, "NaN") {
				t.Errorf("%s %v: err=%v out=%q, want a failure containing %q", src, args[0], err, out, want)
			}
		}
	}
}

// A NaN score passed (or (< s 0.0) (> s 100.0)), and an infinite limit
// passed (> amount limit) the wrong way. Both programs printed ACCEPT before
// the fix; now neither reaches the decision.
func TestNumericNonFiniteCannotPassPolicyChecks(t *testing.T) {
	bin := c1c2Binary(t)
	for _, src := range []string{
		`(do (let (big (to_float "1e308") inf (* big 10.0) score (- inf inf)) (if (or (< score 0.0) (> score 100.0)) (print "REJECT") (print "ACCEPT"))))`,
		`(do (let (big (to_float "1e308") limit (* big 10.0)) (if (> 5000.0 limit) (print "REJECT") (print "ACCEPT"))))`,
	} {
		for _, args := range issue149Paths(t, bin, src) {
			out, err := issue151Run(t, bin, args)
			if err == nil || strings.Contains(out, "ACCEPT") || !strings.Contains(out, "floating-point overflow") {
				t.Errorf("%s %v: err=%v out=%q, want floating-point overflow before the decision", src, args[0], err, out)
			}
		}
	}
}

// Finite results, including those at the edge of the float64 range, and the
// existing integer overflow error are unchanged.
func TestNumericFiniteArithmeticUnchangedOnEveryPath(t *testing.T) {
	bin := c1c2Binary(t)
	src := `(do (print (+ 0.1 0.2)) (print (* 2.5 4)) (print (- 1 0.5)) (print (/ 9 2))` +
		` (print (* (to_float "1e154") (to_float "1e154")))` +
		` (let (m (to_float "1.7976931348623157e308")) (do (print (+ m 0.0)) (print (* m 1.0)) (print (- m m)) (print (- (- 0.0 m) 0.0))))` +
		` (print (* (to_float "1e-200") (to_float "1e-200"))) (print (* (to_float "5e-324") 0.5))` +
		` (print (* 9223372036854775807 1.0)) (print (+ 9223372036854775806 1)))`
	want := "0.30000000000000004\n10\n0.5\n4.5\n1e+308\n1.7976931348623157e+308\n1.7976931348623157e+308\n0\n-1.7976931348623157e+308\n0\n0\n9.223372036854776e+18\n9223372036854775807\n"
	issue161Run(t, src, want)
	for _, args := range issue149Paths(t, bin, `(do (print (+ 9223372036854775807 1)))`) {
		if out, err := issue151Run(t, bin, args); err == nil || !strings.Contains(out, "integer overflow") {
			t.Errorf("%v: err=%v out=%q, want integer overflow", args[0], err, out)
		}
	}
}

// A literal outside the int64 or finite float64 range is a check error on
// every entry point. check used to accept it, the interpreter refused it at
// run time, and build wrote an artifact in which the integer saturated to an
// int64 bound and the float became +Inf.
func TestNumericLiteralRangeOnEveryEntryPoint(t *testing.T) {
	bin := c1c2Binary(t)
	dir := t.TempDir()
	for literal, want := range map[string]string{
		"99999999999999999999":                 "integer literal 99999999999999999999 is outside the int64 range",
		"9223372036854775808":                  "integer literal 9223372036854775808 is outside the int64 range",
		"-9223372036854775809":                 "integer literal -9223372036854775809 is outside the int64 range",
		"1" + strings.Repeat("0", 309) + ".0":  "is outside the finite float64 range",
		"-1" + strings.Repeat("0", 309) + ".0": "is outside the finite float64 range",
	} {
		p := writeTemp(t, dir, "lit.howl", "(cli_app\n  (print (+ "+literal+" 0)))\n")
		for _, args := range [][]string{
			{"check", p},
			{"-validate", p},
			{"-run", p},
			{"run", p},
			{"build", p, "-o", filepath.Join(dir, "lit.hfbc")},
			{"-compile-bc", p, "-o", filepath.Join(dir, "lit2.hfbc")},
			{"build", p, "--target", "go", "-o", filepath.Join(dir, "server.go")},
			{"build", p, "--target", "js", "-o", filepath.Join(dir, "app.js")},
		} {
			out, err := exec.Command(bin, args...).CombinedOutput()
			if err == nil || !strings.Contains(string(out), want) || !strings.Contains(string(out), `"line":2,"column":13`) {
				t.Errorf("%s %v: err=%v out=%.300q, want %q at line 2, column 13", literal, args[0], err, out, want)
			}
		}
	}

	maxFloat := strconv.FormatFloat(math.MaxFloat64, 'f', -1, 64) + ".0"
	src := "(do (print 9223372036854775807) (print -9223372036854775808) (print " + maxFloat + ") (print -" + maxFloat + "))"
	issue161Run(t, src, "9223372036854775807\n-9223372036854775808\n1.7976931348623157e+308\n-1.7976931348623157e+308\n")
}
