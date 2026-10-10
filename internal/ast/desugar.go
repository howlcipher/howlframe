package ast

// Desugar applies density sugar that expands into existing constructs:
//   - root (do ...) becomes (cli_app ...) so programs skip the two-token root
//   - expression-bodied defun: last value form becomes (return <form>)
//   - (assert expr) becomes a failing if/stderr/exit check
//   - (fname args...) becomes (call fname args...) when fname is a defun
//
// Desugar is idempotent and mutates the tree in place. Call it after
// include/module/patch/with_context expansion and before checker.Check or
// any lowering path that must not see the sugar heads.
func Desugar(node *Node) {
	if node == nil {
		return
	}
	if node.Type == "List" && len(node.Children) > 0 &&
		node.Children[0].Type == "SYMBOL" && node.Children[0].Value == "do" {
		node.Children[0].Value = "cli_app"
	}
	funcs := map[string]bool{}
	collectDefunNames(node, funcs)
	desugarNode(node)
	rewriteAutoCalls(node, funcs)
}

func desugarNode(node *Node) {
	if node == nil || node.Type != "List" {
		return
	}
	for _, child := range node.Children {
		desugarNode(child)
	}
	if len(node.Children) == 0 || node.Children[0].Type != "SYMBOL" {
		return
	}
	switch node.Children[0].Value {
	case "defun":
		desugarDefunExprBody(node)
	case "assert":
		desugarAssert(node)
	}
}

func collectDefunNames(node *Node, funcs map[string]bool) {
	if node == nil || node.Type != "List" || len(node.Children) == 0 {
		return
	}
	if node.Children[0].Type == "SYMBOL" && node.Children[0].Value == "defun" &&
		len(node.Children) > 1 && node.Children[1].Type == "SYMBOL" {
		name := node.Children[1].Value
		if !reservedCallHeads[name] {
			funcs[name] = true
		}
	}
	for _, child := range node.Children {
		collectDefunNames(child, funcs)
	}
}

func rewriteAutoCalls(node *Node, funcs map[string]bool) {
	if node == nil || node.Type != "List" || len(node.Children) == 0 {
		return
	}
	for _, child := range node.Children {
		rewriteAutoCalls(child, funcs)
	}
	head := node.Children[0]
	if head.Type != "SYMBOL" || head.Value == "call" || reservedCallHeads[head.Value] {
		return
	}
	if !funcs[head.Value] {
		return
	}
	callSym := &Node{Type: "SYMBOL", Value: "call", Line: head.Line, Column: head.Column, Filename: head.Filename}
	node.Children = append([]*Node{callSym}, node.Children...)
}

// reservedCallHeads are list heads that must never be rewritten into (call ...),
// even if a defun shares the name. Kept local to avoid an ast→construct import.
var reservedCallHeads = map[string]bool{
	"+": true, "-": true, "*": true, "/": true,
	"<": true, ">": true, "<=": true, ">=": true,
	"==": true, "!=": true, "=": true, "and": true, "or": true,
	"assert": true, "call": true, "cli_app": true, "do": true, "defun": true,
	"dict": true, "exit": true, "for": true, "if": true, "let": true,
	"list": true, "print": true, "return": true, "set": true, "stderr": true,
	"test": true, "try_let": true, "type_hint": true, "type_hints": true,
	"type_param": true, "while": true, "http_server": true, "web_app": true,
	"wasm_app": true, "route": true, "lambda": true, "import": true,
	"module": true, "export": true, "use": true, "include": true,
	"map_get": true, "map_set": true, "map_delete": true, "map_keys": true,
	"list_get": true, "list_len": true, "append": true,
}

func desugarDefunExprBody(node *Node) {
	if len(node.Children) < 4 {
		return
	}
	bodyStart := 3
	if len(node.Children) > 4 && node.Children[3].Type == "SYMBOL" {
		if node.Children[3].Value == "void" {
			return
		}
		bodyStart = 4
	}
	for i := bodyStart; i < len(node.Children); i++ {
		if defunDeclaresVoidReturn(node.Children[i]) {
			return
		}
	}
	lastIdx := len(node.Children) - 1
	if lastIdx < bodyStart {
		return
	}
	last := node.Children[lastIdx]
	if last == nil || isAnnotationForm(last) {
		return
	}
	if isReturnForm(last) || !isExprBodyForm(last) {
		return
	}
	node.Children[lastIdx] = &Node{
		Type:     "List",
		Line:     last.Line,
		Column:   last.Column,
		Filename: last.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "return", Line: last.Line, Column: last.Column, Filename: last.Filename},
			last,
		},
	}
}

func desugarAssert(node *Node) {
	if len(node.Children) != 2 {
		ReportError("assert expects (assert expr)", node.Line, node.Column)
		return
	}
	expr := node.Children[1]
	ok := &Node{
		Type:     "List",
		Line:     node.Line,
		Column:   node.Column,
		Filename: node.Filename,
		Children: []*Node{{Type: "SYMBOL", Value: "do", Line: node.Line, Column: node.Column, Filename: node.Filename}},
	}
	fail := &Node{
		Type:     "List",
		Line:     node.Line,
		Column:   node.Column,
		Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "do", Line: node.Line, Column: node.Column, Filename: node.Filename},
			{
				Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
				Children: []*Node{
					{Type: "SYMBOL", Value: "stderr", Line: node.Line, Column: node.Column, Filename: node.Filename},
					{Type: "STRING", Value: "assertion failed\n", Line: node.Line, Column: node.Column, Filename: node.Filename},
				},
			},
			{
				Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
				Children: []*Node{
					{Type: "SYMBOL", Value: "exit", Line: node.Line, Column: node.Column, Filename: node.Filename},
					{Type: "INT", Value: "1", Line: node.Line, Column: node.Column, Filename: node.Filename},
				},
			},
		},
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "if", Line: node.Line, Column: node.Column, Filename: node.Filename},
		expr,
		ok,
		fail,
	}
}

func defunDeclaresVoidReturn(node *Node) bool {
	if node == nil || node.Type != "List" || len(node.Children) == 0 {
		return false
	}
	head := node.Children[0].Value
	switch head {
	case "type_hint":
		return len(node.Children) >= 3 && node.Children[1].Value == "return" && node.Children[2].Value == "void"
	case "type_hints":
		for _, pair := range node.Children[1:] {
			if pair != nil && pair.Type == "List" && len(pair.Children) >= 2 &&
				pair.Children[0].Value == "return" && pair.Children[1].Value == "void" {
				return true
			}
		}
	}
	return false
}

func isAnnotationForm(node *Node) bool {
	if node == nil || node.Type != "List" || len(node.Children) == 0 {
		return false
	}
	switch node.Children[0].Value {
	case "type_hint", "type_hints", "type_param":
		return true
	}
	return false
}

func isReturnForm(node *Node) bool {
	return node != nil && node.Type == "List" && len(node.Children) > 0 && node.Children[0].Value == "return"
}

// isExprBodyForm reports whether a form is safe to wrap in (return ...).
// Statement-shaped forms (print, if, let, loops, ...) stay unwrapped so Go/JS
// emitters do not emit `return if ...`.
func isExprBodyForm(node *Node) bool {
	if node == nil {
		return false
	}
	switch node.Type {
	case "SYMBOL", "STRING", "INT", "FLOAT", "NUMBER":
		return true
	case "List":
		if len(node.Children) == 0 || node.Children[0].Type != "SYMBOL" {
			return false
		}
		return exprBodyHeads[node.Children[0].Value]
	default:
		return false
	}
}

var exprBodyHeads = map[string]bool{
	"+": true, "-": true, "*": true, "/": true,
	"<": true, ">": true, "<=": true, ">=": true,
	"==": true, "!=": true, "=": true, "and": true, "or": true,
	"call": true, "list": true, "dict": true,
	"map_get": true, "list_get": true, "list_len": true, "map_keys": true,
	"to_int": true, "to_float": true, "to_string": true, "bytes_to_string": true,
	"env": true, "read_file": true, "read_line": true, "cli_args": true,
	"str_split": true, "str_join": true, "regex_match": true,
	"parse_json": true, "encode_json": true, "is_nil": true,
	"html_escape": true, "attr_escape": true, "time_now": true,
	"req_header": true, "req_method": true, "req_path": true, "req_query": true,
}
