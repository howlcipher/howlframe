// Package construct is the single authoritative description of which HowlFrame
// language constructs the standalone bytecode target can actually execute.
//
// It exists because internal/bytecode's compileNode switch used to have no
// default case: any head symbol it did not recognize compiled to zero
// instructions, silently, with exit code 0 (bugs.md #45). Nothing downstream
// could catch that - the VM's opcode dispatch fails closed, but no instruction
// was ever emitted to dispatch on.
//
// This package is deliberately backend-independent, exactly like
// internal/capability (the precedent established by improvements.md #94): it
// imports only internal/ast, and nothing here may import internal/bytecode,
// internal/hfir, or any backend. Consumers wire it in themselves -
// internal/hfir/constructs.go turns Scan's violations into HFIR diagnostics, and
// internal/bytecode keeps an independent fail-closed default as a backstop.
package construct

import (
	"fmt"
	"sort"

	"github.com/howlcipher/howlframe/internal/ast"
)

// Support is how the standalone bytecode target handles a construct.
type Support int

const (
	// Supported means internal/bytecode's compileNode has a real case for
	// this head and emits instructions the VM can execute.
	Supported Support = iota

	// CompileTimeOnly means the construct legitimately emits no
	// instructions: it is either a pure annotation with no runtime meaning,
	// or a form that an earlier pass consumes before lowering ever sees it.
	// These must keep compiling and running unchanged.
	CompileTimeOnly

	// Unsupported means the construct has real runtime meaning that the
	// bytecode target cannot express. Compiling it must fail closed rather
	// than silently drop it.
	Unsupported
)

func (s Support) String() string {
	switch s {
	case Supported:
		return "Supported"
	case CompileTimeOnly:
		return "CompileTimeOnly"
	case Unsupported:
		return "Unsupported"
	}
	return "Unknown"
}

// Entry is one construct's classification for the bytecode target.
type Entry struct {
	// Name is the canonical HowlFrame head symbol.
	Name string

	// Support is the classification.
	Support Support

	// Tracker names the backlog item that owns closing this gap, for
	// Unsupported entries that have one. Empty when no item owns it.
	Tracker string

	// Note is the one-line evidence for the classification.
	Note string

	// Opaque reports that this construct's children are not compiled as
	// nested constructs, so Scan must not descend into them. Verified
	// against compileNode: achieve stringifies its operands via
	// ast.Stringify, and the others read child .Value fields directly.
	Opaque bool
}

// table is kept sorted by Name so Table() is deterministic without re-sorting
// a copy on every call. construct_test.go enforces the ordering.
var table = []Entry{
	{Name: "!=", Support: Supported, Note: "compileNode binop case"},
	{Name: "*", Support: Supported, Note: "compileNode binop case"},
	{Name: "+", Support: Supported, Note: "compileNode binop case"},
	{Name: "-", Support: Supported, Note: "compileNode binop case"},
	{Name: "/", Support: Supported, Note: "compileNode binop case"},
	{Name: "<", Support: Supported, Note: "compileNode binop case"},
	{Name: "<=", Support: Supported, Note: "compileNode binop case"},
	{Name: "=", Support: Supported, Note: "compileNode binop case, normalized to =="},
	{Name: "==", Support: Supported, Note: "compileNode binop case"},
	{Name: ">", Support: Supported, Note: "compileNode binop case"},
	{Name: ">=", Support: Supported, Note: "compileNode binop case"},
	{Name: "achieve", Support: Supported, Opaque: true, Note: "ACHIEVE; operands are stringified via ast.Stringify, not compiled"},
	{Name: "and", Support: Supported, Note: "compileNode binop case"},
	{Name: "append", Support: Supported, Note: "APPEND"},
	{Name: "assert", Support: CompileTimeOnly, Note: "density sugar; expanded to if/stderr/exit by ast.Desugar before lowering"},
	{Name: "assert_semantic", Support: Unsupported, Note: "LLM intent assertion; no opcode, Go backend only"},
	{Name: "attr_escape", Support: Supported, Note: "ATTR_ESCAPE; same encoding as html_escape, named for attribute context"},
	{Name: "blank", Support: CompileTimeOnly, Note: "density sugar; (blank x) → (= x \"\") by ast.Desugar"},
	{Name: "bytes_to_string", Support: Supported, Note: "CONVERT"},
	{Name: "call", Support: Supported, Note: "CALL"},
	{Name: "cli_app", Support: Supported, Note: "compiles every child in sequence"},
	{Name: "cli_args", Support: Supported, Note: "CLI_ARGS / CLI_ARGS_GET"},
	{Name: "cond", Support: CompileTimeOnly, Note: "density sugar; expanded to nested if by ast.Desugar before lowering"},
	{Name: "confidence", Support: Supported, Note: "CONFIDENCE"},
	{Name: "db_connect", Support: Supported, Opaque: true, Note: "DB_CONNECT; reads child .Value directly"},
	{Name: "dec", Support: CompileTimeOnly, Note: "density sugar; (dec x) → (set x (- x 1)) by ast.Desugar"},
	{Name: "defun", Support: Supported, Note: "registers a BCFunction; the name and parameter list are structural"},
	{Name: "dict", Support: Supported, Note: "MAKE_DICT; key/value pairs are destructured by the case"},
	{Name: "do", Support: Supported, Note: "compiles every child in sequence"},
	{Name: "encode_json", Support: Supported, Note: "ENCODE_JSON"},
	{Name: "ends_with", Support: CompileTimeOnly, Note: "density sugar; suffix test via sentinel+str_contains by ast.Desugar"},
	{Name: "env", Support: Supported, Note: "ENV"},
	{Name: "ephemeral_circuit", Support: Supported, Note: "EPHEMERAL_CIRCUIT"},
	{Name: "eq_ci", Support: CompileTimeOnly, Note: "density sugar; case-insensitive = via to_lower by ast.Desugar"},
	{Name: "exec", Support: Supported, Note: "EXEC"},
	{Name: "exit", Support: Supported, Note: "EXIT"},
	{Name: "export", Support: CompileTimeOnly, Note: "unwrapped by ast.ResolveModules inside a top-level (module ...) child before checker.Check or lowering ever run (improvements.md #95); a surviving export means the source is malformed and fails earlier, in checker.Check"},
	{Name: "fetch", Support: Supported, Note: "FETCH"},
	{Name: "for", Support: Supported, Note: "FOR_INIT / FOR_NEXT"},
	{Name: "fuzzy_cast", Support: Unsupported, Note: "LLM struct coercion; no opcode, Go backend only"},
	{Name: "getd", Support: CompileTimeOnly, Note: "density sugar; (getd m k d) → map_has gated map_get default by ast.Desugar"},
	{Name: "go_import", Support: Unsupported, Note: "Go-backend-only directive; the standalone VM cannot honour a Go package dependency"},
	{Name: "html_escape", Support: Supported, Note: "HTML_ESCAPE; encodes &, <, >, \", and ' with no capability"},
	{Name: "http_body", Support: CompileTimeOnly, Note: "density sugar; (http_body r) → bytes_to_string of map_get body by ast.Desugar"},
	{Name: "http_request", Support: Supported, Note: "HTTP_REQUEST; returns dict (status int, body bytes); optional body sent"},
	{Name: "http_res", Support: CompileTimeOnly, Note: "density sugar; (http_res r) → res status+json body from http_request result by ast.Desugar"},
	{Name: "http_server", Support: Supported, Note: "HTTP_SERVER_START / HTTP_ROUTE / HTTP_SERVER_SERVE"},
	{Name: "http_status", Support: CompileTimeOnly, Note: "density sugar; (http_status r) → map_get status by ast.Desugar"},
	{Name: "if", Support: Supported, Note: "JUMP_IF_FALSE / JUMP"},
	{Name: "import", Support: Unsupported, Note: "Go-backend-only directive; same reasoning as go_import"},
	{Name: "inc", Support: CompileTimeOnly, Note: "density sugar; (inc x) → (set x (+ x 1)) by ast.Desugar"},
	{Name: "include", Support: CompileTimeOnly, Note: "expanded by parser.ExpandIncludes before lowering"},
	{Name: "is_nil", Support: Supported, Note: "IS_NIL"},
	{Name: "lazy_synthesize", Support: Supported, Opaque: true, Note: "registers a function; children are name, params and docstring"},
	{Name: "let", Support: Supported, Note: "STORE_VAR"},
	{Name: "lines", Support: CompileTimeOnly, Note: "density sugar; expanded to str_split/bytes_to_string by ast.Desugar before lowering"},
	{Name: "list", Support: Supported, Note: "MAKE_LIST"},
	{Name: "list_contains", Support: CompileTimeOnly, Note: "density sugar; list membership scan via for/set by ast.Desugar"},
	{Name: "list_get", Support: Supported, Note: "LIST_GET"},
	{Name: "list_len", Support: Supported, Note: "LIST_LEN; list, string, or bytes"},
	{Name: "llm_generate", Support: Supported, Note: "LLM_GENERATE"},
	{Name: "map_delete", Support: Supported, Note: "MAP_DELETE"},
	{Name: "map_get", Support: Supported, Note: "MAP_GET"},
	{Name: "map_has", Support: CompileTimeOnly, Note: "density sugar; dict key membership via map_keys by ast.Desugar"},
	{Name: "map_keys", Support: Supported, Note: "MAP_KEYS; keys in UTF-8 byte order (sort.Strings), including non-BMP"},
	{Name: "map_set", Support: Supported, Note: "MAP_SET"},
	{Name: "match", Support: Unsupported, Note: "no opcode; the interpreter also fails closed on it (internal/vm/vm.go); use switch"},
	{Name: "max", Support: CompileTimeOnly, Note: "density sugar; (max a b) → let+if comparison by ast.Desugar"},
	{Name: "middleware", Support: Unsupported, Note: "HTTP middleware chain; no opcode, Go backend only"},
	{Name: "min", Support: CompileTimeOnly, Note: "density sugar; (min a b) → let+if comparison by ast.Desugar"},
	{Name: "mkdir", Support: Supported, Note: "MKDIR"},
	{Name: "module", Support: CompileTimeOnly, Note: "parser.ExpandIncludes/ast.ResolveModules consume (module ...) children of the root before checker.Check or lowering ever run (improvements.md #95); a root that is itself a module (no importer) is a malformed standalone program and fails earlier, in checker.Check"},
	{Name: "neural_circuit", Support: Supported, Note: "NEURAL_CIRCUIT"},
	{Name: "not", Support: CompileTimeOnly, Note: "density sugar; (not x) → boolean negation by ast.Desugar"},
	{Name: "nz", Support: CompileTimeOnly, Note: "density sugar; (nz x d) → nil-coalesce to d by ast.Desugar"},
	{Name: "optimize_block", Support: Supported, Note: "compiles children[3:]"},
	{Name: "optimize_signature", Support: Supported, Note: "compiles the final child; the preceding metric/test/candidate forms are metadata"},
	{Name: "or", Support: Supported, Note: "compileNode binop case"},
	{Name: "parse_json", Support: Supported, Opaque: true, Note: "PARSE_JSON; reads child .Value directly"},
	{Name: "patch", Support: CompileTimeOnly, Note: "applied and removed by ast.ApplyPatches before lowering"},
	{Name: "print", Support: Supported, Note: "PRINT"},
	{Name: "read_file", Support: Supported, Note: "READ_FILE"},
	{Name: "read_line", Support: Supported, Note: "READ_LINE"},
	{Name: "read_text", Support: CompileTimeOnly, Note: "density sugar; expanded to bytes_to_string/read_file by ast.Desugar before lowering"},
	{Name: "regex_match", Support: Supported, Note: "REGEX_MATCH"},
	{Name: "req_header", Support: Supported, Note: "HTTP_REQ_HEADER; pure read of an already-accepted request"},
	{Name: "req_method", Support: Supported, Note: "REQ_METHOD"},
	{Name: "req_path", Support: Supported, Note: "HTTP_REQ_PATH; pure read of a {name} path segment"},
	{Name: "req_query", Support: Supported, Note: "HTTP_REQ_QUERY; pure read of an already-accepted request"},
	{Name: "res", Support: Supported, Note: "RES"},
	{Name: "res_header", Support: Supported, Note: "RES_HEADER"},
	{Name: "res_json", Support: Supported, Note: "RES_JSON"},
	{Name: "return", Support: Supported, Note: "RETURN"},
	{Name: "schema", Support: Unsupported, Note: "declarative SQL migration; no opcode, Go backend only"},
	{Name: "schema_bridge", Support: Supported, Note: "compiles the wrapped source expression"},
	{Name: "semantic_match", Support: Unsupported, Note: "LLM intent dispatch; no opcode, Go backend only"},
	{Name: "set", Support: Supported, Note: "SET_VAR"},
	{Name: "sget", Support: CompileTimeOnly, Note: "density sugar; store string-val get via store_get/map_get by ast.Desugar"},
	{Name: "sinc", Support: CompileTimeOnly, Note: "density sugar; store string-counter increment+print by ast.Desugar"},
	{Name: "sleep", Support: Supported, Note: "SLEEP"},
	{Name: "snxt", Support: CompileTimeOnly, Note: "density sugar; next string id from store counter by ast.Desugar"},
	{Name: "spawn", Support: Supported, Note: "SPAWN"},
	{Name: "spawn_agent", Support: Supported, Note: "SPAWN_AGENT"},
	{Name: "sput", Support: CompileTimeOnly, Note: "density sugar; store string-val put via store_put/dict by ast.Desugar"},
	{Name: "sql_query", Support: Supported, Opaque: true, Note: "SQL_QUERY; reads child .Value directly"},
	{Name: "starts_with", Support: CompileTimeOnly, Note: "density sugar; prefix test via str_split/list_get by ast.Desugar"},
	{Name: "stderr", Support: Supported, Note: "STDERR"},
	{Name: "store_delete", Support: Supported, Note: "STORE_DELETE"},
	{Name: "store_get", Support: Supported, Note: "STORE_GET"},
	{Name: "store_keys", Support: Supported, Note: "STORE_KEYS"},
	{Name: "store_open", Support: Supported, Note: "STORE_OPEN; handle is a symbol, URI is a runtime string expression"},
	{Name: "store_put", Support: Supported, Note: "STORE_PUT"},
	{Name: "str", Support: CompileTimeOnly, Note: "density sugar; (str a b…) → str_join of to_string parts by ast.Desugar"},
	{Name: "str_contains", Support: CompileTimeOnly, Note: "density sugar; substring test via str_split/list_len by ast.Desugar"},
	{Name: "str_join", Support: Supported, Note: "STR_JOIN"},
	{Name: "str_replace", Support: CompileTimeOnly, Note: "density sugar; replace via str_split/str_join by ast.Desugar"},
	{Name: "str_split", Support: Supported, Note: "STR_SPLIT"},
	{Name: "str_trim", Support: Supported, Note: "CONVERT; strings.TrimSpace"},
	{Name: "struct", Support: Unsupported, Note: "the VM has no struct representation; accepting the declaration would hide the gap"},
	{Name: "switch", Support: CompileTimeOnly, Note: "density sugar; (switch expr (pat body...)... (else body...)) → let+cond by ast.Desugar"},
	{Name: "task", Support: Supported, Opaque: true, Note: "TASK; reads description child .Value directly; optional body forms execute through SPAWN_AGENT"},
	{Name: "test", Support: Unsupported, Tracker: "improvements.md #96", Note: "test blocks are extracted into Go _test.go output; the VM cannot run them"},
	{Name: "time_now", Support: Supported, Note: "TIME_NOW"},
	{Name: "to_float", Support: Supported, Note: "CONVERT"},
	{Name: "to_int", Support: Supported, Note: "CONVERT"},
	{Name: "to_lower", Support: Supported, Note: "CONVERT; strings.ToLower"},
	{Name: "to_string", Support: Supported, Note: "CONVERT"},
	{Name: "to_upper", Support: Supported, Note: "CONVERT; strings.ToUpper"},
	{Name: "trace", Support: Unsupported, Note: "auto-tracing macro expanded by the Go backend; no opcode"},
	{Name: "try_let", Support: Supported, Note: "TRY_LET"},
	{Name: "type_hint", Support: CompileTimeOnly, Note: "deliberate empty case in compileNode; a pure annotation"},
	{Name: "type_hints", Support: CompileTimeOnly, Opaque: true, Note: "pure annotation; children are type names, not code"},
	{Name: "type_param", Support: CompileTimeOnly, Opaque: true, Note: "generic parameter annotation; children are type names, not code"},
	{Name: "unix_now", Support: CompileTimeOnly, Note: "density sugar; (unix_now) → (to_string (time_now)) by ast.Desugar"},
	{Name: "unless", Support: CompileTimeOnly, Note: "density sugar; (unless test body…) → if with empty then by ast.Desugar"},
	{Name: "use", Support: CompileTimeOnly, Note: "resolved by parser.ExpandIncludes/ast.ResolveModules before checker.Check or lowering ever run (improvements.md #95); nested use inside an already-used module is rejected at parse time instead"},
	{Name: "wasm_app", Support: Supported, Note: "compiles every child in sequence"},
	{Name: "web_app", Support: Supported, Note: "compiles every child in sequence"},
	{Name: "when", Support: CompileTimeOnly, Note: "density sugar; (when test body…) → if with empty else by ast.Desugar"},
	{Name: "while", Support: Supported, Note: "JUMP_IF_FALSE / JUMP"},
	{Name: "with_context", Support: CompileTimeOnly, Opaque: true, Note: "eliminated by ast.ApplyWithContext before lowering; its variable list is structural, not code"},
	{Name: "write_file", Support: Supported, Note: "WRITE_FILE"},
}

var index = func() map[string]Entry {
	m := make(map[string]Entry, len(table))
	for _, entry := range table {
		m[entry.Name] = entry
	}
	return m
}()

// Table returns every classified construct in deterministic name order.
func Table() []Entry {
	out := make([]Entry, len(table))
	copy(out, table)
	return out
}

// Lookup returns the entry for a canonical head symbol.
func Lookup(name string) (Entry, bool) {
	entry, ok := index[name]
	return entry, ok
}

// SubForm is a head symbol that only carries meaning as a child of a specific
// parent construct, and that the parent's own compileNode case destructures
// rather than dispatching on. These are why the support check has to run over
// the AST and not over HFIR node kinds: hfir.LowerAST names a node's Kind after
// the head of *every* list, so a catch clause becomes a node with Kind
// "catch" even though it never reaches compileNode in head position, and a
// deny-list over HFIR kinds would reject every try_let program (bugs.md #45's
// implementation caveat).
type SubForm struct {
	Head   string
	Parent string
}

// subForms is sorted by Head then Parent for deterministic reporting.
var subForms = []SubForm{
	{Head: "candidate", Parent: "optimize_signature"},
	{Head: "catch", Parent: "try_let"},
	{Head: "column", Parent: "schema"},
	{Head: "lambda", Parent: "route"},
	{Head: "lambda", Parent: "spawn"},
	{Head: "metric", Parent: "optimize_signature"},
	{Head: "route", Parent: "http_server"},
	{Head: "test", Parent: "optimize_signature"},
}

// SubForms returns every known parent-scoped sub-form in deterministic order.
func SubForms() []SubForm {
	out := make([]SubForm, len(subForms))
	copy(out, subForms)
	return out
}

func isSubForm(head, parent string) bool {
	for _, sub := range subForms {
		if sub.Head == head && sub.Parent == parent {
			return true
		}
	}
	return false
}

// Violation is one construct the bytecode target cannot execute, located in
// the source that declared it.
type Violation struct {
	// Name is the offending head symbol.
	Name string

	// Tracker names the backlog item that owns the gap, when one exists.
	Tracker string

	// Reason states why the construct cannot be compiled.
	Reason string

	Filename string
	Line     int
	Column   int
}

// Message renders the violation as a single human-readable sentence,
// including the owning tracker item when there is one.
func (v Violation) Message() string {
	msg := fmt.Sprintf("construct %q cannot be compiled for the standalone bytecode target: %s", v.Name, v.Reason)
	if v.Tracker != "" {
		msg += " (tracked by " + v.Tracker + ")"
	}
	return msg
}

// Scan walks a checked AST and reports every construct the bytecode target
// cannot execute, in source order.
//
// It is context-aware on purpose. A list's head is only classified when the
// list actually sits in construct position - not when it is a let binding, a
// parameter list, a dict pair, or a sub-form its parent destructures itself.
// Scan stops descending at the first offending construct so one unsupported
// form does not cascade into a violation for every identifier inside it.
func Scan(root *ast.Node) []Violation {
	var out []Violation
	scanNode(root, "", &out, ProfileDefault)
	return out
}

func scanNode(node *ast.Node, parentHead string, out *[]Violation, profile Profile) {
	if node == nil || node.Type != "List" || len(node.Children) == 0 {
		return
	}

	head := node.Children[0]
	if head.Type != "SYMBOL" {
		// A computed head, or a dict pair keyed by a literal or an
		// expression. Not a construct; its children still are.
		scanChildren(node, parentHead, out, 0, profile)
		return
	}

	// A sub-form is legal here and is destructured by its parent's own
	// compileNode case, so it is never classified - but its body is real
	// code and must still be scanned.
	if isSubForm(head.Value, parentHead) {
		scanSubForm(node, head.Value, out, profile)
		return
	}

	if profile == ProfileGoverned && !Allowed(profile, head.Value) {
		*out = append(*out, Violation{Name: head.Value, Reason: "forbidden by governed profile", Filename: node.Filename, Line: head.Line, Column: head.Column})
		return
	}
	entry, known := Lookup(head.Value)
	if !known {
		*out = append(*out, Violation{
			Name:   head.Value,
			Reason: "no bytecode lowering exists for it",
			Line:   head.Line,
			Column: head.Column,
		})
		return
	}

	switch entry.Support {
	case Unsupported:
		*out = append(*out, Violation{
			Name:    entry.Name,
			Tracker: entry.Tracker,
			Reason:  entry.Note,
			Line:    head.Line,
			Column:  head.Column,
		})
		return
	case CompileTimeOnly:
		if profile == ProfileGoverned && entry.Name == "with_context" {
			scanChildren(node, entry.Name, out, 2, profile)
			return
		}
		if entry.Opaque {
			return
		}
		scanChildren(node, entry.Name, out, 1, profile)
		return
	}

	if entry.Opaque {
		if profile == ProfileGoverned && entry.Name == "task" {
			scanChildren(node, entry.Name, out, 2, profile)
			return
		}
		if entry.Name == "parse_json" {
			scanSupported(node, entry.Name, out, profile)
		}
		return
	}
	scanSupported(node, entry.Name, out, profile)
}

// scanSupported descends into the child positions of a supported construct
// that compileNode actually compiles. The special cases below are the shapes
// where a child list is structural rather than a construct - each one mirrors
// a specific destructuring in internal/bytecode's compileNode.
func scanSupported(node *ast.Node, head string, out *[]Violation, profile Profile) {
	switch head {
	case "let":
		// (let (var val) body): the binding list's head is the variable
		// name, not a construct.
		scanBinding(node, 1, out, profile)
		if len(node.Children) > 2 {
			scanNode(node.Children[2], head, out, profile)
		}
	case "try_let":
		// (try_let (var val) (catch err body) body)
		scanBinding(node, 1, out, profile)
		scanChildren(node, head, out, 2, profile)
	case "defun":
		// (defun name (params) body...): children 1 and 2 are the name
		// and the parameter list, whose entries are identifiers or
		// type_hint annotations. If child 3 is a return type symbol
		// with subsequent body children, start at child 4.
		bodyStart := 3
		if len(node.Children) > 4 && node.Children[3].Type == "SYMBOL" {
			bodyStart = 4
		}
		scanChildren(node, head, out, bodyStart, profile)
	case "lambda":
		// (lambda (params) body...)
		scanChildren(node, head, out, 2, profile)
	case "dict":
		// (dict (key value)...): each child is a pair, not a construct.
		for _, pair := range node.Children[1:] {
			if pair == nil || pair.Type != "List" {
				continue
			}
			scanChildren(pair, head, out, 0, profile)
		}
	case "neural_circuit", "ephemeral_circuit":
		// ((args...) "instruction"): the argument list is structural,
		// but each argument is a compiled expression.
		if len(node.Children) > 1 && node.Children[1] != nil && node.Children[1].Type == "List" {
			scanChildren(node.Children[1], head, out, 0, profile)
		}
		scanChildren(node, head, out, 2, profile)
	case "parse_json":
		if len(node.Children) > 2 && node.Children[2] != nil && node.Children[2].Type != "SYMBOL" {
			*out = append(*out, Violation{
				Name:   "parse_json",
				Reason: "the second argument to parse_json must be a variable name symbol, not an expression",
				Line:   node.Children[2].Line,
				Column: node.Children[2].Column,
			})
			return
		}
	default:
		scanChildren(node, head, out, 1, profile)
	}
}

// scanSubForm descends into the body of a parent-destructured sub-form. The
// parent passed down is the sub-form's own head so nested sub-forms (a lambda
// inside a route) still resolve.
func scanSubForm(node *ast.Node, head string, out *[]Violation, profile Profile) {
	switch head {
	case "catch":
		// (catch err body)
		scanChildren(node, head, out, 2, profile)
	case "lambda":
		// (lambda (req) body)
		scanChildren(node, head, out, 2, profile)
	case "route":
		// (route "path" (lambda ...))
		scanChildren(node, head, out, 2, profile)
	case "metric", "candidate", "test", "column":
		// Pure metadata for their parent; no code inside.
	default:
		scanChildren(node, head, out, 1, profile)
	}
}

// scanBinding descends into the value half of a (var value) binding list.
func scanBinding(node *ast.Node, index int, out *[]Violation, profile Profile) {
	if len(node.Children) <= index {
		return
	}
	binding := node.Children[index]
	if binding == nil || binding.Type != "List" || len(binding.Children) != 2 {
		return
	}
	scanNode(binding.Children[1], "", out, profile)
}

func scanChildren(node *ast.Node, head string, out *[]Violation, from int, profile Profile) {
	for i := from; i < len(node.Children); i++ {
		scanNode(node.Children[i], head, out, profile)
	}
}

// SupportedNames returns every construct classified Supported, in name order.
// internal/bytecode's drift test uses it to prove the registry and
// compileNode's switch cannot diverge silently.
func SupportedNames() []string {
	var names []string
	for _, entry := range table {
		if entry.Support == Supported {
			names = append(names, entry.Name)
		}
	}
	sort.Strings(names)
	return names
}
