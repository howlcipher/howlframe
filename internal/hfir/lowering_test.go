package hfir

import (
	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
	"testing"
)

func TestASTLowering(t *testing.T) {
	root := &ast.Node{
		Type: "List",
		Children: []*ast.Node{
			{Type: "SYMBOL", Value: "add"},
			{Type: "INT", Value: "1"},
			{Type: "INT", Value: "2"},
		},
	}

	g, err := LowerAST(root, "test")
	if err != nil {
		t.Fatal(err)
	}

	if len(g.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(g.Nodes))
	}

	if g.EntryNode != "n1" {
		t.Errorf("expected entry node n1, got %s", g.EntryNode)
	}
}

// TestASTLoweringHandlesIntegerLiterals is a regression test for the
// NUMBER-vs-INT mismatch (bug #40): the old hand-built ast.Node{Type:
// "NUMBER"} fixtures never went through the real lexer, so they masked the
// fact that real integer literals lex as "INT" and could not be lowered.
func TestASTLoweringHandlesIntegerLiterals(t *testing.T) {
	src := `(cli_app (print 42))`
	lx := lexer.NewLexer(src)
	p := parser.NewParser(lx, "int_literal_test.howl")
	root := p.ParseExpression()
	if p.Cur.Type != lexer.TokenEOF {
		t.Fatalf("unexpected tokens after EOF")
	}

	g, err := LowerAST(root, "test")
	if err != nil {
		t.Fatalf("LowerAST failed on a real integer literal: %v", err)
	}
	if len(g.Nodes) != 3 {
		t.Errorf("expected 3 nodes (cli_app, print, const 42), got %d", len(g.Nodes))
	}
}

// TestASTLoweringOnRealCheckedFixture lowers a real checker-validated cli_app
// program end to end (lexer -> parser -> checker.Check -> LowerAST), closing
// the gap where every prior test used hand-built AST/graph literals.
func TestASTLoweringOnRealCheckedFixture(t *testing.T) {
	src := `(cli_app (let (x 1) (print (+ x 2))))`
	lx := lexer.NewLexer(src)
	p := parser.NewParser(lx, "checked_fixture_test.howl")
	root := p.ParseExpression()
	if p.Cur.Type != lexer.TokenEOF {
		t.Fatalf("unexpected tokens after EOF")
	}

	checker.Check(root)

	g, err := LowerAST(root, "test")
	if err != nil {
		t.Fatalf("LowerAST failed on a real checked fixture: %v", err)
	}
	if g.EntryNode == "" {
		t.Errorf("expected a non-empty entry node")
	}
}

// TestASTLoweringHandlesEmptyParameterLists is a regression test for a
// second lowering gap found while validating the production gate against
// the real tests/*.howl corpus: a zero-argument defun/lambda parameter list
// (e.g. "(defun f () ...)") lowers to an empty ast.Node{Type:"List"} with no
// children, which previously fell through to LowerAST's unknown-node-type
// error path since only non-empty Lists were handled.
func TestASTLoweringHandlesEmptyParameterLists(t *testing.T) {
	src := `(cli_app (defun f () (return 1)) (print (call f)))`
	lx := lexer.NewLexer(src)
	p := parser.NewParser(lx, "empty_params_test.howl")
	root := p.ParseExpression()
	if p.Cur.Type != lexer.TokenEOF {
		t.Fatalf("unexpected tokens after EOF")
	}

	g, err := LowerAST(root, "test")
	if err != nil {
		t.Fatalf("LowerAST failed on a zero-argument defun: %v", err)
	}
	if g.EntryNode == "" {
		t.Errorf("expected a non-empty entry node")
	}
}

func TestASTLoweringReturnsErrorNotPanicOnUnknownNodeType(t *testing.T) {
	root := &ast.Node{Type: "BOGUS_TYPE", Value: "x"}

	_, err := LowerAST(root, "test")
	if err == nil {
		t.Fatal("expected an error for an unrecognized AST node type, got nil")
	}
}
