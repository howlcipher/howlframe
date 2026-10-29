package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const jsModuleProgram = `(web_app
  (use "shapes.howl" as shapes)

  (defun shout (s)
    (return (to_upper s)))

  (export (defun greet (name)
    (return (str_join (list "Hello, " (call shout name)) ""))))

  (export (defun area (w h)
    (type_hint w "int")
    (type_hint h "int")
    (type_hint return "int")
    (return (call shapes/area w h))))

  (export (defun describe (d tags flag anything)
    (type_hint d "dict")
    (type_hint tags "list")
    (type_hint flag "bool")
    (type_hint anything "any")
    (type_hint return "dict")
    (return (dict ("keys" (list_len (map_keys d))) ("tags" (list_len tags)) ("flag" flag)))))

  (export (defun note (s)
    (type_hint return "void")
    (print s)))

  (test "area" (assert (= (call area 2 3) 6)))
)`

const jsModuleShapes = `(module
  (export (defun area (w h)
    (type_hint w "int")
    (type_hint h "int")
    (type_hint return "int")
    (return (* w h)))))`

func buildJSModule(t *testing.T, bin, src string) (string, string, error) {
	t.Helper()
	dir := t.TempDir()
	mustWriteFile(t, dir, "shapes.howl", jsModuleShapes)
	path := mustWriteFile(t, dir, "lib.howl", src)
	out := filepath.Join(dir, "out")
	b, err := exec.Command(bin, "build", path, "--target", "js", "--module", "-o", out).CombinedOutput()
	return out, string(b), err
}

// --module writes an ES module that exports exactly the program's top-level
// exported functions, with TypeScript declarations from their type hints.
func TestJSModuleExportsTopLevelFunctions(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	bin := buildHowlFrameBinaryForTest(t)
	out, log, err := buildJSModule(t, bin, jsModuleProgram)
	if err != nil {
		t.Fatalf("build: %v %s", err, log)
	}
	consumer := mustWriteFile(t, out, "consumer.mjs", `import * as lib from "./app.mjs";
console.log(JSON.stringify(Object.keys(lib).sort()));
console.log(await lib.greet("ada"));
console.log(await lib.area(6, 7));
console.log(JSON.stringify(await lib.describe({a: 1, b: 2}, ["x"], true, null)));
`)
	got, err := exec.Command("node", consumer).CombinedOutput()
	if err != nil {
		t.Fatalf("consumer failed: %v %s", err, got)
	}
	want := `["area","describe","greet","note"]
Hello, ADA
42
{"keys":2,"tags":1,"flag":true}
`
	if string(got) != want {
		t.Fatalf("consumer printed:\n%s\nwant:\n%s", got, want)
	}

	decl, err := os.ReadFile(filepath.Join(out, "app.d.mts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"export declare function greet(name: string): Promise<string>;",
		"export declare function area(w: number, h: number): Promise<number>;",
		"export declare function describe(d: Record<string, unknown>, tags: unknown[], flag: boolean, anything: unknown): Promise<Record<string, unknown>>;",
		"export declare function note(s: string): Promise<void>;",
	} {
		if !strings.Contains(string(decl), line) {
			t.Fatalf("app.d.mts lacks %q:\n%s", line, decl)
		}
	}
	if strings.Contains(string(decl), "shout") {
		t.Fatalf("private function declared:\n%s", decl)
	}

	// The node:test file is unchanged by --module and still runs.
	cmd := exec.Command("node", "--test", "app.test.js")
	cmd.Dir = out
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("app.test.js: %v %s", err, b)
	}
}

func TestJSModuleRefusals(t *testing.T) {
	bin := buildHowlFrameBinaryForTest(t)
	for _, tc := range []struct{ name, src, want string }{
		{"no exports", `(web_app (defun f () (return "x")))`, "no top-level (export (defun ...))"},
		{"node host operation", `(web_app (export (defun f () (return (env "HOME")))))`, "not available in --module output"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, log, err := buildJSModule(t, bin, tc.src)
			if err == nil || !strings.Contains(log, tc.want) {
				t.Fatalf("want refusal containing %q, got %v %s", tc.want, err, log)
			}
		})
	}
	dir := t.TempDir()
	path := mustWriteFile(t, dir, "b.howl", `(cli_app (print "x"))`)
	if b, err := exec.Command(bin, "build", path, "--module", "-o", filepath.Join(dir, "o.hfbc")).CombinedOutput(); err == nil || !strings.Contains(string(b), "--module requires --target js") {
		t.Fatalf("--module without --target js must be refused, got %v %s", err, b)
	}
}
