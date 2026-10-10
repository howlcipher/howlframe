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
//   - (switch expr (pat body...) ... (else|_/default body...)) → let+cond
//   - (inc x) → (set x (+ x 1)) for symbol x
//   - (str a b…) → (str_join (list (to_string a)…) "")
//   - (nz x d) → (let (__nz x) (if (is_nil __nz) d __nz))
//   - (map_has d k) → membership check via map_keys (bool)
//   - (when test body…) / (unless test body…) → single-branch if
//   - (and a b c…) / (or a b c…) → nested binary and/or
//   - (str_contains hay needle) → (> (list_len (str_split hay needle)) 1)
//   - (str_replace s old new) → join of split parts with new
//   - (starts_with s p) / (ends_with s p) → prefix/suffix tests
//   - (http_body r) → (bytes_to_string (map_get r "body"))
//   - (http_status r) → (map_get r "status")
//   - (http_res r) → (res status application/json body) from http_request result
//   - (list_contains lst x) → membership scan via for/set
//   - (dec x) → (set x (- x 1)) for symbol x
//   - (not x) → boolean negation via if
//   - (blank x) → (= x "")
//   - (unix_now) → (to_string (time_now))
//   - (getd m k d) → map_has gated map_get default
//   - (min a b) / (max a b) → comparison via let+if
//   - (eq_ci a b) → case-insensitive = via to_lower
//   - (snxt handle key) → next string id from store counter
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
		case "defun", "lazy_synthesize":
			// (defun name (params) body...) — params at index 2
			if i == 2 {
				childParent = "params"
			}
		case "lambda":
			// (lambda (params) body...) — params at index 1
			if i == 1 {
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
	case "switch":
		if isSugarExprContext(parentHead, childIndex) {
			desugarSwitch(node)
			desugarInContext(node, parentHead, childIndex)
		}
	case "inc", "dec":
		if isSugarExprContext(parentHead, childIndex) {
			if head == "inc" {
				desugarInc(node)
			} else {
				desugarDec(node)
			}
		}
	case "str":
		if isSugarExprContext(parentHead, childIndex) {
			desugarStr(node)
		}
	case "nz":
		if isSugarExprContext(parentHead, childIndex) {
			desugarNz(node)
		}
	case "map_has":
		if isSugarExprContext(parentHead, childIndex) {
			desugarMapHas(node)
			desugarInContext(node, parentHead, childIndex)
		}
	case "when", "unless":
		if isSugarExprContext(parentHead, childIndex) {
			desugarWhenUnless(node)
			desugarInContext(node, parentHead, childIndex)
		}
	case "and", "or":
		if isSugarExprContext(parentHead, childIndex) && len(node.Children) > 3 {
			desugarNaryLogic(node)
			desugarInContext(node, parentHead, childIndex)
		}
	case "str_contains":
		if isSugarExprContext(parentHead, childIndex) {
			desugarStrContains(node)
		}
	case "str_replace":
		if isSugarExprContext(parentHead, childIndex) {
			desugarStrReplace(node)
		}
	case "starts_with", "ends_with":
		if isSugarExprContext(parentHead, childIndex) {
			desugarStartsEndsWith(node)
			desugarInContext(node, parentHead, childIndex)
		}
	case "http_body":
		if isSugarExprContext(parentHead, childIndex) {
			desugarHTTPBody(node)
		}
	case "http_status":
		if isSugarExprContext(parentHead, childIndex) {
			desugarHTTPStatus(node)
		}
	case "http_res":
		if isSugarExprContext(parentHead, childIndex) {
			desugarHTTPRes(node)
			desugarInContext(node, parentHead, childIndex)
		}
	case "list_contains":
		if isSugarExprContext(parentHead, childIndex) {
			desugarListContains(node)
			desugarInContext(node, parentHead, childIndex)
		}
	case "not":
		if isSugarExprContext(parentHead, childIndex) {
			desugarNot(node)
		}
	case "blank":
		if isSugarExprContext(parentHead, childIndex) {
			desugarBlank(node)
		}
	case "unix_now":
		if isSugarExprContext(parentHead, childIndex) {
			desugarUnixNow(node)
		}
	case "getd":
		if isSugarExprContext(parentHead, childIndex) {
			desugarGetd(node)
			desugarInContext(node, parentHead, childIndex)
		}
	case "min", "max":
		if isSugarExprContext(parentHead, childIndex) {
			desugarMinMax(node)
			desugarInContext(node, parentHead, childIndex)
		}
	case "eq_ci":
		if isSugarExprContext(parentHead, childIndex) {
			desugarEqCi(node)
			desugarInContext(node, parentHead, childIndex)
		}
	case "snxt":
		if isSugarExprContext(parentHead, childIndex) {
			desugarSNxt(node)
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
		"store_get", "store_put", "store_delete", "http_request", "fetch",
		"switch", "inc", "dec", "str", "nz", "map_has", "when", "unless",
		"str_contains", "str_replace", "starts_with", "ends_with",
		"http_body", "http_status", "http_res", "list_contains", "not",
		"blank", "unix_now", "getd", "min", "max", "eq_ci", "snxt",
		"to_lower", "to_upper", "str_trim":
		return true
	case "for":
		return childIndex == 2 || childIndex == 3
	case "let":
		return childIndex == 2
	case "lambda":
		// Body forms start at index 2: (lambda (params) body...)
		return childIndex >= 2
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
		case "defun", "lazy_synthesize":
			if i == 2 {
				childParent = "params"
			}
		case "lambda":
			if i == 1 {
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
	"switch": true, "inc": true, "dec": true, "str": true, "nz": true, "map_has": true, "when": true, "unless": true,
	"str_contains": true, "str_replace": true, "starts_with": true, "ends_with": true,
	"http_body": true, "http_status": true, "http_res": true, "list_contains": true, "not": true,
	"blank": true, "unix_now": true, "getd": true, "min": true, "max": true, "eq_ci": true, "snxt": true,
	"dict": true, "exit": true, "for": true, "if": true, "let": true,
	"list": true, "print": true, "return": true, "set": true, "stderr": true,
	"to_lower": true, "to_upper": true, "str_trim": true,
	"test": true, "try_let": true, "type_hint": true, "type_hints": true,
	"type_param": true, "while": true, "http_server": true, "web_app": true,
	"wasm_app": true, "route": true, "lambda": true, "import": true,
	"module": true, "export": true, "use": true, "include": true,
	"map_get": true, "map_set": true, "map_delete": true, "map_keys": true,
	"list_get": true, "list_len": true, "append": true,
	"store_open": true, "store_get": true, "store_put": true, "store_delete": true,
	"http_request": true, "fetch": true, "req_method": true, "req_path": true,
	"req_query": true, "req_header": true, "res": true, "res_json": true,
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

func desugarSwitch(node *Node) {
	// (switch expr (pat body...) ... (else|default|_ body...))
	if len(node.Children) < 3 {
		ReportError("switch expects (switch expr clause...)", node.Line, node.Column)
		return
	}
	expr := node.Children[1]
	tmp := &Node{Type: "SYMBOL", Value: "__sw", Line: node.Line, Column: node.Column, Filename: node.Filename}
	cond := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{{Type: "SYMBOL", Value: "cond", Line: node.Line, Column: node.Column, Filename: node.Filename}},
	}
	for _, clause := range node.Children[2:] {
		if clause == nil || clause.Type != "List" || len(clause.Children) < 2 {
			ReportError("switch clause expects (pat body...) or (else body...)", node.Line, node.Column)
			return
		}
		pat := clause.Children[0]
		bodyForms := clause.Children[1:]
		if pat != nil && pat.Type == "SYMBOL" {
			switch pat.Value {
			case "else", "default", "_":
				elseClause := &Node{
					Type: "List", Line: clause.Line, Column: clause.Column, Filename: clause.Filename,
					Children: append([]*Node{{Type: "SYMBOL", Value: "else", Line: pat.Line, Column: pat.Column, Filename: pat.Filename}}, bodyForms...),
				}
				cond.Children = append(cond.Children, elseClause)
				continue
			}
		}
		eq := &Node{
			Type: "List", Line: clause.Line, Column: clause.Column, Filename: clause.Filename,
			Children: []*Node{
				{Type: "SYMBOL", Value: "=", Line: clause.Line, Column: clause.Column, Filename: clause.Filename},
				tmp,
				pat,
			},
		}
		cond.Children = append(cond.Children, &Node{
			Type: "List", Line: clause.Line, Column: clause.Column, Filename: clause.Filename,
			Children: append([]*Node{eq}, bodyForms...),
		})
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "let", Line: node.Line, Column: node.Column, Filename: node.Filename},
		{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{tmp, expr},
		},
		cond,
	}
}

func desugarInc(node *Node) {
	if len(node.Children) != 2 || node.Children[1] == nil || node.Children[1].Type != "SYMBOL" {
		ReportError("inc expects (inc symbol)", node.Line, node.Column)
		return
	}
	sym := node.Children[1]
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "set", Line: node.Line, Column: node.Column, Filename: node.Filename},
		sym,
		{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{
				{Type: "SYMBOL", Value: "+", Line: node.Line, Column: node.Column, Filename: node.Filename},
				{Type: "SYMBOL", Value: sym.Value, Line: sym.Line, Column: sym.Column, Filename: sym.Filename},
				{Type: "INT", Value: "1", Line: node.Line, Column: node.Column, Filename: node.Filename},
			},
		},
	}
}

func desugarDec(node *Node) {
	if len(node.Children) != 2 || node.Children[1] == nil || node.Children[1].Type != "SYMBOL" {
		ReportError("dec expects (dec symbol)", node.Line, node.Column)
		return
	}
	sym := node.Children[1]
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "set", Line: node.Line, Column: node.Column, Filename: node.Filename},
		sym,
		{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{
				{Type: "SYMBOL", Value: "-", Line: node.Line, Column: node.Column, Filename: node.Filename},
				{Type: "SYMBOL", Value: sym.Value, Line: sym.Line, Column: sym.Column, Filename: sym.Filename},
				{Type: "INT", Value: "1", Line: node.Line, Column: node.Column, Filename: node.Filename},
			},
		},
	}
}

func desugarHTTPBody(node *Node) {
	// (http_body r) → (bytes_to_string (map_get r "body"))
	if len(node.Children) != 2 {
		ReportError("http_body expects (http_body response)", node.Line, node.Column)
		return
	}
	get := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "map_get", Line: node.Line, Column: node.Column, Filename: node.Filename},
			node.Children[1],
			{Type: "STRING", Value: "body", Line: node.Line, Column: node.Column, Filename: node.Filename},
		},
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "bytes_to_string", Line: node.Line, Column: node.Column, Filename: node.Filename},
		get,
	}
}

func desugarHTTPStatus(node *Node) {
	// (http_status r) → (map_get r "status")
	if len(node.Children) != 2 {
		ReportError("http_status expects (http_status response)", node.Line, node.Column)
		return
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "map_get", Line: node.Line, Column: node.Column, Filename: node.Filename},
		node.Children[1],
		{Type: "STRING", Value: "status", Line: node.Line, Column: node.Column, Filename: node.Filename},
	}
}

func desugarHTTPRes(node *Node) {
	// (http_res r) → (let (__r r)
	//   (res (http_status __r) "application/json" (http_body __r)))
	if len(node.Children) != 2 {
		ReportError("http_res expects (http_res response)", node.Line, node.Column)
		return
	}
	rSym := &Node{Type: "SYMBOL", Value: "__r", Line: node.Line, Column: node.Column, Filename: node.Filename}
	res := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "res", Line: node.Line, Column: node.Column, Filename: node.Filename},
			{
				Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
				Children: []*Node{
					{Type: "SYMBOL", Value: "http_status", Line: node.Line, Column: node.Column, Filename: node.Filename},
					rSym,
				},
			},
			{Type: "STRING", Value: "application/json", Line: node.Line, Column: node.Column, Filename: node.Filename},
			{
				Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
				Children: []*Node{
					{Type: "SYMBOL", Value: "http_body", Line: node.Line, Column: node.Column, Filename: node.Filename},
					rSym,
				},
			},
		},
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "let", Line: node.Line, Column: node.Column, Filename: node.Filename},
		{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{rSym, node.Children[1]},
		},
		res,
	}
}

func desugarNot(node *Node) {
	// (not x) → (if x (= 0 1) (= 1 1))
	if len(node.Children) != 2 {
		ReportError("not expects (not expr)", node.Line, node.Column)
		return
	}
	falseLit := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "=", Line: node.Line, Column: node.Column, Filename: node.Filename},
			{Type: "INT", Value: "0", Line: node.Line, Column: node.Column, Filename: node.Filename},
			{Type: "INT", Value: "1", Line: node.Line, Column: node.Column, Filename: node.Filename},
		},
	}
	trueLit := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "=", Line: node.Line, Column: node.Column, Filename: node.Filename},
			{Type: "INT", Value: "1", Line: node.Line, Column: node.Column, Filename: node.Filename},
			{Type: "INT", Value: "1", Line: node.Line, Column: node.Column, Filename: node.Filename},
		},
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "if", Line: node.Line, Column: node.Column, Filename: node.Filename},
		node.Children[1],
		falseLit,
		trueLit,
	}
}

func desugarBlank(node *Node) {
	// (blank x) → (= x "")
	if len(node.Children) != 2 {
		ReportError("blank expects (blank expr)", node.Line, node.Column)
		return
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "=", Line: node.Line, Column: node.Column, Filename: node.Filename},
		node.Children[1],
		{Type: "STRING", Value: "", Line: node.Line, Column: node.Column, Filename: node.Filename},
	}
}

func desugarUnixNow(node *Node) {
	// (unix_now) → (to_string (time_now))
	if len(node.Children) != 1 {
		ReportError("unix_now expects (unix_now)", node.Line, node.Column)
		return
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "to_string", Line: node.Line, Column: node.Column, Filename: node.Filename},
		{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{
				{Type: "SYMBOL", Value: "time_now", Line: node.Line, Column: node.Column, Filename: node.Filename},
			},
		},
	}
}

func desugarGetd(node *Node) {
	// (getd m k d) → (let (__m m __k k) (if (map_has __m __k) (map_get __m __k) d))
	// Missing keys are "" in HowlFrame, so membership (not is_nil) is the default gate.
	if len(node.Children) != 4 {
		ReportError("getd expects (getd dict key default)", node.Line, node.Column)
		return
	}
	mSym := &Node{Type: "SYMBOL", Value: "__m", Line: node.Line, Column: node.Column, Filename: node.Filename}
	kSym := &Node{Type: "SYMBOL", Value: "__k", Line: node.Line, Column: node.Column, Filename: node.Filename}
	has := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "map_has", Line: node.Line, Column: node.Column, Filename: node.Filename},
			mSym,
			kSym,
		},
	}
	mapGet := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "map_get", Line: node.Line, Column: node.Column, Filename: node.Filename},
			mSym,
			kSym,
		},
	}
	iff := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "if", Line: node.Line, Column: node.Column, Filename: node.Filename},
			has,
			mapGet,
			node.Children[3],
		},
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "let", Line: node.Line, Column: node.Column, Filename: node.Filename},
		{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{mSym, node.Children[1], kSym, node.Children[2]},
		},
		iff,
	}
}

func desugarMinMax(node *Node) {
	// (min a b) → (let (__a a __b b) (if (< __a __b) __a __b))
	// (max a b) → (let (__a a __b b) (if (> __a __b) __a __b))
	if len(node.Children) != 3 {
		ReportError("min/max expects (min a b) or (max a b)", node.Line, node.Column)
		return
	}
	head := node.Children[0].Value
	cmp := "<"
	if head == "max" {
		cmp = ">"
	}
	aSym := &Node{Type: "SYMBOL", Value: "__a", Line: node.Line, Column: node.Column, Filename: node.Filename}
	bSym := &Node{Type: "SYMBOL", Value: "__b", Line: node.Line, Column: node.Column, Filename: node.Filename}
	cmpNode := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: cmp, Line: node.Line, Column: node.Column, Filename: node.Filename},
			aSym,
			bSym,
		},
	}
	iff := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "if", Line: node.Line, Column: node.Column, Filename: node.Filename},
			cmpNode,
			aSym,
			bSym,
		},
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "let", Line: node.Line, Column: node.Column, Filename: node.Filename},
		{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{aSym, node.Children[1], bSym, node.Children[2]},
		},
		iff,
	}
}

func desugarEqCi(node *Node) {
	// (eq_ci a b) → (= (to_lower a) (to_lower b))
	if len(node.Children) != 3 {
		ReportError("eq_ci expects (eq_ci a b)", node.Line, node.Column)
		return
	}
	lowerA := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "to_lower", Line: node.Line, Column: node.Column, Filename: node.Filename},
			node.Children[1],
		},
	}
	lowerB := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "to_lower", Line: node.Line, Column: node.Column, Filename: node.Filename},
			node.Children[2],
		},
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "=", Line: node.Line, Column: node.Column, Filename: node.Filename},
		lowerA,
		lowerB,
	}
}

func desugarSNxt(node *Node) {
	// (snxt handle key) → allocate next string id from a string counter
	// (let (__k key __v (sget handle __k) __n (if (is_nil __v) 1 (to_int __v)))
	//   (do (sput handle __k (to_string (+ __n 1))) (to_string __n)))
	if len(node.Children) != 3 {
		ReportError("snxt expects (snxt handle key)", node.Line, node.Column)
		return
	}
	handle := node.Children[1]
	key := node.Children[2]
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
	nInit := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "if", Line: node.Line, Column: node.Column, Filename: node.Filename},
			isNilV,
			{Type: "INT", Value: "1", Line: node.Line, Column: node.Column, Filename: node.Filename},
			toInt,
		},
	}
	plus := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "+", Line: node.Line, Column: node.Column, Filename: node.Filename},
			nSym,
			{Type: "INT", Value: "1", Line: node.Line, Column: node.Column, Filename: node.Filename},
		},
	}
	toStringNext := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "to_string", Line: node.Line, Column: node.Column, Filename: node.Filename},
			plus,
		},
	}
	sput := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "sput", Line: node.Line, Column: node.Column, Filename: node.Filename},
			handle,
			kSym,
			toStringNext,
		},
	}
	toStringN := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "to_string", Line: node.Line, Column: node.Column, Filename: node.Filename},
			nSym,
		},
	}
	body := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "do", Line: node.Line, Column: node.Column, Filename: node.Filename},
			sput,
			toStringN,
		},
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "let", Line: node.Line, Column: node.Column, Filename: node.Filename},
		{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{kSym, key, vSym, sget, nSym, nInit},
		},
		body,
	}
}

func desugarListContains(node *Node) {
	// (list_contains lst x) → (let (__lst lst __x x __h 0)
	//   (do (for __i __lst (if (= __i __x) (set __h 1))) (= __h 1)))
	if len(node.Children) != 3 {
		ReportError("list_contains expects (list_contains list value)", node.Line, node.Column)
		return
	}
	lstSym := &Node{Type: "SYMBOL", Value: "__lst", Line: node.Line, Column: node.Column, Filename: node.Filename}
	xSym := &Node{Type: "SYMBOL", Value: "__x", Line: node.Line, Column: node.Column, Filename: node.Filename}
	hSym := &Node{Type: "SYMBOL", Value: "__h", Line: node.Line, Column: node.Column, Filename: node.Filename}
	iSym := &Node{Type: "SYMBOL", Value: "__i", Line: node.Line, Column: node.Column, Filename: node.Filename}
	setOne := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "set", Line: node.Line, Column: node.Column, Filename: node.Filename},
			hSym,
			{Type: "INT", Value: "1", Line: node.Line, Column: node.Column, Filename: node.Filename},
		},
	}
	eq := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "=", Line: node.Line, Column: node.Column, Filename: node.Filename},
			iSym, xSym,
		},
	}
	iff := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "if", Line: node.Line, Column: node.Column, Filename: node.Filename},
			eq, setOne,
		},
	}
	forLoop := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "for", Line: node.Line, Column: node.Column, Filename: node.Filename},
			iSym, lstSym, iff,
		},
	}
	eqH := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "=", Line: node.Line, Column: node.Column, Filename: node.Filename},
			hSym,
			{Type: "INT", Value: "1", Line: node.Line, Column: node.Column, Filename: node.Filename},
		},
	}
	body := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "do", Line: node.Line, Column: node.Column, Filename: node.Filename},
			forLoop, eqH,
		},
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "let", Line: node.Line, Column: node.Column, Filename: node.Filename},
		{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{
				lstSym, node.Children[1],
				xSym, node.Children[2],
				hSym, {Type: "INT", Value: "0", Line: node.Line, Column: node.Column, Filename: node.Filename},
			},
		},
		body,
	}
}

func desugarStr(node *Node) {
	// (str a b…) → (str_join (list (to_string a) (to_string b)…) "")
	if len(node.Children) < 2 {
		ReportError("str expects (str part...)", node.Line, node.Column)
		return
	}
	parts := make([]*Node, 0, len(node.Children))
	parts = append(parts, &Node{Type: "SYMBOL", Value: "list", Line: node.Line, Column: node.Column, Filename: node.Filename})
	for _, part := range node.Children[1:] {
		parts = append(parts, &Node{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{
				{Type: "SYMBOL", Value: "to_string", Line: node.Line, Column: node.Column, Filename: node.Filename},
				part,
			},
		})
	}
	list := &Node{Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename, Children: parts}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "str_join", Line: node.Line, Column: node.Column, Filename: node.Filename},
		list,
		{Type: "STRING", Value: "", Line: node.Line, Column: node.Column, Filename: node.Filename},
	}
}

func desugarNz(node *Node) {
	// (nz x d) → (let (__nz x) (if (is_nil __nz) d __nz))
	if len(node.Children) != 3 {
		ReportError("nz expects (nz value default)", node.Line, node.Column)
		return
	}
	tmp := &Node{Type: "SYMBOL", Value: "__nz", Line: node.Line, Column: node.Column, Filename: node.Filename}
	isNil := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "is_nil", Line: node.Line, Column: node.Column, Filename: node.Filename},
			tmp,
		},
	}
	iff := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "if", Line: node.Line, Column: node.Column, Filename: node.Filename},
			isNil,
			node.Children[2],
			tmp,
		},
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "let", Line: node.Line, Column: node.Column, Filename: node.Filename},
		{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{tmp, node.Children[1]},
		},
		iff,
	}
}

func desugarMapHas(node *Node) {
	// (map_has d k) → (let (__d d __k k __h 0)
	//   (do (for __x (map_keys __d) (if (= __x __k) (set __h 1))) (= __h 1)))
	if len(node.Children) != 3 {
		ReportError("map_has expects (map_has dict key)", node.Line, node.Column)
		return
	}
	dSym := &Node{Type: "SYMBOL", Value: "__d", Line: node.Line, Column: node.Column, Filename: node.Filename}
	kSym := &Node{Type: "SYMBOL", Value: "__k", Line: node.Line, Column: node.Column, Filename: node.Filename}
	hSym := &Node{Type: "SYMBOL", Value: "__h", Line: node.Line, Column: node.Column, Filename: node.Filename}
	xSym := &Node{Type: "SYMBOL", Value: "__x", Line: node.Line, Column: node.Column, Filename: node.Filename}
	keys := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "map_keys", Line: node.Line, Column: node.Column, Filename: node.Filename},
			dSym,
		},
	}
	setOne := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "set", Line: node.Line, Column: node.Column, Filename: node.Filename},
			hSym,
			{Type: "INT", Value: "1", Line: node.Line, Column: node.Column, Filename: node.Filename},
		},
	}
	eq := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "=", Line: node.Line, Column: node.Column, Filename: node.Filename},
			xSym,
			kSym,
		},
	}
	iff := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "if", Line: node.Line, Column: node.Column, Filename: node.Filename},
			eq,
			setOne,
		},
	}
	forLoop := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "for", Line: node.Line, Column: node.Column, Filename: node.Filename},
			xSym,
			keys,
			iff,
		},
	}
	eqH := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "=", Line: node.Line, Column: node.Column, Filename: node.Filename},
			hSym,
			{Type: "INT", Value: "1", Line: node.Line, Column: node.Column, Filename: node.Filename},
		},
	}
	body := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "do", Line: node.Line, Column: node.Column, Filename: node.Filename},
			forLoop,
			eqH,
		},
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "let", Line: node.Line, Column: node.Column, Filename: node.Filename},
		{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{
				dSym, node.Children[1],
				kSym, node.Children[2],
				hSym, {Type: "INT", Value: "0", Line: node.Line, Column: node.Column, Filename: node.Filename},
			},
		},
		body,
	}
}

func desugarStrContains(node *Node) {
	// (str_contains hay needle) → (> (list_len (str_split hay needle)) 1)
	if len(node.Children) != 3 {
		ReportError("str_contains expects (str_contains hay needle)", node.Line, node.Column)
		return
	}
	split := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "str_split", Line: node.Line, Column: node.Column, Filename: node.Filename},
			node.Children[1],
			node.Children[2],
		},
	}
	llen := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "list_len", Line: node.Line, Column: node.Column, Filename: node.Filename},
			split,
		},
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: ">", Line: node.Line, Column: node.Column, Filename: node.Filename},
		llen,
		{Type: "INT", Value: "1", Line: node.Line, Column: node.Column, Filename: node.Filename},
	}
}

func desugarStrReplace(node *Node) {
	// (str_replace s old new) → (str_join (str_split s old) new)
	if len(node.Children) != 4 {
		ReportError("str_replace expects (str_replace s old new)", node.Line, node.Column)
		return
	}
	split := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "str_split", Line: node.Line, Column: node.Column, Filename: node.Filename},
			node.Children[1],
			node.Children[2],
		},
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "str_join", Line: node.Line, Column: node.Column, Filename: node.Filename},
		split,
		node.Children[3],
	}
}

func desugarStartsEndsWith(node *Node) {
	// (starts_with s p) → (let (__s s __p p __parts (str_split __s __p))
	//   (or (= __p "") (= (list_get __parts 0) "")))
	// (ends_with s p) → (let (__s s __p p)
	//   (or (= __p "") (str_contains (str __s "\x1e") (str __p "\x1e"))))
	// Sentinel is injected by desugar (not user source) so suffix match is exact.
	if len(node.Children) != 3 {
		ReportError(node.Children[0].Value+" expects ("+node.Children[0].Value+" s prefix)", node.Line, node.Column)
		return
	}
	head := node.Children[0].Value
	sSym := &Node{Type: "SYMBOL", Value: "__s", Line: node.Line, Column: node.Column, Filename: node.Filename}
	pSym := &Node{Type: "SYMBOL", Value: "__p", Line: node.Line, Column: node.Column, Filename: node.Filename}
	emptyP := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{
			{Type: "SYMBOL", Value: "=", Line: node.Line, Column: node.Column, Filename: node.Filename},
			pSym,
			{Type: "STRING", Value: "", Line: node.Line, Column: node.Column, Filename: node.Filename},
		},
	}
	var test *Node
	bindings := []*Node{sSym, node.Children[1], pSym, node.Children[2]}
	if head == "starts_with" {
		partsSym := &Node{Type: "SYMBOL", Value: "__parts", Line: node.Line, Column: node.Column, Filename: node.Filename}
		split := &Node{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{
				{Type: "SYMBOL", Value: "str_split", Line: node.Line, Column: node.Column, Filename: node.Filename},
				sSym, pSym,
			},
		}
		bindings = append(bindings, partsSym, split)
		test = &Node{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{
				{Type: "SYMBOL", Value: "=", Line: node.Line, Column: node.Column, Filename: node.Filename},
				{
					Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
					Children: []*Node{
						{Type: "SYMBOL", Value: "list_get", Line: node.Line, Column: node.Column, Filename: node.Filename},
						partsSym,
						{Type: "INT", Value: "0", Line: node.Line, Column: node.Column, Filename: node.Filename},
					},
				},
				{Type: "STRING", Value: "", Line: node.Line, Column: node.Column, Filename: node.Filename},
			},
		}
	} else {
		sentinel := &Node{Type: "STRING", Value: "\x1e", Line: node.Line, Column: node.Column, Filename: node.Filename}
		hay := &Node{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{
				{Type: "SYMBOL", Value: "str", Line: node.Line, Column: node.Column, Filename: node.Filename},
				sSym, sentinel,
			},
		}
		needle := &Node{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{
				{Type: "SYMBOL", Value: "str", Line: node.Line, Column: node.Column, Filename: node.Filename},
				pSym, sentinel,
			},
		}
		test = &Node{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{
				{Type: "SYMBOL", Value: "str_contains", Line: node.Line, Column: node.Column, Filename: node.Filename},
				hay, needle,
			},
		}
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "let", Line: node.Line, Column: node.Column, Filename: node.Filename},
		{Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename, Children: bindings},
		{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{
				{Type: "SYMBOL", Value: "or", Line: node.Line, Column: node.Column, Filename: node.Filename},
				emptyP, test,
			},
		},
	}
}

func desugarNaryLogic(node *Node) {
	// (or a b c…) → (or a (or b (or c…))); same for and.
	if len(node.Children) < 4 {
		return
	}
	head := node.Children[0]
	acc := node.Children[len(node.Children)-1]
	for i := len(node.Children) - 2; i >= 1; i-- {
		acc = &Node{
			Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{
				{Type: "SYMBOL", Value: head.Value, Line: head.Line, Column: head.Column, Filename: head.Filename},
				node.Children[i],
				acc,
			},
		}
	}
	node.Children = acc.Children
}

func desugarWhenUnless(node *Node) {
	// (when test body…) → (if test (do body…) (do))
	// (unless test body…) → (if test (do) (do body…))
	if len(node.Children) < 3 {
		ReportError(node.Children[0].Value+" expects ( "+node.Children[0].Value+" test body...)", node.Line, node.Column)
		return
	}
	head := node.Children[0].Value
	test := node.Children[1]
	body := wrapBodyForms(node.Children[2:], node.Line, node.Column, node.Filename)
	empty := &Node{
		Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
		Children: []*Node{{Type: "SYMBOL", Value: "do", Line: node.Line, Column: node.Column, Filename: node.Filename}},
	}
	thenForm, elseForm := body, empty
	if head == "unless" {
		thenForm, elseForm = empty, body
	}
	node.Children = []*Node{
		{Type: "SYMBOL", Value: "if", Line: node.Line, Column: node.Column, Filename: node.Filename},
		test,
		thenForm,
		elseForm,
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
	"to_lower": true, "to_upper": true, "str_trim": true,
	"env": true, "read_file": true, "read_line": true, "read_text": true, "cli_args": true,
	"lines": true, "str_split": true, "str_join": true, "str": true, "str_contains": true,
	"str_replace": true, "starts_with": true, "ends_with": true, "http_body": true,
	"http_status": true, "list_contains": true, "not": true, "blank": true,
	"unix_now": true, "getd": true, "min": true, "max": true, "eq_ci": true, "snxt": true,
	"regex_match": true,
	"parse_json":  true, "encode_json": true, "is_nil": true, "nz": true, "map_has": true,
	"html_escape": true, "attr_escape": true, "time_now": true,
	"req_header": true, "req_method": true, "req_path": true, "req_query": true,
	"http_request": true, "fetch": true,
}
