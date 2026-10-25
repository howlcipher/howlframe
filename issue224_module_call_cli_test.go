package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const issue224Module = `(module
  (defun helper (x) (type_hint x "int") (type_hint return "int") (return x))
  (export (defun add_one (x) (type_hint x "int") (type_hint return "int") (return (+ x 1)))))`

type issue224Env struct {
	t      *testing.T
	binary string
	dir    string
}

func newIssue224Env(t *testing.T) issue224Env {
	t.Helper()
	env := issue224Env{t: t, binary: buildHowlFrameBinaryForTest(t), dir: t.TempDir()}
	env.write("inc.howl", issue224Module)
	return env
}

func (e issue224Env) write(name, content string) string {
	e.t.Helper()
	path := filepath.Join(e.dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		e.t.Fatal(err)
	}
	return path
}

func (e issue224Env) run(args ...string) (string, error) {
	e.t.Helper()
	cmd := exec.Command(e.binary, args...)
	cmd.Dir = e.dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Issue #224: an imported module function is called as alias/name. The dotted
// alias.name spelling used to pass check and build, and the VM then refused
// the artifact for a missing function. Every path now rejects it before an
// artifact exists, with a diagnostic that names the slash form, including a
// dotted alias and a program that also declares go_import.
func TestIssue224DottedModuleCallRejectedBeforeArtifact(t *testing.T) {
	env := newIssue224Env(t)
	cases := []struct {
		name, source string
		want         []string
	}{
		{"exported", `(cli_app (use "inc.howl" as m) (print (call m.add_one 41)))`,
			[]string{`undefined reference to function \"m.add_one\"`, `(call m/add_one ...)`}},
		{"private", `(cli_app (use "inc.howl" as m) (print (call m.helper 41)))`,
			[]string{`undefined reference to function \"m.helper\"`, `module m does not export helper`}},
		{"dotted alias", `(cli_app (use "inc.howl" as m.sub) (print (call m.sub.add_one 41)))`,
			[]string{`undefined reference to function \"m.sub.add_one\"`, `(call m.sub/add_one ...)`}},
		{"beside go_import", `(cli_app (go_import "strings") (use "inc.howl" as m) (print (call m.add_one 41)))`,
			[]string{`(call m/add_one ...)`}},
		{"unqualified", `(cli_app (use "inc.howl" as m) (print (call add_one 41)))`,
			[]string{`undefined reference to function \"add_one\"`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := env.write("main.howl", tc.source)
			artifact := filepath.Join(env.dir, "out.hfbc")
			for _, args := range [][]string{
				{"check", source},
				{"build", source, "-o", artifact},
				{"run", source},
				{"-run", source},
				{"-compile-bc", source, "-o", artifact},
				{"-compile-hfir-bc", source, "-o", artifact},
			} {
				os.Remove(artifact)
				out, err := env.run(args...)
				if err == nil {
					t.Fatalf("%v accepted an undefined call:\n%s", args, out)
				}
				for _, want := range tc.want {
					if !strings.Contains(out, want) {
						t.Fatalf("%v diagnostic lacks %q:\n%s", args, want, out)
					}
				}
				if _, statErr := os.Stat(artifact); statErr == nil {
					t.Fatalf("%v wrote %s despite the error", args, artifact)
				}
			}
		})
	}
}

// A dotted call that is not one of a module's functions may be Go interop,
// so check leaves it alone as before; a bytecode build still refuses to write
// an artifact whose call the VM cannot resolve.
func TestIssue224UnresolvedDottedCallNeverBecomesAnArtifact(t *testing.T) {
	env := newIssue224Env(t)
	for _, source := range []string{
		`(cli_app (use "inc.howl" as m) (print (call m.nosuch 41)))`,
		`(cli_app (print (call x.y 1)))`,
	} {
		path := env.write("main.howl", source)
		artifact := filepath.Join(env.dir, "out.hfbc")
		for _, args := range [][]string{
			{"build", path, "-o", artifact},
			{"-compile-bc", path, "-o", artifact},
			{"-compile-hfir-bc", path, "-o", artifact},
			{"run", path},
		} {
			os.Remove(artifact)
			out, err := env.run(args...)
			if err == nil || !strings.Contains(out, "references missing function") {
				t.Fatalf("%s %v: err=%v\n%s", source, args, err, out)
			}
			if _, statErr := os.Stat(artifact); statErr == nil {
				t.Fatalf("%s %v wrote %s", source, args, artifact)
			}
		}
	}
}

// Valid alias/name calls keep working on every path, including two modules
// that export the same name, a local defun that shares it, and a dotted alias.
func TestIssue224OverlappingModuleExportsResolve(t *testing.T) {
	env := newIssue224Env(t)
	env.write("a.howl", `(module (export (defun add_one (x) (type_hint x "int") (type_hint return "int") (return (+ x 1)))))`)
	env.write("b.howl", `(module (export (defun add_one (x) (type_hint x "int") (type_hint return "int") (return (+ x 100)))))`)
	env.write("main.howl", `(cli_app (use "a.howl" as a) (use "b.howl" as b) (use "inc.howl" as m.sub)
  (defun add_one (x) (type_hint x "int") (type_hint return "int") (return (+ x 1000)))
  (print (call a/add_one 1)) (print (call b/add_one 1)) (print (call add_one 1)) (print (call m.sub/add_one 1)))`)
	const want = "2\n101\n1001\n2\n"
	must := func(args ...string) string {
		t.Helper()
		out, err := env.run(args...)
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
		return out
	}
	must("build", "main.howl", "-o", "main.hfbc")
	if got := must("run", "main.hfbc"); got != want {
		t.Fatalf("run artifact = %q, want %q", got, want)
	}
	if got := must("-run", "main.howl"); got != want {
		t.Fatalf("-run = %q, want %q", got, want)
	}
	must("-compile-hfir-bc", "main.howl", "-o", "hfir.hfbc")
	if got := must("run", "hfir.hfbc"); got != want {
		t.Fatalf("HFIR artifact = %q, want %q", got, want)
	}
}

// Dotted Go interop is unchanged, including beside user functions whose names
// look like mangled module exports, a local that shadows a module alias, and
// a Go standard-library call without go_import (generated Go imports it).
func TestIssue224GoInteropDottedCallsUnchanged(t *testing.T) {
	env := newIssue224Env(t)
	for _, source := range []string{
		`(cli_app (print (call strings.ToUpper "a")))`,
		`(cli_app (go_import "strings")
  (defun strings_Split (s sep) (type_hint s "string") (type_hint sep "string") (type_hint return "list") (return (str_split s sep)))
  (print (call strings.Split "a,b" ",")))`,
		`(cli_app (go_import "database/sql")
  (defun db_Query (q) (type_hint q "string") (type_hint return "string") (return q))
  (db_connect db "sqlite3" "test.db")
  (call db.Query "SELECT 1"))`,
		`(cli_app (use "inc.howl" as m) (let (m (dict)) (print (call m.add_one 1))))`,
	} {
		env.write("main.howl", source)
		if out, err := env.run("check", "main.howl"); err != nil {
			t.Fatalf("check rejected Go interop:\n%s\n%s", source, out)
		}
		if out, err := env.run("build", "main.howl", "--target=go", "-o", "gen"); err != nil {
			t.Fatalf("Go build rejected Go interop:\n%s\n%s", source, out)
		}
	}
}
