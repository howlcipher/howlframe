package javascript

import (
	"encoding/json"
	"strings"
	"testing"
)

// domHarness runs code against a small DOM stand-in: elements selected by id,
// with attributes, value, checked, focus and dialog state, and a synthetic
// event whose target sits inside a [data-action] ancestor.
func domHarness(t *testing.T, code string) string {
	t.Helper()
	encoded, err := json.Marshal(code)
	if err != nil {
		t.Fatal(err)
	}
	wrapper := `const vm = require("node:vm");
const out = [];
function el(id, attrs, parent) {
  return { id, attrs: Object.assign({}, attrs), parentElement: parent || null, value: "", checked: false, focused: false, open: false,
    getAttribute(n) { return Object.prototype.hasOwnProperty.call(this.attrs, n) ? this.attrs[n] : null; },
    hasAttribute(n) { return Object.prototype.hasOwnProperty.call(this.attrs, n); },
    setAttribute(n, v) { this.attrs[n] = String(v); },
    removeAttribute(n) { delete this.attrs[n]; },
    addEventListener(type, fn) { this.handler = fn; },
    focus() { this.focused = true; out.push("focus:" + this.id); },
    showModal() { this.open = true; out.push("showModal:" + this.id); },
    close() { this.open = false; out.push("close:" + this.id); },
    closest(sel) { const m = /^\[([a-z0-9-]+)\]$/.exec(sel); for (let n = this; n; n = n.parentElement) { if (m && n.hasAttribute(m[1])) return n; } return null; } };
}
const card = el("card", { "data-action": "remove", "data-id": "s-2" });
const button = el("button", {}, card);
const field = el("field", {});
const box = el("box", {});
const dlg = el("dlg", {});
const byId = { "#field": field, "#box": box, "#dlg": dlg, "#button": button };
let prevented = false;
const event = { target: button, key: "Escape", preventDefault() { prevented = true; } };
const browser = { console: { log: (v) => out.push(String(v)), error: () => {} },
  document: { querySelector: (s) => byId[s] || null } };
Promise.resolve(vm.runInNewContext(` + string(encoded) + `, browser))
  .then(() => new Promise((r) => setTimeout(r, 5)))
  .then(() => (button.handler ? button.handler(event) : null)).then(
  () => console.log(JSON.stringify({ out, prevented, field: { value: field.value, attrs: field.attrs }, box: box.checked })),
  (err) => console.log(JSON.stringify({ error: String(err.message), out })));`
	stdout, stderr, exit := runJS(t, wrapper)
	if exit != 0 {
		t.Fatalf("harness: %s %s", stdout, stderr)
	}
	return strings.TrimSpace(stdout)
}

func TestEventDelegationReadsClosestAttribute(t *testing.T) {
	code := compileWebApp(t, `(web_app (on_event (dom_query "#button") "click" (lambda (e) (do
  (print (event_attr e "data-action"))
  (print (event_attr e "data-id"))
  (print (event_attr e "data-missing"))
  (print (event_key e))
  (event_prevent e)))))`)
	got := domHarness(t, code)
	want := `{"out":["remove","s-2","","Escape"],"prevented":true,"field":{"value":"","attrs":{}},"box":false}`
	if got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
}

func TestEventAttrRejectsSelectorInjection(t *testing.T) {
	code := compileWebApp(t, `(web_app (on_event (dom_query "#button") "click" (lambda (e) (try_let (v (event_attr e "x],body,[y")) (catch err (print (to_string err))) (print "read")))))`)
	if got := domHarness(t, code); !strings.Contains(got, "INVALID_ATTRIBUTE_NAME") {
		t.Fatalf("attribute names must be plain names: %s", got)
	}
}

func TestFormControlsAndFocus(t *testing.T) {
	code := compileWebApp(t, `(web_app (do
  (set_value (dom_query "#field") "typed <b>")
  (print (dom_value (dom_query "#field")))
  (set_checked (dom_query "#box") true)
  (print (dom_checked (dom_query "#box")))
  (set_attr (dom_query "#field") "disabled" "")
  (remove_attr (dom_query "#field") "disabled")
  (dom_focus (dom_query "#field"))
  (dialog_open (dom_query "#dlg"))
  (dialog_close (dom_query "#dlg"))))`)
	got := domHarness(t, code)
	want := `{"out":["typed <b>","true","focus:field","showModal:dlg","close:dlg"],"prevented":false,"field":{"value":"typed <b>","attrs":{}},"box":true}`
	if got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
}

func TestDomHostOperationsRejectMissingElements(t *testing.T) {
	for _, form := range []string{
		`(set_value (dom_query "#none") "x")`, `(dom_checked (dom_query "#none"))`, `(set_checked (dom_query "#none") true)`,
		`(dom_focus (dom_query "#none"))`, `(dialog_open (dom_query "#none"))`, `(remove_attr (dom_query "#none") "x")`,
		`(dom_attr (dom_query "#none") "x")`,
	} {
		code := compileWebApp(t, `(web_app (try_let (v `+form+`) (catch err (print (to_string err))) (print "ran")))`)
		if got := domHarness(t, code); !strings.Contains(got, "TYPE_ERROR") {
			t.Fatalf("%s: want TYPE_ERROR, got %s", form, got)
		}
	}
}

func TestDomAttrReadsAttributes(t *testing.T) {
	code := compileWebApp(t, `(web_app (do
  (set_attr (dom_query "#field") "data-motion" "calm")
  (print (dom_attr (dom_query "#field") "data-motion"))
  (print (+ "[" (+ (dom_attr (dom_query "#field") "data-missing") "]")))
  (try_let (v (dom_attr (dom_query "#field") "x],y")) (catch err (print (to_string err))) (print v))))`)
	got := domHarness(t, code)
	if !strings.Contains(got, `"out":["calm","[]","Error: INVALID_ATTRIBUTE_NAME: x],y"]`) {
		t.Fatalf("got %s", got)
	}
}
