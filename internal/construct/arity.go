package construct

import (
	"fmt"

	"github.com/howlcipher/howlframe/internal/ast"
)

// Arity is the argument count a construct accepts: Min..Max arguments after
// the head symbol. Max < 0 means variadic.
type Arity struct {
	Min, Max int
}

// Variadic marks an open upper bound.
const Variadic = -1

// arities declares the argument count of every registry construct that is
// checked (#141). One table drives every backend: the CLI enforces it before
// desugaring and before any bytecode, HFIR, interpreter, Go or JS codegen, so a
// wrong arity is a coded ARITY_ERROR instead of an index-out-of-range panic.
// arity_test.go fails if a registry entry has neither an arity here
// nor an exemption in arityExempt.
var arities = map[string]Arity{
	// export and module survive ResolveModules when malformed, so they need
	// an arity (an empty one used to panic in the checker).
	"export": {1, Variadic}, "module": {1, Variadic},
	"!=": {2, 2}, "*": {2, 2}, "+": {2, 2}, "-": {2, 2}, "/": {2, 2},
	"<": {2, 2}, "<=": {2, 2}, "=": {2, 2}, "==": {2, 2}, ">": {2, 2}, ">=": {2, 2},
	"and": {2, Variadic}, "or": {2, Variadic}, // n-ary folds in desugar
	"achieve": {2, 2}, "all": {3, 3}, "any": {3, 3}, "append": {2, 2}, "assert": {1, 1},
	"attr_escape": {1, 1}, "blank": {1, 1}, "bytes_to_string": {1, 1}, "call": {1, Variadic},
	"cli_app": {0, Variadic}, "cli_args": {0, 1}, "cond": {1, Variadic}, "confidence": {1, 1},
	"count_by": {3, 3}, "db_connect": {3, 3}, "dec": {1, 1}, "defun": {3, Variadic},
	"dict": {0, Variadic}, "do": {0, Variadic}, "encode_json": {1, 1}, "ends_with": {2, 2},
	"env": {1, 1}, "ephemeral_circuit": {2, Variadic}, "exec": {1, Variadic}, "exit": {1, 1},
	"fetch": {2, 3}, "filter": {3, 3}, "for": {3, 3}, "html_escape": {1, 1},
	"http_body": {1, 1}, "http_request": {2, 3}, "http_res": {1, 1}, "http_server": {1, Variadic},
	"http_status": {1, 1}, "if": {2, Variadic}, "inc": {1, 1}, "index_of": {2, 2}, "is_nil": {1, 1},
	"lazy_synthesize": {3, 3}, "let": {2, 2}, "lines": {1, 1}, "list": {0, Variadic},
	"list_contains": {2, 2}, "list_get": {2, 2}, "list_len": {1, 1}, "list_slice": {2, 3},
	"llm_generate": {1, 2}, "map_delete": {2, 2}, "map_get": {2, 2}, "map_has": {2, 2},
	"map_keys": {1, 1}, "map_list": {3, 3}, "map_set": {3, 3}, "max": {1, 1}, "min": {1, 1},
	"mkdir": {1, 1}, "neural_circuit": {2, Variadic}, "not": {1, 1}, "nz": {2, 2},
	"optimize_block": {3, Variadic}, "optimize_signature": {1, Variadic}, "parse_json": {2, 2},
	"print": {0, Variadic}, "read_file": {1, 1}, "read_line": {0, 0}, "rand_hex": {1, 1}, "read_stdin": {0, 0}, "secure_equal": {2, 2}, "sha256_hex": {1, 1}, "read_text": {1, 1},
	"reduce": {5, 5}, "regex_match": {2, 2}, "req_header": {2, 2}, "req_method": {1, 1},
	"req_path": {2, 2}, "req_query": {2, 2}, "res": {3, 3}, "res_header": {2, 2},
	"res_json": {2, 2}, "return": {1, 1}, "reverse": {1, 1}, "schema_bridge": {2, 2},
	"set": {2, 2}, "sget": {2, 2}, "sinc": {2, 2}, "sleep": {1, 1}, "sort": {1, 1},
	"spawn": {1, 2}, "spawn_agent": {2, 3}, "sput": {3, 3}, "sql_query": {2, Variadic},
	"starts_with": {2, 2}, "stderr": {1, 1}, "store_delete": {2, 2}, "store_get": {2, 2},
	"store_keys": {1, 1}, "store_open": {2, 2}, "store_put": {3, 3}, "str": {1, Variadic},
	"str_contains": {2, 2}, "str_join": {2, 2}, "str_len": {1, 1}, "str_replace": {3, 3},
	"str_slice": {2, 3}, "str_split": {2, 2}, "str_trim": {1, 1}, "sum": {1, 1},
	"switch": {1, Variadic}, "task": {1, Variadic}, "time_now": {0, 0}, "to_float": {1, 1},
	"to_int": {1, 1}, "to_lower": {1, 1}, "to_string": {1, 1}, "to_upper": {1, 1},
	"try_let": {2, Variadic}, "unix_now": {0, 0}, "unless": {1, Variadic}, "ver_ge": {2, 2},
	"ver_gt": {2, 2}, "wasm_app": {0, Variadic}, "web_app": {0, Variadic}, "when": {1, Variadic},
	"while": {2, 2}, "write_file": {2, 2},
	// Control-flow and core built-ins (heldout-v2 follow-up).
	"%": {2, 2}, "//": {2, 2}, "break": {0, 0}, "range": {1, 2}, "max_by": {3, 3}, "min_by": {3, 3},
	"sort_by": {3, 3}, "pad_left": {2, 3}, "pad_right": {2, 3}, "str_repeat": {2, 2}, "rtrim": {1, 1},
	"ltrim": {1, 1}, "to_base": {2, 2},
	// Records and JSON sugar (heldout-v3 records slice).
	"rec": {0, Variadic}, "get": {2, Variadic}, "put": {3, Variadic}, "read_json": {1, 1}, "write_json": {2, 2},
	"pluck": {2, 2}, "select": {2, Variadic}, "grants": {0, Variadic}, "group_by": {3, 3}, "merge": {1, Variadic},
}

// arityExempt lists registry constructs with no arity entry, each with the
// reason the check does not apply.
var arityExempt = map[string]string{
	"assert_semantic": "Unsupported in the construct gate; the Go backend checks its shape (fail closed since #141)",
	"fuzzy_cast":      "Unsupported in the construct gate; the Go backend checks its shape (fail closed since #141)",
	"go_import":       "Unsupported directive; rejected by the construct gate",
	"import":          "Unsupported directive; rejected by the construct gate",
	"match":           "Unsupported: rejected by the construct gate and the interpreter",
	"middleware":      "Unsupported: rejected by the construct gate; Go backend validates its own shape",
	"schema":          "Unsupported: declarative, validated by the Go backend",
	"semantic_match":  "Unsupported in the construct gate; the Go backend checks its shape (fail closed since #141)",
	"struct":          "Unsupported: declaration shape validated by the checker",
	"test":            "Unsupported: extracted into Go _test.go output",
	"trace":           "Unsupported in the construct gate; the Go backend checks its shape (fail closed since #141)",
	"include":         "consumed by parser.ExpandIncludes before the arity check runs",
	"use":             "consumed by ast.ResolveModules before the arity check runs",
	"patch":           "consumed by ast.ApplyPatches before the arity check runs",
	"with_context":    "consumed by ast.ApplyWithContext before the arity check runs",
	"type_hint":       "pure annotation; children are type names",
	"type_hints":      "pure annotation; children are type names",
	"type_param":      "pure annotation; children are type names",
}

// LookupArity returns the declared arity of name.
func LookupArity(name string) (Arity, bool) {
	a, ok := arities[name]
	return a, ok
}

// ArityExemptions returns the exempt constructs and their reasons.
func ArityExemptions() map[string]string {
	out := make(map[string]string, len(arityExempt))
	for k, v := range arityExempt {
		out[k] = v
	}
	return out
}

// ArityViolation is one construct called with the wrong number of arguments.
type ArityViolation struct {
	Name         string
	Got          int
	Want         Arity
	Line, Column int
}

func plural(n int) string {
	if n == 1 {
		return "arg"
	}
	return "args"
}

// Message renders the coded diagnostic, e.g.
// "ARITY_ERROR: read_file expects 1 arg, got 0".
func (v ArityViolation) Message() string {
	var want string
	switch {
	case v.Want.Max == v.Want.Min:
		want = fmt.Sprintf("%d %s", v.Want.Min, plural(v.Want.Min))
	case v.Want.Max < 0:
		want = fmt.Sprintf("at least %d %s", v.Want.Min, plural(v.Want.Min))
	default:
		want = fmt.Sprintf("%d to %d args", v.Want.Min, v.Want.Max)
	}
	return fmt.Sprintf("ARITY_ERROR: %s expects %s, got %d", v.Name, want, v.Got)
}

// CheckArity walks the AST in construct position (binding lists, parameter
// lists, dict pairs, sub-forms and opaque operands are structural and are not
// checked as calls) and returns every arity violation in source order.
func CheckArity(root *ast.Node) []ArityViolation {
	var out []ArityViolation
	arityNode(root, "", &out)
	return out
}

// EnforceArity reports the first arity violation as a fail-closed diagnostic
// (JSON reason, exit 1) before anything is compiled, written or run.
func EnforceArity(root *ast.Node) {
	if vs := CheckArity(root); len(vs) > 0 {
		ast.ReportError(vs[0].Message(), vs[0].Line, vs[0].Column)
	}
}

func arityNode(node *ast.Node, parent string, out *[]ArityViolation) {
	if node == nil || node.Type != "List" || len(node.Children) == 0 {
		return
	}
	head := node.Children[0]
	if head.Type != "SYMBOL" {
		arityChildren(node, 0, out)
		return
	}
	name := head.Value
	if isSubForm(name, parent) {
		switch name {
		case "metric", "candidate", "test", "column":
			return
		case "catch", "lambda", "route":
			arityChildren(node, 2, out)
		default:
			arityChildren(node, 1, out)
		}
		return
	}
	if _, exempt := arityExempt[name]; exempt {
		return
	}
	want, known := arities[name]
	if !known {
		// A user function call or an unknown head: not a registry
		// construct, but its arguments still are code.
		arityChildren(node, 1, out)
		return
	}
	got := len(node.Children) - 1
	if got < want.Min || (want.Max >= 0 && got > want.Max) {
		*out = append(*out, ArityViolation{Name: name, Got: got, Want: want, Line: head.Line, Column: head.Column})
		return
	}
	if entry, ok := Lookup(name); ok && entry.Opaque {
		return
	}
	switch name {
	case "let", "try_let":
		if b := node.Children[1]; b != nil && b.Type == "List" {
			for i := 1; i < len(b.Children); i += 2 {
				arityNode(b.Children[i], "", out)
			}
		}
		arityChildren(node, 2, out)
		if name == "try_let" {
			return
		}
	case "defun":
		arityChildren(node, 3, out)
	case "lambda":
		arityChildren(node, 2, out)
	case "dict":
		for _, pair := range node.Children[1:] {
			if pair != nil && pair.Type == "List" {
				arityChildren(pair, 0, out)
			}
		}
	case "neural_circuit", "ephemeral_circuit":
		if a := node.Children[1]; a != nil && a.Type == "List" {
			arityChildren(a, 0, out)
		}
		arityChildren(node, 2, out)
	case "switch":
		arityNode(node.Children[1], name, out)
		for _, clause := range node.Children[2:] {
			if clause != nil && clause.Type == "List" {
				arityChildren(clause, 1, out)
			}
		}
	case "cond":
		for _, clause := range node.Children[1:] {
			if clause != nil && clause.Type == "List" {
				arityChildren(clause, 0, out)
			}
		}
	default:
		arityChildren(node, 1, out)
	}
}

func arityChildren(node *ast.Node, from int, out *[]ArityViolation) {
	head := ""
	if len(node.Children) > 0 && node.Children[0].Type == "SYMBOL" {
		head = node.Children[0].Value
	}
	for i := from; i < len(node.Children); i++ {
		arityNode(node.Children[i], head, out)
	}
}

// ArityTable returns a copy of the arity table, for the CLI tests that drive
// every entry through every backend.
func ArityTable() map[string]Arity {
	out := make(map[string]Arity, len(arities))
	for k, v := range arities {
		out[k] = v
	}
	return out
}
