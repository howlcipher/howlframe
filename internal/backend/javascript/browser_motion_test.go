package javascript

import (
	"encoding/json"
	"strings"
	"testing"
)

// motionHarness runs code against stand-ins for the browser APIs behind the
// layout, preference and subscription host operations: querySelectorAll,
// classList, style custom properties, matchMedia with change listeners,
// IntersectionObserver, window scroll listeners, requestAnimationFrame and
// localStorage. After the program, it scrolls twice (requesting frames),
// fires a media change, makes element "a" visible and then runs the pending
// frames, and reports what the program printed and which
// listeners are still registered.
func motionHarness(t *testing.T, code string, setup string) string {
	t.Helper()
	encoded, err := json.Marshal(code)
	if err != nil {
		t.Fatal(err)
	}
	wrapper := `const vm = require("node:vm");
const out = [];
function el(id) {
  const classes = new Set();
  return { id, props: {},
    classList: { toggle(n, on) { if (on) classes.add(n); else classes.delete(n); return on; }, contains: (n) => classes.has(n), get list() { return [...classes].sort(); } },
    style: { setProperty(n, v) { this.owner.props[n] = v; } },
    getBoundingClientRect() { return { top: 120.5, height: 300, left: 0, width: 10 }; },
    scrollIntoView(opts) { out.push("scrolled:" + id + ":" + JSON.stringify(opts)); } };
}
const a = el("a"), b = el("b");
a.style.owner = a; b.style.owner = b;
const html = el("html"); html.style.owner = html;
const mediaListeners = [];
let mediaMatches = false;
const observers = [];
const windowListeners = [];
const frames = new Map();
let nextFrame = 1;
let due = 0;
const store = new Map();
const browser = {
  console: { log: (v) => out.push(String(v)), error: (e) => out.push("error:" + String(e && e.message || e)) },
  setTimeout, Promise,
  scrollY: 250, scrollX: 0, innerHeight: 800,
  scrollTo(opts) { out.push("scrollTo:" + JSON.stringify(opts)); },
  document: {
    querySelector: (s) => ({ "#a": a, "#b": b, "html": html })[s] || null,
    querySelectorAll: (s) => s === ".scene" ? [a, b] : [],
  },
  matchMedia: (q) => ({ get matches() { return q === "(prefers-reduced-motion: reduce)" ? mediaMatches : false; },
    addEventListener(type, fn) { mediaListeners.push(fn); },
    removeEventListener(type, fn) { const i = mediaListeners.indexOf(fn); if (i >= 0) mediaListeners.splice(i, 1); } }),
  IntersectionObserver: class { constructor(fn) { this.fn = fn; this.targets = []; observers.push(this); }
    observe(t) { this.targets.push(t); } disconnect() { this.targets = []; observers.splice(observers.indexOf(this), 1); } },
  addEventListener(type, fn, opts) { windowListeners.push({ type, fn, passive: !!(opts && opts.passive) }); },
  removeEventListener(type, fn) { const i = windowListeners.findIndex((l) => l.type === type && l.fn === fn); if (i >= 0) windowListeners.splice(i, 1); },
  requestAnimationFrame(fn) { const id = nextFrame++; frames.set(id, fn); return id; },
  cancelAnimationFrame(id) { frames.delete(id); },
  localStorage: { getItem: (k) => store.has(k) ? store.get(k) : null, setItem: (k, v) => { if (String(v).length > 500) throw new Error("QuotaExceededError"); store.set(k, String(v)); } },
};
` + setup + `
const settle = () => new Promise((r) => setTimeout(r, 5));
Promise.resolve(vm.runInNewContext(` + string(encoded) + `, browser))
  .then(settle)
  .then(() => windowListeners.filter((l) => l.type === "scroll").forEach((l) => { l.fn(); l.fn(); }))
  .then(() => { mediaMatches = true; mediaListeners.slice().forEach((fn) => fn({ matches: true })); })
  .then(settle)
  .then(() => observers.slice().forEach((o) => o.fn(o.targets.filter((x) => x === a).map((x) => ({ target: x, isIntersecting: true })))))
  .then(settle)
  .then(() => { const run = [...frames.values()]; due = run.length; frames.clear(); run.forEach((f) => f()); })
  .then(settle)
  .then(() => console.log(JSON.stringify({ out,
      live: { media: mediaListeners.length, observers: observers.length, window: windowListeners.map((l) => l.type + (l.passive ? ":passive" : "")).sort(), frames: frames.size, due },
      a: { classes: a.classList.list, props: a.props }, store: Object.fromEntries(store) })),
    (err) => console.log(JSON.stringify({ error: String(err.message), out })));`
	stdout, stderr, exit := runJS(t, wrapper)
	if exit != 0 {
		t.Fatalf("harness: %s %s", stdout, stderr)
	}
	return strings.TrimSpace(stdout)
}

func TestLayoutHostOperations(t *testing.T) {
	code := compileWebApp(t, `(web_app (do
  (print (list_len (dom_query_all ".scene")))
  (print (list_len (dom_query_all ".none")))
  (set_class (dom_query "#a") "hs-in" true)
  (set_class (dom_query "#a") "gone" true)
  (set_class (dom_query "#a") "gone" false)
  (set_style_var (dom_query "#a") "--hs-scroll" "0.25")
  (set_style_var (dom_query "#a") "--hs-shift" 12)
  (print (scroll_y))
  (print (viewport_height))
  (print (map_get (element_box (dom_query "#a")) "top"))
  (print (map_get (element_box (dom_query "#a")) "height"))
  (print (media_matches "(prefers-reduced-motion: reduce)"))))`)
	got := motionHarness(t, code, "")
	want := `{"out":["2","0","250","800","120.5","300","false"],"live":{"media":0,"observers":0,"window":[],"frames":0,"due":0},"a":{"classes":["hs-in"],"props":{"--hs-scroll":"0.25","--hs-shift":"12"}},"store":{}}`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestLayoutHostOperationsRejectUnsafeInput(t *testing.T) {
	for form, code := range map[string]string{
		`(set_style_var (dom_query "#a") "color" "red")`:                   "INVALID_STYLE_VAR",
		`(set_style_var (dom_query "#a") "--x" "red; background: url(x)")`: "INVALID_STYLE_VALUE",
		`(set_style_var (dom_query "#a") "--x" "url(https://e.example/)")`: "INVALID_STYLE_VALUE",
		`(set_style_var (dom_query "#a") "--x" "URL (x)")`:                 "INVALID_STYLE_VALUE",
		`(set_class (dom_query "#a") "a b" true)`:                          "INVALID_CLASS_NAME",
		`(set_class (dom_query "#none") "x" true)`:                         "TYPE_ERROR",
		`(element_box (dom_query "#none"))`:                                "TYPE_ERROR",
		`(pref_put "Bad Key" "x")`:                                         "PREF_INVALID_KEY",
		`(off 5)`:                                                          "TYPE_ERROR",
	} {
		compiled := compileWebApp(t, `(web_app (try_let (v `+form+`) (catch err (print (to_string err))) (print "ran")))`)
		if got := motionHarness(t, compiled, ""); !strings.Contains(got, code) {
			t.Errorf("%s: want %s, got %s", form, code, got)
		}
	}
}

func TestPreferencesPersistSmallStrings(t *testing.T) {
	code := compileWebApp(t, `(web_app (do
  (print (pref_get "motion"))
  (print (pref_put "motion" "off"))
  (print (pref_get "motion"))
  (print (pref_put "motion" "`+strings.Repeat("x", 600)+`"))
  (print (pref_get "motion"))))`)
	got := motionHarness(t, code, "")
	if !strings.Contains(got, `"out":["","true","off","false","off"]`) || !strings.Contains(got, `"store":{"howlframe:motion":"off"}`) {
		t.Fatalf("a missing preference reads as empty, a refused write returns false and keeps the old value: %s", got)
	}
}

func TestPreferencesWithoutLocalStorageReadEmptyAndRefuseWrites(t *testing.T) {
	code := compileWebApp(t, `(web_app (do (print (pref_get "motion")) (print (pref_put "motion" "off"))))`)
	got := motionHarness(t, code, `Object.defineProperty(browser, "localStorage", { get() { throw new Error("SecurityError"); } });`)
	if !strings.Contains(got, `"out":["","false"]`) {
		t.Fatalf("got %s", got)
	}
}

func TestSubscriptionsDeliverEventsAndThrottleScrollToFrames(t *testing.T) {
	code := compileWebApp(t, `(web_app (do
  (on_media "(prefers-reduced-motion: reduce)" (lambda (m) (print (+ "media " (to_string m)))))
  (on_visible (dom_query_all ".scene") (lambda (el v) (do (set_class el "hs-in" v) (print (+ "visible " (to_string v))))))
  (on_scroll (lambda () (print "frame")))
  (print (subscription_count))))`)
	got := motionHarness(t, code, "")
	want := `{"out":["3","media true","visible true","frame"],"live":{"media":1,"observers":1,"window":["resize:passive","scroll:passive"],"frames":0,"due":1},"a":{"classes":["hs-in"],"props":{}},"store":{}}`
	if got != want {
		t.Fatalf("two scroll events in one frame run the handler once:\ngot  %s\nwant %s", got, want)
	}
}

func TestOffDisposesSubscriptionsCompletely(t *testing.T) {
	code := compileWebApp(t, `(web_app
  (let (m (on_media "(prefers-reduced-motion: reduce)" (lambda (x) (print "media"))))
    (let (v (on_visible (dom_query_all ".scene") (lambda (el x) (print "visible"))))
      (let (s (on_scroll (lambda () (print "frame"))))
        (do
          (off m)
          (off v)
          (off s)
          (off s)
          (print (subscription_count)))))))`)
	got := motionHarness(t, code, "")
	want := `{"out":["0"],"live":{"media":0,"observers":0,"window":[],"frames":0,"due":0},"a":{"classes":[],"props":{}},"store":{}}`
	if got != want {
		t.Fatalf("after off nothing is registered and no handler runs (off is idempotent):\ngot  %s\nwant %s", got, want)
	}
}

func TestOffCancelsAPendingFrame(t *testing.T) {
	// The harness scrolls after the program; here the program turns scrolling
	// off from inside the media handler, which runs before the scroll.
	code := compileWebApp(t, `(web_app
  (let (s (on_scroll (lambda () (print "frame"))))
    (on_media "(prefers-reduced-motion: reduce)" (lambda (x) (do (off s) (print (subscription_count)))))))`)
	got := motionHarness(t, code, "")
	if !strings.Contains(got, `"out":["1"]`) || !strings.Contains(got, `"window":[]`) || !strings.Contains(got, `"due":0`) {
		t.Fatalf("got %s", got)
	}
}

func TestSubscriptionsDegradeWithoutBrowserSupport(t *testing.T) {
	code := compileWebApp(t, `(web_app (do
  (print (media_matches "(prefers-reduced-motion: reduce)"))
  (on_media "(prefers-reduced-motion: reduce)" (lambda (m) (print "media")))
  (on_visible (dom_query_all ".scene") (lambda (el v) (print (+ "visible " (to_string v)))))
  (print (subscription_count))))`)
	got := motionHarness(t, code, `delete browser.matchMedia; delete browser.IntersectionObserver;`)
	// Without IntersectionObserver every element counts as visible at once;
	// without matchMedia no query matches and no change ever arrives.
	if !strings.Contains(got, `"out":["false","visible true","visible true","2"]`) {
		t.Fatalf("got %s", got)
	}
}

func TestScrollIntoViewScrollsOnlyItsOwnDocument(t *testing.T) {
	// The element's top (120.5 in the viewport, scrolled 250) goes to the top
	// of this document's viewport. Element.scrollIntoView is not used: it
	// also scrolls any page embedding this one, such as an editor around a
	// preview frame.
	code := compileWebApp(t, `(web_app (do
  (scroll_into_view (dom_query "#a"))
  (try_let (v (scroll_into_view (dom_query "#none"))) (catch err (print (to_string err))) (print "ran"))))`)
	got := motionHarness(t, code, "")
	if !strings.Contains(got, `"out":["scrollTo:{\"top\":370.5,\"left\":0,\"behavior\":\"instant\"}","Error: TYPE_ERROR: scroll_into_view expected an element"]`) || strings.Contains(got, "scrolled:") {
		t.Fatalf("got %s", got)
	}
}
