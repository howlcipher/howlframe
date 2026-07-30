package ir

import (
	"testing"
	"zero/internal/ast"
)

func TestLowerSharedRejectsMalformedFormsWithoutPanicking(t *testing.T) {
	tests := []*ast.Node{
		sharedTestNode("return"),
		sharedTestNode("if", sharedTestSymbol("true")),
		sharedTestNode("while", sharedTestSymbol("true")),
		sharedTestNode("set", sharedTestSymbol("value")),
		sharedTestNode("match", sharedTestSymbol("value")),
		sharedTestNode("match", sharedTestSymbol("value"), sharedTestNode("default")),
		sharedTestNode("sleep"),
		sharedTestNode("to_int"),
		sharedTestNode("to_float"),
		sharedTestNode("to_string"),
		sharedTestNode("bytes_to_string"),
		sharedTestNode("str_split", sharedTestSymbol("value")),
		sharedTestNode("str_join", sharedTestSymbol("values")),
		sharedTestNode("regex_match", sharedTestSymbol("pattern")),
		sharedTestNode("append", sharedTestSymbol("values")),
		sharedTestNode("map_set", sharedTestSymbol("values"), sharedTestSymbol("key")),
		sharedTestNode("map_delete", sharedTestSymbol("values")),
		sharedTestNode("map_get", sharedTestSymbol("values")),
		sharedTestNode("list_get", sharedTestSymbol("values")),
		sharedTestNode("+", sharedTestSymbol("value")),
		sharedTestNode("call"),
		sharedTestNode("let"),
		sharedTestNode("let", sharedTestSymbol("value"), sharedTestSymbol("body")),
		sharedTestNode("try_let", sharedTestNode("value", sharedTestSymbol("1")), sharedTestNode("catch", sharedTestSymbol("err"), sharedTestSymbol("body"))),
		sharedTestNode("try_let", sharedTestSymbol("value"), sharedTestNode("catch", sharedTestSymbol("err"), sharedTestSymbol("body")), sharedTestSymbol("body")),
		sharedTestNode("try_let", sharedTestNode("value", sharedTestSymbol("1")), sharedTestSymbol("err"), sharedTestSymbol("body")),
		sharedTestNode("spawn"),
		sharedTestNode("spawn", sharedTestSymbol("worker")),
		sharedTestNode("for", sharedTestSymbol("item"), sharedTestSymbol("items")),
		sharedTestNode("dict", sharedTestNode("ok", sharedTestSymbol("value")), sharedTestNode("bad")),
	}

	for _, node := range tests {
		t.Run(node.Children[0].Value, func(t *testing.T) {
			if lowered, ok := LowerShared(node); ok || lowered != nil {
				t.Fatalf("malformed form lowered: %+v", lowered)
			}
		})
	}
}

func sharedTestNode(head string, children ...*ast.Node) *ast.Node {
	return &ast.Node{
		Type:     "List",
		Children: append([]*ast.Node{sharedTestSymbol(head)}, children...),
	}
}

func sharedTestSymbol(value string) *ast.Node {
	return &ast.Node{Type: "SYMBOL", Value: value}
}
