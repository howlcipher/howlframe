package wasmgc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	before, _ := filepath.Glob("*.hfbc")
	code := m.Run()
	os.RemoveAll(dir)
	// Runs happen in temporary directories; none may leave bytecode here.
	if after, _ := filepath.Glob("*.hfbc"); len(after) > len(before) && code == 0 {
		fmt.Fprintf(os.Stderr, "tests left bytecode in the package directory: %v\n", after)
		code = 1
	}
	os.Exit(code)
}

type outcome struct {
	stdout string
	failed bool
	code   int // exit status
	errMsg string
}

// run runs a command in a fresh temporary directory: the bare
// `howlframe prog.howl` path writes prog.hfbc into the working directory,
// which must not be the package. Every path passed in is absolute.
func run(t *testing.T, name string, args ...string) outcome {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = t.TempDir()
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	code := 0
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		code = exit.ExitCode()
	}
	return outcome{stdout: out.String(), failed: err != nil, code: code, errMsg: errOut.String() + out.String()}
}

func writeSource(t *testing.T, src string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.howl")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeFiles writes a program of several files (name -> source) into one
// directory and returns the path of main.
func writeFiles(t *testing.T, files map[string]string, main string) string {
	t.Helper()
	dir := t.TempDir()
	for name, src := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, main)
}

// onVM runs a program on the bytecode VM, the reference semantics.
func onVM(t *testing.T, src string) outcome {
	t.Helper()
	return onVMAt(t, writeSource(t, src))
}

func onVMAt(t *testing.T, path string) outcome {
	t.Helper()
	return run(t, compilerBin, path)
}

// onWasm builds a program with this backend and runs it under Node.
func onWasm(t *testing.T, src string) outcome {
	t.Helper()
	return onWasmAt(t, writeSource(t, src))
}

func onWasmAt(t *testing.T, path string) outcome {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	dir := t.TempDir()
	if out := run(t, compilerBin, "build", path, "--target", "wasmgc", "-o", dir); out.failed {
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
	path := writeSource(t, src)
	sameAsVMAt(t, path)
}

// sameAsVMAt is sameAsVM for a program already on disk (for example one
// that uses modules).
func sameAsVMAt(t *testing.T, path string) {
	t.Helper()
	want := onVMAt(t, path)
	matchVM(t, "wasm", want, onWasmAt(t, path))
	// The same program as a WASI command under wasmtime, when installed.
	if _, err := exec.LookPath("wasmtime"); err == nil {
		matchVM(t, "wasmtime", want, onWasmtimeAt(t, path))
	}
}

// onWasmtimeAt builds a program for a WASI host and runs it in wasmtime.
func onWasmtimeAt(t *testing.T, path string) outcome {
	t.Helper()
	dir := t.TempDir()
	if out := run(t, compilerBin, "build", path, "--target", "wasmgc", "--host", "wasi", "-o", dir); out.failed {
		t.Fatalf("build (wasi): %s", out.errMsg)
	}
	return run(t, "wasmtime", "run", "-W", "gc=y,function-references=y", filepath.Join(dir, "app.wasm"))
}

// matchVM requires identical output and, for a failing program, the same
// error code and message.
func matchVM(t *testing.T, engine string, want, got outcome) {
	t.Helper()
	if got.failed != want.failed || got.code != want.code {
		t.Fatalf("failure differs: vm exit %d (%s), %s exit %d (%s)", want.code, want.errMsg, engine, got.code, got.errMsg)
	}
	if want.failed {
		// Output before the error, then the message, compared decoded (the VM
		// escapes < and > in its JSON report; the loader does not).
		vmCode, vmMsg := errorReport(want.errMsg)
		gotCode, gotMsg := errorReport(got.errMsg)
		if vmMsg == "" || vmCode != gotCode || !strings.Contains(gotMsg, vmMsg) {
			t.Fatalf("error differs:\nvm   %s\n%s %s", want.errMsg, engine, got.errMsg)
		}
		return
	}
	if got.stdout != want.stdout {
		t.Fatalf("output differs:\nvm   %q\n%s %q", want.stdout, engine, got.stdout)
	}
}

// errorReport is the code and message of the JSON error report in a run's
// output, or no code and the whole output when there is none.
func errorReport(out string) (code, message string) {
	for _, line := range strings.Split(out, "\n") {
		var report struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal([]byte(line), &report) == nil && report.Message != "" {
			return report.Code, report.Message
		}
	}
	return "", out
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
		// Float arithmetic whose result is not finite is an error on the VM.
		"float overflow":    `(let (x 1.0 i 0) (do (while (< i 307) (do (set x (* x 10.0)) (set i (+ i 1)))) (print x) (print (* x 100))))`,
		"float add":         `(let (x 1.0 i 0) (do (while (< i 308) (do (set x (* x 10.0)) (set i (+ i 1)))) (print (+ x (* x 0.9)))))`,
		"float sub":         `(let (x 1.0 i 0) (do (while (< i 308) (do (set x (* x 10.0)) (set i (+ i 1)))) (print (- (- 0.0 x) (* x 0.9)))))`,
		"division overflow": `(let (x 1.0 i 0) (do (while (< i 307) (do (set x (* x 10.0)) (set i (+ i 1)))) (print (/ x 0.01))))`,
		"small floats":      `(print (* 0.000001 0.5) (* 1.5 -2) (/ 1 3))`,
		// Operands of unknown type are checked at run time.
		"add a list":          `(let (d ` + mixed + `) (print (+ (map_get d "a") (map_get d "c"))))`,
		"subtract a string":   `(let (d ` + mixed + `) (print (- (map_get d "b") 1)))`,
		"multiply a bool":     `(let (d ` + mixed + `) (print (* (map_get d "e") 2)))`,
		"divide a string":     `(let (d ` + mixed + `) (print (/ (map_get d "b") 2)))`,
		"compare mixed":       `(let (d ` + mixed + `) (print (< (map_get d "a") (map_get d "b"))))`,
		"compare a list":      `(let (d ` + mixed + `) (print (>= (map_get d "c") 1)))`,
		"modulo a string":     `(let (d ` + mixed + `) (print (% (map_get d "b") 2)))`,
		"if of an int":        `(let (d ` + mixed + `) (if (map_get d "a") (print 1)))`,
		"if of nil":           `(let (d ` + mixed + `) (if (map_get d "zz") (print 1)))`,
		"while of a string":   `(let (d ` + mixed + `) (while (map_get d "b") (print 1)))`,
		"and of an int":       `(let (d ` + mixed + `) (print (and true (map_get d "a"))))`,
		"or of an int":        `(let (d ` + mixed + `) (print (or false (map_get d "a"))))`,
		"not of an int":       `(let (d ` + mixed + `) (print (not (map_get d "a"))))`,
		"logic of bools":      `(let (d ` + mixed + `) (print (and (map_get d "e") (map_get d "e")) (and false (= (/ 1 0) 1)) (or true (= (/ 1 0) 1))))`,
		"split an int":        `(let (d ` + mixed + `) (print (str_split (map_get d "a") ",")))`,
		"split by an int":     `(let (d ` + mixed + `) (print (str_split "a" (map_get d "a"))))`,
		"join a string":       `(let (d ` + mixed + `) (print (str_join (map_get d "b") ",")))`,
		"escape an int":       `(let (d ` + mixed + `) (print (html_escape (map_get d "a"))))`,
		"attr_escape a float": `(let (d ` + mixed + `) (print (attr_escape (map_get d "d"))))`,
		"to_int of a list":    `(let (d ` + mixed + `) (print (to_int (map_get d "c"))))`,
		"to_float of a dict":  `(let (d ` + mixed + `) (print (to_float (map_get d "n"))))`,
		"to_lower of a list":  `(let (d ` + mixed + `) (print (to_lower (map_get d "c")) (to_upper (map_get d "e"))))`,
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, "(cli_app "+body+")") })
	}
}

// try_let binds the value of an expression, or runs its catch body with
// the error when the expression (or anything it calls) fails. The error is
// the VM's JSON report; its "instruction" and "opcode" fields describe the
// VM's own code, so fixtures read only its code and message.
func TestDiffTryLet(t *testing.T) {
	has := func(field string) string {
		return `(print (str_contains e "\"code\":\"` + field + `\""))`
	}
	for name, body := range map[string]string{
		"success":            `(try_let (v (+ 1 2)) (catch e (print "caught")) (print "ok" v))`,
		"caught":             `(try_let (v (/ 1 0)) (catch e (do (print "caught") ` + has("RUNTIME_ERROR") + ` (print (str_contains e "\"message\":\"division by zero\"")))) (print "ok" v))`,
		"type error":         `(let (d (dict)) (try_let (v (map_get (map_get d "z") "q")) (catch e (do ` + has("TYPE_ERROR") + ` (print (str_contains e "got \\u003cnil\\u003e")))) (print v)))`,
		"conversion error":   `(try_let (v (to_int "x")) (catch e (do ` + has("CONVERSION_ERROR") + ` (print (str_contains e "cannot convert \\\"x\\\" to int")))) (print v))`,
		"overflow":           `(try_let (v (* 4611686018427387904 2)) (catch e ` + has("RUNTIME_ERROR") + `) (print v))`,
		"error report shape": `(try_let (v (/ 1 0)) (catch e (print (starts_with e "{\"phase\":\"runtime\",\"code\":\"RUNTIME_ERROR\",\"function\":\"main\",") (ends_with e "\"message\":\"division by zero\"}"))) (print v))`,
		"from a function": `(defun f (n) (type_hint n "int") (type_hint return "float") (return (/ n 0)))
  (try_let (v (call f 1)) (catch e (print "fn caught")) (print v))`,
		"return from catch": `(defun g (n) (type_hint n "int") (type_hint return "string") (try_let (v (/ 1 n)) (catch e (return "from catch")) (return (+ "ok " (to_string v)))))
  (print (call g 0) (call g 2))`,
		"error in catch":     `(try_let (v (/ 1 0)) (catch e (print (/ 2 0))) (print v))`,
		"error in success":   `(try_let (v 1) (catch e (print "never")) (print (/ v 0)))`,
		"exit is not caught": `(try_let (v (min (list))) (catch e (print "never")) (print v))`,
		"as a value":         `(print (try_let (v 1) (catch e 0) v) (try_let (v (/ 1 0)) (catch e "c") (+ "ok" (to_string v))))`,
		"in sequence":        `(do (try_let (v (/ 1 0)) (catch e (print "c1")) (print "s")) (try_let (v (/ 1 1)) (catch e (print "c2")) (print "s2" v)))`,
		"nested":             `(try_let (v (try_let (w (/ 1 0)) (catch e2 (/ 2 0)) w)) (catch e (print "outer")) (print v))`,
		"inner handled":      `(try_let (v (try_let (w (/ 1 0)) (catch e2 7) w)) (catch e (print "outer")) (print "inner" v))`,
		"in a loop":          `(let (n 0) (do (for x (list 1 0 2 0) (try_let (v (/ 10 x)) (catch e (set n (+ n 1))) (print v))) (print n)))`,
		"partial output":     `(try_let (v (do (print "side effect") (/ 1 0))) (catch e (print "caught")) (print v))`,
		"uncaught after try": `(do (try_let (v 1) (catch e (print "never")) (print v)) (print (/ 1 0)))`,
		"message escaping":   `(try_let (v (to_int "<a&b>\"q\n\té✓")) (catch e (print (str_contains e "\\u003ca\\u0026b\\u003e\\\\\\\"q\\\\n\\\\té✓\\\" to int\"}"))) (print v))`,
	} {
		t.Run(name, func(t *testing.T) {
			// The checks print true; a false on the VM is a wrong fixture.
			if out := onVM(t, "(cli_app "+body+")"); strings.Contains(out.stdout, "false") {
				t.Fatalf("fixture check is false on the VM: %q", out.stdout)
			}
			sameAsVM(t, "(cli_app "+body+")")
		})
	}
}

// Modules are linked before any backend runs: each used module's
// definitions join the program under alias_name (exports) and
// alias_private_name (the rest), so one WebAssembly module holds them all.
func TestDiffModules(t *testing.T) {
	geo := `(module
  (defun square (n) (type_hint n "int") (type_hint return "int") (return (* n n)))
  (export (defun area (w h) (type_hint w "int") (type_hint h "int") (type_hint return "int") (return (call square (+ w h)))))
  (export (defun label (n) (type_hint n "int") (type_hint return "string") (return (+ "n=" (to_string (call square n)))))))`
	text := `(module
  (export (defun wrap (s) (type_hint s "string") (type_hint return "string") (return (+ "[" (+ s "]")))))
  (export (defun tally (words) (type_hint words "list") (type_hint return "dict")
    (let (d (dict)) (do (for w words (map_set d w (+ (nz (map_get d w) 0) 1))) (return d)))))
  (export (defun safe_int (s) (type_hint s "string") (type_hint return "int")
    (try_let (n (to_int s)) (catch e (return -1)) (return n))))
  (export (defun boom () (type_hint return "float") (return (/ 1 0)))))`
	for name, tc := range map[string]struct {
		files map[string]string
	}{
		"exports and a private helper": {map[string]string{"geo.howl": geo,
			"main.howl": `(cli_app (use "geo.howl" as geo) (print (call geo/area 2 3) (call geo/label 4)))`}},
		"a main function shares a private name": {map[string]string{"geo.howl": geo,
			"main.howl": `(cli_app (use "geo.howl" as geo) (defun square (n) (type_hint n "int") (type_hint return "int") (return (+ n 1000))) (print (call geo/area 1 1) (call square 1)))`}},
		"another alias": {map[string]string{"geo.howl": geo,
			"main.howl": `(cli_app (use "geo.howl" as g) (print (call g/area 1 1)))`}},
		"two modules": {map[string]string{"geo.howl": geo, "text.howl": text,
			"main.howl": `(cli_app (use "geo.howl" as geo) (use "text.howl" as text) (print (call text/wrap (call geo/label 3)) (call text/tally (list "a" "b" "a"))))`}},
		"a subdirectory": {map[string]string{"lib/text.howl": text,
			"main.howl": `(cli_app (use "lib/text.howl" as text) (print (call text/safe_int "42") (call text/safe_int "x")))`}},
		"an error inside a module": {map[string]string{"text.howl": text,
			"main.howl": `(cli_app (use "text.howl" as text) (print "before") (print (call text/boom)))`}},
		"caught from a module": {map[string]string{"text.howl": text,
			"main.howl": `(cli_app (use "text.howl" as text) (print (try_let (v (call text/boom)) (catch e "caught") (to_string v))))`}},
	} {
		t.Run(name, func(t *testing.T) { sameAsVMAt(t, writeFiles(t, tc.files, "main.howl")) })
	}
}

// encode_json is standard-library HowlFrame (std/json.howl) on byte
// intrinsics; the VM's is Go's encoding/json.
func TestDiffEncodeJSON(t *testing.T) {
	for name, body := range map[string]string{
		"scalars":         `(print (encode_json 1) (encode_json -7) (encode_json 9223372036854775807) (encode_json "plain") (encode_json true) (encode_json false) (encode_json ""))`,
		"nil":             `(let (m (dict ("a" 1) ("n" (dict)))) (print (encode_json (map_get m "zz")) (encode_json (list (map_get m "zz")))))`,
		"collections":     `(print (encode_json (list)) (encode_json (dict)) (encode_json (list 1 2 3)) (encode_json (list (list "b") (list))) (encode_json (dict ("b" 2) ("a" (dict ("y" 1) ("x" (list)))))))`,
		"mixed record":    `(let (m (dict ("a" 1) ("n" (dict)))) (print (encode_json (dict ("b" (list 1 2)) ("f" 2.5) ("t" true) ("nil" (map_get m "zz")) ("e" (list)) ("d" (dict)) ("" -0.0)))))`,
		"key order":       `(print (encode_json (dict ("b" 1) ("a" 2) ("é" 3) ("" 4) ("A" 5) ("ab" 6))))`,
		"escapes":         `(print (encode_json "q\"\\<>&\n\t\r") (encode_json (dict ("<k>" "v&"))))`,
		"unicode":         `(print (encode_json "héllo ✓ 日本 🎉") (encode_json "LS` + " " + `PS` + " " + `."))`,
		"floats":          `(print (encode_json 2.5) (encode_json -2.5) (encode_json 3.0) (encode_json 0.1) (encode_json (+ 0.1 0.2)) (encode_json 123456789.125) (encode_json 0.00001234) (encode_json -0.0) (encode_json 0.0))`,
		"float layout":    `(print (encode_json 0.000001) (encode_json 0.0000001) (encode_json 0.00000123) (encode_json 100000000000000000000.0) (encode_json 1000000000000000000000.0) (encode_json 12345678901234567890123.0))`,
		"float extremes":  `(let (x 1.0 i 0) (do (while (< i 120) (do (set x (* x 10.0)) (set i (+ i 1)))) (print (encode_json x) (encode_json (/ 1.0 x)) (encode_json (- 0.0 x)) (encode_json (list x (/ 3.0 x))))))`,
		"in a function":   `(defun row (n) (type_hint n "int") (type_hint return "string") (return (encode_json (dict ("n" n) ("sq" (* n n)))))) (print (call row 3) (call row 4))`,
		"round trip text": `(let (s (encode_json (list "a" "b"))) (print s (str_len s) (+ s s)))`,
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, "(cli_app "+body+")") })
	}
}

// regex_match is standard-library HowlFrame (std/regex.howl): a Thompson
// NFA over the RE2 subset HowlScene HF and the corpus use. The VM's is Go's
// regexp.MatchString.
func TestDiffRegex(t *testing.T) {
	for name, body := range map[string]string{
		"literals":                   "(print (regex_match \"abc\" \"abc\") (regex_match \"abc\" \"xabcx\") (regex_match \"abc\" \"ab\") (regex_match \"abc\" \"\") (regex_match \"abc\" \"ABC\"))",
		"empty pattern":              "(print (regex_match \"\" \"\") (regex_match \"\" \"x\"))",
		"anchors":                    "(print (regex_match \"^ab$\" \"ab\") (regex_match \"^ab$\" \"abc\") (regex_match \"^ab$\" \"cab\") (regex_match \"^ab$\" \"a\\nb\"))",
		"start anchor":               "(print (regex_match \"^https?://\" \"http://x\") (regex_match \"^https?://\" \"https://x\") (regex_match \"^https?://\" \"xhttp://\") (regex_match \"^https?://\" \"ftp://\"))",
		"dot":                        "(print (regex_match \"a.c\" \"abc\") (regex_match \"a.c\" \"a\\nc\") (regex_match \"a.c\" \"aéc\") (regex_match \"a.c\" \"ac\"))",
		"classes":                    "(print (regex_match \"^[a-z0-9][a-z0-9-]{0,39}$\" \"hello-1\") (regex_match \"^[a-z0-9][a-z0-9-]{0,39}$\" \"-x\") (regex_match \"^[a-z0-9][a-z0-9-]{0,39}$\" \"Hello\") (regex_match \"^[a-z0-9][a-z0-9-]{0,39}$\" \"a\") (regex_match \"^[a-z0-9][a-z0-9-]{0,39}$\" \"abbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\") (regex_match \"^[a-z0-9][a-z0-9-]{0,39}$\" \"abbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\") (regex_match \"^[a-z0-9][a-z0-9-]{0,39}$\" \"é\"))",
		"hex colour":                 "(print (regex_match \"^#[0-9a-fA-F]{6}$\" \"#00ff0a\") (regex_match \"^#[0-9a-fA-F]{6}$\" \"#00FF0G\") (regex_match \"^#[0-9a-fA-F]{6}$\" \"#abc\") (regex_match \"^#[0-9a-fA-F]{6}$\" \"#abcdef0\"))",
		"negated class":              "(print (regex_match \"^[^a-c]+$\" \"xyz\") (regex_match \"^[^a-c]+$\" \"xaz\") (regex_match \"^[^a-c]+$\" \"é✓\") (regex_match \"^[^a-c]+$\" \"\\n\"))",
		"class edge chars":           "(print (regex_match \"^[]a-]+$\" \"]a-\") (regex_match \"^[]a-]+$\" \"b\"))",
		"perl classes":               "(print (regex_match \"\\\\d+\\\\s\\\\w\" \"12 a\") (regex_match \"\\\\d+\\\\s\\\\w\" \"12a\") (regex_match \"\\\\d+\\\\s\\\\w\" \" 1 _\"))",
		"negated perl":               "(print (regex_match \"^\\\\D\\\\S\\\\W$\" \"a!.\") (regex_match \"^\\\\D\\\\S\\\\W$\" \"1!.\") (regex_match \"^\\\\D\\\\S\\\\W$\" \"a .\") (regex_match \"^\\\\D\\\\S\\\\W$\" \"aé.\"))",
		"class with perl":            "(print (regex_match \"^[^\\\\s<>\\\"'`\\\\\\\\]*$\" \"plain/path?q=1\") (regex_match \"^[^\\\\s<>\\\"'`\\\\\\\\]*$\" \"has space\") (regex_match \"^[^\\\\s<>\\\"'`\\\\\\\\]*$\" \"a<b\") (regex_match \"^[^\\\\s<>\\\"'`\\\\\\\\]*$\" \"back\\\\slash\") (regex_match \"^[^\\\\s<>\\\"'`\\\\\\\\]*$\" \"quo\\\"te\") (regex_match \"^[^\\\\s<>\\\"'`\\\\\\\\]*$\" \"tick`\"))",
		"alternation":                "(print (regex_match \"^(cat|dog|)$\" \"cat\") (regex_match \"^(cat|dog|)$\" \"dog\") (regex_match \"^(cat|dog|)$\" \"\") (regex_match \"^(cat|dog|)$\" \"cow\") (regex_match \"^(cat|dog|)$\" \"catdog\"))",
		"nested groups":              "(print (regex_match \"^(a(b|c)*d)+$\" \"ad\") (regex_match \"^(a(b|c)*d)+$\" \"abcd\") (regex_match \"^(a(b|c)*d)+$\" \"abdacd\") (regex_match \"^(a(b|c)*d)+$\" \"abc\") (regex_match \"^(a(b|c)*d)+$\" \"\"))",
		"non-capturing":              "(print (regex_match \"^(?:ab)+$\" \"abab\") (regex_match \"^(?:ab)+$\" \"aba\"))",
		"named group":                "(print (regex_match \"^(?P<y>[0-9]{4})-(?<m>[0-9]{2})$\" \"2026-10\") (regex_match \"^(?P<y>[0-9]{4})-(?<m>[0-9]{2})$\" \"26-10\"))",
		"repetition":                 "(print (regex_match \"^a{2}b{2,}c{1,3}$\" \"aabbc\") (regex_match \"^a{2}b{2,}c{1,3}$\" \"aabbbbccc\") (regex_match \"^a{2}b{2,}c{1,3}$\" \"abbc\") (regex_match \"^a{2}b{2,}c{1,3}$\" \"aabbcccc\") (regex_match \"^a{2}b{2,}c{1,3}$\" \"aab\"))",
		"optional":                   "(print (regex_match \"^colou?r$\" \"color\") (regex_match \"^colou?r$\" \"colour\") (regex_match \"^colou?r$\" \"colouur\"))",
		"lazy":                       "(print (regex_match \"^a+?b*?$\" \"aab\") (regex_match \"^a+?b*?$\" \"a\") (regex_match \"^a+?b*?$\" \"b\"))",
		"literal brace":              "(print (regex_match \"a{,2}x{y}\" \"a{,2}x{y}\") (regex_match \"a{,2}x{y}\" \"aa\"))",
		"escaped metas":              "(print (regex_match \"^\\\\.\\\\*\\\\+\\\\?\\\\(\\\\)\\\\[\\\\]\\\\{\\\\}\\\\|\\\\^\\\\$\\\\\\\\$\" \".*+?()[]{}|^$\\\\\") (regex_match \"^\\\\.\\\\*\\\\+\\\\?\\\\(\\\\)\\\\[\\\\]\\\\{\\\\}\\\\|\\\\^\\\\$\\\\\\\\$\" \"x\"))",
		"control escapes":            "(print (regex_match \"^\\\\t\\\\n\\\\r$\" \"\\t\\n\\r\") (regex_match \"^\\\\t\\\\n\\\\r$\" \"tnr\"))",
		"empty loops":                "(print (regex_match \"^(a*)*$\" \"\") (regex_match \"^(a*)*$\" \"aaa\") (regex_match \"^(a*)*$\" \"b\"))",
		"versions":                   "(print (regex_match \"^[0-9]+[.][0-9]+[.][0-9]+$\" \"1.2.3\") (regex_match \"^[0-9]+[.][0-9]+[.][0-9]+$\" \"10.20.30\") (regex_match \"^[0-9]+[.][0-9]+[.][0-9]+$\" \"1.2\") (regex_match \"^[0-9]+[.][0-9]+[.][0-9]+$\" \"1.2.3.4\") (regex_match \"^[0-9]+[.][0-9]+[.][0-9]+$\" \"a.b.c\"))",
		"language tags":              "(print (regex_match \"^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})?$\" \"en\") (regex_match \"^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})?$\" \"en-GB\") (regex_match \"^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})?$\" \"zh-Hant\") (regex_match \"^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})?$\" \"e\") (regex_match \"^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})?$\" \"en-\") (regex_match \"^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})?$\" \"english\"))",
		"script hash":                "(print (regex_match \"^[A-Za-z0-9+/]{43}=$\" \"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\") (regex_match \"^[A-Za-z0-9+/]{43}=$\" \"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\") (regex_match \"^[A-Za-z0-9+/]{43}=$\" \"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\"))",
		"http url":                   "(print (regex_match \"^https?://[a-z0-9.-]+(:[0-9]{1,5})?([/?#][^\\\\s<>\\\"'`\\\\\\\\]*)?$\" \"https://example.com\") (regex_match \"^https?://[a-z0-9.-]+(:[0-9]{1,5})?([/?#][^\\\\s<>\\\"'`\\\\\\\\]*)?$\" \"http://a.b:8080/x?y=1#z\") (regex_match \"^https?://[a-z0-9.-]+(:[0-9]{1,5})?([/?#][^\\\\s<>\\\"'`\\\\\\\\]*)?$\" \"https://bad host\") (regex_match \"^https?://[a-z0-9.-]+(:[0-9]{1,5})?([/?#][^\\\\s<>\\\"'`\\\\\\\\]*)?$\" \"https://x.org/<b>\") (regex_match \"^https?://[a-z0-9.-]+(:[0-9]{1,5})?([/?#][^\\\\s<>\\\"'`\\\\\\\\]*)?$\" \"https://x.org:123456\"))",
		"mailto":                     "(print (regex_match \"^mailto:[^\\\\s<>\\\"'`\\\\\\\\@]+@[^\\\\s<>\\\"'`\\\\\\\\@]+$\" \"mailto:a@b.c\") (regex_match \"^mailto:[^\\\\s<>\\\"'`\\\\\\\\@]+@[^\\\\s<>\\\"'`\\\\\\\\@]+$\" \"mailto:a@b@c\") (regex_match \"^mailto:[^\\\\s<>\\\"'`\\\\\\\\@]+@[^\\\\s<>\\\"'`\\\\\\\\@]+$\" \"mailto:@b\"))",
		"tel":                        "(print (regex_match \"^tel:\\\\+?[0-9 ().-]{3,30}$\" \"tel:+1 (555) 010-0000\") (regex_match \"^tel:\\\\+?[0-9 ().-]{3,30}$\" \"tel:12\") (regex_match \"^tel:\\\\+?[0-9 ().-]{3,30}$\" \"tel:abc\"))",
		"relative path":              "(print (regex_match \"^(\\\\./|(\\\\.\\\\./)+)?[A-Za-z0-9_~-][A-Za-z0-9._~-]*(/[A-Za-z0-9_~-][A-Za-z0-9._~-]*)*/?(#[a-z0-9][a-z0-9-]{0,39})?$\" \"../img/a.png#top\") (regex_match \"^(\\\\./|(\\\\.\\\\./)+)?[A-Za-z0-9_~-][A-Za-z0-9._~-]*(/[A-Za-z0-9_~-][A-Za-z0-9._~-]*)*/?(#[a-z0-9][a-z0-9-]{0,39})?$\" \"./a\") (regex_match \"^(\\\\./|(\\\\.\\\\./)+)?[A-Za-z0-9_~-][A-Za-z0-9._~-]*(/[A-Za-z0-9_~-][A-Za-z0-9._~-]*)*/?(#[a-z0-9][a-z0-9-]{0,39})?$\" \"a/b/\") (regex_match \"^(\\\\./|(\\\\.\\\\./)+)?[A-Za-z0-9_~-][A-Za-z0-9._~-]*(/[A-Za-z0-9_~-][A-Za-z0-9._~-]*)*/?(#[a-z0-9][a-z0-9-]{0,39})?$\" \"/abs\") (regex_match \"^(\\\\./|(\\\\.\\\\./)+)?[A-Za-z0-9_~-][A-Za-z0-9._~-]*(/[A-Za-z0-9_~-][A-Za-z0-9._~-]*)*/?(#[a-z0-9][a-z0-9-]{0,39})?$\" \"a//b\") (regex_match \"^(\\\\./|(\\\\.\\\\./)+)?[A-Za-z0-9_~-][A-Za-z0-9._~-]*(/[A-Za-z0-9_~-][A-Za-z0-9._~-]*)*/?(#[a-z0-9][a-z0-9-]{0,39})?$\" \"../../x.css\"))",
		"unicode":                    "(print (regex_match \"^[é✓]+日$\" \"é✓é日\") (regex_match \"^[é✓]+日$\" \"e日\") (regex_match \"^[é✓]+日$\" \"✓\"))",
		"search inside":              "(print (regex_match \"[0-9]{3}\" \"ab123cd\") (regex_match \"[0-9]{3}\" \"ab12cd\"))",
		"missing paren":              "(print (regex_match \"(abc\" \"abc\"))",
		"unexpected paren":           "(print (regex_match \"ab)\" \"abc\"))",
		"missing bracket":            "(print (regex_match \"[abc\" \"abc\"))",
		"missing argument":           "(print (regex_match \"*a\" \"abc\"))",
		"missing argument after bar": "(print (regex_match \"x|+\" \"abc\"))",
		"missing argument brace":     "(print (regex_match \"{2}a\" \"abc\"))",
		"nested repetition":          "(print (regex_match \"a**\" \"abc\"))",
		"nested brace":               "(print (regex_match \"a{2}{3}\" \"abc\"))",
		"trailing backslash":         "(print (regex_match \"a\\\\\" \"abc\"))",
		"invalid escape":             "(print (regex_match \"a\\\\q\" \"abc\"))",
		"invalid range":              "(print (regex_match \"[z-a]\" \"abc\"))",
		"range to a class":           "(print (regex_match \"[a-\\\\d]\" \"abc\"))",
		"repeat too big":             "(print (regex_match \"a{1001}\" \"abc\"))",
		"repeat reversed":            "(print (regex_match \"a{3,2}\" \"abc\"))",
		"in a function":              `(defun valid (id) (type_hint id "string") (type_hint return "bool") (return (regex_match "^img-[a-z0-9]{6,32}$" id))) (print (call valid "img-abc123") (call valid "img-ab"))`,
		"long text":                  `(let (s "") (do (for i (range 0 200) (set s (+ s "ab"))) (print (regex_match "^(ab)*$" s) (regex_match "^(a|b)*c" s) (regex_match "(ab){200}" s))))`,
		"dynamic pattern":            `(let (parts (list "^[a-z]" "+$")) (print (regex_match (str_join parts "") "abc") (regex_match (str_join parts "") "ab1")))`,
		"caught":                     `(print (try_let (v (regex_match "(" "x")) (catch e (str_contains e "INVALID_REGEX")) v))`,
		"wrong type":                 `(let (d ` + mixed + `) (print (regex_match "a" (map_get d "a"))))`,
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, "(cli_app "+body+")") })
	}
}

// Syntax outside the supported subset fails closed instead of matching
// differently from the VM.
func TestRegexUnsupportedFailsClosed(t *testing.T) {
	for _, pattern := range []string{"\"(?i)a\"", "\"\\\\bx\"", "\"\\\\pL\"", "\"\\\\x41\"", "\"[[:alpha:]]\"", "\"\\\\101\"", "\"\\\\Qa\\\\E\"", "\"\\\\Az\""} {
		t.Run(pattern, func(t *testing.T) {
			got := onWasm(t, "(cli_app (print (regex_match "+pattern+" \"abc\")))")
			if code, msg := errorReport(got.errMsg); !got.failed || code != "WASM_UNSUPPORTED" {
				t.Fatalf("want WASM_UNSUPPORTED, got failed=%v %s %s", got.failed, code, msg)
			}
		})
	}
}

// parse_json is standard-library HowlFrame (std/json.howl); the VM's is
// Go 1.25's json.Decoder with UseNumber and a check for a second value,
// which the VM reproduces itself, whatever the toolchain (bugs.md #63).
func TestDiffParseJSON(t *testing.T) {
	for name, body := range map[string]string{
		"object":                       "(let (t \"{\\\"a\\\":1,\\\"b\\\":[true,null,2.5,\\\"x\\\"],\\\"a\\\":3}\") (print (parse_json Any t)))",
		"nested":                       "(let (t \"{\\\"k\\\":{\\\"n\\\":[]},\\\"m\\\":[[1,[2]],{}],\\\"e\\\":{}}\") (print (parse_json Any t)))",
		"whitespace":                   "(let (t \" \\t\\n\\r {\\\"a\\\" : [ 1 , 2 ] } \\n\") (print (parse_json Any t)))",
		"scalars":                      "(let (t \"[true,false,null,0,-0,7,-12,\\\"\\\",\\\"\\\"]\") (print (parse_json Any t)))",
		"numbers":                      "(let (t \"[1e2,2E-3,-0.0,0.5,1.25e+3,123456789012,9223372036854775807,-9223372036854775808]\") (print (parse_json Any t)))",
		"big numbers":                  "(let (t \"[9223372036854775808,12345678901234567890.5,1e300]\") (print (parse_json Any t)))",
		"escapes":                      "(let (t \"\\\"q\\\\\\\"\\\\\\\\\\\\/\\\\b\\\\f\\\\n\\\\r\\\\t\\\"\") (print (parse_json Any t)))",
		"unicode escapes":              "(let (t \"[\\\"\\\\u00e9\\\\u00E9\\\",\\\"\\\\u263a\\\",\\\"\\\\u0041\\\"]\") (print (parse_json Any t)))",
		"surrogate pair":               "(let (t \"\\\"\\\\ud83d\\\\ude00\\\"\") (print (parse_json Any t)))",
		"lone surrogates":              "(let (t \"[\\\"\\\\ud800x\\\",\\\"\\\\udc00\\\",\\\"\\\\ud800\\\\u0041\\\"]\") (print (parse_json Any t)))",
		"raw unicode":                  "(let (t \"\\\"héllo ✓ 日本 🎉\\\"\") (print (parse_json Any t)))",
		"top-level string":             "(let (t \"\\\"plain\\\"\") (print (parse_json Any t)))",
		"top-level number":             "(let (t \"  7  \") (print (parse_json Any t)))",
		"empty":                        "(let (t \"\") (print (parse_json Any t)))",
		"only spaces":                  "(let (t \"   \") (print (parse_json Any t)))",
		"open object":                  "(let (t \"{\") (print (parse_json Any t)))",
		"open array":                   "(let (t \"[1,\") (print (parse_json Any t)))",
		"trailing comma array":         "(let (t \"[1,]\") (print (parse_json Any t)))",
		"trailing comma object":        "(let (t \"{\\\"a\\\":1,}\") (print (parse_json Any t)))",
		"missing colon":                "(let (t \"{\\\"a\\\" 1}\") (print (parse_json Any t)))",
		"bad pair end":                 "(let (t \"{\\\"a\\\":1 \\\"b\\\":2}\") (print (parse_json Any t)))",
		"bad array end":                "(let (t \"[1 2]\") (print (parse_json Any t)))",
		"number then letters in array": "(let (t \"[12abc]\") (print (parse_json Any t)))",
		"non-string key":               "(let (t \"{1:2}\") (print (parse_json Any t)))",
		"truncated literal":            "(let (t \"nul\") (print (parse_json Any t)))",
		"wrong literal":                "(let (t \"nulx\") (print (parse_json Any t)))",
		"wrong true":                   "(let (t \"trux\") (print (parse_json Any t)))",
		"wrong false":                  "(let (t \"fax\") (print (parse_json Any t)))",
		"leading zero":                 "(let (t \"01\") (print (parse_json Any t)))",
		"lone minus":                   "(let (t \"-\") (print (parse_json Any t)))",
		"minus letter":                 "(let (t \"-x\") (print (parse_json Any t)))",
		"dot end":                      "(let (t \"1.\") (print (parse_json Any t)))",
		"dot letter":                   "(let (t \"1.x\") (print (parse_json Any t)))",
		"exponent end":                 "(let (t \"1e\") (print (parse_json Any t)))",
		"exponent letter":              "(let (t \"1e+x\") (print (parse_json Any t)))",
		"plus sign":                    "(let (t \"+1\") (print (parse_json Any t)))",
		"open string":                  "(let (t \"\\\"abc\") (print (parse_json Any t)))",
		"bad escape":                   "(let (t \"\\\"\\\\q\\\"\") (print (parse_json Any t)))",
		"bad unicode escape":           "(let (t \"\\\"\\\\u12g4\\\"\") (print (parse_json Any t)))",
		"short unicode escape":         "(let (t \"\\\"\\\\u12\") (print (parse_json Any t)))",
		"control in string":            "(let (t \"\\\"a\\tb\\\"\") (print (parse_json Any t)))",
		"newline in string":            "(let (t \"\\\"a\\nb\\\"\") (print (parse_json Any t)))",
		"two numbers":                  "(let (t \"1 2\") (print (parse_json Any t)))",
		"number then word":             "(let (t \"12x\") (print (parse_json Any t)))",
		"two objects":                  "(let (t \"{}{}\") (print (parse_json Any t)))",
		"number then null":             "(let (t \"1 null\") (print (parse_json Any t)))",
		"two arrays":                   "(let (t \"[1][2]\") (print (parse_json Any t)))",
		"number then bool":             "(let (t \"1true\") (print (parse_json Any t)))",
		"number then string":           "(let (t \"1 \\\"s\\\"\") (print (parse_json Any t)))",
		"object then junk":             "(let (t \"{} x\") (print (parse_json Any t)))",
		"second truncated":             "(let (t \"1 [\") (print (parse_json Any t)))",
		"out of range":                 "(let (t \"1e400\") (print (parse_json Any t)))",
		"out of range nested":          "(let (t \"[1, {\\\"a\\\": 1e400}]\") (print (parse_json Any t)))",
		"range then second value":      "(let (t \"1e400 x\") (print (parse_json Any t)))",
		"word":                         "(let (t \"x\") (print (parse_json Any t)))",
		"non-ascii":                    "(let (t \"é\") (print (parse_json Any t)))",
		"quote char":                   "(let (t \"'\") (print (parse_json Any t)))",
		"control char":                 "(let (t \"\u0001\") (print (parse_json Any t)))",
		"tab then junk":                "(let (t \"\\t)\") (print (parse_json Any t)))",
		// Only an int overflows, which tells ints from floats.
		"number kinds": `(let (t "[1, 1.0, 1e0, 5, -5.0, 9223372036854775808]") (for v (parse_json Any t) (print (try_let (s (+ v 9223372036854775807)) (catch e "int") "float"))))`,
		"round trip":   `(let (v (dict ("a" (list 1 2)) ("f" 2.5) ("s" "x<y>") ("b" (dict ("c" true)))) t (encode_json v)) (print (= (encode_json (parse_json Any t)) t)))`,
		"read fields":  `(let (t "{\"name\":\"Ann\",\"tags\":[\"a\",\"b\"]}") (let (v (parse_json Any t)) (print (map_get v "name") (list_len (map_get v "tags")) (map_has v "x"))))`,
		"caught":       `(let (t "{") (print (try_let (v (parse_json Any t)) (catch e (str_contains e "unexpected EOF")) v)))`,
		"expression":   `(print (parse_json Any (+ "[1," "2]")))`,
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, "(cli_app "+body+")") })
	}
}

// Floats are formatted and parsed by standard-library HowlFrame
// (std/float.howl, a port of Go's strconv decimal); the VM's are Go's own.
// The random texts come from a fixed seed (20261030); each is read in a
// try_let, so a text out of range (-1) does not hide the rest of its line.
func TestDiffFloats(t *testing.T) {
	for name, body := range map[string]string{
		"parse 0":              "(print (try_let (v (to_float \".38560894415462221E+17\")) (catch e -1.0) v) (try_let (v (to_float \"-7.e176\")) (catch e -1.0) v) (try_let (v (to_float \"689220712479329.11731e273\")) (catch e -1.0) v) (try_let (v (to_float \"16.\")) (catch e -1.0) v) (try_let (v (to_float \"943514948438804\")) (catch e -1.0) v) (try_let (v (to_float \"9577250064561.552369213144E+28\")) (catch e -1.0) v) (try_let (v (to_float \"-44412350.85427252\")) (catch e -1.0) v) (try_let (v (to_float \"-31.321112e-12\")) (catch e -1.0) v) (try_let (v (to_float \"3999384000232191448343.079778559326995267\")) (catch e -1.0) v) (try_let (v (to_float \"+.4E+16\")) (catch e -1.0) v))",
		"parse 1":              "(print (try_let (v (to_float \"+5024599772674149139130249e-292\")) (catch e -1.0) v) (try_let (v (to_float \"+759118632045600E+14\")) (catch e -1.0) v) (try_let (v (to_float \"03.338e-110\")) (catch e -1.0) v) (try_let (v (to_float \"-77057761233261255e9\")) (catch e -1.0) v) (try_let (v (to_float \"-0916909372288311e-24\")) (catch e -1.0) v) (try_let (v (to_float \"07.05382000915262882070185e-16\")) (catch e -1.0) v) (try_let (v (to_float \"27252925.E+3\")) (catch e -1.0) v) (try_let (v (to_float \"01643.6098972E+22\")) (catch e -1.0) v) (try_let (v (to_float \"84707054.2134376\")) (catch e -1.0) v) (try_let (v (to_float \"2.786342270118724752255925\")) (catch e -1.0) v))",
		"parse 2":              "(print (try_let (v (to_float \"+429e-1\")) (catch e -1.0) v) (try_let (v (to_float \"+6685508240424732529214.045e-6\")) (catch e -1.0) v) (try_let (v (to_float \"232.\")) (catch e -1.0) v) (try_let (v (to_float \"6653929848322869949311981\")) (catch e -1.0) v) (try_let (v (to_float \"+83.9496509307859e27\")) (catch e -1.0) v) (try_let (v (to_float \"7e-234\")) (catch e -1.0) v) (try_let (v (to_float \"5327619832538802656332403e-29\")) (catch e -1.0) v) (try_let (v (to_float \"896953685396375462.e-328\")) (catch e -1.0) v) (try_let (v (to_float \"99906856.646572620e-5\")) (catch e -1.0) v) (try_let (v (to_float \"-038.87e295\")) (catch e -1.0) v))",
		"parse 3":              "(print (try_let (v (to_float \"-8152726266718325.1E+5\")) (catch e -1.0) v) (try_let (v (to_float \"+70.8e-27\")) (catch e -1.0) v) (try_let (v (to_float \"-82024716e212\")) (catch e -1.0) v) (try_let (v (to_float \"-3658794332733239609862933664937955.105198e-104\")) (catch e -1.0) v) (try_let (v (to_float \"10397225534221189734e-114\")) (catch e -1.0) v) (try_let (v (to_float \"7.9645701\")) (catch e -1.0) v) (try_let (v (to_float \"2020322540218288541276255195074593552905e-27\")) (catch e -1.0) v) (try_let (v (to_float \"-2592876178332196576035765796841378511465e192\")) (catch e -1.0) v) (try_let (v (to_float \"-807037.4094749008473650963e82\")) (catch e -1.0) v) (try_let (v (to_float \"85104.70633383251898882971e-8\")) (catch e -1.0) v))",
		"parse 4":              "(print (try_let (v (to_float \"04088763542426093\")) (catch e -1.0) v) (try_let (v (to_float \"02376e-4\")) (catch e -1.0) v) (try_let (v (to_float \"+28711533442359226\")) (catch e -1.0) v) (try_let (v (to_float \"90.71257581443118E+11\")) (catch e -1.0) v) (try_let (v (to_float \"+692984423366965494E+19\")) (catch e -1.0) v) (try_let (v (to_float \"+910622587.372261212e139\")) (catch e -1.0) v) (try_let (v (to_float \"+5093132409113733\")) (catch e -1.0) v) (try_let (v (to_float \"36.e-23\")) (catch e -1.0) v) (try_let (v (to_float \"+831845696484154.8901686537e220\")) (catch e -1.0) v) (try_let (v (to_float \"7615.9626059041233993E+29\")) (catch e -1.0) v))",
		"parse 5":              "(print (try_let (v (to_float \"46651286425911.6\")) (catch e -1.0) v) (try_let (v (to_float \"18718E+10\")) (catch e -1.0) v) (try_let (v (to_float \"+844008261.1615948248585850182404859531342E+27\")) (catch e -1.0) v) (try_let (v (to_float \"1020309048075594688345465e32\")) (catch e -1.0) v) (try_let (v (to_float \"+601988268.03674308171e-250\")) (catch e -1.0) v) (try_let (v (to_float \"-39442324.0813311e137\")) (catch e -1.0) v) (try_let (v (to_float \"+52.4e-23\")) (catch e -1.0) v) (try_let (v (to_float \"-1445557072732.17e-172\")) (catch e -1.0) v) (try_let (v (to_float \"+85619006284.1730349\")) (catch e -1.0) v) (try_let (v (to_float \"18718718.e208\")) (catch e -1.0) v))",
		"parse 6":              "(print (try_let (v (to_float \"2076173684540877E+23\")) (catch e -1.0) v) (try_let (v (to_float \"-0.2e-11\")) (catch e -1.0) v) (try_let (v (to_float \"+742044999405E+29\")) (catch e -1.0) v) (try_let (v (to_float \".3e-121\")) (catch e -1.0) v) (try_let (v (to_float \"-611348666435252447.2802052292351006548624e-23\")) (catch e -1.0) v) (try_let (v (to_float \"980\")) (catch e -1.0) v) (try_let (v (to_float \"34316036388348.3732E+26\")) (catch e -1.0) v) (try_let (v (to_float \"22.211E+28\")) (catch e -1.0) v) (try_let (v (to_float \"+9097241359168495507621937157690594936.608e-19\")) (catch e -1.0) v) (try_let (v (to_float \"3.e-24\")) (catch e -1.0) v))",
		"parse 7":              "(print (try_let (v (to_float \"-18.9E+3\")) (catch e -1.0) v) (try_let (v (to_float \".03183808e-21\")) (catch e -1.0) v) (try_let (v (to_float \"86.990197504677065231e-223\")) (catch e -1.0) v) (try_let (v (to_float \"-1497670810172512679593223772981829591.253e-24\")) (catch e -1.0) v) (try_let (v (to_float \"715635949192499946e-29\")) (catch e -1.0) v) (try_let (v (to_float \"+0.8e-7\")) (catch e -1.0) v) (try_let (v (to_float \"62195.e-7\")) (catch e -1.0) v) (try_let (v (to_float \"+80633876595206757772656767065.12809526660e-314\")) (catch e -1.0) v) (try_let (v (to_float \"94160926308489589426e34\")) (catch e -1.0) v) (try_let (v (to_float \"-5756820527479384E+13\")) (catch e -1.0) v))",
		"parse 8":              "(print (try_let (v (to_float \"013567.442359054e-4\")) (catch e -1.0) v) (try_let (v (to_float \"-89620137775741764\")) (catch e -1.0) v) (try_let (v (to_float \"09571634283086.9e161\")) (catch e -1.0) v) (try_let (v (to_float \"-37560E+29\")) (catch e -1.0) v) (try_let (v (to_float \"26972453545408790082\")) (catch e -1.0) v) (try_let (v (to_float \"+5400221788525864e-47\")) (catch e -1.0) v) (try_let (v (to_float \"49205.058489281936537e269\")) (catch e -1.0) v) (try_let (v (to_float \"+12.E+24\")) (catch e -1.0) v) (try_let (v (to_float \"-.553e-8\")) (catch e -1.0) v) (try_let (v (to_float \"70.1e-0\")) (catch e -1.0) v))",
		"parse 9":              "(print (try_let (v (to_float \"3942.97790206094\")) (catch e -1.0) v) (try_let (v (to_float \"+31274122537.997885e-13\")) (catch e -1.0) v) (try_let (v (to_float \"9764596660282195.15e-115\")) (catch e -1.0) v) (try_let (v (to_float \"03.2779901964641e-16\")) (catch e -1.0) v) (try_let (v (to_float \"-7974591409557.060072162892\")) (catch e -1.0) v) (try_let (v (to_float \"+061.54965\")) (catch e -1.0) v) (try_let (v (to_float \"96710.67641222565E+18\")) (catch e -1.0) v) (try_let (v (to_float \"3225665024333905137997937e-19\")) (catch e -1.0) v) (try_let (v (to_float \"+1.5\")) (catch e -1.0) v) (try_let (v (to_float \"931.42105913764648E+3\")) (catch e -1.0) v))",
		"parse 10":             "(print (try_let (v (to_float \"917e-213\")) (catch e -1.0) v) (try_let (v (to_float \"303e-12\")) (catch e -1.0) v) (try_let (v (to_float \"6.7e280\")) (catch e -1.0) v) (try_let (v (to_float \"85404266213199.739e-6\")) (catch e -1.0) v) (try_let (v (to_float \".5E+1\")) (catch e -1.0) v) (try_let (v (to_float \"+2853272608800.19558e-2\")) (catch e -1.0) v) (try_let (v (to_float \"69.42824859110276E+25\")) (catch e -1.0) v) (try_let (v (to_float \"+78834\")) (catch e -1.0) v) (try_let (v (to_float \"-8.\")) (catch e -1.0) v) (try_let (v (to_float \"-53484655315932973601\")) (catch e -1.0) v))",
		"parse 11":             "(print (try_let (v (to_float \"+36.65805680836577360596913\")) (catch e -1.0) v) (try_let (v (to_float \"+41153e-0\")) (catch e -1.0) v) (try_let (v (to_float \"+73.361966e-16\")) (catch e -1.0) v) (try_let (v (to_float \"47863981168343.9E+16\")) (catch e -1.0) v) (try_let (v (to_float \"5803819.7278904998571E+16\")) (catch e -1.0) v) (try_let (v (to_float \"9346886596.03652822e65\")) (catch e -1.0) v) (try_let (v (to_float \"82.0e-3\")) (catch e -1.0) v) (try_let (v (to_float \"+70603355655539598\")) (catch e -1.0) v) (try_let (v (to_float \"5760127660555028.7349E+6\")) (catch e -1.0) v) (try_let (v (to_float \"4.622902769681474e-7\")) (catch e -1.0) v))",
		"parse 12":             "(print (try_let (v (to_float \"-717174836587091E+28\")) (catch e -1.0) v) (try_let (v (to_float \"0.e-288\")) (catch e -1.0) v) (try_let (v (to_float \"+59835671511956040225E+19\")) (catch e -1.0) v) (try_let (v (to_float \"-1.9E+20\")) (catch e -1.0) v) (try_let (v (to_float \"+62504256556105.56e-15\")) (catch e -1.0) v) (try_let (v (to_float \"-485861431576.104637e-27\")) (catch e -1.0) v) (try_let (v (to_float \"255142555688965.25754e-11\")) (catch e -1.0) v) (try_let (v (to_float \"+4.E+28\")) (catch e -1.0) v) (try_let (v (to_float \"42204364.5302695\")) (catch e -1.0) v) (try_let (v (to_float \"+813.e-268\")) (catch e -1.0) v))",
		"parse 13":             "(print (try_let (v (to_float \"72622e-288\")) (catch e -1.0) v) (try_let (v (to_float \"271705902736698E+19\")) (catch e -1.0) v) (try_let (v (to_float \"0178364408972804E+12\")) (catch e -1.0) v) (try_let (v (to_float \"9418335370467243438742780\")) (catch e -1.0) v) (try_let (v (to_float \"99800352\")) (catch e -1.0) v) (try_let (v (to_float \"99842606284185629315E+19\")) (catch e -1.0) v) (try_let (v (to_float \"+222584781422227e-19\")) (catch e -1.0) v) (try_let (v (to_float \"+8.5576e-27\")) (catch e -1.0) v) (try_let (v (to_float \"-1.E+14\")) (catch e -1.0) v) (try_let (v (to_float \"+557401699.076054650\")) (catch e -1.0) v))",
		"parse 14":             "(print (try_let (v (to_float \"-85524182845.371485401e-21\")) (catch e -1.0) v) (try_let (v (to_float \"13997970.e-20\")) (catch e -1.0) v) (try_let (v (to_float \".055014668078003E+28\")) (catch e -1.0) v) (try_let (v (to_float \"+0916063113189101644562676\")) (catch e -1.0) v) (try_let (v (to_float \"-263750.209677E+6\")) (catch e -1.0) v) (try_let (v (to_float \"+68512005.e-203\")) (catch e -1.0) v) (try_let (v (to_float \"97.626075714044399215E+10\")) (catch e -1.0) v) (try_let (v (to_float \"+99902.3722324e-7\")) (catch e -1.0) v) (try_let (v (to_float \"-153077.10\")) (catch e -1.0) v) (try_let (v (to_float \"307.3060533237829719891755e-14\")) (catch e -1.0) v))",
		"parse 15":             "(print (try_let (v (to_float \"6.54168646390745\")) (catch e -1.0) v) (try_let (v (to_float \"39.E+28\")) (catch e -1.0) v) (try_let (v (to_float \"-98986571\")) (catch e -1.0) v) (try_let (v (to_float \"+185.81\")) (catch e -1.0) v) (try_let (v (to_float \"+3.5e-21\")) (catch e -1.0) v) (try_let (v (to_float \"-0932350728806968780823427532184118946916E+4\")) (catch e -1.0) v) (try_let (v (to_float \"-5556.5e-192\")) (catch e -1.0) v) (try_let (v (to_float \"+34.50361949181493653287516e-0\")) (catch e -1.0) v) (try_let (v (to_float \"+.830629240958493016E+19\")) (catch e -1.0) v) (try_let (v (to_float \"-475079195347\")) (catch e -1.0) v))",
		"parse 16":             "(print (try_let (v (to_float \"-002394077589.937e-85\")) (catch e -1.0) v) (try_let (v (to_float \"-6804953660501601784500474341473660284674\")) (catch e -1.0) v) (try_let (v (to_float \"+.2e-9\")) (catch e -1.0) v) (try_let (v (to_float \"-5147787391046700098386927725049942480020E+19\")) (catch e -1.0) v) (try_let (v (to_float \"4.e-11\")) (catch e -1.0) v) (try_let (v (to_float \"+81166274e-201\")) (catch e -1.0) v) (try_let (v (to_float \"0265542619045647413823703.e-0\")) (catch e -1.0) v) (try_let (v (to_float \"+1768386273.70182e-0\")) (catch e -1.0) v) (try_let (v (to_float \"-9410109375765.03E+29\")) (catch e -1.0) v) (try_let (v (to_float \"-71445468055181994621722.84971013244658520e-139\")) (catch e -1.0) v))",
		"parse 17":             "(print (try_let (v (to_float \"+027464971468176215083223276229922439812.3\")) (catch e -1.0) v) (try_let (v (to_float \"0.2\")) (catch e -1.0) v) (try_let (v (to_float \"-0979396862.939704358147678505863303514384e-14\")) (catch e -1.0) v) (try_let (v (to_float \"+93.9902753425e-24\")) (catch e -1.0) v) (try_let (v (to_float \"+834E+21\")) (catch e -1.0) v) (try_let (v (to_float \"041720329918621236e126\")) (catch e -1.0) v) (try_let (v (to_float \"-3794.1296255998424\")) (catch e -1.0) v) (try_let (v (to_float \"8888344.20934767e-235\")) (catch e -1.0) v) (try_let (v (to_float \"19953139350155968E+22\")) (catch e -1.0) v) (try_let (v (to_float \"35.639\")) (catch e -1.0) v))",
		"parse 18":             "(print (try_let (v (to_float \"-05913555525062806\")) (catch e -1.0) v) (try_let (v (to_float \"7156624.52723474085E+11\")) (catch e -1.0) v) (try_let (v (to_float \"79.6199345332e-26\")) (catch e -1.0) v) (try_let (v (to_float \"+807540719803\")) (catch e -1.0) v) (try_let (v (to_float \"657506207302.166e123\")) (catch e -1.0) v) (try_let (v (to_float \"-361.e-3\")) (catch e -1.0) v) (try_let (v (to_float \"663659.806804e-13\")) (catch e -1.0) v) (try_let (v (to_float \"-0729690722234411549435690579457.234064127E+27\")) (catch e -1.0) v) (try_let (v (to_float \"58323668281553357.E+9\")) (catch e -1.0) v) (try_let (v (to_float \"0.4E+1\")) (catch e -1.0) v))",
		"parse 19":             "(print (try_let (v (to_float \"44657280437.3103171E+24\")) (catch e -1.0) v) (try_let (v (to_float \"75e-301\")) (catch e -1.0) v) (try_let (v (to_float \"-401468.0625556294593669820e241\")) (catch e -1.0) v) (try_let (v (to_float \"241564805554619799\")) (catch e -1.0) v) (try_let (v (to_float \"-1790361352332008.E+1\")) (catch e -1.0) v) (try_let (v (to_float \"178.20945955574418999E+3\")) (catch e -1.0) v) (try_let (v (to_float \"5e-301\")) (catch e -1.0) v) (try_let (v (to_float \"+8185216.737751436e-2\")) (catch e -1.0) v) (try_let (v (to_float \"+889287260573\")) (catch e -1.0) v) (try_let (v (to_float \"-9152.6951272070200E+29\")) (catch e -1.0) v))",
		"parse 20":             "(print (try_let (v (to_float \"13175562\")) (catch e -1.0) v) (try_let (v (to_float \"+3E+24\")) (catch e -1.0) v) (try_let (v (to_float \"+6074.3023e-11\")) (catch e -1.0) v) (try_let (v (to_float \"+005650639407721517531128129451873216142.0E+16\")) (catch e -1.0) v) (try_let (v (to_float \"+9927730717776023727885622006584.748121409\")) (catch e -1.0) v) (try_let (v (to_float \"-62968838896450962433161.20e-13\")) (catch e -1.0) v) (try_let (v (to_float \"-4736044759659867177606699e-22\")) (catch e -1.0) v) (try_let (v (to_float \"+0087731164468606217210146E+17\")) (catch e -1.0) v) (try_let (v (to_float \"047853147200e-6\")) (catch e -1.0) v) (try_let (v (to_float \"-9048221961063891635550711182773185466333e-238\")) (catch e -1.0) v))",
		"parse 21":             "(print (try_let (v (to_float \"9541615271757.2401467\")) (catch e -1.0) v) (try_let (v (to_float \"+28.593113976940820948e-208\")) (catch e -1.0) v) (try_let (v (to_float \"5478.18869544e-224\")) (catch e -1.0) v) (try_let (v (to_float \"-262605.658039915\")) (catch e -1.0) v) (try_let (v (to_float \"622122349057492.21\")) (catch e -1.0) v) (try_let (v (to_float \"18203079e115\")) (catch e -1.0) v) (try_let (v (to_float \"-056234548.0998456361064599081308613437471e-42\")) (catch e -1.0) v) (try_let (v (to_float \"-23855498745968.0e-284\")) (catch e -1.0) v) (try_let (v (to_float \"+.0E+3\")) (catch e -1.0) v) (try_let (v (to_float \"+368790814568.e-4\")) (catch e -1.0) v))",
		"parse 22":             "(print (try_let (v (to_float \"71.5e-315\")) (catch e -1.0) v) (try_let (v (to_float \"+8832495127082043.4498\")) (catch e -1.0) v) (try_let (v (to_float \"+474072.2563530643e-8\")) (catch e -1.0) v) (try_let (v (to_float \"+282\")) (catch e -1.0) v) (try_let (v (to_float \"+5363770.06169247\")) (catch e -1.0) v) (try_let (v (to_float \"+956043392703407922329.3187E+22\")) (catch e -1.0) v) (try_let (v (to_float \"+0.7e-9\")) (catch e -1.0) v) (try_let (v (to_float \"78823989821595.651429\")) (catch e -1.0) v) (try_let (v (to_float \"+8734238059266747\")) (catch e -1.0) v) (try_let (v (to_float \"+18358640363.2228e-11\")) (catch e -1.0) v))",
		"parse 23":             "(print (try_let (v (to_float \"576916491825702.84\")) (catch e -1.0) v) (try_let (v (to_float \"+218315346927e-307\")) (catch e -1.0) v) (try_let (v (to_float \"+.1e-244\")) (catch e -1.0) v) (try_let (v (to_float \"+79.9912081044622659E+19\")) (catch e -1.0) v) (try_let (v (to_float \"-1469236894590023058608212216612368532582E+19\")) (catch e -1.0) v) (try_let (v (to_float \"+0.90739104447943182e-13\")) (catch e -1.0) v) (try_let (v (to_float \"+56628689602.56309107719777\")) (catch e -1.0) v) (try_let (v (to_float \"82.15875165324495\")) (catch e -1.0) v) (try_let (v (to_float \"-2540897025465526217231603834109657102425E+26\")) (catch e -1.0) v) (try_let (v (to_float \"+8278531489579291640915.330587490263938718\")) (catch e -1.0) v))",
		"parse 24":             "(print (try_let (v (to_float \"48716723899281.549403e-14\")) (catch e -1.0) v) (try_let (v (to_float \"40225736357284842e-13\")) (catch e -1.0) v) (try_let (v (to_float \"-0\")) (catch e -1.0) v) (try_let (v (to_float \"+0.700E+21\")) (catch e -1.0) v) (try_let (v (to_float \"23.\")) (catch e -1.0) v) (try_let (v (to_float \"44e-26\")) (catch e -1.0) v) (try_let (v (to_float \"949E+0\")) (catch e -1.0) v) (try_let (v (to_float \"+748e-20\")) (catch e -1.0) v) (try_let (v (to_float \"35479577.3454e-12\")) (catch e -1.0) v) (try_let (v (to_float \"-699095397.277961801e-72\")) (catch e -1.0) v))",
		"parse 25":             "(print (try_let (v (to_float \"3659079331.79077777e-27\")) (catch e -1.0) v) (try_let (v (to_float \"-3418939473117893E+7\")) (catch e -1.0) v) (try_let (v (to_float \"120\")) (catch e -1.0) v) (try_let (v (to_float \"2.77e158\")) (catch e -1.0) v) (try_let (v (to_float \".22e33\")) (catch e -1.0) v) (try_let (v (to_float \"70680927e140\")) (catch e -1.0) v) (try_let (v (to_float \"9167063609505.805e268\")) (catch e -1.0) v) (try_let (v (to_float \"-6978406.646060685527871966499763103532117e146\")) (catch e -1.0) v) (try_let (v (to_float \"618950984.5713703e-30\")) (catch e -1.0) v) (try_let (v (to_float \"019992.1110507546456410616469576984112436e-198\")) (catch e -1.0) v))",
		"parse 26":             "(print (try_let (v (to_float \"24E+4\")) (catch e -1.0) v) (try_let (v (to_float \"+23.054182838733858\")) (catch e -1.0) v) (try_let (v (to_float \"-.58e-5\")) (catch e -1.0) v) (try_let (v (to_float \"+59784e35\")) (catch e -1.0) v) (try_let (v (to_float \"-298742344891954.156e283\")) (catch e -1.0) v) (try_let (v (to_float \"-64730.704769602941972e-14\")) (catch e -1.0) v) (try_let (v (to_float \"061851821011.6240252556374911292120377877\")) (catch e -1.0) v) (try_let (v (to_float \"4338438638588353578379.929492129829731980\")) (catch e -1.0) v) (try_let (v (to_float \"81926.54460870599019636374e-9\")) (catch e -1.0) v) (try_let (v (to_float \"-49259638616.418731357E+14\")) (catch e -1.0) v))",
		"parse 27":             "(print (try_let (v (to_float \"-7629344776076983\")) (catch e -1.0) v) (try_let (v (to_float \"+2.43446853236285e-22\")) (catch e -1.0) v) (try_let (v (to_float \"2093.52508908492954e161\")) (catch e -1.0) v) (try_let (v (to_float \"911E+1\")) (catch e -1.0) v) (try_let (v (to_float \"+169.810874356137979e-15\")) (catch e -1.0) v) (try_let (v (to_float \"+577499707700.35304\")) (catch e -1.0) v) (try_let (v (to_float \"191435841228004\")) (catch e -1.0) v) (try_let (v (to_float \"+876706318693239e-114\")) (catch e -1.0) v) (try_let (v (to_float \"+495.31E+6\")) (catch e -1.0) v) (try_let (v (to_float \"0214814.360120972e-28\")) (catch e -1.0) v))",
		"parse 28":             "(print (try_let (v (to_float \"+52360228E+20\")) (catch e -1.0) v) (try_let (v (to_float \"7265246480745125.77e-2\")) (catch e -1.0) v) (try_let (v (to_float \"+4129924597247.797739121643e229\")) (catch e -1.0) v) (try_let (v (to_float \"+462736896976525e-316\")) (catch e -1.0) v) (try_let (v (to_float \"0.\")) (catch e -1.0) v) (try_let (v (to_float \"110724930926E+28\")) (catch e -1.0) v) (try_let (v (to_float \"+7410859699691782176078346E+1\")) (catch e -1.0) v) (try_let (v (to_float \"+550157756.95001103848\")) (catch e -1.0) v) (try_let (v (to_float \"-58786\")) (catch e -1.0) v) (try_let (v (to_float \"82.0996852552e-23\")) (catch e -1.0) v))",
		"parse 29":             "(print (try_let (v (to_float \"-17256681702651823231.45517E+20\")) (catch e -1.0) v) (try_let (v (to_float \"925.99708105219587\")) (catch e -1.0) v) (try_let (v (to_float \"-66601e-76\")) (catch e -1.0) v) (try_let (v (to_float \"5182266.5\")) (catch e -1.0) v) (try_let (v (to_float \"+88063903289433.53E+27\")) (catch e -1.0) v) (try_let (v (to_float \"4.5329E+29\")) (catch e -1.0) v) (try_let (v (to_float \"990E+19\")) (catch e -1.0) v) (try_let (v (to_float \"090259179610832023228895762836288711925.1e-19\")) (catch e -1.0) v) (try_let (v (to_float \"+740593218286\")) (catch e -1.0) v) (try_let (v (to_float \"1480297473773809.5E+28\")) (catch e -1.0) v))",
		"parse 30":             "(print (try_let (v (to_float \"9007199254740993\")) (catch e -1.0) v) (try_let (v (to_float \"9007199254740992.5\")) (catch e -1.0) v) (try_let (v (to_float \"9007199254740991\")) (catch e -1.0) v) (try_let (v (to_float \"0.1\")) (catch e -1.0) v) (try_let (v (to_float \"0.2\")) (catch e -1.0) v) (try_let (v (to_float \"0.3\")) (catch e -1.0) v) (try_let (v (to_float \"1.7976931348623157e308\")) (catch e -1.0) v) (try_let (v (to_float \"1.7976931348623158e308\")) (catch e -1.0) v) (try_let (v (to_float \"1.7976931348623159e308\")) (catch e -1.0) v) (try_let (v (to_float \"4.9e-324\")) (catch e -1.0) v))",
		"parse 31":             "(print (try_let (v (to_float \"5e-324\")) (catch e -1.0) v) (try_let (v (to_float \"2.4703282292062327e-324\")) (catch e -1.0) v) (try_let (v (to_float \"2.4703282292062328e-324\")) (catch e -1.0) v) (try_let (v (to_float \"2.2250738585072011e-308\")) (catch e -1.0) v) (try_let (v (to_float \"2.2250738585072014e-308\")) (catch e -1.0) v) (try_let (v (to_float \"2.225073858507201e-308\")) (catch e -1.0) v) (try_let (v (to_float \"1e23\")) (catch e -1.0) v) (try_let (v (to_float \"8.589973e9\")) (catch e -1.0) v) (try_let (v (to_float \"123456789012345678901234567890e-10\")) (catch e -1.0) v) (try_let (v (to_float \"0.0000000000000000000000000000001\")) (catch e -1.0) v))",
		"parse 32":             "(print (try_let (v (to_float \"10000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000e-400\")) (catch e -1.0) v) (try_let (v (to_float \"3.141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535141592653514159265351415926535\")) (catch e -1.0) v) (try_let (v (to_float \"0.000000000000000000000000000000000000000000001e-280\")) (catch e -1.0) v) (try_let (v (to_float \"1_000.000_1\")) (catch e -1.0) v) (try_let (v (to_float \"-0\")) (catch e -1.0) v) (try_let (v (to_float \"-0.0\")) (catch e -1.0) v) (try_let (v (to_float \"0e0\")) (catch e -1.0) v) (try_let (v (to_float \"00012.50\")) (catch e -1.0) v) (try_let (v (to_float \"1e-400\")) (catch e -1.0) v) (try_let (v (to_float \"1e310\")) (catch e -1.0) v))",
		"parse 33":             "(print (try_let (v (to_float \"7.0e-10\")) (catch e -1.0) v) (try_let (v (to_float \"4.35e-15\")) (catch e -1.0) v))",
		"powers of ten":        `(let (x 1.0 i 0) (do (while (< i 308) (do (set x (* x 10.0)) (set i (+ i 1)) (if (or (< i 30) (> i 295)) (print x)))) (print (/ 1.0 x))))`,
		"tenths":               `(let (x 0.0 i 0) (while (< i 40) (do (set x (+ x 0.1)) (set i (+ i 1)) (print x))))`,
		"halvings":             `(let (x 1.0 i 0) (while (< i 1074) (do (set x (/ x 2.0)) (set i (+ i 1)) (if (or (< i 20) (> i 1015)) (print x)))))`,
		"ratios":               `(let (i 1) (while (< i 60) (do (print (/ 1.0 i) (/ 2.0 (+ i 1.5)) (* i 1.1) (/ i 7)) (set i (+ i 1)))))`,
		"large ints as floats": `(print (to_float 9007199254740993) (to_float 9223372036854775807) (* 1.0 123456789012) (to_float -42))`,
		"text and json":        `(print (to_string 0.1) (encode_json (list 0.1 1000000000000000000000.0 0.000001)) (+ "" (to_string (/ 1.0 3.0))))`,
		"hex float":            `(print (to_float "0x1p-2"))`,
		"hex not a float":      `(print (to_float "0x10"))`,
		"bad float text":       `(print (to_float "1__0"))`,
		"bad exponent":         `(print (to_float "1e+"))`,
		"infinity text":        `(print (to_float "inf"))`,
		"overflow text":        `(print (to_float "2e308"))`,
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, "(cli_app "+body+")") })
	}
}

// build --host takes node (the default: app.wasm and the app.mjs loader)
// or wasi (a WASI command, app.wasm only); anything else is refused.
func TestBuildHosts(t *testing.T) {
	src := writeSource(t, `(cli_app (print "hi"))`)
	for host, files := range map[string][]string{"node": {"app.wasm", "app.mjs"}, "wasi": {"app.wasm"}} {
		dir := t.TempDir()
		if out := run(t, compilerBin, "build", src, "--target", "wasmgc", "--host", host, "-o", dir); out.failed {
			t.Fatalf("--host %s: %s", host, out.errMsg)
		}
		entries, _ := os.ReadDir(dir)
		var got []string
		for _, e := range entries {
			got = append(got, e.Name())
		}
		if strings.Join(got, ",") != strings.Join(sortedCopy(files), ",") {
			t.Fatalf("--host %s wrote %v, want %v", host, got, files)
		}
	}
	if out := run(t, compilerBin, "build", src, "--target", "wasmgc", "--host", "browser", "-o", t.TempDir()); !out.failed || !strings.Contains(out.errMsg, "Unknown --host") {
		t.Fatalf("--host browser: failed=%v %s", out.failed, out.errMsg)
	}
}

func sortedCopy(xs []string) []string {
	out := append([]string(nil), xs...)
	sort.Strings(out)
	return out
}

// Hexadecimal floats (Go's atofHex): hard cases, then texts from a fixed
// seed (20261031), each read in a try_let.
func TestDiffHexFloats(t *testing.T) {
	for name, body := range map[string]string{
		"hex 0":  "(print (try_let (v (to_float \"0x1p-2\")) (catch e -1.0) v) (try_let (v (to_float \"0x10p0\")) (catch e -1.0) v) (try_let (v (to_float \"0x1.8p1\")) (catch e -1.0) v) (try_let (v (to_float \"-0x1p0\")) (catch e -1.0) v) (try_let (v (to_float \"+0x.8p0\")) (catch e -1.0) v) (try_let (v (to_float \"0x1_0p0\")) (catch e -1.0) v) (try_let (v (to_float \"0x_1p0\")) (catch e -1.0) v) (try_let (v (to_float \"0x1__0p0\")) (catch e -1.0) v) (try_let (v (to_float \"0x1p\")) (catch e -1.0) v) (try_let (v (to_float \"0x1p+\")) (catch e -1.0) v) (try_let (v (to_float \"0xp1\")) (catch e -1.0) v) (try_let (v (to_float \"0x1\")) (catch e -1.0) v))",
		"hex 1":  "(print (try_let (v (to_float \"0x1.fffffffffffffp1023\")) (catch e -1.0) v) (try_let (v (to_float \"0x1.fffffffffffff8p1023\")) (catch e -1.0) v) (try_let (v (to_float \"0x1p1024\")) (catch e -1.0) v) (try_let (v (to_float \"0x1p-1074\")) (catch e -1.0) v) (try_let (v (to_float \"0x1p-1075\")) (catch e -1.0) v) (try_let (v (to_float \"0x1.8p-1074\")) (catch e -1.0) v) (try_let (v (to_float \"0x3p-1075\")) (catch e -1.0) v) (try_let (v (to_float \"0x123456789abcdef12p0\")) (catch e -1.0) v) (try_let (v (to_float \"0x123456789abcdef1Fp-4\")) (catch e -1.0) v) (try_let (v (to_float \"0XAp0\")) (catch e -1.0) v) (try_let (v (to_float \"0x1P-3\")) (catch e -1.0) v) (try_let (v (to_float \"0x1p1_0\")) (catch e -1.0) v))",
		"hex 2":  "(print (try_let (v (to_float \"0x1p_10\")) (catch e -1.0) v) (try_let (v (to_float \"0x0p0\")) (catch e -1.0) v) (try_let (v (to_float \"-0x0p0\")) (catch e -1.0) v) (try_let (v (to_float \"0x0.0000000000000000000001p0\")) (catch e -1.0) v) (try_let (v (to_float \"0x1.0000000000000000000001p0\")) (catch e -1.0) v) (try_let (v (to_float \"0x1.00000000000008p0\")) (catch e -1.0) v) (try_let (v (to_float \"0x1.00000000000018p0\")) (catch e -1.0) v) (try_let (v (to_float \"0x1.000000000000080000001p0\")) (catch e -1.0) v) (try_let (v (to_float \"+0x7.c2A1p-195\")) (catch e -1.0) v) (try_let (v (to_float \"+0x727Cfp-42\")) (catch e -1.0) v) (try_let (v (to_float \"-0X1.p-34\")) (catch e -1.0) v) (try_let (v (to_float \"0xbEcb3BE851d29P+58\")) (catch e -1.0) v))",
		"hex 3":  "(print (try_let (v (to_float \"+0x625A7207EbcAB72aP+28\")) (catch e -1.0) v) (try_let (v (to_float \"-0Xd4181130F296b7AbF969P+38\")) (catch e -1.0) v) (try_let (v (to_float \"0x1109BfBC824C7BD9p0\")) (catch e -1.0) v) (try_let (v (to_float \"0XCD8D6c52DEaf.A829da7bp-942\")) (catch e -1.0) v) (try_let (v (to_float \"+0x38.p-32\")) (catch e -1.0) v) (try_let (v (to_float \"+0X2441Bp-281\")) (catch e -1.0) v) (try_let (v (to_float \"+0X089F7B7.6AC8b9fdBp0\")) (catch e -1.0) v) (try_let (v (to_float \"0x49fD67a9eE83e.C97p-9\")) (catch e -1.0) v) (try_let (v (to_float \"-0Xec.p-42\")) (catch e -1.0) v) (try_let (v (to_float \"+0x.bDb3b5d0ED9f01A1Dp0\")) (catch e -1.0) v) (try_let (v (to_float \"0XB8D68FDb019F51fa.p217\")) (catch e -1.0) v) (try_let (v (to_float \"0XC.p0\")) (catch e -1.0) v))",
		"hex 4":  "(print (try_let (v (to_float \"+0X0DE97DFFc78.Fdb5Dp-698\")) (catch e -1.0) v) (try_let (v (to_float \"-0xAE1B9p0\")) (catch e -1.0) v) (try_let (v (to_float \"0x53dc00f82bFEbP+1\")) (catch e -1.0) v) (try_let (v (to_float \"-0X.CFP+48\")) (catch e -1.0) v) (try_let (v (to_float \"0xC813CCF1B51AD0.9dCEb8P+38\")) (catch e -1.0) v) (try_let (v (to_float \"-0Xe4AfFp-924\")) (catch e -1.0) v) (try_let (v (to_float \"0XE83473946C1.BFe0AeE65p-438\")) (catch e -1.0) v) (try_let (v (to_float \"-0XEep0\")) (catch e -1.0) v) (try_let (v (to_float \"-0Xd3CA1p-19\")) (catch e -1.0) v) (try_let (v (to_float \"+0xe946Fdf7E83Dc7CB7E8.Dp-628\")) (catch e -1.0) v) (try_let (v (to_float \"+0X334EEp-57\")) (catch e -1.0) v) (try_let (v (to_float \"-0X.Ap-30\")) (catch e -1.0) v))",
		"hex 5":  "(print (try_let (v (to_float \"0X2fEE8F9f6a7B6582EEDEp1083\")) (catch e -1.0) v) (try_let (v (to_float \"-0X.Ap691\")) (catch e -1.0) v) (try_let (v (to_float \"+0X2Cbf18Dae9fa7B2ED9FdP+41\")) (catch e -1.0) v) (try_let (v (to_float \"-0XDfCD6P+57\")) (catch e -1.0) v) (try_let (v (to_float \"+0X.2eP+1\")) (catch e -1.0) v) (try_let (v (to_float \"-0xCBp-243\")) (catch e -1.0) v) (try_let (v (to_float \"-0x9.ep-21\")) (catch e -1.0) v) (try_let (v (to_float \"+0Xba4.cBp-31\")) (catch e -1.0) v) (try_let (v (to_float \"-0X.e613Bp-390\")) (catch e -1.0) v) (try_let (v (to_float \"0XcC839daCdFfdfEe9.p-13\")) (catch e -1.0) v) (try_let (v (to_float \"-0X6fFfDFA6AfEd6.76bP+22\")) (catch e -1.0) v) (try_let (v (to_float \"-0X6CCeFEA.4d1DDA124p0\")) (catch e -1.0) v))",
		"hex 6":  "(print (try_let (v (to_float \"+0XE.9p-54\")) (catch e -1.0) v) (try_let (v (to_float \"0x0F7.D8p-30\")) (catch e -1.0) v) (try_let (v (to_float \"0X6e85DcfAf2DCEC6.0p-54\")) (catch e -1.0) v) (try_let (v (to_float \"0X6p-45\")) (catch e -1.0) v) (try_let (v (to_float \"+0X3a4.Edp-53\")) (catch e -1.0) v) (try_let (v (to_float \"+0xe2P+25\")) (catch e -1.0) v) (try_let (v (to_float \"-0x85a7b.1d6AF6EE68bp-6\")) (catch e -1.0) v) (try_let (v (to_float \"+0x179419fD04C81p630\")) (catch e -1.0) v) (try_let (v (to_float \"-0XdD8A3C2AFaCb.CC4FBfb9p-23\")) (catch e -1.0) v) (try_let (v (to_float \"0X0A3E24EcFbA5f6c.bEp0\")) (catch e -1.0) v) (try_let (v (to_float \"0xF5FE2B14Afdda13E3.p0\")) (catch e -1.0) v) (try_let (v (to_float \"-0x0EF5c.773FAB2fp-17\")) (catch e -1.0) v))",
		"hex 7":  "(print (try_let (v (to_float \"-0X.F9p-56\")) (catch e -1.0) v) (try_let (v (to_float \"+0XB.p-43\")) (catch e -1.0) v) (try_let (v (to_float \"+0X4A4d8dfD2bE5bP+13\")) (catch e -1.0) v) (try_let (v (to_float \"+0X159cEe770BB8.Eae8p-60\")) (catch e -1.0) v) (try_let (v (to_float \"0Xe.Bp-49\")) (catch e -1.0) v) (try_let (v (to_float \"-0X1.p-51\")) (catch e -1.0) v) (try_let (v (to_float \"-0X6ep-17\")) (catch e -1.0) v) (try_let (v (to_float \"+0XABp0\")) (catch e -1.0) v) (try_let (v (to_float \"-0X.Eep0\")) (catch e -1.0) v) (try_let (v (to_float \"-0xe0be.21Dc33C47P+27\")) (catch e -1.0) v) (try_let (v (to_float \"-0X8d49353a0Dbb1283p-59\")) (catch e -1.0) v) (try_let (v (to_float \"+0X523F0p0\")) (catch e -1.0) v))",
		"hex 8":  "(print (try_let (v (to_float \"+0XB0D40p783\")) (catch e -1.0) v) (try_let (v (to_float \"0x06B6.6F52a66eC9dffbffp420\")) (catch e -1.0) v) (try_let (v (to_float \"+0x341BbfBE6A6E1P+45\")) (catch e -1.0) v) (try_let (v (to_float \"0X82E48FE8f4E6cAe197ed.P+36\")) (catch e -1.0) v) (try_let (v (to_float \"+0XB0.p-412\")) (catch e -1.0) v) (try_let (v (to_float \"-0xacDDBa4FC2CA2EB1p-216\")) (catch e -1.0) v) (try_let (v (to_float \"-0XA6P+54\")) (catch e -1.0) v) (try_let (v (to_float \"0XbEC8f2487.D2e1B99p-46\")) (catch e -1.0) v) (try_let (v (to_float \"0xFb.1FdP+43\")) (catch e -1.0) v) (try_let (v (to_float \"0XDF6A7e.C5835d4p0\")) (catch e -1.0) v) (try_let (v (to_float \"-0x0eC18e8B6e.7cDA2AEp720\")) (catch e -1.0) v) (try_let (v (to_float \"0XDea650D7Ef28F9A74P+42\")) (catch e -1.0) v))",
		"hex 9":  "(print (try_let (v (to_float \"0X4F.6fCd9AB9e8eP+9\")) (catch e -1.0) v) (try_let (v (to_float \"0x50d152fB48b1dC4ccp0\")) (catch e -1.0) v) (try_let (v (to_float \"-0X7Ca068F47E0f89E.2d742P+25\")) (catch e -1.0) v) (try_let (v (to_float \"+0x82bB72ba646d817A.p0\")) (catch e -1.0) v) (try_let (v (to_float \"+0x5dB.1cp0\")) (catch e -1.0) v) (try_let (v (to_float \"0xfEDfB0c.c17bCbP+31\")) (catch e -1.0) v) (try_let (v (to_float \"-0X7.p-117\")) (catch e -1.0) v) (try_let (v (to_float \"0XAp0\")) (catch e -1.0) v) (try_let (v (to_float \"+0X0578c816Cc5Fa0cFeF0Ap0\")) (catch e -1.0) v) (try_let (v (to_float \"+0x3DE76P+50\")) (catch e -1.0) v) (try_let (v (to_float \"+0xD8p-5\")) (catch e -1.0) v) (try_let (v (to_float \"0x2ee6cF88E2194Ea5.Ap-10\")) (catch e -1.0) v))",
		"hex 10": "(print (try_let (v (to_float \"+0X.8066A0FFC6763BAaDp0\")) (catch e -1.0) v) (try_let (v (to_float \"+0X.4CFcC55AE76f3P+12\")) (catch e -1.0) v) (try_let (v (to_float \"-0x7.p0\")) (catch e -1.0) v) (try_let (v (to_float \"+0x.6cp0\")) (catch e -1.0) v) (try_let (v (to_float \"0x2dFE1p-251\")) (catch e -1.0) v) (try_let (v (to_float \"+0X50e4dFfD4828ce909fb.Ep-36\")) (catch e -1.0) v) (try_let (v (to_float \"+0xfp-239\")) (catch e -1.0) v) (try_let (v (to_float \"0Xd0D367A2aE0E7e2C5p0\")) (catch e -1.0) v) (try_let (v (to_float \"-0XDcp0\")) (catch e -1.0) v) (try_let (v (to_float \"0xap30\")) (catch e -1.0) v) (try_let (v (to_float \"-0x297aC9DcCF47aA0Bp917\")) (catch e -1.0) v) (try_let (v (to_float \"-0x869e3332.426E3p-47\")) (catch e -1.0) v))",
		"hex 11": "(print (try_let (v (to_float \"-0X9dbA576dac1bf08Ap660\")) (catch e -1.0) v) (try_let (v (to_float \"-0x.4Bdc6Db2E01e4p653\")) (catch e -1.0) v) (try_let (v (to_float \"-0X3.2p0\")) (catch e -1.0) v) (try_let (v (to_float \"-0xcdp304\")) (catch e -1.0) v) (try_let (v (to_float \"+0X9F842e60b1d6EE3E2P+32\")) (catch e -1.0) v) (try_let (v (to_float \"+0Xb318A1169358F.D2Cp-893\")) (catch e -1.0) v) (try_let (v (to_float \"0xbP+45\")) (catch e -1.0) v) (try_let (v (to_float \"-0x.fp-30\")) (catch e -1.0) v) (try_let (v (to_float \"+0xep1082\")) (catch e -1.0) v) (try_let (v (to_float \"-0x9.p-830\")) (catch e -1.0) v) (try_let (v (to_float \"-0XAb0b0.0EF12dd23A2Cp-48\")) (catch e -1.0) v) (try_let (v (to_float \"-0X.e8cF2e9bfD6E1eDCfFBAp-48\")) (catch e -1.0) v))",
		"hex 12": "(print (try_let (v (to_float \"+0x9aaE4p0\")) (catch e -1.0) v) (try_let (v (to_float \"-0X4957.b39744d3bp0\")) (catch e -1.0) v) (try_let (v (to_float \"-0x21ca3d7e09E6ab1B6p-9\")) (catch e -1.0) v) (try_let (v (to_float \"0XCC.p-12\")) (catch e -1.0) v) (try_let (v (to_float \"0X7f7aa18EE0DFdb4EeP+43\")) (catch e -1.0) v) (try_let (v (to_float \"-0x71A.d7P+31\")) (catch e -1.0) v) (try_let (v (to_float \"0Xa.bP+30\")) (catch e -1.0) v) (try_let (v (to_float \"-0X9Bb4287.2B94522F75p0\")) (catch e -1.0) v) (try_let (v (to_float \"+0x442be19dC05D2p0\")) (catch e -1.0) v) (try_let (v (to_float \"+0x6fp-122\")) (catch e -1.0) v) (try_let (v (to_float \"0xE5ce8b52919aDp0\")) (catch e -1.0) v) (try_let (v (to_float \"0xdp0\")) (catch e -1.0) v))",
		"hex 13": "(print (try_let (v (to_float \"-0X17db.8p0\")) (catch e -1.0) v) (try_let (v (to_float \"+0xcC5.1Cp-52\")) (catch e -1.0) v) (try_let (v (to_float \"+0XFEe07fbEeaBe2BB3ap-839\")) (catch e -1.0) v) (try_let (v (to_float \"+0X4cP+27\")) (catch e -1.0) v) (try_let (v (to_float \"-0xa9B2d2aE29e5de52p0\")) (catch e -1.0) v) (try_let (v (to_float \"0X.27143df5D3aafE3CDP+43\")) (catch e -1.0) v) (try_let (v (to_float \"+0x.Ep-6\")) (catch e -1.0) v) (try_let (v (to_float \"-0x.dB56fp-60\")) (catch e -1.0) v) (try_let (v (to_float \"0x6C912.8b7fd67Ap487\")) (catch e -1.0) v) (try_let (v (to_float \"-0xb88dFBEA194AEef7p-780\")) (catch e -1.0) v) (try_let (v (to_float \"-0X440E.5p496\")) (catch e -1.0) v) (try_let (v (to_float \"-0Xb0fBbB22d.417Cp0\")) (catch e -1.0) v))",
		"hex 14": "(print (try_let (v (to_float \"+0xdee9D78cb9d9F5bbP+17\")) (catch e -1.0) v) (try_let (v (to_float \"-0xCaP+49\")) (catch e -1.0) v) (try_let (v (to_float \"-0xDB7f8BCa0BdF6C7baP+1\")) (catch e -1.0) v) (try_let (v (to_float \"0xD1E0a7b7BdF02P+60\")) (catch e -1.0) v) (try_let (v (to_float \"-0X.deEe1EA3f10a3p-25\")) (catch e -1.0) v) (try_let (v (to_float \"+0xC.Fp0\")) (catch e -1.0) v) (try_let (v (to_float \"-0XC051eaC.B4692eP+3\")) (catch e -1.0) v) (try_let (v (to_float \"0xCp-5\")) (catch e -1.0) v) (try_let (v (to_float \"+0Xccdafp77\")) (catch e -1.0) v) (try_let (v (to_float \"+0X.cFp-10\")) (catch e -1.0) v) (try_let (v (to_float \"+0XECa9ae.2D5AF4Cp548\")) (catch e -1.0) v) (try_let (v (to_float \"0xBe.3D926e4Dfa0AdEde97p-2\")) (catch e -1.0) v))",
		"hex 15": "(print (try_let (v (to_float \"+0X83b11P+27\")) (catch e -1.0) v) (try_let (v (to_float \"0XC48b2p0\")) (catch e -1.0) v) (try_let (v (to_float \"0XE8cCb.d7B8f1B229cp-53\")) (catch e -1.0) v) (try_let (v (to_float \"+0x5.p-45\")) (catch e -1.0) v) (try_let (v (to_float \"+0x3.Cp305\")) (catch e -1.0) v) (try_let (v (to_float \"+0XbAeCDp0\")) (catch e -1.0) v) (try_let (v (to_float \"-0X.968612dAEeBA0244A5f0P+27\")) (catch e -1.0) v) (try_let (v (to_float \"+0x304FAP+3\")) (catch e -1.0) v) (try_let (v (to_float \"0x401f6f48F2b60d54cp0\")) (catch e -1.0) v) (try_let (v (to_float \"+0xBA9Eae011.5BCBBAeap751\")) (catch e -1.0) v) (try_let (v (to_float \"+0x.3p-5\")) (catch e -1.0) v) (try_let (v (to_float \"-0X.dAp-54\")) (catch e -1.0) v))",
		"hex 16": "(print (try_let (v (to_float \"0xee50Be8D9Cf7e6bBcdf2p0\")) (catch e -1.0) v) (try_let (v (to_float \"-0X.FAp-1\")) (catch e -1.0) v) (try_let (v (to_float \"+0X2cA.3Dp0\")) (catch e -1.0) v) (try_let (v (to_float \"+0xDca4DP+12\")) (catch e -1.0) v) (try_let (v (to_float \"-0xA0P+14\")) (catch e -1.0) v) (try_let (v (to_float \"+0x9.bp-44\")) (catch e -1.0) v) (try_let (v (to_float \"+0x5CDb7dBc8be5C4Fd6p-38\")) (catch e -1.0) v) (try_let (v (to_float \"0X3d2D3532f1825BE5916AP+17\")) (catch e -1.0) v) (try_let (v (to_float \"0Xe.p-33\")) (catch e -1.0) v) (try_let (v (to_float \"+0x4c.p0\")) (catch e -1.0) v) (try_let (v (to_float \"+0XCeBF27960b1DC5726FbAP+15\")) (catch e -1.0) v) (try_let (v (to_float \"0X3eBADED4b3B1b0e6p0\")) (catch e -1.0) v))",
		"hex 17": "(print (try_let (v (to_float \"+0Xb636e6b3FEF2A8ff8p-47\")) (catch e -1.0) v) (try_let (v (to_float \"-0XcB3c1a6280859ff5dE66p0\")) (catch e -1.0) v) (try_let (v (to_float \"0xc.P+5\")) (catch e -1.0) v) (try_let (v (to_float \"-0X.bcp0\")) (catch e -1.0) v) (try_let (v (to_float \"-0xf.p592\")) (catch e -1.0) v) (try_let (v (to_float \"0x4B8Ffb2bB129C3c66218P+5\")) (catch e -1.0) v) (try_let (v (to_float \"-0xD9Ef1B1DB28E7P+40\")) (catch e -1.0) v) (try_let (v (to_float \"-0X0b6A8c2D3c0.dF935p0\")) (catch e -1.0) v) (try_let (v (to_float \"-0xC.Fa0140ddfD935aDp0\")) (catch e -1.0) v) (try_let (v (to_float \"+0xc0p-1\")) (catch e -1.0) v) (try_let (v (to_float \"-0xc2d.e8P+21\")) (catch e -1.0) v) (try_let (v (to_float \"0xB.4p-12\")) (catch e -1.0) v))",
		"hex 18": "(print (try_let (v (to_float \"0x1B8B9d0E85F21991BbE7P+59\")) (catch e -1.0) v) (try_let (v (to_float \"0x91dD03B0f15df7FeAp758\")) (catch e -1.0) v) (try_let (v (to_float \"-0x.8P+23\")) (catch e -1.0) v) (try_let (v (to_float \"-0X518aCcCB9Db3fd23b8d7p-45\")) (catch e -1.0) v) (try_let (v (to_float \"0x.ap0\")) (catch e -1.0) v) (try_let (v (to_float \"-0xB3299f.0eFB9dA4256p-40\")) (catch e -1.0) v) (try_let (v (to_float \"+0Xb7EE7b021F8.1d2274p791\")) (catch e -1.0) v) (try_let (v (to_float \"-0x20Bea.4F9fAB8522E2P+31\")) (catch e -1.0) v) (try_let (v (to_float \"-0Xa.p-53\")) (catch e -1.0) v) (try_let (v (to_float \"+0XC7DD0p-17\")) (catch e -1.0) v) (try_let (v (to_float \"0X6p-1030\")) (catch e -1.0) v) (try_let (v (to_float \"0X9a05Dc.0fe77c9BDCP+44\")) (catch e -1.0) v))",
		"hex 19": "(print (try_let (v (to_float \"-0X028.C4p-167\")) (catch e -1.0) v) (try_let (v (to_float \"0xF0Ee7p-9\")) (catch e -1.0) v) (try_let (v (to_float \"+0X2.p-42\")) (catch e -1.0) v) (try_let (v (to_float \"-0XCp-30\")) (catch e -1.0) v))",
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, "(cli_app "+body+")") })
	}
}

// onJSAt runs a cli_app on the JavaScript backend as a web_app in Node:
// the reference for browser host operations that have no VM form. An
// uncaught error is reported as the VM's would be, with its code and
// message, so matchVM can compare it.
func onJSAt(t *testing.T, path string) outcome {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Next to the original, so relative (use ...) paths still resolve.
	web := filepath.Join(filepath.Dir(path), "web_"+filepath.Base(path))
	if err := os.WriteFile(web, []byte(strings.Replace(string(src), "(cli_app", "(web_app", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if out := run(t, compilerBin, "build", web, "--target", "js", "-o", dir); out.failed {
		t.Fatalf("build (js): %s", out.errMsg)
	}
	got := run(t, "node", filepath.Join(dir, "app.js"))
	if got.failed {
		// Node prints "Error: CODE: message" for an uncaught error.
		for _, line := range strings.Split(got.errMsg, "\n") {
			if rest, ok := strings.CutPrefix(line, "Error: "); ok {
				code, msg, _ := strings.Cut(rest, ": ")
				report, _ := json.Marshal(map[string]string{"code": code, "message": msg})
				got.errMsg = string(report)
				got.code = 1
				break
			}
		}
	}
	return got
}

// sameAsJS compares the WebAssembly builds with the JavaScript backend.
func sameAsJS(t *testing.T, src string) {
	t.Helper()
	path := writeSource(t, src)
	want := onJSAt(t, path)
	matchVM(t, "wasm", want, onWasmAt(t, path))
	if _, err := exec.LookPath("wasmtime"); err == nil {
		matchVM(t, "wasmtime", want, onWasmtimeAt(t, path))
	}
}

// base64_text and sniff_media_type are browser host operations of the
// JavaScript backend with no VM form, so it is their reference; this
// backend has them in std/base64.howl.
func TestBase64LikeJavaScript(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	for name, body := range map[string]string{
		"text":            `(print (base64_text "aGVsbG8g4pyT") (base64_text "") (base64_text "YQ==") (base64_text "YWI=") (base64_text "YWJj"))`,
		"bom dropped":     `(print (+ "[" (+ (base64_text "77u/aGk=") "]")) (+ "[" (+ (base64_text "77u/") "]")) (+ "[" (+ (base64_text "aO+7vw==") "]")))`,
		"spare bits":      `(print (base64_text "YR==") (base64_text "YWJ="))`,
		"bad length":      `(print (base64_text "abc"))`,
		"bad character":   `(print (base64_text "ab*d"))`,
		"inner padding":   `(print (base64_text "ab=d"))`,
		"three pads":      `(print (base64_text "a==="))`,
		"not utf-8":       `(print (base64_text "/w=="))`,
		"surrogate":       `(print (base64_text "7aCA"))`,
		"not a string":    `(let (d (dict ("n" 1))) (print (base64_text (map_get d "n"))))`,
		"sniff types":     `(print (sniff_media_type "iVBORw0KGgoAAAANSUhEUg==") (sniff_media_type "/9j/4AAQSkZJRg==") (sniff_media_type "R0lGODlhAQABAAAAACw=") (sniff_media_type "UklGRiQAAABXRUJQVlA4"))`,
		"sniff unknown":   `(print (+ "[" (+ (sniff_media_type "aGVsbG8=") (+ (sniff_media_type "x") (+ (sniff_media_type "") "]")))))`,
		"sniff long data": `(print (sniff_media_type "iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB***not base64***"))`,
		"sniff short":     `(print (+ "[" (+ (sniff_media_type "iVBO") (+ (sniff_media_type "UklGRiQAAAA=") "]"))))`,
		"sniff not text":  `(let (d (dict ("n" 1))) (print (+ "[" (+ (sniff_media_type (map_get d "n")) "]"))))`,
		"caught":          `(try_let (t (base64_text "abc")) (catch e (print "caught")) (print t))`,
	} {
		t.Run(name, func(t *testing.T) { sameAsJS(t, "(cli_app "+body+")") })
	}
}

// The standard library's intrinsics and names are reserved for it.
func TestStdIsReserved(t *testing.T) {
	for name, src := range map[string]string{
		"intrinsic":     `(cli_app (print (__hf_byte_len "abc")))`,
		"reserved name": `(cli_app (defun __std_json_encode (v) (return "x")) (print (encode_json 1)))`,
	} {
		t.Run(name, func(t *testing.T) {
			out := run(t, compilerBin, "build", writeSource(t, src), "--target", "wasmgc", "-o", t.TempDir())
			if !out.failed || !strings.Contains(out.errMsg, "WASM_UNSUPPORTED") {
				t.Fatalf("want a WASM_UNSUPPORTED build failure, got failed=%v: %s", out.failed, out.errMsg)
			}
		})
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
		"typed parameters":   `(cli_app (defun add ((a int) (b int)) int (return (+ a b))) (defun mul (a b) (type_hints (a int) (b int) (return int)) (return (* a b))) (print (call add 2 1) (call mul 2 3)))`,
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, src) })
	}
}

func TestDiffConversions(t *testing.T) {
	for name, body := range map[string]string{
		"to_int":            `(print (to_int 3.7) (to_int -3.7) (to_int "42") (to_int " 42 ") (to_int "-17") (to_int "+5") (to_int 5) (to_int "-9223372036854775808"))`,
		"to_float":          `(print (to_float 3) (to_float "2.5") (to_float "1e3") (to_float " -0.5 ") (to_float 1.5))`,
		"to_int bad text":   `(print (to_int "4.5"))`,
		"to_int word":       `(print (to_int "x"))`,
		"to_int too big":    `(print (to_int "9223372036854775808"))`,
		"to_int bool":       `(print (to_int true))`,
		"to_int big float":  `(print (to_int 9223372036854775807.0))`,
		"to_float word":     `(print (to_float "abc"))`,
		"quoted in message": `(print (to_int "a\"b\\c\nd\te"))`,
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

// to_lower, to_upper and str_trim follow Go's strings.ToLower, ToUpper
// and TrimSpace beyond ASCII (std/unicode.howl, generated from Go's case
// tables). The sweeps cover whole blocks that have been fully assigned
// for a long time, so a newer Unicode version maps them the same.
func TestDiffUnicodeText(t *testing.T) {
	for name, body := range map[string]string{
		"sweep latin-1":                   "(let (s \"ÀÁÂÃÄÅÆÇÈÉÊËÌÍÎÏÐÑÒÓÔÕÖ×ØÙÚÛÜÝÞßàáâãäåæçèéêëìíîïðñòóôõö÷øùúûüýþÿ\") (print (to_lower s) (to_upper s)))",
		"sweep latin extended-a":          "(let (s \"ĀāĂăĄąĆćĈĉĊċČčĎďĐđĒēĔĕĖėĘęĚěĜĝĞğĠġĢģĤĥĦħĨĩĪīĬĭĮįİıĲĳĴĵĶķĸĹĺĻļĽľĿŀŁłŃńŅņŇňŉŊŋŌōŎŏŐőŒœŔŕŖŗŘřŚśŜŝŞşŠšŢţŤťŦŧŨũŪūŬŭŮůŰűŲųŴŵŶŷŸŹźŻżŽžſ\") (print (to_lower s) (to_upper s)))",
		"sweep latin extended additional": "(let (s \"ḀḁḂḃḄḅḆḇḈḉḊḋḌḍḎḏḐḑḒḓḔḕḖḗḘḙḚḛḜḝḞḟḠḡḢḣḤḥḦḧḨḩḪḫḬḭḮḯḰḱḲḳḴḵḶḷḸḹḺḻḼḽḾḿṀṁṂṃṄṅṆṇṈṉṊṋṌṍṎṏṐṑṒṓṔṕṖṗṘṙṚṛṜṝṞṟṠṡṢṣṤṥṦṧṨṩṪṫṬṭṮṯṰṱṲṳṴṵṶṷṸṹṺṻṼṽṾṿẀẁẂẃẄẅẆẇẈẉẊẋẌẍẎẏẐẑẒẓẔẕẖẗẘẙẚẛẜẝẞẟẠạẢảẤấẦầẨẩẪẫẬậẮắẰằẲẳẴẵẶặẸẹẺẻẼẽẾếỀềỂểỄễỆệỈỉỊịỌọỎỏỐốỒồỔổỖỗỘộỚớỜờỞởỠỡỢợỤụỦủỨứỪừỬửỮữỰựỲỳỴỵỶỷỸỹỺỻỼỽỾỿ\") (print (to_lower s) (to_upper s)))",
		"sweep greek":                     "(let (s \"ΑΒΓΔΕΖΗΘΙΚΛΜΝΞΟΠΡΣΤΥΦΧΨΩΪΫάέήίΰαβγδεζηθικλμνξοπρςστυφχψω\") (print (to_lower s) (to_upper s)))",
		"sweep cyrillic":                  "(let (s \"ЀЁЂЃЄЅІЇЈЉЊЋЌЍЎЏАБВГДЕЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯабвгдежзийклмнопрстуфхцчшщъыьэюяѐёђѓєѕіїјљњћќѝўџѠѡѢѣѤѥѦѧѨѩѪѫѬѭѮѯѰѱѲѳѴѵѶѷѸѹѺѻѼѽѾѿҀҁ҂҃҄҅҆҇҈҉ҊҋҌҍҎҏҐґҒғҔҕҖҗҘҙҚқҜҝҞҟҠҡҢңҤҥҦҧҨҩҪҫҬҭҮүҰұҲҳҴҵҶҷҸҹҺһҼҽҾҿӀӁӂӃӄӅӆӇӈӉӊӋӌӍӎӏӐӑӒӓӔӕӖӗӘәӚӛӜӝӞӟӠӡӢӣӤӥӦӧӨөӪӫӬӭӮӯӰӱӲӳӴӵӶӷӸӹӺӻӼӽӾӿ\") (print (to_lower s) (to_upper s)))",
		"sweep armenian":                  "(let (s \"ԱԲԳԴԵԶԷԸԹԺԻԼԽԾԿՀՁՂՃՄՅՆՇՈՉՊՋՌՍՎՏՐՑՒՓՔՕՖաբգդեզէըթժիլխծկհձղճմյնշոչպջռսվտրցւփքօֆ\") (print (to_lower s) (to_upper s)))",
		"sweep deseret":                   "(let (s \"𐐀𐐁𐐂𐐃𐐄𐐅𐐆𐐇𐐈𐐉𐐊𐐋𐐌𐐍𐐎𐐏𐐐𐐑𐐒𐐓𐐔𐐕𐐖𐐗𐐘𐐙𐐚𐐛𐐜𐐝𐐞𐐟𐐠𐐡𐐢𐐣𐐤𐐥𐐦𐐧𐐨𐐩𐐪𐐫𐐬𐐭𐐮𐐯𐐰𐐱𐐲𐐳𐐴𐐵𐐶𐐷𐐸𐐹𐐺𐐻𐐼𐐽𐐾𐐿𐑀𐑁𐑂𐑃𐑄𐑅𐑆𐑇𐑈𐑉𐑊𐑋𐑌𐑍𐑎𐑏\") (print (to_lower s) (to_upper s)))",
		"sweep fullwidth":                 "(let (s \"ＡＢＣＤＥＦＧＨＩＪＫＬＭＮＯＰＱＲＳＴＵＶＷＸＹＺ［＼］＾＿｀ａｂｃｄｅｆｇｈｉｊｋｌｍｎｏｐｑｒｓｔｕｖｗｘｙｚ\") (print (to_lower s) (to_upper s)))",
		"unicode case":                    "(print (to_lower \"ÀÉÎÕÜ ΑΒΓ ДЖЯ İ ẞ Ǆǅǆ ⅠⅫ Ａ ΣΑΣ\") (to_upper \"àéîõü αβγ джя ı ß ǆ ⅰ ａ ſ ﬀ\") (str_len (to_lower \"İ\")) (str_len (to_upper \"ſ\")))",
		"nil text":                        "(let (d (dict (\"a\" 1) (\"n\" (dict)))) (print (+ \"[\" (+ (to_lower (map_get d \"zz\")) \"]\")) (+ \"[\" (+ (to_upper (map_get d \"zz\")) \"]\")) (+ \"[\" (+ (str_trim (map_get d \"zz\")) \"]\"))))",
		"unicode trim":                    "(print (+ \"[\" (+ (str_trim \"   x y　 \") \"]\")) (+ \"[\" (+ (str_trim \"​ x ​\") \"]\")) (+ \"[\" (+ (str_trim \"      \") \"]\")) (+ \"[\" (+ (str_trim \"é \") \"]\")))",
		"conversions trim unicode":        "(print (to_int \" 42　\") (to_float \"  2.5 \"))",
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, "(cli_app "+body+")") })
	}
}

// bytes_to_string builds a string from a list of byte values (Go's
// byte(float64(n)) for each), which is also how a program makes text that
// is not valid UTF-8; the operations after it must treat such text as the
// VM's Go code does.
func TestDiffBytes(t *testing.T) {
	// b is (bytes_to_string (list ...)).
	b := func(bytes string) string { return "(bytes_to_string (list " + bytes + "))" }
	valid := func(bytes string) string { return "(utf8_valid " + b(bytes) + ")" }
	for name, body := range map[string]string{
		"text":             `(print ` + b("104 105 195 169") + ` (str_len ` + b("104 105 195 169") + `) "[" ` + b("") + ` "]")`,
		"byte values":      `(print (encode_json (str_split ` + b("-1 256 300 511 1000 65") + ` "")) (encode_json (str_split ` + b("2.7 -3.5 66.9 1000000000000000000000000000000.0 -0.9") + ` "")))`,
		"raw output":       `(do (print ` + b("97 255 98 0 99") + `) (stderr ` + b("99 128 100") + `))`,
		"valid utf-8":      `(print (utf8_valid "") (utf8_valid "ok") (utf8_valid "héllo ✓ 🎉") ` + valid("244 143 191 191") + ` ` + valid("194 128") + ` ` + valid("237 159 191") + `)`,
		"invalid utf-8":    `(print ` + valid("255") + ` ` + valid("128") + ` ` + valid("192 128") + ` ` + valid("224 128 128") + ` ` + valid("237 160 128") + ` ` + valid("244 144 128 128") + ` ` + valid("248 136 128 128 128") + ` ` + valid("226 130") + ` ` + valid("97 226") + `)`,
		"json of invalid":  `(print (encode_json ` + b("97 255 98 226 130 99 237 160 128 100") + `))`,
		"split invalid":    `(print (list_len (str_split ` + b("97 255 226 130 240 159 152 128 98") + ` "")) (encode_json (str_split ` + b("97 255 226 130 240 159 152 128 98") + ` "")))`,
		"case of invalid":  `(print (encode_json (to_lower ` + b("65 255 66 195") + `)) (encode_json (to_upper ` + b("97 128 98") + `)) (encode_json (to_lower ` + b("65 66") + `)))`,
		"regex on invalid": `(print (regex_match "^a.b$" ` + b("97 255 98") + `) (regex_match "^a..b$" ` + b("97 226 130 98") + `) (regex_match "^[^x]+$" ` + b("255 254") + `))`,
		"parse invalid":    `(let (t ` + b("34 97 255 98 34") + `) (print (encode_json (parse_json Any t))))`,
		"escape invalid":   `(print (encode_json (html_escape ` + b("60 255 62") + `)) (str_contains ` + b("97 255 98") + ` ` + b("255") + `))`,
		"not a list":       `(let (d ` + mixed + `) (print (bytes_to_string (map_get d "b"))))`,
		"of nil":           `(let (d ` + mixed + `) (print (bytes_to_string (map_get d "zz"))))`,
		"of a dict":        `(let (d ` + mixed + `) (print (bytes_to_string (map_get d "n"))))`,
		"bad element":      `(let (d ` + mixed + `) (print (bytes_to_string (list (map_get d "b")))))`,
		"bool element":     `(let (d ` + mixed + `) (print (bytes_to_string (list (map_get d "e")))))`,
		"utf8_valid type":  `(let (d ` + mixed + `) (print (utf8_valid (map_get d "a"))))`,
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
