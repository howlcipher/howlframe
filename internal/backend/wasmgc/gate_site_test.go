package wasmgc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// sitePageJS is a stand-in for a page HowlScene HF publishes: the root
// element with its motion preset, the body, two opening scenes
// (.hs-hero), three chapters ([data-scene]) with their links
// (.hs-chapters a), two revealed sections ([data-reveal]) and the motion
// toggle, each with a box. After the site runtime starts it scrolls (the
// chapters' boxes move up), runs the frames, makes everything visible,
// clicks the toggle twice (pausing and resuming motion), then turns on
// reduced motion. It prints, as JSON, the program's output, every
// element's classes, attributes and custom properties, the preferences and
// the listeners still live. argv: the script, then app.wasm for the
// WebAssembly build.
const sitePageJS = `const vm = require("node:vm");
const fs = require("node:fs");
const code = fs.readFileSync(process.argv[2], "utf8");
const out = [];
process.on("unhandledRejection", (e) => out.push("unhandled:" + String(e && e.message)));
let scroll = 0;
function el(id, tag, attrs, top, height) {
  const classes = new Set();
  const e = { id, tagName: tag.toUpperCase(), attrs: Object.assign({}, attrs), props: {}, listeners: {}, top, height,
    getAttribute(n) { return Object.prototype.hasOwnProperty.call(this.attrs, n) ? this.attrs[n] : null; },
    hasAttribute(n) { return Object.prototype.hasOwnProperty.call(this.attrs, n); },
    setAttribute(n, v) { this.attrs[n] = String(v); },
    removeAttribute(n) { delete this.attrs[n]; },
    addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); },
    getBoundingClientRect() { return { top: this.top - scroll, height: this.height, left: 0, width: 800 }; },
    classList: { toggle(n, on) { if (on === undefined) on = !classes.has(n); if (on) classes.add(n); else classes.delete(n); return on; }, get list() { return [...classes].sort(); } },
  };
  e.style = { setProperty(n, v) { e.props[n] = String(v); } };
  return e;
}
const root = el("root", "html", { "data-motion": "cinematic" }, 0, 4000);
const body = el("body", "body", {}, 0, 4000);
const heroes = [el("hero1", "section", {}, 0, 800), el("hero2", "section", {}, 2400, 800)];
const chapters = [el("ch1", "section", { "data-scene": "1", id: "intro" }, 0, 1200), el("ch2", "section", { "data-scene": "2", id: "middle" }, 1200, 1200), el("ch3", "section", { "data-scene": "3", id: "end" }, 2400, 1600)];
const links = [el("l1", "a", { href: "#intro" }, 0, 20), el("l2", "a", { href: "#middle" }, 0, 20), el("l3", "a", { href: "#end" }, 0, 20)];
const reveals = [el("r1", "div", { "data-reveal": "" }, 900, 200), el("r2", "div", { "data-reveal": "" }, 3000, 200)];
const toggle = el("toggle", "button", { "data-motion-toggle": "", hidden: "" }, 10, 30);
const all = [root, body, ...heroes, ...chapters, ...links, ...reveals, toggle];
const store = new Map();
const mediaListeners = [];
let reduced = false;
const observers = [];
const windowListeners = [];
const frames = new Map();
let nextFrame = 1;
const browser = {
  console: { log: (v) => out.push(String(v)), error: (v) => out.push("stderr:" + String(v)) },
  TextEncoder, TextDecoder, setTimeout, Promise,
  get scrollY() { return scroll; }, scrollX: 0, innerHeight: 800,
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
  localStorage: { getItem: (k) => store.has(k) ? store.get(k) : null, setItem: (k, v) => { store.set(k, String(v)); } },
  document: {
    querySelector: (s) => ({ "html": root, "body": body, "[data-motion-toggle]": toggle })[s] || all.find((e) => "#" + e.attrs.id === s) || null,
    querySelectorAll: (s) => ({ ".hs-hero": heroes, "[data-scene]": chapters, ".hs-chapters a": links, "[data-reveal]": reveals })[s] || [],
  },
};
if (process.argv[3]) browser.howlFrameWasm = fs.readFileSync(process.argv[3]);
const settle = () => new Promise((r) => setTimeout(r, 5));
const frame = () => { const run = [...frames.values()]; frames.clear(); run.forEach((f) => f()); };
const scrollTo = (y) => { scroll = y; windowListeners.filter((l) => l.type === "scroll").forEach((l) => l.fn()); frame(); };
const state = () => Object.fromEntries(all.map((e) => [e.id, { classes: e.classList.list, attrs: { ...e.attrs }, props: { ...e.props } }]));
const live = () => ({ media: mediaListeners.length, observers: observers.length, window: windowListeners.map((l) => l.type).sort(), frames: frames.size });
(async () => {
  let error;
  try { await vm.runInNewContext(code, browser); } catch (e) { error = String(e && e.message); }
  await settle();
  const steps = [];
  const snap = (name) => steps.push({ name, els: state(), live: live() });
  snap("loaded");
  observers.slice().forEach((o) => o.fn(o.targets.map((x) => ({ target: x, isIntersecting: x.top < 1600 }))));
  for (const y of [300, 1300, 2600]) scrollTo(y);
  await settle();
  snap("scrolled");
  for (const fn of toggle.listeners.click || []) await fn({ target: toggle, key: "", preventDefault() {} });
  await settle();
  snap("paused");
  for (const fn of toggle.listeners.click || []) await fn({ target: toggle, key: "", preventDefault() {} });
  await settle();
  scrollTo(500);
  // The JavaScript backend's handlers await every call, so they finish in
  // later microtasks: let them, as before every snapshot.
  await settle();
  snap("resumed");
  reduced = true;
  mediaListeners.slice().forEach((fn) => fn({ matches: true }));
  await settle();
  snap("reduced motion");
  const result = { out, steps, store: Object.fromEntries(store) };
  if (error !== undefined) result.error = error;
  console.log(JSON.stringify(result));
})();
`

// TestHowlSceneSiteGate runs HowlScene HF's published-site runtime
// (src/site/site.howl) built with --target js and --target wasmgc on a
// stand-in published page, and requires the same page after every step:
// reveal on visibility, scene progress and the active chapter on scroll, the
// motion toggle and its preference, and reduced motion stopping every
// effect. It needs a checkout of HowlScene HF, named by HOWLSCENE_HF.
func TestHowlSceneSiteGate(t *testing.T) {
	hs := os.Getenv("HOWLSCENE_HF")
	if hs == "" {
		t.Skip("set HOWLSCENE_HF to a HowlScene HF checkout to run the site runtime gate")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	site := filepath.Join(hs, "src", "site", "site.howl")
	page := filepath.Join(t.TempDir(), "site-page.js")
	if err := os.WriteFile(page, []byte(sitePageJS), 0o644); err != nil {
		t.Fatal(err)
	}
	pages := map[string]string{}
	for _, target := range []string{"js", "wasmgc"} {
		dir := t.TempDir()
		if out := run(t, compilerBin, "build", site, "--target", target, "-o", dir); out.failed {
			t.Fatalf("build (%s): %s", target, out.errMsg)
		}
		args := []string{page, filepath.Join(dir, "app.js")}
		if target == "wasmgc" {
			args = append(args, filepath.Join(dir, "app.wasm"))
		}
		got := run(t, "node", args...)
		if got.failed {
			t.Fatalf("page (%s): %s", target, got.errMsg)
		}
		pages[target] = strings.TrimSpace(got.stdout)
	}
	if pages["wasmgc"] != pages["js"] {
		t.Fatalf("the site runtime's page differs on WebAssembly\n got %s\nwant %s", pages["wasmgc"], pages["js"])
	}
	// Not vacuous: the runtime ran and changed the page at every step.
	for _, want := range []string{`"hs-js"`, `"hs-motion"`, `"hs-in"`, `"hs-active"`, `"--hs-scroll"`, `"aria-current":"true"`, `"aria-pressed":"true"`, `"howlframe:howlscene-motion":"on"`} {
		if !strings.Contains(pages["js"], want) {
			t.Errorf("the page never shows %s: %s", want, pages["js"])
		}
	}
	t.Logf("site page: %s", pages["js"])
}
