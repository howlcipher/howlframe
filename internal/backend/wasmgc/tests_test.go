package wasmgc

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// testReport is a run of a program's test blocks, as node:test's spec
// reporter shows it: each test's result in order, the error of each
// failing test in order, the counts, the program's own output and the exit
// status. Durations are left out.
type testReport struct {
	results []string // "✔ name" or "✖ name"
	errs    []string // "Error: ..." for each failing test
	counts  []string // "ℹ tests 3", ... without duration_ms
	output  []string // the tests' own lines (print, stderr), sorted
	code    int
}

var specDuration = regexp.MustCompile(` \([0-9.]+ms\)$`)

// parseSpec reads node:test's spec reporter output, or this backend's
// runner's, which writes the same lines without durations and with each
// failing test's error right under it.
func parseSpec(out string, code int) testReport {
	r := testReport{code: code}
	failing := false
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case line == "✖ failing tests:":
			failing = true
		case failing:
			// "test at ...", then "✖ name (..ms)" and its "  Error: ..."
			if strings.HasPrefix(line, "✖ ") && i+1 < len(lines) {
				r.errs = append(r.errs, strings.TrimSpace(lines[i+1]))
				i++
			}
		case strings.HasPrefix(line, "✔ ") || strings.HasPrefix(line, "✖ "):
			r.results = append(r.results, specDuration.ReplaceAllString(line, ""))
			if strings.HasPrefix(line, "✖ ") && i+1 < len(lines) && strings.HasPrefix(lines[i+1], "  Error: ") {
				r.errs = append(r.errs, strings.TrimSpace(lines[i+1]))
				i++
			}
		case strings.HasPrefix(line, "ℹ "):
			if !strings.HasPrefix(line, "ℹ duration_ms") {
				r.counts = append(r.counts, line)
			}
		case line != "":
			r.output = append(r.output, line)
		}
	}
	sort.Strings(r.output)
	return r
}

// runMerged runs a command with standard output and error in one stream,
// as node:test reports a test file's output.
func runMerged(t *testing.T, name string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = t.TempDir()
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return out.String(), exit.ExitCode()
	}
	if err != nil {
		t.Fatal(err)
	}
	return out.String(), 0
}

// jsTestsAt runs a program's tests on the JavaScript backend, the
// reference for test blocks: its app.test.js under node --test. A program
// without tests has no app.test.js; its report is node:test's for none.
func jsTestsAt(t *testing.T, path string) testReport {
	t.Helper()
	dir := t.TempDir()
	if out := run(t, compilerBin, "build", path, "--target", "js", "-o", dir); out.failed {
		t.Fatalf("build (js): %s", out.errMsg)
	}
	file := filepath.Join(dir, "app.test.js")
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return testReport{counts: []string{"ℹ tests 0", "ℹ suites 0", "ℹ pass 0", "ℹ fail 0", "ℹ cancelled 0", "ℹ skipped 0", "ℹ todo 0"}}
	}
	return parseSpec(runMerged(t, "node", "--test", "--test-reporter=spec", file))
}

// wasmTestsAt builds a program's tests with --tests and runs them in Node
// or, for the wasi host, in wasmtime.
func wasmTestsAt(t *testing.T, path string, host Host) testReport {
	t.Helper()
	dir := t.TempDir()
	if out := run(t, compilerBin, "build", path, "--target", "wasmgc", "--tests", "--host", string(host), "-o", dir); out.failed {
		t.Fatalf("build (%s): %s", host, out.errMsg)
	}
	wasm := filepath.Join(dir, "app.test.wasm")
	if tools, err := exec.LookPath("wasm-tools"); err == nil {
		if out, err := exec.Command(tools, "validate", "--features", "all", wasm).CombinedOutput(); err != nil {
			t.Fatalf("wasm-tools rejects the module: %v\n%s", err, out)
		}
	}
	if host == HostWASI {
		return parseSpec(runMerged(t, "wasmtime", "run", "-W", "gc=y,function-references=y", wasm))
	}
	return parseSpec(runMerged(t, "node", filepath.Join(dir, "app.test.mjs")))
}

// sameTestsAsJS runs a program's tests on the JavaScript backend and as
// WebAssembly in Node and, when installed, wasmtime, and requires the same
// report.
func sameTestsAsJS(t *testing.T, path string) testReport {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	want := jsTestsAt(t, path)
	hosts := []Host{HostNode}
	if _, err := exec.LookPath("wasmtime"); err == nil {
		hosts = append(hosts, HostWASI)
	}
	for _, host := range hosts {
		got := wasmTestsAt(t, path, host)
		for _, field := range []struct {
			what      string
			want, got []string
		}{
			{"results", want.results, got.results},
			{"errors", want.errs, got.errs},
			{"counts", want.counts, got.counts},
			{"output", want.output, got.output},
		} {
			if strings.Join(field.want, "\n") != strings.Join(field.got, "\n") {
				t.Fatalf("%s differ (%s):\njs   %q\nwasm %q", field.what, host, field.want, field.got)
			}
		}
		if got.code != want.code {
			t.Fatalf("exit status differs (%s): js %d, wasm %d", host, want.code, got.code)
		}
	}
	return want
}

// A program's test blocks compiled with --tests report like the
// JavaScript backend's app.test.js under node --test: the same tests in the
// same order, passing and failing alike, with the same errors, counts,
// output and exit status. want is the JavaScript run's results, checked so
// that no fixture is vacuous.
func TestTestsLikeJavaScript(t *testing.T) {
	for name, tc := range map[string]struct {
		files map[string]string
		want  []string
	}{
		"pass": {map[string]string{"main.howl": `(web_app
  (defun sq ((x int)) (type_hint return "int") (return (* x x)))
  (test "squares" (assert (= (call sq 3) 9)))
  (test "strings" (do (assert (= (str_join (list "a" "b") "-") "a-b")) (print "ran strings"))))`},
			[]string{"✔ squares", "✔ strings"}},
		"fail": {map[string]string{"main.howl": `(web_app
  (test "first" (assert true))
  (test "second fails" (do (print "before") (assert (= 1 2)) (print "not reached")))
  (test "third still runs" (print "third")))`},
			[]string{"✔ first", "✖ second fails", "✔ third still runs"}},
		"error in a test": {map[string]string{"main.howl": `(web_app
  (test "divides by zero" (print (/ 1 0)))
  (test "bad conversion" (print (to_int "x1")))
  (test "after errors" (assert (= (+ 1 1) 2))))`},
			[]string{"✖ divides by zero", "✖ bad conversion", "✔ after errors"}},
		"assert in a function": {map[string]string{"main.howl": `(web_app
  (defun check ((x int)) (type_hint return "int") (do (assert (= x 3)) (return x)))
  (test "deep pass" (print (call check 3)))
  (test "deep fail" (print (call check 4))))`},
			[]string{"✔ deep pass", "✖ deep fail"}},
		"exit": {map[string]string{"main.howl": `(web_app
  (test "exits 3" (do (stderr "leaving\n") (exit 3)))
  (test "next" (assert true)))`},
			[]string{"✖ exits 3", "✔ next"}},
		"exit passes try_let": {map[string]string{"main.howl": `(web_app
  (defun leave () (type_hint return "int") (do (exit 2) (return 0)))
  (test "not caught" (try_let (v (call leave)) (catch e (print "caught" e)) (print "value" v)))
  (test "errors still caught" (try_let (v (to_int "x")) (catch e (print "caught")) (print v)))
  (test "nested" (try_let (a (to_int "1")) (catch e (print "outer")) (try_let (b (call leave)) (catch e (print "inner")) (print a b)))))`},
			[]string{"✖ not caught", "✔ errors still caught", "✖ nested"}},
		"empty file": {map[string]string{"main.howl": `(web_app (defun f () (type_hint return "int") (return 1)))`}, nil},
		"top level is not run": {map[string]string{"main.howl": `(web_app
  (print "application start")
  (test "only the test" (print "in test")))`},
			[]string{"✔ only the test"}},
		"cli_app root": {map[string]string{"main.howl": `(cli_app
  (print "application start")
  (test "cli test" (assert (= (to_int "42") 42))))`},
			[]string{"✔ cli test"}},
		"tests from modules": {map[string]string{
			"lib/strings.howl": `(module
  (defun twice ((s string)) (type_hint return "string") (return (+ s s)))
  (export (defun shout ((s string)) (type_hint return "string") (return (call twice (to_upper s)))))
  (test "module private helper" (assert (= (call twice "ab") "abab")))
  (test "module fails" (assert (= (call twice "a") "a"))))`,
			"main.howl": `(web_app
  (test "before the use" (assert true))
  (use "lib/strings.howl" as str)
  (test "exported" (assert (= (call str/shout "hi") "HIHI"))))`},
			[]string{"✔ before the use", "✔ module private helper", "✖ module fails", "✔ exported"}},
		"same names": {map[string]string{"main.howl": `(web_app
  (test "twin" (assert true))
  (test "twin" (assert false)))`},
			[]string{"✔ twin", "✖ twin"}},
		"unicode name": {map[string]string{"main.howl": `(web_app
  (test "größe ✓ \"quoted\"" (assert (= (str_len "größe") 5))))`},
			[]string{"✔ größe ✓ \"quoted\""}},
	} {
		t.Run(name, func(t *testing.T) {
			js := sameTestsAsJS(t, writeFiles(t, tc.files, "main.howl"))
			if strings.Join(js.results, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("the JavaScript reference gave %q, the fixture expects %q", js.results, tc.want)
			}
		})
	}
}

// Test mode fails closed: a root other than cli_app or web_app, a test
// block whose body this backend cannot compile (with its location), a
// malformed test, and --tests without --target wasmgc are build errors;
// and a cli_app with test blocks still refuses them when built as the
// program, as the VM does.
func TestTestsFailClosed(t *testing.T) {
	for name, tc := range map[string]struct {
		src  string
		args []string
		want string
	}{
		"http_server root":   {`(http_server (port 8080) (test "t" (assert true)))`, []string{"--tests"}, "WASM_UNSUPPORTED: test blocks compile from a cli_app or web_app"},
		"unsupported body":   {"(web_app\n  (test \"dom\"\n    (dom_query \"#x\")))", []string{"--tests"}, `WASM_UNSUPPORTED: "dom_query" is not supported by the WebAssembly backend yet (line 3, column 5)`},
		"unnamed test":       {`(web_app (test (assert true) (assert true)))`, []string{"--tests"}, "test description must be a string"},
		"web_app as program": {`(web_app (test "t" (assert true)))`, nil, "the test blocks of a web_app build with --tests"},
		"cli_app as program": {`(cli_app (print "x") (test "t" (assert true)))`, nil, `WASM_UNSUPPORTED: "test"`},
	} {
		t.Run(name, func(t *testing.T) {
			args := append([]string{"build", writeSource(t, tc.src), "--target", "wasmgc", "-o", t.TempDir()}, tc.args...)
			out := run(t, compilerBin, args...)
			if !out.failed || !strings.Contains(out.errMsg, tc.want) {
				t.Fatalf("want a build failure with %q, got failed=%v: %s", tc.want, out.failed, out.errMsg)
			}
		})
	}
	out := run(t, compilerBin, "build", writeSource(t, `(web_app (test "t" (assert true)))`), "--target", "js", "--tests", "-o", t.TempDir())
	if !out.failed || !strings.Contains(out.errMsg, "--tests requires --target wasmgc") {
		t.Fatalf("--tests with --target js: failed=%v: %s", out.failed, out.errMsg)
	}
}
