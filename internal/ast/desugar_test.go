package ast_test

import (
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func parseDesugar(t *testing.T, source string) *ast.Node {
	t.Helper()
	p := parser.NewParser(lexer.NewLexer(source), "desugar.howl")
	root := p.ParseExpression()
	if p.Cur.Type != lexer.TokenEOF {
		t.Fatalf("parser stopped at %s", p.Cur.Value)
	}
	ast.Desugar(root)
	return root
}

func TestDesugarDefunExprBodyWrapsLastValue(t *testing.T) {
	root := parseDesugar(t, `(cli_app
		(defun add ((a int) (b int)) int (+ a b))
		(print (call add 2 3)))`)
	defun := root.Children[1]
	last := defun.Children[len(defun.Children)-1]
	if last.Children[0].Value != "return" {
		t.Fatalf("expected last body form to be return, got %s", last.Children[0].Value)
	}
	if last.Children[1].Children[0].Value != "+" {
		t.Fatalf("expected return of +, got %#v", last.Children[1])
	}
}

func TestDesugarDefunExprBodyIdempotentAndSkipsExplicitReturn(t *testing.T) {
	root := parseDesugar(t, `(cli_app
		(defun add ((a int) (b int)) int (return (+ a b))))`)
	ast.Desugar(root)
	defun := root.Children[1]
	last := defun.Children[len(defun.Children)-1]
	if last.Children[0].Value != "return" {
		t.Fatalf("expected return, got %s", last.Children[0].Value)
	}
	if last.Children[1].Children[0].Value != "+" {
		t.Fatalf("double-wrap produced nested return: %#v", last)
	}
}

func TestDesugarDefunSkipsVoidAndStatements(t *testing.T) {
	root := parseDesugar(t, `(cli_app
		(defun side (a)
			(type_hint return "void")
			(print a))
		(defun weird () int
			(print "x")))`)
	side := root.Children[1]
	if side.Children[len(side.Children)-1].Children[0].Value != "print" {
		t.Fatalf("void body should stay print, got %#v", side.Children[len(side.Children)-1])
	}
	weird := root.Children[2]
	if weird.Children[len(weird.Children)-1].Children[0].Value != "print" {
		t.Fatalf("statement-only body should stay print, got %#v", weird.Children[len(weird.Children)-1])
	}
}

func TestDesugarAssertExpandsToIf(t *testing.T) {
	root := parseDesugar(t, `(cli_app
		(defun add ((a int) (b int)) int (+ a b))
		(assert (= (call add 2 3) 5)))`)
	asserted := root.Children[2]
	if asserted.Children[0].Value != "if" {
		t.Fatalf("assert should desugar to if, got %s", asserted.Children[0].Value)
	}
	if asserted.Children[1].Children[0].Value != "=" {
		t.Fatalf("expected condition =, got %#v", asserted.Children[1])
	}
	fail := asserted.Children[3]
	if fail.Children[0].Value != "do" || len(fail.Children) < 3 {
		t.Fatalf("expected failing do with stderr/exit, got %#v", fail)
	}
	ast.Desugar(root)
	if root.Children[2].Children[0].Value != "if" {
		t.Fatalf("second Desugar changed assert expansion")
	}
}

func TestDesugarRootDoBecomesCliApp(t *testing.T) {
	root := parseDesugar(t, `(do (print "hi"))`)
	if root.Children[0].Value != "cli_app" {
		t.Fatalf("root do should become cli_app, got %s", root.Children[0].Value)
	}
}

func TestDesugarAutoCall(t *testing.T) {
	root := parseDesugar(t, `(do
		(defun add ((a int) (b int)) int (+ a b))
		(assert (= (add 2 3) 5)))`)
	if root.Children[0].Value != "cli_app" {
		t.Fatalf("expected cli_app root, got %s", root.Children[0].Value)
	}
	cond := root.Children[2].Children[1]
	if cond.Children[0].Value != "=" {
		t.Fatalf("expected = condition, got %#v", cond)
	}
	call := cond.Children[1]
	if call.Children[0].Value != "call" || call.Children[1].Value != "add" {
		t.Fatalf("expected (call add ...), got %#v", call)
	}
}
