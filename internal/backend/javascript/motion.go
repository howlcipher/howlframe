package javascript

import (
	"fmt"
	"strings"

	"github.com/howlcipher/howlframe/internal/ast"
)

// jsMotionHelpers maps the layout, preference and subscription-control host
// operations to their runtime helpers. Each argument is emitted as an
// expression, in order; none is asynchronous.
var jsMotionHelpers = map[string]string{
	"dom_query_all":      "howlFrameDomQueryAll",
	"set_class":          "howlFrameSetClass",
	"set_style_var":      "howlFrameSetStyleVar",
	"scroll_y":           "howlFrameScrollY",
	"viewport_height":    "howlFrameViewportHeight",
	"element_box":        "howlFrameElementBox",
	"media_matches":      "howlFrameMediaMatches",
	"pref_get":           "howlFramePrefGet",
	"pref_put":           "howlFramePrefPut",
	"off":                "howlFrameOff",
	"subscription_count": "howlFrameSubscriptionCount",
}

// jsSubscriptionHelpers maps the operations whose last argument is a lambda
// to their runtime helpers. Each returns a subscription handle for off.
var jsSubscriptionHelpers = map[string]string{
	"on_media":   "howlFrameOnMedia",
	"on_visible": "howlFrameOnVisible",
	"on_scroll":  "howlFrameOnScroll",
}

// jsNeedsMotion is set when GenerateJSCode emits one of these operations.
var jsNeedsMotion bool

func jsMotionCall(head string, node *ast.Node, reqVar string, depth int) (string, bool) {
	helper, ok := jsMotionHelpers[head]
	isSubscription := false
	if !ok {
		if helper, ok = jsSubscriptionHelpers[head]; !ok {
			return "", false
		}
		isSubscription = true
	}
	jsNeedsMotion = true
	kids := node.Children[1:]
	if isSubscription && len(kids) > 0 {
		kids = kids[:len(kids)-1]
	}
	args := make([]string, 0, len(node.Children))
	for _, child := range kids {
		args = append(args, generateJSStatementRaw(child, reqVar, depth+1))
	}
	if isSubscription {
		lambda := node.Children[len(node.Children)-1]
		params := make([]string, 0, len(lambda.Children[1].Children))
		for _, p := range lambda.Children[1].Children {
			params = append(params, sanitizeJSName(p.Value))
		}
		body := generateJSStatement(lambda.Children[2], reqVar, depth+1)
		args = append(args, fmt.Sprintf("async (%s) => {\n%s\n}", strings.Join(params, ", "), body))
	}
	return fmt.Sprintf("%s(%s)", helper, strings.Join(args, ", ")), true
}

// motionJSHelper returns the layout, preference and subscription runtime.
//
// Subscriptions (on_media, on_visible, on_scroll) return a handle; off
// removes every listener, observer and pending animation frame behind it, is
// idempotent, and stops its handler from running again. subscription_count
// reports live subscriptions, so a program (or its tests) can prove that it
// left nothing running. on_scroll runs its handler at most once per
// animation frame from passive scroll and resize listeners. Without
// IntersectionObserver every element counts as visible at once; without
// matchMedia no query matches.
//
// set_class takes a plain class name; set_style_var sets only custom
// properties ("--name"), to values of letters, digits, spaces and . , % # ( )
// + -, never url(), so a value cannot add declarations or load anything.
// pref_get and pref_put keep small strings in localStorage under
// "howlframe:" + key; where storage is unavailable, pref_get is "" and
// pref_put returns false.
func motionJSHelper() string {
	return `var howlFrameSubscriptions = new Set();
function howlFrameSubscribe() {
  var h = { howlFrameSubscription: true, live: true, dispose: function () {} };
  howlFrameSubscriptions.add(h);
  return h;
}
function howlFrameOff(h) {
  if (h === null || typeof h !== "object" || h.howlFrameSubscription !== true) {
    throw new Error("TYPE_ERROR: off expected a subscription");
  }
  if (!h.live) return;
  h.live = false;
  howlFrameSubscriptions.delete(h);
  h.dispose();
}
function howlFrameSubscriptionCount() {
  return howlFrameSubscriptions.size;
}
function howlFrameMotionElement(kind, el) {
  if (el === null || el === undefined || typeof el !== "object") {
    throw new Error("TYPE_ERROR: " + kind + " expected an element");
  }
  return el;
}
function howlFrameDomQueryAll(selector) {
  if (typeof selector !== "string") throw new Error("TYPE_ERROR: dom_query_all expected a selector string");
  return Array.prototype.slice.call(document.querySelectorAll(selector));
}
function howlFrameSetClass(el, name, on) {
  el = howlFrameMotionElement("set_class", el);
  if (typeof name !== "string" || !/^[A-Za-z_-][A-Za-z0-9_-]{0,63}$/.test(name)) {
    throw new Error("INVALID_CLASS_NAME: " + String(name).slice(0, 64));
  }
  el.classList.toggle(name, on === true);
}
function howlFrameSetStyleVar(el, name, value) {
  el = howlFrameMotionElement("set_style_var", el);
  if (typeof name !== "string" || !/^--[a-z][a-z0-9-]{0,63}$/.test(name)) {
    throw new Error("INVALID_STYLE_VAR: only custom properties (--name) can be set");
  }
  var v = typeof value === "number" && isFinite(value) ? String(value) : value;
  if (typeof v !== "string" || v.length > 200 || !/^[A-Za-z0-9 .,%#()+-]*$/.test(v) || /url\s*\(/i.test(v)) {
    throw new Error("INVALID_STYLE_VALUE: " + name);
  }
  el.style.setProperty(name, v);
}
function howlFrameScrollY() {
  return typeof scrollY === "number" ? scrollY : 0;
}
function howlFrameViewportHeight() {
  return typeof innerHeight === "number" ? innerHeight : 0;
}
function howlFrameElementBox(el) {
  var r = howlFrameMotionElement("element_box", el).getBoundingClientRect();
  return { top: r.top, height: r.height };
}
function howlFrameMediaList(q) {
  if (typeof q !== "string" || q.length > 200) throw new Error("TYPE_ERROR: expected a media query string");
  return typeof matchMedia === "function" ? matchMedia(q) : null;
}
function howlFrameMediaMatches(q) {
  var m = howlFrameMediaList(q);
  return m ? m.matches === true : false;
}
function howlFrameOnMedia(q, fn) {
  var m = howlFrameMediaList(q);
  var h = howlFrameSubscribe();
  var listener = function (e) { if (h.live) fn(e && e.matches === true); };
  if (m && typeof m.addEventListener === "function") {
    m.addEventListener("change", listener);
    h.dispose = function () { m.removeEventListener("change", listener); };
  }
  return h;
}
function howlFrameOnVisible(els, fn) {
  if (!Array.isArray(els)) throw new Error("TYPE_ERROR: on_visible expected a list of elements");
  els.forEach(function (e) { howlFrameMotionElement("on_visible", e); });
  var h = howlFrameSubscribe();
  if (typeof IntersectionObserver === "function") {
    var obs = new IntersectionObserver(function (entries) {
      entries.forEach(function (en) { if (h.live) fn(en.target, en.isIntersecting === true); });
    });
    els.forEach(function (e) { obs.observe(e); });
    h.dispose = function () { obs.disconnect(); };
  } else {
    els.forEach(function (e) { fn(e, true); });
  }
  return h;
}
function howlFrameOnScroll(fn) {
  var h = howlFrameSubscribe();
  if (typeof addEventListener !== "function") return h;
  var raf = typeof requestAnimationFrame === "function" ? requestAnimationFrame : function (f) { return setTimeout(f, 16); };
  var cancel = typeof cancelAnimationFrame === "function" ? cancelAnimationFrame : clearTimeout;
  var pending = 0;
  var run = function () { pending = 0; if (h.live) fn(); };
  var listener = function () { if (!pending && h.live) pending = raf(run); };
  addEventListener("scroll", listener, { passive: true });
  addEventListener("resize", listener, { passive: true });
  h.dispose = function () {
    removeEventListener("scroll", listener);
    removeEventListener("resize", listener);
    if (pending) { cancel(pending); pending = 0; }
  };
  return h;
}
function howlFramePrefKey(k) {
  if (typeof k !== "string" || !/^[a-z][a-z0-9-]{0,39}$/.test(k)) {
    throw new Error("PREF_INVALID_KEY: " + JSON.stringify(String(k).slice(0, 40)));
  }
  return "howlframe:" + k;
}
function howlFramePrefStore() {
  try { return typeof localStorage !== "undefined" && localStorage ? localStorage : null; } catch (e) { return null; }
}
function howlFramePrefGet(k) {
  k = howlFramePrefKey(k);
  var s = howlFramePrefStore();
  if (!s) return "";
  try { var v = s.getItem(k); return typeof v === "string" ? v : ""; } catch (e) { return ""; }
}
function howlFramePrefPut(k, v) {
  k = howlFramePrefKey(k);
  if (typeof v !== "string" || v.length > 4096) throw new Error("TYPE_ERROR: pref_put expected a string of at most 4096 characters");
  var s = howlFramePrefStore();
  if (!s) return false;
  try { s.setItem(k, v); return true; } catch (e) { return false; }
}
`
}
