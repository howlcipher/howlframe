package javascript

import (
	"fmt"
	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/ir"
	"net/netip"
	"strconv"
	"strings"
)

func BinOpJSToken(head string) string {
	switch head {
	case "and":
		return "&&"
	case "or":
		return "||"
	case "=", "==":
		return "==="
	case "!=":
		return "!=="
	default:
		return head
	}
}

func flattenModulesJS(nodes []*ast.Node) []*ast.Node {
	var result []*ast.Node
	for _, node := range nodes {
		if node.Type == "List" && len(node.Children) > 0 {
			head := node.Children[0].Value
			if head == "module" {
				result = append(result, flattenModulesJS(node.Children[2:])...)
				continue
			}
			if head == "export" && len(node.Children) == 2 {
				result = append(result, flattenModulesJS([]*ast.Node{node.Children[1]})...)
				continue
			}
		}
		result = append(result, node)
	}
	return result
}

// jsNeedsRequestRead is set when GenerateJSCode emits req_query, req_header,
// or req_path, so the helper is prepended once. It is reset on every call.
var jsNeedsRequestRead bool

// jsNeedsHTMLEscape is set when GenerateJSCode emits html_escape or
// attr_escape. Both share one helper. It is reset on every call.
var jsNeedsHTMLEscape bool

// jsNeedsEncodeJSON is set when GenerateJSCode emits encode_json, which
// writes JSON as the VM's Go encoding/json does.
var jsNeedsEncodeJSON bool

// jsNeedsEq is set when GenerateJSCode emits =, == or !=, which follow the
// VM's equality through howlFrameEq.
var jsNeedsEq bool

// jsNeedsArchive is set when GenerateJSCode emits dom_text, zip_build, or
// download_file, so the browser archive helpers are prepended.
var jsNeedsArchive bool

// jsNeedsExit is set when GenerateJSCode emits stderr or exit (assert lowers
// to both).
var jsNeedsExit bool

// jsNeedsCollection is set when GenerateJSCode emits a dict or list op
// that must fail closed. map_keys keeps its own guard. The flag is reset
// on every call.
var jsNeedsCollection bool

// jsNeedsMapKeyOrder is set when GenerateJSCode emits map_keys. The
// comparator orders keys by UTF-8 bytes. It is reset on every call.
var jsNeedsMapKeyOrder bool

// jsNeedsEnv is set when GenerateJSCode emits env. The helper checks the
// runner grant before reading the requested variable. It is reset on every call.
var jsNeedsEnv bool

// jsNeedsExec is set when GenerateJSCode emits exec. The helper checks the
// runner grant before spawning a process. It is reset on every call.
var jsNeedsExec bool

// jsNeedsReadFile is set when GenerateJSCode emits read_file. The helper
// checks the runner grant before any filesystem read. It is reset on every call.
var jsNeedsReadFile bool

// jsNeedsFetch is set when GenerateJSCode emits fetch. The helper checks
// the runner grant before any HTTP request. It is reset on every call.
var jsNeedsFetch bool

// jsNeedsWriteFile is set when GenerateJSCode emits write_file. The helper
// checks the runner grant before any filesystem write. It is reset on every call.
var jsNeedsWriteFile bool

// jsNeedsMkdir is set when GenerateJSCode emits mkdir. The helper checks
// the runner grant before any directory creation. It is reset on every call.
var jsNeedsMkdir bool

// jsNumeric is set when GenerateJSCode emits a numeric operation that must not
// inherit host-language semantics (division, checked conversion). The helper is
// prepended once. It is reset on every call.
var jsNumeric bool

func collectionJSHelper() string {
	// Dicts are plain objects. Lists are arrays. A missing map_get key is
	// null (rendered as "" by print/to_string). list_get of an in-range element returns that element. An
	// out-of-range index is "". A wrong receiver is TYPE_ERROR. A list_get
	// index that is not a whole number is TYPE_ERROR, matching Atoi.
	return `function howlFrameValueKind(v) {
  if (v === null) return "null";
  if (Array.isArray(v)) return "list";
  return typeof v;
}
function howlFrameIsDict(v) {
  return v !== null && typeof v === "object" && !Array.isArray(v);
}
function howlFrameMapGet(dict, key) {
  if (!howlFrameIsDict(dict)) {
    throw new Error("TYPE_ERROR: map_get expected dict, got " + howlFrameValueKind(dict));
  }
  return Object.prototype.hasOwnProperty.call(dict, key) ? dict[key] : null;
}
function howlFrameMapSet(dict, key, val) {
  if (!howlFrameIsDict(dict)) {
    throw new Error("TYPE_ERROR: map_set expected dict, got " + howlFrameValueKind(dict));
  }
  if (key === "__proto__") {
    // Assignment would replace the prototype; a dict key is data.
    Object.defineProperty(dict, key, { value: val, writable: true, enumerable: true, configurable: true });
  } else {
    dict[key] = val;
  }
}
function howlFrameMapDelete(dict, key) {
  if (!howlFrameIsDict(dict)) {
    throw new Error("TYPE_ERROR: map_delete expected dict, got " + howlFrameValueKind(dict));
  }
  delete dict[key];
}
function howlFrameAppend(list, item) {
  if (!Array.isArray(list)) {
    throw new Error("TYPE_ERROR: append expected list, got " + howlFrameValueKind(list));
  }
  // Lists are values: append returns a copy, matching the VM.
  var out = list.slice();
  out.push(item);
  return out;
}
function howlFrameListIndex(idx) {
  if (typeof idx === "number" && Number.isInteger(idx)) return idx;
  if (typeof idx === "string" && /^[+-]?\d+$/.test(idx.trim())) return parseInt(idx.trim(), 10);
  throw new Error("TYPE_ERROR: list_get index must be a number, got " + howlFrameValueKind(idx));
}
function howlFrameListGet(list, idx) {
  if (!Array.isArray(list)) {
    throw new Error("TYPE_ERROR: list_get expected list, got " + howlFrameValueKind(list));
  }
  var i = howlFrameListIndex(idx);
  if (i < 0 || i >= list.length) return "";
  return list[i] ?? "";
}
function howlFrameListLen(list) {
  if (!Array.isArray(list)) {
    throw new Error("TYPE_ERROR: list_len expected list, got " + howlFrameValueKind(list));
  }
  return list.length;
}
`
}

// mapKeyOrderJSHelper sorts dict keys by UTF-8 bytes.
// Go's sort.Strings and the VM use that order. Array.prototype.sort with no
// comparator compares UTF-16 code units, which places a supplementary-plane
// key before U+F000. Encoding each code point to UTF-8 and comparing those
// bytes matches Go, including non-BMP keys.
func mapKeyOrderJSHelper() string {
	return `function howlFrameUTF8Bytes(s) {
  var out = [];
  var i = 0;
  while (i < s.length) {
    var cp = s.codePointAt(i);
    i += cp > 0xFFFF ? 2 : 1;
    if (cp < 0x80) {
      out.push(cp);
    } else if (cp < 0x800) {
      out.push(0xC0 | (cp >>> 6), 0x80 | (cp & 0x3F));
    } else if (cp < 0x10000) {
      out.push(0xE0 | (cp >>> 12), 0x80 | ((cp >>> 6) & 0x3F), 0x80 | (cp & 0x3F));
    } else {
      out.push(0xF0 | (cp >>> 18), 0x80 | ((cp >>> 12) & 0x3F), 0x80 | ((cp >>> 6) & 0x3F), 0x80 | (cp & 0x3F));
    }
  }
  return out;
}
function howlFrameCompareUTF8(a, b) {
  var ab = howlFrameUTF8Bytes(a);
  var bb = howlFrameUTF8Bytes(b);
  var n = ab.length < bb.length ? ab.length : bb.length;
  for (var i = 0; i < n; i++) {
    if (ab[i] !== bb[i]) return ab[i] - bb[i];
  }
  return ab.length - bb.length;
}
`
}

// encodeJSONJSHelper writes encode_json as the VM does (Go's encoding/json,
// as Go 1.25 writes it): object keys in UTF-8 byte order (code point order,
// which JavaScript's default sort of UTF-16 units is not), no spaces,
// < > & U+2028 U+2029 escaped as \u003c and so on, other control
// characters as \u00XX, a lone surrogate (text that was not UTF-8) as
// \ufffd, -0 kept, and a number that is not finite an ENCODE_JSON_ERROR.
// Byte arrays are their number lists, as on the VM.
func encodeJSONJSHelper() string {
	return `function howlFrameJSONCompareKeys(a, b) {
  for (let i = 0, j = 0; i < a.length && j < b.length; ) {
    const x = a.codePointAt(i), y = b.codePointAt(j);
    if (x !== y) return x < y ? -1 : 1;
    i += x > 0xffff ? 2 : 1;
    j += y > 0xffff ? 2 : 1;
  }
  return a.length - b.length;
}
function howlFrameJSONString(s) {
  let out = "\"";
  for (let i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i);
    if (c >= 0xd800 && c <= 0xdbff && i + 1 < s.length) {
      const d = s.charCodeAt(i + 1);
      if (d >= 0xdc00 && d <= 0xdfff) { out += s[i] + s[i + 1]; i++; continue; }
    }
    if (c >= 0xd800 && c <= 0xdfff) { out += "\\ufffd"; continue; }
    switch (c) {
      case 0x22: out += "\\\""; continue;
      case 0x5c: out += "\\\\"; continue;
      case 0x08: out += "\\b"; continue;
      case 0x0c: out += "\\f"; continue;
      case 0x0a: out += "\\n"; continue;
      case 0x0d: out += "\\r"; continue;
      case 0x09: out += "\\t"; continue;
    }
    if (c < 0x20 || c === 0x3c || c === 0x3e || c === 0x26 || c === 0x2028 || c === 0x2029) {
      out += "\\u" + c.toString(16).padStart(4, "0");
      continue;
    }
    out += s[i];
  }
  return out + "\"";
}
function howlFrameEncodeJSON(v) {
  if (v === null || v === undefined) return "null";
  if (typeof v === "boolean") return v ? "true" : "false";
  if (typeof v === "bigint") return v.toString();
  if (typeof v === "number") {
    if (!Number.isFinite(v)) throw new Error("ENCODE_JSON_ERROR: json: unsupported value: " + (Number.isNaN(v) ? "NaN" : v > 0 ? "+Inf" : "-Inf"));
    return Object.is(v, -0) ? "-0" : JSON.stringify(v);
  }
  if (typeof v === "string") return howlFrameJSONString(v);
  if (ArrayBuffer.isView(v)) return "[" + Array.from(v).join(",") + "]";
  if (Array.isArray(v)) return "[" + v.map(howlFrameEncodeJSON).join(",") + "]";
  if (typeof v === "object") {
    const keys = Object.keys(v).sort(howlFrameJSONCompareKeys);
    return "{" + keys.map((k) => howlFrameJSONString(k) + ":" + howlFrameEncodeJSON(v[k])).join(",") + "}";
  }
  return JSON.stringify(v);
}
`
}

// eqJSHelper is = as the VM has it (BcValuesEqual): nil equals only nil, a
// list or dict (an array or plain object) equals nothing, not even itself,
// numbers compare by value (an int held as a BigInt included), and anything
// else by ===, so host objects such as elements keep their identity.
func eqJSHelper() string {
	return `function howlFrameEq(a, b) {
  if (a === null || a === undefined || b === null || b === undefined) {
    return (a === null || a === undefined) && (b === null || b === undefined);
  }
  const collection = (v) => Array.isArray(v) || (typeof v === "object" && (Object.getPrototypeOf(v) === Object.prototype || Object.getPrototypeOf(v) === null));
  if (collection(a) || collection(b)) return false;
  if (typeof a === "bigint" || typeof b === "bigint") {
    const num = (v) => typeof v === "bigint" || typeof v === "number";
    return num(a) && num(b) && a == b;
  }
  return a === b;
}
`
}

func htmlEscapeJSHelper() string {
	// The replacements match Go's html.EscapeString: & < > " '.
	// A non-string throws TYPE_ERROR. This is not a JavaScript encoder.
	return `function howlFrameHTMLEscape(kind, v) {
  if (typeof v !== "string") {
    throw new Error("TYPE_ERROR: " + kind + " expected string, got " + (v === null ? "null" : typeof v));
  }
  return v.replace(/[&<>"']/g, function (c) {
    switch (c) {
      case "&": return "&amp;";
      case "<": return "&lt;";
      case ">": return "&gt;";
      case '"': return "&#34;";
      case "'": return "&#39;";
      default: return c;
    }
  });
}
`
}

func requestReadJSHelper() string {
	return `function howlRequestRead(kind, req, name) {
  if (typeof name !== "string" || name === "") {
    throw new Error("TYPE_ERROR: " + kind + " expected non-empty string name");
  }
  if (req === null || typeof req !== "object" || Array.isArray(req)) {
    throw new Error("TYPE_ERROR: " + kind + " expected request");
  }
  if (kind === "req_query") return howlReadQuery(req, name);
  if (kind === "req_header") return howlReadHeader(req, name);
  if (kind === "req_path") return howlReadPath(req, name);
  throw new Error("TYPE_ERROR: " + kind + " expected request");
}
function howlReadQuery(req, name) {
  if (req.searchParams && typeof req.searchParams.get === "function") {
    var found = req.searchParams.get(name);
    return found == null ? "" : String(found);
  }
  if (typeof req.url === "string") {
    var parsed = null;
    try { parsed = new URL(req.url, "http://localhost"); } catch (e) { parsed = null; }
    if (!parsed) throw new Error("TYPE_ERROR: req_query expected request");
    var value = parsed.searchParams.get(name);
    return value == null ? "" : String(value);
  }
  if (req.query && typeof req.query === "object" && !Array.isArray(req.query)) {
    if (!Object.prototype.hasOwnProperty.call(req.query, name)) return "";
    return howlRequestString("req_query", req.query[name]);
  }
  throw new Error("TYPE_ERROR: req_query expected request");
}
function howlReadHeader(req, name) {
  var headers = req.headers;
  if (headers == null) return "";
  if (typeof headers.get === "function") {
    var found = headers.get(name);
    return found == null ? "" : String(found);
  }
  if (typeof headers !== "object" || Array.isArray(headers)) {
    throw new Error("TYPE_ERROR: req_header expected request");
  }
  var want = name.toLowerCase();
  var keys = Object.keys(headers);
  for (var i = 0; i < keys.length; i++) {
    if (keys[i].toLowerCase() === want) return howlRequestString("req_header", headers[keys[i]]);
  }
  return "";
}
function howlReadPath(req, name) {
  var params = req.pathParams;
  if (params == null) return "";
  if (typeof params !== "object" || Array.isArray(params)) {
    throw new Error("TYPE_ERROR: req_path expected request");
  }
  if (!Object.prototype.hasOwnProperty.call(params, name)) return "";
  return howlRequestString("req_path", params[name]);
}
function howlRequestString(kind, value) {
  if (Array.isArray(value)) {
    if (value.length === 0) return "";
    value = value[0];
  }
  if (value == null) return "";
  if (typeof value !== "string") throw new Error("TYPE_ERROR: " + kind + " expected string value");
  return value;
}

`
}

// grantJSHelper reads the host's explicit grant. Node uses its environment;
// browsers use a host-supplied global before loading the compiled script.
// An empty, unset, or non-string grant denies every name.
func grantJSHelper() string {
	return `function howlFrameGrantHas(name) {
  var raw = "";
  if (typeof process !== "undefined" && process.env) {
    raw = process.env.HOWLFRAME_ALLOW_CAPS || "";
  } else if (typeof globalThis !== "undefined") {
    raw = globalThis.HOWLFRAME_ALLOW_CAPS;
  }
  if (typeof raw !== "string") return false;
  var parts = raw.split(",");
  for (var i = 0; i < parts.length; i++) {
    if (parts[i].trim() === name) return true;
  }
  return false;
}

`
}

// envJSHelper mediates (env key). process.env[key] runs only after
// environment is present, so a denial cannot return the secret.
func envJSHelper() string {
	return `function howlFrameEnv(key) {
  if (!howlFrameGrantHas("environment")) {
    throw new Error("CAPABILITY_DENIED: capability denied: environment");
  }
  var value = process.env[key];
  if (value == null) return "";
  return String(value);
}

`
}

// execJSHelper mediates (exec cmd args...). The grant name is process, the
// same name as capability.ForConstruct("exec") and OpExec. spawnSync runs
// only after that grant is present, so a denial cannot start a process or
// put the command in the error. There is no shell. Stdout is returned
// ahead of stderr, which matches CombinedOutput when only one stream writes.
func execJSHelper() string {
	return codeLoadingEnvJSSource() + `function howlFrameExecAllowed(cmd, args) {
  var fs = require("fs");
  if (typeof cmd !== "string" || cmd.charAt(0) !== "/") return false;
  var prefix = "";
  var parts = cmd.slice(1).split("/");
  for (var i = 0; i < parts.length; i++) {
    var part = parts[i];
    if (part === "" || part === "." || part === "..") return false;
    prefix += "/" + part;
    try {
      if (fs.lstatSync(prefix).isSymbolicLink()) return false;
    } catch (e) {
      return false;
    }
  }
  var grants = String(process.env.HOWLFRAME_ALLOW_CAPS || "").split(",");
  for (var g = 0; g < grants.length; g++) {
    var grant = grants[g].trim();
    if (grant.indexOf("process:exec=") !== 0) continue;
    var fields = grant.slice("process:exec=".length).split(/\s+/).filter(function (f) { return f !== ""; });
    if (fields.length === 0 || fields[0] !== cmd) continue;
    var pats = fields.slice(1);
    for (var q = 0; q < pats.length - 1; q++) {
      if (pats[q] === "**") throw new Error("CAPABILITY_DENIED: capability denied: process: invalid process:exec grant (** must be last)");
    }
    var ok = true;
    var rest = false;
    for (var k = 0; k < pats.length; k++) {
      if (pats[k] === "**") { rest = true; break; }
      if (k >= args.length || !howlFrameExecSlot(pats[k], String(args[k]))) { ok = false; break; }
    }
    if (ok && !rest && args.length !== pats.length) ok = false;
    if (ok) return true;
  }
  return false;
}

// "*" refuses empty and "-"-prefixed arguments; a leading backslash escapes a literal.
function howlFrameExecSlot(p, arg) {
  if (p === "*") return arg !== "" && arg.charAt(0) !== "-";
  if (p.charAt(0) === "\\") return arg === p.slice(1);
  return p === arg;
}

// howlFrameCodeLoadingEnv mirrors capability.DynamicLoaderEnv over the shared tables.
function howlFrameCodeLoadingEnv(name) {
  var u = String(name).toUpperCase();
  if (howlFrameCodeLoadingEnvNames.indexOf(u) >= 0) return true;
  for (var i = 0; i < howlFrameCodeLoadingEnvPrefixes.length; i++) {
    if (u.indexOf(howlFrameCodeLoadingEnvPrefixes[i]) === 0) return true;
  }
  for (var j = 0; j < howlFrameCodeLoadingEnvSuffixes.length; j++) {
    var s = howlFrameCodeLoadingEnvSuffixes[j];
    if (u.length >= s.length && u.slice(u.length - s.length) === s) return true;
  }
  return false;
}

// Only process:env=NAME variables reach the child; never the host environment.
function howlFrameExecEnv() {
  var env = {};
  var grants = String(process.env.HOWLFRAME_ALLOW_CAPS || "").split(",");
  for (var i = 0; i < grants.length; i++) {
    var grant = grants[i].trim();
    if (grant.indexOf("process:env=") !== 0) continue;
    var name = grant.slice("process:env=".length);
    if (name === "" || /[= ]/.test(name) || howlFrameCodeLoadingEnv(name)) continue;
    if (Object.prototype.hasOwnProperty.call(process.env, name)) env[name] = process.env[name];
  }
  return env;
}

function howlFrameExec(cmd, args) {
  if (!howlFrameExecAllowed(cmd, args)) {
    throw new Error("CAPABILITY_DENIED: capability denied: process");
  }
  var result = require("child_process").spawnSync(cmd, args, { encoding: "utf8", env: howlFrameExecEnv() });
  if (result.error) {
    throw new Error("IO_ERROR: exec failed: " + result.error.message);
  }
  if (result.status !== 0) {
    var statusText = result.status == null ? "signal" : String(result.status);
    throw new Error("IO_ERROR: exec failed: exit status " + statusText);
  }
  var stdout = result.stdout == null ? "" : String(result.stdout);
  var stderr = result.stderr == null ? "" : String(result.stderr);
  return stdout + stderr;
}

`
}

// readFileJSHelper mediates (read_file path). The grant name is filesystem,
// the same name as capability.ForConstruct("read_file") and OpReadFile.
// readFileSync runs only after that grant is present, so a denial cannot
// read the file or put the path in the error. The return is UTF-8 text,
// which bytes_to_string prints unchanged.
func readFileJSHelper() string {
	return `function howlFrameReadFile(path) {
  if (!howlFrameGrantHas("filesystem")) {
    throw new Error("CAPABILITY_DENIED: capability denied: filesystem");
  }
  return require("fs").readFileSync(path, "utf8");
}

`
}

// fetchJSHelper mediates (fetch url method [body]). The grant name is
// network, the same name as capability.ForConstruct("fetch") and OpFetch.
// fetch runs only after that grant is present, so a denial cannot open a
// connection or put the URL in the error. The return is response text,
// which bytes_to_string prints unchanged. A third argument is the body;
// a two-argument call sends none.
func fetchJSHelper() string {
	return specialPrefixJSSource() + `function howlFrameGrantList() {
  var raw = "";
  if (typeof process !== "undefined" && process.env) {
    raw = process.env.HOWLFRAME_ALLOW_CAPS || "";
  } else if (typeof globalThis !== "undefined") {
    raw = globalThis.HOWLFRAME_ALLOW_CAPS;
  }
  if (typeof raw !== "string") return [];
  return raw.split(",").map(function (p) { return p.trim(); });
}

// Mirrors capability.Grants.AuthorizeFetch: exact host (WHATWG-canonical,
// so IP literals compare in canonical form) and port; an omitted grant port
// means the scheme default. The coarse network grant never fetches.
// howlFrameCanonHost mirrors capability's canonicalIP: an IPv4-mapped IPv6
// literal ([::ffff:a.b.c.d]) is the same host as a.b.c.d (#177).
function howlFrameCanonHost(hostname) {
  var h = String(hostname).toLowerCase();
  if (h.charAt(0) !== "[") return h;
  var b = howlFrameAddrBytes(h);
  if (b && b.slice(0, 12).join(",") === "0,0,0,0,0,0,0,0,0,0,255,255") return b.slice(12).join(".");
  return h;
}

function howlFrameFetchOrigin(u) {
  return u.protocol + "//" + howlFrameCanonHost(u.hostname) + ":" + (u.port || (u.protocol === "https:" ? "443" : "80"));
}

function howlFrameFetchAllowed(u) {
  if (!u || (u.protocol !== "http:" && u.protocol !== "https:") || !u.hostname) return false;
  if (u.hostname.charAt(u.hostname.length - 1) === ".") return false;
  var def = u.protocol === "https:" ? "443" : "80";
  var port = u.port || def;
  var grants = howlFrameGrantList();
  for (var i = 0; i < grants.length; i++) {
    if (grants[i].indexOf("network:fetch=") !== 0) continue;
    var spec = grants[i].slice("network:fetch=".length);
    if (spec === "" || /[\/@?#*%\s\\]/.test(spec) || spec.charAt(spec.length - 1) === ":") continue;
    var g;
    try { g = new URL("http://" + spec); } catch (e) { continue; }
    var writtenHost = spec.charAt(0) === "[" ? spec.slice(0, spec.indexOf("]") + 1) : spec.split(":")[0].toLowerCase();
    if (writtenHost.charAt(0) !== "[" && /(^|\.)[0-9][^.]*$/.test(writtenHost) && writtenHost !== g.hostname) continue;
    if (howlFrameCanonHost(g.hostname) !== howlFrameCanonHost(u.hostname)) continue;
    var gport = /:\d+$/.test(spec) ? spec.slice(spec.lastIndexOf(":") + 1) : def;
    if (String(Number(gport)) === gport && gport === port) return true;
  }
  return false;
}

function howlFrameFetchURL(raw, base) {
  var written = String(raw);
  if (written.indexOf("\\") >= 0 || /^https?:/i.test(written) && !/^https?:\/\/[^/]/i.test(written)) howlFrameFetchDeny();
  var u = new URL(written, base);
  // WHATWG rewrites short, octal and hexadecimal IPv4 before grant matching.
  var authority = /^(?:[a-z][a-z0-9+.-]*:)?\/\/([^/?#]*)/i.exec(written);
  if (authority) {
    var host = authority[1].slice(authority[1].lastIndexOf("@") + 1).split(":")[0].toLowerCase();
    if (host.charAt(0) !== "[" && /(^|\.)[0-9][^.]*$/.test(host) && host !== u.hostname) howlFrameFetchDeny();
  }
  return u;
}

function howlFrameFetchDeny() {
  throw new Error("CAPABILITY_DENIED: capability denied: network");
}

// Node checks private and metadata DNS results and pins the connection. A
// browser cannot inspect DNS, so its own network policy governs named hosts.
function howlFrameAddrBytes(a) {
  a = String(a).toLowerCase().replace(/^\[|\]$/g, "");
  var v4 = /^(\d+)\.(\d+)\.(\d+)\.(\d+)$/.exec(a);
  if (v4) {
    var out4 = [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255, 255];
    for (var k = 1; k <= 4; k++) { var n4 = Number(v4[k]); if (n4 > 255) return null; out4.push(n4); }
    return out4;
  }
  if (a.indexOf(":") < 0 || a.indexOf("%") >= 0) return null;
  var tail = /^(.*:)(\d+\.\d+\.\d+\.\d+)$/.exec(a);
  if (tail) {
    var t = howlFrameAddrBytes(tail[2]);
    if (!t) return null;
    a = tail[1] + ((t[12] << 8) | t[13]).toString(16) + ":" + ((t[14] << 8) | t[15]).toString(16);
  }
  var halves = a.split("::");
  if (halves.length > 2) return null;
  var head = halves[0] ? halves[0].split(":") : [];
  var rest = halves.length === 2 && halves[1] ? halves[1].split(":") : [];
  var fill = 8 - head.length - rest.length;
  if (halves.length === 1 ? fill !== 0 : fill < 1) return null;
  var groups = head.concat(new Array(halves.length === 2 ? fill : 0).fill("0"), rest);
  var out = [];
  for (var g = 0; g < groups.length; g++) {
    if (!/^[0-9a-f]{1,4}$/.test(groups[g])) return null;
    var n = parseInt(groups[g], 16);
    out.push(n >> 8, n & 255);
  }
  return out;
}

// howlFrameIsSpecial mirrors capability.ForbiddenDestination over the shared table.
function howlFrameIsSpecial(b) {
  var mapped = function (x) { return [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255, 255].concat(x); };
  if (b[0] === 0x20 && b[1] === 0x02 && howlFrameIsSpecial(mapped(b.slice(2, 6)))) return true;
  var compat = b.slice(0, 12).every(function (x) { return x === 0; });
  if (compat && !(b[12] === 0 && b[13] === 0 && b[14] === 0)) return howlFrameIsSpecial(mapped(b.slice(12, 16)));
  var translated = b.slice(0, 8).every(function (x) { return x === 0; }) && b[8] === 255 && b[9] === 255 && b[10] === 0 && b[11] === 0;
  if (translated) return howlFrameIsSpecial(mapped(b.slice(12, 16)));
  for (var i = 0; i < howlFrameSpecialPrefixes.length; i++) {
    var p = howlFrameSpecialPrefixes[i][0], bits = howlFrameSpecialPrefixes[i][1], ok = true;
    for (var k = 0; k < 16 && ok && bits > 0; k++, bits -= 8) {
      var mask = bits >= 8 ? 255 : (0xff << (8 - bits)) & 255;
      ok = (b[k] & mask) === (p[k] & mask);
    }
    if (ok) return true;
  }
  return false;
}

function howlFrameIsLoopback(b) {
  if (!b) return false;
  var v4 = b.slice(0, 12).join(",") === "0,0,0,0,0,0,0,0,0,0,255,255";
  return v4 && b[12] === 127 || b.slice(0, 15).every(function (x) { return x === 0; }) && b[15] === 1;
}

function howlFrameForbiddenAddress(address, explicit) {
  var b = howlFrameAddrBytes(address);
  if (!b) return true;
  var v4 = b.slice(0, 12).join(",") === "0,0,0,0,0,0,0,0,0,0,255,255";
  if (v4 && b[12] >= 224 && b[12] <= 239 || b[0] === 0xff) return true;
  if (explicit) return false;
  return howlFrameIsSpecial(b);
}

async function howlFrameFetchResolvedOk(u) {
  var host = u.hostname.replace(/^\[|\]$/g, "");
  var literal = /^\[.*\]$/.test(u.hostname) || /^\d+\.\d+\.\d+\.\d+$/.test(host);
  if (literal) return !howlFrameForbiddenAddress(host, true);
  if (typeof require !== "function") return true;
  var addrs = await require("dns").promises.lookup(host, { all: true });
  if (!addrs.length) return false;
  var localhost = host === "localhost";
  for (var i = 0; i < addrs.length; i++) {
    if (howlFrameForbiddenAddress(addrs[i].address, localhost)) return false;
    // The localhost opt-out reaches loopback only (127/8, ::1).
    if (localhost && !howlFrameIsLoopback(howlFrameAddrBytes(addrs[i].address))) return false;
  }
  // Pin this request to the full vetted set; never resolve again at connect time.
  u.howlFrameAddresses = addrs.map(function (a) { return { address: a.address, family: a.family }; });
  return true;
}

function howlFrameFetchRequest(u, init) {
  if (typeof require !== "function") return fetch(u.href, init);
  // Native requests ignore proxy environment variables, including Node 24's
  // NODE_USE_ENV_PROXY. agent:false also prevents pooled connections bypassing lookup.
  return new Promise(function (resolve, reject) {
    var options = { method: init.method, headers: init.headers, agent: false };
    if (u.howlFrameAddresses) options.lookup = function (host, opts, cb) {
      if (typeof opts === "function") { cb = opts; opts = {}; }
      var all = u.howlFrameAddresses.filter(function (a) {
        return !opts || !opts.family || opts.family === a.family || opts.family === "IPv" + a.family;
      });
      if (!all.length) return cb(new Error("CAPABILITY_DENIED: capability denied: network"));
      if (opts && opts.all) return cb(null, all.map(function (a) { return { address: a.address, family: a.family }; }));
      cb(null, all[0].address, all[0].family);
    };
    var req = require(u.protocol === "https:" ? "https" : "http").request(u, options, function (res) {
      var chunks = [];
      res.on("data", function (chunk) { chunks.push(chunk); });
      res.on("error", reject);
      res.on("end", function () { resolve({
        status: res.statusCode,
        headers: { get: function (name) { return res.headers[name.toLowerCase()]; } },
        text: async function () {
          var data = Buffer.concat(chunks);
          var encoding = String(res.headers["content-encoding"] || "").toLowerCase();
          if (encoding === "gzip") data = require("zlib").gunzipSync(data);
          else if (encoding === "deflate") data = require("zlib").inflateSync(data);
          else if (encoding === "br") data = require("zlib").brotliDecompressSync(data);
          return data.toString("utf8").replace(/^\uFEFF/, "");
        }
      }); });
    });
    req.on("error", reject);
    if (init.body !== undefined) req.write(init.body);
    req.end();
  });
}

async function howlFrameFetch(url, method, body, headers) {
  var u;
  // Browser-relative URLs resolve against the host page, then face the same
  // exact destination grant checks as absolute URLs. Node has no implicit base.
  var base = typeof process === "undefined" && typeof location !== "undefined" ? location.href : undefined;
  try { u = howlFrameFetchURL(url, base); } catch (e) { howlFrameFetchDeny(); }
  if (base && (u.username !== "" || u.password !== "")) howlFrameFetchDeny();
  if (!howlFrameFetchAllowed(u)) howlFrameFetchDeny();
  var init = { method: method, redirect: "manual", headers: Object.assign({}, headers || {}) };
  if (arguments.length >= 3) {
    init.body = body;
  }
  var first = u;
  for (var hop = 0; ; hop++) {
    if (!(await howlFrameFetchResolvedOk(u))) howlFrameFetchDeny();
    var pending = howlFrameFetchRequest(u, init);
    var r = await pending;
    if (r && r.type === "opaqueredirect") howlFrameFetchDeny();
    var loc = r && r.status >= 300 && r.status < 400 && r.headers && r.headers.get ? r.headers.get("location") : null;
    if (!loc) return r.text();
    if (hop >= 10) howlFrameFetchDeny();
    var previous = u;
    try { u = howlFrameFetchURL(loc, u); } catch (e) { howlFrameFetchDeny(); }
    if (previous.protocol === "https:" && u.protocol === "http:") howlFrameFetchDeny();
    var optHost = u.hostname.replace(/^\[|\]$/g, "");
    if ((/^\[.*\]$/.test(u.hostname) || /^\d+\.\d+\.\d+\.\d+$/.test(optHost) || optHost === "localhost") && howlFrameCanonHost(u.hostname) !== howlFrameCanonHost(first.hostname)) howlFrameFetchDeny();
    if (u.username !== "" || u.password !== "") howlFrameFetchDeny();
    if (howlFrameFetchOrigin(previous) !== howlFrameFetchOrigin(u)) {
      Object.keys(init.headers || {}).forEach(function (name) {
        if (howlFrameRedirectSafeHeaders.indexOf(String(name).toLowerCase()) < 0) delete init.headers[name];
      });
    }
    if (!howlFrameFetchAllowed(u)) howlFrameFetchDeny();
    if (r.status === 303 || (r.status === 301 || r.status === 302) && init.method === "POST") {
      init = { method: "GET", redirect: "manual" };
    }
  }
}

`
}

// writeFileJSHelper mediates (write_file path data). The grant name is
// filesystem, the same name as capability.ForConstruct("write_file") and
// OpWriteFile. writeFileSync runs only after that grant is present, so a
// denial cannot write the file or put the path in the error. The mode matches
// the Go backend's 0644. A failure after the grant throws, which is
// writeFileSync.
func writeFileJSHelper() string {
	return `function howlFrameWriteFile(path, data) {
  if (!howlFrameGrantHas("filesystem")) {
    throw new Error("CAPABILITY_DENIED: capability denied: filesystem");
  }
  require("fs").writeFileSync(path, data, { encoding: "utf8", mode: 0o644 });
}

`
}

// mkdirJSHelper mediates (mkdir path). The grant name is filesystem, the
// same name as capability.ForConstruct("mkdir") and OpMkdir. mkdirSync runs
// only after that grant is present, so a denial cannot create the directory
// or put the path in the error. recursive matches os.MkdirAll. The mode
// matches the Go backend's 0755.
func mkdirJSHelper() string {
	return `function howlFrameMkdir(path) {
  if (!howlFrameGrantHas("filesystem")) {
    throw new Error("CAPABILITY_DENIED: capability denied: filesystem");
  }
  require("fs").mkdirSync(path, { recursive: true, mode: 0o755 });
}

`
}

func numericJSHelper() string {
	return `function howlFrameToInt(v) {
  if (typeof v === "number") {
    if (!Number.isFinite(v)) {
      throw new Error("CONVERSION_ERROR: cannot convert " + v + " to int");
    }
    var t = Math.trunc(v);
    if (t < Number.MIN_SAFE_INTEGER || t > Number.MAX_SAFE_INTEGER) {
      throw new Error("CONVERSION_ERROR: cannot convert " + v + " to int");
    }
    return t;
  }
  if (typeof v === "string") {
    var s = v.trim();
    if (!/^[+-]?\d+$/.test(s)) {
      throw new Error("CONVERSION_ERROR: cannot convert " + JSON.stringify(v) + " to int");
    }
    var n = Number(s);
    if (!Number.isInteger(n) || n < Number.MIN_SAFE_INTEGER || n > Number.MAX_SAFE_INTEGER) {
      throw new Error("CONVERSION_ERROR: cannot convert " + JSON.stringify(v) + " to int");
    }
    return n;
  }
  throw new Error("CONVERSION_ERROR: cannot convert " + (typeof v) + " to int");
}
function howlFrameSafeInt(v) {
  if (!Number.isSafeInteger(v)) {
    throw new Error("RUNTIME_ERROR: integer result is outside the exact JavaScript integer range");
  }
  return v;
}
// howlFrameOperands applies the VM's operand rule (bugs.md #66): + joins two
// strings, and otherwise both operands must be numbers. A value of unknown
// type (from a dict of mixed values or parse_json) reaches here unchecked.
// The type names are the VM's (%T); JavaScript cannot tell 2.0 from 2, so a
// whole float is named int64.
function howlFrameGoType(v) {
  if (v === null || v === undefined) return "<nil>";
  if (typeof v === "number") return Number.isInteger(v) ? "int64" : "float64";
  if (typeof v === "string") return "string";
  if (typeof v === "boolean") return "bool";
  if (Array.isArray(v)) return "[]interface {}";
  return "map[string]interface {}";
}
function howlFrameOperands(op, a, b) {
  if (typeof a === "number" && typeof b === "number") return true;
  if (op === "+" && typeof a === "string" && typeof b === "string") return false;
  throw new Error("TYPE_ERROR: " + op + " requires numeric operands, got " + howlFrameGoType(a) + " and " + howlFrameGoType(b));
}
function howlFrameArith(op, a, b) {
  if (!howlFrameOperands(op, a, b)) return a + b;
  var v = op === "+" ? a + b : op === "-" ? a - b : a * b;
  if (typeof v === "number" && !Number.isSafeInteger(v) && Number.isInteger(a) && Number.isInteger(b)) {
    throw new Error("RUNTIME_ERROR: integer result is outside the exact JavaScript integer range");
  }
  return howlFrameFinite(op, a, b, v);
}
function howlFrameCompare(op, a, b) {
  var numbers = typeof a === "number" && typeof b === "number";
  if (!numbers && !(typeof a === "string" && typeof b === "string")) {
    throw new Error("TYPE_ERROR: " + op + " requires numeric operands, got " + howlFrameGoType(a) + " and " + howlFrameGoType(b));
  }
  return op === "<" ? a < b : op === ">" ? a > b : op === "<=" ? a <= b : a >= b;
}
// NUMERIC_CONTRACT section 7: float overflow is an error, not Infinity or NaN.
function howlFrameFloatArith(op, a, b) {
  if (!howlFrameOperands(op, a, b)) return a + b;
  return howlFrameFinite(op, a, b, op === "+" ? a + b : op === "-" ? a - b : a * b);
}
function howlFrameFinite(op, a, b, v) {
  if (typeof v === "number" && !Number.isFinite(v)) {
    throw new Error("RUNTIME_ERROR: floating-point overflow: " + a + " " + op + " " + b + " is not finite");
  }
  return v;
}
function howlFrameParseJSON(text) {
  // JavaScript numbers cannot hold every int64. When the engine exposes the
  // token source, reject only integer tokens that would lose precision. Without
  // it, fail closed on any integer-valued number outside the exact range:
  // an integer beyond 2^53 always decodes to such a value.
  return JSON.parse(text, function (key, value, context) {
    if (typeof value === "number" && !Number.isSafeInteger(value)) {
      var unsafe = context && typeof context.source === "string" ? /^-?\d+$/.test(context.source) : Number.isInteger(value);
      if (unsafe) {
        throw new Error("CONVERSION_ERROR: integer is outside the exact JavaScript integer range");
      }
    }
    return value;
  });
}
function howlFrameToFloat(v) {
  if (typeof v === "number") {
    if (!Number.isFinite(v)) {
      throw new Error("CONVERSION_ERROR: cannot convert " + v + " to float");
    }
    return v;
  }
  if (typeof v === "string") {
    var s = v.trim();
    if (s === "") {
      throw new Error("CONVERSION_ERROR: cannot convert \"\" to float");
    }
    var n = Number(s);
    if (Number.isNaN(n) || !Number.isFinite(n)) {
      throw new Error("CONVERSION_ERROR: cannot convert " + JSON.stringify(v) + " to float");
    }
    return n;
  }
  throw new Error("CONVERSION_ERROR: cannot convert " + (typeof v) + " to float");
}
function howlFrameDiv(a, b) {
  var af = howlFrameToFloat(a);
  var bf = howlFrameToFloat(b);
  if (bf === 0) {
    throw new Error("RUNTIME_ERROR: division by zero");
  }
  var res = af / bf;
  if (!Number.isFinite(res)) {
    throw new Error("RUNTIME_ERROR: division produced an invalid floating-point result");
  }
  return res;
}
`
}

func jsHostHelpers() string {
	if !jsNeedsEnv && !jsNeedsExec && !jsNeedsReadFile && !jsNeedsFetch && !jsNeedsWriteFile && !jsNeedsMkdir {
		return ""
	}
	helper := grantJSHelper()
	if jsNeedsEnv {
		helper += envJSHelper()
	}
	if jsNeedsExec {
		helper += execJSHelper()
	}
	if jsNeedsReadFile {
		helper += readFileJSHelper()
	}
	if jsNeedsFetch {
		helper += fetchJSHelper()
	}
	if jsNeedsWriteFile {
		helper += writeFileJSHelper()
	}
	if jsNeedsMkdir {
		helper += mkdirJSHelper()
	}
	return helper
}

func jsEnvCall(keyNode *ast.Node, reqVar string, depth int) string {
	jsNeedsEnv = true
	if keyNode != nil && keyNode.Type == "STRING" {
		return fmt.Sprintf("howlFrameEnv(%q)", keyNode.Value)
	}
	return fmt.Sprintf("howlFrameEnv(%s)", generateJSExpression(keyNode, reqVar, depth+1))
}

func jsExecCall(node *ast.Node, reqVar string, depth int) string {
	jsNeedsExec = true
	if node == nil || len(node.Children) < 2 {
		return ""
	}
	cmd := generateJSExpression(node.Children[1], reqVar, depth+1)
	args := make([]string, 0, len(node.Children)-2)
	for _, arg := range node.Children[2:] {
		args = append(args, generateJSExpression(arg, reqVar, depth+1))
	}
	return fmt.Sprintf("howlFrameExec(%s, [%s])", cmd, strings.Join(args, ", "))
}

func jsReadFileCall(node *ast.Node, reqVar string, depth int) string {
	jsNeedsReadFile = true
	if node == nil || len(node.Children) != 2 {
		return ""
	}
	pathStr := generateJSExpression(node.Children[1], reqVar, depth+1)
	return fmt.Sprintf("howlFrameReadFile(%s)", pathStr)
}

func jsFetchCall(node *ast.Node, reqVar string, depth int) string {
	jsNeedsFetch = true
	if node == nil || (len(node.Children) < 3 || len(node.Children) > 5) {
		return ""
	}
	urlStr := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
	methodStr := generateJSStatementRaw(node.Children[2], reqVar, depth+1)
	if len(node.Children) == 5 {
		bodyStr := generateJSStatementRaw(node.Children[3], reqVar, depth+1)
		headersStr := generateJSStatementRaw(node.Children[4], reqVar, depth+1)
		return fmt.Sprintf("(await howlFrameFetch(%s, %s, %s, %s))", urlStr, methodStr, bodyStr, headersStr)
	}
	if len(node.Children) == 4 {
		bodyStr := generateJSStatementRaw(node.Children[3], reqVar, depth+1)
		return fmt.Sprintf("(await howlFrameFetch(%s, %s, %s))", urlStr, methodStr, bodyStr)
	}
	return fmt.Sprintf("(await howlFrameFetch(%s, %s))", urlStr, methodStr)
}

func jsWriteFileCall(node *ast.Node, reqVar string, depth int) string {
	jsNeedsWriteFile = true
	if node == nil || len(node.Children) != 3 {
		return ""
	}
	pathStr := generateJSExpression(node.Children[1], reqVar, depth+1)
	dataStr := generateJSExpression(node.Children[2], reqVar, depth+1)
	return fmt.Sprintf("howlFrameWriteFile(%s, %s)", pathStr, dataStr)
}

func jsMkdirCall(node *ast.Node, reqVar string, depth int) string {
	jsNeedsMkdir = true
	if node == nil || len(node.Children) != 2 {
		return ""
	}
	pathStr := generateJSExpression(node.Children[1], reqVar, depth+1)
	return fmt.Sprintf("howlFrameMkdir(%s)", pathStr)
}

// jsDictKey emits a dict literal key. A key named __proto__, quoted or not,
// is emitted as a computed key (["__proto__"]), which creates an ordinary
// property; written plainly it would set the object's prototype instead.
func jsDictKey(key *ast.Node) string {
	if key.Value == "__proto__" {
		return `["__proto__"]`
	}
	if key.Type == "STRING" {
		return fmt.Sprintf("%q", key.Value)
	}
	return key.Value
}

func sanitizeJSName(name string) string {
	if !strings.Contains(name, "/") {
		return name
	}
	parts := strings.Split(name, "/")
	res := parts[0]
	for _, p := range parts[1:] {
		if len(p) > 0 {
			res += "_" + p
		}
	}
	return res
}

// jsKnownOrdered reports whether a node is statically a number or a
// string, which the checker has already paired for a comparison.
func jsKnownOrdered(node *ast.Node) bool {
	switch node.Inferred.Kind {
	case ast.Int, ast.Float, ast.String:
		return true
	}
	return false
}

// jsMayBeInt reports whether a node could hold an integer: it is not
// statically known to be a float, string, or other non-numeric value.
func jsMayBeInt(node *ast.Node) bool {
	switch node.Inferred.Kind {
	case ast.Float, ast.String, ast.Bool, ast.List, ast.Dict, ast.Bytes, ast.Void, ast.Struct:
		return false
	}
	return true
}

func EmitJSIR(ir *ir.IRNode, reqVar string, depth int) string {
	switch ir.Kind {
	case "binop":
		arg1 := generateJSExpression(ir.Kids[0], reqVar, depth+1)
		arg2 := generateJSExpression(ir.Kids[1], reqVar, depth+1)
		if ir.Op == "/" {
			jsNumeric = true
			return fmt.Sprintf("howlFrameDiv(%s, %s)", arg1, arg2)
		}
		if (ir.Op == "+" || ir.Op == "-" || ir.Op == "*") &&
			ir.Kids[0].Inferred.Kind == ast.Int && ir.Kids[1].Inferred.Kind == ast.Int {
			// int64 arithmetic must not silently round once a binary64 result
			// leaves the exact integer range (NUMERIC_CONTRACT sections 4 and 12).
			jsNumeric = true
			return fmt.Sprintf("howlFrameSafeInt(%s %s %s)", arg1, BinOpJSToken(ir.Op), arg2)
		}
		if (ir.Op == "+" || ir.Op == "-" || ir.Op == "*") && jsMayBeInt(ir.Kids[0]) && jsMayBeInt(ir.Kids[1]) {
			// Operand types are not statically known to rule out ints. The
			// helper checks at runtime and still concatenates strings.
			jsNumeric = true
			return fmt.Sprintf("howlFrameArith(%q, %s, %s)", ir.Op, arg1, arg2)
		}
		if ir.Op == "+" || ir.Op == "-" || ir.Op == "*" {
			jsNumeric = true
			return fmt.Sprintf("howlFrameFloatArith(%q, %s, %s)", ir.Op, arg1, arg2)
		}
		if ir.Op == "=" || ir.Op == "==" || ir.Op == "!=" {
			jsNeedsEq = true
			if ir.Op == "!=" {
				return fmt.Sprintf("(!howlFrameEq(%s, %s))", arg1, arg2)
			}
			return fmt.Sprintf("howlFrameEq(%s, %s)", arg1, arg2)
		}
		if (ir.Op == "<" || ir.Op == ">" || ir.Op == "<=" || ir.Op == ">=") &&
			(!jsKnownOrdered(ir.Kids[0]) || !jsKnownOrdered(ir.Kids[1])) {
			// An operand of unknown type: the VM orders two numbers or two
			// strings and fails on anything else (bugs.md #66).
			jsNumeric = true
			return fmt.Sprintf("howlFrameCompare(%q, %s, %s)", ir.Op, arg1, arg2)
		}
		return fmt.Sprintf("(%s %s %s)", arg1, BinOpJSToken(ir.Op), arg2)
	case "let":
		var letPrefix strings.Builder
		letPrefix.WriteString("{\n")
		declaredVars := make(map[string]bool)

		bindings, curr := ast.LetChain(&ast.Node{Type: "List", Children: []*ast.Node{{Type: "SYMBOL", Value: "let"}, ir.Kids[0], ir.Kids[1]}})
		for _, binds := range bindings {
			varName := binds.Children[0].Value
			valNode := binds.Children[1]

			var valStr string
			if valNode.Type == "STRING" {
				valStr = fmt.Sprintf("%q", valNode.Value)
			} else if valNode.Type == "List" && len(valNode.Children) > 0 {
				funcName := valNode.Children[0].Value
				if funcName == "call" {
					var args []string
					for j := 2; j < len(valNode.Children); j++ {
						if valNode.Children[j].Type == "STRING" {
							args = append(args, fmt.Sprintf("%q", valNode.Children[j].Value))
						} else {
							args = append(args, generateJSExpression(valNode.Children[j], reqVar, depth+1))
						}
					}
					valStr = fmt.Sprintf("(await %s(%s))", sanitizeJSName(valNode.Children[1].Value), strings.Join(args, ", "))
				} else if funcName == "list" {
					var items []string
					for j := 1; j < len(valNode.Children); j++ {
						if valNode.Children[j].Type == "STRING" {
							items = append(items, fmt.Sprintf("%q", valNode.Children[j].Value))
						} else {
							items = append(items, generateJSExpression(valNode.Children[j], reqVar, depth+1))
						}
					}
					valStr = fmt.Sprintf("[%s]", strings.Join(items, ", "))
				} else if funcName == "dict" {
					var pairs []string
					for j := 1; j < len(valNode.Children); j++ {
						pair := valNode.Children[j]
						if pair.Type == "List" && len(pair.Children) == 2 {
							k := jsDictKey(pair.Children[0])
							v := pair.Children[1].Value
							if pair.Children[1].Type == "STRING" {
								v = fmt.Sprintf("%q", v)
							} else {
								v = generateJSExpression(pair.Children[1], reqVar, depth+1)
							}
							pairs = append(pairs, fmt.Sprintf("%s: %s", k, v))
						}
					}
					valStr = fmt.Sprintf("{%s}", strings.Join(pairs, ", "))
				} else if funcName == "parse_json" {
					bodyVar := valNode.Children[2].Value
					jsNumeric = true
					valStr = fmt.Sprintf("howlFrameParseJSON(%s)", bodyVar)
				} else {
					valStr = generateJSStatementRaw(valNode, reqVar, depth+1)
				}
			} else {
				valStr = generateJSStatementRaw(valNode, reqVar, depth+1)
			}

			if declaredVars[varName] {
				letPrefix.WriteString(fmt.Sprintf("%s = %s;\n", varName, valStr))
			} else {
				letPrefix.WriteString(fmt.Sprintf("let %s = %s;\n", varName, valStr))
				declaredVars[varName] = true
			}
		}
		bodyCode := generateJSStatement(curr, reqVar, depth+1)
		return fmt.Sprintf("%s%s\n}", letPrefix.String(), bodyCode)
	case "try_let":
		binds := ir.Kids[0]
		varName := binds.Children[0].Value
		valNode := binds.Children[1]

		var valStr string
		if valNode.Type == "List" && len(valNode.Children) > 0 && valNode.Children[0].Value == "parse_json" {
			bodyVar := generateJSStatementRaw(valNode.Children[2], reqVar, depth+1)
			jsNumeric = true
			valStr = fmt.Sprintf("howlFrameParseJSON(%s)", bodyVar)
		} else {
			valStr = generateJSStatementRaw(valNode, reqVar, depth+1)
		}

		catchNode := ir.Kids[1]
		errVar := catchNode.Children[1].Value
		catchBodyCode := generateJSStatement(catchNode.Children[2], reqVar, depth+1)
		successBodyCode := generateJSStatement(ir.Kids[2], reqVar, depth+1)

		return fmt.Sprintf("{\n\tlet %s;\n\tlet %s = null;\n\ttry {\n\t\t%s = %s;\n\t} catch (__howlFrameCaught) {\n\t\tif (__howlFrameCaught && __howlFrameCaught.howlFrameExit) throw __howlFrameCaught;\n\t\t%s = __howlFrameCaught;\n\t}\n\tif (%s !== null) {\n\t\t%s\n\t} else {\n\t\t%s\n\t}\n}", varName, errVar, varName, valStr, errVar, errVar, catchBodyCode, successBodyCode)
	case "for":
		itemNode := ir.Kids[0].Value
		// The iterable may be any expression, not only a bound symbol. Reading
		// .Value directly yielded "" for a list-valued expression such as
		// (for m (map_get d "missions") ...), emitting "for (let m of )".
		listExpr := generateJSExpression(ir.Kids[1], reqVar, depth+1)
		bodyCode := generateJSStatement(ir.Kids[2], reqVar, depth+1)
		return fmt.Sprintf("for (let %s of %s) {\n%s\n}", itemNode, listExpr, bodyCode)
	case "call":
		funcName := sanitizeJSName(ir.Kids[0].Value)
		var args []string
		for j := 1; j < len(ir.Kids); j++ {
			args = append(args, generateJSExpression(ir.Kids[j], reqVar, depth+1))
		}
		return fmt.Sprintf("(await %s(%s))", funcName, strings.Join(args, ", "))
	case "spawn":
		lambdaNode := ir.Kids[0]
		bodyCode := generateJSStatement(lambdaNode.Children[2], reqVar, depth+1)
		return fmt.Sprintf(";(async () => {\n%s\n})();", bodyCode)
	case "return":
		return fmt.Sprintf("return %s;", generateJSStatementRaw(ir.Kids[0], reqVar, depth+1))
	case "if":
		condExpr := generateJSExpression(ir.Kids[0], reqVar, depth+1)
		thenCode := generateJSStatement(ir.Kids[1], reqVar, depth+1)
		if len(ir.Kids) == 2 {
			return fmt.Sprintf("if (%s) {\n%s\n}", condExpr, thenCode)
		}
		elseCode := generateJSStatement(ir.Kids[2], reqVar, depth+1)
		return fmt.Sprintf("if (%s) {\n%s\n} else {\n%s\n}", condExpr, thenCode, elseCode)
	case "while":
		condExpr := generateJSExpression(ir.Kids[0], reqVar, depth+1)
		bodyCode := generateJSStatement(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("while (%s) {\n%s\n}", condExpr, bodyCode)
	case "do":
		var stmts string
		for _, kid := range ir.Kids {
			stmts += generateJSStatement(kid, reqVar, depth+1) + ";\n"
		}
		return fmt.Sprintf("{\n%s}", stmts)
	case "set":
		varStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		valStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("%s = %s", varStr, valStr)
	case "match":
		varStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		var casesStr string
		for _, c := range ir.Cases {
			caseValStr := c.Label.Value
			if c.Label.Type == "STRING" {
				caseValStr = fmt.Sprintf("%q", caseValStr)
			}
			caseBodyCode := generateJSStatement(c.Body, reqVar, depth+1)
			if c.IsDefault {
				casesStr += fmt.Sprintf("default:\n%s;\nbreak;\n", caseBodyCode)
			} else {
				casesStr += fmt.Sprintf("case %s:\n%s;\nbreak;\n", caseValStr, caseBodyCode)
			}
		}
		return fmt.Sprintf("switch (%s) {\n%s}", varStr, casesStr)
	case "sleep":
		msStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("(await new Promise(r => setTimeout(r, %s)))", msStr)
	case "to_int":
		jsNumeric = true
		valStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("howlFrameToInt(%s)", valStr)
	case "to_float":
		jsNumeric = true
		valStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("howlFrameToFloat(%s)", valStr)
	case "to_string", "bytes_to_string":
		valStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("((_v) => _v == null ? \"\" : String(_v))(%s)", valStr)
	case "encode_json":
		jsNeedsEncodeJSON = true
		valStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("howlFrameEncodeJSON(%s)", valStr)
	case "html_escape", "attr_escape":
		jsNeedsHTMLEscape = true
		valStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("howlFrameHTMLEscape(%q, %s)", ir.Kind, valStr)
	case "str_split":
		sStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		sepStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("(%s).split(%s)", sStr, sepStr)
	case "str_join":
		listStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		sepStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("(%s).join(%s)", listStr, sepStr)
	case "regex_match":
		patStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		sStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("new RegExp(%s).test(%s)", patStr, sStr)
	case "append":
		jsNeedsCollection = true
		listNode := ir.Kids[0]
		itemStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("%s = howlFrameAppend(%s, %s)", listNode.Value, listNode.Value, itemStr)
	case "map_set":
		jsNeedsCollection = true
		dictNode := ir.Kids[0]
		keyStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		valStr := generateJSStatementRaw(ir.Kids[2], reqVar, depth+1)
		return fmt.Sprintf("howlFrameMapSet(%s, %s, %s)", dictNode.Value, keyStr, valStr)
	case "map_delete":
		jsNeedsCollection = true
		dictNode := ir.Kids[0]
		keyStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("howlFrameMapDelete(%s, %s)", dictNode.Value, keyStr)
	case "map_get":
		jsNeedsCollection = true
		dictNode := ir.Kids[0]
		keyStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		// Every map_get rejects a non-dict. The miss sentinel stays "".
		// Indexing that sentinel must not become another miss.
		dictStr := dictNode.Value
		if dictNode.Type != "SYMBOL" {
			dictStr = generateJSStatementRaw(dictNode, reqVar, depth+1)
		}
		return fmt.Sprintf("howlFrameMapGet(%s, %s)", dictStr, keyStr)
	case "list_get":
		jsNeedsCollection = true
		listNode := ir.Kids[0]
		idxStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("howlFrameListGet(%s, %s)", listNode.Value, idxStr)
	case "list_len":
		jsNeedsCollection = true
		listStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("howlFrameListLen(%s)", listStr)
	case "map_keys":
		jsNeedsMapKeyOrder = true
		dictStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		// Object.keys order is insertion order. howlFrameCompareUTF8 is
		// UTF-8 byte order, matching sort.Strings on the VM and the Go
		// backend. The default sort is UTF-16 code-unit order.
		return fmt.Sprintf("(function(_d){ if (_d === null || typeof _d !== \"object\" || Array.isArray(_d)) { throw new Error(\"TYPE_ERROR: map_keys expected dict\"); } return Object.keys(_d).sort(howlFrameCompareUTF8); })(%s)", dictStr)
	case "is_nil":
		valStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("(%s === null || %s === undefined)", valStr, valStr)
	case "time_now":
		// Unix seconds, matching the bytecode and Go backends. A web_app that
		// renders relative timestamps has no other way to read the clock.
		return "Math.floor(Date.now() / 1000)"
	case "list":
		var items []string
		for _, kid := range ir.Kids {
			if kid.Type == "STRING" {
				items = append(items, fmt.Sprintf("%q", kid.Value))
			} else {
				items = append(items, generateJSExpression(kid, reqVar, depth+1))
			}
		}
		return fmt.Sprintf("[%s]", strings.Join(items, ", "))
	case "dict":
		var pairs []string
		for _, kid := range ir.Kids {
			if kid.Type != "List" || len(kid.Children) != 2 {
				// ast.ReportError("dict expects (k v) pairs", kid.Line, kid.Column)
			}
			k := jsDictKey(kid.Children[0])
			v := kid.Children[1].Value
			if kid.Children[1].Type == "STRING" {
				v = fmt.Sprintf("%q", v)
			} else {
				v = generateJSExpression(kid.Children[1], reqVar, depth+1)
			}
			pairs = append(pairs, fmt.Sprintf("%s: %s", k, v))
		}
		return fmt.Sprintf("{%s}", strings.Join(pairs, ", "))
	case "print":
		var args []string
		for _, kid := range ir.Kids {
			// null (a map_get miss) renders as "", matching vm.TextOf.
			argStr := generateJSStatementRaw(kid, reqVar, depth+1)
			if kid.Type != "STRING" && kid.Type != "INT" && kid.Type != "FLOAT" {
				argStr = fmt.Sprintf("((_v) => _v == null ? \"\" : _v)(%s)", argStr)
			}
			args = append(args, argStr)
		}
		return fmt.Sprintf("console.log(%s)", strings.Join(args, ", "))
	}
	// ast.ReportError(fmt.Sprintf("Unknown IR kind for JS: %s", ir.Kind), 0, 0)
	return ""
}

func GenerateJSCode(node *ast.Node) (string, string) {
	ast.Desugar(node)
	jsNeedsRequestRead = false
	jsNeedsHTMLEscape = false
	jsNeedsEncodeJSON = false
	jsNeedsEq = false
	jsNeedsArchive = false
	jsNeedsExit = false
	jsNeedsDOM = false
	jsNeedsRichtext = false
	jsNeedsBrowserIO = false
	jsNeedsMotion = false
	jsNeedsCollection = false
	jsNeedsMapKeyOrder = false
	jsNeedsEnv = false
	jsNeedsExec = false
	jsNeedsReadFile = false
	jsNeedsFetch = false
	jsNeedsWriteFile = false
	jsNeedsMkdir = false
	jsNumeric = false
	if node.Type != "List" || len(node.Children) == 0 {
		// ast.ReportError("Expected list at root", node.Line, node.Column)
	}
	head := node.Children[0]
	if head.Type != "SYMBOL" || head.Value != "web_app" {
		// ast.ReportError("Expected web_app as root symbol", head.Line, head.Column)
	}

	var funcsCode string
	var appCode string
	var testCode string

	handlers := flattenModulesJS(node.Children[1:])
	for i := 0; i < len(handlers); i++ {
		handlerNode := handlers[i]
		if handlerNode.Type != "List" || len(handlerNode.Children) == 0 {
			appCode += generateJSStatement(handlerNode, "", 0) + "\n"
			continue
		}

		headVal := handlerNode.Children[0].Value

		if headVal == "intent" {
			continue
		}

		if headVal == "test" {
			if len(handlerNode.Children) < 3 {
				// ast.ReportError(`test expects (test "description" body...)`, handlerNode.Line, handlerNode.Column)
			}
			descNode := handlerNode.Children[1]
			if descNode.Type != "STRING" {
				// ast.ReportError("test description must be a string", descNode.Line, descNode.Column)
			}
			desc := descNode.Value
			var testBodyCode string
			for j := 2; j < len(handlerNode.Children); j++ {
				testBodyCode += generateJSStatement(handlerNode.Children[j], "", 0) + "\n"
			}
			testCode += fmt.Sprintf("test(%q, async (t) => {\n%s\n});\n\n", desc, testBodyCode)
			continue
		}

		if headVal == "defun" {
			if len(handlerNode.Children) < 4 {
				// ast.ReportError("defun expects (defun name (args) body)", handlerNode.Line, handlerNode.Column)
			}
			name := sanitizeJSName(handlerNode.Children[1].Value)
			argsNode := handlerNode.Children[2]

			argsStr := strings.Join(ast.ParamNames(argsNode), ", ")

			var bodyParts []string
			for _, form := range ast.DefunBody(handlerNode) {
				bodyParts = append(bodyParts, generateJSStatement(form, "", 0)+";")
			}
			bodyCode := strings.Join(bodyParts, "\n")
			funcsCode += fmt.Sprintf("async function %s(%s) {\n%s\n}\n\n", name, argsStr, bodyCode)
			continue
		}

		appCode += generateJSStatement(handlerNode, "", 0) + "\n"
	}

	// Top-level statements are the application's bootstrap and routinely
	// contain awaited calls. A classic <script> has no top-level await, so they
	// run inside an async IIFE. Function declarations stay at top level, which
	// keeps them reachable as globals for inline event handlers.
	code := funcsCode
	if strings.TrimSpace(appCode) != "" {
		if jsNeedsEnv || jsNeedsExec || jsNeedsReadFile || jsNeedsFetch || jsNeedsWriteFile || jsNeedsMkdir {
			// A denied env read, exec, read_file, fetch, write_file, or mkdir throws.
			// The async IIFE would otherwise turn that into an unhandled rejection
			// and a zero exit. try_let still catches the throw before it reaches
			// this handler.
			code += fmt.Sprintf(";(async () => {\n%s\n})().catch((err) => {\n  console.error(err && err.message ? err.message : err);\n  if (typeof process !== \"undefined\" && typeof process.exit === \"function\") {\n    process.exit(1);\n  } else {\n    throw err;\n  }\n});\n", appCode)
		} else {
			code += fmt.Sprintf(";(async () => {\n%s\n})();\n", appCode)
		}
	}

	if testCode != "" {
		testCode = "const test = require('node:test');\n" +
			"const assert = require('node:assert');\n\n" +
			funcsCode + testCode
	}

	if jsNeedsRequestRead {
		helper := requestReadJSHelper()
		code = helper + code
		if testCode != "" {
			testCode = helper + testCode
		}
	}
	if jsNeedsEncodeJSON {
		helper := encodeJSONJSHelper()
		code = helper + code
		if testCode != "" {
			testCode = helper + testCode
		}
	}
	if jsNeedsEq {
		helper := eqJSHelper()
		code = helper + code
		if testCode != "" {
			testCode = helper + testCode
		}
	}
	if jsNeedsHTMLEscape {
		helper := htmlEscapeJSHelper()
		code = helper + code
		if testCode != "" {
			testCode = helper + testCode
		}
	}
	if jsNeedsExit {
		code = exitJSHelper(false) + code
		if testCode != "" {
			testCode = exitJSHelper(true) + testCode
		}
	}
	if jsNeedsBrowserIO {
		code = browserIOJSHelper() + code
		if testCode != "" {
			testCode = browserIOJSHelper() + testCode
		}
	}
	if jsNeedsMotion {
		code = motionJSHelper() + code
		if testCode != "" {
			testCode = motionJSHelper() + testCode
		}
	}
	if jsNeedsDOM {
		helper := domJSHelper()
		if jsNeedsRichtext {
			helper += richtextJSHelper()
		}
		code = helper + code
		if testCode != "" {
			testCode = helper + testCode
		}
	}
	if jsNeedsArchive {
		helper := archiveJSHelper()
		code = helper + code
		if testCode != "" {
			testCode = helper + testCode
		}
	}
	if jsNeedsCollection {
		helper := collectionJSHelper()
		code = helper + code
		if testCode != "" {
			testCode = helper + testCode
		}
	}
	if jsNeedsMapKeyOrder {
		helper := mapKeyOrderJSHelper()
		code = helper + code
		if testCode != "" {
			testCode = helper + testCode
		}
	}
	if jsNumeric {
		helper := numericJSHelper()
		code = helper + code
		if testCode != "" {
			testCode = helper + testCode
		}
	}
	if helper := jsHostHelpers(); helper != "" {
		code = helper + code
		if testCode != "" {
			testCode = helper + testCode
		}
	}

	return code, testCode
}

func generateJSStatement(node *ast.Node, reqVar string, depth int) string {
	code := generateJSStatementRaw(node, reqVar, depth)
	if node.Type != "List" || len(node.Children) == 0 {
		return code
	}
	head := node.Children[0].Value
	switch head {
	case "return", "let", "do", "try_let", "spawn", "spawn_agent", "task", "if", "print", "for", "sleep", "while", "match", "set", "call":
		if node.Filename != "" {
			return fmt.Sprintf("//line %s:%d\n%s", node.Filename, node.Line, code)
		}
	}
	return code
}

func generateJSExpression(node *ast.Node, reqVar string, depth int) string {
	return generateJSStatementRaw(node, reqVar, depth)
}

func generateJSStatementRaw(node *ast.Node, reqVar string, depth int) string {
	if depth > 1000 {
		// ast.ReportError("AST too deep", node.Line, node.Column)
	}
	if node.Type == "STRING" {
		return fmt.Sprintf("%q", node.Value)
	}
	if node.Type == "SYMBOL" || node.Type == "INT" || node.Type == "FLOAT" {
		return node.Value
	}
	if node.Type != "List" || len(node.Children) == 0 {
		// ast.ReportError("Expected list for statement", node.Line, node.Column)
	}
	head := node.Children[0].Value
	if head == "intent" {
		return ""
	}
	if ir, ok := ir.LowerShared(node); ok {
		return EmitJSIR(ir, reqVar, depth)
	}
	if head == valueBlockHead {
		return jsValueBlock(node, reqVar, depth)
	}
	if call, ok := jsDOMCall(head, node, reqVar, depth); ok {
		return call
	}
	if call, ok := jsMotionCall(head, node, reqVar, depth); ok {
		return call
	}
	// to_lower, to_upper and str_trim are not shared IR kinds, so they are
	// emitted here rather than in EmitJSIR.
	switch head {
	case "to_lower":
		return fmt.Sprintf("String(%s).toLowerCase()", generateJSStatementRaw(node.Children[1], reqVar, depth+1))
	case "to_upper":
		return fmt.Sprintf("String(%s).toUpperCase()", generateJSStatementRaw(node.Children[1], reqVar, depth+1))
	case "str_trim":
		return fmt.Sprintf("String(%s).trim()", generateJSStatementRaw(node.Children[1], reqVar, depth+1))
	case "stderr":
		jsNeedsExit = true
		return fmt.Sprintf("howlFrameStderr(%s)", generateJSStatementRaw(node.Children[1], reqVar, depth+1))
	case "exit":
		jsNeedsExit = true
		where := fmt.Sprintf("%s:%d", node.Filename, node.Line)
		return fmt.Sprintf("howlFrameExit(%s, %q)", generateJSStatementRaw(node.Children[1], reqVar, depth+1), where)
	case "parse_json":
		// Bindings special-case parse_json above; this covers every other
		// value position, such as the result of a value block.
		jsNumeric = true
		return fmt.Sprintf("howlFrameParseJSON(%s)", generateJSStatementRaw(node.Children[2], reqVar, depth+1))
	}
	if head == "time_now" {
		// Unix seconds, matching the bytecode VM and the Go backend.
		return "Math.floor(Date.now() / 1000)"
	} else if head == "on_event" {
		if len(node.Children) != 4 {
			// ast.ReportError("on_event expects (on_event el event lambda)", node.Line, node.Column)
		}
		el := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		event := generateJSStatementRaw(node.Children[2], reqVar, depth+1)
		lambda := node.Children[3]
		if lambda.Type != "List" || len(lambda.Children) != 3 || lambda.Children[0].Value != "lambda" {
			// ast.ReportError("on_event expects a lambda", lambda.Line, lambda.Column)
		}
		args := lambda.Children[1].Children
		argName := "e"
		if len(args) > 0 {
			argName = args[0].Value
		}
		body := generateJSStatement(lambda.Children[2], reqVar, depth+1)
		// The trailing semicolon is required: without it a following statement
		// that begins with "(" is parsed as a call of this expression's result
		// rather than as its own statement.
		jsNeedsDOM = true
		return fmt.Sprintf("howlFrameOnEvent(%s, %s, async (%s) => {\n%s\n});", el, event, argName, body)
	} else if head == "url_encode" {
		value := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		return fmt.Sprintf("(function(v){%s})(%s)", urlEncodeJS, value)
	} else if head == "dom_text" {
		jsNeedsArchive = true
		el := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		return fmt.Sprintf("howlFrameDomText(%s)", el)
	} else if head == "zip_build" {
		jsNeedsArchive = true
		entries := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		return fmt.Sprintf("howlFrameZipBuild(%s)", entries)
	} else if head == "download_file" {
		jsNeedsArchive = true
		name := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		mime := generateJSStatementRaw(node.Children[2], reqVar, depth+1)
		data := generateJSStatementRaw(node.Children[3], reqVar, depth+1)
		return fmt.Sprintf("howlFrameDownloadFile(%s, %s, %s)", name, mime, data)
	} else if head == "fetch" {
		return jsFetchCall(node, reqVar, depth)
	} else if head == "spawn_agent" {
		if len(node.Children) != 3 {
			// Child grant attenuation is enforced only by the bytecode runtime.
			ast.ReportError("UNSUPPORTED_ON_TARGET: spawn_agent with (grants ...) is enforced only by the bytecode runtime", node.Line, node.Column)
		}
		agentNameStr := generateJSExpression(node.Children[1], reqVar, depth+1)
		taskDescStr := generateJSExpression(node.Children[2], reqVar, depth+1)
		return fmt.Sprintf(";(async () => {\n  console.log(`[Swarm JS] Spawning agent ${%s} for task: ${%s}`);\n  await new Promise(r => setTimeout(r, 100));\n  console.log(`[Swarm JS] Agent ${%s} completed task: ${%s}`);\n})();", agentNameStr, taskDescStr, agentNameStr, taskDescStr)
	} else if head == "task" {
		if len(node.Children) != 2 {
			// ast.ReportError("task expects (task desc)", node.Line, node.Column)
		}
		return generateJSExpression(node.Children[1], reqVar, depth+1)
	} else if head == "env" {
		if len(node.Children) != 2 {
			return ""
		}
		return jsEnvCall(node.Children[1], reqVar, depth)
	} else if head == "exec" {
		return jsExecCall(node, reqVar, depth)
	} else if head == "read_file" {
		return jsReadFileCall(node, reqVar, depth)
	} else if head == "write_file" {
		return jsWriteFileCall(node, reqVar, depth)
	} else if head == "mkdir" {
		return jsMkdirCall(node, reqVar, depth)
	} else if head == "req_query" || head == "req_header" || head == "req_path" {
		jsNeedsRequestRead = true
		if len(node.Children) != 3 {
			return ""
		}
		recv := generateJSExpression(node.Children[1], reqVar, depth+1)
		name := generateJSExpression(node.Children[2], reqVar, depth+1)
		return fmt.Sprintf("howlRequestRead(%q, %s, %s)", head, recv, name)
	} else if head == "schema_bridge" {
		return generateJSStatementRaw(node.Children[2], reqVar, depth+1)
	} else if head == "optimize_signature" {
		return generateJSStatementRaw(node.Children[len(node.Children)-1], reqVar, depth+1)
	}

	// ast.ReportError(fmt.Sprintf("Unknown statement for JS: %s", head), node.Line, node.Column)
	return ""
}

// specialPrefixJSSource renders capability.SpecialPrefixes, the table shared
// by every target, as [16-byte prefix, bits] pairs (IPv4 rows as mapped IPv6).
func specialPrefixJSSource() string {
	var b strings.Builder
	b.WriteString("var howlFrameSpecialPrefixes = [\n")
	for _, raw := range capability.SpecialPrefixes {
		p := netip.MustParsePrefix(raw)
		bits := p.Bits()
		if p.Addr().Is4() {
			bits += 96
		}
		a := p.Addr().As16()
		parts := make([]string, len(a))
		for i, x := range a {
			parts[i] = strconv.Itoa(int(x))
		}
		fmt.Fprintf(&b, "  [[%s], %d], // %s\n", strings.Join(parts, ","), bits, raw)
	}
	b.WriteString("];\n\nvar howlFrameRedirectSafeHeaders = [")
	for i, h := range capability.RedirectSafeHeaders {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Quote(strings.ToLower(h)))
	}
	b.WriteString("];\n\n")
	return b.String()
}

// codeLoadingEnvJSSource renders capability.CodeLoadingEnvNames,
// CodeLoadingEnvPrefixes and CodeLoadingEnvSuffixes, the refusal tables
// shared by every target.
func codeLoadingEnvJSSource() string {
	quote := func(xs []string) string {
		out := make([]string, len(xs))
		for i, x := range xs {
			out[i] = strconv.Quote(x)
		}
		return strings.Join(out, ", ")
	}
	return "var howlFrameCodeLoadingEnvNames = [" + quote(capability.CodeLoadingEnvNames) + "];\n" +
		"var howlFrameCodeLoadingEnvPrefixes = [" + quote(capability.CodeLoadingEnvPrefixes) + "];\n" +
		"var howlFrameCodeLoadingEnvSuffixes = [" + quote(capability.CodeLoadingEnvSuffixes) + "];\n\n"
}
