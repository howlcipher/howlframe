//go:build ignore

package main

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func init() {
	gob.Register(float64(0))
	gob.Register("")
}

type TokenType string

const (
	TokenLParen TokenType = "LPAREN"
	TokenRParen TokenType = "RPAREN"
	TokenSymbol TokenType = "SYMBOL"
	TokenInt    TokenType = "INT"
	TokenString TokenType = "STRING"
	TokenEOF    TokenType = "EOF"
)

type Token struct {
	Type   TokenType
	Value  string
	Line   int
	Column int
}

type ErrorOutput struct {
	Reason string `json:"reason"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

func reportError(reason string, line, column int) {
	errOut := ErrorOutput{Reason: reason, Line: line, Column: column}
	b, _ := json.Marshal(errOut)
	fmt.Println(string(b))
	os.Exit(1)
}

// Lexer
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
				reportError("Unterminated string", startLine, startCol)
			}
			if nextCh == '\\' {
				escapedCh := l.nextChar()
				if escapedCh == 0 {
					reportError("Unterminated string escape", startLine, startCol)
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
			reportError("Invalid string literal: "+err.Error(), startLine, startCol)
		}
		return Token{Type: TokenString, Value: val, Line: startLine, Column: startCol}
	}
	if unicode.IsDigit(ch) {
		val := string(ch)
		for unicode.IsDigit(l.peekChar()) {
			val += string(l.nextChar())
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

	reportError(fmt.Sprintf("Unexpected character: %c", ch), startLine, startCol)
	return Token{}
}

// AST
type Node struct {
	Type     string
	Value    string
	Children []*Node
	Line     int
	Column   int
	Filename string
}

// Parser
type Parser struct {
	lexer    *Lexer
	cur      Token
	filename string
}

func NewParser(lexer *Lexer, filename string) *Parser {
	p := &Parser{lexer: lexer, filename: filename}
	p.cur = p.lexer.NextToken()
	return p
}

func (p *Parser) parseExpression() *Node {
	if p.cur.Type == TokenLParen {
		node := &Node{Type: "List", Line: p.cur.Line, Column: p.cur.Column, Filename: p.filename}
		p.cur = p.lexer.NextToken() // consume '('
		for p.cur.Type != TokenRParen && p.cur.Type != TokenEOF {
			node.Children = append(node.Children, p.parseExpression())
		}
		if p.cur.Type != TokenRParen {
			reportError("Expected ')'", p.cur.Line, p.cur.Column)
		}
		p.cur = p.lexer.NextToken() // consume ')'
		return node
	}
	if p.cur.Type == TokenSymbol || p.cur.Type == TokenInt || p.cur.Type == TokenString {
		node := &Node{Type: string(p.cur.Type), Value: p.cur.Value, Line: p.cur.Line, Column: p.cur.Column, Filename: p.filename}
		p.cur = p.lexer.NextToken()
		return node
	}
	reportError(fmt.Sprintf("Unexpected token: %s", p.cur.Value), p.cur.Line, p.cur.Column)
	return nil
}

// AST Utilities
func copyNode(n *Node) *Node {
	if n == nil {
		return nil
	}
	clone := &Node{Type: n.Type, Value: n.Value, Line: n.Line, Column: n.Column, Filename: n.Filename}
	for _, child := range n.Children {
		clone.Children = append(clone.Children, copyNode(child))
	}
	return clone
}

func replaceNext(node *Node, replacement *Node) {
	if node == nil {
		return
	}
	for i, child := range node.Children {
		if child.Type == "List" && len(child.Children) == 1 && child.Children[0].Value == "next" {
			node.Children[i] = replacement
		} else {
			replaceNext(child, replacement)
		}
	}
}

func renameVar(node *Node, oldName, newName string) {
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
		renameVar(child, oldName, newName)
	}
}

// Code Generator
var currentSchemaDDLs []string

func generateCode(node *Node) (string, string) {
	currentSchemaDDLs = nil
	if node.Type != "List" || len(node.Children) == 0 {
		reportError("Expected list at root", node.Line, node.Column)
	}
	head := node.Children[0]
	if head.Type != "SYMBOL" || (head.Value != "http_server" && head.Value != "cli_app") {
		reportError("Expected http_server or cli_app as root symbol", head.Line, head.Column)
	}

	isCliApp := head.Value == "cli_app"

	var portNode *Node
	var startIndex int
	if isCliApp {
		startIndex = 1
	} else {
		if len(node.Children) < 3 {
			reportError("http_server expects at least a port and 1 route", head.Line, head.Column)
		}
		portNode = node.Children[1]
		if portNode.Type != "INT" {
			reportError("Expected integer for port", portNode.Line, portNode.Column)
		}
		startIndex = 2
	}

	var funcsCode string
	var routesCode string
	var cliCode string
	var testCode string
	var extraImports []string
	defaultImports := map[string]bool{
		"bytes":         true,
		"database/sql":  true,
		"encoding/json": true,
		"fmt":           true,
		"io":            true,
		"net/http":      true,
		"os":            true,
		"os/exec":       true,
		"regexp":        true,
		"runtime":       true,
		"runtime/debug": true,
		"strconv":       true,
		"strings":       true,
		"time":          true,
	}
	seenImports := make(map[string]bool)

	for i := startIndex; i < len(node.Children); i++ {
		handlerNode := node.Children[i]
		if handlerNode.Type != "List" || len(handlerNode.Children) == 0 {
			reportError("Expected route, defun, struct, import, test, or middleware definition", handlerNode.Line, handlerNode.Column)
		}

		head := handlerNode.Children[0].Value

		if head == "intent" {
			continue
		}

		if head == "test" {
			if len(handlerNode.Children) < 3 {
				reportError("test expects (test \"description\" body...)", handlerNode.Line, handlerNode.Column)
			}
			descNode := handlerNode.Children[1]
			if descNode.Type != "STRING" {
				reportError("test description must be a string", descNode.Line, descNode.Column)
			}
			desc := descNode.Value
			safeDesc := ""
			lastWasUnderscore := false
			for _, r := range desc {
				if unicode.IsLetter(r) || unicode.IsDigit(r) {
					safeDesc += string(r)
					lastWasUnderscore = false
				} else {
					if !lastWasUnderscore {
						safeDesc += "_"
						lastWasUnderscore = true
					}
				}
			}
			safeDesc = strings.Trim(safeDesc, "_")
			testFuncName := "Test"
			if len(safeDesc) > 0 {
				testFuncName += "_" + safeDesc
			}

			var testBodyCode string
			for j := 2; j < len(handlerNode.Children); j++ {
				testBodyCode += generateStatement(handlerNode.Children[j], "", 0) + "\n"
			}
			testCode += fmt.Sprintf("func %s(t *testing.T) {\n%s\n}\n\n", testFuncName, testBodyCode)
			continue
		}

		if head == "import" {
			if len(handlerNode.Children) != 2 {
				reportError("import expects (import \"pkg\")", handlerNode.Line, handlerNode.Column)
			}
			pkgNode := handlerNode.Children[1]
			if pkgNode.Type != "STRING" {
				reportError("import package must be a string", pkgNode.Line, pkgNode.Column)
			}
			pkg := pkgNode.Value
			if !defaultImports[pkg] && !seenImports[pkg] {
				seenImports[pkg] = true
				extraImports = append(extraImports, pkg)
			}
			continue
		}

		if head == "struct" {
			if len(handlerNode.Children) < 2 {
				reportError("struct expects (struct Name (field type)...)", handlerNode.Line, handlerNode.Column)
			}
			name := handlerNode.Children[1].Value
			funcsCode += fmt.Sprintf("type %s struct {\n", name)
			for j := 2; j < len(handlerNode.Children); j++ {
				fieldNode := handlerNode.Children[j]
				if fieldNode.Type != "List" || len(fieldNode.Children) != 2 {
					reportError("struct field expects (name type)", fieldNode.Line, fieldNode.Column)
				}
				fieldName := fieldNode.Children[0].Value
				fieldType := fieldNode.Children[1].Value
				if len(fieldName) > 0 {
					fieldName = strings.ToUpper(fieldName[:1]) + fieldName[1:]
				}
				funcsCode += fmt.Sprintf("\t%s %s\n", fieldName, fieldType)
			}
			funcsCode += "}\n\n"
			continue
		}

		if head == "schema" {
			if len(handlerNode.Children) < 2 {
				reportError("schema expects (schema \"tableName\" (column \"name\" \"type\")...)", handlerNode.Line, handlerNode.Column)
			}
			tableName := handlerNode.Children[1].Value
			structName := tableName
			if len(structName) > 0 {
				structName = strings.ToUpper(structName[:1]) + structName[1:]
			}
			funcsCode += fmt.Sprintf("type %s struct {\n", structName)

			var columns []string
			for j := 2; j < len(handlerNode.Children); j++ {
				colNode := handlerNode.Children[j]
				if colNode.Type != "List" {
					reportError("schema column expects (column name type) or (name type)", colNode.Line, colNode.Column)
				}
				var colName, colType string
				if len(colNode.Children) == 3 && colNode.Children[0].Value == "column" {
					colName = colNode.Children[1].Value
					colType = colNode.Children[2].Value
				} else if len(colNode.Children) == 2 {
					colName = colNode.Children[0].Value
					colType = colNode.Children[1].Value
				} else {
					reportError("schema column expects (column name type) or (name type)", colNode.Line, colNode.Column)
				}

				goFieldName := colName
				if len(goFieldName) > 0 {
					goFieldName = strings.ToUpper(goFieldName[:1]) + goFieldName[1:]
				}
				funcsCode += fmt.Sprintf("\t%s %s\n", goFieldName, colType)

				sqlType := colType
				if sqlType == "string" {
					sqlType = "TEXT"
				} else if sqlType == "int" {
					sqlType = "INTEGER"
				} else if sqlType == "float" || sqlType == "float64" {
					sqlType = "REAL"
				} else if sqlType == "bool" {
					sqlType = "BOOLEAN"
				}
				columns = append(columns, fmt.Sprintf("%s %s", colName, sqlType))
			}
			funcsCode += "}\n\n"

			ddl := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s);", tableName, strings.Join(columns, ", "))
			currentSchemaDDLs = append(currentSchemaDDLs, ddl)
			continue
		}

		if head == "defun" {
			if len(handlerNode.Children) < 4 {
				reportError("defun expects (defun name (args) body)", handlerNode.Line, handlerNode.Column)
			}
			name := handlerNode.Children[1].Value
			argsNode := handlerNode.Children[2]

			returnType := "string"
			bodyStartIndex := 3
			if len(handlerNode.Children) > 4 && handlerNode.Children[3].Type == "SYMBOL" {
				returnType = handlerNode.Children[3].Value
				bodyStartIndex = 4
			}

			typeHints := make(map[string]string)
			var typeParams []string
			for j := bodyStartIndex; j < len(handlerNode.Children)-1; j++ {
				cfgNode := handlerNode.Children[j]
				if cfgNode.Type == "List" && len(cfgNode.Children) >= 3 && cfgNode.Children[0].Value == "type_hint" {
					varName := cfgNode.Children[1].Value
					varType := cfgNode.Children[2].Value
					typeHints[varName] = varType
				} else if cfgNode.Type == "List" && len(cfgNode.Children) >= 2 && cfgNode.Children[0].Value == "type_param" {
					typeParams = append(typeParams, cfgNode.Children[1].Value)
				} else if cfgNode.Type == "List" && len(cfgNode.Children) >= 1 && cfgNode.Children[0].Value == "type_hints" {
					for k := 1; k < len(cfgNode.Children); k++ {
						hintPair := cfgNode.Children[k]
						if hintPair.Type == "List" && len(hintPair.Children) >= 2 {
							typeHints[hintPair.Children[0].Value] = hintPair.Children[1].Value
						}
					}
				}
			}

			typeParamsStr := ""
			if len(typeParams) > 0 {
				var typed []string
				for _, tp := range typeParams {
					typed = append(typed, tp+" any")
				}
				typeParamsStr = "[" + strings.Join(typed, ", ") + "]"
			}

			var argsList []string
			for _, arg := range argsNode.Children {
				var argName string
				argType := "string"
				if arg.Type == "List" && len(arg.Children) >= 2 {
					argName = arg.Children[0].Value
					argType = arg.Children[1].Value
				} else {
					argName = arg.Value
				}
				if t, ok := typeHints[argName]; ok {
					argType = t
				}
				argsList = append(argsList, argName+" "+argType)
			}
			argsStr := strings.Join(argsList, ", ")

			if t, ok := typeHints["return"]; ok {
				returnType = t
			}
			returnTypeStr := " " + returnType
			if returnType == "void" {
				returnTypeStr = ""
			}

			bodyNode := handlerNode.Children[len(handlerNode.Children)-1]
			bodyCode := generateStatement(bodyNode, "", 0)
			traceArgs := "map[string]any{"
			for _, arg := range argsList {
				argName := strings.Split(arg, " ")[0]
				traceArgs += fmt.Sprintf("%q: %s, ", argName, argName)
			}
			traceArgs += "}"
			traceInject := fmt.Sprintf("\tdefer observer.Trace(%q, %s)()\n", name, traceArgs)
			funcsCode += fmt.Sprintf("//line %s:%d\nfunc %s%s(%s)%s {\n%s%s\n}\n\n", handlerNode.Filename, handlerNode.Line, name, typeParamsStr, argsStr, returnTypeStr, traceInject, bodyCode)
			continue
		}

		if head == "route" {
			if len(handlerNode.Children) != 3 {
				reportError("route expects (route path handler)", handlerNode.Line, handlerNode.Column)
			}
			pathNode := handlerNode.Children[1]
			if pathNode.Type != "STRING" {
				reportError("route path must be a string", pathNode.Line, pathNode.Column)
			}
			reqNodeList := handlerNode.Children[2].Children[1]
			if reqNodeList.Type != "List" || len(reqNodeList.Children) != 1 {
				reportError("Expected exactly 1 argument in lambda (req)", reqNodeList.Line, reqNodeList.Column)
			}
			reqVar := reqNodeList.Children[0].Value
			bodyNode := handlerNode.Children[2].Children[2]
			bodyCode := generateStatement(bodyNode, reqVar, 0)
			traceInject := fmt.Sprintf("\t\tdefer observer.Trace(%q, map[string]any{%q: %s.URL.Path})()\n", "route:"+pathNode.Value, reqVar, reqVar)
			routesCode += fmt.Sprintf(`	http.HandleFunc(%q, func(w http.ResponseWriter, %s *http.Request) {
%s%s
	})
`, pathNode.Value, reqVar, traceInject, bodyCode)
			continue
		}

		if head == "middleware" {
			if len(handlerNode.Children) < 3 {
				reportError("middleware expects (middleware (lambda (req) body) routes...)", handlerNode.Line, handlerNode.Column)
			}
			lambdaNode := handlerNode.Children[1]
			if lambdaNode.Type != "List" || len(lambdaNode.Children) != 3 || lambdaNode.Children[0].Value != "lambda" {
				reportError("middleware expects a lambda", lambdaNode.Line, lambdaNode.Column)
			}
			reqNodeList := lambdaNode.Children[1]
			if reqNodeList.Type != "List" || len(reqNodeList.Children) != 1 {
				reportError("middleware lambda expects exactly 1 argument", reqNodeList.Line, reqNodeList.Column)
			}
			mwReqVar := reqNodeList.Children[0].Value
			mwBodyNode := lambdaNode.Children[2]

			for j := 2; j < len(handlerNode.Children); j++ {
				routeNode := handlerNode.Children[j]
				if routeNode.Type != "List" || len(routeNode.Children) == 0 || routeNode.Children[0].Value != "route" {
					reportError("middleware block can only contain routes", routeNode.Line, routeNode.Column)
				}
				if len(routeNode.Children) != 3 {
					reportError("route expects (route path handler)", routeNode.Line, routeNode.Column)
				}
				pathNode := routeNode.Children[1]
				if pathNode.Type != "STRING" {
					reportError("route path must be a string", pathNode.Line, pathNode.Column)
				}

				routeLambdaNode := routeNode.Children[2]
				routeReqList := routeLambdaNode.Children[1]
				routeReqVar := routeReqList.Children[0].Value
				routeBodyNode := routeLambdaNode.Children[2]

				clonedMwBody := copyNode(mwBodyNode)
				clonedRouteBody := copyNode(routeBodyNode)
				if routeReqVar != mwReqVar {
					renameVar(clonedRouteBody, routeReqVar, mwReqVar)
				}

				replaceNext(clonedMwBody, clonedRouteBody)
				combinedCode := generateStatement(clonedMwBody, mwReqVar, 0)
				traceInject := fmt.Sprintf("\t\tdefer observer.Trace(%q, map[string]any{%q: %s.URL.Path})()\n", "middleware_route:"+pathNode.Value, mwReqVar, mwReqVar)

				routesCode += fmt.Sprintf(`	http.HandleFunc(%q, func(w http.ResponseWriter, %s *http.Request) {
%s%s
	})
`, pathNode.Value, mwReqVar, traceInject, combinedCode)
			}
			continue
		}

		if isCliApp {
			// For cli_app, unhandled blocks are treated as statements executed in main
			cliCode += generateStatement(handlerNode, "", 0) + "\n"
			continue
		}

		reportError("Expected route, defun, struct, import, test, or middleware block", handlerNode.Line, handlerNode.Column)
	}

	code := `package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
	"zero/observer"
`
	for _, imp := range extraImports {
		code += fmt.Sprintf("\t%q\n", imp)
	}
	code += `)
`
	code += funcsCode
	code += `func main() {
	defer func() {
		if r := recover(); r != nil {
			crashData := struct {
				Error string
				Stack string
			}{
				Error: fmt.Sprintf("%v", r),
				Stack: string(debug.Stack()),
			}
			dump, _ := json.Marshal(crashData)
			_ = os.WriteFile("crash.json", dump, 0644)
			os.Exit(1)
		}
	}()
	var _ = runtime.GOOS
	var _ = debug.Stack
	var _ = sql.Open
	var _ = os.Getenv
	var _ = json.Marshal
	var _ = io.ReadAll
	var _ = bytes.NewBuffer
	var _ = http.DefaultClient
	var _ = exec.Command
	var _ = regexp.MatchString
	var _ = strings.Split
	var _ = time.Sleep
	var _ = strconv.Atoi
	var _ = fmt.Println
	var _ = observer.Trace
`
	if isCliApp {
		code += cliCode
		code += "}\n"
	} else {
		code += routesCode
		code += fmt.Sprintf(`	
	fmt.Println("Starting server on port %s...")
	if err := http.ListenAndServe(":%s", nil); err != nil {
		fmt.Println("Server error:", err)
	}
}
`, portNode.Value, portNode.Value)
	}

	if testCode != "" {
		fullTestCode := `package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"zero/observer"
`
		for _, imp := range extraImports {
			parts := strings.Split(imp, "/")
			pkgName := parts[len(parts)-1]
			if strings.Contains(testCode, pkgName+".") {
				fullTestCode += fmt.Sprintf("\t%q\n", imp)
			}
		}
		fullTestCode += `)

var _ = sql.Open
var _ = os.Getenv
var _ = json.Marshal
var _ = io.ReadAll
var _ = bytes.NewBuffer
var _ = http.DefaultClient
var _ = exec.Command
var _ = regexp.MatchString
var _ = strings.Split
var _ = time.Sleep
var _ = strconv.Atoi
var _ = fmt.Println
var _ = observer.Trace

` + testCode
		testCode = fullTestCode
	}

	return code, testCode
}

func generateStatement(node *Node, reqVar string, depth int) string {
	code := generateStatementRaw(node, reqVar, depth)
	if node.Type != "List" || len(node.Children) == 0 {
		return code
	}
	head := node.Children[0].Value
	switch head {
	case "return", "res_json", "res", "let", "do", "try_let", "spawn", "if", "print", "db_connect", "sql_query", "append", "map_set", "map_delete", "for", "sleep", "write_file", "mkdir", "exec", "while", "match", "set", "call", "cli_args":
		if node.Filename != "" {
			return fmt.Sprintf("//line %s:%d\n%s", node.Filename, node.Line, code)
		}
	}
	return code
}

func generateExpression(node *Node, reqVar string, depth int) string {
	return generateStatementRaw(node, reqVar, depth)
}

// IRNode is a backend-agnostic intermediate representation for the subset of
// AST node kinds whose semantics are identical across every codegen backend
// (control flow and simple expressions). Kids holds positional pointers into
// the original AST rather than pre-rendered strings, so each backend still
// decides whether a given child renders through the line-directive-wrapped
// statement path (generateStatement/generateJSStatement) or the raw
// expression path (generateStatementRaw/generateExpression and their JS
// equivalents) — that choice is backend-specific (see bug #19) and is not
// itself part of the shared shape.
//
// Node kinds with real per-backend divergence (let, try_let, call, for,
// spawn) and backend-native runtime primitives with no cross-backend meaning
// (db_connect, read_file, llm_generate, fetch, res_json, ...) are not lowered
// here and keep their own per-backend implementation. See improvements.md
// item #53 for the Phase 1 scoping rationale.
type IRNode struct {
	Kind  string
	Op    string
	Kids  []*Node
	Cases []irCase
}

type irCase struct {
	Label     *Node
	Body      *Node
	IsDefault bool
}

var binOpKinds = map[string]bool{
	"+": true, "-": true, "*": true, "/": true,
	"<": true, ">": true, "<=": true, ">=": true,
	"and": true, "or": true, "==": true, "!=": true, "=": true,
}

// lowerShared performs the shared arity/shape validation and child
// extraction for the ~19 node kinds with identical cross-backend semantics.
// It intentionally does not unify the small number of validation asymmetries
// already present between the Go and JS backends (e.g. Go type-checks the
// append/map_set/map_delete target is a SYMBOL and JS does not; JS's binop
// arity check is absent where Go's exists) — preserving each backend's exact
// pre-refactor behavior takes priority over cosmetic validation parity, so
// those extra checks stay in the per-backend emit functions.
func lowerShared(node *Node) (*IRNode, bool) {
	if node.Type != "List" || len(node.Children) == 0 {
		return nil, false
	}
	head := node.Children[0].Value
	if binOpKinds[head] {
		return &IRNode{Kind: "binop", Op: head, Kids: node.Children[1:]}, true
	}
	switch head {
	case "return":
		if len(node.Children) != 2 {
			reportError("return expects (return val)", node.Line, node.Column)
		}
		return &IRNode{Kind: "return", Kids: []*Node{node.Children[1]}}, true
	case "if":
		if len(node.Children) != 3 && len(node.Children) != 4 {
			reportError("if expects (if cond then) or (if cond then else)", node.Line, node.Column)
		}
		return &IRNode{Kind: "if", Kids: node.Children[1:]}, true
	case "while":
		if len(node.Children) != 3 {
			reportError("while expects (while cond body)", node.Line, node.Column)
		}
		return &IRNode{Kind: "while", Kids: node.Children[1:]}, true
	case "do":
		return &IRNode{Kind: "do", Kids: node.Children[1:]}, true
	case "set":
		if len(node.Children) != 3 {
			reportError("set expects (set var val)", node.Line, node.Column)
		}
		return &IRNode{Kind: "set", Kids: node.Children[1:]}, true
	case "match":
		if len(node.Children) < 3 {
			reportError("match expects (match var (val body)...)", node.Line, node.Column)
		}
		ir := &IRNode{Kind: "match", Kids: []*Node{node.Children[1]}}
		for j := 2; j < len(node.Children); j++ {
			caseNode := node.Children[j]
			if caseNode.Type != "List" || len(caseNode.Children) != 2 {
				reportError("match case expects (val body)", caseNode.Line, caseNode.Column)
			}
			labelNode := caseNode.Children[0]
			isDefault := labelNode.Type == "SYMBOL" && labelNode.Value == "default"
			ir.Cases = append(ir.Cases, irCase{Label: labelNode, Body: caseNode.Children[1], IsDefault: isDefault})
		}
		return ir, true
	case "sleep":
		if len(node.Children) != 2 {
			reportError("sleep expects (sleep ms)", node.Line, node.Column)
		}
		return &IRNode{Kind: "sleep", Kids: []*Node{node.Children[1]}}, true
	case "to_int":
		if len(node.Children) != 2 {
			reportError("to_int expects (to_int val)", node.Line, node.Column)
		}
		return &IRNode{Kind: "to_int", Kids: []*Node{node.Children[1]}}, true
	case "to_float":
		if len(node.Children) != 2 {
			reportError("to_float expects (to_float val)", node.Line, node.Column)
		}
		return &IRNode{Kind: "to_float", Kids: []*Node{node.Children[1]}}, true
	case "to_string":
		if len(node.Children) != 2 {
			reportError("to_string expects 1 argument", node.Line, node.Column)
		}
		return &IRNode{Kind: "to_string", Kids: []*Node{node.Children[1]}}, true
	case "bytes_to_string":
		if len(node.Children) != 2 {
			reportError("bytes_to_string expects 1 argument", node.Line, node.Column)
		}
		return &IRNode{Kind: "bytes_to_string", Kids: []*Node{node.Children[1]}}, true
	case "str_split":
		if len(node.Children) != 3 {
			reportError("str_split expects (str_split s sep)", node.Line, node.Column)
		}
		return &IRNode{Kind: "str_split", Kids: node.Children[1:]}, true
	case "str_join":
		if len(node.Children) != 3 {
			reportError("str_join expects (str_join list sep)", node.Line, node.Column)
		}
		return &IRNode{Kind: "str_join", Kids: node.Children[1:]}, true
	case "regex_match":
		if len(node.Children) != 3 {
			reportError("regex_match expects (regex_match pattern s)", node.Line, node.Column)
		}
		return &IRNode{Kind: "regex_match", Kids: node.Children[1:]}, true
	case "append":
		if len(node.Children) != 3 {
			reportError("append expects (append list item)", node.Line, node.Column)
		}
		return &IRNode{Kind: "append", Kids: node.Children[1:]}, true
	case "map_set":
		if len(node.Children) != 4 {
			reportError("map_set expects (map_set dict key val)", node.Line, node.Column)
		}
		return &IRNode{Kind: "map_set", Kids: node.Children[1:]}, true
	case "map_delete":
		if len(node.Children) != 3 {
			reportError("map_delete expects (map_delete dict key)", node.Line, node.Column)
		}
		return &IRNode{Kind: "map_delete", Kids: node.Children[1:]}, true
	case "map_get":
		if len(node.Children) != 3 {
			reportError("map_get expects (map_get dict key)", node.Line, node.Column)
		}
		return &IRNode{Kind: "map_get", Kids: node.Children[1:]}, true
	case "list_get":
		if len(node.Children) != 3 {
			reportError("list_get expects (list_get list idx)", node.Line, node.Column)
		}
		return &IRNode{Kind: "list_get", Kids: node.Children[1:]}, true
	case "list":
		return &IRNode{Kind: "list", Kids: node.Children[1:]}, true
	case "dict":
		return &IRNode{Kind: "dict", Kids: node.Children[1:]}, true
	case "print":
		return &IRNode{Kind: "print", Kids: node.Children[1:]}, true
	}
	return nil, false
}

func binOpGoToken(head string) string {
	switch head {
	case "and":
		return "&&"
	case "or":
		return "||"
	case "=":
		return "=="
	default:
		return head
	}
}

func binOpJSToken(head string) string {
	switch head {
	case "and":
		return "&&"
	case "or":
		return "||"
	case "=", "==":
		return "==="
	case "!=":
		return "!=="
	default:
		return head
	}
}

// emitGoIR renders an IRNode using the exact templates and wrapped-vs-raw
// child render choices the pre-refactor Go backend used per node kind.
func emitGoIR(ir *IRNode, reqVar string, depth int) string {
	switch ir.Kind {
	case "binop":
		if len(ir.Kids) != 2 {
			reportError(fmt.Sprintf("%s expects 2 arguments", ir.Op), 0, 0)
		}
		arg1 := generateExpression(ir.Kids[0], reqVar, depth+1)
		arg2 := generateExpression(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("(%s %s %s)", arg1, binOpGoToken(ir.Op), arg2)
	case "return":
		return fmt.Sprintf("		return %s", generateStatementRaw(ir.Kids[0], reqVar, depth+1))
	case "if":
		condExpr := generateExpression(ir.Kids[0], reqVar, depth+1)
		thenCode := generateStatement(ir.Kids[1], reqVar, depth+1)
		if len(ir.Kids) == 2 {
			return fmt.Sprintf(`		if %s {
%s
		}`, condExpr, thenCode)
		}
		elseCode := generateStatement(ir.Kids[2], reqVar, depth+1)
		return fmt.Sprintf(`		if %s {
%s
		} else {
%s
		}`, condExpr, thenCode, elseCode)
	case "while":
		condExpr := generateExpression(ir.Kids[0], reqVar, depth+1)
		bodyCode := generateStatement(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf(`		for %s {
%s
		}`, condExpr, bodyCode)
	case "do":
		var stmts string
		for _, kid := range ir.Kids {
			stmts += generateStatement(kid, reqVar, depth+1) + "\n"
		}
		return fmt.Sprintf("		{\n%s\n		}", stmts)
	case "set":
		varStr := generateStatement(ir.Kids[0], reqVar, depth+1)
		valStr := generateStatement(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("		%s = %s", varStr, valStr)
	case "match":
		varStr := generateStatement(ir.Kids[0], reqVar, depth+1)
		var casesStr string
		for _, c := range ir.Cases {
			caseValStr := c.Label.Value
			if c.IsDefault {
				caseValStr = "default"
			} else if c.Label.Type == "STRING" {
				caseValStr = fmt.Sprintf("%q", caseValStr)
			}
			caseBodyCode := generateStatement(c.Body, reqVar, depth+1)
			if caseValStr == "default" {
				casesStr += fmt.Sprintf("		default:\n%s\n", caseBodyCode)
			} else {
				casesStr += fmt.Sprintf("		case %s:\n%s\n", caseValStr, caseBodyCode)
			}
		}
		return fmt.Sprintf("		switch %s {\n%s		}", varStr, casesStr)
	case "sleep":
		msStr := generateStatement(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("		time.Sleep(time.Duration(%s) * time.Millisecond)", msStr)
	case "to_int":
		valStr := generateStatement(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("func() int { v, _ := strconv.Atoi(%s); return v }()", valStr)
	case "to_float":
		valStr := generateStatement(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("func() float64 { v, _ := strconv.ParseFloat(%s, 64); return v }()", valStr)
	case "to_string":
		valStr := generateStatement(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("fmt.Sprint(%s)", valStr)
	case "bytes_to_string":
		valStr := generateStatement(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("string(%s)", valStr)
	case "str_split":
		sStr := generateStatement(ir.Kids[0], reqVar, depth+1)
		sepStr := generateStatement(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("strings.Split(%s, %s)", sStr, sepStr)
	case "str_join":
		listStr := generateStatement(ir.Kids[0], reqVar, depth+1)
		sepStr := generateStatement(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("strings.Join(%s, %s)", listStr, sepStr)
	case "regex_match":
		patStr := generateStatement(ir.Kids[0], reqVar, depth+1)
		sStr := generateStatement(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("regexp.MatchString(%s, %s)", patStr, sStr)
	case "append":
		listNode := ir.Kids[0]
		if listNode.Type != "SYMBOL" {
			reportError("append requires a symbol for list", listNode.Line, listNode.Column)
		}
		itemStr := generateStatement(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("		%s = append(%s, %s)", listNode.Value, listNode.Value, itemStr)
	case "map_set":
		dictNode := ir.Kids[0]
		if dictNode.Type != "SYMBOL" {
			reportError("map_set requires a symbol for dict", dictNode.Line, dictNode.Column)
		}
		keyStr := generateStatement(ir.Kids[1], reqVar, depth+1)
		valStr := generateStatement(ir.Kids[2], reqVar, depth+1)
		return fmt.Sprintf("		%s[%s] = %s", dictNode.Value, keyStr, valStr)
	case "map_delete":
		dictNode := ir.Kids[0]
		if dictNode.Type != "SYMBOL" {
			reportError("map_delete requires a symbol for dict", dictNode.Line, dictNode.Column)
		}
		keyStr := generateStatement(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("		delete(%s, %s)", dictNode.Value, keyStr)
	case "map_get":
		dictNode := ir.Kids[0]
		if dictNode.Type != "SYMBOL" {
			reportError("map_get requires a symbol for dict", dictNode.Line, dictNode.Column)
		}
		keyStr := generateStatement(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("%s[%s]", dictNode.Value, keyStr)
	case "list_get":
		listNode := ir.Kids[0]
		if listNode.Type != "SYMBOL" {
			reportError("list_get requires a symbol for list", listNode.Line, listNode.Column)
		}
		idxStr := generateStatement(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("func() string { _i, _ := strconv.Atoi(fmt.Sprint(%s)); if _i >= 0 && _i < len(%s) { return %s[_i] }; return \"\" }()", idxStr, listNode.Value, listNode.Value)
	case "list":
		var items []string
		for _, kid := range ir.Kids {
			if kid.Type == "STRING" {
				items = append(items, fmt.Sprintf("%q", kid.Value))
			} else {
				items = append(items, generateExpression(kid, reqVar, depth+1))
			}
		}
		return fmt.Sprintf("[]string{%s}", strings.Join(items, ", "))
	case "dict":
		var pairs []string
		for _, kid := range ir.Kids {
			if kid.Type != "List" || len(kid.Children) != 2 {
				reportError("dict expects (k v) pairs", kid.Line, kid.Column)
			}
			k := kid.Children[0].Value
			if kid.Children[0].Type == "STRING" {
				k = fmt.Sprintf("%q", k)
			}
			v := kid.Children[1].Value
			if kid.Children[1].Type == "STRING" {
				v = fmt.Sprintf("%q", v)
			} else {
				v = generateExpression(kid.Children[1], reqVar, depth+1)
			}
			pairs = append(pairs, fmt.Sprintf("%s: %s", k, v))
		}
		return fmt.Sprintf("map[string]string{%s}", strings.Join(pairs, ", "))
	case "print":
		var args []string
		for _, kid := range ir.Kids {
			args = append(args, generateStatement(kid, reqVar, depth+1))
		}
		return fmt.Sprintf("		fmt.Println(%s)", strings.Join(args, ", "))
	}
	reportError(fmt.Sprintf("Unknown IR kind: %s", ir.Kind), 0, 0)
	return ""
}

// emitJSIR renders an IRNode using the exact templates and wrapped-vs-raw
// child render choices the pre-refactor JS backend used per node kind.
func emitJSIR(ir *IRNode, reqVar string, depth int) string {
	switch ir.Kind {
	case "binop":
		arg1 := generateJSExpression(ir.Kids[0], reqVar, depth+1)
		arg2 := generateJSExpression(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("(%s %s %s)", arg1, binOpJSToken(ir.Op), arg2)
	case "return":
		return fmt.Sprintf("return %s;", generateJSStatementRaw(ir.Kids[0], reqVar, depth+1))
	case "if":
		condExpr := generateJSExpression(ir.Kids[0], reqVar, depth+1)
		thenCode := generateJSStatement(ir.Kids[1], reqVar, depth+1)
		if len(ir.Kids) == 2 {
			return fmt.Sprintf("if (%s) {\n%s\n}", condExpr, thenCode)
		}
		elseCode := generateJSStatement(ir.Kids[2], reqVar, depth+1)
		return fmt.Sprintf("if (%s) {\n%s\n} else {\n%s\n}", condExpr, thenCode, elseCode)
	case "while":
		condExpr := generateJSExpression(ir.Kids[0], reqVar, depth+1)
		bodyCode := generateJSStatement(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("while (%s) {\n%s\n}", condExpr, bodyCode)
	case "do":
		var stmts string
		for _, kid := range ir.Kids {
			stmts += generateJSStatement(kid, reqVar, depth+1) + ";\n"
		}
		return fmt.Sprintf("{\n%s}", stmts)
	case "set":
		varStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		valStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("%s = %s", varStr, valStr)
	case "match":
		varStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		var casesStr string
		for _, c := range ir.Cases {
			caseValStr := c.Label.Value
			if c.Label.Type == "STRING" {
				caseValStr = fmt.Sprintf("%q", caseValStr)
			}
			caseBodyCode := generateJSStatement(c.Body, reqVar, depth+1)
			if c.IsDefault {
				casesStr += fmt.Sprintf("default:\n%s;\nbreak;\n", caseBodyCode)
			} else {
				casesStr += fmt.Sprintf("case %s:\n%s;\nbreak;\n", caseValStr, caseBodyCode)
			}
		}
		return fmt.Sprintf("switch (%s) {\n%s}", varStr, casesStr)
	case "sleep":
		msStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("(await new Promise(r => setTimeout(r, %s)))", msStr)
	case "to_int":
		valStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("parseInt(%s, 10)", valStr)
	case "to_float":
		valStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("parseFloat(%s)", valStr)
	case "to_string", "bytes_to_string":
		valStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("String(%s)", valStr)
	case "str_split":
		sStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		sepStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("(%s).split(%s)", sStr, sepStr)
	case "str_join":
		listStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		sepStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("(%s).join(%s)", listStr, sepStr)
	case "regex_match":
		patStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		sStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("new RegExp(%s).test(%s)", patStr, sStr)
	case "append":
		listNode := ir.Kids[0]
		itemStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("%s.push(%s)", listNode.Value, itemStr)
	case "map_set":
		dictNode := ir.Kids[0]
		keyStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		valStr := generateJSStatementRaw(ir.Kids[2], reqVar, depth+1)
		return fmt.Sprintf("%s[%s] = %s", dictNode.Value, keyStr, valStr)
	case "map_delete":
		dictNode := ir.Kids[0]
		keyStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("delete %s[%s]", dictNode.Value, keyStr)
	case "map_get":
		dictNode := ir.Kids[0]
		keyStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("(%s[%s] ?? \"\")", dictNode.Value, keyStr)
	case "list_get":
		listNode := ir.Kids[0]
		idxStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("(%s[%s] ?? \"\")", listNode.Value, idxStr)
	case "list":
		var items []string
		for _, kid := range ir.Kids {
			if kid.Type == "STRING" {
				items = append(items, fmt.Sprintf("%q", kid.Value))
			} else {
				items = append(items, generateJSExpression(kid, reqVar, depth+1))
			}
		}
		return fmt.Sprintf("[%s]", strings.Join(items, ", "))
	case "dict":
		var pairs []string
		for _, kid := range ir.Kids {
			if kid.Type != "List" || len(kid.Children) != 2 {
				reportError("dict expects (k v) pairs", kid.Line, kid.Column)
			}
			k := kid.Children[0].Value
			if kid.Children[0].Type == "STRING" {
				k = fmt.Sprintf("%q", k)
			}
			v := kid.Children[1].Value
			if kid.Children[1].Type == "STRING" {
				v = fmt.Sprintf("%q", v)
			} else {
				v = generateJSExpression(kid.Children[1], reqVar, depth+1)
			}
			pairs = append(pairs, fmt.Sprintf("%s: %s", k, v))
		}
		return fmt.Sprintf("{%s}", strings.Join(pairs, ", "))
	case "print":
		var args []string
		for _, kid := range ir.Kids {
			args = append(args, generateJSStatementRaw(kid, reqVar, depth+1))
		}
		return fmt.Sprintf("console.log(%s)", strings.Join(args, ", "))
	}
	reportError(fmt.Sprintf("Unknown IR kind for JS: %s", ir.Kind), 0, 0)
	return ""
}

func generateStatementRaw(node *Node, reqVar string, depth int) string {
	if depth > 1000 {
		reportError("AST too deep: exceeded maximum nesting limit of 1000", node.Line, node.Column)
	}
	if node.Type == "STRING" {
		return fmt.Sprintf("%q", node.Value)
	}
	if node.Type == "SYMBOL" || node.Type == "INT" {
		if node.Value == "req.method" {
			return reqVar + ".Method"
		}
		return node.Value
	}
	if node.Type != "List" || len(node.Children) == 0 {
		reportError("Expected list for statement", node.Line, node.Column)
	}
	head := node.Children[0].Value
	if head == "intent" {
		return ""
	}
	if ir, ok := lowerShared(node); ok {
		return emitGoIR(ir, reqVar, depth)
	}
	if head == "res_json" {
		if len(node.Children) != 3 {
			reportError("res_json expects (res_json status data)", node.Line, node.Column)
		}
		status := node.Children[1].Value
		dataNode := node.Children[2]
		dataVar := dataNode.Value
		if dataNode.Type == "STRING" {
			dataVar = fmt.Sprintf("%q", dataVar)
		}
		return fmt.Sprintf(`		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(%s)
		_ = json.NewEncoder(w).Encode(%s)`, status, dataVar)
	} else if head == "res" {
		if len(node.Children) != 4 {
			reportError("res expects status, contentType, and body", node.Line, node.Column)
		}
		status := node.Children[1].Value
		contentType := node.Children[2].Value
		resBody := node.Children[3].Value
		if node.Children[3].Type == "SYMBOL" || (node.Children[3].Type == "List" && node.Children[3].Children[0].Value == "call") {
			var bodyStr string
			if node.Children[3].Type == "List" {
				funcName := node.Children[3].Children[1].Value
				var args []string
				for j := 2; j < len(node.Children[3].Children); j++ {
					if node.Children[3].Children[j].Type == "STRING" {
						args = append(args, fmt.Sprintf("%q", node.Children[3].Children[j].Value))
					} else {
						args = append(args, node.Children[3].Children[j].Value)
					}
				}
				bodyStr = fmt.Sprintf("%s(%s)", funcName, strings.Join(args, ", "))
			} else {
				bodyStr = resBody
			}
			return fmt.Sprintf(`		w.Header().Set("Content-Type", %q)
		w.WriteHeader(%s)
		fmt.Fprint(w, %s)`, contentType, status, bodyStr)
		} else {
			return fmt.Sprintf(`		w.Header().Set("Content-Type", %q)
		w.WriteHeader(%s)
		fmt.Fprint(w, %q)`, contentType, status, resBody)
		}
	} else if head == "let" {
		if len(node.Children) != 3 {
			reportError("let expects (let (var val) body) — wrap multiple body statements in (do ...)", node.Line, node.Column)
		}
		var letPrefix strings.Builder
		letPrefix.WriteString("		{\n")
		declaredVars := make(map[string]bool)

		curr := node
		for curr.Type == "List" && len(curr.Children) == 3 && curr.Children[0].Value == "let" {
			binds := curr.Children[1]
			if binds.Type != "List" || len(binds.Children) != 2 {
				reportError("let binding expects (var val)", binds.Line, binds.Column)
			}
			varName := binds.Children[0].Value
			valNode := binds.Children[1]
			var valStr string
			if valNode.Type == "STRING" {
				valStr = fmt.Sprintf("%q", valNode.Value)
			} else if valNode.Type == "List" && len(valNode.Children) > 0 {
				funcName := valNode.Children[0].Value
				if funcName == "call" {
					var args []string
					for j := 2; j < len(valNode.Children); j++ {
						if valNode.Children[j].Type == "STRING" {
							args = append(args, fmt.Sprintf("%q", valNode.Children[j].Value))
						} else {
							args = append(args, valNode.Children[j].Value)
						}
					}
					valStr = fmt.Sprintf("%s(%s)", valNode.Children[1].Value, strings.Join(args, ", "))
				} else if funcName == "list" {
					var items []string
					for j := 1; j < len(valNode.Children); j++ {
						if valNode.Children[j].Type == "STRING" {
							items = append(items, fmt.Sprintf("%q", valNode.Children[j].Value))
						} else {
							items = append(items, generateExpression(valNode.Children[j], reqVar, depth+1))
						}
					}
					valStr = fmt.Sprintf("[]string{%s}", strings.Join(items, ", "))
				} else if funcName == "dict" {
					var pairs []string
					for j := 1; j < len(valNode.Children); j++ {
						pair := valNode.Children[j]
						if pair.Type == "List" && len(pair.Children) == 2 {
							k := pair.Children[0].Value
							if pair.Children[0].Type == "STRING" {
								k = fmt.Sprintf("%q", k)
							}
							v := pair.Children[1].Value
							if pair.Children[1].Type == "STRING" {
								v = fmt.Sprintf("%q", v)
							} else {
								v = generateExpression(pair.Children[1], reqVar, depth+1)
							}
							pairs = append(pairs, fmt.Sprintf("%s: %s", k, v))
						}
					}
					valStr = fmt.Sprintf("map[string]string{%s}", strings.Join(pairs, ", "))
				} else if funcName == "env" {
					if len(valNode.Children) != 2 {
						reportError("env expects (env \"KEY\")", valNode.Line, valNode.Column)
					}
					keyNode := valNode.Children[1]
					if keyNode.Type == "STRING" {
						valStr = fmt.Sprintf("os.Getenv(%q)", keyNode.Value)
					}
				} else if funcName == "parse_json" {
					if len(valNode.Children) != 3 {
						reportError("parse_json expects (parse_json Type body)", valNode.Line, valNode.Column)
					}
					// Handled downstream
				} else {
					valStr = generateStatement(valNode, reqVar, depth+1)
				}
			} else {
				valStr = generateStatement(valNode, reqVar, depth+1)
			}

			if valNode.Type == "List" && len(valNode.Children) > 0 && valNode.Children[0].Value == "parse_json" {
				targetType := valNode.Children[1].Value
				bodyVar := valNode.Children[2].Value
				if bodyVar == "req.body" {
					bodyVar = reqVar + ".Body"
				}
				if bodyVar == reqVar+".Body" {
					letPrefix.WriteString(fmt.Sprintf("			var %s %s\n			_ = json.NewDecoder(%s).Decode(&%s)\n			_ = %s\n", varName, targetType, bodyVar, varName, varName))
				} else {
					letPrefix.WriteString(fmt.Sprintf("			var %s %s\n			_ = json.Unmarshal([]byte(%s), &%s)\n			_ = %s\n", varName, targetType, bodyVar, varName, varName))
				}
				declaredVars[varName] = true
			} else {
				if declaredVars[varName] {
					letPrefix.WriteString(fmt.Sprintf("			%s = %s\n			_ = %s\n", varName, valStr, varName))
				} else {
					letPrefix.WriteString(fmt.Sprintf("			%s := %s\n			_ = %s\n", varName, valStr, varName))
					declaredVars[varName] = true
				}
			}

			curr = curr.Children[2]
		}

		bodyCode := generateStatement(curr, reqVar, depth+1)
		return fmt.Sprintf("%s%s\n		}", letPrefix.String(), bodyCode)
	} else if head == "try_let" {
		if len(node.Children) != 4 {
			reportError("try_let expects (try_let (var val) (catch err catchBody) successBody)", node.Line, node.Column)
		}
		binds := node.Children[1]
		if binds.Type != "List" || len(binds.Children) != 2 {
			reportError("try_let binding expects (var val)", binds.Line, binds.Column)
		}
		varName := binds.Children[0].Value
		valNode := binds.Children[1]

		catchNode := node.Children[2]
		if catchNode.Type != "List" || len(catchNode.Children) != 3 || catchNode.Children[0].Value != "catch" {
			reportError("try_let catch expects (catch errVar catchBody)", catchNode.Line, catchNode.Column)
		}
		errVar := catchNode.Children[1].Value
		catchBodyCode := generateStatement(catchNode.Children[2], reqVar, depth+1)
		successBodyCode := generateStatement(node.Children[3], reqVar, depth+1)

		if valNode.Type == "List" && len(valNode.Children) > 0 && valNode.Children[0].Value == "parse_json" {
			targetType := valNode.Children[1].Value
			bodyVar := valNode.Children[2].Value
			if bodyVar == "req.body" {
				bodyVar = reqVar + ".Body"
				return fmt.Sprintf(`		{
			var %s %s
			if %s := json.NewDecoder(%s).Decode(&%s); %s != nil {
%s
			} else {
				_ = %s
%s
			}
		}`, varName, targetType, errVar, bodyVar, varName, errVar, catchBodyCode, varName, successBodyCode)
			} else {
				return fmt.Sprintf(`		{
			var %s %s
			if %s := json.Unmarshal([]byte(%s), &%s); %s != nil {
%s
			} else {
				_ = %s
%s
			}
		}`, varName, targetType, errVar, bodyVar, varName, errVar, catchBodyCode, varName, successBodyCode)
			}
		}

		valStr := generateStatement(valNode, reqVar, depth+1)
		return fmt.Sprintf(`		{
			%s, %s := %s
			if %s != nil {
%s
			} else {
				_ = %s
%s
			}
		}`, varName, errVar, valStr, errVar, catchBodyCode, varName, successBodyCode)
	} else if head == "spawn" {
		if len(node.Children) != 2 {
			reportError("spawn expects (spawn (lambda () body))", node.Line, node.Column)
		}
		lambdaNode := node.Children[1]
		if lambdaNode.Type != "List" || len(lambdaNode.Children) != 3 || lambdaNode.Children[0].Value != "lambda" {
			reportError("spawn expects a lambda", lambdaNode.Line, lambdaNode.Column)
		}
		argsNode := lambdaNode.Children[1]
		if argsNode.Type != "List" || len(argsNode.Children) != 0 {
			reportError("spawn lambda expects no arguments ()", argsNode.Line, argsNode.Column)
		}
		bodyCode := generateStatement(lambdaNode.Children[2], reqVar, depth+1)
		traceInject := fmt.Sprintf("\t\tdefer observer.Trace(%q, map[string]any{})()\n", "spawn_lambda")
		return fmt.Sprintf("		go func() {\n%s%s\n		}()", traceInject, bodyCode)
	} else if head == "trace" {
		if len(node.Children) != 2 {
			reportError("trace expects (trace var)", node.Line, node.Column)
		}
		varStr := generateStatement(node.Children[1], reqVar, depth+1)
		fileLine := fmt.Sprintf("[%s:%d]", node.Filename, node.Line)
		varName := node.Children[1].Value
		if node.Children[1].Type == "List" {
			varName = "expression"
		}
		return fmt.Sprintf("		fmt.Println(%q, %q, %s)", fileLine, varName+" =", varStr)
	} else if head == "db_connect" {
		if len(node.Children) != 4 {
			reportError("db_connect expects (db_connect var driver dsn)", node.Line, node.Column)
		}
		varName := node.Children[1].Value
		driverNode := node.Children[2]
		dsnNode := node.Children[3]
		code := fmt.Sprintf("		%s, _ := sql.Open(%q, %q)\n		_ = %s", varName, driverNode.Value, dsnNode.Value, varName)
		for _, ddl := range currentSchemaDDLs {
			code += fmt.Sprintf("\n		%s.Exec(%q)", varName, ddl)
		}
		return code
	} else if head == "sql_query" {
		if len(node.Children) != 3 {
			reportError("sql_query expects (sql_query db query)", node.Line, node.Column)
		}
		dbVar := node.Children[1].Value
		queryNode := node.Children[2]
		queryStr := queryNode.Value
		if queryNode.Type == "STRING" {
			queryStr = fmt.Sprintf("%q", queryStr)
		}
		return fmt.Sprintf("		%s.Query(%s)", dbVar, queryStr)
	} else if head == "for" {
		if len(node.Children) != 4 {
			reportError("for expects (for item list body)", node.Line, node.Column)
		}
		itemNode := node.Children[1].Value
		listNode := node.Children[2].Value
		bodyCode := generateStatement(node.Children[3], reqVar, depth+1)
		return fmt.Sprintf(`		for _, %s := range %s {
			_ = %s
%s
		}`, itemNode, listNode, itemNode, bodyCode)
	} else if head == "read_file" {
		if len(node.Children) != 2 {
			reportError("read_file expects (read_file path)", node.Line, node.Column)
		}
		pathStr := generateStatement(node.Children[1], reqVar, depth+1)
		return fmt.Sprintf("os.ReadFile(%s)", pathStr)
	} else if head == "write_file" {
		if len(node.Children) != 3 {
			reportError("write_file expects (write_file path data)", node.Line, node.Column)
		}
		pathStr := generateStatement(node.Children[1], reqVar, depth+1)
		dataStr := generateStatement(node.Children[2], reqVar, depth+1)
		return fmt.Sprintf("		os.WriteFile(%s, []byte(%s), 0644)", pathStr, dataStr)
	} else if head == "mkdir" {
		if len(node.Children) != 2 {
			reportError("mkdir expects (mkdir path)", node.Line, node.Column)
		}
		pathStr := generateStatement(node.Children[1], reqVar, depth+1)
		return fmt.Sprintf("		os.MkdirAll(%s, 0755)", pathStr)
	} else if head == "exec" {
		if len(node.Children) < 2 {
			reportError("exec expects (exec cmd args...)", node.Line, node.Column)
		}
		cmdStr := generateStatement(node.Children[1], reqVar, depth+1)
		var args []string
		for j := 2; j < len(node.Children); j++ {
			args = append(args, generateStatement(node.Children[j], reqVar, depth+1))
		}
		return fmt.Sprintf("func() ([]byte, error) { return exec.Command(%s, %s).CombinedOutput() }()", cmdStr, strings.Join(args, ", "))
	} else if head == "rate_limit" {
		if len(node.Children) != 3 {
			reportError("rate_limit expects (rate_limit \"10/s\" body)", node.Line, node.Column)
		}
		rateStr := node.Children[1].Value
		bodyCode := generateStatement(node.Children[2], reqVar, depth+1)
		// simple implementation: "10/s" -> sleep 100ms
		ms := 1000
		if strings.HasSuffix(rateStr, "/s") {
			n, _ := strconv.Atoi(strings.TrimSuffix(rateStr, "/s"))
			if n > 0 {
				ms = 1000 / n
			}
		}
		return fmt.Sprintf(`		{
			time.Sleep(%d * time.Millisecond)
			%s
		}`, ms, bodyCode)
	} else if head == "retry" {
		if len(node.Children) != 3 {
			reportError("retry expects (retry times body)", node.Line, node.Column)
		}
		timesStr := generateStatement(node.Children[1], reqVar, depth+1)
		bodyCode := generateStatement(node.Children[2], reqVar, depth+1)
		return fmt.Sprintf(`		for i := 0; i < %s; i++ {
			%s
		}`, timesStr, bodyCode)
	} else if head == "fetch" {
		if len(node.Children) != 3 {
			reportError("fetch expects (fetch url method)", node.Line, node.Column)
		}
		urlStr := generateStatement(node.Children[1], reqVar, depth+1)
		methodStr := generateStatement(node.Children[2], reqVar, depth+1)

		return fmt.Sprintf(`func() ([]byte, error) {
			req, err := http.NewRequest(%s, %s, nil)
			if err != nil { return nil, err }
			resp, err := http.DefaultClient.Do(req)
			if err != nil { return nil, err }
			defer resp.Body.Close()
			return io.ReadAll(resp.Body)
		}()`, methodStr, urlStr)
	} else if head == "confidence" {
		if len(node.Children) != 2 {
			reportError("confidence expects (confidence prompt)", node.Line, node.Column)
		}
		promptStr := generateStatement(node.Children[1], reqVar, depth+1)

		return fmt.Sprintf(`func() float64 {
			reqBody, _ := json.Marshal(map[string]any{
				"model":  "llama3",
				"prompt": fmt.Sprintf("Evaluate the probability of this statement being true. Return ONLY a float between 0.0 and 1.0. Statement: %%v", %s),
				"stream": false,
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil { panic(err) }
			defer resp.Body.Close()
			var res struct {
				Response string `+"`json:\"response\"`"+`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { panic(err) }
			val, _ := strconv.ParseFloat(strings.TrimSpace(res.Response), 64)
			return val
		}()`, promptStr)
	} else if head == "llm_generate" {
		if len(node.Children) < 2 {
			reportError("llm_generate expects (llm_generate prompt [model])", node.Line, node.Column)
		}
		promptStr := generateStatement(node.Children[1], reqVar, depth+1)
		modelStr := `"llama3"`
		if len(node.Children) >= 3 {
			modelStr = generateStatement(node.Children[2], reqVar, depth+1)
		}

		return fmt.Sprintf(`func() (string, error) {
			reqBody, _ := json.Marshal(map[string]any{
				"model":  %s,
				"prompt": %s,
				"stream": false,
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil { return "", err }
			defer resp.Body.Close()
			var res struct {
				Response string `+"`json:\"response\"`"+`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { return "", err }
			return res.Response, nil
		}()`, modelStr, promptStr)
	} else if head == "fuzzy_cast" {
		if len(node.Children) < 3 {
			reportError("fuzzy_cast expects (fuzzy_cast Type var [model])", node.Line, node.Column)
		}
		typeStr := node.Children[1].Value
		varStr := generateStatement(node.Children[2], reqVar, depth+1)
		modelStr := `"llama3"`
		if len(node.Children) >= 4 {
			modelStr = generateStatement(node.Children[3], reqVar, depth+1)
		}

		return fmt.Sprintf(`func() (%s, error) {
			var out %s
			reqBody, _ := json.Marshal(map[string]any{
				"model":  %s,
				"prompt": fmt.Sprintf("Coerce this input into a valid JSON object matching the requested schema. Reply strictly with the JSON object and nothing else.\nInput: %%s", %s),
				"stream": false,
				"format": "json",
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil { return out, err }
			defer resp.Body.Close()
			var res struct {
				Response string `+"`json:\"response\"`"+`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { return out, err }
			err = json.Unmarshal([]byte(res.Response), &out)
			return out, err
		}()`, typeStr, typeStr, modelStr, varStr)
	} else if head == "semantic_match" {
		if len(node.Children) < 3 {
			reportError("semantic_match expects (semantic_match input (intent consequence)...)", node.Line, node.Column)
		}
		varStr := generateStatement(node.Children[1], reqVar, depth+1)
		var casesStr string
		var intentExprs []string

		for i := 2; i < len(node.Children); i++ {
			caseNode := node.Children[i]
			if caseNode.Type != "LIST" || len(caseNode.Children) < 2 {
				reportError("semantic_match case expects (intent consequence...)", caseNode.Line, caseNode.Column)
			}
			label := caseNode.Children[0]
			isDefault := label.Value == "default" || label.Value == "_"

			bodyNode := &Node{Type: "LIST", Children: append([]*Node{{Type: "SYMBOL", Value: "do"}}, caseNode.Children[1:]...)}
			bodyCode := generateStatement(bodyNode, reqVar, depth+1)

			if isDefault {
				casesStr += fmt.Sprintf("\t\t\tdefault:\n%s\n", bodyCode)
			} else {
				intentIdx := len(intentExprs)
				intentExpr := generateExpression(label, reqVar, depth+1)
				intentExprs = append(intentExprs, intentExpr)
				casesStr += fmt.Sprintf("\t\t\tcase %d:\n%s\n", intentIdx, bodyCode)
			}
		}

		intentsArrayStr := "[]string{" + strings.Join(intentExprs, ", ") + "}"

		return fmt.Sprintf(`func() {
			intents := %s
			intentsStr := ""
			for i, intent := range intents {
				intentsStr += fmt.Sprintf("%%d: %%s\n", i, intent)
			}
			reqBody, _ := json.Marshal(map[string]any{
				"model":  "llama3",
				"prompt": fmt.Sprintf("Determine which of the following intents best matches the given input string. Return ONLY the integer ID of the matching intent. If none match, return 'default'.\n\nIntents:\n%%s\nInput: %%s", intentsStr, %s),
				"stream": false,
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil { panic(err) }
			defer resp.Body.Close()
			var res struct {
				Response string `+"`json:\"response\"`"+`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { panic(err) }

			ans := strings.TrimSpace(strings.ToLower(res.Response))
			idx, err := strconv.Atoi(ans)
			if err != nil {
				idx = -1
			}
			switch idx {
%s			}
		}()`, intentsArrayStr, varStr, casesStr)
	} else if head == "assert_semantic" {
		if len(node.Children) != 3 {
			reportError("assert_semantic expects (assert_semantic var \"condition\")", node.Line, node.Column)
		}
		varStr := generateStatement(node.Children[1], reqVar, depth+1)
		condStr := generateStatement(node.Children[2], reqVar, depth+1)

		return fmt.Sprintf(`func() bool {
			reqBody, _ := json.Marshal(map[string]any{
				"model":  "llama3",
				"prompt": fmt.Sprintf("Does this input satisfy the condition: '%%s'? Reply strictly with 'true' or 'false' and nothing else.\nInput: %%s", %s, %s),
				"stream": false,
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil { return false }
			defer resp.Body.Close()
			var res struct {
				Response string `+"`json:\"response\"`"+`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { return false }
			return strings.TrimSpace(strings.ToLower(res.Response)) == "true"
		}()`, condStr, varStr)
	} else if head == "call" {
		if len(node.Children) < 2 {
			reportError("call expects (call func args...)", node.Line, node.Column)
		}
		funcName := node.Children[1].Value
		var args []string
		for j := 2; j < len(node.Children); j++ {
			argNode := node.Children[j]
			if argNode.Type == "STRING" {
				args = append(args, fmt.Sprintf("%q", argNode.Value))
			} else if argNode.Type == "NUMBER" || argNode.Type == "SYMBOL" {
				args = append(args, argNode.Value)
			} else {
				args = append(args, generateExpression(argNode, reqVar, depth+1))
			}
		}
		return fmt.Sprintf("		%s(%s)", funcName, strings.Join(args, ", "))
	} else if head == "cli_args" {
		if len(node.Children) == 1 {
			return "os.Args[1:]"
		} else if len(node.Children) == 2 {
			idxStr := generateStatement(node.Children[1], reqVar, depth+1)
			return fmt.Sprintf("func() string { _idx, _ := strconv.Atoi(fmt.Sprint(%s)); if len(os.Args) > _idx+1 { return os.Args[_idx+1] }; return \"\" }()", idxStr)
		} else {
			reportError("cli_args expects (cli_args) or (cli_args index)", node.Line, node.Column)
		}
	}
	reportError(fmt.Sprintf("Unknown statement: %s", head), node.Line, node.Column)
	return ""
}

func expandIncludes(node *Node, baseDir string, depth int) {
	if depth > 100 {
		reportError("Include depth exceeded (circular include?)", node.Line, node.Column)
	}
	if node.Type != "List" {
		return
	}
	var newChildren []*Node
	for i := 0; i < len(node.Children); i++ {
		child := node.Children[i]
		if child.Type == "List" && len(child.Children) == 2 && child.Children[0].Type == "SYMBOL" && child.Children[0].Value == "include" {
			filenameNode := child.Children[1]
			if filenameNode.Type != "STRING" {
				reportError("include expects a string filename", child.Line, child.Column)
			}
			filename := filenameNode.Value
			fullPath := filepath.Join(baseDir, filename)

			content, err := os.ReadFile(fullPath)
			if err != nil {
				reportError(fmt.Sprintf("Failed to read included file %q: %v", filename, err), child.Line, child.Column)
			}

			lexer := NewLexer(string(content))
			parser := NewParser(lexer, filepath.Base(fullPath))
			includedAst := parser.parseExpression()

			if parser.cur.Type != TokenEOF {
				reportError(fmt.Sprintf("Unexpected tokens after EOF in included file %q", filename), parser.cur.Line, parser.cur.Column)
			}

			expandIncludes(includedAst, filepath.Dir(fullPath), depth+1)

			if includedAst.Type == "List" && len(includedAst.Children) > 0 && includedAst.Children[0].Value == "module" {
				newChildren = append(newChildren, includedAst.Children[1:]...)
			} else {
				newChildren = append(newChildren, includedAst)
			}
		} else {
			expandIncludes(child, baseDir, depth)
			newChildren = append(newChildren, child)
		}
	}
	node.Children = newChildren
}

func applyPatches(node *Node) {
	if node == nil || node.Type != "List" {
		return
	}
	var newChildren []*Node
	for i := 0; i < len(node.Children); i++ {
		child := node.Children[i]
		if child.Type == "List" && len(child.Children) == 3 && child.Children[0].Type == "SYMBOL" && child.Children[0].Value == "patch" {
			funcNameNode := child.Children[1]
			if funcNameNode.Type != "SYMBOL" {
				reportError("patch expects a symbol for function name", funcNameNode.Line, funcNameNode.Column)
			}
			funcName := funcNameNode.Value
			newBody := child.Children[2]

			found := false
			for j := 0; j < len(newChildren); j++ {
				target := newChildren[j]
				if target.Type == "List" && len(target.Children) >= 4 && target.Children[0].Type == "SYMBOL" && target.Children[0].Value == "defun" && target.Children[1].Value == funcName {
					target.Children[len(target.Children)-1] = copyNode(newBody)
					found = true
					break
				}
			}
			if !found {
				for j := i + 1; j < len(node.Children); j++ {
					target := node.Children[j]
					if target.Type == "List" && len(target.Children) >= 4 && target.Children[0].Type == "SYMBOL" && target.Children[0].Value == "defun" && target.Children[1].Value == funcName {
						target.Children[len(target.Children)-1] = copyNode(newBody)
						found = true
						break
					}
				}
			}
			if !found {
				reportError(fmt.Sprintf("patch target function %q not found", funcName), child.Line, child.Column)
			}
		} else {
			applyPatches(child)
			newChildren = append(newChildren, child)
		}
	}
	node.Children = newChildren
}

func applyWithContext(node *Node, ctxVars []*Node) *Node {
	if node == nil {
		return nil
	}
	if node.Type != "List" || len(node.Children) == 0 {
		return node
	}

	head := node.Children[0].Value
	if head == "with_context" {
		if len(node.Children) < 3 {
			reportError("with_context expects (with_context vars body...)", node.Line, node.Column)
		}
		varsNode := node.Children[1]
		var newCtxVars []*Node
		newCtxVars = append(newCtxVars, ctxVars...)
		if varsNode.Type == "SYMBOL" {
			newCtxVars = append(newCtxVars, varsNode)
		} else if varsNode.Type == "List" {
			for _, v := range varsNode.Children {
				if v.Type != "SYMBOL" {
					reportError("with_context vars must be symbols", v.Line, v.Column)
				}
				newCtxVars = append(newCtxVars, v)
			}
		} else {
			reportError("with_context expects symbol or list of symbols", varsNode.Line, varsNode.Column)
		}

		doNode := &Node{Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename}
		doNode.Children = append(doNode.Children, &Node{Type: "SYMBOL", Value: "do", Line: node.Line, Column: node.Column})
		for i := 2; i < len(node.Children); i++ {
			doNode.Children = append(doNode.Children, applyWithContext(node.Children[i], newCtxVars))
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
			newNode.Children = append(newNode.Children, copyNode(cv))
		}
		for i := 2; i < len(node.Children); i++ {
			newNode.Children = append(newNode.Children, applyWithContext(node.Children[i], ctxVars))
		}
		return newNode
	}

	newNode := &Node{Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename}
	newNode.Children = append(newNode.Children, node.Children[0])
	for i := 1; i < len(node.Children); i++ {
		newNode.Children = append(newNode.Children, applyWithContext(node.Children[i], ctxVars))
	}
	return newNode
}

// JS Code Generator
func generateJSCode(node *Node) (string, string) {
	if node.Type != "List" || len(node.Children) == 0 {
		reportError("Expected list at root", node.Line, node.Column)
	}
	head := node.Children[0]
	if head.Type != "SYMBOL" || head.Value != "web_app" {
		reportError("Expected web_app as root symbol", head.Line, head.Column)
	}

	var funcsCode string
	var appCode string
	var testCode string

	for i := 1; i < len(node.Children); i++ {
		handlerNode := node.Children[i]
		if handlerNode.Type != "List" || len(handlerNode.Children) == 0 {
			appCode += generateJSStatement(handlerNode, "", 0) + "\n"
			continue
		}

		headVal := handlerNode.Children[0].Value

		if headVal == "intent" {
			continue
		}

		if headVal == "test" {
			if len(handlerNode.Children) < 3 {
				reportError(`test expects (test "description" body...)`, handlerNode.Line, handlerNode.Column)
			}
			descNode := handlerNode.Children[1]
			if descNode.Type != "STRING" {
				reportError("test description must be a string", descNode.Line, descNode.Column)
			}
			desc := descNode.Value
			var testBodyCode string
			for j := 2; j < len(handlerNode.Children); j++ {
				testBodyCode += generateJSStatement(handlerNode.Children[j], "", 0) + "\n"
			}
			testCode += fmt.Sprintf("test(%q, async (t) => {\n%s\n});\n\n", desc, testBodyCode)
			continue
		}

		if headVal == "defun" {
			if len(handlerNode.Children) < 4 {
				reportError("defun expects (defun name (args) body)", handlerNode.Line, handlerNode.Column)
			}
			name := handlerNode.Children[1].Value
			argsNode := handlerNode.Children[2]

			var argsList []string
			for _, arg := range argsNode.Children {
				argsList = append(argsList, arg.Value)
			}
			argsStr := strings.Join(argsList, ", ")

			bodyNode := handlerNode.Children[len(handlerNode.Children)-1]
			bodyCode := generateJSStatement(bodyNode, "", 0)
			funcsCode += fmt.Sprintf("async function %s(%s) {\n%s\n}\n\n", name, argsStr, bodyCode)
			continue
		}

		appCode += generateJSStatement(handlerNode, "", 0) + "\n"
	}

	code := funcsCode + appCode

	if testCode != "" {
		testCode = "const test = require('node:test');\n" +
			"const assert = require('node:assert');\n\n" +
			funcsCode + testCode
	}

	return code, testCode
}

// generateWasmCode emits the deliberately small, portable Wasm prototype.
// The shared IR already captures the shape of simple expressions and control
// flow; this backend supports only the i32 subset that maps directly to core
// WebAssembly instructions. Runtime-dependent Zero primitives belong in a
// future backend phase rather than being silently miscompiled.
func generateWasmCode(node *Node) string {
	if node.Type != "List" || len(node.Children) != 2 {
		reportError("wasm_app expects exactly one result expression", node.Line, node.Column)
	}
	head := node.Children[0]
	if head.Type != "SYMBOL" || head.Value != "wasm_app" {
		reportError("Expected wasm_app as root symbol", head.Line, head.Column)
	}

	return fmt.Sprintf("(module\n  (func (export \"main\") (result i32)\n    %s\n  )\n)\n", generateWasmExpression(node.Children[1]))
}

func generateWasmExpression(node *Node) string {
	if node.Type == "INT" {
		return fmt.Sprintf("(i32.const %s)", node.Value)
	}
	if node.Type == "SYMBOL" {
		switch node.Value {
		case "true":
			return "(i32.const 1)"
		case "false":
			return "(i32.const 0)"
		default:
			reportError(fmt.Sprintf("Wasm backend does not support symbol %q", node.Value), node.Line, node.Column)
		}
	}
	if node.Type != "List" {
		reportError(fmt.Sprintf("Wasm backend does not support %s literals", node.Type), node.Line, node.Column)
	}

	ir, ok := lowerShared(node)
	if !ok {
		if len(node.Children) == 0 {
			reportError("Wasm backend does not support an empty expression", node.Line, node.Column)
		}
		reportError(fmt.Sprintf("Wasm backend does not support %q", node.Children[0].Value), node.Line, node.Column)
	}
	return emitWasmIR(ir, node)
}

func emitWasmIR(ir *IRNode, source *Node) string {
	switch ir.Kind {
	case "binop":
		if len(ir.Kids) != 2 {
			reportError(fmt.Sprintf("%s expects 2 arguments", ir.Op), source.Line, source.Column)
		}
		ops := map[string]string{
			"+": "i32.add", "-": "i32.sub", "*": "i32.mul", "/": "i32.div_s",
			"<": "i32.lt_s", ">": "i32.gt_s", "<=": "i32.le_s", ">=": "i32.ge_s",
			"=": "i32.eq", "==": "i32.eq", "!=": "i32.ne", "and": "i32.and", "or": "i32.or",
		}
		return fmt.Sprintf("(%s %s %s)", ops[ir.Op], generateWasmExpression(ir.Kids[0]), generateWasmExpression(ir.Kids[1]))
	case "if":
		if len(ir.Kids) != 3 {
			reportError("Wasm backend requires if to include an else branch", source.Line, source.Column)
		}
		return fmt.Sprintf("(if (result i32) %s (then %s) (else %s))", generateWasmExpression(ir.Kids[0]), generateWasmExpression(ir.Kids[1]), generateWasmExpression(ir.Kids[2]))
	case "do":
		if len(ir.Kids) == 0 {
			reportError("Wasm backend requires do to contain a result expression", source.Line, source.Column)
		}
		parts := make([]string, 0, len(ir.Kids))
		for index, kid := range ir.Kids {
			expr := generateWasmExpression(kid)
			if index < len(ir.Kids)-1 {
				expr = fmt.Sprintf("(drop %s)", expr)
			}
			parts = append(parts, expr)
		}
		return fmt.Sprintf("(block (result i32) %s)", strings.Join(parts, " "))
	case "return":
		return fmt.Sprintf("(return %s)", generateWasmExpression(ir.Kids[0]))
	default:
		reportError(fmt.Sprintf("Wasm backend does not support %q", ir.Kind), source.Line, source.Column)
	}
	return ""
}

func generateJSStatement(node *Node, reqVar string, depth int) string {
	code := generateJSStatementRaw(node, reqVar, depth)
	if node.Type != "List" || len(node.Children) == 0 {
		return code
	}
	head := node.Children[0].Value
	switch head {
	case "return", "let", "do", "try_let", "spawn", "if", "print", "for", "sleep", "while", "match", "set", "call":
		if node.Filename != "" {
			return fmt.Sprintf("//line %s:%d\n%s", node.Filename, node.Line, code)
		}
	}
	return code
}

func generateJSExpression(node *Node, reqVar string, depth int) string {
	return generateJSStatementRaw(node, reqVar, depth)
}

func generateJSStatementRaw(node *Node, reqVar string, depth int) string {
	if depth > 1000 {
		reportError("AST too deep", node.Line, node.Column)
	}
	if node.Type == "STRING" {
		return fmt.Sprintf("%q", node.Value)
	}
	if node.Type == "SYMBOL" || node.Type == "INT" {
		return node.Value
	}
	if node.Type != "List" || len(node.Children) == 0 {
		reportError("Expected list for statement", node.Line, node.Column)
	}
	head := node.Children[0].Value
	if head == "intent" {
		return ""
	}
	if ir, ok := lowerShared(node); ok {
		return emitJSIR(ir, reqVar, depth)
	}
	if head == "let" {
		if len(node.Children) != 3 {
			reportError("let expects (let (var val) body) — wrap multiple body statements in (do ...)", node.Line, node.Column)
		}
		var letPrefix strings.Builder
		letPrefix.WriteString("{\n")
		declaredVars := make(map[string]bool)

		curr := node
		for curr.Type == "List" && len(curr.Children) == 3 && curr.Children[0].Value == "let" {
			binds := curr.Children[1]
			if binds.Type != "List" || len(binds.Children) != 2 {
				reportError("let binding expects (var val)", binds.Line, binds.Column)
			}
			varName := binds.Children[0].Value
			valNode := binds.Children[1]

			var valStr string
			if valNode.Type == "STRING" {
				valStr = fmt.Sprintf("%q", valNode.Value)
			} else if valNode.Type == "List" && len(valNode.Children) > 0 {
				funcName := valNode.Children[0].Value
				if funcName == "call" {
					var args []string
					for j := 2; j < len(valNode.Children); j++ {
						if valNode.Children[j].Type == "STRING" {
							args = append(args, fmt.Sprintf("%q", valNode.Children[j].Value))
						} else {
							args = append(args, generateJSExpression(valNode.Children[j], reqVar, depth+1))
						}
					}
					valStr = fmt.Sprintf("(await %s(%s))", valNode.Children[1].Value, strings.Join(args, ", "))
				} else if funcName == "list" {
					var items []string
					for j := 1; j < len(valNode.Children); j++ {
						if valNode.Children[j].Type == "STRING" {
							items = append(items, fmt.Sprintf("%q", valNode.Children[j].Value))
						} else {
							items = append(items, generateJSExpression(valNode.Children[j], reqVar, depth+1))
						}
					}
					valStr = fmt.Sprintf("[%s]", strings.Join(items, ", "))
				} else if funcName == "dict" {
					var pairs []string
					for j := 1; j < len(valNode.Children); j++ {
						pair := valNode.Children[j]
						if pair.Type == "List" && len(pair.Children) == 2 {
							k := pair.Children[0].Value
							if pair.Children[0].Type == "STRING" {
								k = fmt.Sprintf("%q", k)
							}
							v := pair.Children[1].Value
							if pair.Children[1].Type == "STRING" {
								v = fmt.Sprintf("%q", v)
							} else {
								v = generateJSExpression(pair.Children[1], reqVar, depth+1)
							}
							pairs = append(pairs, fmt.Sprintf("%s: %s", k, v))
						}
					}
					valStr = fmt.Sprintf("{%s}", strings.Join(pairs, ", "))
				} else if funcName == "parse_json" {
					bodyVar := valNode.Children[2].Value
					valStr = fmt.Sprintf("JSON.parse(%s)", bodyVar)
				} else {
					valStr = generateJSStatementRaw(valNode, reqVar, depth+1)
				}
			} else {
				valStr = generateJSStatementRaw(valNode, reqVar, depth+1)
			}

			if declaredVars[varName] {
				letPrefix.WriteString(fmt.Sprintf("%s = %s;\n", varName, valStr))
			} else {
				letPrefix.WriteString(fmt.Sprintf("let %s = %s;\n", varName, valStr))
				declaredVars[varName] = true
			}
			curr = curr.Children[2]
		}
		bodyCode := generateJSStatement(curr, reqVar, depth+1)
		return fmt.Sprintf("%s%s\n}", letPrefix.String(), bodyCode)
	} else if head == "try_let" {
		if len(node.Children) != 4 {
			reportError("try_let expects (try_let (var val) (catch err catchBody) successBody)", node.Line, node.Column)
		}
		binds := node.Children[1]
		varName := binds.Children[0].Value
		valNode := binds.Children[1]

		var valStr string
		if valNode.Type == "List" && len(valNode.Children) > 0 && valNode.Children[0].Value == "parse_json" {
			bodyVar := valNode.Children[2].Value
			valStr = fmt.Sprintf("JSON.parse(%s)", bodyVar)
		} else {
			valStr = generateJSStatementRaw(valNode, reqVar, depth+1)
		}

		catchNode := node.Children[2]
		errVar := catchNode.Children[1].Value
		catchBodyCode := generateJSStatement(catchNode.Children[2], reqVar, depth+1)
		successBodyCode := generateJSStatement(node.Children[3], reqVar, depth+1)

		return fmt.Sprintf("{\n\tlet %s;\n\tlet %s = null;\n\ttry {\n\t\t%s = %s;\n\t} catch (e) {\n\t\t%s = e;\n\t}\n\tif (%s !== null) {\n\t\t%s\n\t} else {\n\t\t%s\n\t}\n}", varName, errVar, varName, valStr, errVar, errVar, catchBodyCode, successBodyCode)
	} else if head == "dom_query" {
		if len(node.Children) != 2 {
			reportError("dom_query expects (dom_query selector)", node.Line, node.Column)
		}
		selector := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		return fmt.Sprintf("document.querySelector(%s)", selector)
	} else if head == "on_event" {
		if len(node.Children) != 4 {
			reportError("on_event expects (on_event el event lambda)", node.Line, node.Column)
		}
		el := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		event := generateJSStatementRaw(node.Children[2], reqVar, depth+1)
		lambda := node.Children[3]
		if lambda.Type != "List" || len(lambda.Children) != 3 || lambda.Children[0].Value != "lambda" {
			reportError("on_event expects a lambda", lambda.Line, lambda.Column)
		}
		args := lambda.Children[1].Children
		argName := "e"
		if len(args) > 0 {
			argName = args[0].Value
		}
		body := generateJSStatement(lambda.Children[2], reqVar, depth+1)
		return fmt.Sprintf("%s.addEventListener(%s, async (%s) => {\n%s\n})", el, event, argName, body)
	} else if head == "set_text" {
		if len(node.Children) != 3 {
			reportError("set_text expects (set_text el val)", node.Line, node.Column)
		}
		el := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		val := generateJSStatementRaw(node.Children[2], reqVar, depth+1)
		return fmt.Sprintf("%s.textContent = %s", el, val)
	} else if head == "set_attr" {
		if len(node.Children) != 4 {
			reportError("set_attr expects (set_attr el name val)", node.Line, node.Column)
		}
		el := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		attr := generateJSStatementRaw(node.Children[2], reqVar, depth+1)
		val := generateJSStatementRaw(node.Children[3], reqVar, depth+1)
		return fmt.Sprintf("%s.setAttribute(%s, %s)", el, attr, val)
	} else if head == "fetch" {
		if len(node.Children) != 3 {
			reportError("fetch expects (fetch url method)", node.Line, node.Column)
		}
		urlStr := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		methodStr := generateJSStatementRaw(node.Children[2], reqVar, depth+1)
		return fmt.Sprintf("(await fetch(%s, { method: %s }).then(r => r.text()))", urlStr, methodStr)
	} else if head == "for" {
		if len(node.Children) != 4 {
			reportError("for expects (for item list body)", node.Line, node.Column)
		}
		itemNode := node.Children[1].Value
		listNode := node.Children[2].Value
		bodyCode := generateJSStatement(node.Children[3], reqVar, depth+1)
		return fmt.Sprintf("for (let %s of %s) {\n%s\n}", itemNode, listNode, bodyCode)
	} else if head == "call" {
		if len(node.Children) < 2 {
			reportError("call expects (call func args...)", node.Line, node.Column)
		}
		funcName := node.Children[1].Value
		var args []string
		for j := 2; j < len(node.Children); j++ {
			args = append(args, generateJSExpression(node.Children[j], reqVar, depth+1))
		}
		return fmt.Sprintf("(await %s(%s))", funcName, strings.Join(args, ", "))
	} else if head == "spawn" {
		if len(node.Children) != 2 {
			reportError("spawn expects (spawn (lambda () body))", node.Line, node.Column)
		}
		lambdaNode := node.Children[1]
		if lambdaNode.Type != "List" || len(lambdaNode.Children) != 3 || lambdaNode.Children[0].Value != "lambda" {
			reportError("spawn expects a lambda", lambdaNode.Line, lambdaNode.Column)
		}
		bodyCode := generateJSStatement(lambdaNode.Children[2], reqVar, depth+1)
		return fmt.Sprintf("(async () => {\n%s\n})()", bodyCode)
	}

	reportError(fmt.Sprintf("Unknown statement for JS: %s", head), node.Line, node.Column)
	return ""
}

func main() {
	outDir := flag.String("o", "", "output directory")
	runMode := flag.Bool("run", false, "interpret and execute a cli_app script directly (Phase 1 of improvement #49: no Go/JS text generated, no go build/go run invoked)")
	compileBc := flag.Bool("compile-bc", false, "compile AST to bytecode JSON")
	runBc := flag.Bool("run-bc", false, "run bytecode from JSON file")
	flag.Parse()

	if flag.NArg() < 1 {
		reportError("Missing file argument", 0, 0)
	}
	inputFile := flag.Arg(0)

	content, err := os.ReadFile(inputFile)
	if err != nil {
		reportError(fmt.Sprintf("Cannot read file: %v", err), 0, 0)
	}

	if *runBc {
		var prog BCProgram
		buf := bytes.NewReader(content)
		dec := gob.NewDecoder(buf)
		if err := dec.Decode(&prog); err != nil {
			reportError(fmt.Sprintf("Cannot parse bytecode: %v", err), 0, 0)
		}
		os.Exit(RunBytecode(&prog, flag.Args()[1:]))
	}

	lexer := NewLexer(string(content))
	parser := NewParser(lexer, filepath.Base(inputFile))
	ast := parser.parseExpression()

	if parser.cur.Type != TokenEOF {
		reportError("Unexpected tokens after EOF", parser.cur.Line, parser.cur.Column)
	}

	expandIncludes(ast, filepath.Dir(inputFile), 0)

	applyPatches(ast)
	ast = applyWithContext(ast, nil)

	ast = applyWithContext(ast, nil)

	if *compileBc {
		prog := CompileToBytecode(ast)
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		if err := enc.Encode(prog); err != nil {
			reportError(fmt.Sprintf("Failed to encode bytecode: %v", err), 0, 0)
		}
		outFile := inputFile + ".bc.bin"
		if *outDir != "" {
			outFile = filepath.Join(*outDir, filepath.Base(inputFile)+".bc.bin")
		}
		if err = os.WriteFile(outFile, buf.Bytes(), 0644); err != nil {
			reportError(fmt.Sprintf("Failed to write %s: %v", outFile, err), 0, 0)
		}
		os.Exit(0)
	}

	if *runMode {
		os.Exit(Interpret(ast, flag.Args()[1:]))
	}

	if ast != nil && ast.Type == "List" && len(ast.Children) > 0 && ast.Children[0].Type == "SYMBOL" && ast.Children[0].Value == "wasm_app" {
		wasmCode := generateWasmCode(ast)
		wasmFile := filepath.Join(*outDir, "app.wat")
		if err = os.WriteFile(wasmFile, []byte(wasmCode), 0644); err != nil {
			reportError(fmt.Sprintf("Failed to write %s: %v", wasmFile, err), 0, 0)
		}
	} else if ast != nil && ast.Type == "List" && len(ast.Children) > 0 && ast.Children[0].Type == "SYMBOL" && ast.Children[0].Value == "web_app" {
		jsCode, testCode := generateJSCode(ast)

		appFile := filepath.Join(*outDir, "app.js")
		appTestFile := filepath.Join(*outDir, "app.test.js")

		err = os.WriteFile(appFile, []byte(jsCode), 0644)
		if err != nil {
			reportError(fmt.Sprintf("Failed to write %s: %v", appFile, err), 0, 0)
		}
		if testCode != "" {
			err = os.WriteFile(appTestFile, []byte(testCode), 0644)
			if err != nil {
				reportError(fmt.Sprintf("Failed to write %s: %v", appTestFile, err), 0, 0)
			}
		} else {
			os.Remove(appTestFile)
		}
	} else {
		goCode, testCode := generateCode(ast)

		serverFile := filepath.Join(*outDir, "server.go")
		serverTestFile := filepath.Join(*outDir, "server_test.go")

		err = os.WriteFile(serverFile, []byte(goCode), 0644)
		if err != nil {
			reportError(fmt.Sprintf("Failed to write %s: %v", serverFile, err), 0, 0)
		}

		if testCode != "" {
			err = os.WriteFile(serverTestFile, []byte(testCode), 0644)
			if err != nil {
				reportError(fmt.Sprintf("Failed to write %s: %v", serverTestFile, err), 0, 0)
			}
		} else {
			os.Remove(serverTestFile)
		}
	}
}

// interpreter.go implements Phase 1 of improvement #49 (Direct Neural
// Bytecode Synthesis): a tree-walking interpreter that executes a cli_app
// AST directly, with no Go/JS text ever generated and no go build/go run/
// node subprocess ever invoked. See docs/direct_execution_design.md for the
// full design, covered node subset, and documented deviations from the Go
// backend.

// returnSignal unwinds a (return val) out of arbitrarily nested if/while/do
// blocks via panic/recover, mirroring Go's own return semantics.
type returnSignal struct{ value any }

type interpEnv struct {
	vars   map[string]any
	parent *interpEnv
}

func newInterpEnv(parent *interpEnv) *interpEnv {
	return &interpEnv{vars: make(map[string]any), parent: parent}
}

func (e *interpEnv) get(name string) (any, bool) {
	for env := e; env != nil; env = env.parent {
		if v, ok := env.vars[name]; ok {
			return v, true
		}
	}
	return nil, false
}

func (e *interpEnv) set(name string, val any) bool {
	for env := e; env != nil; env = env.parent {
		if _, ok := env.vars[name]; ok {
			env.vars[name] = val
			return true
		}
	}
	return false
}

type interpFunc struct {
	params []string
	body   *Node
}

// Interpreter holds the global function table for a single -run invocation.
// defun bodies have no closure over caller scope, matching the Go backend's
// model where defun compiles to an independent top-level function.
type Interpreter struct {
	funcs map[string]*interpFunc
	args  []string
}

func interpErr(reason string, node *Node) {
	line, col := 0, 0
	if node != nil {
		line, col = node.Line, node.Column
	}
	reportError(reason, line, col)
}

// Interpret executes a cli_app AST directly and returns a process exit code.
// http_server/web_app roots are rejected with a clear error — Phase 1 is
// cli_app only, per docs/direct_execution_design.md.
func Interpret(ast *Node, args []string) int {
	if ast == nil || ast.Type != "List" || len(ast.Children) == 0 || ast.Children[0].Type != "SYMBOL" {
		interpErr("Expected cli_app as root symbol", ast)
	}
	root := ast.Children[0].Value
	if root != "cli_app" {
		interpErr(fmt.Sprintf("-run only supports cli_app in Phase 1 (see docs/direct_execution_design.md); got %q", root), ast.Children[0])
	}

	interp := &Interpreter{funcs: make(map[string]*interpFunc), args: args}
	globalEnv := newInterpEnv(nil)

	for _, child := range ast.Children[1:] {
		if isDefun(child) {
			interp.registerDefun(child)
		}
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(returnSignal); ok {
					return
				}
				panic(r)
			}
		}()
		for _, child := range ast.Children[1:] {
			if isDefun(child) {
				continue
			}
			interp.eval(child, globalEnv)
		}
	}()
	return 0
}

func isDefun(node *Node) bool {
	return node.Type == "List" && len(node.Children) > 0 &&
		node.Children[0].Type == "SYMBOL" && node.Children[0].Value == "defun"
}

func (interp *Interpreter) registerDefun(node *Node) {
	if len(node.Children) < 4 {
		interpErr("defun expects (defun name (args) body)", node)
	}
	name := node.Children[1].Value
	argsNode := node.Children[2]
	var params []string
	for _, arg := range argsNode.Children {
		if arg.Type == "List" && len(arg.Children) >= 1 {
			params = append(params, arg.Children[0].Value)
		} else {
			params = append(params, arg.Value)
		}
	}
	body := node.Children[len(node.Children)-1]
	interp.funcs[name] = &interpFunc{params: params, body: body}
}

func (interp *Interpreter) eval(node *Node, env *interpEnv) any {
	switch node.Type {
	case "STRING":
		return node.Value
	case "INT":
		v, err := strconv.ParseInt(node.Value, 10, 64)
		if err != nil {
			interpErr(fmt.Sprintf("invalid integer literal: %s", node.Value), node)
		}
		return v
	case "SYMBOL":
		if v, ok := env.get(node.Value); ok {
			return v
		}
		interpErr(fmt.Sprintf("undefined variable: %s", node.Value), node)
	case "List":
		return interp.evalList(node, env)
	}
	interpErr(fmt.Sprintf("cannot evaluate node of type %s", node.Type), node)
	return nil
}

func (interp *Interpreter) evalList(node *Node, env *interpEnv) any {
	if len(node.Children) == 0 {
		interpErr("empty expression", node)
	}
	head := node.Children[0].Value

	if binOpKinds[head] {
		return interp.evalBinop(head, node, env)
	}

	switch head {
	case "intent":
		return nil
	case "let":
		return interp.evalLet(node, env)
	case "set":
		return interp.evalSet(node, env)
	case "if":
		return interp.evalIf(node, env)
	case "while":
		return interp.evalWhile(node, env)
	case "for":
		return interp.evalFor(node, env)
	case "do":
		var result any
		for _, kid := range node.Children[1:] {
			result = interp.eval(kid, env)
		}
		return result
	case "print":
		var args []any
		for _, kid := range node.Children[1:] {
			args = append(args, interp.eval(kid, env))
		}
		fmt.Println(args...)
		return nil
	case "return":
		if len(node.Children) != 2 {
			interpErr("return expects (return val)", node)
		}
		panic(returnSignal{value: interp.eval(node.Children[1], env)})
	case "call":
		return interp.evalCall(node, env)
	case "confidence":
		if len(node.Children) != 2 {
			interpErr("confidence expects (confidence prompt)", node)
		}
		promptStr := fmt.Sprintf("%v", interp.eval(node.Children[1], env))
		reqBody, _ := json.Marshal(map[string]any{
			"model":  "llama3",
			"prompt": "Evaluate the probability of this statement being true. Return ONLY a float between 0.0 and 1.0. Statement: " + promptStr,
			"stream": false,
		})
		resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		var res struct {
			Response string `json:"response"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			panic(err)
		}
		val, _ := strconv.ParseFloat(strings.TrimSpace(res.Response), 64)
		return val
	case "list":
		var items []any
		for _, kid := range node.Children[1:] {
			items = append(items, interp.eval(kid, env))
		}
		return items
	case "dict":
		d := make(map[string]any)
		for _, kid := range node.Children[1:] {
			if kid.Type != "List" || len(kid.Children) != 2 {
				interpErr("dict expects (k v) pairs", kid)
			}
			k := fmt.Sprint(interp.eval(kid.Children[0], env))
			v := interp.eval(kid.Children[1], env)
			d[k] = v
		}
		return d
	case "append":
		return interp.evalAppend(node, env)
	case "map_set":
		return interp.evalMapSet(node, env)
	case "map_delete":
		return interp.evalMapDelete(node, env)
	case "map_get":
		return interp.evalMapGet(node, env)
	case "list_get":
		return interp.evalListGet(node, env)
	case "to_int":
		if len(node.Children) != 2 {
			interpErr("to_int expects (to_int val)", node)
		}
		v, _ := strconv.ParseInt(strings.TrimSpace(fmt.Sprint(interp.eval(node.Children[1], env))), 10, 64)
		return v
	case "to_float":
		if len(node.Children) != 2 {
			interpErr("to_float expects (to_float val)", node)
		}
		v, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(interp.eval(node.Children[1], env))), 64)
		return v
	case "to_string":
		if len(node.Children) != 2 {
			interpErr("to_string expects (to_string val)", node)
		}
		return fmt.Sprint(interp.eval(node.Children[1], env))
	case "bytes_to_string":
		if len(node.Children) != 2 {
			interpErr("bytes_to_string expects (bytes_to_string val)", node)
		}
		val := interp.eval(node.Children[1], env)
		if b, ok := val.([]byte); ok {
			return string(b)
		}
		return fmt.Sprint(val)
	case "str_split":
		if len(node.Children) != 3 {
			interpErr("str_split expects (str_split s sep)", node)
		}
		s := fmt.Sprint(interp.eval(node.Children[1], env))
		sep := fmt.Sprint(interp.eval(node.Children[2], env))
		parts := strings.Split(s, sep)
		items := make([]any, len(parts))
		for i, p := range parts {
			items[i] = p
		}
		return items
	case "str_join":
		if len(node.Children) != 3 {
			interpErr("str_join expects (str_join list sep)", node)
		}
		listVal := interp.eval(node.Children[1], env)
		sep := fmt.Sprint(interp.eval(node.Children[2], env))
		items, ok := listVal.([]any)
		if !ok {
			interpErr("str_join expects a list", node.Children[1])
		}
		strs := make([]string, len(items))
		for i, it := range items {
			strs[i] = fmt.Sprint(it)
		}
		return strings.Join(strs, sep)
	case "regex_match":
		if len(node.Children) != 3 {
			interpErr("regex_match expects (regex_match pattern s)", node)
		}
		pat := fmt.Sprint(interp.eval(node.Children[1], env))
		s := fmt.Sprint(interp.eval(node.Children[2], env))
		matched, err := regexp.MatchString(pat, s)
		if err != nil {
			interpErr(fmt.Sprintf("invalid regex: %v", err), node)
		}
		return matched
	case "cli_args":
		if len(node.Children) == 1 {
			return sliceToAny(interp.args)
		} else if len(node.Children) == 2 {
			idx, err := toInt(interp.eval(node.Children[1], env))
			if err != nil {
				interpErr("cli_args index must be a number", node)
			}
			if int(idx) >= 0 && int(idx) < len(interp.args) {
				return interp.args[idx]
			}
			return ""
		}
		interpErr("cli_args expects (cli_args) or (cli_args index)", node)
	case "sleep":
		if len(node.Children) != 2 {
			interpErr("sleep expects (sleep ms)", node)
		}
		ms, err := toInt(interp.eval(node.Children[1], env))
		if err != nil {
			interpErr("sleep expects a numeric argument", node)
		}
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return nil
	case "env":
		if len(node.Children) != 2 {
			interpErr("env expects (env \"KEY\")", node)
		}
		key := fmt.Sprint(interp.eval(node.Children[1], env))
		return os.Getenv(key)
	}

	interpErr(fmt.Sprintf("%q is not supported under -run in Phase 1 (see docs/direct_execution_design.md)", head), node.Children[0])
	return nil
}

func (interp *Interpreter) evalLet(node *Node, env *interpEnv) any {
	if len(node.Children) != 3 {
		interpErr("let expects (let (var val) body) — wrap multiple body statements in (do ...)", node)
	}
	binding := node.Children[1]
	if binding.Type != "List" || len(binding.Children) != 2 {
		interpErr("let binding expects (var val)", binding)
	}
	varName := binding.Children[0].Value
	val := interp.eval(binding.Children[1], env)
	childEnv := newInterpEnv(env)
	childEnv.vars[varName] = val
	return interp.eval(node.Children[2], childEnv)
}

func (interp *Interpreter) evalSet(node *Node, env *interpEnv) any {
	if len(node.Children) != 3 {
		interpErr("set expects (set var val)", node)
	}
	varName := node.Children[1].Value
	val := interp.eval(node.Children[2], env)
	if !env.set(varName, val) {
		interpErr(fmt.Sprintf("undefined variable: %s", varName), node.Children[1])
	}
	return nil
}

func (interp *Interpreter) evalIf(node *Node, env *interpEnv) any {
	if len(node.Children) != 3 && len(node.Children) != 4 {
		interpErr("if expects (if cond then [else])", node)
	}
	if toBool(interp.eval(node.Children[1], env), node.Children[1]) {
		return interp.eval(node.Children[2], env)
	} else if len(node.Children) == 4 {
		return interp.eval(node.Children[3], env)
	}
	return nil
}

func (interp *Interpreter) evalWhile(node *Node, env *interpEnv) any {
	if len(node.Children) != 3 {
		interpErr("while expects (while cond body)", node)
	}
	for toBool(interp.eval(node.Children[1], env), node.Children[1]) {
		interp.eval(node.Children[2], env)
	}
	return nil
}

func (interp *Interpreter) evalFor(node *Node, env *interpEnv) any {
	if len(node.Children) != 4 {
		interpErr("for expects (for item list body)", node)
	}
	itemName := node.Children[1].Value
	listVal := interp.eval(node.Children[2], env)
	items, ok := listVal.([]any)
	if !ok {
		interpErr("for requires a list value to iterate", node.Children[2])
	}
	for _, item := range items {
		childEnv := newInterpEnv(env)
		childEnv.vars[itemName] = item
		interp.eval(node.Children[3], childEnv)
	}
	return nil
}

func (interp *Interpreter) evalCall(node *Node, env *interpEnv) any {
	if len(node.Children) < 2 {
		interpErr("call expects (call func args...)", node)
	}
	funcName := node.Children[1].Value
	fn, ok := interp.funcs[funcName]
	if !ok {
		interpErr(fmt.Sprintf("%q is not a defined function (only user defun functions are callable under -run in Phase 1)", funcName), node.Children[1])
	}
	var argVals []any
	for _, argNode := range node.Children[2:] {
		argVals = append(argVals, interp.eval(argNode, env))
	}
	if len(argVals) != len(fn.params) {
		interpErr(fmt.Sprintf("%s expects %d argument(s), got %d", funcName, len(fn.params), len(argVals)), node)
	}
	callEnv := newInterpEnv(nil)
	for i, p := range fn.params {
		callEnv.vars[p] = argVals[i]
	}
	var result any
	func() {
		defer func() {
			if r := recover(); r != nil {
				if rs, ok := r.(returnSignal); ok {
					result = rs.value
					return
				}
				panic(r)
			}
		}()
		interp.eval(fn.body, callEnv)
	}()
	return result
}

func (interp *Interpreter) evalAppend(node *Node, env *interpEnv) any {
	if len(node.Children) != 3 {
		interpErr("append expects (append list item)", node)
	}
	listNode := node.Children[1]
	if listNode.Type != "SYMBOL" {
		interpErr("append requires a symbol for list", listNode)
	}
	current, ok := env.get(listNode.Value)
	if !ok {
		interpErr(fmt.Sprintf("undefined variable: %s", listNode.Value), listNode)
	}
	items, ok := current.([]any)
	if !ok {
		interpErr(fmt.Sprintf("append target %q is not a list", listNode.Value), listNode)
	}
	item := interp.eval(node.Children[2], env)
	newItems := append(append([]any{}, items...), item)
	if !env.set(listNode.Value, newItems) {
		interpErr(fmt.Sprintf("undefined variable: %s", listNode.Value), listNode)
	}
	return nil
}

func (interp *Interpreter) evalMapSet(node *Node, env *interpEnv) any {
	if len(node.Children) != 4 {
		interpErr("map_set expects (map_set dict key val)", node)
	}
	dictNode := node.Children[1]
	if dictNode.Type != "SYMBOL" {
		interpErr("map_set requires a symbol for dict", dictNode)
	}
	current, ok := env.get(dictNode.Value)
	if !ok {
		interpErr(fmt.Sprintf("undefined variable: %s", dictNode.Value), dictNode)
	}
	d, ok := current.(map[string]any)
	if !ok {
		interpErr(fmt.Sprintf("map_set target %q is not a dict", dictNode.Value), dictNode)
	}
	key := fmt.Sprint(interp.eval(node.Children[2], env))
	val := interp.eval(node.Children[3], env)
	d[key] = val
	return nil
}

func (interp *Interpreter) evalMapDelete(node *Node, env *interpEnv) any {
	if len(node.Children) != 3 {
		interpErr("map_delete expects (map_delete dict key)", node)
	}
	dictNode := node.Children[1]
	if dictNode.Type != "SYMBOL" {
		interpErr("map_delete requires a symbol for dict", dictNode)
	}
	current, ok := env.get(dictNode.Value)
	if !ok {
		interpErr(fmt.Sprintf("undefined variable: %s", dictNode.Value), dictNode)
	}
	d, ok := current.(map[string]any)
	if !ok {
		interpErr(fmt.Sprintf("map_delete target %q is not a dict", dictNode.Value), dictNode)
	}
	key := fmt.Sprint(interp.eval(node.Children[2], env))
	delete(d, key)
	return nil
}

func (interp *Interpreter) evalMapGet(node *Node, env *interpEnv) any {
	if len(node.Children) != 3 {
		interpErr("map_get expects (map_get dict key)", node)
	}
	dictNode := node.Children[1]
	if dictNode.Type != "SYMBOL" {
		interpErr("map_get requires a symbol for dict", dictNode)
	}
	current, ok := env.get(dictNode.Value)
	if !ok {
		interpErr(fmt.Sprintf("undefined variable: %s", dictNode.Value), dictNode)
	}
	d, ok := current.(map[string]any)
	if !ok {
		interpErr(fmt.Sprintf("map_get target %q is not a dict", dictNode.Value), dictNode)
	}
	key := fmt.Sprint(interp.eval(node.Children[2], env))
	return d[key]
}

func (interp *Interpreter) evalListGet(node *Node, env *interpEnv) any {
	if len(node.Children) != 3 {
		interpErr("list_get expects (list_get list idx)", node)
	}
	listNode := node.Children[1]
	if listNode.Type != "SYMBOL" {
		interpErr("list_get requires a symbol for list", listNode)
	}
	current, ok := env.get(listNode.Value)
	if !ok {
		interpErr(fmt.Sprintf("undefined variable: %s", listNode.Value), listNode)
	}
	items, ok := current.([]any)
	if !ok {
		interpErr(fmt.Sprintf("list_get target %q is not a list", listNode.Value), listNode)
	}
	idxVal := interp.eval(node.Children[2], env)
	idx, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(idxVal)))
	if err != nil {
		interpErr("list_get index must be a number", node.Children[2])
	}
	if idx < 0 || idx >= len(items) {
		return ""
	}
	return items[idx]
}

func (interp *Interpreter) evalBinop(op string, node *Node, env *interpEnv) any {
	if len(node.Children) != 3 {
		interpErr(fmt.Sprintf("%s expects 2 arguments", op), node)
	}
	a := interp.eval(node.Children[1], env)
	b := interp.eval(node.Children[2], env)
	switch op {
	case "and":
		return toBool(a, node.Children[1]) && toBool(b, node.Children[2])
	case "or":
		return toBool(a, node.Children[1]) || toBool(b, node.Children[2])
	case "==", "=":
		return valuesEqual(a, b)
	case "!=":
		return !valuesEqual(a, b)
	case "+":
		if as, ok := a.(string); ok {
			if bs, ok2 := b.(string); ok2 {
				return as + bs
			}
		}
		return numericBinop(op, a, b, node)
	default: // - * / < > <= >=
		return numericBinop(op, a, b, node)
	}
}

func numericBinop(op string, a, b any, node *Node) any {
	ai, aIsInt := a.(int64)
	bi, bIsInt := b.(int64)
	if aIsInt && bIsInt {
		switch op {
		case "+":
			return ai + bi
		case "-":
			return ai - bi
		case "*":
			return ai * bi
		case "/":
			if bi == 0 {
				interpErr("division by zero", node)
			}
			return ai / bi
		case "<":
			return ai < bi
		case ">":
			return ai > bi
		case "<=":
			return ai <= bi
		case ">=":
			return ai >= bi
		}
	}

	af, aOk := toFloat(a)
	bf, bOk := toFloat(b)
	if !aOk || !bOk {
		interpErr(fmt.Sprintf("%s requires numeric operands, got %T and %T", op, a, b), node)
	}
	switch op {
	case "+":
		return af + bf
	case "-":
		return af - bf
	case "*":
		return af * bf
	case "/":
		if bf == 0 {
			interpErr("division by zero", node)
		}
		return af / bf
	case "<":
		return af < bf
	case ">":
		return af > bf
	case "<=":
		return af <= bf
	case ">=":
		return af >= bf
	}
	return nil
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case int64:
		return float64(t), true
	case float64:
		return t, true
	}
	return 0, false
}

func toInt(v any) (int64, error) {
	switch t := v.(type) {
	case int64:
		return t, nil
	case float64:
		return int64(t), nil
	case string:
		return strconv.ParseInt(t, 10, 64)
	}
	return 0, fmt.Errorf("cannot convert %T to int", v)
}

func toBool(v any, node *Node) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	interpErr(fmt.Sprintf("expected boolean, got %T", v), node)
	return false
}

func valuesEqual(a, b any) bool {
	switch a.(type) {
	case []any, map[string]any:
		return false
	}
	switch b.(type) {
	case []any, map[string]any:
		return false
	}
	return a == b
}

func sliceToAny(strs []string) []any {
	out := make([]any, len(strs))
	for i, s := range strs {
		out[i] = s
	}
	return out
}

type BCInstruction struct {
	Op   string `json:"op"`
	Args []any  `json:"args,omitempty"`
}

type BCProgram struct {
	Version   int                    `json:"version"`
	Functions map[string]*BCFunction `json:"functions"`
	Main      []BCInstruction        `json:"main"`
}

type BCFunction struct {
	Params       []string        `json:"params"`
	Instructions []BCInstruction `json:"instructions"`
}

// compiler state
type BCCompiler struct {
	prog  *BCProgram
	funcs map[string]*BCFunction
}

func CompileToBytecode(ast *Node) *BCProgram {
	comp := &BCCompiler{
		prog: &BCProgram{
			Version:   1,
			Functions: make(map[string]*BCFunction),
		},
		funcs: make(map[string]*BCFunction),
	}

	// Phase 1 interpreter expects ast to be a single block (cli_app) usually,
	// but ast is just a Node.
	mainInsts := comp.compileNode(ast)
	comp.prog.Main = mainInsts
	comp.prog.Functions = comp.funcs
	return comp.prog
}

func (c *BCCompiler) compileNode(node *Node) []BCInstruction {
	if node == nil {
		return nil
	}
	var insts []BCInstruction
	if node.Type == "INT" {
		val, _ := strconv.ParseInt(node.Value, 10, 64)
		insts = append(insts, BCInstruction{Op: "LOAD_CONST", Args: []any{float64(val)}})
		return insts
	}
	if node.Type == "STRING" {
		insts = append(insts, BCInstruction{Op: "LOAD_CONST", Args: []any{node.Value}})
		return insts
	}
	if node.Type == "SYMBOL" {
		insts = append(insts, BCInstruction{Op: "LOAD_VAR", Args: []any{node.Value}})
		return insts
	}

	if node.Type == "List" && len(node.Children) > 0 {
		head := node.Children[0].Value
		switch head {
		case "cli_app":
			for _, child := range node.Children[1:] {
				insts = append(insts, c.compileNode(child)...)
			}
		case "defun":
			funcName := node.Children[1].Value
			var params []string
			for _, p := range node.Children[2].Children {
				params = append(params, p.Value)
			}
			var bodyInsts []BCInstruction
			for _, child := range node.Children[3:] {
				bodyInsts = append(bodyInsts, c.compileNode(child)...)
			}
			c.funcs[funcName] = &BCFunction{
				Params:       params,
				Instructions: bodyInsts,
			}
		case "type_hint":
			// ignore type hints in execution

		case "try_let":
			binding := node.Children[1]
			varName := binding.Children[0].Value
			valInsts := c.compileNode(binding.Children[1])

			catchNode := node.Children[2]
			errVar := catchNode.Children[1].Value
			catchBodyInsts := c.compileNode(catchNode.Children[2])
			successBodyInsts := c.compileNode(node.Children[3])

			insts = append(insts, BCInstruction{
				Op:   "TRY_LET",
				Args: []any{varName, errVar, float64(len(valInsts)), float64(len(catchBodyInsts)), float64(len(successBodyInsts))},
			})
			insts = append(insts, valInsts...)
			insts = append(insts, catchBodyInsts...)
			insts = append(insts, successBodyInsts...)
		case "db_connect":
			varName := node.Children[1].Value
			driverNode := node.Children[2]
			dsnNode := node.Children[3]
			insts = append(insts, BCInstruction{Op: "DB_CONNECT", Args: []any{varName, driverNode.Value, dsnNode.Value}})
		case "sql_query":
			dbVar := node.Children[1].Value
			queryStr := node.Children[2].Value
			insts = append(insts, BCInstruction{Op: "SQL_QUERY", Args: []any{dbVar, queryStr}})
		case "fetch":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, c.compileNode(node.Children[2])...)
			insts = append(insts, BCInstruction{Op: "FETCH"})
		case "read_file":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, BCInstruction{Op: "READ_FILE"})
		case "write_file":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, c.compileNode(node.Children[2])...)
			insts = append(insts, BCInstruction{Op: "WRITE_FILE"})
		case "mkdir":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, BCInstruction{Op: "MKDIR"})
		case "exec":
			insts = append(insts, c.compileNode(node.Children[1])...)
			for _, arg := range node.Children[2:] {
				insts = append(insts, c.compileNode(arg)...)
			}
			insts = append(insts, BCInstruction{Op: "EXEC", Args: []any{float64(len(node.Children) - 2)}})
		case "parse_json":
			bodyVar := node.Children[2].Value
			insts = append(insts, BCInstruction{Op: "PARSE_JSON", Args: []any{bodyVar}})
		case "spawn":
			lambdaNode := node.Children[1]
			bodyInsts := c.compileNode(lambdaNode.Children[2])
			insts = append(insts, BCInstruction{Op: "SPAWN", Args: []any{float64(len(bodyInsts))}})
			insts = append(insts, bodyInsts...)
		case "res":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, c.compileNode(node.Children[2])...)
			insts = append(insts, c.compileNode(node.Children[3])...)
			insts = append(insts, BCInstruction{Op: "RES"})
		case "res_json":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, c.compileNode(node.Children[2])...)
			insts = append(insts, BCInstruction{Op: "RES_JSON"})
		case "http_server":
			portNode := node.Children[1]
			insts = append(insts, BCInstruction{Op: "HTTP_SERVER_START", Args: []any{portNode.Value}})
			for _, child := range node.Children[2:] {
				if child.Type == "List" && len(child.Children) > 0 && child.Children[0].Value == "route" {
					path := child.Children[1].Value
					reqVar := child.Children[2].Children[1].Children[0].Value
					bodyInsts := c.compileNode(child.Children[2].Children[2])
					insts = append(insts, BCInstruction{Op: "HTTP_ROUTE", Args: []any{path, reqVar, float64(len(bodyInsts))}})
					insts = append(insts, bodyInsts...)
				} else {
					insts = append(insts, c.compileNode(child)...)
				}
			}
			insts = append(insts, BCInstruction{Op: "HTTP_SERVER_SERVE"})
		case "confidence":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, BCInstruction{Op: "CONFIDENCE"})

		case "llm_generate":
			insts = append(insts, c.compileNode(node.Children[1])...)
			modelStr := "llama3"
			if len(node.Children) >= 3 {
				modelStr = node.Children[2].Value
			}
			insts = append(insts, BCInstruction{Op: "LLM_GENERATE", Args: []any{modelStr}})

		case "let":
			binding := node.Children[1]
			varName := binding.Children[0].Value
			valInsts := c.compileNode(binding.Children[1])

			insts = append(insts, valInsts...)
			insts = append(insts, BCInstruction{Op: "STORE_VAR", Args: []any{varName}})
			insts = append(insts, c.compileNode(node.Children[2])...)
		case "set":
			varName := node.Children[1].Value
			insts = append(insts, c.compileNode(node.Children[2])...)
			insts = append(insts, BCInstruction{Op: "SET_VAR", Args: []any{varName}})
		case "if":
			condInsts := c.compileNode(node.Children[1])
			thenInsts := c.compileNode(node.Children[2])
			var elseInsts []BCInstruction
			if len(node.Children) == 4 {
				elseInsts = c.compileNode(node.Children[3])
			}
			insts = append(insts, condInsts...)

			// JUMP_IF_FALSE to else (or end)
			if len(elseInsts) == 0 {
				insts = append(insts, BCInstruction{Op: "JUMP_IF_FALSE", Args: []any{float64(len(thenInsts) + 1)}})
				insts = append(insts, thenInsts...)
			} else {
				insts = append(insts, BCInstruction{Op: "JUMP_IF_FALSE", Args: []any{float64(len(thenInsts) + 2)}})
				insts = append(insts, thenInsts...)
				insts = append(insts, BCInstruction{Op: "JUMP", Args: []any{float64(len(elseInsts) + 1)}}) // jump past else
				insts = append(insts, elseInsts...)
			}
		case "while":
			condInsts := c.compileNode(node.Children[1])
			bodyInsts := c.compileNode(node.Children[2])

			insts = append(insts, condInsts...)
			// jump to end if false
			insts = append(insts, BCInstruction{Op: "JUMP_IF_FALSE", Args: []any{float64(len(bodyInsts) + 2)}})
			insts = append(insts, bodyInsts...)
			// jump back to cond
			insts = append(insts, BCInstruction{Op: "JUMP", Args: []any{float64(-(len(condInsts) + 1 + len(bodyInsts)))}})
		case "for":
			// (for item list body)
			itemName := node.Children[1].Value
			listInsts := c.compileNode(node.Children[2])
			bodyInsts := c.compileNode(node.Children[3])

			insts = append(insts, listInsts...)
			insts = append(insts, BCInstruction{Op: "FOR_INIT"}) // pops list, pushes iterator

			loopStart := len(insts)
			insts = append(insts, BCInstruction{Op: "FOR_NEXT", Args: []any{itemName, float64(len(bodyInsts) + 2)}}) // if done, jump to end
			insts = append(insts, bodyInsts...)
			insts = append(insts, BCInstruction{Op: "JUMP", Args: []any{float64(-(len(insts) - loopStart))}})
		case "return":
			if len(node.Children) > 1 {
				insts = append(insts, c.compileNode(node.Children[1])...)
			} else {
				insts = append(insts, BCInstruction{Op: "LOAD_CONST", Args: []any{nil}})
			}
			insts = append(insts, BCInstruction{Op: "RETURN"})
		case "print":
			for _, arg := range node.Children[1:] {
				insts = append(insts, c.compileNode(arg)...)
			}
			insts = append(insts, BCInstruction{Op: "PRINT", Args: []any{float64(len(node.Children) - 1)}})
		case "call":
			funcName := node.Children[1].Value
			for _, arg := range node.Children[2:] {
				insts = append(insts, c.compileNode(arg)...)
			}
			insts = append(insts, BCInstruction{Op: "CALL", Args: []any{funcName, float64(len(node.Children) - 2)}})
		case "+", "-", "*", "/", "<", ">", "<=", ">=", "==", "!=", "=", "and", "or":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, c.compileNode(node.Children[2])...)
			op := head
			if op == "=" {
				op = "=="
			}
			insts = append(insts, BCInstruction{Op: "BINOP", Args: []any{op}})
		case "to_int", "to_float", "to_string", "bytes_to_string":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, BCInstruction{Op: "CONVERT", Args: []any{head}})
		case "str_split":
			insts = append(insts, c.compileNode(node.Children[1])...) // string
			insts = append(insts, c.compileNode(node.Children[2])...) // sep
			insts = append(insts, BCInstruction{Op: "STR_SPLIT"})
		case "str_join":
			insts = append(insts, c.compileNode(node.Children[1])...) // list
			insts = append(insts, c.compileNode(node.Children[2])...) // sep
			insts = append(insts, BCInstruction{Op: "STR_JOIN"})
		case "regex_match":
			insts = append(insts, c.compileNode(node.Children[1])...) // regex
			insts = append(insts, c.compileNode(node.Children[2])...) // string
			insts = append(insts, BCInstruction{Op: "REGEX_MATCH"})
		case "list":
			for _, child := range node.Children[1:] {
				insts = append(insts, c.compileNode(child)...)
			}
			insts = append(insts, BCInstruction{Op: "MAKE_LIST", Args: []any{float64(len(node.Children) - 1)}})
		case "dict":
			for _, child := range node.Children[1:] {
				if len(child.Children) == 2 {
					insts = append(insts, c.compileNode(child.Children[0])...) // key
					insts = append(insts, c.compileNode(child.Children[1])...) // value
				}
			}
			insts = append(insts, BCInstruction{Op: "MAKE_DICT", Args: []any{float64(len(node.Children) - 1)}})
		case "append":
			insts = append(insts, c.compileNode(node.Children[2])...)
			insts = append(insts, BCInstruction{Op: "APPEND", Args: []any{node.Children[1].Value}})
		case "map_set":
			insts = append(insts, c.compileNode(node.Children[2])...) // key
			insts = append(insts, c.compileNode(node.Children[3])...) // value
			insts = append(insts, BCInstruction{Op: "MAP_SET", Args: []any{node.Children[1].Value}})
		case "map_delete":
			insts = append(insts, c.compileNode(node.Children[2])...) // key
			insts = append(insts, BCInstruction{Op: "MAP_DELETE", Args: []any{node.Children[1].Value}})
		case "map_get":
			insts = append(insts, c.compileNode(node.Children[2])...) // key
			insts = append(insts, BCInstruction{Op: "MAP_GET", Args: []any{node.Children[1].Value}})
		case "list_get":
			insts = append(insts, c.compileNode(node.Children[2])...) // index
			insts = append(insts, BCInstruction{Op: "LIST_GET", Args: []any{node.Children[1].Value}})
		case "cli_args":
			insts = append(insts, BCInstruction{Op: "CLI_ARGS"})
		case "sleep":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, BCInstruction{Op: "SLEEP"})
		case "env":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, BCInstruction{Op: "ENV"})
		case "do":
			for _, child := range node.Children[1:] {
				insts = append(insts, c.compileNode(child)...)
			}
		}
	}
	return insts
}

// VM state
type BCVM struct {
	prog  *BCProgram
	stack []any
	env   *bcEnv
	ip    int
	insts []BCInstruction
	args  []string
}

type bcEnv struct {
	vars   map[string]any
	parent *bcEnv
}

func newBcEnv(parent *bcEnv) *bcEnv {
	return &bcEnv{vars: make(map[string]any), parent: parent}
}

func (e *bcEnv) get(name string) (any, bool) {
	if val, ok := e.vars[name]; ok {
		return val, true
	}
	if e.parent != nil {
		return e.parent.get(name)
	}
	return nil, false
}

func (e *bcEnv) set(name string, val any) bool {
	if _, ok := e.vars[name]; ok {
		e.vars[name] = val
		return true
	}
	if e.parent != nil {
		return e.parent.set(name, val)
	}
	return false // undefined
}

type vmReturn struct {
	val any
}

func RunBytecode(prog *BCProgram, cliArgs []string) int {
	vm := &BCVM{
		prog:  prog,
		env:   newBcEnv(nil),
		insts: prog.Main,
		args:  cliArgs,
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("VM panic: %v\n", r)
			os.Exit(1)
		}
	}()

	vm.run(vm.insts, vm.env)
	return 0
}

func (vm *BCVM) push(v any) {
	vm.stack = append(vm.stack, v)
}

func (vm *BCVM) pop(inst string) any {
	if len(vm.stack) == 0 {
		panic("stack underflow at " + inst)
	}
	v := vm.stack[len(vm.stack)-1]
	vm.stack = vm.stack[:len(vm.stack)-1]
	return v
}

func (vm *BCVM) run(insts []BCInstruction, env *bcEnv) any {
	ip := 0
	for ip < len(insts) {
		inst := insts[ip]
		switch inst.Op {

		case "TRY_LET":
			varName := inst.Args[0].(string)
			errVar := inst.Args[1].(string)
			valLen := int(inst.Args[2].(float64))
			catchLen := int(inst.Args[3].(float64))
			successLen := int(inst.Args[4].(float64))

			valInsts := insts[ip+1 : ip+1+valLen]
			catchInsts := insts[ip+1+valLen : ip+1+valLen+catchLen]
			successInsts := insts[ip+1+valLen+catchLen : ip+1+valLen+catchLen+successLen]

			var tryErr error
			func() {
				defer func() {
					if r := recover(); r != nil {
						if _, ok := r.(vmReturn); ok {
							panic(r)
						}
						tryErr = fmt.Errorf("%v", r)
					}
				}()
				vm.run(valInsts, env)
			}()

			if tryErr != nil {
				env.vars[errVar] = tryErr.Error()
				vm.run(catchInsts, env)
			} else {
				env.vars[varName] = vm.pop(inst.Op)
				vm.run(successInsts, env)
			}
			ip += valLen + catchLen + successLen
			continue
		case "DB_CONNECT":
			varName := inst.Args[0].(string)
			driver := inst.Args[1].(string)
			dsn := inst.Args[2].(string)
			db, err := sql.Open(driver, dsn)
			if err != nil {
				panic(err)
			}
			env.vars[varName] = db
		case "SQL_QUERY":
			dbVar := inst.Args[0].(string)
			queryStr := inst.Args[1].(string)
			dbAny, ok := env.get(dbVar)
			if !ok {
				panic("undefined db: " + dbVar)
			}
			db := dbAny.(*sql.DB)
			rows, err := db.Query(queryStr)
			if err != nil {
				panic(err)
			}
			var results []any
			cols, _ := rows.Columns()
			for rows.Next() {
				vals := make([]any, len(cols))
				valPtrs := make([]any, len(cols))
				for i := range vals {
					valPtrs[i] = &vals[i]
				}
				rows.Scan(valPtrs...)
				rowMap := make(map[string]any)
				for i, col := range cols {
					if b, ok := vals[i].([]byte); ok {
						rowMap[col] = string(b)
					} else {
						rowMap[col] = vals[i]
					}
				}
				results = append(results, rowMap)
			}
			vm.push(results)
		case "FETCH":
			method := vm.pop(inst.Op).(string)
			urlStr := vm.pop(inst.Op).(string)
			req, err := http.NewRequest(method, urlStr, nil)
			if err != nil {
				panic(err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				panic(err)
			}
			defer resp.Body.Close()
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				panic(err)
			}
			var bytesAny []any
			for _, bb := range b {
				bytesAny = append(bytesAny, float64(bb))
			}
			vm.push(bytesAny)
		case "READ_FILE":
			path := vm.pop(inst.Op).(string)
			b, err := os.ReadFile(path)
			if err != nil {
				panic(err)
			}
			var bytesAny []any
			for _, bb := range b {
				bytesAny = append(bytesAny, float64(bb))
			}
			vm.push(bytesAny)
		case "WRITE_FILE":
			dataAny := vm.pop(inst.Op)
			path := vm.pop(inst.Op).(string)
			var data []byte
			if s, ok := dataAny.(string); ok {
				data = []byte(s)
			} else if b, ok := dataAny.([]any); ok {
				for _, bb := range b {
					data = append(data, byte(bb.(float64)))
				}
			}
			err := os.WriteFile(path, data, 0644)
			if err != nil {
				panic(err)
			}
		case "MKDIR":
			path := vm.pop(inst.Op).(string)
			err := os.MkdirAll(path, 0755)
			if err != nil {
				panic(err)
			}
		case "EXEC":
			numArgs := int(inst.Args[0].(float64))
			var args []string
			for i := 0; i < numArgs; i++ {
				args = append([]string{fmt.Sprint(vm.pop(inst.Op))}, args...)
			}
			cmdStr := fmt.Sprint(vm.pop(inst.Op))
			out, err := exec.Command(cmdStr, args...).CombinedOutput()
			if err != nil {
				panic(err)
			}
			var bytesAny []any
			for _, bb := range out {
				bytesAny = append(bytesAny, float64(bb))
			}
			vm.push(bytesAny)
		case "PARSE_JSON":
			bodyVar := inst.Args[0].(string)
			var data []byte
			if bodyVar == "req.body" {
				reqAny, ok := env.get("req")
				if !ok {
					panic("undefined req")
				}
				req := reqAny.(*http.Request)
				data, _ = io.ReadAll(req.Body)
				req.Body = io.NopCloser(bytes.NewBuffer(data))
			} else {
				bodyAny, ok := env.get(bodyVar)
				if !ok {
					panic("undefined var: " + bodyVar)
				}
				if s, ok := bodyAny.(string); ok {
					data = []byte(s)
				}
				if b, ok := bodyAny.([]any); ok {
					for _, bb := range b {
						data = append(data, byte(bb.(float64)))
					}
				}
			}
			var result any
			err := json.Unmarshal(data, &result)
			if err != nil {
				panic(err)
			}
			vm.push(result)
		case "SPAWN":
			bodyLen := int(inst.Args[0].(float64))
			bodyInsts := insts[ip+1 : ip+1+bodyLen]
			capturedEnv := newBcEnv(nil)
			for e := env; e != nil; e = e.parent {
				for k, v := range e.vars {
					if _, exists := capturedEnv.vars[k]; !exists {
						capturedEnv.vars[k] = v
					}
				}
			}
			go func(cEnv *bcEnv) {
				defer func() { recover() }()
				childVM := &BCVM{prog: vm.prog, env: cEnv}
				childVM.run(bodyInsts, cEnv)
			}(capturedEnv)
			ip += bodyLen
			continue
		case "RES":
			bodyAny := vm.pop(inst.Op)
			contentType := vm.pop(inst.Op).(string)
			status := int(toBCFloat(vm.pop(inst.Op)))
			wAny, ok := env.get("w")
			if !ok {
				panic("no response writer")
			}
			w := wAny.(http.ResponseWriter)
			w.Header().Set("Content-Type", contentType)
			w.WriteHeader(status)
			if s, ok := bodyAny.(string); ok {
				w.Write([]byte(s))
			} else if b, ok := bodyAny.([]any); ok {
				var data []byte
				for _, bb := range b {
					data = append(data, byte(bb.(float64)))
				}
				w.Write(data)
			}
		case "RES_JSON":
			data := vm.pop(inst.Op)
			status := int(toBCFloat(vm.pop(inst.Op)))
			wAny, ok := env.get("w")
			if !ok {
				panic("no response writer")
			}
			w := wAny.(http.ResponseWriter)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(data)
		case "HTTP_SERVER_START":
			port := inst.Args[0].(string)
			env.vars["__http_mux"] = http.NewServeMux()
			env.vars["__http_port"] = port
		case "HTTP_ROUTE":
			path := inst.Args[0].(string)
			reqVar := inst.Args[1].(string)
			bodyLen := int(inst.Args[2].(float64))
			bodyInsts := insts[ip+1 : ip+1+bodyLen]

			muxAny, _ := env.get("__http_mux")
			mux := muxAny.(*http.ServeMux)

			capturedEnv := newBcEnv(nil)
			for e := env; e != nil; e = e.parent {
				for k, v := range e.vars {
					if _, exists := capturedEnv.vars[k]; !exists {
						capturedEnv.vars[k] = v
					}
				}
			}

			prog := vm.prog
			mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
				reqEnv := newBcEnv(capturedEnv)
				reqEnv.vars["w"] = w
				reqEnv.vars[reqVar] = r
				reqEnv.vars["req"] = r
				childVM := &BCVM{prog: prog, env: reqEnv}
				func() {
					defer func() { recover() }()
					childVM.run(bodyInsts, reqEnv)
				}()
			})
			ip += bodyLen
			continue
		case "HTTP_SERVER_SERVE":
			muxAny, _ := env.get("__http_mux")
			mux := muxAny.(*http.ServeMux)
			portAny, _ := env.get("__http_port")
			port := portAny.(string)
			fmt.Println("Listening on " + port)
			err := http.ListenAndServe(port, mux)
			if err != nil {
				panic(err)
			}
		case "CONFIDENCE":
			promptStrAny := vm.pop(inst.Op)
			promptStr := fmt.Sprintf("%v", promptStrAny)
			reqBody, _ := json.Marshal(map[string]any{
				"model":  "llama3",
				"prompt": "Evaluate the probability of this statement being true. Return ONLY a float between 0.0 and 1.0. Statement: " + promptStr,
				"stream": false,
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil {
				panic(err)
			}
			defer resp.Body.Close()
			var res struct {
				Response string `json:"response"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
				panic(err)
			}
			val, _ := strconv.ParseFloat(strings.TrimSpace(res.Response), 64)
			vm.push(val)

		case "LLM_GENERATE":
			modelStr := inst.Args[0].(string)
			promptStr := vm.pop(inst.Op).(string)
			reqBody, _ := json.Marshal(map[string]any{
				"model":  modelStr,
				"prompt": promptStr,
				"stream": false,
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil {
				panic(err)
			}
			defer resp.Body.Close()
			var res struct {
				Response string `json:"response"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
				panic(err)
			}
			vm.push(res.Response)

		case "LOAD_CONST":
			vm.push(inst.Args[0])
		case "LOAD_VAR":
			name := inst.Args[0].(string)
			if val, ok := env.get(name); ok {
				vm.push(val)
			} else {
				panic("undefined variable: " + name)
			}
		case "STORE_VAR":
			name := inst.Args[0].(string)
			env.vars[name] = vm.pop(inst.Op)
		case "SET_VAR":
			name := inst.Args[0].(string)
			if !env.set(name, vm.pop(inst.Op)) {
				panic("undefined variable: " + name)
			}
		case "JUMP_IF_FALSE":
			cond := vm.pop(inst.Op)
			if !bcToBool(cond) {
				ip += int(inst.Args[0].(float64))
				continue
			}
		case "JUMP":
			ip += int(inst.Args[0].(float64))
			continue
		case "FOR_INIT":
			list := vm.pop(inst.Op)
			items, ok := list.([]any)
			if !ok {
				panic("for requires a list")
			}
			vm.push(items)
			vm.push(0.0) // index
		case "FOR_NEXT":
			varName := inst.Args[0].(string)
			offset := int(inst.Args[1].(float64))

			idxAny := vm.pop(inst.Op)
			itemsAny := vm.pop(inst.Op)

			idx := int(idxAny.(float64))
			items := itemsAny.([]any)

			if idx < len(items) {
				env.vars[varName] = items[idx]
				vm.push(items)
				vm.push(float64(idx + 1))
			} else {
				ip += offset
				continue
			}
		case "CALL":
			funcName := inst.Args[0].(string)
			numArgs := int(inst.Args[1].(float64))

			fn, ok := vm.prog.Functions[funcName]
			if !ok {
				panic("undefined function: " + funcName)
			}

			var argVals []any
			for i := 0; i < numArgs; i++ {
				argVals = append([]any{vm.pop(inst.Op)}, argVals...)
			}

			callEnv := newBcEnv(nil)
			for i, p := range fn.Params {
				callEnv.vars[p] = argVals[i]
			}

			var result any
			func() {
				defer func() {
					if r := recover(); r != nil {
						if ret, ok := r.(vmReturn); ok {
							result = ret.val
							return
						}
						panic(r)
					}
				}()
				vm.run(fn.Instructions, callEnv)
			}()
			vm.push(result)
		case "RETURN":
			panic(vmReturn{val: vm.pop(inst.Op)})
		case "PRINT":
			numArgs := int(inst.Args[0].(float64))
			var out []string
			var vals []any
			for i := 0; i < numArgs; i++ {
				vals = append([]any{vm.pop(inst.Op)}, vals...)
			}
			for _, v := range vals {
				out = append(out, fmt.Sprint(v))
			}
			fmt.Println(strings.Join(out, " "))
		case "BINOP":
			b := vm.pop(inst.Op)
			a := vm.pop(inst.Op)
			op := inst.Args[0].(string)
			vm.push(bcBinop(op, a, b))
		case "CONVERT":
			a := vm.pop(inst.Op)
			target := inst.Args[0].(string)
			vm.push(bcConvert(target, a))
		case "STR_SPLIT":
			sep := vm.pop(inst.Op).(string)
			s := vm.pop(inst.Op).(string)
			vm.push(bcSliceToAny(strings.Split(s, sep)))
		case "STR_JOIN":
			sep := vm.pop(inst.Op).(string)
			list := vm.pop(inst.Op).([]any)
			var strs []string
			for _, item := range list {
				strs = append(strs, fmt.Sprint(item))
			}
			vm.push(strings.Join(strs, sep))
		case "REGEX_MATCH":
			s := vm.pop(inst.Op).(string)
			pattern := vm.pop(inst.Op).(string)
			matched, err := regexp.MatchString(pattern, s)
			if err != nil {
				panic(err)
			}
			vm.push(matched)
		case "MAKE_LIST":
			num := int(inst.Args[0].(float64))
			var items []any
			for i := 0; i < num; i++ {
				items = append([]any{vm.pop(inst.Op)}, items...)
			}
			vm.push(items)
		case "MAKE_DICT":
			num := int(inst.Args[0].(float64))
			dict := make(map[string]any)
			// pop pairs: value then key
			var pairs []any
			for i := 0; i < num*2; i++ {
				pairs = append([]any{vm.pop(inst.Op)}, pairs...)
			}
			for i := 0; i < num*2; i += 2 {
				key := fmt.Sprint(pairs[i])
				dict[key] = pairs[i+1]
			}
			vm.push(dict)
		case "APPEND":
			varName := inst.Args[0].(string)
			item := vm.pop(inst.Op)

			current, ok := env.get(varName)
			if !ok {
				panic("undefined variable: " + varName)
			}
			items := current.([]any)
			newItems := append(append([]any{}, items...), item)
			env.set(varName, newItems)
		case "MAP_SET":
			varName := inst.Args[0].(string)
			val := vm.pop(inst.Op)
			key := fmt.Sprint(vm.pop(inst.Op))

			current, ok := env.get(varName)
			if !ok {
				panic("undefined variable: " + varName)
			}
			dict := current.(map[string]any)
			dict[key] = val
		case "MAP_DELETE":
			varName := inst.Args[0].(string)
			key := fmt.Sprint(vm.pop(inst.Op))

			current, ok := env.get(varName)
			if !ok {
				panic("undefined variable: " + varName)
			}
			dict := current.(map[string]any)
			delete(dict, key)
		case "MAP_GET":
			varName := inst.Args[0].(string)
			key := fmt.Sprint(vm.pop(inst.Op))

			current, ok := env.get(varName)
			if !ok {
				panic("undefined variable: " + varName)
			}
			dict := current.(map[string]any)
			vm.push(dict[key])
		case "LIST_GET":
			varName := inst.Args[0].(string)
			idxAny := vm.pop(inst.Op)

			idxStr := strings.TrimSpace(fmt.Sprint(idxAny))
			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				panic("list_get index must be a number")
			}

			current, ok := env.get(varName)
			if !ok {
				panic("undefined variable: " + varName)
			}
			items := current.([]any)
			if idx < 0 || idx >= len(items) {
				vm.push("")
			} else {
				vm.push(items[idx])
			}
		case "CLI_ARGS":
			var argsAny []any
			for _, arg := range vm.args {
				argsAny = append(argsAny, arg)
			}
			vm.push(argsAny)
		case "SLEEP":
			msAny := vm.pop(inst.Op)
			ms, ok := msAny.(float64)
			if !ok {
				panic("sleep requires number")
			}
			time.Sleep(time.Duration(ms) * time.Millisecond)
		case "ENV":
			name := vm.pop(inst.Op).(string)
			vm.push(os.Getenv(name))
		default:
			panic("unknown opcode: " + inst.Op)
		}
		ip++
	}
	return nil
}

func bcToBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	panic("expected boolean")
}

func bcBinop(op string, a, b any) any {
	switch op {
	case "and":
		return bcToBool(a) && bcToBool(b)
	case "or":
		return bcToBool(a) || bcToBool(b)
	case "==":
		return bcValuesEqual(a, b)
	case "!=":
		return !bcValuesEqual(a, b)
	case "+":
		if as, ok := a.(string); ok {
			if bs, ok2 := b.(string); ok2 {
				return as + bs
			}
		}
		return bcNumericBinop(op, a, b)
	default:
		return bcNumericBinop(op, a, b)
	}
}

func bcNumericBinop(op string, a, b any) any {
	// try to coerce float64 since JSON uses float64
	af := toBCFloat(a)
	bf := toBCFloat(b)

	switch op {
	case "+":
		return af + bf
	case "-":
		return af - bf
	case "*":
		return af * bf
	case "/":
		return af / bf
	case "<":
		return af < bf
	case ">":
		return af > bf
	case "<=":
		return af <= bf
	case ">=":
		return af >= bf
	}
	panic("unknown binop")
}

func toBCFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	}
	panic(fmt.Sprintf("expected number, got %T", v))
}

func bcValuesEqual(a, b any) bool {
	switch a.(type) {
	case []any, map[string]any:
		return false
	}
	switch b.(type) {
	case []any, map[string]any:
		return false
	}
	return a == b
}

func bcConvert(target string, a any) any {
	switch target {
	case "to_int":
		switch t := a.(type) {
		case float64:
			return float64(int64(t))
		case string:
			v, _ := strconv.ParseInt(t, 10, 64)
			return float64(v)
		}
	case "to_float":
		switch t := a.(type) {
		case string:
			v, _ := strconv.ParseFloat(t, 64)
			return v
		case float64:
			return t
		}
	case "to_string":
		return fmt.Sprint(a)
	case "bytes_to_string":
		if b, ok := a.([]any); ok {
			var bytes []byte
			for _, bb := range b {
				bytes = append(bytes, byte(bb.(float64)))
			}
			return string(bytes)
		}
	}
	return a
}

func bcSliceToAny(strs []string) []any {
	out := make([]any, len(strs))
	for i, s := range strs {
		out[i] = s
	}
	return out
}
