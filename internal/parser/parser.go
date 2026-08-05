package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"zero/internal/ast"
	"zero/internal/lexer"
)

type Parser struct {
	lexer    *lexer.Lexer
	Cur      lexer.Token
	filename string
}

func NewParser(lexer *lexer.Lexer, filename string) *Parser {
	p := &Parser{lexer: lexer, filename: filename}
	p.Cur = p.lexer.NextToken()
	return p
}

func (p *Parser) ParseExpression() *ast.Node {
	if p.Cur.Type == lexer.TokenLParen {
		node := &ast.Node{Type: "List", Line: p.Cur.Line, Column: p.Cur.Column, Filename: p.filename}
		p.Cur = p.lexer.NextToken() // consume '('
		for p.Cur.Type != lexer.TokenRParen && p.Cur.Type != lexer.TokenEOF {
			node.Children = append(node.Children, p.ParseExpression())
		}
		if p.Cur.Type != lexer.TokenRParen {
			ast.ReportError("Expected ')'", p.Cur.Line, p.Cur.Column)
		}
		p.Cur = p.lexer.NextToken() // consume ')'
		return node
	}
	if p.Cur.Type == lexer.TokenSymbol || p.Cur.Type == lexer.TokenInt || p.Cur.Type == lexer.TokenFloat || p.Cur.Type == lexer.TokenString {
		node := &ast.Node{Type: string(p.Cur.Type), Value: p.Cur.Value, Line: p.Cur.Line, Column: p.Cur.Column, Filename: p.filename}
		p.Cur = p.lexer.NextToken()
		return node
	}
	ast.ReportError(fmt.Sprintf("Unexpected token: %s", p.Cur.Value), p.Cur.Line, p.Cur.Column)
	return nil
}

func ExpandIncludes(node *ast.Node, baseDir string, depth int) {
	if depth > 100 {
		ast.ReportError("Include depth exceeded (circular include?)", node.Line, node.Column)
	}
	if node.Type != "List" {
		return
	}
	var newChildren []*ast.Node
	for i := 0; i < len(node.Children); i++ {
		child := node.Children[i]
		if child.Type == "List" && len(child.Children) == 4 && child.Children[0].Type == "SYMBOL" && child.Children[0].Value == "use" && child.Children[2].Type == "SYMBOL" && child.Children[2].Value == "as" {
			filenameNode := child.Children[1]
			if filenameNode.Type != "STRING" {
				ast.ReportError("use expects a string filename", child.Line, child.Column)
			}
			filename := filenameNode.Value
			aliasNode := child.Children[3]
			if aliasNode.Type != "SYMBOL" {
				ast.ReportError("use expects a symbol alias", child.Line, child.Column)
			}
			alias := aliasNode.Value
			fullPath := filepath.Join(baseDir, filename)

			content, err := os.ReadFile(fullPath)
			if err != nil {
				ast.ReportError(fmt.Sprintf("Failed to read used file %q: %v", filename, err), child.Line, child.Column)
			}

			lx := lexer.NewLexer(string(content))
			parser := NewParser(lx, filepath.Base(fullPath))
			includedAst := parser.ParseExpression()

			if parser.Cur.Type != lexer.TokenEOF {
				ast.ReportError(fmt.Sprintf("Unexpected tokens after EOF in used file %q", filename), parser.Cur.Line, parser.Cur.Column)
			}

			ExpandIncludes(includedAst, filepath.Dir(fullPath), depth+1)

			moduleNode := &ast.Node{Type: "List", Line: child.Line, Column: child.Column, Filename: child.Filename}
			moduleNode.Children = append(moduleNode.Children, &ast.Node{Type: "SYMBOL", Value: "module", Line: child.Line, Column: child.Column})
			moduleNode.Children = append(moduleNode.Children, &ast.Node{Type: "STRING", Value: alias, Line: child.Line, Column: child.Column})

			if includedAst.Type == "List" && len(includedAst.Children) > 0 && includedAst.Children[0].Value == "module" {
				moduleNode.Children = append(moduleNode.Children, includedAst.Children[2:]...)
			} else {
				moduleNode.Children = append(moduleNode.Children, includedAst)
			}
			newChildren = append(newChildren, moduleNode)
		} else if child.Type == "List" && len(child.Children) == 2 && child.Children[0].Type == "SYMBOL" && child.Children[0].Value == "include" {
			fmt.Println("Warning: (include) is deprecated, please migrate to (use ...)")
			filenameNode := child.Children[1]
			if filenameNode.Type != "STRING" {
				ast.ReportError("include expects a string filename", child.Line, child.Column)
			}
			filename := filenameNode.Value
			fullPath := filepath.Join(baseDir, filename)

			content, err := os.ReadFile(fullPath)
			if err != nil {
				ast.ReportError(fmt.Sprintf("Failed to read included file %q: %v", filename, err), child.Line, child.Column)
			}

			lx := lexer.NewLexer(string(content))
			parser := NewParser(lx, filepath.Base(fullPath))
			includedAst := parser.ParseExpression()

			if parser.Cur.Type != lexer.TokenEOF {
				ast.ReportError(fmt.Sprintf("Unexpected tokens after EOF in included file %q", filename), parser.Cur.Line, parser.Cur.Column)
			}

			ExpandIncludes(includedAst, filepath.Dir(fullPath), depth+1)

			if includedAst.Type == "List" && len(includedAst.Children) > 0 && includedAst.Children[0].Value == "module" {
				newChildren = append(newChildren, includedAst.Children[2:]...)
			} else {
				newChildren = append(newChildren, includedAst)
			}
		} else {
			ExpandIncludes(child, baseDir, depth)
			newChildren = append(newChildren, child)
		}
	}
	node.Children = newChildren
}
