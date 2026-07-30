package checker

import (
	"testing"
	"zero/internal/ast"
	"zero/internal/ir"
	"zero/internal/lexer"
	"zero/internal/parser"
)

func parseTestProgram(t *testing.T, source string) *ast.Node {
	t.Helper()
	p := parser.NewParser(lexer.NewLexer(source), "types_test.zero")
	root := p.ParseExpression()
	if p.Cur.Type != lexer.TokenEOF {
		t.Fatalf("parser stopped at %s", p.Cur.Value)
	}
	return root
}

func TestAnalyzePropagatesTypesAndNativeLayout(t *testing.T) {
	root := parseTestProgram(t, `(cli_app
		(defun add (a b)
			(type_hint a "int")
			(type_hint b "int")
			(type_hint return "int")
			(return (+ a b)))
		(let (items (list "a" "b"))
			(print (call add 1 2))))`)

	analysis := Analyze(root)
	if len(analysis.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", analysis.Diagnostics)
	}
	add := analysis.Functions["add"]
	if len(add.Params) != 2 || add.Params[0].Kind != ast.Int || add.Params[1].Kind != ast.Int {
		t.Fatalf("unexpected add signature: %+v", add)
	}
	if add.Return.Kind != ast.Int || add.Return.Size != 8 || add.Return.Align != 8 {
		t.Fatalf("unexpected return layout: %+v", add.Return)
	}
	var plus *ast.Node
	var findPlus func(*ast.Node)
	findPlus = func(node *ast.Node) {
		if node == nil {
			return
		}
		if node.Type == "List" && len(node.Children) > 0 && node.Children[0].Value == "+" {
			plus = node
		}
		for _, child := range node.Children {
			findPlus(child)
		}
	}
	findPlus(root)
	typedIR, ok := ir.LowerShared(plus)
	if !ok || typedIR.Type.Kind != ast.Int || typedIR.Type.Size != 8 {
		t.Fatalf("IR did not preserve inferred layout: ok=%v ir=%+v", ok, typedIR)
	}

	var listType ast.TypeInfo
	var visit func(*ast.Node)
	visit = func(node *ast.Node) {
		if node == nil {
			return
		}
		if node.Type == "List" && len(node.Children) > 0 && node.Children[0].Value == "list" {
			listType = node.Inferred
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(root)
	if listType.Kind != ast.List || listType.Element == nil || listType.Element.Kind != ast.String {
		t.Fatalf("unexpected list inference: %+v", listType)
	}
	if !listType.Pointer || listType.Size != 24 || listType.Align != 8 {
		t.Fatalf("unexpected list layout: %+v", listType)
	}
}

func TestAnalyzeReportsProvableTypeErrors(t *testing.T) {
	root := parseTestProgram(t, `(cli_app
		(defun bad () int (return "not an int"))
		(if 1 (print "never")))`)

	analysis := Analyze(root)
	if len(analysis.Diagnostics) != 2 {
		t.Fatalf("expected two diagnostics, got %+v", analysis.Diagnostics)
	}
	if analysis.Diagnostics[0].Line == 0 || analysis.Diagnostics[1].Line == 0 {
		t.Fatalf("diagnostics must retain source locations: %+v", analysis.Diagnostics)
	}
}

func TestAnalyzePropagatesDeferredControlFlow(t *testing.T) {
	root := parseTestProgram(t, `(cli_app
		(let (items (list "a" "b"))
			(for item items (print item)))
		(try_let (n (to_int "1"))
			(catch err (print err))
			(if (> n 0) (print n) (print "no")))
		(spawn (lambda () (print "worker")))
		(match 1 (1 (print "one")) (default (print "other"))))`)

	analysis := Analyze(root)
	if len(analysis.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", analysis.Diagnostics)
	}
	var loopItem ast.TypeInfo
	var visit func(*ast.Node)
	visit = func(node *ast.Node) {
		if node == nil {
			return
		}
		if node.Type == "SYMBOL" && node.Value == "item" {
			loopItem = node.Inferred
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(root)
	if loopItem.Kind != ast.String || loopItem.Size != 16 || !loopItem.Pointer {
		t.Fatalf("for-loop element lost its inferred layout: %+v", loopItem)
	}
}
