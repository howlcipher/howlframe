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
class Node_ { constructor() { this.childNodes = []; this.parentNode = null; }
  appendChild(c) { c.parentNode = this; this.childNodes.push(c); return c; }
  get textContent() { return this.childNodes.map((c) => c.textContent).join(""); }
  set textContent(v) { this.childNodes = []; if (v !== "") this.appendChild(new Text_(String(v))); } }
class Text_ extends Node_ { constructor(t) { super(); this.nodeType = 3; this.data = t; } get textContent() { return this.data; } }
class Element_ extends Node_ { constructor(tag) { super(); this.nodeType = 1; this.tagName = tag.toUpperCase(); this.attrs = {}; this.listeners = {}; }
  setAttribute(n, v) { this.attrs[n] = String(v); } getAttribute(n) { return n in this.attrs ? this.attrs[n] : null; }
  addEventListener(t, f) { this.listeners[t] = f; } focus() {}
  get innerHTML() { throw new Error("innerHTML must not be used"); } set innerHTML(v) { throw new Error("innerHTML must not be used"); } }
const document = { createElement: (t) => new Element_(t), createTextNode: (t) => new Text_(t),
  execCommand: (c, ui, v) => { commands.push([c, v === undefined ? null : v]); return true; },
  querySelector: (s) => s === "#ed" ? ed : null };
const ed = new Element_("div");
function h(tag, ...kids) { const e = new Element_(tag); for (const k of kids) e.appendChild(typeof k === "string" ? new Text_(k) : k); return e; }
` + setup + `
const browser = { console: { log: (v) => out.push(howlNorm(v)), error: () => {} }, document };
Promise.resolve(` + code.inContext("browser") + `).then(
  () => console.log(JSON.stringify({ out, commands, attrs: ed.attrs, listeners: Object.keys(ed.listeners).sort() })),
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
			`"listeners":["drop","paste"]`,
			`["bold",null],["italic",null],["createLink","https://example.org/"],["unlink",null],["formatBlock","<h2>"],["formatBlock","<p>"],["insertUnorderedList",null],["insertOrderedList",null]`,
			"RICHTEXT_UNKNOWN_COMMAND",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("want %s in %s", want, got)
			}
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
