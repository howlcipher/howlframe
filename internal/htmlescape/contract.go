package htmlescape

import (
	"strconv"
	"strings"

	"github.com/howlcipher/howlframe/internal/ast"
)

// Piece is one fragment of a concatenated markup string.
// A constant is trusted text. An escape call is still untrusted inside a
// <script> body or an inline handler: HTML encoding is not JavaScript encoding.
// Data-attribute values are not handler bodies.
type Piece struct {
	Text     string
	Constant bool
	Escape   string
}

type scanMode int

const (
	modeText scanMode = iota
	modeScript
	modeHandler
	modeAttr
)

// ScanHandlers walks concatenated markup. It returns the constant names used
// as inline event handlers, and a violation when a non-constant is placed
// inside a <script> element or an inline handler attribute.
func ScanHandlers(pieces []Piece) (names []string, violation string) {
	var s handlerScan
	for _, piece := range pieces {
		if piece.Constant {
			s.feed(piece.Text, &names)
			continue
		}
		switch s.mode {
		case modeScript:
			return names, "untrusted value inside <script>"
		case modeHandler:
			return names, "untrusted value inside inline handler"
		}
	}
	return names, ""
}

type handlerScan struct {
	mode    scanMode
	quote   byte
	handler strings.Builder
}

func (s *handlerScan) feed(text string, names *[]string) {
	for i := 0; i < len(text); {
		if s.mode == modeHandler || s.mode == modeAttr {
			if s.quote != 0 && text[i] == s.quote {
				if s.mode == modeHandler {
					*names = append(*names, s.handler.String())
					s.handler.Reset()
				}
				s.mode = modeText
				s.quote = 0
				i++
				continue
			}
			if s.quote == 0 && (text[i] == ' ' || text[i] == '\t' || text[i] == '>') {
				if s.mode == modeHandler {
					*names = append(*names, s.handler.String())
					s.handler.Reset()
				}
				s.mode = modeText
				continue
			}
			if s.mode == modeHandler {
				s.handler.WriteByte(text[i])
			}
			i++
			continue
		}
		if s.mode == modeScript {
			if closeAt := scriptClose(text[i:]); closeAt >= 0 {
				i += closeAt
				s.mode = modeText
				continue
			}
			i++
			continue
		}
		if openAt := scriptOpen(text[i:]); openAt >= 0 {
			i += openAt
			s.mode = modeScript
			continue
		}
		if quoteAt, quote, ok := inlineHandler(text[i:]); ok && boundaryBefore(text, i) {
			i += quoteAt
			s.mode = modeHandler
			s.quote = quote
			continue
		}
		if quoteAt, quote, ok := quotedAttribute(text[i:]); ok && boundaryBefore(text, i) {
			i += quoteAt
			s.mode = modeAttr
			s.quote = quote
			continue
		}
		i++
	}
}

func boundaryBefore(text string, i int) bool {
	if i == 0 {
		return true
	}
	prev := text[i-1]
	return prev == ' ' || prev == '\t' || prev == '<' || prev == '>' || prev == '"' || prev == '\''
}

func scriptOpen(s string) int {
	low := strings.ToLower(s)
	if !strings.HasPrefix(low, "<script") {
		return -1
	}
	rest := s[len("<script"):]
	rel := strings.IndexByte(rest, '>')
	if rel < 0 {
		return len(s)
	}
	return len("<script") + rel + 1
}

func scriptClose(s string) int {
	low := strings.ToLower(s)
	if !strings.HasPrefix(low, "</script") {
		return -1
	}
	rest := s[len("</script"):]
	rel := strings.IndexByte(rest, '>')
	if rel < 0 {
		return len(s)
	}
	return len("</script") + rel + 1
}

func inlineHandler(s string) (advance int, quote byte, ok bool) {
	low := strings.ToLower(s)
	if !strings.HasPrefix(low, "on") || len(s) < 3 || !isAlpha(s[2]) {
		return 0, 0, false
	}
	i := 2
	for i < len(s) && isAlpha(s[i]) {
		i++
	}
	if i == 2 {
		return 0, 0, false
	}
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	if i >= len(s) || s[i] != '=' {
		return 0, 0, false
	}
	i++
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	if i < len(s) && (s[i] == '"' || s[i] == '\'') {
		return i + 1, s[i], true
	}
	return i, 0, true
}

func quotedAttribute(s string) (advance int, quote byte, ok bool) {
	if len(s) == 0 || !isNameStart(s[0]) {
		return 0, 0, false
	}
	i := 1
	for i < len(s) && isNameCont(s[i]) {
		i++
	}
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	if i >= len(s) || s[i] != '=' {
		return 0, 0, false
	}
	i++
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	if i >= len(s) || (s[i] != '"' && s[i] != '\'') {
		return 0, 0, false
	}
	return i + 1, s[i], true
}

func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isNameStart(c byte) bool {
	return isAlpha(c) || c == '_' || c == ':'
}

func isNameCont(c byte) bool {
	return isNameStart(c) || (c >= '0' && c <= '9') || c == '-'
}

// ProgramMarkupPieces finds a str_join that builds handler or data-attribute
// markup and returns its fragments in order. The separator is inserted between
// fragments when it is a non-empty string.
func ProgramMarkupPieces(root *ast.Node) ([]Piece, bool) {
	var found []Piece
	var walk func(*ast.Node)
	walk = func(node *ast.Node) {
		if node == nil {
			return
		}
		if pieces, ok := strJoinPieces(node); ok && hasMarkup(pieces) && len(pieces) >= len(found) {
			found = pieces
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(root)
	return found, len(found) > 0
}

func strJoinPieces(node *ast.Node) ([]Piece, bool) {
	if node.Type != "List" || len(node.Children) < 2 || node.Children[0].Type != "SYMBOL" || node.Children[0].Value != "str_join" {
		return nil, false
	}
	list := node.Children[1]
	if list.Type != "List" || len(list.Children) == 0 || list.Children[0].Type != "SYMBOL" || list.Children[0].Value != "list" {
		return nil, false
	}
	var pieces []Piece
	for _, item := range list.Children[1:] {
		pieces = append(pieces, pieceFromNode(item))
	}
	if len(node.Children) >= 3 && node.Children[2].Type == "STRING" && node.Children[2].Value != "" {
		sep := Piece{Constant: true, Text: node.Children[2].Value}
		joined := make([]Piece, 0, len(pieces)*2)
		for i, piece := range pieces {
			if i > 0 {
				joined = append(joined, sep)
			}
			joined = append(joined, piece)
		}
		pieces = joined
	}
	return pieces, true
}

func pieceFromNode(node *ast.Node) Piece {
	if node == nil {
		return Piece{}
	}
	if node.Type == "STRING" {
		return Piece{Constant: true, Text: node.Value}
	}
	if node.Type == "List" && len(node.Children) > 0 && node.Children[0].Type == "SYMBOL" {
		head := node.Children[0].Value
		if head == "html_escape" || head == "attr_escape" {
			return Piece{Escape: head}
		}
	}
	return Piece{}
}

func hasMarkup(pieces []Piece) bool {
	for _, piece := range pieces {
		if !piece.Constant {
			continue
		}
		low := strings.ToLower(piece.Text)
		if strings.Contains(low, "<script") || strings.Contains(low, "onclick") || strings.Contains(low, "onerror") || strings.Contains(low, "onload") || strings.Contains(piece.Text, "data-") || strings.Contains(piece.Text, "<") {
			return true
		}
	}
	return false
}

// ParseMarkupExpr reads one backend expression: a []string or JS list, or a
// + chain. String literals are constants. howlFrameHTMLEscape calls record
// the escape kind. Anything else is untrusted.
func ParseMarkupExpr(expr string) ([]Piece, bool) {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "[]string{") {
		body, ok := takeUntilClose(expr[len("[]string{"):], '}')
		if !ok {
			return nil, false
		}
		return piecesFromElements(splitTop(body, ',')), true
	}
	if strings.HasPrefix(expr, "[") {
		body, ok := takeUntilClose(expr[1:], ']')
		if !ok {
			return nil, false
		}
		pieces := piecesFromElements(splitTop(body, ','))
		if !hasMarkup(pieces) {
			return nil, false
		}
		return pieces, true
	}
	parts := splitTop(expr, '+')
	if len(parts) < 2 {
		return nil, false
	}
	pieces := piecesFromElements(parts)
	if !hasMarkup(pieces) {
		return nil, false
	}
	return pieces, true
}

// InspectBackendMarkup scans generated Go or JavaScript for markup lists and
// + chains. found is false when the source has no such expression.
func InspectBackendMarkup(src string) (names []string, violation string, found bool) {
	for _, expr := range markupExprs(src) {
		pieces, ok := ParseMarkupExpr(expr)
		if !ok {
			continue
		}
		found = true
		got, bad := ScanHandlers(pieces)
		names = append(names, got...)
		if bad != "" && violation == "" {
			violation = bad
		}
	}
	return names, violation, found
}

func markupExprs(src string) []string {
	var exprs []string
	for from := 0; from < len(src); {
		listAt := strings.Index(src[from:], "[]string{")
		bracketAt := indexBracket(src[from:])
		next, kind := earliest(listAt, bracketAt)
		if next < 0 {
			break
		}
		abs := from + next
		if kind == 0 {
			body, ok := takeUntilClose(src[abs+len("[]string{"):], '}')
			if !ok {
				from = abs + len("[]string{")
				continue
			}
			exprs = append(exprs, "[]string{"+body+"}")
			from = abs + len("[]string{") + len(body) + 1
			continue
		}
		body, ok := takeUntilClose(src[abs+1:], ']')
		if !ok {
			from = abs + 1
			continue
		}
		expr := "[" + body + "]"
		if pieces, parsed := ParseMarkupExpr(expr); parsed && hasMarkup(pieces) {
			exprs = append(exprs, expr)
		}
		from = abs + 1 + len(body) + 1
	}
	return exprs
}

func earliest(listAt, bracketAt int) (int, int) {
	if listAt < 0 {
		return bracketAt, 1
	}
	if bracketAt < 0 || listAt <= bracketAt {
		return listAt, 0
	}
	return bracketAt, 1
}

func indexBracket(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] != '[' {
			continue
		}
		if strings.HasPrefix(s[i:], "[]string") {
			continue
		}
		return i
	}
	return -1
}

func piecesFromElements(elements []string) []Piece {
	var pieces []Piece
	for _, element := range elements {
		element = strings.TrimSpace(element)
		if element == "" {
			continue
		}
		if element[0] == '"' {
			if text, err := strconv.Unquote(element); err == nil {
				pieces = append(pieces, Piece{Constant: true, Text: text})
				continue
			}
		}
		if strings.Contains(element, "howlFrameHTMLEscape") {
			kind := "html_escape"
			if strings.Contains(element, `"attr_escape"`) || strings.Contains(element, "`attr_escape`") {
				kind = "attr_escape"
			}
			pieces = append(pieces, Piece{Escape: kind})
			continue
		}
		pieces = append(pieces, Piece{})
	}
	return pieces
}

func splitTop(s string, sep byte) []string {
	var parts []string
	depth := 0
	inStr := false
	esc := false
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			continue
		}
		switch c {
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			if depth > 0 {
				depth--
			}
		default:
			if c == sep && depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func takeUntilClose(s string, close byte) (string, bool) {
	depth := 1
	inStr := false
	esc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			continue
		}
		switch c {
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			depth--
			if depth == 0 {
				if c == close {
					return s[:i], true
				}
				return "", false
			}
		}
	}
	return "", false
}
