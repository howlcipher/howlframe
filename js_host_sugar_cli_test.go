package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Sugar nested in an argument of a browser host operation expands and is
// emitted as a value, as under any other value-taking form. Before, the
// desugarer skipped host operations and (set_value el (not x)) failed with
// "Unknown statement for JS: not".
func TestJSHostOperationArgumentsExpandSugar(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	bin := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "app.howl")
	os.WriteFile(src, []byte(`(web_app (do
  (set_value (dom_query "#a") (not true))
  (set_attr (dom_query "#a") "data-k" (str "x" (to_string (not false))))
  (set_class (dom_query "#a") "on" (not false))
  (on_visible (dom_query_all ".scene") (lambda (el v) (set_class el "seen" (not (not v)))))))
`), 0o600)
	out := filepath.Join(dir, "out")
	if msg, err := exec.Command(bin, "build", src, "--target", "js", "-o", out).CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, msg)
	}
	app, err := os.ReadFile(filepath.Join(out, "app.js"))
	if err != nil {
		t.Fatal(err)
	}
	harness := filepath.Join(dir, "run.js")
	os.WriteFile(harness, []byte(`const vm = require("node:vm");
const classes = new Set(); const attrs = {};
const a = { value: "", setAttribute(n, v) { attrs[n] = v; }, classList: { toggle(n, on) { if (on) classes.add(n); else classes.delete(n); } } };
const ctx = { console, setTimeout, document: { querySelector: () => a, querySelectorAll: () => [a] } };
Promise.resolve(vm.runInNewContext(require("fs").readFileSync(process.argv[2], "utf8"), ctx))
  .then(() => new Promise((r) => setTimeout(r, 10)))
  .then(() => console.log(JSON.stringify({ value: a.value, attrs, classes: [...classes].sort() })));
`), 0o600)
	got, err := exec.Command("node", harness, filepath.Join(out, "app.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\n%s\n%s", err, got, app)
	}
	want := `{"value":"false","attrs":{"data-k":"xtrue"},"classes":["on","seen"]}`
	if strings.TrimSpace(string(got)) != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
}
