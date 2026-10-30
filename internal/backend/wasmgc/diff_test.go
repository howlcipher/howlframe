package wasmgc

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var compilerBin string

// TestMain builds the compiler once: the VM reports a runtime error by
// exiting the process, so both the reference run and the WebAssembly build
// go through the real command line, as tools/difftest does.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "wasmgc-test-*")
	if err != nil {
		panic(err)
	}
	compilerBin = filepath.Join(dir, "howlframe")
	root, _ := filepath.Abs("../../..")
	cmd := exec.Command("go", "build", "-o", compilerBin, "howlframe.go")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		panic(string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type outcome struct {
	stdout string
	failed bool
	errMsg string
}

func run(t *testing.T, name string, args ...string) outcome {
	t.Helper()
	cmd := exec.Command(name, args...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			t.Fatal(err)
		}
	}
	return outcome{stdout: out.String(), failed: err != nil, errMsg: errOut.String() + out.String()}
}

func writeSource(t *testing.T, src string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.howl")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// onVM runs a program on the bytecode VM, the reference semantics.
func onVM(t *testing.T, src string) outcome {
	t.Helper()
	return run(t, compilerBin, writeSource(t, src))
}

// onWasm builds a program with this backend and runs it under Node.
func onWasm(t *testing.T, src string) outcome {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	dir := t.TempDir()
	if out := run(t, compilerBin, "build", writeSource(t, src), "--target", "wasmgc", "-o", dir); out.failed {
		t.Fatalf("build: %s", out.errMsg)
	}
	if tools, err := exec.LookPath("wasm-tools"); err == nil {
		if out, err := exec.Command(tools, "validate", "--features", "all", filepath.Join(dir, "app.wasm")).CombinedOutput(); err != nil {
			t.Fatalf("wasm-tools rejects the module: %v\n%s", err, out)
		}
	}
	return run(t, "node", filepath.Join(dir, "app.mjs"))
}

// sameAsVM requires identical output and, for a failing program, the same
// error message.
func sameAsVM(t *testing.T, src string) {
	t.Helper()
	want := onVM(t, src)
	got := onWasm(t, src)
	if got.failed != want.failed {
		t.Fatalf("failure differs: vm failed=%v (%s), wasm failed=%v (%s)", want.failed, want.errMsg, got.failed, got.errMsg)
	}
	if want.failed {
		// Output before the error, then the message, compared decoded (the VM
		// escapes < and > in its JSON report; the loader does not).
		vmMsg := errorMessage(want.errMsg)
		if vmMsg == "" || !strings.Contains(errorMessage(got.errMsg), vmMsg) {
			t.Fatalf("error differs:\nvm   %s\nwasm %s", want.errMsg, got.errMsg)
		}
		return
	}
	if got.stdout != want.stdout {
		t.Fatalf("output differs:\nvm   %q\nwasm %q", want.stdout, got.stdout)
	}
}

// errorMessage is the message of the JSON error report in a run's output,
// or the whole output when there is none.
func errorMessage(out string) string {
	for _, line := range strings.Split(out, "\n") {
		var report struct {
			Message string `json:"message"`
		}
		if json.Unmarshal([]byte(line), &report) == nil && report.Message != "" {
			return report.Message
		}
	}
	return out
}

func TestDiffValuesAndPrinting(t *testing.T) {
	for name, body := range map[string]string{
		"literals":       `(print 1 "two" 3.5 true false -4)`,
		"empty print":    `(print)`,
		"utf8":           `(print "héllo ✓")`,
		"floats":         `(print 100000.0 1000000.0 123456789.0 0.0001 0.00001 (* 1.0 0.1) (+ 0.1 0.2) 2.0 0.5 -2.5 1.0)`,
		"float extremes": `(print 179769313486231570000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000.0 0.000000000000000000000000001)`,
		"to_string":      `(print (to_string 3.0) (to_string 2.5) (to_string true) (to_string 12))`,
		"concat":         `(print (+ "a" "b") (+ "x" (to_string 7)))`,
		"int arithmetic": `(print (+ 2 3) (- 2 3) (* -4 5) (/ 7 2) (/ -7 2))`,
		"mixed":          `(print (+ 1 2.5) (- 1.5 1) (* 2 0.25) (/ 1 4))`,
		"comparison":     `(print (< 1 2) (> 1 2) (<= 2 2) (>= 1 2) (< 1.5 2) (< "a" "b") (> "b" "ab"))`,
		"equality":       `(print (= 1 1) (= 1 1.0) (= "a" "a") (= "a" "b") (!= 1 2) (= true true) (= true false))`,
		"logic":          `(print (and true false) (or false true) (and true true) (or false false))`,
		"max int":        `(print 9223372036854775807 -9223372036854775807)`,
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, "(cli_app "+body+")") })
	}
}

func TestDiffErrors(t *testing.T) {
	for name, body := range map[string]string{
		"add overflow":   `(print (+ 9223372036854775807 1))`,
		"sub overflow":   `(print (- -9223372036854775807 2))`,
		"mul overflow":   `(print (* 4611686018427387904 2))`,
		"int div zero":   `(print (/ 1 0))`,
		"float div zero": `(print (/ 1.0 0.0))`,

		"output first": `(do (print "before") (print (/ 1 0)) (print "after"))`,
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, "(cli_app "+body+")") })
	}
}

func TestDiffControlFlowAndFunctions(t *testing.T) {
	for name, src := range map[string]string{
		"let and set": `(cli_app (let (x 5 y 2) (do (set x (+ x y)) (print x y))))`,
		"if":          `(cli_app (do (if (> 2 1) (print "yes") (print "no")) (if false (print "never")) (print (if true "a" "b"))))`,
		"while":       `(cli_app (let (i 0 total 0) (do (while (< i 5) (do (set total (+ total i)) (set i (+ i 1)))) (print total))))`,
		"defun":       `(cli_app (defun sq (n) (type_hint n "int") (type_hint return "int") (return (* n n))) (print (call sq 7)))`,
		"recursion": `(cli_app
  (defun fib (n) (type_hint n "int") (type_hint return "int") (if (< n 2) (return n) (return (+ (call fib (- n 1)) (call fib (- n 2))))))
  (print (call fib 12)))`,
		"several body forms": `(cli_app (defun f (m) (type_hint return "string") (if (= m "a") (return "A")) (return "Z")) (print (call f "a") (call f "b")))`,
		"error in function":  `(cli_app (defun boom (n) (type_hint n "int") (type_hint return "float") (return (/ n 0))) (print "start") (print (call boom 3)))`,
		"let as value":       `(cli_app (print (let (a 2) (* a 10))))`,
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, src) })
	}
}

func TestDiffConversions(t *testing.T) {
	for name, body := range map[string]string{
		"to_int":           `(print (to_int 3.7) (to_int -3.7) (to_int "42") (to_int " 42 ") (to_int "-17") (to_int "+5") (to_int 5) (to_int "-9223372036854775808"))`,
		"to_float":         `(print (to_float 3) (to_float "2.5") (to_float "1e3") (to_float " -0.5 ") (to_float 1.5))`,
		"to_int bad text":  `(print (to_int "4.5"))`,
		"to_int word":      `(print (to_int "x"))`,
		"to_int too big":   `(print (to_int "9223372036854775808"))`,
		"to_int bool":      `(print (to_int true))`,
		"to_int big float": `(print (to_int 9223372036854775807.0))`,
		"to_float word":    `(print (to_float "abc"))`,
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, "(cli_app "+body+")") })
	}
}

// An inner let shadows an outer variable of the same name. The interpreter,
// the JavaScript backend and this backend print 2 then 1; the bytecode VM
// printed 2 then 2 until its compiler scoped let bindings (bugs.md #58).
func TestDiffShadowing(t *testing.T) {
	sameAsVM(t, `(cli_app (let (x 1) (do (let (x 2) (print x)) (print x))))`)
	sameAsVM(t, `(cli_app (let (x 10) (do (for x (list 1 2) (print x)) (print x))))`)
}

// % and // are desugared into exact integer code that uses lists and
// string operations.
func TestDiffModulo(t *testing.T) {
	sameAsVM(t, `(cli_app (print (% 7 3) (% -7 3) (% 7 -3) (% 7.5 2) (// 7 2) (// -7 2)))`)
	sameAsVM(t, `(cli_app (print (% 5 0)))`)
}

func TestDiffLists(t *testing.T) {
	for name, body := range map[string]string{
		"literals and printing": `(print (list) (list 1 2) (list "x" "y") (list 1.5 2.0) (list true) (list (list 1) (list 2 3)) (list "a b" ""))`,
		"values not references": `(let (a (list 1 2) b a) (do (append b 3) (print a b)))`,
		"both ends append":      `(let (a (list 1) b a) (do (append a 2) (append b 3) (print a b)))`,
		"function gets a copy": `(defun f (l) (type_hint l "list") (type_hint return "void") (append l 9))
  (let (a (list 1)) (do (call f a) (print a)))`,
		"get and len":        `(let (l (list 10 20)) (print (list_get l 1) (list_len l) (list_get l 5) (list_get l -1) "|"))`,
		"for":                `(let (xs (list 3 1 2) total 0) (do (for x xs (do (print x) (set total (+ total x)))) (print total)))`,
		"for snapshot":       `(let (xs (list 1 2)) (do (for x xs (append xs (* x 10))) (print xs)))`,
		"build in a loop":    `(let (xs (list) i 0) (do (while (< i 50) (do (append xs i) (set i (+ i 1)))) (print (list_len xs) (list_get xs 49))))`,
		"slice and contains": `(let (l (list 1 2 3 4)) (print (list_slice l 1 3) (list_slice l 0 9) (list_slice l 2 1) (list_contains l 3) (list_contains l 7)))`,
		// The VM never treats a list or dict as equal, even to itself.
		"never equal":   `(let (l (list 1)) (print (= (list 1) (list 1)) (= l l) (!= l l) (list_contains (list l) l)))`,
		"nil elements":  `(let (d (dict ("a" 1) ("b" "s"))) (print (list (map_get d "z") 1) (to_string (list (map_get d "z")))))`,
		"len of string": `(print (list_len "héllo") (list_len ""))`,
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, "(cli_app "+body+")") })
	}
}

// mixed is a dict whose values have different types, so the checker leaves
// the type of a map_get unknown and the runtime checks it.
const mixed = `(dict ("a" 1) ("b" "s") ("c" (list 1)) ("d" 2.5) ("e" true) ("n" (dict)))`

func TestDiffDicts(t *testing.T) {
	for name, body := range map[string]string{
		"literals and printing": `(print (dict ("b" 2) ("a" 1)) (dict) (dict ("x" (dict ("y" (list 1 2)))) ("s" "") ("f" 1.5) ("t" false)))`,
		"duplicate key":         `(print (dict ("a" 1) ("b" 2) ("a" 3)))`,
		"key expressions":       `(let (k "z") (print (dict (k 1) ((+ k "2") 2))))`,
		"get":                   `(let (d (dict ("a" 1) ("b" "x"))) (print (map_get d "a") (map_get d "b") (is_nil (map_get d "z")) (+ "[" (+ (to_string (map_get d "z")) "]"))))`,
		"get of an expression":  `(print (map_get (dict ("a" 1)) "a") (map_get (map_get (dict ("a" (dict ("b" 5)))) "a") "b"))`,
		"keys sorted":           `(print (map_keys (dict ("b" 1) ("a" 2) ("é" 3) ("" 4) ("A" 5) ("B" 6) ("ab" 7))) (map_keys (dict)))`,
		"set and order": `(let (d (dict)) (do (map_set d "z" 1) (map_set d "a" 2) (map_set d "m" 3) (map_set d "a" 9)
		  (print d (map_keys d) (list_len (map_keys d)))))`,
		"delete":          `(let (d (dict ("a" 1) ("b" 2) ("c" 3))) (do (map_delete d "a") (map_delete d "zz") (print d) (map_delete d "c") (map_delete d "b") (print d)))`,
		"delete then set": `(let (d (dict ("a" 1) ("b" 2))) (do (map_delete d "a") (map_set d "a" 5) (map_set d "c" 6) (print d (map_get d "a"))))`,
		"has":             `(let (d (dict ("a" 1))) (print (map_has d "a") (map_has d "b")))`,
		"references":      `(let (d (dict) e d) (do (map_set d "k" 1) (print e)))`,
		"function mutates": `(defun f (d) (type_hint d "dict") (type_hint return "void") (map_set d "x" 1))
  (let (d (dict)) (do (call f d) (print d)))`,
		"list values are copies": `(let (d (dict ("c" (list 1))) x (map_get d "c")) (do (append x 2) (print x d)))`,
		"dict in a list":         `(print (list (dict ("a" 1)) (dict)) (to_string (dict ("a" (list 1 2)))))`,
		"never equal":            `(let (d (dict)) (print (= d d) (= (dict) (dict)) (!= d d)))`,
		"many keys": `(let (d (dict) i 0) (do (while (< i 40) (do (map_set d (+ "k" (to_string (- 40 i))) i) (set i (+ i 1))))
		  (print (list_len (map_keys d)) (map_get d "k1") (map_get d "k40") (list_get (map_keys d) 0) (list_get (map_keys d) 39))))`,
		"loop over keys": `(let (d (dict ("b" 2) ("a" 1)) total 0) (do (for k (map_keys d) (set total (+ total (map_get d k)))) (print total)))`,
		// Keys are the text of any value: fmt.Sprint, so nil is "<nil>".
		"stringified keys": `(let (d ` + mixed + ` e (dict)) (do (map_set e (map_get d "a") "i") (map_set e (map_get d "d") "f") (map_set e (map_get d "e") "b") (map_set e (map_get d "zz") "nil")
		  (print e (map_get e (map_get d "a")) (map_get e "1") (map_keys e) (map_has e "1") (map_get e "<nil>"))))`,
		"get of nil":         `(let (d (dict)) (print (map_get (map_get d "a") "b")))`,
		"get of a string":    `(let (d ` + mixed + `) (print (map_get (map_get d "b") "x")))`,
		"get of a list":      `(let (d ` + mixed + `) (print (map_get (map_get d "c") "x")))`,
		"get of a float":     `(let (d ` + mixed + `) (print (map_get (map_get d "d") "x")))`,
		"get of a bool":      `(let (d ` + mixed + `) (print (map_get (map_get d "e") "x")))`,
		"keys of an int":     `(let (d ` + mixed + `) (print (map_keys (map_get d "a"))))`,
		"set on an int":      `(let (d ` + mixed + ` x (map_get d "a")) (map_set x "k" 1))`,
		"delete on a string": `(let (d ` + mixed + ` x (map_get d "b")) (map_delete x "k"))`,
		"for over a dict":    `(let (d ` + mixed + `) (for x (map_get d "n") (print x)))`,
		"for over nil":       `(let (d ` + mixed + `) (for x (map_get d "zz") (print x)))`,
		"len of a dict":      `(let (d ` + mixed + `) (print (list_len (map_get d "n"))))`,
		"len of an int":      `(let (d ` + mixed + `) (print (list_len (map_get d "a"))))`,
		"list_get of an int": `(let (d ` + mixed + ` x (map_get d "a")) (print (list_get x 0)))`,
		"append to a dict":   `(let (d ` + mixed + ` x (map_get d "n")) (append x 0))`,
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, "(cli_app "+body+")") })
	}
}

// The core-library and control-flow sugar desugars into the primitives
// above, so these fixtures pin that each form keeps matching the VM.
func TestDiffSugar(t *testing.T) {
	for name, body := range map[string]string{
		"not":             `(print (not true) (not false) (not (= 1 2)))`,
		"cond":            `(let (x 5) (print (cond ((< x 3) "small") ((< x 10) "mid") (else "big")) (cond ((> x 9) "no") (else "fallback"))))`,
		"switch":          `(let (x "b") (do (switch x ("a" (print "A")) ("b" (print "B")) (else (print "other"))) (switch "q" ("a" (print "A")) (else (print "other")))))`,
		"n-ary if":        `(let (x 5) (print (if (< x 3) "small" (< x 10) "mid" "big") (if (< x 0) "neg" (< x 1) "zero" "pos")))`,
		"inc and dec":     `(let (x 1) (do (inc x) (inc x) (dec x) (print x)))`,
		"str":             `(print (str "a" 1 2.5 true) (str ""))`,
		"nz":              `(let (d (dict ("a" 1))) (print (nz (map_get d "a") 0) (nz (map_get d "z") 9)))`,
		"when and unless": `(let (x 1) (do (when (= x 1) (print "w1") (print "w2")) (unless (= x 1) (print "never")) (unless false (print "u"))))`,
		"n-ary and, or":   `(print (and true true false) (or false false true) (and true true true) (or false false false))`,
		"blank":           `(print (blank "") (blank "x"))`,
		"all and any":     `(print (all x (list 1 2 3) (> x 0)) (any x (list 1 2 3) (> x 2)) (all x (list) false) (any x (list) true))`,
		"versions":        `(print (ver_gt "1.2.3" "1.2.0") (ver_ge "1.0.0" "1.0.0") (ver_gt "1.10.0" "1.9.9") (ver_gt "1.0.0" "1.0.0"))`,
		"index_of":        `(print (index_of (list 3 4 5) 5) (index_of (list 3) 9) (index_of (list "a" "b") "b"))`,
		"map_list":        `(print (map_list x (list 1 2 3) (* x 2)) (map_list x (list) x))`,
		"filter":          `(print (filter x (list 1 2 3 4) (> x 2)) (filter s (list "a" "" "b") (not (blank s))))`,
		"reduce":          `(print (reduce acc 0 x (list 1 2 3) (+ acc x)) (reduce acc "" s (list "a" "b") (+ acc s)))`,
		"sum":             `(print (sum (list 1 2 3)) (sum (list)) (sum (list -4 4)))`,
		"min and max":     `(print (min (list 3 1 2)) (max (list 3 1 2)) (min (list "b" "a")) (max (list 2.5 1.0)))`,
		"min of empty":    `(do (print "before") (print (min (list))))`,
		"reverse":         `(print (reverse (list 1 2 3)) (reverse (list)))`,
		"count_by":        `(print (count_by x (list "a" "b" "a") x))`,
		"sort":            `(print (sort (list 3 1 2 1)) (sort (list "b" "a" "c" "B")) (sort (list)))`,
		"range":           `(print (range 0 5) (range 3 3))`,
		"sort_by":         `(print (sort_by x (list (dict ("n" 2)) (dict ("n" 1)) (dict ("n" 3))) (map_get x "n")))`,
		"max_by, min_by":  `(print (max_by x (list 3 1 2) x) (min_by x (list 3 1 2) x))`,
		"padding":         `(print (pad_left "5" 3 "0") (pad_right "ab" 4 ".") (str_repeat "ab" 3) (pad_left "long" 2 "0"))`,
		"ltrim, rtrim":    `(print (+ "[" (+ (rtrim "  a  ") "]")) (+ "[" (+ (ltrim "  a  ") "]")))`,
		"to_base":         `(print (to_base 255 16) (to_base 5 2) (to_base 0 10))`,
		"records":         `(let (m (rec a (rec b (list 10 20)) name "x")) (print m (get m "a" "b" 1) (is_nil (get m "z"))))`,
		"get through nil": `(let (m ` + mixed + `) (print (get m "zz" "y")))`,
		"put":             `(let (m (dict)) (do (put m "a" "b" 5) (put m "a" "c" 6) (print m)))`,
		"pluck, select":   `(print (pluck (list (rec a 1) (rec a 2)) "a") (select (list (rec a 1 b 2)) a))`,
		"group_by":        `(print (group_by x (list 1 2 3 4) (% x 2)))`,
		"merge":           `(let (a (rec a 1 b 2)) (print (merge a (rec b 3 c 4)) a))`,
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, "(cli_app "+body+")") })
	}
}

func TestDiffStrings(t *testing.T) {
	for name, body := range map[string]string{
		"split":         `(print (str_split "a,b,,c" ",") (str_split "" ",") (str_split "abc" "") (str_split "héllo✓" "") (str_split "a--b--" "--") (str_split "abc" "x"))`,
		"split counts":  `(print (list_len (str_split "a,b,,c" ",")) (list_len (str_split "" "")) (list_len (str_split "é✓" "")))`,
		"join":          `(print (str_join (list "a" "b" "c") ", ") (str_join (list) "-") (str_join (list 1 2) "+"))`,
		"case":          `(print (to_lower "Hello WORLD 1") (to_upper "Hello world 1") (to_lower 12))`,
		"trim":          `(print (+ "[" (+ (str_trim "  a b \n\t") "]")))`,
		"escape":        `(print (html_escape "<a href=\"x\">Tom & 'Jerry'</a>") (attr_escape "a\"b"))`,
		"is_nil":        `(let (l (list "a")) (print (is_nil (list_get l 5)) (is_nil (list_get l 0)) (+ "[" (+ (list_get l 5) "]"))))`,
		"len and slice": `(print (str_len "héllo✓") (str_slice "héllo" 1 3) (str_slice "abc" 1) (str_len ""))`,
		"contains":      `(print (str_contains "hello" "ell") (str_contains "hello" "xyz") (starts_with "hello" "he") (ends_with "hello" "lo") (starts_with "hi" "hello"))`,
		"replace":       `(print (str_replace "a-b-c" "-" "+") (str_replace "aaa" "a" "bb"))`,
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, "(cli_app "+body+")") })
	}
}

func TestDiffStderrAndExit(t *testing.T) {
	for name, body := range map[string]string{
		"exit status":     `(do (print "a") (exit 3) (print "never"))`,
		"exit zero":       `(do (print "a") (exit 0))`,
		"stderr and exit": `(do (stderr "problem") (exit 2))`,
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, "(cli_app "+body+")") })
	}
}
