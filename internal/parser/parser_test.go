package parser

import (
	"testing"
	"zero/internal/lexer"
)

func TestParserCreatesFloatAtom(t *testing.T) {
	parser := NewParser(lexer.NewLexer("(> score 0.8)"), "float.zero")
	root := parser.ParseExpression()
	if got := root.Children[2]; got.Type != "FLOAT" || got.Value != "0.8" {
		t.Fatalf("got float node %#v", got)
	}
}
