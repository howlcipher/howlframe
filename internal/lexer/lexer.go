package lexer

import (
	"fmt"
	"github.com/howlcipher/howlframe/internal/ast"
	"strconv"
	"unicode"
)

type TokenType string

const (
	TokenLParen TokenType = "LPAREN"
	TokenRParen TokenType = "RPAREN"
	TokenSymbol TokenType = "SYMBOL"
	TokenInt    TokenType = "INT"
	TokenFloat  TokenType = "FLOAT"
	TokenString TokenType = "STRING"
	TokenEOF    TokenType = "EOF"
)

type Token struct {
	Type   TokenType
	Value  string
	Line   int
	Column int
}

type Lexer struct {
	input  string
	pos    int
	line   int
	column int
}

func NewLexer(input string) *Lexer {
	return &Lexer{input: input, line: 1, column: 1}
}

func (l *Lexer) nextChar() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	ch := rune(l.input[l.pos])
	l.pos++
	if ch == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
	return ch
}

func (l *Lexer) peekChar() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return rune(l.input[l.pos])
}

func (l *Lexer) NextToken() Token {
	for {
		ch := l.peekChar()
		if ch == 0 {
			return Token{Type: TokenEOF, Line: l.line, Column: l.column}
		}
		if unicode.IsSpace(ch) {
			l.nextChar()
			continue
		}
		if ch == ';' {
			for {
				c := l.peekChar()
				if c == 0 || c == '\n' {
					break
				}
				l.nextChar()
			}
			continue
		}
		break
	}

	startLine := l.line
	startCol := l.column
	ch := l.nextChar()

	if ch == '(' {
		return Token{Type: TokenLParen, Value: "(", Line: startLine, Column: startCol}
	}
	if ch == ')' {
		return Token{Type: TokenRParen, Value: ")", Line: startLine, Column: startCol}
	}
	if ch == '"' {
		raw := "\""
		for {
			nextCh := l.nextChar()
			if nextCh == 0 {
				ast.ReportError("Unterminated string", startLine, startCol)
			}
			if nextCh == '\\' {
				escapedCh := l.nextChar()
				if escapedCh == 0 {
					ast.ReportError("Unterminated string escape", startLine, startCol)
				}
				if escapedCh == '\'' {
					raw += "'"
				} else {
					raw += "\\" + string(escapedCh)
				}
				continue
			}
			raw += string(nextCh)
			if nextCh == '"' {
				break
			}
		}
		val, err := strconv.Unquote(raw)
		if err != nil {
			ast.ReportError("Invalid string literal: "+err.Error(), startLine, startCol)
		}
		return Token{Type: TokenString, Value: val, Line: startLine, Column: startCol}
	}
	if unicode.IsDigit(ch) {
		val := string(ch)
		for unicode.IsDigit(l.peekChar()) {
			val += string(l.nextChar())
		}
		if l.peekChar() == '.' {
			l.nextChar()
			if unicode.IsDigit(l.peekChar()) {
				val += "."
				for unicode.IsDigit(l.peekChar()) {
					val += string(l.nextChar())
				}
				return Token{Type: TokenFloat, Value: val, Line: startLine, Column: startCol}
			}
			// Leave a trailing dot available to the normal symbol lexer.
			l.pos--
			l.column--
		}
		return Token{Type: TokenInt, Value: val, Line: startLine, Column: startCol}
	}
	if unicode.IsLetter(ch) || ch == '_' || ch == '/' || ch == '-' || ch == '=' || ch == '.' || ch == '+' || ch == '*' || ch == '<' || ch == '>' || ch == '!' {
		val := string(ch)
		for unicode.IsLetter(l.peekChar()) || unicode.IsDigit(l.peekChar()) || l.peekChar() == '_' || l.peekChar() == '/' || l.peekChar() == '-' || l.peekChar() == '=' || l.peekChar() == '.' || l.peekChar() == '+' || l.peekChar() == '*' || l.peekChar() == '<' || l.peekChar() == '>' || l.peekChar() == '!' {
			val += string(l.nextChar())
		}
		return Token{Type: TokenSymbol, Value: val, Line: startLine, Column: startCol}
	}

	ast.ReportError(fmt.Sprintf("Unexpected character: %c", ch), startLine, startCol)
	return Token{}
}
