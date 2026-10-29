package wasmgc

import (
	"bytes"
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
		// Output before the error, then the message.
		vmMsg := want.errMsg
		if i := strings.Index(vmMsg, `"message":"`); i >= 0 {
			vmMsg = strings.SplitN(vmMsg[i+len(`"message":"`):], `"`, 2)[0]
		}
		if !strings.Contains(got.errMsg, vmMsg) {
			t.Fatalf("error differs:\nvm   %s\nwasm %s", want.errMsg, got.errMsg)
		}
		return
	}
	if got.stdout != want.stdout {
		t.Fatalf("output differs:\nvm   %q\nwasm %q", want.stdout, got.stdout)
	}
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
// printed 2 then 2 until its compiler scoped let bindings (bugs.md).
func TestDiffShadowing(t *testing.T) {
	t.Skip("blocked on the bytecode VM's let scoping fix (bugs.md, VM let shadowing)")
	sameAsVM(t, `(cli_app (let (x 1) (do (let (x 2) (print x)) (print x))))`)
}

// % and // are desugared into exact integer code that uses lists and
// string operations.
func TestDiffModulo(t *testing.T) {
	t.Skip("needs string operations (str_split; next M1 slice)")
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
		"identity equality":  `(print (= (list 1) (list 1)))`,
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, "(cli_app "+body+")") })
	}
}
