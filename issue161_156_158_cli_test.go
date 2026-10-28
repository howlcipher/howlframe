package main

import (
	"strings"
	"testing"
)

// issue161Run runs src on the interpreter, governed AST bytecode and HFIR
// bytecode and requires the same exact stdout on every path.
func issue161Run(t *testing.T, src, want string) {
	t.Helper()
	bin := c1c2Binary(t)
	for _, args := range issue149Paths(t, bin, src) {
		out, err := issue151Run(t, bin, args)
		if err != nil || out != want {
			t.Errorf("%v: err=%v out=%q want %q", args[0], err, out, want)
		}
	}
}

// Issue #161: put evaluates to the updated root dict on every path, so it
// can be used as an expression (encode_json used to see null or underflow).
func TestIssue161PutReturnsDictOnEveryPath(t *testing.T) {
	src := `(do (let (d (dict)) (do (print (encode_json (put d "a" 1))) (print (encode_json d))))` +
		` (let (n (dict)) (do (print (encode_json (put n "a" "b" 2))) (print (encode_json (put n "a" "c" 3)))))` +
		` (let (s (dict)) (do (put s "k" 5) (print (encode_json s)))))`
	want := "{\"a\":1}\n{\"a\":1}\n{\"a\":{\"b\":2}}\n{\"a\":{\"b\":2,\"c\":3}}\n{\"k\":5}\n"
	issue161Run(t, src, want)
}

// Issue #156: encode_json writes int64 exactly, with no float64 rounding
// above 2^53.
func TestIssue156EncodeJSONInt64ExactOnEveryPath(t *testing.T) {
	src := `(do (print (encode_json 9007199254740993)) (print (encode_json (list 9223372036854775807 -9223372036854775808 9007199254740992)))` +
		` (print (encode_json (dict ("big" 1234567890123456789)))) (print (encode_json 2.5)))`
	want := "9007199254740993\n[9223372036854775807,-9223372036854775808,9007199254740992]\n{\"big\":1234567890123456789}\n2.5\n"
	issue161Run(t, src, want)
}

// Issue #158: <, <=, > and >= with a NaN operand are false (IEEE 754,
// Python). This test used to build its NaN from Inf - Inf. NUMERIC_CONTRACT
// section 7 is now enforced, so no source program can build a NaN: to_float
// refuses "nan" (#225) and the overflow that makes the infinity fails. The
// operator guarantee is kept by TestNaNOrderingIsFalseAtTheOperators
// (internal/vm); here mixed int/float ordering is kept and the former NaN
// construction must fail on every path.
func TestIssue158NaNOrderingOnEveryPath(t *testing.T) {
	issue161Run(t, `(do (print (< 1 2.0) (<= 2 2.0) (>= 3.0 3) (> 1.5 1)))`, "true true true true\n")
	bin := c1c2Binary(t)
	src := `(do (let (big (to_float "1e308") n (- (* big 10.0) (* big 10.0))) (print (< n 1) (> n 1))))`
	for _, args := range issue149Paths(t, bin, src) {
		if out, err := issue151Run(t, bin, args); err == nil || !strings.Contains(out, "floating-point overflow") || strings.Contains(out, "false") {
			t.Errorf("%v: err=%v out=%q, want floating-point overflow before any comparison", args[0], err, out)
		}
	}
}
