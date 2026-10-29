package javascript

import (
	"fmt"
	"strings"

	"github.com/howlcipher/howlframe/internal/ast"
)

// jsDOMHelpers maps the DOM and event host operations to their runtime
// helpers. Each argument is emitted as an expression, in order.
var jsDOMHelpers = map[string]string{
	"event_attr":    "howlFrameEventAttr",
	"event_key":     "howlFrameEventKey",
	"event_prevent": "howlFrameEventPrevent",
	"set_value":     "howlFrameSetValue",
	"dom_checked":   "howlFrameDomChecked",
	"set_checked":   "howlFrameSetChecked",
	"dom_focus":     "howlFrameDomFocus",
	"remove_attr":   "howlFrameRemoveAttr",
	"dialog_open":   "howlFrameDialogOpen",
	"dialog_close":  "howlFrameDialogClose",
}

// jsNeedsDOM is set when GenerateJSCode emits a DOM or event host operation.
var jsNeedsDOM bool

func jsDOMCall(head string, node *ast.Node, reqVar string, depth int) (string, bool) {
	helper, ok := jsDOMHelpers[head]
	if !ok {
		return "", false
	}
	jsNeedsDOM = true
	args := make([]string, 0, len(node.Children)-1)
	for _, child := range node.Children[1:] {
		args = append(args, generateJSStatementRaw(child, reqVar, depth+1))
	}
	return fmt.Sprintf("%s(%s)", helper, strings.Join(args, ", ")), true
}

// domJSHelper returns the DOM and event runtime. event_attr reads an
// attribute from the event target or its nearest ancestor that has it, which
// is what delegated handlers need; the name must be a plain attribute name,
// so it cannot alter the selector. Operations on a missing element fail with
// TYPE_ERROR instead of doing nothing.
func domJSHelper() string {
	return `function howlFrameElement(kind, el) {
  if (el === null || el === undefined || typeof el !== "object") {
    throw new Error("TYPE_ERROR: " + kind + " expected an element");
  }
  return el;
}
function howlFrameAttrName(name) {
  if (typeof name !== "string" || !/^[a-z][a-z0-9-]*$/.test(name)) {
    throw new Error("INVALID_ATTRIBUTE_NAME: " + String(name));
  }
  return name;
}
function howlFrameEventAttr(e, name) {
  name = howlFrameAttrName(name);
  var t = e && e.target;
  if (!t || typeof t.closest !== "function") return "";
  var found = t.closest("[" + name + "]");
  if (!found) return "";
  var v = found.getAttribute(name);
  return v === null ? "" : v;
}
function howlFrameEventKey(e) {
  return e && typeof e.key === "string" ? e.key : "";
}
function howlFrameEventPrevent(e) {
  if (e && typeof e.preventDefault === "function") e.preventDefault();
}
function howlFrameSetValue(el, v) {
  howlFrameElement("set_value", el).value = v === null || v === undefined ? "" : String(v);
}
function howlFrameDomChecked(el) {
  return howlFrameElement("dom_checked", el).checked === true;
}
function howlFrameSetChecked(el, b) {
  howlFrameElement("set_checked", el).checked = b === true;
}
function howlFrameDomFocus(el) {
  howlFrameElement("dom_focus", el).focus();
}
function howlFrameRemoveAttr(el, name) {
  howlFrameElement("remove_attr", el).removeAttribute(howlFrameAttrName(name));
}
function howlFrameDialogOpen(el) {
  el = howlFrameElement("dialog_open", el);
  if (!el.open) el.showModal();
}
function howlFrameDialogClose(el) {
  el = howlFrameElement("dialog_close", el);
  if (el.open) el.close();
}
`
}
