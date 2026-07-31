package lexer

import "testing"

func TestLexerTokenizesDecimalAsFloat(t *testing.T) {
	lexer := NewLexer("0.8 42 1.")
	tests := []struct {
		typ   TokenType
		value string
	}{
		{TokenFloat, "0.8"},
		{TokenInt, "42"},
		{TokenInt, "1"},
		{TokenSymbol, "."},
		{TokenEOF, ""},
	}

	for _, want := range tests {
		got := lexer.NextToken()
		if got.Type != want.typ || got.Value != want.value {
			t.Fatalf("got token %#v, want type=%q value=%q", got, want.typ, want.value)
		}
	}
}
