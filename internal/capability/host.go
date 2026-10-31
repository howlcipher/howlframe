package capability

import "sort"

// HostKind is the kind of a value passed to or returned by a browser host
// operation.
type HostKind string

const (
	HostNone         HostKind = ""             // no result
	HostText         HostKind = "text"         // a string
	HostTextOrNil    HostKind = "text?"        // a string, or nil when absent
	HostNumber       HostKind = "number"       // an int or a float
	HostInt          HostKind = "int"          // an integer
	HostBool         HostKind = "bool"         // true or false
	HostValue        HostKind = "value"        // any HowlFrame value
	HostList         HostKind = "list"         // a list of values
	HostDict         HostKind = "dict"         // a dict
	HostElement      HostKind = "element"      // a page element
	HostElementOrNil HostKind = "element?"     // a page element, or nil when none matches
	HostElements     HostKind = "elements"     // a list of page elements
	HostEvent        HostKind = "event"        // the event a handler receives
	HostSubscription HostKind = "subscription" // a handle for off
	HostLambda       HostKind = "lambda"       // a (lambda (params) body) form, not a value
)

// HostParam is one parameter of a host operation. Name is the word its usage
// shows; Lambda lists the parameters a lambda argument receives.
type HostParam struct {
	Name   string
	Kind   HostKind
	Lambda []string
}

// HostOperation is one browser host operation: an effect a web_app asks of
// the page it runs in. HostOperations is the one description of them, shared
// by the checker (arity and usage), the JavaScript backend (a runtime helper
// each) and the WebAssembly backend (a typed import each, and the loader
// generated from this table).
//
// Async operations complete later: the JavaScript backend awaits them and
// the WebAssembly backend suspends the calling code until the host resumes
// it. Capability is what the program must be granted to reach the
// operation; None means it touches only the page and its origin-scoped
// state.
type HostOperation struct {
	Name       string
	Params     []HostParam
	Result     HostKind
	Capability Capability
	Async      bool
}

// Usage is the operation's usage text, as checker errors show it.
func (op HostOperation) Usage() string {
	s := "(" + op.Name
	for _, p := range op.Params {
		s += " " + p.Name
	}
	return s + ")"
}

// LambdaLast reports whether the operation's last argument is a lambda.
func (op HostOperation) LambdaLast() bool {
	return len(op.Params) > 0 && op.Params[len(op.Params)-1].Kind == HostLambda
}

func hp(name string, kind HostKind) HostParam { return HostParam{Name: name, Kind: kind} }

func hText(name string) HostParam { return hp(name, HostText) }

func hNum(name string) HostParam { return hp(name, HostNumber) }

func hlambda(params ...string) HostParam {
	return HostParam{Name: "lambda", Kind: HostLambda, Lambda: params}
}

var (
	hEl    = hp("el", HostElement)
	hEvent = hp("event", HostEvent)
)

// hostOperations is kept sorted by name (TestHostOperationsSorted).
var hostOperations = []HostOperation{
	{Name: "base64_text", Params: []HostParam{hText("base64")}, Result: HostText},
	{Name: "dialog_close", Params: []HostParam{hEl}},
	{Name: "dialog_open", Params: []HostParam{hEl}},
	{Name: "dom_attr", Params: []HostParam{hEl, hText("name")}, Result: HostText},
	{Name: "dom_checked", Params: []HostParam{hEl}, Result: HostBool},
	{Name: "dom_focus", Params: []HostParam{hEl}},
	{Name: "dom_query", Params: []HostParam{hText("selector")}, Result: HostElementOrNil},
	{Name: "dom_query_all", Params: []HostParam{hText("selector")}, Result: HostElements},
	{Name: "dom_text", Params: []HostParam{hEl}, Result: HostText},
	{Name: "dom_value", Params: []HostParam{hEl}, Result: HostText},
	{Name: "download_file", Params: []HostParam{hText("name"), hText("mime"), hText("base64")}},
	{Name: "element_box", Params: []HostParam{hEl}, Result: HostDict},
	{Name: "event_attr", Params: []HostParam{hEvent, hText("name")}, Result: HostText},
	{Name: "event_key", Params: []HostParam{hEvent}, Result: HostText},
	{Name: "event_prevent", Params: []HostParam{hEvent}},
	{Name: "file_read_input", Params: []HostParam{hEl, hNum("max_bytes")}, Result: HostList, Async: true},
	{Name: "image_decode", Params: []HostParam{hText("base64"), hText("mime")}, Result: HostDict, Async: true},
	{Name: "image_encode", Params: []HostParam{hText("base64"), hText("mime"), hNum("max_side"), hText("out_mime"), hNum("quality")}, Result: HostDict, Async: true},
	{Name: "media_matches", Params: []HostParam{hText("query")}, Result: HostBool},
	{Name: "off", Params: []HostParam{hp("subscription", HostSubscription)}},
	{Name: "on_event", Params: []HostParam{hEl, hText("event"), hlambda("event")}},
	{Name: "on_media", Params: []HostParam{hText("query"), hlambda("matches")}, Result: HostSubscription},
	{Name: "on_scroll", Params: []HostParam{hlambda()}, Result: HostSubscription},
	{Name: "on_visible", Params: []HostParam{hp("elements", HostElements), hlambda("el", "visible")}, Result: HostSubscription},
	{Name: "pref_get", Params: []HostParam{hText("key")}, Result: HostText},
	{Name: "pref_put", Params: []HostParam{hText("key"), hText("value")}, Result: HostBool},
	{Name: "remove_attr", Params: []HostParam{hEl, hText("name")}},
	{Name: "richtext_attach", Params: []HostParam{hEl, hText("label")}},
	{Name: "richtext_command", Params: []HostParam{hEl, hText("command"), hp("value", HostValue)}},
	{Name: "richtext_get", Params: []HostParam{hEl}, Result: HostList},
	{Name: "richtext_set", Params: []HostParam{hEl, hp("blocks", HostList)}},
	{Name: "scroll_into_view", Params: []HostParam{hEl}},
	{Name: "scroll_y", Result: HostNumber},
	{Name: "set_attr", Params: []HostParam{hEl, hText("name"), hp("val", HostValue)}},
	{Name: "set_checked", Params: []HostParam{hEl, hp("bool", HostBool)}},
	{Name: "set_class", Params: []HostParam{hEl, hText("name"), hp("on", HostBool)}},
	{Name: "set_html", Params: []HostParam{hEl, hp("val", HostValue)}},
	{Name: "set_style_var", Params: []HostParam{hEl, hText("name"), hp("value", HostValue)}},
	{Name: "set_text", Params: []HostParam{hEl, hp("val", HostValue)}},
	{Name: "set_value", Params: []HostParam{hEl, hp("value", HostValue)}},
	{Name: "sniff_media_type", Params: []HostParam{hText("base64")}, Result: HostText},
	{Name: "storage_delete", Params: []HostParam{hText("namespace"), hText("key")}, Async: true},
	{Name: "storage_get", Params: []HostParam{hText("namespace"), hText("key")}, Result: HostTextOrNil, Async: true},
	{Name: "storage_keys", Params: []HostParam{hText("namespace")}, Result: HostList, Async: true},
	{Name: "storage_put", Params: []HostParam{hText("namespace"), hText("key"), hText("value")}, Async: true},
	{Name: "subscription_count", Result: HostInt},
	{Name: "toggle_class", Params: []HostParam{hEl, hText("class")}, Result: HostBool},
	{Name: "url_encode", Params: []HostParam{hText("string")}, Result: HostText},
	{Name: "viewport_height", Result: HostNumber},
	{Name: "zip_build", Params: []HostParam{hp("entries", HostList)}, Result: HostText},
	{Name: "zip_read", Params: []HostParam{hText("base64"), hNum("max_entries"), hNum("max_bytes")}, Result: HostList, Async: true},
}

var hostIndex = func() map[string]HostOperation {
	m := make(map[string]HostOperation, len(hostOperations))
	for _, op := range hostOperations {
		m[op.Name] = op
	}
	return m
}()

// HostOperations returns every browser host operation, sorted by name.
func HostOperations() []HostOperation {
	return append([]HostOperation(nil), hostOperations...)
}

// LookupHostOperation returns the browser host operation named name.
func LookupHostOperation(name string) (HostOperation, bool) {
	op, ok := hostIndex[name]
	return op, ok
}

// HostOperationNames returns the names of every browser host operation,
// sorted.
func HostOperationNames() []string {
	names := make([]string, 0, len(hostOperations))
	for _, op := range hostOperations {
		names = append(names, op.Name)
	}
	sort.Strings(names)
	return names
}
