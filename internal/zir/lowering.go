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

	if astNode.Type == "STRING" || astNode.Type == "NUMBER" || astNode.Type == "SYMBOL" || astNode.Type == "FLOAT" {
		node.Kind = "const"
		if astNode.Type == "SYMBOL" {
			node.Kind = "symbol"
		}
		node.Value = astNode.Value
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
