package main

import (
	"math"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// Issue #151: float (and mixed int/float) % and // follow Python: % is
// math.fmod floored so the result takes the divisor's sign (a zero result
// takes the divisor's sign too), // is CPython's float floor division
// returned as an int. Inf/NaN operands and // results outside int64 are a
// VALUE_ERROR. Expected values were generated with Python 3 (a % b, a // b).
var issue151Cases = [][4]string{
	{"9223372036854775807", "189944.0", "33112.0", "48558375294059"},
	{"0.0", "-9223372036854775808", "-0.0", "0"},
	{"-9223372036854775808", "-95093.0", "-86339.0", "96993175489833"},
	{"9223372036854775807", "1e+33", "9.223372036854776e+18", "0"},
	{"9223372036854775807", "-1e+33", "-9.999999999999907e+32", "-1"},
	{"-9223372036854775807", "1e+33", "9.999999999999907e+32", "-1"},
	{"-9223372036854775808", "1e+33", "9.999999999999907e+32", "-1"},
	{"-9223372036854775808", "-1e+33", "-9.223372036854776e+18", "0"},
	{"1e+308", "3.0", "2.0", ""},
	{"-1e+308", "3.0", "1.0", ""},
	{"1e+308", "-7.5", "-4.0", ""},
	{"1e+308", "1e-308", "3.498445546245627e-309", ""},
	{"1e+308", "5e-324", "0.0", ""},
	{"5e-324", "1e+308", "5e-324", "0"},
	{"-5e-324", "1e+308", "1e+308", "-1"},
	{"1e+308", "1e+308", "0.0", "1"},
	{"-1e+308", "1e+308", "0.0", "-1"},
	{"1", "0.1", "0.09999999999999995", "9"},
	{"-1", "0.1", "5.551115123125783e-17", "-10"},
	{"1", "-0.1", "-5.551115123125783e-17", "-10"},
	{"0.3", "0.1", "0.09999999999999998", "2"},
	{"7.5", "2", "1.5", "3"},
	{"-7.5", "2", "0.5", "-4"},
	{"7.5", "-2", "-0.5", "-4"},
	{"7", "2.5", "2.0", "2"},
	{"-7", "2.5", "0.5", "-3"},
	{"9223372036854775807", "2.5", "0.5", "3689348814741910528"},
	{"-9223372036854775808", "0.5", "0.0", ""},
	{"9007199254740992.0", "3", "2.0", "3002399751580330"},
	{"9007199254740994.0", "-3", "-2.0", "-3002399751580332"},
	{"-0.0", "5.0", "0.0", "0"},
	{"-0.0", "-5.0", "-0.0", "0"},
	{"0.0", "-5.0", "-0.0", "0"},
	{"4.0", "-2.0", "-0.0", "-2"},
	{"-4.0", "2.0", "0.0", "-2"},
	{"6.0", "4", "2.0", "1"},
	{"9.0", "-2", "-1.0", "-5"},
	{"9223372036854775807", "9.223372036854776e+18", "0.0", "1"},
	{"-9223372036854775808", "-9.223372036854776e+18", "-0.0", "1"},
	{"1e+33", "9223372036854775807", "4.0352252661239644e+18", "108420217248550"},
	{"1.2345678901234568e+17", "1000", "680.0", "123456789012345"},
}

// issue151Lit renders a case operand as a HowlFrame literal: ints as is,
// floats as their shortest round-tripping decimal (no exponent syntax).
func issue151Lit(s string) string {
	if !strings.ContainsAny(s, ".e") {
		return s
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(err)
	}
	if f == 0 {
		if math.Signbit(f) {
			return "-0.0"
		}
		return "0.0"
	}
	out := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(out, ".") {
		out += ".0"
	}
	return out
}

func issue151Run(t *testing.T, bin string, args []string) (string, error) {
	t.Helper()
	if args[0] == "-run-bc" {
		args = append([]string{"-max-instructions", "2000000"}, args...)
	}
	out, err := exec.Command(bin, args...).CombinedOutput()
	return string(out), err
}

// Boundaries 1e308, 5e-324, +-2^63, MaxInt64/MinInt64 mixed with floats,
// -0.0 and non-integral divisors, identical on the interpreter, governed AST
// bytecode and HFIR bytecode.
func TestIssue151FloatModDivOnEveryPath(t *testing.T) {
	bin := c1c2Binary(t)
	var src strings.Builder
	var wantMod []float64
	var wantDiv []string
	src.WriteString("(do")
	for _, c := range issue151Cases {
		src.WriteString(" (print (% " + issue151Lit(c[0]) + " " + issue151Lit(c[1]) + "))")
		w, _ := strconv.ParseFloat(c[2], 64)
		wantMod = append(wantMod, w)
	}
	for _, c := range issue151Cases {
		if c[3] != "" {
			src.WriteString(" (print (// " + issue151Lit(c[0]) + " " + issue151Lit(c[1]) + "))")
			wantDiv = append(wantDiv, c[3])
		}
	}
	src.WriteString(")")
	for _, args := range issue149Paths(t, bin, src.String()) {
		out, err := issue151Run(t, bin, args)
		if err != nil {
			t.Fatalf("%v: %v %s", args[0], err, out)
		}
		lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if len(lines) != len(wantMod)+len(wantDiv) {
			t.Fatalf("%v: %d lines, want %d: %q", args, len(lines), len(wantMod)+len(wantDiv), out)
		}
		for i, w := range wantMod {
			g, err := strconv.ParseFloat(lines[i], 64)
			if err != nil || g != w || math.Signbit(g) != math.Signbit(w) {
				t.Errorf("%v (%% %s %s) = %q, want %v", args, issue151Cases[i][0], issue151Cases[i][1], lines[i], w)
			}
		}
		for i, w := range wantDiv {
			if lines[len(wantMod)+i] != w {
				t.Errorf("%v // #%d = %q, want %s", args, i, lines[len(wantMod)+i], w)
			}
		}
	}
}

func TestIssue151ErrorsOnEveryPath(t *testing.T) {
	bin := c1c2Binary(t)
	inf := "(* 1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000.0 1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000.0)"
	nan := "(let (i " + inf + ") (- i i))"
	cases := map[string]string{
		"(do (print (% " + inf + " 2.0)))":              "VALUE_ERROR: % and // need finite operands",
		"(do (print (% 2.0 " + inf + ")))":              "VALUE_ERROR: % and // need finite operands",
		"(do (print (% -2 (- 0.0 " + inf + "))))":       "VALUE_ERROR: % and // need finite operands",
		"(do (print (// " + inf + " 3)))":               "VALUE_ERROR: % and // need finite operands",
		"(do (print (% " + nan + " 2.0)))":              "VALUE_ERROR: % and // need finite operands",
		"(do (print (// 1.5 " + nan + ")))":             "VALUE_ERROR: % and // need finite operands",
		"(do (print (% 7.5 0.0)))":                      "division by zero",
		"(do (print (// 7.5 -0.0)))":                    "division by zero",
		"(do (print (% 9223372036854775807 0.0)))":      "division by zero",
		"(do (print (// -9223372036854775808 0.5)))":    "VALUE_ERROR: // result outside int64",
		"(do (print (// 9223372036854775807 1.0)))":     "VALUE_ERROR: // result outside int64",
		"(do (print (// 1000000000000000000000.0 -3)))": "VALUE_ERROR: // result outside int64",
		"(do\n  (print (// 1.5 " + nan + ")))":          "at line 2, column 10",
		"(do\n (let ((a b) (list 1 2 3)) (print a b)))": "VALUE_ERROR: destructuring expects 2 elements at line 2, column 8",
	}
	for src, want := range cases {
		for _, args := range issue149Paths(t, bin, src) {
			out, err := issue151Run(t, bin, args)
			if err == nil || !strings.Contains(out, want) {
				t.Errorf("%s %v: err=%v out=%q want %q", src, args[0], err, out, want)
			}
		}
	}
}

// A non-numeric operand is reported at the % or // form in the user's terms,
// not as an error about a comparison inside the expansion.
func TestIssue151NonNumericOperandMessage(t *testing.T) {
	bin := c1c2Binary(t)
	for _, c := range [][2]string{
		{`(do (print (% "a" 2)))`, "% requires numeric operands (int or float)"},
		{`(do (print (// 1 "x")))`, "// requires numeric operands (int or float)"},
		{`(do (print (% true 2.5)))`, "% requires numeric operands (int or float)"},
		{`(do (print (// (list 1) 3)))`, "// requires numeric operands (int or float)"},
	} {
		dir := t.TempDir()
		p := writeTemp(t, dir, "p.howl", c[0])
		out, err := exec.Command(bin, "-run", p).CombinedOutput()
		if err == nil || !strings.Contains(string(out), c[1]) || strings.Contains(string(out), "compares incompatible") {
			t.Errorf("%s: err=%v out=%s", c[0], err, out)
		}
	}
}

// Mixed int/float comparisons order values outside the int64 range and
// fractional floats without rounding the int through float64 (2^63 used to
// compare as MinInt64, which made (% MaxInt64 1e33) add the divisor).
func TestIssue151MixedCompareOnEveryPath(t *testing.T) {
	bin := c1c2Binary(t)
	src := `(do (print (< 9223372036854775807 9223372036854775808.0) (> 9223372036854775808.0 0) (< -9223372036854775808 -9223372036854777856.0) (= 9223372036854775807 9223372036854775808.0) (< 3 3.5) (> 4 3.5) (< -4 -3.5) (> -3 -3.5) (<= 3 3.0) (< (- 9223372036854775807 0.0) 0)))`
	want := "true true false false true true true true true false\n"
	for _, args := range issue149Paths(t, bin, src) {
		out, err := issue151Run(t, bin, args)
		if err != nil || out != want {
			t.Errorf("%v: err=%v out=%q want %q", args[0], err, out, want)
		}
	}
}

// Go and JS still reject % and // with UNSUPPORTED_ON_TARGET.
func TestIssue151RejectedOnGoJS(t *testing.T) {
	bin := c1c2Binary(t)
	for _, f := range []string{`(print (% 9223372036854775807 1.5))`, `(print (// -0.0 2.5))`, `(print (% 1.0 0.1))`} {
		for _, target := range []string{"go", "js"} {
			dir := t.TempDir()
			p := writeTemp(t, dir, "g.howl", "(do "+f+")")
			out, err := exec.Command(bin, "build", "--target", target, p, "-o", dir+"/out").CombinedOutput()
			if err == nil || !strings.Contains(string(out), "UNSUPPORTED_ON_TARGET") {
				t.Errorf("%s on %s: err=%v out=%s", f, target, err, out)
			}
		}
	}
}
