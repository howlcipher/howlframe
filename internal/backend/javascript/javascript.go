package javascript

import (
	"fmt"
	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/ir"
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

func collectionJSHelper() string {
	// Dicts are plain objects. Lists are arrays. A missing map_get key is
	// still "". list_get of an in-range element returns that element. An
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
  return dict[key] ?? "";
}
function howlFrameMapSet(dict, key, val) {
  if (!howlFrameIsDict(dict)) {
    throw new Error("TYPE_ERROR: map_set expected dict, got " + howlFrameValueKind(dict));
  }
  dict[key] = val;
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
  list.push(item);
  return list;
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

// grantJSHelper reads HOWLFRAME_ALLOW_CAPS, comma-separated, the same names
// as -allow-caps. An empty or unset grant denies every name.
func grantJSHelper() string {
	return `function howlFrameGrantHas(name) {
  var raw = process.env.HOWLFRAME_ALLOW_CAPS || "";
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
	return `function howlFrameExec(cmd, args) {
  if (!howlFrameGrantHas("process")) {
    throw new Error("CAPABILITY_DENIED: capability denied: process");
  }
  var result = require("child_process").spawnSync(cmd, args, { encoding: "utf8" });
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
	return `function howlFrameFetch(url, method, body) {
  if (!howlFrameGrantHas("network")) {
    throw new Error("CAPABILITY_DENIED: capability denied: network");
  }
  var init = { method: method };
  if (arguments.length >= 3) {
    init.body = body;
  }
  return fetch(url, init).then(function (r) { return r.text(); });
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
	if node == nil || (len(node.Children) != 3 && len(node.Children) != 4) {
		return ""
	}
	urlStr := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
	methodStr := generateJSStatementRaw(node.Children[2], reqVar, depth+1)
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

func EmitJSIR(ir *ir.IRNode, reqVar string, depth int) string {
	switch ir.Kind {
	case "binop":
		arg1 := generateJSExpression(ir.Kids[0], reqVar, depth+1)
		arg2 := generateJSExpression(ir.Kids[1], reqVar, depth+1)
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
							k := pair.Children[0].Value
							if pair.Children[0].Type == "STRING" {
								k = fmt.Sprintf("%q", k)
							}
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
					valStr = fmt.Sprintf("JSON.parse(%s)", bodyVar)
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
			valStr = fmt.Sprintf("JSON.parse(%s)", bodyVar)
		} else {
			valStr = generateJSStatementRaw(valNode, reqVar, depth+1)
		}

		catchNode := ir.Kids[1]
		errVar := catchNode.Children[1].Value
		catchBodyCode := generateJSStatement(catchNode.Children[2], reqVar, depth+1)
		successBodyCode := generateJSStatement(ir.Kids[2], reqVar, depth+1)

		return fmt.Sprintf("{\n\tlet %s;\n\tlet %s = null;\n\ttry {\n\t\t%s = %s;\n\t} catch (e) {\n\t\t%s = e;\n\t}\n\tif (%s !== null) {\n\t\t%s\n\t} else {\n\t\t%s\n\t}\n}", varName, errVar, varName, valStr, errVar, errVar, catchBodyCode, successBodyCode)
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
		valStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("parseInt(%s, 10)", valStr)
	case "to_float":
		valStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("parseFloat(%s)", valStr)
	case "to_string", "bytes_to_string":
		valStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("String(%s)", valStr)
	case "encode_json":
		valStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("JSON.stringify(%s)", valStr)
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
		return fmt.Sprintf("howlFrameAppend(%s, %s)", listNode.Value, itemStr)
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
			k := kid.Children[0].Value
			if kid.Children[0].Type == "STRING" {
				k = fmt.Sprintf("%q", k)
			}
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
			args = append(args, generateJSStatementRaw(kid, reqVar, depth+1))
		}
		return fmt.Sprintf("console.log(%s)", strings.Join(args, ", "))
	}
	// ast.ReportError(fmt.Sprintf("Unknown IR kind for JS: %s", ir.Kind), 0, 0)
	return ""
}

func GenerateJSCode(node *ast.Node) (string, string) {
	jsNeedsRequestRead = false
	jsNeedsHTMLEscape = false
	jsNeedsCollection = false
	jsNeedsMapKeyOrder = false
	jsNeedsEnv = false
	jsNeedsExec = false
	jsNeedsReadFile = false
	jsNeedsFetch = false
	jsNeedsWriteFile = false
	jsNeedsMkdir = false
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

			var argsList []string
			for _, arg := range argsNode.Children {
				argsList = append(argsList, arg.Value)
			}
			argsStr := strings.Join(argsList, ", ")

			bodyNode := handlerNode.Children[len(handlerNode.Children)-1]
			bodyCode := generateJSStatement(bodyNode, "", 0)
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
			code += fmt.Sprintf(";(async () => {\n%s\n})().catch((err) => {\n  console.error(err && err.message ? err.message : err);\n  process.exit(1);\n});\n", appCode)
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
	if jsNeedsHTMLEscape {
		helper := htmlEscapeJSHelper()
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
	if head == "time_now" {
		// Unix seconds, matching the bytecode VM and the Go backend.
		return "Math.floor(Date.now() / 1000)"
	} else if head == "dom_query" {
		if len(node.Children) != 2 {
			// ast.ReportError("dom_query expects (dom_query selector)", node.Line, node.Column)
		}
		selector := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		return fmt.Sprintf("document.querySelector(%s)", selector)
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
		return fmt.Sprintf("%s.addEventListener(%s, async (%s) => {\n%s\n});", el, event, argName, body)
	} else if head == "set_html" {
		if len(node.Children) != 3 {
			// ast.ReportError("set_html expects (set_html el val)", node.Line, node.Column)
		}
		el := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		val := generateJSStatementRaw(node.Children[2], reqVar, depth+1)
		return fmt.Sprintf("%s.innerHTML = %s", el, val)
	} else if head == "toggle_class" {
		if len(node.Children) != 3 {
			// ast.ReportError("toggle_class expects (toggle_class el class)", node.Line, node.Column)
		}
		el := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		cls := generateJSStatementRaw(node.Children[2], reqVar, depth+1)
		return fmt.Sprintf("%s.classList.toggle(%s)", el, cls)
	} else if head == "set_text" {
		if len(node.Children) != 3 {
			// ast.ReportError("set_text expects (set_text el val)", node.Line, node.Column)
		}
		el := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		val := generateJSStatementRaw(node.Children[2], reqVar, depth+1)
		return fmt.Sprintf("%s.textContent = %s", el, val)
	} else if head == "set_attr" {
		if len(node.Children) != 4 {
			// ast.ReportError("set_attr expects (set_attr el name val)", node.Line, node.Column)
		}
		el := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		attr := generateJSStatementRaw(node.Children[2], reqVar, depth+1)
		val := generateJSStatementRaw(node.Children[3], reqVar, depth+1)
		return fmt.Sprintf("%s.setAttribute(%s, %s)", el, attr, val)
	} else if head == "dom_value" {
		if len(node.Children) != 2 {
			// ast.ReportError("dom_value expects (dom_value el)", node.Line, node.Column)
		}
		el := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		return fmt.Sprintf("%s.value", el)
	} else if head == "fetch" {
		return jsFetchCall(node, reqVar, depth)
	} else if head == "spawn_agent" {
		if len(node.Children) != 3 {
			// ast.ReportError("spawn_agent expects (spawn_agent name task)", node.Line, node.Column)
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
