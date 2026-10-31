package javascript

import (
	"fmt"
	"strings"
)

// urlEncodeJS is url_encode's body, which this backend emits inline.
const urlEncodeJS = `if(typeof v !== "string"){throw new Error("TYPE_ERROR: url_encode expected string");}return encodeURIComponent(v);`

// HostRuntime returns, for browser host operations
// (capability.HostOperations), the runtime function that implements each
// and the runtime source that defines them and what they depend on. The
// WebAssembly backend's generated browser loader calls these functions, so
// both backends run the same host code and report the same errors. An
// operation with a lambda argument is called with a JavaScript function in
// its place.
func HostRuntime(ops []string) (map[string]string, string, error) {
	helpers := map[string]string{}
	var dom, richtext, io, motion, archive, urlEncode, fetch bool
	for _, op := range ops {
		var name string
		switch {
		case jsDOMHelpers[op] != "":
			name, dom = jsDOMHelpers[op], true
			richtext = richtext || strings.HasPrefix(op, "richtext_")
		case jsBrowserIOHelpers[op] != "":
			name, io, archive = jsBrowserIOHelpers[op], true, true
		case jsMotionHelpers[op] != "":
			name, motion = jsMotionHelpers[op], true
		case jsSubscriptionHelpers[op] != "":
			// Called with the operation's arguments and then a handler.
			name, motion = jsSubscriptionHelpers[op], true
		case op == "dom_text":
			name, archive = "howlFrameDomText", true
		case op == "download_file":
			name, archive = "howlFrameDownloadFile", true
		case op == "zip_build":
			name, archive = "howlFrameZipBuild", true
		case op == "on_event":
			// Called with the element, the event type and a handler.
			name, dom = "howlFrameOnEvent", true
		case op == "fetch":
			name, fetch = "howlFrameFetch", true
		case op == "url_encode":
			name, urlEncode = "howlFrameUrlEncode", true
		default:
			return nil, "", fmt.Errorf("browser host operation %q has no runtime function", op)
		}
		helpers[op] = name
	}
	var b strings.Builder
	if archive {
		b.WriteString(archiveJSHelper())
	}
	if dom || richtext {
		b.WriteString(domJSHelper())
	}
	if richtext {
		b.WriteString(richtextJSHelper())
	}
	if motion {
		b.WriteString(motionJSHelper())
	}
	if io {
		b.WriteString(browserIOJSHelper())
	}
	if fetch {
		b.WriteString(fetchJSHelper())
	}
	if urlEncode {
		b.WriteString("function howlFrameUrlEncode(v) {" + urlEncodeJS + "}\n")
	}
	return helpers, b.String(), nil
}

// OutputRuntime returns the runtime of stderr and exit in a browser page:
// howlFrameStderr(text) and howlFrameExit(code, where), which throws the
// error a try_let passes on.
func OutputRuntime() string { return exitJSHelper(false) }
