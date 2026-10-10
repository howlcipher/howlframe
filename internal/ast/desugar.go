package ast

// Desugar applies density sugar that expands into existing constructs:
//   - root (do ...) becomes (cli_app ...) so programs skip the two-token root
//   - expression-bodied defun: last value form becomes (return <form>)
//   - (assert expr) becomes a failing if/stderr/exit check
//   - (lines expr) becomes (str_split (bytes_to_string expr) "\n")
//   - (read_text path) becomes (bytes_to_string (read_file path))
//   - (cond (test body...) ... (else body...)) becomes nested if
//   - (let (a x b y) body) becomes nested single-binding let
//   - (sget handle key) / (sput handle key val) wrap store_get/store_put
//     for the common string "val" record shape
//   - (sinc handle key) increments a string counter, stores it, and prints it
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
	desugarInContext(node, "root", 0)
	rewriteAutoCallsInContext(node, funcs, "root", 0)
}

// desugarInContext walks the tree with parent context so binding pairs and
// parameter lists are not mistaken for (lines …) / (read_text …) / (assert …).
func desugarInContext(node *Node, parentHead string, childIndex int) {
	if node == nil || node.Type != "List" {
		return
	}
	// Binding / param pairs are (name value-or-type). Never treat the name as
	// a construct; only walk the value/type side for further sugar.
	if parentHead == "binding" || parentHead == "params" {
		if len(node.Children) >= 2 {
			desugarInContext(node.Children[1], "let_value", 1)
		}
		return
	}
	head := ""
	if len(node.Children) > 0 && node.Children[0].Type == "SYMBOL" {
		head = node.Children[0].Value
	}
	// Expand multi-binding let before walking so children see single bindings.
	if head == "let" {
		desugarMultiLet(node)
		head = node.Children[0].Value
	}
	for i, child := range node.Children {
		childParent := head
		if childParent == "" {
			childParent = parentHead
		}
		switch head {
		case "defun", "lambda", "lazy_synthesize":
			if i == 2 {
				childParent = "params"
			}
		case "let", "try_let":
			if i == 1 {
				childParent = "binding"
			}
		case "dict", "type_hints", "type_hint", "type_param":
			childParent = "binding"
		}
		desugarInContext(child, childParent, i)
	}
	if head == "" {
		return
	}
	switch head {
	case "defun":
		desugarDefunExprBody(node)
	case "assert":
		if isSugarExprContext(parentHead, childIndex) {
			desugarAssert(node)
		}
	case "lines":
		if isSugarExprContext(parentHead, childIndex) {
			desugarLines(node)
		}
	case "read_text":
		if isSugarExprContext(parentHead, childIndex) {
			desugarReadText(node)
		}
	case "cond":
		if isSugarExprContext(parentHead, childIndex) {
			desugarCond(node)
		}
	case "sget":
		if isSugarExprContext(parentHead, childIndex) {
			desugarSGet(node)
		}
	case "sput":
		if isSugarExprContext(parentHead, childIndex) {
			desugarSPut(node)
		}
	case "sinc":
		if isSugarExprContext(parentHead, childIndex) {
			desugarSInc(node)
			// sinc expands into sget/sput forms; rewalk so those expand too.
			desugarInContext(node, parentHead, childIndex)
		}
	}
}

func isSugarExprContext(parentHead string, childIndex int) bool {
	switch parentHead {
	case "root", "cli_app", "do", "http_server", "web_app", "wasm_app",
		"if", "while", "print", "stderr", "return", "call", "assert", "cond",
		"+", "-", "*", "/", "=", "==", "!=", "<", ">", "<=", ">=", "and", "or",
		"list", "append", "map_set", "map_get", "map_delete", "list_get",
		"str_split", "str_join", "encode_json", "to_int", "to_float", "to_string",
		"bytes_to_string", "set", "exit", "spawn", "spawn_agent", "task",
		"res", "res_json", "route", "middleware", "optimize_block", "optimize_signature",
		"schema_bridge", "test", "let_value", "sget", "sput", "sinc",
		"store_get", "store_put", "store_delete":
		return true
	case "for":
		return childIndex == 2 || childIndex == 3
	case "let":
		return childIndex == 2
	case "try_let":
		return childIndex == 2 || childIndex == 3
	case "catch":
		return childIndex == 2
	case "params", "binding":
		return false
	default:
		return false
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

func rewriteAutoCallsInContext(node *Node, funcs map[string]bool, parentHead string, childIndex int) {
	if node == nil || node.Type != "List" || len(node.Children) == 0 {
		return
	}
	if parentHead == "binding" || parentHead == "params" {
		if len(node.Children) >= 2 {
			rewriteAutoCallsInContext(node.Children[1], funcs, "let_value", 1)
		}
		return
	}
	head := ""
	if node.Children[0].Type == "SYMBOL" {
		head = node.Children[0].Value
	}
	for i, child := range node.Children {
		childParent := head
		if childParent == "" {
			childParent = parentHead
		}
		switch head {
		case "defun", "lambda", "lazy_synthesize":
			if i == 2 {
				childParent = "params"
			}
		case "let", "try_let":
			if i == 1 {
				childParent = "binding"
			}
		case "dict", "type_hints", "type_hint", "type_param":
			childParent = "binding"
		}
		rewriteAutoCallsInContext(child, funcs, childParent, i)
	}
	if !isSugarExprContext(parentHead, childIndex) {
		return
	}
	sym := node.Children[0]
	if sym.Type != "SYMBOL" || sym.Value == "call" || reservedCallHeads[sym.Value] {
		return
	}
	if !funcs[sym.Value] {
		return
	}
	callSym := &Node{Type: "SYMBOL", Value: "call", Line: sym.Line, Column: sym.Column, Filename: sym.Filename}
	node.Children = append([]*Node{callSym}, node.Children...)
}

// reservedCallHeads are list heads that must never be rewritten into (call ...),
// even if a defun shares the name. Kept local to avoid an ast→construct import.
var reservedCallHeads = map[string]bool{
	"+": true, "-": true, "*": true, "/": true,
	"<": true, ">": true, "<=": true, ">=": true,
	"==": true, "!=": true, "=": true, "and": true, "or": true,
	"assert": true, "call": true, "cli_app": true, "do": true, "defun": true,
	"lines": true, "read_text": true, "cond": true, "sget": true, "sput": true, "sinc": true,
	"dict": true, "exit": true, "for": true, "if": true, "let": true,
	"list": true, "print": true, "return": true, "set": true, "stderr": true,
	"test": true, "try_let": true, "type_hint": true, "type_hints": true,
	"type_param": true, "while": true, "http_server": true, "web_app": true,
	"wasm_app": true, "route": true, "lambda": true, "import": true,
	"module": true, "export": true, "use": true, "include": true,
	"map_get": true, "map_set": true, "map_delete": true, "map_keys": true,
	"list_get": true, "list_len": true, "append": true,
	"store_open": true, "store_get": true, "store_put": true, "store_delete": true,
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

func desugarLines(node *Node) {
	if len(node.Children) != 2 {
		ReportError("lines expects (lines expr)", node.Line, node.Column)
		return
	}
	expr := node.Children[1]
	bytesToString := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "bytes_to_string", Line: node.Line, Column: node.Column, Filename: node.Filename},
			expr,
		},
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "str_split", Line: node.Line, Column: node.Column, Filename: node.Filename},
		bytesToString,
		{Type: "STRING", Value: "\n", Line: node.Line, Column: node.Column, Filename: node.Filename},
	}
}

func desugarReadText(node *Node) {
	if len(node.Children) != 2 {
		ReportError("read_text expects (read_text path)", node.Line, node.Column)
		return
	}
	path := node.Children[1]
	readFile := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "read_file", Line: node.Line, Column: node.Column, Filename: node.Filename},
			path,
		},
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "bytes_to_string", Line: node.Line, Column: node.Column, Filename: node.Filename},
		readFile,
	}
}

func desugarMultiLet(node *Node) {
	if len(node.Children) < 3 {
		return
	}
	binding := node.Children[1]
	if binding == nil || binding.Type != "List" {
		return
	}
	n := len(binding.Children)
	if n <= 2 || n%2 != 0 {
		return
	}
	body := node.Children[2]
	// Build innermost let first: (let (lastName lastVal) body)
	acc := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "let", Line: node.Line, Column: node.Column, Filename: node.Filename},
			{
				Type: "List", Line: binding.Line, Column: binding.Column, Filename: binding.Filename,
				Children: []*Node{binding.Children[n-2], binding.Children[n-1]},
			},
			body,
		},
	}
	for i := n - 4; i >= 0; i -= 2 {
		acc = &Node{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{
				{Type: "SYMBOL", Value: "let", Line: node.Line, Column: node.Column, Filename: node.Filename},
				{
					Type: "List", Line: binding.Line, Column: binding.Column, Filename: binding.Filename,
					Children: []*Node{binding.Children[i], binding.Children[i+1]},
				},
				acc,
			},
		}
	}
	node.Children = acc.Children
}

func desugarCond(node *Node) {
	if len(node.Children) < 2 {
		ReportError("cond expects at least one clause", node.Line, node.Column)
		return
	}
	clauses := node.Children[1:]
	var acc *Node
	for i := len(clauses) - 1; i >= 0; i-- {
		clause := clauses[i]
		if clause == nil || clause.Type != "List" || len(clause.Children) < 2 {
			ReportError("cond clause expects (test body...) or (else body...)", node.Line, node.Column)
			return
		}
		body := wrapBodyForms(clause.Children[1:], clause.Line, clause.Column, clause.Filename)
		test := clause.Children[0]
		if test != nil && test.Type == "SYMBOL" && test.Value == "else" {
			if i != len(clauses)-1 {
				ReportError("cond else clause must be last", test.Line, test.Column)
				return
			}
			acc = body
			continue
		}
		if acc == nil {
			acc = &Node{
				Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
				Children: []*Node{{Type: "SYMBOL", Value: "do", Line: node.Line, Column: node.Column, Filename: node.Filename}},
			}
		}
		acc = &Node{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{
				{Type: "SYMBOL", Value: "if", Line: node.Line, Column: node.Column, Filename: node.Filename},
				test,
				body,
				acc,
			},
		}
	}
	if acc == nil {
		return
	}
	node.Children = acc.Children
}

func wrapBodyForms(forms []*Node, line, col int, filename string) *Node {
	if len(forms) == 1 {
		return forms[0]
	}
	children := make([]*Node, 0, len(forms)+1)
	children = append(children, &Node{Type: "SYMBOL", Value: "do", Line: line, Column: col, Filename: filename})
	children = append(children, forms...)
	return &Node{Type: "List", Line: line, Column: col, Filename: filename, Children: children}
}

func desugarSGet(node *Node) {
	if len(node.Children) != 3 {
		ReportError("sget expects (sget handle key)", node.Line, node.Column)
		return
	}
	handle := node.Children[1]
	key := node.Children[2]
	tmp := &Node{Type: "SYMBOL", Value: "__sget", Line: node.Line, Column: node.Column, Filename: node.Filename}
	storeGet := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "store_get", Line: node.Line, Column: node.Column, Filename: node.Filename},
			handle,
			key,
		},
	}
	isNil := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "is_nil", Line: node.Line, Column: node.Column, Filename: node.Filename},
			tmp,
		},
	}
	mapGet := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "map_get", Line: node.Line, Column: node.Column, Filename: node.Filename},
			tmp,
			{Type: "STRING", Value: "val", Line: node.Line, Column: node.Column, Filename: node.Filename},
		},
	}
	iff := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "if", Line: node.Line, Column: node.Column, Filename: node.Filename},
			isNil,
			tmp,
			mapGet,
		},
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "let", Line: node.Line, Column: node.Column, Filename: node.Filename},
		{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{tmp, storeGet},
		},
		iff,
	}
}

func desugarSPut(node *Node) {
	if len(node.Children) != 4 {
		ReportError("sput expects (sput handle key val)", node.Line, node.Column)
		return
	}
	handle := node.Children[1]
	key := node.Children[2]
	val := node.Children[3]
	dict := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "dict", Line: node.Line, Column: node.Column, Filename: node.Filename},
			{
				Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
				Children: []*Node{
					{Type: "STRING", Value: "val", Line: node.Line, Column: node.Column, Filename: node.Filename},
					val,
				},
			},
		},
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "store_put", Line: node.Line, Column: node.Column, Filename: node.Filename},
		handle,
		key,
		dict,
	}
}

func desugarSInc(node *Node) {
	if len(node.Children) != 3 {
		ReportError("sinc expects (sinc handle key)", node.Line, node.Column)
		return
	}
	handle := node.Children[1]
	key := node.Children[2]
	// (let (k key v (sget handle k) n (+ (if (is_nil v) 0 (to_int v)) 1))
	//   (do (sput handle k (to_string n)) (print n)))
	kSym := &Node{Type: "SYMBOL", Value: "__k", Line: node.Line, Column: node.Column, Filename: node.Filename}
	vSym := &Node{Type: "SYMBOL", Value: "__v", Line: node.Line, Column: node.Column, Filename: node.Filename}
	nSym := &Node{Type: "SYMBOL", Value: "__n", Line: node.Line, Column: node.Column, Filename: node.Filename}
	sget := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "sget", Line: node.Line, Column: node.Column, Filename: node.Filename},
			handle,
			kSym,
		},
	}
	isNilV := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "is_nil", Line: node.Line, Column: node.Column, Filename: node.Filename},
			vSym,
		},
	}
	toInt := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "to_int", Line: node.Line, Column: node.Column, Filename: node.Filename},
			vSym,
		},
	}
	ifZero := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "if", Line: node.Line, Column: node.Column, Filename: node.Filename},
			isNilV,
			{Type: "INT", Value: "0", Line: node.Line, Column: node.Column, Filename: node.Filename},
			toInt,
		},
	}
	plus := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "+", Line: node.Line, Column: node.Column, Filename: node.Filename},
			ifZero,
			{Type: "INT", Value: "1", Line: node.Line, Column: node.Column, Filename: node.Filename},
		},
	}
	toString := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "to_string", Line: node.Line, Column: node.Column, Filename: node.Filename},
			nSym,
		},
	}
	sput := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "sput", Line: node.Line, Column: node.Column, Filename: node.Filename},
			handle,
			kSym,
			toString,
		},
	}
	printN := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "print", Line: node.Line, Column: node.Column, Filename: node.Filename},
			nSym,
		},
	}
	body := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "do", Line: node.Line, Column: node.Column, Filename: node.Filename},
			sput,
			printN,
		},
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "let", Line: node.Line, Column: node.Column, Filename: node.Filename},
		{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{kSym, key, vSym, sget, nSym, plus},
		},
		body,
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
	"env": true, "read_file": true, "read_line": true, "read_text": true, "cli_args": true,
	"lines": true, "str_split": true, "str_join": true, "regex_match": true,
	"parse_json": true, "encode_json": true, "is_nil": true,
	"html_escape": true, "attr_escape": true, "time_now": true,
	"req_header": true, "req_method": true, "req_path": true, "req_query": true,
}
