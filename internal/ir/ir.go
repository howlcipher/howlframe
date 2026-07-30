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
		}
		return &IRNode{Kind: "return", Kids: []*ast.Node{node.Children[1]}}, true
	case "if":
		if len(node.Children) != 3 && len(node.Children) != 4 {
		}
		return &IRNode{Kind: "if", Kids: node.Children[1:]}, true
	case "while":
		if len(node.Children) != 3 {
		}
		return &IRNode{Kind: "while", Kids: node.Children[1:]}, true
	case "set":
		if len(node.Children) != 3 {
		}
		return &IRNode{Kind: "set", Kids: node.Children[1:]}, true
	case "match":
		if len(node.Children) < 3 {
		}
		ir := &IRNode{Kind: "match", Kids: []*ast.Node{node.Children[1]}}
		for j := 2; j < len(node.Children); j++ {
			caseNode := node.Children[j]
			if caseNode.Type != "List" || len(caseNode.Children) != 2 {
			}
			labelNode := caseNode.Children[0]
			isDefault := labelNode.Type == "SYMBOL" && labelNode.Value == "default"
			ir.Cases = append(ir.Cases, IRCase{Label: labelNode, Body: caseNode.Children[1], IsDefault: isDefault})
		}
		return ir, true
	case "sleep":
		if len(node.Children) != 2 {
		}
		return &IRNode{Kind: "sleep", Kids: []*ast.Node{node.Children[1]}}, true
	case "to_int":
		if len(node.Children) != 2 {
		}
		return &IRNode{Kind: "to_int", Kids: []*ast.Node{node.Children[1]}}, true
	case "to_float":
		if len(node.Children) != 2 {
		}
		return &IRNode{Kind: "to_float", Kids: []*ast.Node{node.Children[1]}}, true
	case "to_string":
		if len(node.Children) != 2 {
		}
		return &IRNode{Kind: "to_string", Kids: []*ast.Node{node.Children[1]}}, true
	case "bytes_to_string":
		if len(node.Children) != 2 {
		}
		return &IRNode{Kind: "bytes_to_string", Kids: []*ast.Node{node.Children[1]}}, true
	case "str_split":
		if len(node.Children) != 3 {
		}
		return &IRNode{Kind: "str_split", Kids: node.Children[1:]}, true
	case "str_join":
		if len(node.Children) != 3 {
		}
		return &IRNode{Kind: "str_join", Kids: node.Children[1:]}, true
	case "regex_match":
		if len(node.Children) != 3 {
		}
		return &IRNode{Kind: "regex_match", Kids: node.Children[1:]}, true
	case "append":
		if len(node.Children) != 3 {
		}
		return &IRNode{Kind: "append", Kids: node.Children[1:]}, true
	case "map_set":
		if len(node.Children) != 4 {
		}
		return &IRNode{Kind: "map_set", Kids: node.Children[1:]}, true
	case "map_delete":
		if len(node.Children) != 3 {
		}
		return &IRNode{Kind: "map_delete", Kids: node.Children[1:]}, true
	case "map_get":
		if len(node.Children) != 3 {
		}
		return &IRNode{Kind: "map_get", Kids: node.Children[1:]}, true
	case "list_get":
		if len(node.Children) != 3 {
		}
		return &IRNode{Kind: "list_get", Kids: node.Children[1:]}, true
	case "call", "let", "try_let", "spawn", "for", "list", "dict", "print", "do":
		return &IRNode{Kind: head, Kids: node.Children[1:]}, true
	}
	return nil, false
}
