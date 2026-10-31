package wasmgc

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// pageJS runs a built web_app in node:vm against a stand-in page: elements
// #a, #b, #field, #box, #dlg and #none (absent), with attributes, value,
// checked, text, HTML, classes, custom properties, a box, focus and dialog
// state; matchMedia, scroll position, viewport and localStorage. The page
// prints what the program printed, its uncaught error, and every element's
// state, as JSON. argv: the script, then app.wasm for the WebAssembly build.
const pageJS = `const vm = require("node:vm");
const fs = require("node:fs");
const code = fs.readFileSync(process.argv[2], "utf8");
const out = [];
function el(id, attrs) {
  const classes = new Set();
  const e = { id, attrs: Object.assign({}, attrs), value: "", checked: false, open: false, textContent: "", innerHTML: "", props: {},
    getAttribute(n) { return Object.prototype.hasOwnProperty.call(this.attrs, n) ? this.attrs[n] : null; },
    hasAttribute(n) { return Object.prototype.hasOwnProperty.call(this.attrs, n); },
    setAttribute(n, v) { this.attrs[n] = String(v); },
    removeAttribute(n) { delete this.attrs[n]; },
    focus() { out.push("focus:" + id); },
    showModal() { this.open = true; out.push("showModal:" + id); },
    close() { this.open = false; out.push("close:" + id); },
    getBoundingClientRect() { return { top: 120.5, height: 300, left: 0, width: 10 }; },
    closest(sel) { return null; },
    classList: { toggle(n, on) { if (on === undefined) on = !classes.has(n); if (on) classes.add(n); else classes.delete(n); return on; }, get list() { return [...classes].sort(); } },
  };
  e.style = { setProperty(n, v) { e.props[n] = v; } };
  return e;
}
const els = { a: el("a"), b: el("b"), field: el("field"), box: el("box"), dlg: el("dlg") };
const store = new Map();
const browser = {
  console: { log: (v) => out.push(String(v)), error: (v) => out.push("stderr:" + String(v)) },
  TextEncoder, TextDecoder, atob, btoa, setTimeout, Promise,
  scrollY: 250, scrollX: 0, innerHeight: 800,
  scrollTo(opts) { out.push("scrollTo:" + JSON.stringify(opts)); },
  matchMedia: (q) => ({ matches: q === "(prefers-reduced-motion: reduce)", addEventListener() {}, removeEventListener() {} }),
  localStorage: { getItem: (k) => store.has(k) ? store.get(k) : null, setItem: (k, v) => { if (String(v).length > 500) throw new Error("QuotaExceededError"); store.set(k, String(v)); } },
  document: {
    querySelector: (s) => els[s.replace(/^#/, "")] || null,
    querySelectorAll: (s) => s === ".scene" ? [els.a, els.b] : [],
  },
};
if (process.argv[3]) browser.howlFrameWasm = fs.readFileSync(process.argv[3]);
const state = () => Object.fromEntries(Object.entries(els).map(([k, e]) => [k,
  { attrs: e.attrs, value: e.value, checked: e.checked, open: e.open, text: e.textContent, html: e.innerHTML, classes: e.classList.list, props: e.props }]));
Promise.resolve(vm.runInNewContext(code, browser))
  .then(() => new Promise((r) => setTimeout(r, 5)))
  .then(() => console.log(JSON.stringify({ out, els: state(), store: Object.fromEntries(store) })),
    (err) => console.log(JSON.stringify({ error: String(err && err.message), out, els: state() })));
`

// caughtError matches a caught error as each backend prints it: the
// JavaScript backend's Error ("Error: CODE: message", or an engine error
// such as "TypeError: message") and the VM's report, which this backend
// binds.
var caughtError = regexp.MustCompile(`^(?:Error: ([A-Z][A-Z0-9_]*): (.*)|\{"phase":"runtime","code":"([^"]*)","function":"main","message":"(.*)"\}|([A-Za-z]*Error: .*))$`)

// normalizePage rewrites caught errors in a page's output to CODE: message.
// The two backends bind a caught error differently (the plan's "Caught
// error text"); everything else must match exactly.
func normalizePage(t *testing.T, page string) string {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(page), &v); err != nil {
		t.Fatalf("page output is not JSON: %v\n%s", err, page)
	}
	if out, ok := v["out"].([]any); ok {
		for i, line := range out {
			s, _ := line.(string)
			if m := caughtError.FindStringSubmatch(s); m != nil {
				if m[1] != "" {
					out[i] = m[1] + ": " + m[2]
				} else if m[5] != "" {
					// An engine error, which this backend reports as a
					// RUNTIME_ERROR naming it.
					out[i] = "RUNTIME_ERROR: " + m[5]
				} else {
					var msg string
					_ = json.Unmarshal([]byte(`"`+m[4]+`"`), &msg)
					out[i] = m[3] + ": " + msg
				}
			}
		}
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// onPage builds a web_app for target (js or wasmgc) and runs it on the
// stand-in page.
func onPage(t *testing.T, path, target string) string {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	dir := t.TempDir()
	if out := run(t, compilerBin, "build", path, "--target", target, "-o", dir); out.failed {
		t.Fatalf("build (%s): %s", target, out.errMsg)
	}
	page := filepath.Join(dir, "page.js")
	if err := os.WriteFile(page, []byte(pageJS), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{page, filepath.Join(dir, "app.js")}
	if target == "wasmgc" {
		wasm := filepath.Join(dir, "app.wasm")
		if tools, err := exec.LookPath("wasm-tools"); err == nil {
			if out, err := exec.Command(tools, "validate", "--features", "all", wasm).CombinedOutput(); err != nil {
				t.Fatalf("wasm-tools rejects the module: %v\n%s", err, out)
			}
		}
		args = append(args, wasm)
	}
	got := run(t, "node", args...)
	if got.failed {
		t.Fatalf("page (%s): %s", target, got.errMsg)
	}
	return normalizePage(t, strings.TrimSpace(got.stdout))
}

// samePageAsJS runs a web_app on both backends and requires the same page.
func samePageAsJS(t *testing.T, src string) string {
	t.Helper()
	path := writeSource(t, src)
	want := onPage(t, path, "js")
	if got := onPage(t, path, "wasmgc"); got != want {
		t.Fatalf("WebAssembly page differs from the JavaScript backend's\n got %s\nwant %s", got, want)
	}
	t.Logf("page: %s", want)
	return want
}

// The browser host's synchronous operations, against the JavaScript
// backend: effects on the page, results, and errors caught and uncaught.
func TestBrowserLikeJavaScript(t *testing.T) {
	for name, body := range map[string]string{
		"form controls": `(set_value (dom_query "#field") "typed <b>")
  (print (dom_value (dom_query "#field")))
  (set_checked (dom_query "#box") true)
  (print (dom_checked (dom_query "#box")))
  (set_attr (dom_query "#field") "disabled" "")
  (remove_attr (dom_query "#field") "disabled")
  (dom_focus (dom_query "#field"))
  (dialog_open (dom_query "#dlg"))
  (dialog_close (dom_query "#dlg"))`,
		"text and html": `(set_text (dom_query "#a") "héllo 𝄞 <b>")
  (set_html (dom_query "#b") "<i>x</i>")
  (print (dom_text (dom_query "#a")))`,
		"attributes": `(set_attr (dom_query "#a") "data-motion" "calm")
  (print (dom_attr (dom_query "#a") "data-motion"))
  (print (+ "[" (+ (dom_attr (dom_query "#a") "data-missing") "]")))
  (try_let (v (dom_attr (dom_query "#a") "x],y")) (catch err (print (to_string err))) (print v))
  (set_attr (dom_query "#b") "data-n" 42)
  (set_attr (dom_query "#b") "data-f" 2.5)
  (set_attr (dom_query "#b") "data-t" true)`,
		"values of every kind": `(set_value (dom_query "#a") 7)
  (set_value (dom_query "#b") (map_get (dict) "none"))
  (set_value (dom_query "#field") 1.5)
  (print (+ (to_int (dom_value (dom_query "#a"))) 1))
  (print (+ "[" (+ (dom_value (dom_query "#b")) "]")))
  (print (dom_value (dom_query "#field")))`,
		"classes and style": `(print (toggle_class (dom_query "#a") "open"))
  (print (toggle_class (dom_query "#a") "open"))
  (set_class (dom_query "#b") "is-on" true)
  (set_class (dom_query "#b") "is-off" false)
  (set_style_var (dom_query "#b") "--progress" 0.25)
  (set_style_var (dom_query "#b") "--label" "12% (a)")
  (try_let (v (set_class (dom_query "#b") "a b" true)) (catch err (print (to_string err))) (print "set"))
  (try_let (v (set_style_var (dom_query "#b") "color" "red")) (catch err (print (to_string err))) (print "set"))
  (try_let (v (set_style_var (dom_query "#b") "--x" "url(a)")) (catch err (print (to_string err))) (print "set"))`,
		"layout": `(print (scroll_y))
  (print (viewport_height))
  (let (box (element_box (dom_query "#a"))) (do
    (print (map_get box "top"))
    (print (map_get box "height"))
    (print (+ (map_get box "height") 1))))
  (scroll_into_view (dom_query "#a"))
  (print (list_len (dom_query_all ".scene")))
  (print (list_len (dom_query_all ".none")))
  (for e (dom_query_all ".scene") (set_class e "seen" true))`,
		"media and preferences": `(print (media_matches "(prefers-reduced-motion: reduce)"))
  (print (media_matches "(min-width: 1px)"))
  (print (pref_put "motion" "calm"))
  (print (pref_get "motion"))
  (print (+ "[" (+ (pref_get "unset") "]")))
  (let (big "x") (do (while (< (str_len big) 600) (set big (+ big big))) (print (pref_put "big" big))))
  (try_let (v (pref_get "Bad Key")) (catch err (print (to_string err))) (print v))
  (print (subscription_count))`,
		"url_encode": `(print (url_encode "a b&é/?=𝄞"))
  (try_let (v (url_encode 5)) (catch err (print (to_string err))) (print v))`,
		"missing elements": `(for form (list "set_value" "dom_checked" "set_checked" "dom_focus" "dialog_open" "dialog_close" "remove_attr" "dom_attr" "dom_text" "set_text" "set_html" "set_attr" "toggle_class" "dom_value" "set_class" "set_style_var" "element_box" "scroll_into_view")
    (print form))
  (try_let (v (set_value (dom_query "#none") "x")) (catch err (print (to_string err))) (print v))
  (try_let (v (dom_checked (dom_query "#none"))) (catch err (print (to_string err))) (print v))
  (try_let (v (set_checked (dom_query "#none") true)) (catch err (print (to_string err))) (print v))
  (try_let (v (dom_focus (dom_query "#none"))) (catch err (print (to_string err))) (print v))
  (try_let (v (dialog_open (dom_query "#none"))) (catch err (print (to_string err))) (print v))
  (try_let (v (dialog_close (dom_query "#none"))) (catch err (print (to_string err))) (print v))
  (try_let (v (remove_attr (dom_query "#none") "x")) (catch err (print (to_string err))) (print v))
  (try_let (v (dom_attr (dom_query "#none") "x")) (catch err (print (to_string err))) (print v))
  (try_let (v (dom_text (dom_query "#none"))) (catch err (print (to_string err))) (print v))
  (try_let (v (set_text (dom_query "#none") "x")) (catch err (print (to_string err))) (print v))
  (try_let (v (set_html (dom_query "#none") "x")) (catch err (print (to_string err))) (print v))
  (try_let (v (set_attr (dom_query "#none") "a" "x")) (catch err (print (to_string err))) (print v))
  (try_let (v (toggle_class (dom_query "#none") "c")) (catch err (print (to_string err))) (print v))
  (try_let (v (dom_value (dom_query "#none"))) (catch err (print (to_string err))) (print v))
  (try_let (v (set_class (dom_query "#none") "c" true)) (catch err (print (to_string err))) (print v))
  (try_let (v (set_style_var (dom_query "#none") "--c" "1")) (catch err (print (to_string err))) (print v))
  (try_let (v (element_box (dom_query "#none"))) (catch err (print (to_string err))) (print v))
  (try_let (v (scroll_into_view (dom_query "#none"))) (catch err (print (to_string err))) (print v))
  (try_let (v (dom_query 5)) (catch err (print (to_string err))) (print v))
  (print (is_nil (dom_query "#none")))`,
		"uncaught host error": `(print "before")
  (set_text (dom_query "#none") "x")
  (print "after")`,
		"uncaught program error": `(print "before")
  (print (+ 1 (to_int (dom_value (dom_query "#a")))))
  (print "after")`,
		"exit": `(print "before")
  (try_let (v (call quit)) (catch err (print "caught")) (print v))
  (print "after"))
(defun quit () (do (exit 3) (return "never"))`,
		"stderr": `(stderr "warn")
  (print "done")`,
		"download refused": `(try_let (v (download_file "../x" "text/plain" "aGk=")) (catch err (print (to_string err))) (print v))
  (try_let (v (download_file "a.txt" "nope" "aGk=")) (catch err (print (to_string err))) (print v))
  (try_let (v (download_file "a.txt" "text/plain" "a")) (catch err (print (to_string err))) (print v))
  (try_let (v (download_file "a.txt" "text/plain" "aGk=")) (catch err (print (to_string err))) (print v))`,
	} {
		t.Run(name, func(t *testing.T) {
			samePageAsJS(t, "(web_app (do\n  "+body+"))")
		})
	}
}

// The fixtures are not vacuous: the page records each program's effects.
func TestBrowserPageRecordsEffects(t *testing.T) {
	got := samePageAsJS(t, `(web_app (do
  (set_text (dom_query "#a") "hi")
  (set_class (dom_query "#a") "on" true)
  (try_let (v (dom_attr (dom_query "#a") "x],y")) (catch err (print (to_string err))) (print v))
  (print (map_get (element_box (dom_query "#a")) "top"))))`)
	for _, want := range []string{`"text":"hi"`, `"classes":["on"]`, `"INVALID_ATTRIBUTE_NAME: x],y"`, `"out":[`} {
		if !strings.Contains(got, want) {
			t.Errorf("page lacks %s: %s", want, got)
		}
	}
}

// Operations the browser host does not run yet fail the build, naming the
// operation, and browser operations need a web_app.
func TestBrowserFailsClosed(t *testing.T) {
	for src, want := range map[string]string{
		`(web_app (on_event (dom_query "#a") "click" (lambda (e) (print 1))))`: `"on_event (a handler (lambda))" is not supported`,
		`(web_app (on_scroll (lambda () (print 1))))`:                          `"on_scroll (a handler (lambda))" is not supported`,
		`(web_app (storage_get "ns" "k"))`:                                     `"storage_get (an asynchronous operation)" is not supported`,
		`(web_app (image_decode "x" "image/png"))`:                             `"image_decode (an asynchronous operation)" is not supported`,
		`(cli_app (print (dom_query "#a")))`:                                   `"dom_query (a browser host operation; it needs a web_app)" is not supported`,
	} {
		out := run(t, compilerBin, "build", writeSource(t, src), "--target", "wasmgc", "-o", t.TempDir())
		if !out.failed || !strings.Contains(out.errMsg, want) || !strings.Contains(out.errMsg, "WASM_UNSUPPORTED") {
			t.Errorf("%s: want WASM_UNSUPPORTED %s, got %s", src, want, out.errMsg)
		}
	}
	for _, args := range [][]string{{"--host", "node"}, {"--host", "wasi"}} {
		out := run(t, compilerBin, append([]string{"build", writeSource(t, `(web_app (print 1))`), "--target", "wasmgc", "-o", t.TempDir()}, args...)...)
		if !out.failed || !strings.Contains(out.errMsg, "web_app builds only for the browser") {
			t.Errorf("web_app %v: want a host error, got %s", args, out.errMsg)
		}
	}
}
