package javascript_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// richtextHarness provides a minimal DOM: elements with child nodes, text
// nodes, attributes, and a recorded document.execCommand.
func richtextHarness(t *testing.T, code program, setup string) string {
	t.Helper()
	wrapper := howlNormJS + `const vm = require("node:vm");
const out = [];
const commands = [];
const selectionAtCommand = [];
class Node_ { constructor() { this.childNodes = []; this.parentNode = null; }
  appendChild(c) { c.parentNode = this; this.childNodes.push(c); return c; }
  contains(n) { for (let p = n; p; p = p.parentNode) if (p === this) return true; return false; }
  get textContent() { return this.childNodes.map((c) => c.textContent).join(""); }
  set textContent(v) { for (const c of this.childNodes) c.parentNode = null; this.childNodes = []; if (v !== "") this.appendChild(new Text_(String(v))); } }
class Text_ extends Node_ { constructor(t) { super(); this.nodeType = 3; this.data = t; } get textContent() { return this.data; } }
class Element_ extends Node_ { constructor(tag) { super(); this.nodeType = 1; this.tagName = tag.toUpperCase(); this.attrs = {}; this.listeners = {}; }
  setAttribute(n, v) { this.attrs[n] = String(v); } getAttribute(n) { return n in this.attrs ? this.attrs[n] : null; }
  addEventListener(t, f) { this.listeners[t] = f; if (t === "drop") afterAttach(this); if (t === "blur" && simulateBlurOnAttach) {
    document.activeElement = this; selection.addRange(new Range_(this, false, "selected words"));
    document.listeners.selectionchange(); document.listeners.pointerdown({ target: new Element_("button") });
    selection.removeAllRanges(); selection.addRange(new Range_(this, true, "collapsed after focus"));
    document.listeners.selectionchange(); f();
    const dialog = new Element_("input"); document.activeElement = dialog; selection.removeAllRanges();
    selection.addRange(new Range_(dialog, true, "dialog caret")); } }
  focus() { document.activeElement = this; if (!selection.rangeCount || !this.contains(selection.getRangeAt(0).commonAncestorContainer)) selection.addRange(new Range_(this, true, "focus caret")); }
  get innerHTML() { throw new Error("innerHTML must not be used"); } set innerHTML(v) { throw new Error("innerHTML must not be used"); } }
class Range_ { constructor(node, collapsed, marker) { this.commonAncestorContainer = node; this.collapsed = collapsed; this.marker = marker; }
  cloneRange() { return new Range_(this.commonAncestorContainer, this.collapsed, this.marker); } }
const selection = { ranges: [], get rangeCount() { return this.ranges.length; }, getRangeAt(i) { return this.ranges[i]; },
  removeAllRanges() { this.ranges = []; }, addRange(r) { this.ranges = [r]; } };
let simulateBlurOnAttach = false;
let afterAttach = () => {};
const document = { createElement: (t) => new Element_(t), createTextNode: (t) => new Text_(t),
  activeElement: null, listeners: {}, addEventListener(t, f) { this.listeners[t] = f; }, getSelection: () => selection,
  execCommand: (c, ui, v) => { commands.push([c, v === undefined ? null : v]); selectionAtCommand.push(selection.rangeCount ? selection.getRangeAt(0).marker : null); return true; },
  querySelector: (s) => s === "#ed" ? ed : s === "#ed2" ? ed2 : null };
const ed = new Element_("div");
const ed2 = new Element_("div");
function h(tag, ...kids) { const e = new Element_(tag); for (const k of kids) e.appendChild(typeof k === "string" ? new Text_(k) : k); return e; }
` + setup + `
const browser = { console: { log: (v) => out.push(howlNorm(v)), error: () => {} }, document };
Promise.resolve(` + code.inContext("browser") + `).then(
  () => console.log(JSON.stringify({ out, commands, selectionAtCommand, attrs: ed.attrs, listeners: Object.keys(ed.listeners).sort() })),
  (err) => console.log(JSON.stringify({ error: String(err.message), out })));`
	stdout, stderr, exit := runNode(t, wrapper)
	if exit != 0 {
		t.Fatalf("harness: %s %s", stdout, stderr)
	}
	return strings.TrimSpace(stdout)
}

func TestRichtextSetThenGetRoundTrips(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		model := `[{"type":"p","content":[{"text":"I make "},{"text":"calm","italic":true},{"text":" and <b>safe</b>","bold":true}]},` +
			`{"type":"h2","content":[{"text":"Tools"}]},{"type":"ul","items":[[{"text":"Pencil"}],[{"text":"Ink ","bold":true},{"text":"site","link":"https://example.org/"}]]},` +
			`{"type":"ol","items":[[{"text":"one\ntwo"}]]},{"type":"h3","content":[{"text":"End"}]}]`
		encodedModel, _ := json.Marshal(model)
		code := build(t, b, `(web_app (let (raw `+string(encodedModel)+`) (try_let (blocks (parse_json Any raw)) (catch err (print "bad")) (do
  (richtext_set (dom_query "#ed") blocks)
  (print (encode_json (richtext_get (dom_query "#ed"))))))))`)
		got := richtextHarness(t, code, "")
		var r struct{ Out []string }
		if err := json.Unmarshal([]byte(got), &r); err != nil || len(r.Out) != 1 {
			t.Fatalf("bad harness output %s", got)
		}
		var want, have any
		json.Unmarshal([]byte(model), &want)
		json.Unmarshal([]byte(r.Out[0]), &have)
		w, _ := json.Marshal(want)
		h, _ := json.Marshal(have)
		if string(w) != string(h) {
			t.Fatalf("round trip changed the model:\nwant %s\ngot  %s", w, h)
		}
	})
}

// Markup produced by browsers (or pasted before attach) maps onto the model:
// b/strong, i/em, a[href], div as paragraph, h1 as h2, br as a line break;
// anything else contributes only its text.
func TestRichtextGetNormalizesBrowserMarkup(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		setup := `ed.appendChild(h("div", "Hello ", h("strong", "big"), h("span", " world"), h("script", "alert(1)")));
ed.appendChild(h("h1", "Title"));
const a = h("a", "link"); a.setAttribute("href", "javascript:alert(1)"); a.setAttribute("onclick", "x");
const p = h("p", h("em", h("b", "both")), h("br"), "next ", a);
ed.appendChild(p);
ed.appendChild(new Text_("loose text"));
ed.appendChild(h("p", "  "));`
		code := build(t, b, `(web_app (print (encode_json (richtext_get (dom_query "#ed")))))`)
		got := richtextHarness(t, code, setup)
		// encode_json writes keys in sorted order, as the VM does (bugs.md #64).
		want := `[{\"content\":[{\"text\":\"Hello \"},{\"bold\":true,\"text\":\"big\"},{\"text\":\" worldalert(1)\"}],\"type\":\"p\"},` +
			`{\"content\":[{\"text\":\"Title\"}],\"type\":\"h2\"},` +
			`{\"content\":[{\"bold\":true,\"italic\":true,\"text\":\"both\"},{\"text\":\"\\nnext \"},{\"link\":\"javascript:alert(1)\",\"text\":\"link\"}],\"type\":\"p\"},` +
			`{\"content\":[{\"text\":\"loose text\"}],\"type\":\"p\"}]`
		if !strings.Contains(got, want) {
			t.Fatalf("got %s\nwant out containing %s", got, want)
		}
	})
}

func TestRichtextAttachAndCommands(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		code := build(t, b, `(web_app (let (ed (dom_query "#ed")) (do
  (richtext_attach ed "Introduction")
  (richtext_command ed "bold" "")
  (richtext_command ed "italic" "")
  (richtext_command ed "link" "https://example.org/")
  (richtext_command ed "unlink" "")
  (richtext_command ed "h2" "")
  (richtext_command ed "p" "")
  (richtext_command ed "ul" "")
  (richtext_command ed "ol" "")
  (try_let (r (richtext_command ed "insertHTML" "<img src=x>")) (catch err (print (to_string err))) (print "ran")))))`)
		got := richtextHarness(t, code, "")
		for _, want := range []string{
			`"attrs":{"contenteditable":"true","role":"textbox","aria-multiline":"true","aria-label":"Introduction"}`,
			`"listeners":["blur","drop","paste"]`,
			`["bold",null],["italic",null],["createLink","https://example.org/"],["unlink",null],["formatBlock","<h2>"],["formatBlock","<p>"],["insertUnorderedList",null],["insertOrderedList",null]`,
			"RICHTEXT_UNKNOWN_COMMAND",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("want %s in %s", want, got)
			}
		}
	})
}

func TestRichtextCommandRestoresSelectionAfterEditorBlur(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		code := build(t, b, `(web_app (let (ed (dom_query "#ed")) (do
  (richtext_attach ed "Introduction")
  (richtext_command ed "link" "https://example.org/"))))`)
		got := richtextHarness(t, code, "simulateBlurOnAttach = true;")
		if !strings.Contains(got, `"selectionAtCommand":["selected words"]`) {
			t.Fatalf("rich text command did not restore the editor selection after focus moved: %s", got)
		}
	})
}

func TestRichtextSetRejectsMalformedModels(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		for _, model := range []string{`"text"`, `[{"type":"table"}]`, `[{"type":"p","content":"x"}]`, `[{"type":"ul","items":[["x"]]}]`} {
			encodedModel, _ := json.Marshal(model)
			code := build(t, b, `(web_app (let (raw `+string(encodedModel)+`) (try_let (blocks (parse_json Any raw)) (catch err (print "bad json"))
  (try_let (r (richtext_set (dom_query "#ed") blocks)) (catch err (print (to_string err))) (print "set")))))`)
			if got := richtextHarness(t, code, ""); !strings.Contains(got, "RICHTEXT_INVALID") {
				t.Fatalf("%s: want RICHTEXT_INVALID, got %s", model, got)
			}
		}
	})
}

// Browsers nest blocks inside paragraphs while editing: Chrome's list command
// turns <p>text</p> into <p><ul><li>text</li></ul></p>. A p or div that
// holds block elements is read as a container, so the list survives.
func TestRichtextGetReadsBlocksNestedInParagraphs(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		setup := `ed.appendChild(h("p", h("ul", h("li", h("b", "Analyst")), h("li", "Two"))));
ed.appendChild(h("div", "lead ", h("h2", "Inner title"), "tail"));`
		code := build(t, b, `(web_app (print (encode_json (richtext_get (dom_query "#ed")))))`)
		got := richtextHarness(t, code, setup)
		want := `[{\"items\":[[{\"bold\":true,\"text\":\"Analyst\"}],[{\"text\":\"Two\"}]],\"type\":\"ul\"},` +
			`{\"content\":[{\"text\":\"lead \"}],\"type\":\"p\"},{\"content\":[{\"text\":\"Inner title\"}],\"type\":\"h2\"},{\"content\":[{\"text\":\"tail\"}],\"type\":\"p\"}]`
		if !strings.Contains(got, want) {
			t.Fatalf("got %s\nwant out containing %s", got, want)
		}
	})
}

// A toolbar interaction that does not blur must not poison subsequent caret
// tracking. Keyboard focus changes must preserve the newly chosen caret.
func TestRichtextCaretAfterOutsidePointerWithoutBlur(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		code := build(t, b, `(web_app (let (ed (dom_query "#ed")) (do
   (richtext_attach ed "Introduction")
   (richtext_command ed "bold" ""))))`)
		for _, event := range []string{"pointerdown", "keydown", "focusin", "empty-pointerdown"} {
			setup := `afterAttach = (el) => {
    document.activeElement = el;
    selection.addRange(new Range_(el, false, "old selection"));
    document.listeners.selectionchange();
    document.listeners.pointerdown({target: new Element_("button")});
    if (document.listeners.EVENT) document.listeners.EVENT({target: el});
    selection.addRange(new Range_(el, true, "intentional caret"));
    document.listeners.selectionchange();
    document.activeElement = new Element_("button");
    el.listeners.blur();
   };`
			trigger := event
			if event == "empty-pointerdown" {
				trigger = "pointerdown"
				setup = strings.Replace(setup, "if (document.listeners.EVENT)", "selection.removeAllRanges(); if (document.listeners.EVENT)", 1)
			}
			got := richtextHarness(t, code, strings.ReplaceAll(setup, "EVENT", trigger))
			if !strings.Contains(got, `"selectionAtCommand":["intentional caret"]`) {
				t.Fatalf("%s: restored stale selection: %s", event, got)
			}
		}
	})
}

func TestRichtextReplacementDiscardsSavedSelection(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		code := build(t, b, `(web_app (let (ed (dom_query "#ed")) (do
   (richtext_attach ed "Introduction")
   (richtext_set ed (list))
   (richtext_command ed "bold" ""))))`)
		got := richtextHarness(t, code, `afterAttach = (el) => {
   document.activeElement = el;
   selection.addRange(new Range_(el, false, "obsolete selection"));
   document.listeners.selectionchange();
   el.listeners.blur();
   document.activeElement = new Element_("button");
   selection.removeAllRanges();
  };`)
		if !strings.Contains(got, `"selectionAtCommand":["focus caret"]`) {
			t.Fatalf("content replacement reused old selection: %s", got)
		}
	})
}

func TestRichtextEditorsKeepIndependentSelections(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		code := build(t, b, `(web_app (do
   (richtext_attach (dom_query "#ed") "First")
   (richtext_attach (dom_query "#ed2") "Second")
   (richtext_command (dom_query "#ed") "bold" "")
   (richtext_command (dom_query "#ed2") "italic" "")))`)
		got := richtextHarness(t, code, `afterAttach = (el) => {
   document.activeElement = el;
   selection.addRange(new Range_(el, false, el === ed ? "first selection" : "second selection"));
   document.listeners.selectionchange();
   // Keyboard focus transfer: there is no preceding outside pointer event.
   el.listeners.blur();
   document.activeElement = new Element_("button");
   selection.removeAllRanges();
  };`)
		if !strings.Contains(got, `"selectionAtCommand":["first selection","second selection"]`) {
			t.Fatalf("editors did not retain independent keyboard-blur selections: %s", got)
		}
	})
}
