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
	got := onWasmAt(t, path)
	if got.failed != want.failed {
		t.Fatalf("failure differs: vm failed=%v (%s), wasm failed=%v (%s)", want.failed, want.errMsg, got.failed, got.errMsg)
	}
	if want.failed {
		// Output before the error, then the message, compared decoded (the VM
		// escapes < and > in its JSON report; the loader does not).
		vmCode, vmMsg := errorReport(want.errMsg)
		gotCode, gotMsg := errorReport(got.errMsg)
		if vmMsg == "" || vmCode != gotCode || !strings.Contains(gotMsg, vmMsg) {
			t.Fatalf("error differs:\nvm   %s\nwasm %s", want.errMsg, got.errMsg)
		}
		return
	}
	if got.stdout != want.stdout {
		t.Fatalf("output differs:\nvm   %q\nwasm %q", want.stdout, got.stdout)
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
// Go's json.Decoder with UseNumber and a check for a second value.
//
// The VM's syntax messages are encoding/json's, which newer Go releases
// word differently (bugs.md #63). std/json.howl gives the messages of the
// toolchain releases are built with; when the test's toolchain words them
// otherwise, a failing fixture is compared by error code and the
// "parse_json failed: " prefix only.
func TestDiffParseJSON(t *testing.T) {
	var v any
	classic := json.Unmarshal([]byte(`"\q"`), &v).Error() == "invalid character 'q' in string escape code"
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
		t.Run(name, func(t *testing.T) {
			if classic {
				sameAsVM(t, "(cli_app "+body+")")
				return
			}
			want, got := onVM(t, "(cli_app "+body+")"), onWasm(t, "(cli_app "+body+")")
			if !want.failed {
				sameAsVM(t, "(cli_app "+body+")")
				return
			}
			wantCode, wantMsg := errorReport(want.errMsg)
			gotCode, gotMsg := errorReport(got.errMsg)
			const prefix = "parse_json failed: "
			if !got.failed || wantCode != gotCode || strings.HasPrefix(wantMsg, prefix) != strings.HasPrefix(gotMsg, prefix) {
				t.Fatalf("error differs:\nvm   %s\nwasm %s", want.errMsg, got.errMsg)
			}
		})
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

func TestDiffStderrAndExit(t *testing.T) {
	for name, body := range map[string]string{
		"exit status":     `(do (print "a") (exit 3) (print "never"))`,
		"exit zero":       `(do (print "a") (exit 0))`,
		"stderr and exit": `(do (stderr "problem") (exit 2))`,
	} {
		t.Run(name, func(t *testing.T) { sameAsVM(t, "(cli_app "+body+")") })
	}
}
