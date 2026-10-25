package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Issue #224: an imported module function is called as alias/name. The dotted
// alias.name spelling used to pass check and build, and the VM then refused
// the artifact for a missing function. Every compile path must now reject it
// before an artifact exists, with a diagnostic that names the slash form.
func TestIssue224DottedModuleCallRejectedBeforeArtifact(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	write := func(name, content string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	write("inc.howl", `(module
  (defun helper (x) (type_hint x "int") (type_hint return "int") (return x))
  (export (defun add_one (x) (type_hint x "int") (type_hint return "int") (return (+ x 1)))))`)

	run := func(args ...string) (string, error) {
		t.Helper()
		cmd := exec.Command(binary, args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	cases := []struct {
		name, source string
		want         []string
	}{
		{"exported", `(cli_app (use "inc.howl" as m) (print (call m.add_one 41)))`,
			[]string{`undefined reference to function \"m.add_one\"`, `(call m/add_one ...)`}},
		{"private", `(cli_app (use "inc.howl" as m) (print (call m.helper 41)))`,
			[]string{`undefined reference to function \"m.helper\"`, `module m does not export helper`}},
		{"missing member", `(cli_app (use "inc.howl" as m) (print (call m.nosuch 41)))`,
			[]string{`undefined reference to function \"m.nosuch\"`, `(call m/nosuch ...)`}},
		{"unknown prefix", `(cli_app (print (call x.y 1)))`,
			[]string{`undefined reference to function \"x.y\"`}},
		{"unqualified", `(cli_app (use "inc.howl" as m) (print (call add_one 41)))`,
			[]string{`undefined reference to function \"add_one\"`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := write("main.howl", tc.source)
			artifact := filepath.Join(dir, "out.hfbc")
			invocations := [][]string{
				{"check", source},
				{"build", source, "-o", artifact},
				{"run", source},
				{"-run", source},
				{"-compile-bc", source, "-o", artifact},
				{"-compile-hfir-bc", source, "-o", artifact},
			}
			for _, args := range invocations {
				os.Remove(artifact)
				out, err := run(args...)
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

// Valid alias/name calls keep working on every path, including two modules
// that export the same name and a local defun that shares it.
func TestIssue224OverlappingModuleExportsResolve(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	files := map[string]string{
		"a.howl": `(module (export (defun add_one (x) (type_hint x "int") (type_hint return "int") (return (+ x 1)))))`,
		"b.howl": `(module (export (defun add_one (x) (type_hint x "int") (type_hint return "int") (return (+ x 100)))))`,
		"main.howl": `(cli_app (use "a.howl" as a) (use "b.howl" as b)
  (defun add_one (x) (type_hint x "int") (type_hint return "int") (return (+ x 1000)))
  (print (call a/add_one 1)) (print (call b/add_one 1)) (print (call add_one 1)))`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	const want = "2\n101\n1001\n"
	exec1 := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(binary, args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	exec1("build", "main.howl", "-o", "main.hfbc")
	if got := exec1("run", "main.hfbc"); got != want {
		t.Fatalf("run artifact = %q, want %q", got, want)
	}
	if got := exec1("-run", "main.howl"); got != want {
		t.Fatalf("-run = %q, want %q", got, want)
	}
	exec1("-compile-hfir-bc", "main.howl", "-o", "hfir.hfbc")
	if got := exec1("run", "hfir.hfbc"); got != want {
		t.Fatalf("HFIR artifact = %q, want %q", got, want)
	}
}

// Dotted calls remain Go interop: a go_import package function still builds
// for the Go target, but a dotted spelling of a module export does not.
func TestIssue224GoInteropDottedCallsUnchanged(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "inc.howl"),
		[]byte(`(module (export (defun add_one (x) (type_hint x "int") (type_hint return "int") (return (+ x 1)))))`), 0o600); err != nil {
		t.Fatal(err)
	}
	build := func(source string) (string, error) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "main.howl"), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binary, "build", "main.howl", "--target=go", "-o", "gen")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := build(`(cli_app (go_import "strings") (print (call strings.ToUpper "a")))`); err != nil {
		t.Fatalf("go_import interop call rejected: %v\n%s", err, out)
	}
	out, err := build(`(cli_app (go_import "strings") (use "inc.howl" as m) (print (call m.add_one 41)))`)
	if err == nil || !strings.Contains(out, `(call m/add_one ...)`) {
		t.Fatalf("dotted module call accepted beside go_import: %v\n%s", err, out)
	}
}
