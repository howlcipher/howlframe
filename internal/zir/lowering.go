package zir

import (
	"fmt"
	"zero/internal/ast"
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
		if astNode.Type == "SYMBOL" {
			node.Kind = "symbol"
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
