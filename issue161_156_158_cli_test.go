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
// Python), and sort/min/max and the _by forms keep their prior output.
func TestIssue158NaNOrderingOnEveryPath(t *testing.T) {
	cmp := []string{}
	for _, op := range []string{"<", "<=", ">", ">="} {
		for _, pair := range []string{"n 1", "1 n", "n 1.0", "1.0 n", "n n"} {
			cmp = append(cmp, "("+op+" "+pair+")")
		}
	}
	src := `(do (let (n (to_float "nan")) (do (print (str_join (list ` + strings.Join(cmp, " ") + `) " ")) (print (< 1 2.0) (<= 2 2.0) (>= 3.0 3) (> 1.5 1))` +
		` (let (xs (list 3.0 n 1.0 2.0)) (do (print (sort xs)) (print (min xs)) (print (max xs))` +
		` (print (sort_by x xs (* x 1.0))) (print (max_by x xs x)) (print (min_by x xs x)))))))`
	want := strings.TrimSpace(strings.Repeat("false ", len(cmp))) + "\ntrue true true true\n[1 2 3 NaN]\n1\n3\n[1 2 3 NaN]\n3\n1\n"
	issue161Run(t, src, want)
}
