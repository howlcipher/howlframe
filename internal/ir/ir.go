package ir

import "zero/internal/ast"

type IRNode struct {
	Kind  string
	Op    string
	Kids  []*ast.Node
	Cases []IRCase
}

type IRCase struct {
	Label     *ast.Node
	Body      *ast.Node
	IsDefault bool
}

var BinOpKinds = map[string]bool{
	"+": true, "-": true, "*": true, "/": true,
	"<": true, ">": true, "<=": true, ">=": true,
	"and": true, "or": true, "==": true, "!=": true, "=": true,
}

// LowerShared performs the shared arity/shape validation and child
// extraction for the ~19 node kinds with identical cross-backend semantics.
// It intentionally does not unify the small number of validation asymmetries
// already present between the Go and JS backends (e.g. Go type-checks the
// append/map_set/map_delete target is a SYMBOL and JS does not; JS's binop
// arity check is absent where Go's exists) — preserving each backend's exact
// pre-refactor behavior takes priority over cosmetic validation parity, so
// those extra checks stay in the per-backend emit functions.
func LowerShared(node *ast.Node) (*IRNode, bool) {
	if node.Type != "List" || len(node.Children) == 0 {
		return nil, false
	}
	head := node.Children[0].Value
	if BinOpKinds[head] {
		return &IRNode{Kind: "binop", Op: head, Kids: node.Children[1:]}, true
	}
	switch head {
	case "return":
		if len(node.Children) != 2 {
			ast.ReportError("return expects (return val)", node.Line, node.Column)
		}
		return &IRNode{Kind: "return", Kids: []*ast.Node{node.Children[1]}}, true
	case "if":
		if len(node.Children) != 3 && len(node.Children) != 4 {
			ast.ReportError("if expects (if cond then) or (if cond then else)", node.Line, node.Column)
		}
		return &IRNode{Kind: "if", Kids: node.Children[1:]}, true
	case "while":
		if len(node.Children) != 3 {
			ast.ReportError("while expects (while cond body)", node.Line, node.Column)
		}
		return &IRNode{Kind: "while", Kids: node.Children[1:]}, true
	case "do":
		return &IRNode{Kind: "do", Kids: node.Children[1:]}, true
	case "set":
		if len(node.Children) != 3 {
			ast.ReportError("set expects (set var val)", node.Line, node.Column)
		}
		return &IRNode{Kind: "set", Kids: node.Children[1:]}, true
	case "match":
		if len(node.Children) < 3 {
			ast.ReportError("match expects (match var (val body)...)", node.Line, node.Column)
		}
		ir := &IRNode{Kind: "match", Kids: []*ast.Node{node.Children[1]}}
		for j := 2; j < len(node.Children); j++ {
			caseNode := node.Children[j]
			if caseNode.Type != "List" || len(caseNode.Children) != 2 {
				ast.ReportError("match case expects (val body)", caseNode.Line, caseNode.Column)
			}
			labelNode := caseNode.Children[0]
			isDefault := labelNode.Type == "SYMBOL" && labelNode.Value == "default"
			ir.Cases = append(ir.Cases, IRCase{Label: labelNode, Body: caseNode.Children[1], IsDefault: isDefault})
		}
		return ir, true
	case "sleep":
		if len(node.Children) != 2 {
			ast.ReportError("sleep expects (sleep ms)", node.Line, node.Column)
		}
		return &IRNode{Kind: "sleep", Kids: []*ast.Node{node.Children[1]}}, true
	case "to_int":
		if len(node.Children) != 2 {
			ast.ReportError("to_int expects (to_int val)", node.Line, node.Column)
		}
		return &IRNode{Kind: "to_int", Kids: []*ast.Node{node.Children[1]}}, true
	case "to_float":
		if len(node.Children) != 2 {
			ast.ReportError("to_float expects (to_float val)", node.Line, node.Column)
		}
		return &IRNode{Kind: "to_float", Kids: []*ast.Node{node.Children[1]}}, true
	case "to_string":
		if len(node.Children) != 2 {
			ast.ReportError("to_string expects 1 argument", node.Line, node.Column)
		}
		return &IRNode{Kind: "to_string", Kids: []*ast.Node{node.Children[1]}}, true
	case "bytes_to_string":
		if len(node.Children) != 2 {
			ast.ReportError("bytes_to_string expects 1 argument", node.Line, node.Column)
		}
		return &IRNode{Kind: "bytes_to_string", Kids: []*ast.Node{node.Children[1]}}, true
	case "str_split":
		if len(node.Children) != 3 {
			ast.ReportError("str_split expects (str_split s sep)", node.Line, node.Column)
		}
		return &IRNode{Kind: "str_split", Kids: node.Children[1:]}, true
	case "str_join":
		if len(node.Children) != 3 {
			ast.ReportError("str_join expects (str_join list sep)", node.Line, node.Column)
		}
		return &IRNode{Kind: "str_join", Kids: node.Children[1:]}, true
	case "regex_match":
		if len(node.Children) != 3 {
			ast.ReportError("regex_match expects (regex_match pattern s)", node.Line, node.Column)
		}
		return &IRNode{Kind: "regex_match", Kids: node.Children[1:]}, true
	case "append":
		if len(node.Children) != 3 {
			ast.ReportError("append expects (append list item)", node.Line, node.Column)
		}
		return &IRNode{Kind: "append", Kids: node.Children[1:]}, true
	case "map_set":
		if len(node.Children) != 4 {
			ast.ReportError("map_set expects (map_set dict key val)", node.Line, node.Column)
		}
		return &IRNode{Kind: "map_set", Kids: node.Children[1:]}, true
	case "map_delete":
		if len(node.Children) != 3 {
			ast.ReportError("map_delete expects (map_delete dict key)", node.Line, node.Column)
		}
		return &IRNode{Kind: "map_delete", Kids: node.Children[1:]}, true
	case "map_get":
		if len(node.Children) != 3 {
			ast.ReportError("map_get expects (map_get dict key)", node.Line, node.Column)
		}
		return &IRNode{Kind: "map_get", Kids: node.Children[1:]}, true
	case "list_get":
		if len(node.Children) != 3 {
			ast.ReportError("list_get expects (list_get list idx)", node.Line, node.Column)
		}
		return &IRNode{Kind: "list_get", Kids: node.Children[1:]}, true
	case "list":
		return &IRNode{Kind: "list", Kids: node.Children[1:]}, true
	case "dict":
		return &IRNode{Kind: "dict", Kids: node.Children[1:]}, true
	case "print":
		return &IRNode{Kind: "print", Kids: node.Children[1:]}, true
	}
	return nil, false
}
