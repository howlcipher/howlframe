package wasmgc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestModuleExportsAsNodeESM(t *testing.T) {
	source := `(web_app
  (export (defun add (a b)
    (type_hint a "int")
    (type_hint b "int")
    (type_hint return "int")
    (return (+ a b))))
  (export (defun parse (s)
    (type_hint s "string")
    (type_hint return "int")
    (return (to_int s))))
  (export (defun echo (value)
    (type_hint value "dict")
    (type_hint return "dict")
    (return value)))
  (export (defun delayed_add (a b)
    (type_hint a "int")
    (type_hint b "int")
    (type_hint return "int")
    (sleep 1)
    (return (+ a b))))
  (export (defun nothing ()
    (type_hint return "void")
    (print "void-called"))))`
	input := writeSource(t, source)
	outDir := t.TempDir()
	if out := run(t, compilerBin, "build", input, "-o", outDir, "--target", "wasmgc", "--module"); out.failed {
		t.Fatalf("compile WasmGC module: %s", out.errMsg)
	}
	for _, name := range []string{"app.wasm", "app.mjs", "app.d.mts"} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Fatalf("module output %s: %v", name, err)
		}
	}
	decl, err := os.ReadFile(filepath.Join(outDir, "app.d.mts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"export declare function add(a: number, b: number): Promise<number>;",
		"export declare function parse(s: string): Promise<number>;",
		"export declare function echo(value: Record<string, unknown>): Promise<Record<string, unknown>>;",
		"export declare function delayed_add(a: number, b: number): Promise<number>;",
		"export declare function nothing(): Promise<void>;",
	} {
		if !strings.Contains(string(decl), want) {
			t.Errorf("declarations do not include %q:\n%s", want, decl)
		}
	}
	if wasmTools, err := exec.LookPath("wasm-tools"); err == nil {
		cmd := exec.Command(wasmTools, "validate", "--features", "all", filepath.Join(outDir, "app.wasm"))
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("wasm-tools rejects module: %v\n%s", err, output)
		}
	}
	node := `import assert from "node:assert/strict";
import { add, delayed_add, echo, nothing, parse } from "./app.mjs";
assert.equal(await add(20, 22), 42);
assert.deepEqual(await echo({ text: "λ", values: [true, 4, null] }), { text: "λ", values: [true, 4, null] });
assert.deepEqual(await Promise.all([delayed_add(20, 22), add(1, 2)]), [42, 3]);
assert.equal(await nothing(), undefined);
await assert.rejects(() => parse("not-a-number"), /CONVERSION_ERROR/);
console.log("external Node consumer passed");
`
	consumer := filepath.Join(outDir, "consumer.mjs")
	if err := os.WriteFile(consumer, []byte(node), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", consumer)
	cmd.Dir = outDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Node consumer: %v\n%s", err, output)
	} else if got := strings.TrimSpace(string(output)); got != "void-called\nexternal Node consumer passed" {
		t.Fatalf("Node consumer output %q", got)
	}
}

func TestModuleRejectsBrowserHostOperations(t *testing.T) {
	source := `(web_app
  (export (defun query ()
    (type_hint return "any")
    (return (dom_query "#secret")))) )`
	input := writeSource(t, source)
	out := run(t, compilerBin, "build", input, "-o", t.TempDir(), "--target", "wasmgc", "--module")
	if !out.failed || !strings.Contains(out.errMsg, "Node module exports cannot use browser host operations: dom_query") {
		t.Fatalf("module should reject browser host operation, got failed=%v output=%s", out.failed, out.errMsg)
	}
}
