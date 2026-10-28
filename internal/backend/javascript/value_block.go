package javascript

import (
	"fmt"

	"github.com/howlcipher/howlframe/internal/ast"
)

// valueBlockHead marks a let, do or if that the build found in value
// position. It is internal: WrapValueForms inserts it after checking.
const valueBlockHead = "__js_value"

// WrapValueForms marks each value-position let, do or if so the backend emits
// it as a value block. The nodes come from the build's value-position walk; an
// outer form is wrapped in place and keeps its children, so nested forms in the
// list are still reached.
func WrapValueForms(forms []*ast.Node) {
	for _, n := range forms {
		inner := *n
		n.Children = []*ast.Node{
			{Type: "SYMBOL", Value: valueBlockHead, Line: n.Line, Column: n.Column, Filename: n.Filename},
			&inner,
		}
	}
}

// jsValueBlock emits a value-position form as an awaited async arrow whose
// body is the form with its value returned. Every function the backend emits
// is async, so the await is always legal where a value is.
func jsValueBlock(node *ast.Node, reqVar string, depth int) string {
	body := generateJSStatement(jsReturning(node.Children[1]), reqVar, depth+1)
	return fmt.Sprintf("(await (async () => {\n%s\n})())", body)
}

// jsReturning rewrites a form so its value is returned: the body of a let,
// the last form of a do, and both branches of an if. A do with no forms and an
// if with no else branch yield no value, as on the other targets.
func jsReturning(node *ast.Node) *ast.Node {
	if node.Type == "List" && len(node.Children) > 0 && node.Children[0].Type == "SYMBOL" {
		switch node.Children[0].Value {
		case valueBlockHead:
			return jsReturning(node.Children[1])
		case "let", "do":
			if len(node.Children) < 2 {
				return node
			}
			out := *node
			out.Children = append([]*ast.Node(nil), node.Children...)
			last := len(out.Children) - 1
			out.Children[last] = jsReturning(out.Children[last])
			return &out
		case "if":
			out := *node
			out.Children = append([]*ast.Node(nil), node.Children...)
			for i := 2; i < len(out.Children) && i <= 3; i++ {
				out.Children[i] = jsReturning(out.Children[i])
			}
			return &out
		}
	}
	return &ast.Node{
		Type:     "List",
		Children: []*ast.Node{{Type: "SYMBOL", Value: "return", Line: node.Line, Column: node.Column, Filename: node.Filename}, node},
		Line:     node.Line, Column: node.Column, Filename: node.Filename,
	}
}
