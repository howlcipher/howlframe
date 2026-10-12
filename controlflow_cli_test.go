package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
)

const controlflowProgram = `(do
 (print (% 7 3) (% -7 3) (% 7 -3) (% -7 -3) (// 7 2) (// -7 2) (// 7 -2) -5 (+ 3 -2) -1.5 (* 3 -2))
 (print (if (= 1 2) "a" (= 2 2) "b" "c") (if false 1 false 2 3) (cond ((= 1 2) "x") (else "y")))
 (print (encode_json (range 4)) (encode_json (range 2 5)) (encode_json (range 3 1)))
 (let (ws (list "pear" "fig" "banana" "kiwi"))
  (print (max_by w ws (str_len w)) (min_by w ws (str_len w)) (encode_json (sort_by w ws (str_len w))) (encode_json (sort_by w ws (- 0 (str_len w))))))
 (print (str "[" (pad_left 42 5) "|" (pad_left "x" 3 "0") "|" (pad_right "ab" 4) "|" (pad_right "abcdef" 2) "]"))
 (print (str "[" (str_repeat "ab" 3) "|" (str_repeat "x" 0) "|" (rtrim "  hi \t\n") "|" (ltrim " \t hi ") "]"))
 (print (to_base 10 2) (to_base 0 2) (to_base 255 16) (to_base -10 2) (to_base 35 36))
 (let ((a b c) (list 1 2 3) d 4) (print a b c d))
 (let (s 0)
  (do
   (for x (range 10) (do (when (= x 5) (break)) (set s (+ s x))))
   (print s)))
 (let (i 0 found -1)
  (do
   (while (< i 100) (do (when (= (% i 7) 6) (do (set found i) (break))) (set i (+ i 1))))
   (print i found)))
 (let (out (list))
  (do
   (for a (range 3) (for b (range 3) (do (when (> b a) (break)) (append out (str a b)))))
   (print (str_join out ",")))))`

const controlflowWant = "1 2 -2 -1 3 -4 -4 -5 1 -1.5 -6\n" +
	"b 3 y\n" +
	"[0,1,2,3] [2,3,4] []\n" +
	"banana fig [\"fig\",\"pear\",\"kiwi\",\"banana\"] [\"banana\",\"pear\",\"kiwi\",\"fig\"]\n" +
	"[   42|00x|ab  |abcdef]\n" +
	"[ababab||  hi|hi ]\n" +
	"1010 0 ff -1010 z\n" +
	"1 2 3 4\n" +
	"10\n" +
	"6 6\n" +
	"00,10,11,20,21,22\n"

func writeTemp(t *testing.T, dir, name, src string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(src), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

// The control-flow and core built-ins are pure sugar: no capability under the
// governed profile, and the same output on the interpreter, governed AST
// bytecode and HFIR bytecode.
func TestControlflowBuiltinsArePureOnEveryPath(t *testing.T) {
	bin := c1c2Binary(t)
	dir := t.TempDir()
	src := writeTemp(t, dir, "p.howl", controlflowProgram)
	art := filepath.Join(dir, "p.hfbc")
	hart := filepath.Join(dir, "h.hfbc")
	for _, profile := range []string{"default", "governed"} {
		if out, err := exec.Command(bin, "check", "--profile", profile, src).CombinedOutput(); err != nil {
			t.Fatalf("check %s: %v %s", profile, err, out)
		}
	}
	if out, err := exec.Command(bin, "build", "--profile", "governed", src, "-o", art).CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	out, err := exec.Command(bin, "inspect", "--caps", art).CombinedOutput()
	if err != nil {
		t.Fatalf("inspect: %v %s", err, out)
	}
	var report bytecode.Report
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if len(report.Capabilities) != 0 {
		t.Fatalf("pure control-flow built-ins required capabilities %v", report.Capabilities)
	}
	if out, err := exec.Command(bin, "-compile-hfir-bc", src, "-o", hart).CombinedOutput(); err != nil {
		t.Fatalf("hfir compile: %v %s", err, out)
	}
	runs := map[string][]string{
		"interpreter":       {"-run", src},
		"bytecode_governed": {"-run-bc", art},
		"hfir_bytecode":     {"-run-bc", hart},
	}
	for name, args := range runs {
		t.Run(name, func(t *testing.T) {
			got, err := exec.Command(bin, args...).CombinedOutput()
			if err != nil {
				t.Fatalf("run: %v %s", err, got)
			}
			if string(got) != controlflowWant {
				t.Fatalf("got %q want %q", got, controlflowWant)
			}
		})
	}
}

// Runtime value errors fail closed with the same stderr shape on every path.
func TestControlflowValueErrorsOnEveryPath(t *testing.T) {
	bin := c1c2Binary(t)
	cases := map[string]string{
		`(do (print (max_by x (list) x)))`: "VALUE_ERROR: max_by of empty list",
		`(do (print (min_by x (list) x)))`: "VALUE_ERROR: min_by of empty list",
		`(do (print (to_base 5 1)))`:       "VALUE_ERROR: to_base base must be 2..36",
		`(do (print (to_base 5 37)))`:      "VALUE_ERROR: to_base base must be 2..36",
		`(do (print (% 5 0)))`:             "division by zero",
		`(do (print (// 5 0)))`:            "division by zero",
		// The signed multiply check still catches real overflow after the
		// fix that stopped (* 3 -2) from reporting one.
		`(do (print (* 9223372036854775807 2)))`:  "integer overflow",
		`(do (print (* -9223372036854775807 3)))`: "integer overflow",
	}
	for src, want := range cases {
		dir := t.TempDir()
		p := writeTemp(t, dir, "e.howl", src)
		art := filepath.Join(dir, "e.hfbc")
		hart := filepath.Join(dir, "h.hfbc")
		if out, err := exec.Command(bin, "build", "--profile", "governed", p, "-o", art).CombinedOutput(); err != nil {
			t.Fatalf("%s build: %v %s", src, err, out)
		}
		if out, err := exec.Command(bin, "-compile-hfir-bc", p, "-o", hart).CombinedOutput(); err != nil {
			t.Fatalf("%s hfir: %v %s", src, err, out)
		}
		for _, args := range [][]string{{"-run", p}, {"-run-bc", art}, {"-run-bc", hart}} {
			out, err := exec.Command(bin, args...).CombinedOutput()
			if err == nil || !strings.Contains(string(out), want) {
				t.Errorf("%s %v: err=%v out=%q want %q", src, args[0], err, out, want)
			}
		}
	}
}

// A break outside a loop, or in a value position, is a compile error on every
// entry point; it never reaches a backend.
func TestStrayBreakIsRejected(t *testing.T) {
	bin := c1c2Binary(t)
	for _, src := range []string{
		`(do (break))`,
		`(do (for x (list 1) (print (break))))`,
		`(do (let (y (break)) (print y)))`,
	} {
		dir := t.TempDir()
		p := writeTemp(t, dir, "b.howl", src)
		for _, args := range [][]string{{"-run", p}, {"build", "--profile", "governed", p, "-o", filepath.Join(dir, "b.hfbc")}, {"-compile-hfir-bc", p, "-o", filepath.Join(dir, "h.hfbc")}} {
			out, err := exec.Command(bin, args...).CombinedOutput()
			if err == nil || !strings.Contains(string(out), "break must be a statement inside for/while") {
				t.Errorf("%s %v: err=%v out=%q", src, args[0], err, out)
			}
		}
	}
}

// Go and JS reject every new form with UNSUPPORTED_ON_TARGET before writing
// anything. Negative literals are plain literals and still build.
func TestControlflowRejectedOnGoJS(t *testing.T) {
	bin := c1c2Binary(t)
	forms := []string{
		`(print (% 7 3))`, `(print (// 7 2))`, `(print (encode_json (range 3)))`,
		`(print (max_by x (list 1) x))`, `(print (min_by x (list 1) x))`, `(print (encode_json (sort_by x (list 1) x)))`,
		`(print (pad_left "a" 3))`, `(print (pad_right "a" 3))`, `(print (str_repeat "a" 3))`,
		`(print (rtrim "a "))`, `(print (ltrim " a"))`, `(print (to_base 5 2))`,
		`(for x (list 1) (break))`, `(print (if false 1 false 2 3))`, `(let ((a b) (list 1 2)) (print a))`,
	}
	for _, f := range forms {
		for _, target := range []string{"go", "js"} {
			dir := t.TempDir()
			p := writeTemp(t, dir, "g.howl", "(do "+f+")")
			o := filepath.Join(dir, "out")
			out, err := exec.Command(bin, "build", "--target", target, p, "-o", o).CombinedOutput()
			if err == nil || !strings.Contains(string(out), "UNSUPPORTED_ON_TARGET") || strings.Contains(string(out), "Built") {
				t.Errorf("%s on %s: err=%v out=%s", f, target, err, out)
			}
			if _, statErr := os.Stat(o); statErr == nil {
				t.Errorf("%s on %s wrote output", f, target)
			}
		}
	}
	dir := t.TempDir()
	p := writeTemp(t, dir, "n.howl", `(do (print (+ 3 -2) (- 5 -2)))`)
	for _, target := range []string{"go", "js"} {
		if out, err := exec.Command(bin, "build", "--target", target, p, "-o", filepath.Join(dir, "n."+target)).CombinedOutput(); err != nil {
			t.Errorf("negative literal on %s: %v %s", target, err, out)
		}
	}
}

// Governed checks fail closed through the new forms: a forbidden construct
// nested in any of them (or in a cond clause) is still rejected, and the
// governed scan no longer mistakes cond's else clause for a construct.
func TestControlflowGovernedFailsClosed(t *testing.T) {
	bin := c1c2Binary(t)
	for _, src := range []string{
		`(do (print (% (exec "ls") 2)))`,
		`(do (print (sort_by x (list 1) (exec "ls"))))`,
		`(do (print (cond ((= 1 1) (exec "ls")) (else 1))))`,
		`(do (print (cond ((= 1 2) 1) (else (exec "ls")))))`,
		`(do (for x (range 2) (do (exec "ls") (break))))`,
		`(do (let ((a b) (list (exec "ls") 1)) (print a)))`,
		`(do (print (if false 1 false (exec "ls") 3)))`,
	} {
		dir := t.TempDir()
		p := writeTemp(t, dir, "g.howl", src)
		out, err := exec.Command(bin, "build", "--profile", "governed", p, "-o", filepath.Join(dir, "g.hfbc")).CombinedOutput()
		if err == nil || !strings.Contains(string(out), "PROFILE_FORBIDDEN_CONSTRUCT") {
			t.Errorf("%s: err=%v out=%s", src, err, out)
		}
	}
}
