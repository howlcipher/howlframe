package hfir

import (
	"fmt"
	"github.com/howlcipher/howlframe/internal/ast"
)

type LoweringContext struct {
	Graph  *Graph
	Module string
}

func LowerAST(root *ast.Node, module string) (*Graph, error) {
	ctx := &LoweringContext{
		Graph:  NewGraph(),
		Module: module,
	}

	entry, err := ctx.lowerNode(root)
	if err != nil {
		return nil, err
	}

	ctx.Graph.EntryNode = entry
	return ctx.Graph, nil
}

func (ctx *LoweringContext) lowerNode(astNode *ast.Node) (NodeID, error) {
	if astNode == nil {
		return "", nil
	}

	node := &Node{
		Module: ctx.Module,
		Provenance: Provenance{
			Filename: astNode.Filename,
			Line:     astNode.Line,
			Column:   astNode.Column,
		},
		Type: astNode.Inferred,
	}

	// Leaf type strings must track internal/lexer/lexer.go's TokenType
	// constants exactly (integers lex as "INT", never "NUMBER").
	if astNode.Type == "STRING" || astNode.Type == "INT" || astNode.Type == "SYMBOL" || astNode.Type == "FLOAT" {
		node.Kind = "const"
		node.LiteralKind = astNode.Type
		if astNode.Type == "SYMBOL" {
			if astNode.Value != "true" && astNode.Value != "false" {
				node.Kind = "symbol"
			} else {
				node.LiteralKind = "BOOL"
			}
		}
		node.Value = astNode.Value
		return ctx.Graph.AddNode(node), nil
	}

	if astNode.Type == "List" && len(astNode.Children) == 0 {
		// An empty parenthesized list: a zero-argument defun/lambda
		// parameter list, or an empty list/dict literal. There is no head
		// symbol to name this node's Kind, so it lowers to an explicit
		// "empty_list" leaf with no data inputs, rather than falling through
		// to the unknown-node-type error below.
		node.Kind = "empty_list"
		return ctx.Graph.AddNode(node), nil
	}

	if astNode.Type == "List" && len(astNode.Children) > 0 {
		head := astNode.Children[0]
		if head.Type == "SYMBOL" {
			if id, handled, err := ctx.lowerSemanticList(node, astNode, head.Value); handled {
				return id, err
			}
		}
		node.Kind = head.Value
		if head.Type != "SYMBOL" {
			node.Kind = "call"
		}

		id := ctx.Graph.AddNode(node)

		for i, child := range astNode.Children {
			if i == 0 && head.Type == "SYMBOL" {
				continue
			}
			childID, err := ctx.lowerNode(child)
			if err != nil {
				return "", err
			}
			if childID != "" {
				node.DataInputs = append(node.DataInputs, DataEdge{
					SourceNode: childID,
				})
			}
		}

		return id, nil
	}

	return "", fmt.Errorf("unknown AST node type: %s", astNode.Type)
}

// lowerSemanticList gives the executable HFIR subset explicit semantic shape.
// The generic fallback above remains intentionally available to the verifier
// for constructs outside that subset; it is never consumed by the direct
// bytecode lowerer.
func (ctx *LoweringContext) lowerSemanticList(node *Node, astNode *ast.Node, head string) (NodeID, bool, error) {
	addChildren := func(start int, edgeName string) (NodeID, error) {
		id := ctx.Graph.AddNode(node)
		for _, child := range astNode.Children[start:] {
			childID, err := ctx.lowerNode(child)
			if err != nil {
				return "", err
			}
			node.DataInputs = append(node.DataInputs, DataEdge{Name: edgeName, SourceNode: childID})
		}
		return id, nil
	}
	addNamed := func(parts []struct {
		name  string
		child *ast.Node
	}) (NodeID, error) {
		id := ctx.Graph.AddNode(node)
		for _, part := range parts {
			childID, err := ctx.lowerNode(part.child)
			if err != nil {
				return "", err
			}
			node.DataInputs = append(node.DataInputs, DataEdge{Name: part.name, SourceNode: childID})
		}
		return id, nil
	}

	switch head {
	case "cli_app":
		node.Kind = "program"
		id, err := addChildren(1, "body")
		return id, true, err
	case "do":
		node.Kind = "sequence"
		id, err := addChildren(1, "body")
		return id, true, err
	case "let":
		if len(astNode.Children) != 3 || astNode.Children[1].Type != "List" || len(astNode.Children[1].Children) != 2 || astNode.Children[1].Children[0].Type != "SYMBOL" {
			return "", false, nil
		}
		node.Kind = "let"
		node.Value = astNode.Children[1].Children[0].Value
		id, err := addNamed([]struct {
			name  string
			child *ast.Node
		}{{"value", astNode.Children[1].Children[1]}, {"body", astNode.Children[2]}})
		return id, true, err
	case "set":
		if len(astNode.Children) != 3 || astNode.Children[1].Type != "SYMBOL" {
			return "", false, nil
		}
		node.Kind = "set"
		node.Value = astNode.Children[1].Value
		id, err := addNamed([]struct {
			name  string
			child *ast.Node
		}{{"value", astNode.Children[2]}})
		return id, true, err
	case "if":
		if len(astNode.Children) != 3 && len(astNode.Children) != 4 {
			return "", false, nil
		}
		node.Kind = "if"
		parts := []struct {
			name  string
			child *ast.Node
		}{{"condition", astNode.Children[1]}, {"then", astNode.Children[2]}}
		if len(astNode.Children) == 4 {
			parts = append(parts, struct {
				name  string
				child *ast.Node
			}{"else", astNode.Children[3]})
		}
		id, err := addNamed(parts)
		return id, true, err
	case "+", "-", "*", "/", "<", ">", "<=", ">=", "==", "!=", "=", "and", "or":
		if len(astNode.Children) != 3 {
			return "", false, nil
		}
		node.Kind = "binary"
		node.Value = head
		if node.Value == "=" {
			node.Value = "=="
		}
		id, err := addNamed([]struct {
			name  string
			child *ast.Node
		}{{"left", astNode.Children[1]}, {"right", astNode.Children[2]}})
		return id, true, err
	case "list":
		node.Kind = "list"
		id, err := addChildren(1, "item")
		return id, true, err
	case "dict":
		node.Kind = "dict"
		id := ctx.Graph.AddNode(node)
		for _, entry := range astNode.Children[1:] {
			if entry.Type != "List" || len(entry.Children) != 2 {
				return "", true, fmt.Errorf("dict entry must contain a key and value")
			}
			entryNode := &Node{Kind: "dict_entry", Module: ctx.Module, Provenance: node.Provenance}
			entryID := ctx.Graph.AddNode(entryNode)
			for index, name := range []string{"key", "value"} {
				childID, err := ctx.lowerNode(entry.Children[index])
				if err != nil {
					return "", true, err
				}
				entryNode.DataInputs = append(entryNode.DataInputs, DataEdge{Name: name, SourceNode: childID})
			}
			node.DataInputs = append(node.DataInputs, DataEdge{Name: "entry", SourceNode: entryID})
		}
		return id, true, nil
	case "print", "stderr", "exit":
		node.Kind = head
		id, err := addChildren(1, "value")
		return id, true, err
	case "to_int", "to_float", "to_string", "bytes_to_string", "encode_json":
		if len(astNode.Children) != 2 {
			return "", false, nil
		}
		node.Kind = "convert"
		node.Value = head
		id, err := addChildren(1, "value")
		return id, true, err
	case "str_split", "str_join":
		if len(astNode.Children) != 3 {
			return "", false, nil
		}
		node.Kind = head
		id, err := addNamed([]struct {
			name  string
			child *ast.Node
		}{{"value", astNode.Children[1]}, {"separator", astNode.Children[2]}})
		return id, true, err
	case "list_len", "env":
		if len(astNode.Children) != 2 {
			return "", false, nil
		}
		node.Kind = head
		id, err := addChildren(1, "value")
		return id, true, err
	case "map_get", "map_delete", "list_get", "append", "cli_args", "read_file", "parse_json", "is_nil":
		if len(astNode.Children) < 2 {
			return "", false, nil
		}
		if (head == "map_get" || head == "map_delete" || head == "list_get" || head == "append" || head == "parse_json") && astNode.Children[1].Type != "SYMBOL" {
			return "", false, nil
		}
		node.Kind = head
		if head == "map_get" || head == "map_delete" || head == "list_get" || head == "append" || head == "parse_json" {
			node.Value = astNode.Children[1].Value
		}
		if head == "parse_json" && len(astNode.Children) == 3 {
			node.Value = astNode.Children[2].Value
		}

		if head == "cli_args" {
			if len(astNode.Children) == 2 {
				id, err := addNamed([]struct {
					name  string
					child *ast.Node
				}{{"index", astNode.Children[1]}})
				return id, true, err
			} else if len(astNode.Children) == 1 {
				return ctx.Graph.AddNode(node), true, nil
			}
			return "", false, nil
		} else if head == "read_file" {
			if len(astNode.Children) != 2 {
				return "", false, nil
			}
			id, err := addNamed([]struct {
				name  string
				child *ast.Node
			}{{"path", astNode.Children[1]}})
			return id, true, err
		} else if head == "parse_json" {
			if len(astNode.Children) != 3 {
				return "", false, nil
			}
			id, err := addNamed([]struct {
				name  string
				child *ast.Node
			}{{"content", astNode.Children[2]}})
			return id, true, err
		} else if head == "is_nil" {
			if len(astNode.Children) != 2 {
				return "", false, nil
			}
			id, err := addNamed([]struct {
				name  string
				child *ast.Node
			}{{"value", astNode.Children[1]}})
			return id, true, err
		}

		edgeName := "key"
		if head == "list_get" {
			edgeName = "index"
		} else if head == "append" {
			edgeName = "item"
		}
		id, err := addChildren(2, edgeName)
		return id, true, err
	case "map_set":
		if len(astNode.Children) != 4 || astNode.Children[1].Type != "SYMBOL" {
			return "", false, nil
		}
		node.Kind = head
		node.Value = astNode.Children[1].Value
		id, err := addNamed([]struct {
			name  string
			child *ast.Node
		}{{"key", astNode.Children[2]}, {"value", astNode.Children[3]}})
		return id, true, err
	case "try_let":
		if len(astNode.Children) != 4 || astNode.Children[1].Type != "List" || len(astNode.Children[1].Children) != 2 || astNode.Children[1].Children[0].Type != "SYMBOL" {
			return "", false, nil
		}
		if astNode.Children[2].Type != "List" || len(astNode.Children[2].Children) != 3 || astNode.Children[2].Children[0].Value != "catch" || astNode.Children[2].Children[1].Type != "SYMBOL" {
			return "", false, nil
		}
		node.Kind = "try"
		node.Value = astNode.Children[1].Children[0].Value // success binding

		id := ctx.Graph.AddNode(node)

		exprID, err := ctx.lowerNode(astNode.Children[1].Children[1])
		if err != nil {
			return "", true, err
		}
		node.DataInputs = append(node.DataInputs, DataEdge{Name: "expression", SourceNode: exprID})

		successBodyID, err := ctx.lowerNode(astNode.Children[3])
		if err != nil {
			return "", true, err
		}
		node.DataInputs = append(node.DataInputs, DataEdge{Name: "success_body", SourceNode: successBodyID})

		catchNode := &Node{Kind: "catch", Module: ctx.Module, Provenance: node.Provenance, Value: astNode.Children[2].Children[1].Value} // catch binding
		catchID := ctx.Graph.AddNode(catchNode)

		catchBodyID, err := ctx.lowerNode(astNode.Children[2].Children[2])
		if err != nil {
			return "", true, err
		}
		catchNode.DataInputs = append(catchNode.DataInputs, DataEdge{Name: "body", SourceNode: catchBodyID})

		node.DataInputs = append(node.DataInputs, DataEdge{Name: "catch", SourceNode: catchID})

		return id, true, nil
	case "for":
		if len(astNode.Children) != 4 || astNode.Children[1].Type != "SYMBOL" {
			return "", false, nil
		}
		node.Kind = "for"
		node.Value = astNode.Children[1].Value
		id, err := addNamed([]struct {
			name  string
			child *ast.Node
		}{{"iterable", astNode.Children[2]}, {"body", astNode.Children[3]}})
		return id, true, err
	}
	return "", false, nil
}
