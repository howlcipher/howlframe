package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The JavaScript backend emits let, do and if in value position (an operand,
// a binding or a call argument) as an awaited async arrow that returns the
// form's value. That is the shape value sugar and the core library lower to,
// so each probe must print exactly what the bytecode VM prints.
func TestJSValueFormsMatchBytecode(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	bin := buildHowlFrameBinaryForTest(t)
	probes := []struct{ name, body string }{
		{"if_value", `(let (a 1) (print (if (> a 0) "pos" "neg")))`},
		{"if_binding", `(let (s (if false "y" "n")) (print s))`},
		{"if_nested_value", `(let (a 5) (print (if (> a 10) "big" (if (> a 3) "mid" "small"))))`},
		{"not", `(do (print (not true)) (print (not false)))`},
		{"starts_with", `(do (print (starts_with "foobar" "foo")) (print (starts_with "foobar" "bar")))`},
		{"ends_with", `(do (print (ends_with "foobar" "bar")) (print (ends_with "foobar" "foo")))`},
		{"map_has", `(let (d (dict ("a" "b"))) (do (print (map_has d "a")) (print (map_has d "z"))))`},
		{"list_contains", `(let (xs (list "a" "b")) (do (print (list_contains xs "b")) (print (list_contains xs "c"))))`},
		{"all_any", `(let (xs (list 1 2 3)) (do (print (all x xs (> x 0))) (print (any x xs (> x 2))) (print (any x xs (> x 9)))))`},
		{"ver_gt", `(do (print (ver_gt "1.10.0" "1.9.9")) (print (ver_ge "1.2.3" "1.2.4")))`},
		{"str_slice", `(do (print (str_slice "abcdef" 1 3)) (print (str_slice "abcdef" 2)) (print (str_slice "abcdef" -2)))`},
		{"str_len", `(print (str_len "abcdef"))`},
		{"index_of", `(let (xs (list "a" "b" "c")) (do (print (index_of xs "c")) (print (index_of xs "z"))))`},
		{"map_list", `(let (xs (list 1 2 3)) (print (str_join (map_list x xs (to_string (* x 2))) ",")))`},
		{"filter", `(let (xs (list 1 2 3 4)) (print (list_len (filter x xs (> x 2)))))`},
		{"reduce", `(let (xs (list 1 2 3 4)) (print (reduce acc 0 x xs (+ acc x))))`},
		{"sum", `(print (sum (list 1 2 3)))`},
		{"reverse", `(print (str_join (reverse (list "a" "b" "c")) ""))`},
		{"sort", `(do (print (str_join (sort (list "b" "c" "a")) "")) (print (encode_json (sort (list 3 1 2)))))`},
		{"count_by", `(let (xs (list "a" "b" "a")) (print (encode_json (count_by x xs x))))`},
		{"list_slice", `(print (encode_json (list_slice (list 1 2 3 4 5) 1 -1)))`},
		{"nested_sugar", `(let (xs (list "alpha" "beta" "gamma")) (print (str_join (filter x xs (not (starts_with x "b"))) ",")))`},
		{"call_argument", `(print (str_join (list (if true "a" "b") (if false "c" "d")) "-"))`},
		{"set_value", `(let (v "x") (do (set v (if (= v "x") "matched" "missed")) (print v)))`},
		{"case_and_trim", `(do (print (to_lower "AbC")) (print (to_upper "AbC")) (print (str_trim "  x y  ")))`},
		{"trim_in_binding", `(let (s (str_trim "  {} ")) (print s))`},
		{"parse_json_expr", `(let (d (parse_json Any (str_trim " {\"a\": 1} "))) (print (map_get d "a")))`},
		{"nz", `(let (d (dict ("a" "b"))) (do (print (nz (map_get d "a") "z")) (print (nz (map_get d "q") "z"))))`},
	}
	for _, p := range probes {
		p := p
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			cli := filepath.Join(dir, "cli.howl")
			os.WriteFile(cli, []byte("(cli_app\n"+p.body+")\n"), 0o600)
			want, err := exec.Command(bin, "run", cli).CombinedOutput()
			if err != nil {
				t.Fatalf("bytecode run failed: %v %s", err, want)
			}
			web := filepath.Join(dir, "web.howl")
			os.WriteFile(web, []byte("(web_app\n"+p.body+")\n"), 0o600)
			outDir := filepath.Join(dir, "js")
			if out, err := exec.Command(bin, "build", web, "--target", "js", "-o", outDir).CombinedOutput(); err != nil {
				t.Fatalf("js build rejected: %v %s", err, out)
			}
			got, err := exec.Command("node", filepath.Join(outDir, "app.js")).CombinedOutput()
			if err != nil {
				t.Fatalf("node failed: %v %s", err, got)
			}
			if string(got) != string(want) {
				t.Fatalf("javascript printed %q, bytecode printed %q", got, want)
			}
		})
	}
}

// A return inside a value-position form would return from the generated
// arrow, not from the enclosing function, so the build refuses it.
func TestJSValueFormRejectsReturnInside(t *testing.T) {
	bin := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	src := mustWriteFile(t, dir, "r.howl", `(web_app
  (defun f (a)
    (print (if (= a "x") (return "early") "late"))
    (return "end")))`)
	out, err := exec.Command(bin, "build", src, "--target", "js", "-o", filepath.Join(dir, "out")).CombinedOutput()
	if err == nil || !strings.Contains(string(out), "return inside") {
		t.Fatalf("return inside a value form must be rejected, got %v %s", err, out)
	}
}

// Loops still have no value form on the JavaScript target.
func TestJSLoopAsValueStillRejected(t *testing.T) {
	bin := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	src := mustWriteFile(t, dir, "w.howl", `(web_app (print (while false (print "x"))))`)
	out, err := exec.Command(bin, "build", src, "--target", "js", "-o", filepath.Join(dir, "out")).CombinedOutput()
	if err == nil || !strings.Contains(string(out), "UNSUPPORTED_ON_TARGET") {
		t.Fatalf("while as a value must stay rejected, got %v %s", err, out)
	}
}
