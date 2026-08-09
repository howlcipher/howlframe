package ir

import (
	"fmt"
	"github.com/howlcipher/howlframe/internal/ast"
)

type IRNode struct {
	Kind  string
	Op    string
	Kids  []*ast.Node
	Cases []IRCase
	Type  ast.TypeInfo
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
// LowerShared preserves the semantic type annotation produced by the
// checker. Backends that need native layout information can consume IRNode's
// Type without walking back to the source node.
func LowerShared(node *ast.Node) (*IRNode, bool) {
	if reason, shared := ValidateShared(node); !shared || reason != "" {
		return nil, false
	}
	result, ok := lowerShared(node)
	if ok {
		result.Type = node.Inferred
	}
	return result, ok
}

// ValidateShared returns a semantic error for malformed shared forms. The
// boolean reports whether the node is a shared form, including invalid ones.
func ValidateShared(node *ast.Node) (string, bool) {
	if node == nil || node.Type != "List" || len(node.Children) == 0 {
		return "", false
	}
	head := node.Children[0].Value
	if BinOpKinds[head] {
		return requireArguments(node, 2), true
	}
	switch head {
	case "return", "sleep", "to_int", "to_float", "to_string", "bytes_to_string":
		return requireArguments(node, 1), true
	case "if":
		arguments := len(node.Children) - 1
		if arguments != 2 && arguments != 3 {
			return "if expects 2 or 3 arguments", true
		}
		return "", true
	case "while", "set", "str_split", "str_join", "regex_match", "append", "map_delete", "map_get", "list_get":
		return requireArguments(node, 2), true
	case "map_set":
		return requireArguments(node, 3), true
	case "match":
		if len(node.Children) < 3 {
			return "match expects a value and at least one case", true
		}
		for _, caseNode := range node.Children[2:] {
			if caseNode.Type != "List" || len(caseNode.Children) != 2 {
				return "match cases expect a label and body", true
			}
		}
		return "", true
	case "call":
		if len(node.Children) < 2 {
			return "call expects at least a function name", true
		}
		return "", true
	case "let":
		if reason := requireArguments(node, 2); reason != "" {
			return reason, true
		}
		binding := node.Children[1]
		if binding.Type != "List" || len(binding.Children) != 2 {
			return "let binding expects (var val)", true
		}
		return "", true
	case "try_let":
		if reason := requireArguments(node, 3); reason != "" {
			return reason, true
		}
		binding := node.Children[1]
		if binding.Type != "List" || len(binding.Children) != 2 {
			return "try_let binding expects (var val)", true
		}
		catchNode := node.Children[2]
		if catchNode.Type != "List" || len(catchNode.Children) != 3 || catchNode.Children[0].Value != "catch" {
			return "try_let expects (catch err body)", true
		}
		return "", true
	case "spawn":
		if reason := requireArguments(node, 1); reason != "" {
			return reason, true
		}
		lambda := node.Children[1]
		if lambda.Type != "List" || len(lambda.Children) != 3 || lambda.Children[0].Value != "lambda" {
			return "spawn expects a lambda", true
		}
		return "", true
	case "for":
		return requireArguments(node, 3), true
	case "dict":
		for _, pair := range node.Children[1:] {
			if pair.Type != "List" || len(pair.Children) != 2 {
				return "dict expects (k v) pairs", true
			}
		}
		return "", true
	case "list", "print", "do":
		return "", true
	default:
		return "", false
	}
}

func requireArguments(node *ast.Node, want int) string {
	got := len(node.Children) - 1
	if got == want {
		return ""
	}
	noun := "arguments"
	if want == 1 {
		noun = "argument"
	}
	return fmt.Sprintf("%s expects %d %s, got %d", node.Children[0].Value, want, noun, got)
}

func lowerShared(node *ast.Node) (*IRNode, bool) {
	if node.Type != "List" || len(node.Children) == 0 {
		return nil, false
	}
	head := node.Children[0].Value
	if BinOpKinds[head] {
		return &IRNode{Kind: "binop", Op: head, Kids: node.Children[1:]}, true
	}
	switch head {
	case "return":
		return &IRNode{Kind: "return", Kids: []*ast.Node{node.Children[1]}}, true
	case "if":
		return &IRNode{Kind: "if", Kids: node.Children[1:]}, true
	case "while":
		return &IRNode{Kind: "while", Kids: node.Children[1:]}, true
	case "set":
		return &IRNode{Kind: "set", Kids: node.Children[1:]}, true
	case "match":
		ir := &IRNode{Kind: "match", Kids: []*ast.Node{node.Children[1]}}
		for j := 2; j < len(node.Children); j++ {
			caseNode := node.Children[j]
			labelNode := caseNode.Children[0]
			isDefault := labelNode.Type == "SYMBOL" && labelNode.Value == "default"
			ir.Cases = append(ir.Cases, IRCase{Label: labelNode, Body: caseNode.Children[1], IsDefault: isDefault})
		}
		return ir, true
	case "sleep":
		return &IRNode{Kind: "sleep", Kids: []*ast.Node{node.Children[1]}}, true
	case "to_int":
		return &IRNode{Kind: "to_int", Kids: []*ast.Node{node.Children[1]}}, true
	case "to_float":
		return &IRNode{Kind: "to_float", Kids: []*ast.Node{node.Children[1]}}, true
	case "to_string":
		return &IRNode{Kind: "to_string", Kids: []*ast.Node{node.Children[1]}}, true
	case "bytes_to_string":
		return &IRNode{Kind: "bytes_to_string", Kids: []*ast.Node{node.Children[1]}}, true
	case "str_split":
		return &IRNode{Kind: "str_split", Kids: node.Children[1:]}, true
	case "str_join":
		return &IRNode{Kind: "str_join", Kids: node.Children[1:]}, true
	case "regex_match":
		return &IRNode{Kind: "regex_match", Kids: node.Children[1:]}, true
	case "append":
		return &IRNode{Kind: "append", Kids: node.Children[1:]}, true
	case "map_set":
		return &IRNode{Kind: "map_set", Kids: node.Children[1:]}, true
	case "map_delete":
		return &IRNode{Kind: "map_delete", Kids: node.Children[1:]}, true
	case "map_get":
		return &IRNode{Kind: "map_get", Kids: node.Children[1:]}, true
	case "list_get":
		return &IRNode{Kind: "list_get", Kids: node.Children[1:]}, true
	case "call", "let", "try_let", "spawn", "for", "list", "dict", "print", "do":
		return &IRNode{Kind: head, Kids: node.Children[1:]}, true
	}
	return nil, false
}
