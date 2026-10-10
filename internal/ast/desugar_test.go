package ast_test

import (
	"strings"
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

func TestDesugarLinesAndReadText(t *testing.T) {
	root := parseDesugar(t, `(cli_app
		(let (text (read_text "names.txt"))
			(for line (lines text)
				(print line))))`)
	letNode := root.Children[1]
	binding := letNode.Children[1]
	readText := binding.Children[1]
	if readText.Children[0].Value != "bytes_to_string" ||
		readText.Children[1].Children[0].Value != "read_file" {
		t.Fatalf("read_text should expand to bytes_to_string/read_file, got %#v", readText)
	}
	forNode := letNode.Children[2]
	lines := forNode.Children[2]
	if lines.Children[0].Value != "str_split" ||
		lines.Children[1].Children[0].Value != "bytes_to_string" ||
		lines.Children[2].Value != "\n" {
		t.Fatalf("lines should expand to str_split/bytes_to_string, got %#v", lines)
	}
}

func TestDesugarPreservesLinesBindingName(t *testing.T) {
	root := parseDesugar(t, `(cli_app
		(let (content "a\nb")
			(let (lines (str_split content "\n"))
				(print lines))))`)
	outer := root.Children[1]
	inner := outer.Children[2]
	binding := inner.Children[1]
	if binding.Children[0].Value != "lines" {
		t.Fatalf("let binding name lines was rewritten: %#v", binding)
	}
	if binding.Children[1].Children[0].Value != "str_split" {
		t.Fatalf("expected str_split value, got %#v", binding.Children[1])
	}
}

func TestDesugarCondAndMultiLet(t *testing.T) {
	root := parseDesugar(t, `(cli_app
		(let (a 1 b 2)
			(cond
				((= a 1) (print b))
				(else (print "no")))))`)
	outer := root.Children[1]
	if outer.Children[0].Value != "let" {
		t.Fatalf("expected let, got %#v", outer)
	}
	if outer.Children[1].Children[0].Value != "a" {
		t.Fatalf("outer binding should be a, got %#v", outer.Children[1])
	}
	inner := outer.Children[2]
	if inner.Children[0].Value != "let" || inner.Children[1].Children[0].Value != "b" {
		t.Fatalf("multi-let should nest b binding, got %#v", inner)
	}
	cond := inner.Children[2]
	if cond.Children[0].Value != "if" {
		t.Fatalf("cond should expand to if, got %#v", cond)
	}
}

func TestDesugarSGetSPut(t *testing.T) {
	root := parseDesugar(t, `(cli_app
		(sput kv "k" "v")
		(print (sget kv "k")))`)
	put := root.Children[1]
	if put.Children[0].Value != "store_put" {
		t.Fatalf("sput should expand to store_put, got %#v", put)
	}
	if put.Children[3].Children[0].Value != "dict" {
		t.Fatalf("sput should wrap dict, got %#v", put.Children[3])
	}
	printNode := root.Children[2]
	get := printNode.Children[1]
	if get.Children[0].Value != "let" {
		t.Fatalf("sget should expand to let, got %#v", get)
	}
	ast.Desugar(root)
	if root.Children[1].Children[0].Value != "store_put" {
		t.Fatalf("second Desugar changed sput expansion")
	}
}

func TestDesugarCondInsideLambda(t *testing.T) {
	root := parseDesugar(t, "(http_server 8090 (route \"/x\" (lambda (req) (cond ((= (req_method req) \"GET\") (res_json 200 (dict (\"ok\" \"1\")))) (else (res_json 405 (dict (\"error\" \"no\"))))))))")
	if root == nil || len(root.Children) < 3 {
		t.Fatalf("unexpected root: %#v", root)
	}
	route := root.Children[2]
	if route == nil || len(route.Children) < 3 {
		t.Fatalf("unexpected route: %#v", route)
	}
	lambda := route.Children[2]
	if lambda == nil || len(lambda.Children) < 3 {
		t.Fatalf("unexpected lambda: %#v", lambda)
	}
	body := lambda.Children[2]
	if body == nil || len(body.Children) == 0 || body.Children[0].Value != "if" {
		t.Fatalf("cond inside lambda should desugar to if, got %#v", body)
	}
}

func TestDesugarSInc(t *testing.T) {
	root := parseDesugar(t, `(cli_app
		(store_open kv "memory://session")
		(sinc kv "visits"))`)
	sinc := root.Children[2]
	if sinc.Children[0].Value != "let" {
		t.Fatalf("sinc should expand to let, got %#v", sinc)
	}
	src := ast.Stringify(sinc)
	if !strings.Contains(src, "store_get") || !strings.Contains(src, "store_put") || !strings.Contains(src, "print") {
		t.Fatalf("sinc expansion missing store/print forms: %s", src)
	}
}
