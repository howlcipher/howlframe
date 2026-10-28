package lexer

import (
	"strconv"
	"testing"
)

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

// String literals are UTF-8 source text. Each code point must survive
// lexing unchanged; reading the input one byte per rune re-encoded every
// byte of a multibyte character as its own Latin-1 code point.
func TestStringLiteralPreservesUTF8(t *testing.T) {
	for _, want := range []string{"Héllo", "✓ done", "日本語", "emoji 🐺", "mixed \"q\" é"} {
		src := strconv.Quote(want)
		tok := NewLexer(src).NextToken()
		if tok.Type != TokenString || tok.Value != want {
			t.Fatalf("lexing %s: got %q (%s), want %q", src, tok.Value, tok.Type, want)
		}
	}
}

func TestColumnsCountCodePointsNotBytes(t *testing.T) {
	l := NewLexer(`("é" x)`)
	l.NextToken() // (
	l.NextToken() // "é"
	tok := l.NextToken()
	if tok.Value != "x" || tok.Column != 6 {
		t.Fatalf("want symbol x at column 6, got %q at column %d", tok.Value, tok.Column)
	}
}
