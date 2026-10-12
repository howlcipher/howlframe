package lexer

import "testing"

// A '-' directly followed by a digit is a negative number literal; "-" alone,
// "-x" and symbols containing '-' are unchanged, and '%' lexes as a symbol.
func TestNegativeLiteralsAndPercent(t *testing.T) {
	cases := []struct {
		src  string
		want []Token
	}{
		{"-1", []Token{{Type: TokenInt, Value: "-1"}}},
		{"-2.5", []Token{{Type: TokenFloat, Value: "-2.5"}}},
		{"- 1", []Token{{Type: TokenSymbol, Value: "-"}, {Type: TokenInt, Value: "1"}}},
		{"-x", []Token{{Type: TokenSymbol, Value: "-x"}}},
		{"a-1", []Token{{Type: TokenSymbol, Value: "a-1"}}},
		{"%", []Token{{Type: TokenSymbol, Value: "%"}}},
		{"//", []Token{{Type: TokenSymbol, Value: "//"}}},
	}
	for _, c := range cases {
		l := NewLexer(c.src)
		for i, w := range c.want {
			got := l.NextToken()
			if got.Type != w.Type || got.Value != w.Value {
				t.Errorf("%q token %d: got %s %q want %s %q", c.src, i, got.Type, got.Value, w.Type, w.Value)
			}
		}
		if got := l.NextToken(); got.Type != TokenEOF {
			t.Errorf("%q: trailing token %s %q", c.src, got.Type, got.Value)
		}
	}
}
