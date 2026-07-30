package ast

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type ErrorOutput struct {
	Reason string `json:"reason"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

func ReportError(reason string, line, column int) {
	errOut := ErrorOutput{Reason: reason, Line: line, Column: column}
	b, _ := json.Marshal(errOut)
	fmt.Println(string(b))
	os.Exit(1)
}

type Node struct {
	Type     string
	Value    string
	Children []*Node
	Line     int
	Column   int
	Filename string
	Inferred TypeInfo
}

// ValueKind is the language-level type category inferred by the semantic
// checker. Unknown is intentional: Zero permits dynamic values at runtime,
// while known values still carry enough layout information for native
// backends to make safe decisions.
type ValueKind string

const (
	Unknown ValueKind = "unknown"
	Any     ValueKind = "any"
	Void    ValueKind = "void"
	Bool    ValueKind = "bool"
	Int     ValueKind = "int"
	Float   ValueKind = "float"
	String  ValueKind = "string"
	Bytes   ValueKind = "bytes"
	List    ValueKind = "list"
	Dict    ValueKind = "dict"
)

// TypeInfo describes both the source-level type and its native layout. Size
// and Align are zero for Unknown/Any because no backend-safe layout is known.
type TypeInfo struct {
	Kind    ValueKind
	Name    string
	Size    uint64
	Align   uint64
	Pointer bool
	Element *TypeInfo
}

func Layout(kind ValueKind) TypeInfo {
	switch kind {
	case Bool:
		return TypeInfo{Kind: Bool, Name: "bool", Size: 1, Align: 1}
	case Int:
		return TypeInfo{Kind: Int, Name: "int", Size: 8, Align: 8}
	case Float:
		return TypeInfo{Kind: Float, Name: "float64", Size: 8, Align: 8}
	case String:
		return TypeInfo{Kind: String, Name: "string", Size: 16, Align: 8, Pointer: true}
	case Bytes:
		return TypeInfo{Kind: Bytes, Name: "[]byte", Size: 24, Align: 8, Pointer: true}
	case List:
		return TypeInfo{Kind: List, Name: "list", Size: 24, Align: 8, Pointer: true}
	case Dict:
		return TypeInfo{Kind: Dict, Name: "dict", Size: 8, Align: 8, Pointer: true}
	case Void:
		return TypeInfo{Kind: Void, Name: "void", Align: 1}
	case Any:
		return TypeInfo{Kind: Any, Name: "any"}
	default:
		return TypeInfo{Kind: Unknown, Name: "unknown"}
	}
}

func CopyNode(n *Node) *Node {
	if n == nil {
		return nil
	}
	clone := &Node{Type: n.Type, Value: n.Value, Line: n.Line, Column: n.Column, Filename: n.Filename}
	for _, child := range n.Children {
		clone.Children = append(clone.Children, CopyNode(child))
	}
	return clone
}

func ReplaceNext(node *Node, replacement *Node) {
	if node == nil {
		return
	}
	for i, child := range node.Children {
		if child.Type == "List" && len(child.Children) == 1 && child.Children[0].Value == "next" {
			node.Children[i] = replacement
		} else {
			ReplaceNext(child, replacement)
		}
	}
}

func RenameVar(node *Node, oldName, newName string) {
	if node == nil {
		return
	}
	if node.Type == "SYMBOL" {
		if node.Value == oldName {
			node.Value = newName
		} else if strings.HasPrefix(node.Value, oldName+".") {
			node.Value = newName + node.Value[len(oldName):]
		}
	}
	for _, child := range node.Children {
		RenameVar(child, oldName, newName)
	}
}

func ApplyPatches(node *Node) {
	if node == nil || node.Type != "List" {
		return
	}
	var newChildren []*Node
	for i := 0; i < len(node.Children); i++ {
		child := node.Children[i]
		if child.Type == "List" && len(child.Children) == 3 && child.Children[0].Type == "SYMBOL" && child.Children[0].Value == "patch" {
			funcNameNode := child.Children[1]
			if funcNameNode.Type != "SYMBOL" {
				ReportError("patch expects a symbol for function name", funcNameNode.Line, funcNameNode.Column)
			}
			funcName := funcNameNode.Value
			newBody := child.Children[2]

			found := false
			for j := 0; j < len(newChildren); j++ {
				target := newChildren[j]
				if target.Type == "List" && len(target.Children) >= 4 && target.Children[0].Type == "SYMBOL" && target.Children[0].Value == "defun" && target.Children[1].Value == funcName {
					target.Children[len(target.Children)-1] = CopyNode(newBody)
					found = true
					break
				}
			}
			if !found {
				for j := i + 1; j < len(node.Children); j++ {
					target := node.Children[j]
					if target.Type == "List" && len(target.Children) >= 4 && target.Children[0].Type == "SYMBOL" && target.Children[0].Value == "defun" && target.Children[1].Value == funcName {
						target.Children[len(target.Children)-1] = CopyNode(newBody)
						found = true
						break
					}
				}
			}
			if !found {
				ReportError(fmt.Sprintf("patch target function %q not found", funcName), child.Line, child.Column)
			}
		} else {
			ApplyPatches(child)
			newChildren = append(newChildren, child)
		}
	}
	node.Children = newChildren
}

func ApplyWithContext(node *Node, ctxVars []*Node) *Node {
	if node == nil {
		return nil
	}
	if node.Type != "List" || len(node.Children) == 0 {
		return node
	}

	head := node.Children[0].Value
	if head == "with_context" {
		if len(node.Children) < 3 {
			ReportError("with_context expects (with_context vars body...)", node.Line, node.Column)
		}
		varsNode := node.Children[1]
		var newCtxVars []*Node
		newCtxVars = append(newCtxVars, ctxVars...)
		if varsNode.Type == "SYMBOL" {
			newCtxVars = append(newCtxVars, varsNode)
		} else if varsNode.Type == "List" {
			for _, v := range varsNode.Children {
				if v.Type != "SYMBOL" {
					ReportError("with_context vars must be symbols", v.Line, v.Column)
				}
				newCtxVars = append(newCtxVars, v)
			}
		} else {
			ReportError("with_context expects symbol or list of symbols", varsNode.Line, varsNode.Column)
		}

		doNode := &Node{Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename}
		doNode.Children = append(doNode.Children, &Node{Type: "SYMBOL", Value: "do", Line: node.Line, Column: node.Column})
		for i := 2; i < len(node.Children); i++ {
			doNode.Children = append(doNode.Children, ApplyWithContext(node.Children[i], newCtxVars))
		}
		return doNode
	}

	if head == "call" && len(ctxVars) > 0 {
		newNode := &Node{Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename}
		newNode.Children = append(newNode.Children, node.Children[0])
		if len(node.Children) > 1 {
			newNode.Children = append(newNode.Children, node.Children[1])
		}
		for _, cv := range ctxVars {
			newNode.Children = append(newNode.Children, CopyNode(cv))
		}
		for i := 2; i < len(node.Children); i++ {
			newNode.Children = append(newNode.Children, ApplyWithContext(node.Children[i], ctxVars))
		}
		return newNode
	}

	newNode := &Node{Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename}
	newNode.Children = append(newNode.Children, node.Children[0])
	for i := 1; i < len(node.Children); i++ {
		newNode.Children = append(newNode.Children, ApplyWithContext(node.Children[i], ctxVars))
	}
	return newNode
}

func Stringify(node *Node) string {
	if node == nil {
		return ""
	}
	if node.Type == "List" {
		var parts []string
		for _, c := range node.Children {
			parts = append(parts, Stringify(c))
		}
		return "(" + strings.Join(parts, " ") + ")"
	}
	if node.Type == "STRING" {
		return `"` + node.Value + `"`
	}
	return node.Value
}
