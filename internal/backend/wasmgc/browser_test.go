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
// checked, text, HTML, classes, custom properties, a box, focus, dialog
// state and event listeners; matchMedia with change listeners,
// IntersectionObserver, window scroll listeners, animation frames, scroll
// position, viewport and localStorage. After the program it clicks every
// element with a click listener (awaiting each handler), scrolls twice,
// flips the reduced-motion query, makes #a visible and runs the pending
// frames, then waits 100 ms for tasks. Storage is an in-memory IndexedDB (a put to the namespace "full"
// fails with a quota error), #files and #big are file inputs, and images
// decode and re-encode through createImageBitmap and OffscreenCanvas
// stand-ins. It prints, as JSON, what the program printed, handler rejections
// and unhandled rejections in order, the uncaught error, every element's
// state and the listeners still live. argv: the script, then app.wasm for
// the WebAssembly build.
const pageJS = `const vm = require("node:vm");
const fs = require("node:fs");
const code = fs.readFileSync(process.argv[2], "utf8");
const out = [];
process.on("unhandledRejection", (e) => out.push("unhandled:" + String(e && e.message)));
function el(id, attrs) {
  const classes = new Set();
  const e = { id, attrs: Object.assign({}, attrs), value: "", checked: false, open: false, textContent: "", innerHTML: "", props: {}, listeners: {},
    getAttribute(n) { return Object.prototype.hasOwnProperty.call(this.attrs, n) ? this.attrs[n] : null; },
    hasAttribute(n) { return Object.prototype.hasOwnProperty.call(this.attrs, n); },
    setAttribute(n, v) { this.attrs[n] = String(v); },
    removeAttribute(n) { delete this.attrs[n]; },
    addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); },
    focus() { out.push("focus:" + id); },
    showModal() { this.open = true; out.push("showModal:" + id); },
    close() { this.open = false; out.push("close:" + id); },
    getBoundingClientRect() { return { top: 120.5, height: 300, left: 0, width: 10 }; },
    closest(sel) { const m = /^\[([a-z0-9-]+)\]$/.exec(sel); return m && this.hasAttribute(m[1]) ? this : null; },
    classList: { toggle(n, on) { if (on === undefined) on = !classes.has(n); if (on) classes.add(n); else classes.delete(n); return on; }, get list() { return [...classes].sort(); } },
  };
  e.style = { setProperty(n, v) { e.props[n] = v; } };
  return e;
}
const els = { a: el("a", { "data-id": "a-1" }), b: el("b"), field: el("field"), box: el("box"), dlg: el("dlg") };
const store = new Map();
const dbs = {};
function req(fn) { const r = {}; setTimeout(() => { try { r.result = fn(); r.onsuccess && r.onsuccess({ target: r }); } catch (e) { r.error = e; r.onerror && r.onerror({ target: r }); } }, 0); return r; }
const indexedDB = {
  open(name, version) {
    const r = {};
    setTimeout(() => {
      const fresh = !dbs[name];
      const db = dbs[name] || (dbs[name] = { stores: {}, objectStoreNames: { contains(n) { return !!db.stores[n]; } } });
      db.createObjectStore = (n) => { db.stores[n] = new Map(); };
      db.close = () => {};
      db.transaction = (n, mode) => {
        const tx = { error: null };
        const st = db.stores[n];
        tx.objectStore = () => ({
          get: (k) => req(() => st.has(k) ? st.get(k) : undefined),
          put: (v, k) => { if (name === "howlframe:full") { const e = new Error("full"); e.name = "QuotaExceededError"; setTimeout(() => { tx.error = e; tx.onerror && tx.onerror({ target: tx }); }, 0); return {}; }
                           st.set(k, v); setTimeout(() => tx.oncomplete && tx.oncomplete(), 0); return {}; },
          delete: (k) => { st.delete(k); setTimeout(() => tx.oncomplete && tx.oncomplete(), 0); return {}; },
          getAllKeys: () => req(() => [...st.keys()]),
        });
        return tx;
      };
      r.result = db;
      if (fresh) r.onupgradeneeded && r.onupgradeneeded({ target: r });
      r.onsuccess && r.onsuccess({ target: r });
    }, 0);
    return r;
  },
};
const inputs = {
  files: { files: [new File([Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 1, 2])], "photo.png", { type: "image/png" }),
    new File([Buffer.from("hello")], "notes.txt", { type: "text/plain" })] },
  big: { files: [new File([Buffer.alloc(2048)], "big.png", { type: "image/png" })] },
};
async function createImageBitmap(blob) {
  const b = Buffer.from(await blob.arrayBuffer());
  if (b[0] !== 0x89) throw new Error("bad image");
  return { width: 4000, height: 3000, close() {} };
}
class OffscreenCanvas {
  constructor(w, h) { this.width = w; this.height = h; }
  getContext() { const c = this; return { drawImage(img, x, y, w, h) { c.drew = [w, h]; } }; }
  async convertToBlob(opts) { return new Blob([Buffer.from("re-encoded:" + this.width + "x" + this.height + ":" + opts.type)], { type: opts.type }); }
}
const mediaListeners = [];
let reduced = false;
const observers = [];
const windowListeners = [];
const frames = new Map();
let nextFrame = 1;
const browser = {
  console: { log: (v) => out.push(String(v)), error: (v) => out.push("stderr:" + String(v)) },
  TextEncoder, TextDecoder, atob, btoa, setTimeout, Promise, indexedDB, Blob, createImageBitmap, OffscreenCanvas,
  scrollY: 250, scrollX: 0, innerHeight: 800,
  scrollTo(opts) { out.push("scrollTo:" + JSON.stringify(opts)); },
  matchMedia: (q) => ({ get matches() { return q === "(prefers-reduced-motion: reduce)" ? reduced : false; },
    addEventListener(type, fn) { mediaListeners.push(fn); },
    removeEventListener(type, fn) { const i = mediaListeners.indexOf(fn); if (i >= 0) mediaListeners.splice(i, 1); } }),
  IntersectionObserver: class { constructor(fn) { this.fn = fn; this.targets = []; observers.push(this); }
    observe(t) { this.targets.push(t); } disconnect() { this.targets = []; observers.splice(observers.indexOf(this), 1); } },
  addEventListener(type, fn, opts) { windowListeners.push({ type, fn, passive: !!(opts && opts.passive) }); },
  removeEventListener(type, fn) { const i = windowListeners.findIndex((l) => l.type === type && l.fn === fn); if (i >= 0) windowListeners.splice(i, 1); },
  requestAnimationFrame(fn) { const id = nextFrame++; frames.set(id, fn); return id; },
  cancelAnimationFrame(id) { frames.delete(id); },
  localStorage: { getItem: (k) => store.has(k) ? store.get(k) : null, setItem: (k, v) => { if (String(v).length > 500) throw new Error("QuotaExceededError"); store.set(k, String(v)); } },
  document: {
    querySelector: (s) => els[s.replace(/^#/, "")] || inputs[s.replace(/^#/, "")] || null,
    querySelectorAll: (s) => s === ".scene" ? [els.a, els.b] : [],
  },
};
if (process.argv[3]) browser.howlFrameWasm = fs.readFileSync(process.argv[3]);
const settle = () => new Promise((r) => setTimeout(r, 5));
const state = () => Object.fromEntries(Object.entries(els).map(([k, e]) => [k,
  { attrs: e.attrs, value: e.value, checked: e.checked, open: e.open, text: e.textContent, html: e.innerHTML, classes: e.classList.list, props: e.props }]));
const live = () => ({ media: mediaListeners.length, observers: observers.length, window: windowListeners.map((l) => l.type + (l.passive ? ":passive" : "")).sort(), frames: frames.size });
(async () => {
  let error;
  try { await vm.runInNewContext(code, browser); } catch (e) { error = String(e && e.message); }
  await settle();
  for (const e of Object.values(els)) {
    for (const fn of e.listeners.click || []) {
      const event = { target: e, key: "Enter", preventDefault() { out.push("prevented:" + e.id); } };
      try { await fn(event); } catch (err) { out.push("rejected:" + String(err && err.message)); }
    }
  }
  windowListeners.filter((l) => l.type === "scroll").forEach((l) => { l.fn(); l.fn(); });
  reduced = true;
  mediaListeners.slice().forEach((fn) => fn({ matches: true }));
  await settle();
  observers.slice().forEach((o) => o.fn(o.targets.filter((x) => x === els.a).map((x) => ({ target: x, isIntersecting: true }))));
  await settle();
  const run = [...frames.values()]; frames.clear(); run.forEach((f) => f());
  await new Promise((r) => setTimeout(r, 100));
  const result = { out, els: state(), live: live(), store: Object.fromEntries(store) };
  if (error !== undefined) result.error = error;
  console.log(JSON.stringify(result));
})();
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
		"test blocks left out": `(print "page"))
  (test "t" (do (print "test ran") (assert false))`,
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
		`(web_app (on_media "(x)" (lambda (a b c) (print 1))))`: `"a lambda of more than two parameters" is not supported`,
		`(cli_app (spawn (lambda () (print 1))))`:               `"spawn" is not supported`,
		`(cli_app (print (dom_query "#a")))`:                    `"dom_query (a browser host operation: it runs in a web_app built for the browser)" is not supported`,
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

// Handlers: closures share the variables they capture with the code around
// them (a handler's set is seen outside, and later events see earlier
// ones), a variable bound in a loop is that iteration's, and handlers nest.
// A handler's uncaught error or exit rejects its own promise and nothing
// else; a spawned task runs at once. Subscriptions run their handlers and
// off stops them.
func TestBrowserHandlersLikeJavaScript(t *testing.T) {
	for name, src := range map[string]string{
		"shared variables": `(web_app (let (count 0) (do
  (on_event (dom_query "#a") "click" (lambda (e) (do
    (set count (+ count 1))
    (print (+ "a " (to_string count))))))
  (on_event (dom_query "#a") "click" (lambda (e) (do
    (set count (* count 10))
    (print (+ "a again " (to_string count))))))
  (set count 5)
  (print "registered"))))`,
		"event": `(web_app (on_event (dom_query "#a") "click" (lambda (e) (do
  (print (event_attr e "data-id"))
  (print (+ "[" (+ (event_attr e "data-none") "]")))
  (print (event_key e))
  (event_prevent e)))))`,
		"loop iterations": `(web_app (do
  (for el (list "#a" "#b") (on_event (dom_query el) "click" (lambda (e) (print el))))
  (let (i 0) (while (< i 2) (do
    (let (n i) (on_event (dom_query "#field") "click" (lambda (e) (print (+ "n " (to_string n))))))
    (set i (+ i 1)))))))`,
		"captured list and dict": `(web_app (let (seen (list) tally (dict)) (do
  (on_event (dom_query "#a") "click" (lambda (e) (do
    (append seen "a")
    (map_set tally "a" (list_len seen))
    (print (str_join seen ","))
    (print (map_get tally "a")))))
  (on_event (dom_query "#b") "click" (lambda (e) (do
    (append seen "b")
    (print (str_join seen ","))))))))`,
		"nested handlers": `(web_app (let (outer "o") (on_event (dom_query "#a") "click" (lambda (e) (let (inner "i") (do
  (print (+ outer inner))
  (on_event (dom_query "#b") "click" (lambda (e2) (do
    (set outer "O")
    (print (+ outer (+ inner (event_key e2)))))))))))))`,
		"handler in a function": `(web_app (do
  (call wire "#a" "first")
  (call wire "#b" "second"))
(defun wire (sel label) (do
  (on_event (dom_query sel) "click" (lambda (e) (do (set label (+ label "!")) (print label))))
  (return label))))`,
		"handler errors": `(web_app (do
  (on_event (dom_query "#a") "click" (lambda (e) (do (print "a runs") (print (+ 1 (to_int "x"))) (print "not reached"))))
  (on_event (dom_query "#b") "click" (lambda (e) (do (print "b runs") (set_text (dom_query "#none") "x"))))
  (on_event (dom_query "#field") "click" (lambda (e) (try_let (v (dom_text (dom_query "#none"))) (catch err (print (to_string err))) (print v))))
  (on_event (dom_query "#box") "click" (lambda (e) (do (print "box runs") (call quit))))
  (print "main done"))
(defun quit () (do (exit 4) (return "never"))))`,
		"missing element": `(web_app (do
  (try_let (v (on_event (dom_query "#none") "click" (lambda (e) (print 1)))) (catch err (print (to_string err))) (print "registered"))
  (print "continues")))`,
		"spawn": `(web_app (let (n 1) (do
  (spawn (lambda () (do (set n (+ n 1)) (print (+ "task " (to_string n))))))
  (print (+ "main " (to_string n)))
  (spawn (lambda () (print (+ 1 (to_int "x")))))
  (print "after a failing task"))))`,
		"subscriptions": `(web_app (let (subs (list)) (do
  (append subs (on_media "(prefers-reduced-motion: reduce)" (lambda (m) (print (+ "media " (to_string m))))))
  (append subs (on_visible (dom_query_all ".scene") (lambda (el v) (do (set_class el "seen" v) (print (+ "visible " (to_string v)))))))
  (append subs (on_scroll (lambda () (print (+ "scroll " (to_string (scroll_y)))))))
  (print (subscription_count))
  (on_event (dom_query "#a") "click" (lambda (e) (do
    (off (list_get subs 2))
    (off (list_get subs 2))
    (print (subscription_count))))))))`,
		"off everything": `(web_app (let (subs (list)) (do
  (append subs (on_media "(prefers-reduced-motion: reduce)" (lambda (m) (print "media"))))
  (append subs (on_visible (dom_query_all ".scene") (lambda (el v) (print "visible"))))
  (append subs (on_scroll (lambda () (print "scroll"))))
  (for s subs (off s))
  (print (subscription_count))
  (try_let (v (off "nope")) (catch err (print (to_string err))) (print v)))))`,
		"subscription handler error": `(web_app (do
  (on_media "(prefers-reduced-motion: reduce)" (lambda (m) (print (+ 1 (to_int "m")))))
  (print "subscribed")))`,
	} {
		t.Run(name, func(t *testing.T) {
			samePageAsJS(t, src)
		})
	}
}

// Asynchronous operations against the JavaScript backend, which awaits
// them: storage, files and images through their stand-ins, sleep, and
// asynchronous functions suspending inside loops, conditions, arguments,
// try_let, handlers and tasks, with errors raised after a suspension.
func TestBrowserAsyncLikeJavaScript(t *testing.T) {
	for name, src := range map[string]string{
		"storage": `(web_app (do
  (storage_put "notes" "b" "two")
  (storage_put "notes" "a" "one")
  (print (storage_get "notes" "a"))
  (print (+ "[" (+ (to_string (storage_get "notes" "missing")) "]")))
  (print (is_nil (storage_get "notes" "missing")))
  (print (str_join (storage_keys "notes") ","))
  (storage_delete "notes" "a")
  (print (str_join (storage_keys "notes") ","))
  (print (list_len (storage_keys "empty")))
  (try_let (v (storage_get "Bad Name" "k")) (catch err (print (to_string err))) (print v))
  (try_let (v (storage_put "notes" "" "x")) (catch err (print (to_string err))) (print "stored"))
  (try_let (v (storage_put "full" "k" "x")) (catch err (print (to_string err))) (print "stored"))
  (print "done")))`,
		"files": `(web_app (do
  (for f (file_read_input (dom_query "#files") 1000) (do
    (print (map_get f "name"))
    (print (map_get f "type"))
    (print (map_get f "size"))
    (print (map_get f "base64"))
    (print (sniff_media_type (map_get f "base64")))))
  (try_let (v (file_read_input (dom_query "#big") 1000)) (catch err (print (to_string err))) (print "read"))
  (try_let (v (file_read_input (dom_query "#none") 1000)) (catch err (print (to_string err))) (print "read"))))`,
		"images": `(web_app (let (png (map_get (list_get (file_read_input (dom_query "#files") 1000) 0) "base64")) (do
  (let (info (image_decode png "image/png")) (print (+ (to_string (map_get info "width")) (+ "x" (to_string (map_get info "height"))))))
  (let (out (image_encode png "image/png" 1000 "image/webp" 0.8)) (do
    (print (map_get out "width"))
    (print (map_get out "height"))
    (print (map_get out "type"))
    (print (base64_text (map_get out "base64")))))
  (try_let (v (image_decode "aGVsbG8=" "image/png")) (catch err (print (to_string err))) (print "decoded"))
  (try_let (v (image_encode png "image/png" 0 "image/webp" 0.8)) (catch err (print (to_string err))) (print "encoded")))))`,
		"async functions": `(web_app (do
  (storage_put "n" "x" "5")
  (print (call twice (call fetch_n)))
  (print (+ (call fetch_n) (call fetch_n)))
  (if (= (call fetch_n) 5) (print "five") (print "not five"))
  (let (i 0) (do (while (< i (call fetch_n)) (set i (+ i 2))) (print i)))
  (for k (call names) (print k))
  (print (call depth 3))
  (let (total 0) (do (set total (call fetch_n)) (print total))))
(defun fetch_n () int (return (to_int (storage_get "n" "x"))))
(defun twice ((v int)) int (return (* v 2)))
(defun names () (do (storage_put "n" "y" "6") (return (storage_keys "n"))))
(defun depth ((k int)) int (do
  (if (= k 0) (return (call fetch_n)))
  (sleep 1)
  (return (+ 1 (call depth (- k 1)))))))`,
		"order of arguments": `(web_app (do
  (storage_put "o" "a" "1")
  (print (str_join (list (call note "first") (storage_get "o" "a") (call note "third")) " ")))
(defun note (s) (do (print (+ "eval " s)) (return s))))`,
		"errors after a suspension": `(web_app (do
  (try_let (v (call risky)) (catch err (print (to_string err))) (print v))
  (try_let (v (call deep 2)) (catch err (print (to_string err))) (print v))
  (sleep 1)
  (print "then fails")
  (print (+ 1 (to_int (+ "y" (storage_get "e" "k"))))))
(defun risky () int (do (storage_put "e" "k" "v") (return (to_int (storage_get "e" "k")))))
(defun deep ((k int)) int (do (sleep 1) (if (= k 0) (return (to_int "x"))) (return (call deep (- k 1))))))`,
		"exit after a suspension": `(web_app (do
  (sleep 1)
  (print "before exit")
  (call quit)
  (print "after"))
(defun quit () (do (exit 2) (return "never"))))`,
		"handlers": `(web_app (let (count 0) (do
  (on_event (dom_query "#a") "click" (lambda (e) (do
    (set count (+ count 1))
    (storage_put "h" "count" (to_string count))
    (set count (+ count 10))
    (print (+ "a stored " (storage_get "h" "count")))
    (print (+ "a count " (to_string count))))))
  (on_event (dom_query "#b") "click" (lambda (e) (do
    (sleep 1)
    (print (+ "b sees " (to_string count)))
    (print (+ 1 (to_int (storage_get "h" "count")))))))
  (sleep 1)
  (set count 100)
  (print "main done"))))`,
		"captured across a suspension": `(web_app (let (x 1) (do
  (for el (list "#a" "#b") (do
    (sleep 1)
    (on_event (dom_query el) "click" (lambda (e) (print (+ el (+ " " (to_string x))))))))
  (sleep 1)
  (set x 2))))`,
		"tasks": `(web_app (do
  (spawn (lambda () (do (sleep 1) (print "task 1 woke") (storage_put "t" "k" "from task"))))
  (spawn (lambda () (do (sleep 60) (print (+ "task 2 reads " (storage_get "t" "k"))))))
  (spawn (lambda () (do (sleep 1) (print (+ 1 (to_int "x"))))))
  (print "main goes on")))`,
	} {
		t.Run(name, func(t *testing.T) {
			samePageAsJS(t, src)
		})
	}
}

// An asynchronous call where it cannot be moved ahead of its form, in a
// conditionally evaluated operand, fails the build.
func TestBrowserAsyncFailsClosed(t *testing.T) {
	for src, want := range map[string]string{
		`(web_app (print (and true (is_nil (storage_get "a" "b")))))`:                             "an asynchronous call in a branch of and",
		`(web_app (print (if true (storage_get "a" "b") "x")))`:                                   "an asynchronous call in a branch of if",
		`(web_app (try_let (v (+ "x" (storage_get "a" "b"))) (catch err (print err)) (print v)))`: "an asynchronous call inside the value of a try_let",
		`(web_app (print (dict ("a" (storage_get "a" "b")))))`:                                    "an asynchronous call inside a dict literal",
	} {
		out := run(t, compilerBin, "build", writeSource(t, src), "--target", "wasmgc", "-o", t.TempDir())
		if !out.failed || !strings.Contains(out.errMsg, want) || !strings.Contains(out.errMsg, "WASM_UNSUPPORTED") {
			t.Errorf("%s: want WASM_UNSUPPORTED %s, got %s", src, want, out.errMsg)
		}
	}
}
